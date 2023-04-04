from firexapp.engine.celery import app
from celery.utils.log import get_task_logger
from microservices.workspace_tasks import Warn
from firexapp.common import silent_mkdir
from firexapp.firex_subprocess import check_output
from firexapp.submit.arguments import whitelist_arguments
from microservices.testbed_tasks import register_testbed_file_generator
from microservices.runners.go_b4_tasks import copy_test_logs_dir, write_output_from_results_json
from microservices.firex_base import returns, flame, InjectArgs, FireX
from services.cflow.code_coverage_tasks import CollectCoverageData
from microservices.runners.runner_base import FireXRunnerBase
from test_framework import register_test_framework_provider
from ci_plugins.vxsim import GenerateGoB4TestbedFile
from html_helper import get_link 
from helper import CommandFailed
from getpass import getuser
from pathlib import Path
import shutil
import random
import string
import tempfile
import time
import json
import yaml
import git
import os
import re

logger = get_task_logger(__name__)

GO_BIN = '/auto/firex/bin/go'
PYTHON_BIN = '/auto/firex/sw/python/3.9.10/bin/python3.9'
TBLOCK_BIN = '/auto/tftpboot-ottawa/b4/bin/tblock'
IXIA_RELEASE_BIN = '/auto/tftpboot-ottawa/b4/bin/ixia_release'

PUBLIC_FP_REPO_URL = 'https://github.com/openconfig/featureprofiles.git'
INTERNAL_FP_REPO_URL = 'git@wwwin-github.cisco.com:B4Test/featureprofiles.git'

TESTBEDS_FILE = 'exec/testbeds.yaml'

whitelist_arguments([
    'test_html_report',
    'release_ixia_ports',
])

class GoTestSegFaultException(Exception):
    pass

def _get_go_env():
    gorootpath = os.path.join('/nobackup', getuser())
    return {
        'GOPATH': os.path.join(gorootpath, 'go'),
        'GOCACHE': os.path.join(gorootpath, '.gocache'),
        'GOROOT': '/auto/firex/sw/go'
    }

def _resolve_path_if_needed(dir, path):
    if path[0] != '/':
        return os.path.join(dir, path)
    return path

def _gnmi_set_file_template(conf):
    return """
    replace: {
  path: {
    origin: "cli"
  }
  val: {
    ascii_val:
""" + '\n'.join(['"' + l + '\\n"' for l in conf if not l.strip().startswith('!')]) + """
  }
}
    """

def _sim_get_vrf(base_conf_file):
    intf_re = r'interface.*?MgmtEth0\/RP0\/CPU0/0(.|\n)*?(\bvrf(\b.*\b))(.|\n)*?!'
    with open(base_conf_file, 'r') as fp:
        matches = re.search(intf_re, fp.read())
        if matches and matches.group(3) is not None:
            return matches.group(3).strip()
    return None

def _sim_get_mgmt_ips(testbed_logs_dir):
    vxr_ports_file = os.path.join(testbed_logs_dir, "bringup_success", "sim-ports.yaml")
    with open(vxr_ports_file, "r") as fp:
        try:
            vxr_ports = yaml.safe_load(fp)
        except yaml.YAMLError:
            logger.warning("Failed to parse vxr ports file...")
            return
    mgmt_ips = {}
    for dut, entry in vxr_ports.items():
        if "xr_mgmt_ip" in entry:
            mgmt_ips[dut] = entry["xr_mgmt_ip"]
    return mgmt_ips

def _cli_to_gnmi_set_file(cli_file, gnmi_file, extra_conf=[]):
    with open(cli_file, 'r') as cli:
        lines = []
        for l in cli.read().splitlines():
            if l.strip() == 'end':
                lines.extend(extra_conf)
            lines.append(l)
        gnmi_set = _gnmi_set_file_template(lines)

    with open(gnmi_file, 'w') as gnmi:
        gnmi.write(gnmi_set)

def _check_json_output(cmd):
    return json.loads(check_output(cmd))

def _get_testbeds_file(internal_fp_repo_dir):
    return _resolve_path_if_needed(internal_fp_repo_dir, TESTBEDS_FILE)

def _get_locks_dir(testbed_logs_dir):
    return os.path.join(os.path.dirname(testbed_logs_dir), 'tblocks')

def _get_testbed_by_id(internal_fp_repo_dir, testbed_id):
    with open(_get_testbeds_file(internal_fp_repo_dir), 'r') as fp:
        tf = yaml.safe_load(fp)
        for t in tf['testbeds']:
            if t['id'] == testbed_id:
                return t
    raise Exception(f'Testbed ${testbed_id} not found')

def _trylock_testbed(internal_fp_repo_dir, testbed_id, testbed_logs_dir):
    try:
        output = _check_json_output(f'{TBLOCK_BIN} -d {_get_locks_dir(testbed_logs_dir)} -f {_get_testbeds_file(internal_fp_repo_dir)} -j lock {testbed_id}')
        if output['status'] == 'ok':
            return output['testbed']
        return None
    except:
        return None

def _release_testbed(internal_fp_repo_dir, testbed_id, testbed_logs_dir):
    logger.print(f'Releasing testbed {testbed_id}')
    try:
        output = _check_json_output(f'{TBLOCK_BIN} -d {_get_locks_dir(testbed_logs_dir)} -f {_get_testbeds_file(internal_fp_repo_dir)} -j release {testbed_id}')
        if output['status'] != 'ok':
            logger.warn(f'Cannot release testbed {testbed_id}: {output["status"]}')
        return True
    except:
        logger.warn(f'Cannot release testbed {testbed_id}')
        return False

@app.task(base=FireX, bind=True, soft_time_limit=12*60*60, time_limit=12*60*60)
@returns('internal_fp_repo_dir', 'reserved_testbed', 'ondatra_binding_path', 
		'ondatra_testbed_path', 'testbed_info_path', 
        'slurm_cluster_head', 'sim_working_dir', 'slurm_jobid', 'topo_path')
def BringupTestbed(self, ws, testbed_logs_dir, testbeds, images, test_name,
                        internal_fp_repo_url=INTERNAL_FP_REPO_URL,
                        internal_fp_repo_branch='master',
                        internal_fp_repo_rev=None,
                        collect_tb_info=False,
                        install_image=False,
                        ignore_install_errors=True):

    internal_pkgs_dir = os.path.join(ws, 'internal_go_pkgs')
    internal_fp_repo_dir = os.path.join(internal_pkgs_dir, 'openconfig', 'featureprofiles')

    if not os.path.exists(internal_fp_repo_dir):
        c = CloneRepo.s(repo_url=internal_fp_repo_url,
                    repo_branch=internal_fp_repo_branch,
                    repo_rev=internal_fp_repo_rev,
                    target_dir=internal_fp_repo_dir)

        self.enqueue_child_and_get_results(c)

    reserved_testbed = None
    if len(testbeds) == 1:
        reserved_testbed = _get_testbed_by_id(internal_fp_repo_dir, testbeds[0])

    c = InjectArgs(internal_fp_repo_dir=internal_fp_repo_dir, 
                reserved_testbed=reserved_testbed, **self.abog)

    using_sim = reserved_testbed and reserved_testbed.get('sim', False) 
    if using_sim:
        overrides = reserved_testbed.get('overrides', {}).get(test_name, {})
        reserved_testbed.update(overrides)

        topo_file = _resolve_path_if_needed(internal_fp_repo_dir, reserved_testbed['topology'])
        with open(topo_file, "r") as fp:
            topo_yaml = yaml.safe_load(fp)

        if not type(reserved_testbed['baseconf']) is dict:
            reserved_testbed['baseconf'] = {
                'dut': reserved_testbed['baseconf']
            }

        for dut, conf in reserved_testbed['baseconf'].items():
            baseconf_file = _resolve_path_if_needed(internal_fp_repo_dir, conf)
            baseconf_file_copy = os.path.join(testbed_logs_dir, f'baseconf_{dut}.conf')
            shutil.copyfile(baseconf_file, baseconf_file_copy)
            topo_yaml['devices'][dut]['cvac'] = baseconf_file_copy

        with open(topo_file, "w") as fp:
            fp.write(yaml.dump(topo_yaml))
        c |= self.orig.s(plat='8000', topo_file=topo_file)
    else:
        c |= ReserveTestbed.s()

    c |= GenerateOndatraTestbedFiles.s()
    if install_image and not using_sim:
        c |= SoftwareUpgrade.s(ignore_install_errors=ignore_install_errors)
    if collect_tb_info:
        c |= CollectTestbedInfo.s()
    result = self.enqueue_child_and_get_results(c)
    return (internal_fp_repo_dir, reserved_testbed, result["ondatra_binding_path"], 
            result["ondatra_testbed_path"], result["testbed_info_path"], 
            result.get("slurm_cluster_head", None), result.get("sim_working_dir", None),
            result.get("slurm_jobid", None), result.get("topo_path", None))

@app.task(base=FireX, bind=True)
def CleanupTestbed(self, ws, testbed_logs_dir, 
        internal_fp_repo_dir, reserved_testbed=None):
    logger.print('Cleaning up...')
    if reserved_testbed.get('sim', False):
        self.enqueue_child(
            self.orig.s(**self.abog),
            block=True
        )
    else:
        _release_testbed(internal_fp_repo_dir, reserved_testbed['id'], testbed_logs_dir)

def max_testbed_requests():
    if 'B4_FIREX_TESTBEDS_COUNT' in os.environ:
        return int(os.environ.get('B4_FIREX_TESTBEDS_COUNT'))
    return 10

def decommission_testbed_after_tests():
    if 'B4_FIREX_DECOMMISSION_TESTBED' in os.environ:
        return bool(int(os.environ.get('B4_FIREX_DECOMMISSION_TESTBED')))
    return False

@register_test_framework_provider('b4')
def b4_chain_provider(ws, testsuite_id, cflow,
                        internal_fp_repo_dir,
                        test_name,
                        test_path,
                        test_branch='main',
                        test_revision=None,
                        test_pr=None,
                        test_args=None,
                        test_timeout=0,
                        fp_pre_tests=[],
                        fp_post_tests=[],
                        internal_test=False,
                        test_debug=True,
                        test_verbose=True,
                        test_html_report=True,
                        release_ixia_ports=True,
                        testbed=None,
                        **kwargs):

    test_repo_dir = os.path.join(ws, 'go_pkgs', 'openconfig', 'featureprofiles')

    test_repo_url = PUBLIC_FP_REPO_URL
    if internal_test:
        test_repo_url = INTERNAL_FP_REPO_URL

    use_patched_repo = test_branch == 'main' and not test_revision and not test_pr
    if use_patched_repo:
        test_repo_url = INTERNAL_FP_REPO_URL
        test_branch = 'firex/run'

    chain = InjectArgs(ws=ws,
                    testsuite_id=testsuite_id,
                    internal_fp_repo_dir=internal_fp_repo_dir,
                    test_repo_dir=test_repo_dir,
                    test_name=test_name,
                    test_path=test_path,
                    test_args=test_args,
                    test_timeout=test_timeout,
                    test_debug=test_debug,
                    test_verbose=test_verbose,
                    **kwargs)

    chain |= CloneRepo.s(repo_url=test_repo_url,
                    repo_branch=test_branch,
                    repo_rev=test_revision,
                    repo_pr=test_pr,
                    target_dir=test_repo_dir)

    chain |= GoTidy.s(repo=test_repo_dir)

    if release_ixia_ports and '/ate_tests/' in test_path:
        chain |= ReleaseIxiaPorts.s()

    if fp_pre_tests:
        for pt in fp_pre_tests:
            for k, v in pt.items():
                chain |= RunGoTest.s(test_repo_dir=internal_fp_repo_dir, test_path = v['test_path'], test_args = v.get('test_args'))

    chain |= RunGoTest.s(test_repo_dir=test_repo_dir, test_path = test_path, test_args = test_args, test_timeout = test_timeout)

    if fp_post_tests:
        for pt in fp_post_tests:
            for k, v in pt.items():
                chain |= RunGoTest.s(test_repo_dir=internal_fp_repo_dir, test_path = v['test_path'], test_args = v.get('test_args'))

    if test_html_report:
        chain |= GoReporting.s()

    if cflow and testbed:
        chain |= CollectCoverageData.s(pyats_testbed=testbed)
    return chain

# noinspection PyPep8Naming
@app.task(bind=True, base=FireXRunnerBase, max_retries=2, autoretry_for=[GoTestSegFaultException])
@flame('log_file', lambda p: get_link(p, 'Test Output'))
@flame('test_log_directory_path', lambda p: get_link(p, 'All Logs'))
@returns('cflow_dat_dir', 'xunit_results', 'log_file', "start_time", "stop_time")
def RunGoTest(self, ws, testsuite_id, test_log_directory_path, xunit_results_filepath,
        test_repo_dir, internal_fp_repo_dir, ondatra_binding_path, ondatra_testbed_path, 
        test_path, test_args=None, test_timeout=0, test_debug=False, test_verbose=False, testbed_info_path=None):
    
    logger.print('Running Go test...')
    json_results_file = Path(test_log_directory_path) / f'go_logs.json'
    xml_results_file = Path(test_log_directory_path) / f'ondatra_logs.xml'
    test_logs_dir_in_ws = Path(ws) / f'{testsuite_id}_logs'

    check_output(f'rm -rf {test_logs_dir_in_ws}')
    silent_mkdir(test_logs_dir_in_ws)

    shutil.copyfile(ondatra_binding_path,
            os.path.join(test_log_directory_path, "ondatra_binding.txt"))
    shutil.copyfile(ondatra_testbed_path,
            os.path.join(test_log_directory_path, "ondatra_testbed.txt"))

    if os.path.exists(testbed_info_path):
        shutil.copyfile(testbed_info_path,
            os.path.join(test_log_directory_path, "testbed_info.txt"))
    
    go_args = ''
    test_args = test_args or ''

    test_args = f'{test_args} ' \
        f'-log_dir {test_logs_dir_in_ws}'

    test_args += f' -binding {ondatra_binding_path} -testbed {ondatra_testbed_path} '
    if test_verbose:
        test_args += f'-v 5 ' \
            f'-alsologtostderr'

    go_args = f'{go_args} ' \
                f'-json ' \
                f'-p 1 ' \
                f'-timeout {test_timeout}s'

    cmd = f'{GO_BIN} test -v ./{test_path} {go_args} -args {test_args} ' \
            f'-xml "{xml_results_file}"'

    start_time = self.get_current_time()
    start_timestamp = int(time.time())

    try:
        inactivity_timeout = 1800
        if test_timeout > 0: inactivity_timeout = 2*test_timeout

        self.run_script(cmd,
                        inactivity_timeout=inactivity_timeout,
                        ok_nonzero_returncodes=(1,),
                        extra_env_vars=_get_go_env(),
                        cwd=test_repo_dir)
        stop_time = self.get_current_time()
    finally:
        have_output = self.console_output_file and Path(self.console_output_file).is_file()
        have_xml_output = xml_results_file and Path(xml_results_file).is_file()

        if have_output:
            shutil.copyfile(self.console_output_file, json_results_file)

        if have_xml_output:
            shutil.copyfile(xml_results_file, xunit_results_filepath)
            if test_debug:
                with open(xml_results_file, 'r') as f:
                    if 'failures="0"' not in f.read():
                        self.enqueue_child(CollectDebugFiles.s(
                            internal_fp_repo_dir=internal_fp_repo_dir, 
                            ondatra_binding_path=ondatra_binding_path, 
                            ondatra_testbed_path=ondatra_testbed_path, 
                            test_log_directory_path=test_log_directory_path,
                            timestamp=start_timestamp
                        ))
        elif have_output:
            with open(json_results_file, 'r') as f:
                content = f.read() 
                if 'segmentation fault (core dumped)' in content:
                    raise GoTestSegFaultException

        copy_test_logs_dir(test_logs_dir_in_ws, test_log_directory_path)

        if not Path(xunit_results_filepath).is_file():
            logger.warn('Test did not produce expected xunit result')
        else: 
            check_output(f"sed -i 's|skipped|disabled|g' {xunit_results_filepath}")

        log_filepath = Path(test_log_directory_path) / 'output_from_json.log'
        write_output_from_results_json(json_results_file, log_filepath)
        log_file = str(log_filepath) if log_filepath.exists() else self.console_output_file
        return None, xunit_results_filepath, log_file, start_time, stop_time

@app.task(bind=True, max_retries=5, autoretry_for=[git.GitCommandError])
def CloneRepo(self, repo_url, repo_branch, target_dir, repo_rev=None, repo_pr=None):
    if Path(target_dir).exists():
        logger.warning(f'The target directory "{target_dir}" already exists; removing first')
        shutil.rmtree(target_dir)

    repo_name = repo_url.split("/")[-1].split(".")[0]
    logger.print(f'Cloning repo {repo_url} to {target_dir} branch {repo_branch}...')
    try:
        repo = git.Repo.clone_from(url=repo_url,
                                   to_path=target_dir,
                                   branch=repo_branch)
        if repo_rev:
            logger.print(f'Checking out revision {repo_rev} from {repo_url}...')
            repo.git.checkout(repo_rev)
        elif repo_pr:
            logger.print(f'Pulling PR#{repo_pr} from {repo_url}...')
            repo.remotes.origin.fetch(f'pull/{repo_pr}/head:pr_{repo_pr}')
            logger.print(f'Checking out branch pr_{repo_pr}...')
            repo.git.checkout(f'pr_{repo_pr}')

    except git.GitCommandError as e:
        err = e.stderr or ''
        if 'Permission denied (publickey).' in err.splitlines():
            err_msg = f'It appears you do not have proper access to the {repo_name} repository. Check your access ' \
                      f'permissions and make sure your ssh keys are added to your user profile here:\n' \
                      f'https://wwwin-github.cisco.com/settings/keys'
            self.enqueue_child(Warn.s(err_msg=err_msg), block=True, raise_exception_on_failure=False)
        raise e

    head_commit_sha = repo.head.commit.hexsha
    logger.info(f'Head Commit Sha: {head_commit_sha}')
    short_sha = repo.git.rev_parse(head_commit_sha, short=7)
    self.send_flame_html(version=f'{repo_name}: {short_sha}')

@app.task(base=FireX, bind=True, returns=('ondatra_testbed_path', 'ondatra_binding_path', 'testbed_info_path', 'install_lock_file'))
def GenerateOndatraTestbedFiles(self, ws, testbed_logs_dir, internal_fp_repo_dir, reserved_testbed, test_name, **kwargs):
    logger.print('Generating Ondatra files...')
    ondatra_files_suffix = ''.join(random.choice(string.ascii_letters) for _ in range(8))
    ondatra_testbed_path = os.path.join(ws, f'ondatra_{ondatra_files_suffix}.testbed')
    ondatra_binding_path = os.path.join(ws, f'ondatra_{ondatra_files_suffix}.binding')
    testbed_info_path = os.path.join(testbed_logs_dir, f'testbed_{ondatra_files_suffix}_info.txt')
    install_lock_file = os.path.join(testbed_logs_dir, f'testbed_{ondatra_files_suffix}_install.lock')

    if reserved_testbed.get('sim', False):
        vxr_testbed = kwargs['testbed_path']
        check_output(f'/auto/firex/sw/pyvxr_binding/pyvxr_binding.sh staticbind service {vxr_testbed}', 
            file=ondatra_binding_path)
        check_output(f'/auto/firex/sw/pyvxr_binding/pyvxr_binding.sh statictestbed service {vxr_testbed}', 
            file=ondatra_testbed_path)

        mgmt_ips = _sim_get_mgmt_ips(testbed_logs_dir)
        if not type(reserved_testbed['baseconf']) is dict:
            reserved_testbed['baseconf'] = {
                'dut': reserved_testbed['baseconf']
            }

        for dut, conf in reserved_testbed['baseconf'].items():
            baseconf_file_path = _resolve_path_if_needed(internal_fp_repo_dir, conf)
            ondatra_baseconf_path = os.path.join(ws, f'ondatra_{ondatra_files_suffix}_{dut}.conf')
            shutil.copyfile(baseconf_file_path, ondatra_baseconf_path)
            extra_conf = []

            mgmt_ip = mgmt_ips[dut]
            logger.info(f"Found management ip: {mgmt_ip} for dut '{dut}'")
            
            mgmt_vrf = _sim_get_vrf(ondatra_baseconf_path)
            if mgmt_vrf:
                extra_conf.append(f'ipv4 virtual address vrf {mgmt_vrf} {mgmt_ip}/24')
            else:
                extra_conf.append(f'ipv4 virtual address {mgmt_ip}/24')

            _cli_to_gnmi_set_file(ondatra_baseconf_path, ondatra_baseconf_path, extra_conf)
            check_output("sed -i 's|id: \"" + dut + "\"|id: \"" + dut + "\"\\nconfig:{\\ngnmi_set_file:\"" + ondatra_baseconf_path + "\"\\n  }|g' " + ondatra_binding_path)
    else:
        ondatra_baseconf_path = os.path.join(ws, f'ondatra_{ondatra_files_suffix}.conf')
        testbed_info_path = os.path.join(os.path.dirname(testbed_logs_dir), 
            f'testbed_{reserved_testbed["id"]}_info.txt')
        install_lock_file = os.path.join(os.path.dirname(testbed_logs_dir), 
            f'testbed_{reserved_testbed["id"]}_install.lock')

        overrides = reserved_testbed.get('overrides', {}).get(test_name, {})
        reserved_testbed.update(overrides)

        hw_testbed_file_path = _resolve_path_if_needed(internal_fp_repo_dir, reserved_testbed['testbed'])
        hw_binding_file_path = _resolve_path_if_needed(internal_fp_repo_dir, reserved_testbed['binding'])
        hw_baseconf_file_path = _resolve_path_if_needed(internal_fp_repo_dir, reserved_testbed['baseconf'])
        
        shutil.copyfile(hw_testbed_file_path, ondatra_testbed_path)
        shutil.copyfile(hw_binding_file_path, ondatra_binding_path)
        shutil.copyfile(hw_baseconf_file_path, ondatra_baseconf_path)
        check_output(f"sed -i 's|$BASE_CONF_PATH|{ondatra_baseconf_path}|g' {ondatra_binding_path}")

    logger.print(f'Ondatra testbed file: {ondatra_testbed_path}')
    logger.print(f'Ondatra binding file: {ondatra_binding_path}')
    return ondatra_testbed_path, ondatra_binding_path, testbed_info_path, install_lock_file

@app.task(base=FireX, bind=True, returns=('reserved_testbed'), 
    soft_time_limit=12*60*60, time_limit=12*60*60)
def ReserveTestbed(self, testbed_logs_dir, internal_fp_repo_dir, testbeds):
    logger.print('Reserving testbed...')
    reserved_testbed = None
    while not reserved_testbed:
        for t in testbeds:
            reserved_testbed = _trylock_testbed(internal_fp_repo_dir, t, testbed_logs_dir)
            if reserved_testbed: break
        time.sleep(1)
    logger.print(f'Reserved testbed {reserved_testbed["id"]}')
    return reserved_testbed

# noinspection PyPep8Naming
@app.task(bind=True, max_retries=2, autoretry_for=[CommandFailed], soft_time_limit=1*60*60, time_limit=1*60*60)
def SoftwareUpgrade(self, ws, internal_fp_repo_dir, testbed_logs_dir, 
                    reserved_testbed, ondatra_binding_path, ondatra_testbed_path, 
                    install_lock_file, images, ignore_install_errors=True):
    if os.path.exists(install_lock_file):
        return
    Path(install_lock_file).touch()

    logger.print("Performing Software Upgrade...")
    su_command = f'{GO_BIN} test -v ' \
            f'./exec/utils/software_upgrade ' \
            f'-timeout 0 ' \
            f'-args ' \
            f'-testbed {ondatra_testbed_path} ' \
            f'-binding {ondatra_binding_path} ' \
            f'-imagePath "{images[0]}"'
    try:
        env = dict(os.environ)
        env.update(_get_go_env())
        check_output(su_command, env=env, cwd=internal_fp_repo_dir)
    except:
        if not ignore_install_errors:
            _release_testbed(internal_fp_repo_dir, reserved_testbed['id'], testbed_logs_dir)
            os.remove(install_lock_file)
            raise
        else: logger.warning(f'Software upgrade failed. Ignoring...')

# noinspection PyPep8Naming
@app.task(bind=True)
def CheckoutRepo(self, repo, repo_branch=None, repo_rev=None):
    r = git.Repo(repo)
    r.git.reset('--hard')
    r.git.clean('-xdf')
    if repo_rev:
        r.git.checkout(repo_rev)
    elif repo_branch:
        r.git.checkout(repo_branch)
    r.git.reset('--hard')
    r.git.clean('-xdf')

# noinspection PyPep8Naming
@app.task(bind=True, soft_time_limit=1*60*60, time_limit=1*60*60)
def CollectDebugFiles(self, internal_fp_repo_dir, ondatra_binding_path, 
        ondatra_testbed_path, test_log_directory_path, timestamp):
    logger.print("Collecting debug files...")

    with tempfile.NamedTemporaryFile(delete=False) as f:
        tmp_binding_file = f.name
        shutil.copyfile(ondatra_binding_path, tmp_binding_file)
        check_output(f"sed -i 's|gnmi_set_file|#gnmi_set_file|g' {tmp_binding_file}")

    collect_debug_cmd = f'{GO_BIN} test -v ' \
            f'./exec/utils/debug ' \
            f'-timeout 0 ' \
            f'-args ' \
            f'-testbed {ondatra_testbed_path} ' \
            f'-binding {tmp_binding_file} ' \
            f'-outDir {test_log_directory_path}/debug_files ' \
            f'-timestamp {str(timestamp)}'
    try:
        env = dict(os.environ)
        env.update(_get_go_env())
        check_output(collect_debug_cmd, env=env, cwd=internal_fp_repo_dir)
    except:
        logger.warning(f'Failed to collect testbed information. Ignoring...') 
    finally:
        os.remove(tmp_binding_file)

# noinspection PyPep8Naming
@app.task(bind=True)
def CollectTestbedInfo(self, ws, internal_fp_repo_dir, ondatra_binding_path, 
        ondatra_testbed_path, testbed_info_path):
    if os.path.exists(testbed_info_path):
        return

    logger.print("Collecting testbed info...")
    testbed_info_cmd = f'{GO_BIN} test -v ' \
            f'./exec/utils/testbed ' \
            f'-timeout 0 ' \
            f'-args ' \
            f'-testbed {ondatra_testbed_path} ' \
            f'-binding {ondatra_binding_path} ' \
            f'-outFile {testbed_info_path}'
    try:
        env = dict(os.environ)
        env.update(_get_go_env())
        check_output(testbed_info_cmd, env=env, cwd=internal_fp_repo_dir)
        logger.print(f'Testbed info file: {testbed_info_path}')
    except:
        logger.warning(f'Failed to collect testbed information. Ignoring...')

# noinspection PyPep8Naming
@app.task(bind=True)
def GoTidy(self, repo):
    env = dict(os.environ)
    env.update(_get_go_env())
    logger.print(
        check_output(f'{GO_BIN} mod tidy', env=env, cwd=repo)
    )

# noinspection PyPep8Naming
@app.task(bind=True)
def ReleaseIxiaPorts(self, ws, ondatra_binding_path):
    logger.print("Releasing ixia ports...")
    try:
        logger.print(
            check_output(f'{IXIA_RELEASE_BIN} {ondatra_binding_path}')
        )
    except:
        logger.warning(f'Failed to release ixia ports. Ignoring...')

@app.task(bind=True)
def GoReporting(self, internal_fp_repo_dir, test_log_directory_path):
    logger.print("Generating HTML report...")
    json_log_file = os.path.join(test_log_directory_path, f'go_logs.json')
    html_report = os.path.join(test_log_directory_path, f'results.html')
    try:
        check_output(f'{PYTHON_BIN} {internal_fp_repo_dir}/exec/utils/reporting/gotest2html.py "{json_log_file}"', 
            file=html_report) 
    except:
        logger.warning(f'Failed to generate HTML report. Ignoring...')

@register_testbed_file_generator('b4')
@app.task(bind=True, returns=('testbed', 'tb_data', 'testbed_path'))
def GenerateSimTestbedFile(self,
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
