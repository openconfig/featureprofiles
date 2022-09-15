import shutil
from celery.utils.log import get_task_logger
from firexapp.engine.celery import app
from firexapp.common import silent_mkdir
from firexapp.submit.arguments import whitelist_arguments
from firexapp.firex_subprocess import check_output, CommandFailed
from microservices.firex_base import returns, flame, InjectArgs, FireX
from microservices.runners.go_b4_tasks import B4GoClone, get_go_env, copy_test_logs_dir, write_output_from_results_json
from microservices.runners.runner_base import FireXRunnerBase
from microservices.testbed_tasks import register_testbed_file_generator
from services.cflow.code_coverage_tasks import CollectCoverageData
from ci_plugins.vxsim import GenerateGoB4TestbedFile
from test_framework import register_test_framework_provider
from html_helper import get_link 
from collections import namedtuple
from pathlib import Path
from gotest2html import GoTest2HTML
import os
import git 

GO_BIN = '/auto/firex/bin/go'

logger = get_task_logger(__name__)

CloneInfo = namedtuple('CloneInfo', ['url', 'path'])
ONDATRA_REPO_CLONE_INFO = CloneInfo('https://github.com/openconfig/ondatra.git', 'openconfig/ondatra')
FP_REPO_CLONE_INFO = CloneInfo('git@wwwin-github.cisco.com:B4Test/featureprofiles.git', 'openconfig/featureprofiles')

ONDATRA_PATCHES = ['exec/firex/plugins/ondatra/0001-windows-ixia-path.patch']

whitelist_arguments([
    'ondatra_repo_branch', 
    'fp_repo_branch', 
    'ondatra_testbed_path', 
    'fp_pre_tests',
    'fp_post_tests',
    'test_path', 
    'test_args'
    'test_patch'
])

@app.task(base=FireX, bind=True)
def BringupTestbed(self, uid, ws, images = None,  
                        ondatra_repo_branch='main',
                        fp_repo_branch='kjahed/firex',                        
                        ondatra_testbed_path=None,
                        ondatra_binding_path=None):

    pkgs_parent_path = os.path.join(ws, f'bringup_go_pkgs')

    ondatra_repo_dir = os.path.join(pkgs_parent_path,
                        ONDATRA_REPO_CLONE_INFO.path)
    fp_repo_dir = os.path.join(pkgs_parent_path, 
                        FP_REPO_CLONE_INFO.path)

    c = B4GoClone.s(b4go_pkg_url=ONDATRA_REPO_CLONE_INFO.url,
                        b4go_pkg_path=ondatra_repo_dir,
                        b4go_pkg_branch=ondatra_repo_branch)

    self.enqueue_child_and_get_results(c)

    c = B4GoClone.s(b4go_pkg_url=FP_REPO_CLONE_INFO.url,
                        b4go_pkg_path=fp_repo_dir,
                        b4go_pkg_branch=fp_repo_branch)

    self.enqueue_child_and_get_results(c)

    ondatra_binding_path = os.path.join(fp_repo_dir, ondatra_binding_path)
    ondatra_testbed_path = os.path.join(fp_repo_dir, ondatra_testbed_path)

    check_output(f"sed -i 's|$FP_ROOT|{fp_repo_dir}|g' " + ondatra_binding_path)

    logger.warn(f'Loading image {images}')

    shutil.copy(images, fp_repo_dir)
    image_path = os.path.join(fp_repo_dir, os.path.basename(images))

    install_cmd = f'{GO_BIN} test -v ' \
        f'./exec/utils/osinstall ' \
        f'-timeout 0 ' \
        f'-args ' \
        f'-testbed {ondatra_testbed_path} ' \
        f'-binding {ondatra_binding_path} ' \
        f'-osfile {image_path} ' \
        f'-osver 7.9.1.08Iv1.0.0'

    logger.warn(f'Install cmd: {install_cmd}')

    logger.warn(check_output(install_cmd, cwd=fp_repo_dir))
    shutil.rmtree(pkgs_parent_path)

@app.task(base=FireX, bind=True)
def CleanupTestbed(self, uid, ws):
    pass

@register_test_framework_provider('b4_fp')
def b4_fp_chain_provider(ws,
                         testsuite_id,
                         script_name,
                         script_path,
                         test_log_directory_path,
                         xunit_results_filepath,
                         cflow,
                         ondatra_repo_branch='main',
                         fp_repo_branch='kjahed/firex',
                         ondatra_testbed_path=None,
                         ondatra_binding_path=None,
                         fp_pre_tests=[],
                         fp_post_tests=[],
                         test_path=None,
                         test_args=None,
                         test_patch=None,
                         **kwargs):

    chain = InjectArgs(ws=ws,
                    testsuite_id=testsuite_id,
                    script_name=script_name,
                    script_path=script_path,
                    test_log_directory_path=test_log_directory_path,
                    xunit_results_filepath=xunit_results_filepath,
                    cflow=cflow,
                    ondatra_testbed_path=ondatra_testbed_path,
                    ondatra_binding_path=ondatra_binding_path,
                    test_path=test_path,
                    test_args=test_args,
                    test_patch=test_patch,
                    **kwargs)

    pkgs_parent_path = os.path.join(ws, f'{testsuite_id}_go_pkgs')

    ondatra_repo_dir = os.path.join(pkgs_parent_path,
                        ONDATRA_REPO_CLONE_INFO.path)
    fp_repo_dir = os.path.join(pkgs_parent_path, 
                        FP_REPO_CLONE_INFO.path)

    chain |= B4GoClone.s(b4go_pkg_url=ONDATRA_REPO_CLONE_INFO.url,
                        b4go_pkg_path=ondatra_repo_dir,
                        b4go_pkg_branch=ondatra_repo_branch)

    chain |= B4GoClone.s(b4go_pkg_url=FP_REPO_CLONE_INFO.url,
                        b4go_pkg_path=fp_repo_dir,
                        b4go_pkg_branch=fp_repo_branch)

    chain |= PatchOndatra.s(ondatra_repo=ondatra_repo_dir, 
                            fp_repo=fp_repo_dir)

    if test_patch:
        chain |= PatchFP.s(fp_repo=fp_repo_dir, patch_path=test_patch)

    if fp_pre_tests:
        for pt in fp_pre_tests:
            for k, v in pt.items():
                chain |= RunB4FPTest.s(fp_ws=fp_repo_dir, test_path = v['test_path'], test_args = v.get('test_args'))

    chain |= RunB4FPTest.s(fp_ws=fp_repo_dir, test_path = test_path, test_args = test_args)

    if fp_post_tests:
        for pt in fp_post_tests:
            for k, v in pt.items():
                chain |= RunB4FPTest.s(fp_ws=fp_repo_dir, test_path = v['test_path'], test_args = v.get('test_args'))

    chain |= GoTest2HTML.s(Path(test_log_directory_path) / f'{script_name}.json', Path(test_log_directory_path) / 'results.html')
    
    chain |= Cleanup.s(go_pkgs_dir=pkgs_parent_path)

    if cflow:
        chain |= CollectCoverageData.s(pyats_testbed='@testbed')

    return chain

# noinspection PyPep8Naming
@app.task(bind=True)
def PatchOndatra(self, ondatra_repo, fp_repo):
    ondatra_repo = git.Repo(ondatra_repo)
    for patch in ONDATRA_PATCHES:
        ondatra_repo.git.apply([os.path.join(fp_repo, patch)])

    with open(os.path.join(fp_repo, 'go.mod'), "a") as fp:
        fp.write("replace github.com/openconfig/ondatra => ../ondatra")

# noinspection PyPep8Naming
@app.task(bind=True)
def PatchFP(self, fp_repo, patch_path):
    repo = git.Repo(fp_repo)
    repo.git.apply([os.path.join(fp_repo, patch_path)])

# noinspection PyPep8Naming
@app.task(bind=True)
def Cleanup(self, go_pkgs_dir):
    shutil.rmtree(go_pkgs_dir)
    
# noinspection PyPep8Naming
@app.task(bind=True, base=FireXRunnerBase)
@flame('log_file', lambda p: get_link(p, 'Test Output'))
@flame('test_log_directory_path', lambda p: get_link(p, 'All Logs'))
@returns('cflow_dat_dir', 'xunit_results', 'log_file', "start_time", "stop_time")
def RunB4FPTest(self,
                ws,
                testsuite_id,
                script_name,
                script_path,
                test_log_directory_path,
                xunit_results_filepath,
                testbed_path=None,
                ondatra_testbed_path=None,
                ondatra_binding_path=None,
                test_path=None,
                test_args=None,
                go_args=None,
                fp_ws = None,
                ):

    if not fp_ws: fp_ws = ws
    if ondatra_binding_path: ondatra_binding_path = os.path.join(fp_ws, ondatra_binding_path)
    ondatra_testbed_path = os.path.join(fp_ws, ondatra_testbed_path)
 
    json_results_file = Path(test_log_directory_path) / f'{script_name}.json'
    test_logs_dir_in_ws = Path(ws) / f'{testsuite_id}_logs'

    check_output(f'rm -rf {test_logs_dir_in_ws}')
    silent_mkdir(test_logs_dir_in_ws)

    test_args = test_args or ''
    go_args = go_args or ''

    test_args = f'{test_args} ' \
        f'-log_dir {test_logs_dir_in_ws} ' \
        f'-v 5 ' \
        f'-alsologtostderr'

    ondatra_binding = ondatra_binding_path
    if not ondatra_binding:
        if not testbed_path or not os.path.isfile(testbed_path):
            raise ValueError('`testbed_path` must be a path to the ondatra topo file for ondatra-based tests')

        ondatra_dir = os.path.join(self.task_dir, 'ondatra')
        silent_mkdir(ondatra_dir)
        
        ondatra_binding = os.path.join(ondatra_dir, 'topology.textproto')
        check_output(f'/auto/firex/sw/pyvxr_binding/pyvxr_binding.sh staticbind service {testbed_path}',
                        file=ondatra_binding)
    else:
        check_output(f"sed -i 's|$FP_ROOT|{fp_ws}|g' " + ondatra_binding)

    test_args += f' -binding {ondatra_binding} -testbed {ondatra_testbed_path}'

    go_args = f'{go_args} ' \
                f'-json ' \
                f'-p 1 ' \
                f'-timeout 0'

    if not test_path:
        raise ValueError('test_path must be set for non-compiled go tests')
    test_path = os.path.join(fp_ws, test_path)

    extra_env_vars = {'GOVERSION': '3.0'}  # Needed by gotestsum; 3.0 is what we see when GO is in path
    extra_env_vars.update(get_go_env())

    cmd = f'/auto/firex/bin/gotestsum ' \
          f'--junitfile {xunit_results_filepath} ' \
          f'--junitfile-testsuite-name short ' \
          f'--junitfile-testcase-classname short ' \
          f'--jsonfile {json_results_file} ' \
          f'--format testname ' \
          f'--debug ' \
          f'--raw-command ' \
          f'-- ' \
          f'{GO_BIN} test -v {test_path} {go_args} -args {test_args}'

    start_time = self.get_current_time()
    try:
        self.run_script(cmd,
                        ok_nonzero_returncodes=(1,),
                        extra_env_vars=extra_env_vars,
                        cwd=fp_ws)
        stop_time = self.get_current_time()
    finally:
        copy_test_logs_dir(test_logs_dir_in_ws, test_log_directory_path)

    if not Path(xunit_results_filepath).is_file():
        logger.warn('Test did not produce expected xunit result')

    log_filepath = Path(test_log_directory_path) / 'output_from_json.log'
    write_output_from_results_json(json_results_file, log_filepath)

    log_file = str(log_filepath) if log_filepath.exists() else self.console_output_file
    return None, xunit_results_filepath, log_file, start_time, stop_time

@register_testbed_file_generator('b4_fp')
@app.task(bind=True, returns=('testbed', 'tb_data', 'testbed_path'))
def GenerateB4FPTestbedFile(self,
                            topology,
                            sim_working_dir,
                            default_xr_username,
                            default_xr_password,
                            testbed_connection_info,
                            configure_unicon=False):

    c = GenerateGoB4TestbedFile.s(topology=topology, 
        sim_working_dir=sim_working_dir, 
        default_xr_username=default_xr_username,
        default_xr_password=default_xr_password,
        testbed_connection_info=testbed_connection_info,
        configure_unicon=configure_unicon)
    return self.enqueue_child_and_get_results(c, return_keys=('testbed', 'tb_data', 'testbed_path'))
