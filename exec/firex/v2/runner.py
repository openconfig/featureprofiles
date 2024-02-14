from firexapp.engine.celery import app
from celery.utils.log import get_task_logger
from celery.utils.log import get_task_logger
from microservices.workspace_tasks import Warn
from firexapp.common import silent_mkdir
from firexapp.firex_subprocess import check_output
from firexkit.task import FireXTask
from firexapp.submit.arguments import whitelist_arguments
from microservices.testbed_tasks import register_testbed_file_generator
from microservices.runners.go_b4_tasks import copy_test_logs_dir, write_output_from_results_json
from microservices.firex_base import returns, flame, InjectArgs, FireX
from services.cflow.code_coverage_tasks import CollectCoverageData
from microservices.runners.runner_base import FireXRunnerBase
from test_framework import register_test_framework_provider
from ci_plugins.vxsim import GenerateGoB4TestbedFile
from html_helper import get_link 
from helper import CommandFailed, remote_exec
from getpass import getuser
import xml.etree.ElementTree as ET
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

MTLS_DEFAULT_TRUST_BUNDLE_FILE = 'internal/cisco/security/cert/keys/CA/ca.cert.pem'
MTLS_DEFAULT_CERT_FILE = 'internal/cisco/security/cert/keys/clients/cafyauto.cert.pem'
MTLS_DEFAULT_KEY_FILE = 'internal/cisco/security/cert/keys/clients/cafyauto.key.pem'

whitelist_arguments([
    'test_html_report',
    'release_ixia_ports',
    'test_ignore_aborted',
    'test_skip',
    'test_fail_skipped',
    'test_show_skipped',
    'test_repo_url',
    'sim_use_mtls'
])

def _get_go_root_path(ws=None):
    p = os.path.join('/nobackup', getuser())
    if os.access(p, os.W_OK | os.X_OK):
        return p
    return ws
        

def _get_go_path():
    return os.path.join(_get_go_root_path(), 'go')

def _get_go_bin_path():
    return os.path.join(_get_go_path(), 'bin')

def _get_go_env(ws=None):
    PATH = "{}:{}".format(
        os.path.dirname(GO_BIN), os.environ["PATH"]
    )

    gorootpath = _get_go_root_path(ws)
    gocache = os.path.join(gorootpath, '.gocache')
    os.makedirs(gocache, exist_ok=True)

    return {
        'GOPATH': _get_go_path(),
        'GOCACHE': gocache,
        'GOTMPDIR': gocache,
        'GOROOT': '/auto/firex/sw/go',
        'PATH': PATH
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

def _otg_docker_compose_template(control_port, gnmi_port):
    return f"""
version: "2"
services:
  controller:
    image: ghcr.io/open-traffic-generator/keng-controller:firex
    restart: always
    ports:
      - "{control_port}:40051"
    depends_on:
      layer23-hw-server:
        condition: service_started
    command:
      - "--accept-eula"
      - "--debug"
      - "--keng-layer23-hw-server"
      - "layer23-hw-server:5001"
    environment:
      - LICENSE_SERVERS=10.85.70.247
    logging:
      driver: "local"
      options:
        max-size: "100m"
        max-file: "10"
        mode: "non-blocking"
  layer23-hw-server:
    image: ghcr.io/open-traffic-generator/keng-layer23-hw-server:firex
    restart: always
    command:
      - "dotnet"
      - "otg-ixhw.dll"
      - "--trace"
      - "--log-level"
      - "trace"
    logging:
      driver: "local"
      options:
        max-size: "100m"
        max-file: "10"
        mode: "non-blocking"
  gnmi-server:
    image: ghcr.io/open-traffic-generator/otg-gnmi-server:firex
    restart: always
    ports:
      - "{gnmi_port}:50051"
    depends_on:
      controller:
        condition: service_started
    command:
      - "-http-server"
      - "https://controller:8443"
      - "--debug"
    logging:
      driver: "local"
      options:
        max-size: "100m"
        max-file: "10"
        mode: "non-blocking"
"""

def _write_otg_docker_compose_file(docker_file, reserved_testbed):
    if not 'otg' in reserved_testbed:
        return
    otg_info = reserved_testbed['otg']
    with open(docker_file, 'w') as fp:
        fp.write(_otg_docker_compose_template(otg_info['controller_port'], otg_info['gnmi_port']))

# def _get_mtls_binding_option(internal_fp_repo_dir, testbed):
#     tb_file = MTLS_DEFAULT_TRUST_BUNDLE_FILE
#     key_file = MTLS_DEFAULT_KEY_FILE
#     cert_file = MTLS_DEFAULT_CERT_FILE

#     if 'trust_bundle_file' in testbed:
#         tb_file = _resolve_path_if_needed(internal_fp_repo_dir, testbed['trust_bundle_file'])
#     if 'cert_file' in testbed:
#         cert_file = _resolve_path_if_needed(internal_fp_repo_dir, testbed['cert_file'])
#     if 'key_file' in testbed:
#         key_file = _resolve_path_if_needed(internal_fp_repo_dir, testbed['key_file'])
        
#     return f"""
#     options {{
#         insecure: false
#         skip_verify: false
#         trust_bundle_file: "{tb_file}"
#         cert_file: "{cert_file}"
#         key_file:  "{key_file}"
#     }}
# """

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

def _cli_to_gnmi_set_file(cli_lines, gnmi_file, extra_conf=[]):
    gnmi_set = _gnmi_set_file_template(cli_lines)
    with open(gnmi_file, 'w') as gnmi:
        gnmi.write(gnmi_set)

    with open(gnmi_file, 'w') as gnmi:
        gnmi.write(gnmi_set)

def _get_dummy_suite_xml(test_name, fail):
    if fail: 
        failures = 1
        body = '<failure message="Failed"></failure>'
    else:
        failures = 0
        body = "<system-out></system-out>"

    return f"""<?xml version='1.0' encoding='UTF-8'?>
<testsuites>
    <!-- This dummy xunit result has been automatically generated since the test was aborted. -->
    <!-- Check test logs for error details. -->
    <testsuite name="{test_name}" tests="1" failures="{failures}" errors="0" skipped="0">
        <testcase name="dummy">
            {body}
        </testcase>
    </testsuite>
</testsuites>
    """

def _write_dummy_xml_output(test_name, xml_file, fail):
    with open(xml_file, 'w') as fp:
        fp.write(_get_dummy_suite_xml(test_name, fail))

def _get_testsuite_from_xml(file_name):
    try:
        tree = ET.parse(file_name)
        for suite in tree.findall("testsuite"):
            return suite
        return None
    except:
        return None

def _extract_env_var_from_arg(arg):
    m = re.findall('\$[0-9a-zA-Z_]+', arg)
    if len(m) > 0: return m[0]
    return None

def _update_arg_val_from_env(arg, extra_env_vars):
    env_var_name = _extract_env_var_from_arg(arg)
    if not env_var_name: return arg
    actual_env_var_name = env_var_name[1:] # remove leading $
    
    if actual_env_var_name in extra_env_vars:
        val = extra_env_vars[actual_env_var_name]
    else:
        val = os.getenv(actual_env_var_name)

    if val: arg = arg.replace(env_var_name, val)
    return arg

def _update_test_args_from_env(test_args, extra_env_vars={}):
    new_args = []
    for arg in test_args.split(' '):
        arg = _update_arg_val_from_env(arg, extra_env_vars)
        new_args.append(arg)
    return ' '.join(new_args)
        
def _check_json_output(cmd):
    return json.loads(check_output(cmd))

def _get_testbeds_file(internal_fp_repo_dir):
    return _resolve_path_if_needed(internal_fp_repo_dir, TESTBEDS_FILE)

def _get_locks_dir(testbed_logs_dir):
    return os.path.join(os.path.dirname(testbed_logs_dir), 'tblocks')

def _get_testbed_by_id(internal_fp_repo_dir, testbed_id):
    with open(_get_testbeds_file(internal_fp_repo_dir), 'r') as fp:
        tf = yaml.safe_load(fp)
        if testbed_id in tf['testbeds']:
            tb = tf['testbeds'][testbed_id]
            tb['id'] = testbed_id
            return tb
    raise Exception(f'Testbed {testbed_id} not found')

def _trylock_testbed(internal_fp_repo_dir, testbed_id, testbed_logs_dir):
    try:
        testbed = _get_testbed_by_id(internal_fp_repo_dir, testbed_id)
        id = testbed.get('hw', testbed_id)
        output = _check_json_output(f'{TBLOCK_BIN} -d {_get_locks_dir(testbed_logs_dir)} -f {_get_testbeds_file(internal_fp_repo_dir)} -j lock {id}')
        if output['status'] == 'ok':
            return output['testbed']
        return None
    except:
        return None

def _release_testbed(internal_fp_repo_dir, testbed_id, testbed_logs_dir):
    testbed = _get_testbed_by_id(internal_fp_repo_dir, testbed_id)
    id = testbed.get('hw', testbed_id)
    logger.print(f'Releasing testbed {id}')
    try:
        output = _check_json_output(f'{TBLOCK_BIN} -d {_get_locks_dir(testbed_logs_dir)} -f {_get_testbeds_file(internal_fp_repo_dir)} -j release {id}')
        if output['status'] != 'ok':
            logger.warn(f'Cannot release testbed {id}: {output["status"]}')
        return True
    except:
        logger.warn(f'Cannot release testbed {id}')
        return False

@app.task(base=FireX, bind=True, soft_time_limit=12*60*60, time_limit=12*60*60)
@returns('internal_fp_repo_url', 'internal_fp_repo_dir', 'reserved_testbed', 
        'slurm_cluster_head', 'sim_working_dir', 'slurm_jobid', 'topo_path', 'testbed')
def BringupTestbed(self, ws, testbed_logs_dir, testbeds, images, 
                        lineup, efr, test_name,
                        internal_fp_repo_url=INTERNAL_FP_REPO_URL,
                        internal_fp_repo_branch='master',
                        internal_fp_repo_rev=None,
                        collect_tb_info=False,
                        install_image=False,
                        force_install=False,
                        force_reboot=False,
                        sim_use_mtls=False,
                        smus=None):
    
    internal_pkgs_dir = os.path.join(ws, 'internal_go_pkgs')
    internal_fp_repo_dir = os.path.join(internal_pkgs_dir, 'openconfig', 'featureprofiles')

    if not os.path.exists(internal_fp_repo_dir):
        c = CloneRepo.s(repo_url=internal_fp_repo_url,
                    repo_branch=internal_fp_repo_branch,
                    repo_rev=internal_fp_repo_rev,
                    target_dir=internal_fp_repo_dir)

        self.enqueue_child_and_get_results(c)

    reserved_testbed = None
    if isinstance(testbeds, str):
        reserved_testbed = _get_testbed_by_id(internal_fp_repo_dir, testbeds)
    elif len(testbeds) == 1:
        reserved_testbed = _get_testbed_by_id(internal_fp_repo_dir, testbeds[0])

    c = InjectArgs(internal_fp_repo_dir=internal_fp_repo_dir, 
                reserved_testbed=reserved_testbed, **self.abog)

    using_sim = reserved_testbed and reserved_testbed.get('sim', False) 
    if using_sim:
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
    elif not reserved_testbed:
        c |= ReserveTestbed.s()

    c |= GenerateOndatraTestbedFiles.s()
    if using_sim and sim_use_mtls:
        c |= GenerateCertificates.s()
        c |= SimEnableMTLS.s()
        
    if install_image and not using_sim:
        c |= SoftwareUpgrade.s(force_install=force_install)
        force_reboot = False
    if smus:
        c |= InstallSMUs.s(smus=smus)
    if force_reboot:
        c |= ForceReboot.s()
    if collect_tb_info:
        c |= CollectTestbedInfo.s()
    result = self.enqueue_child_and_get_results(c)
    return (internal_fp_repo_url, internal_fp_repo_dir, result.get("reserved_testbed"),
            result.get("slurm_cluster_head", None), result.get("sim_working_dir", None),
            result.get("slurm_jobid", None), result.get("topo_path", None), result.get("testbed", None))

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
    return int(os.getenv("B4_FIREX_TESTBEDS_COUNT", '1'))

def decommission_testbed_after_tests():
    return os.getenv("B4_FIREX_DECOMMISSION_TESTBED", '0') == '1'

@register_test_framework_provider('b4')
def b4_chain_provider(ws, testsuite_id, cflow,
                        internal_fp_repo_url,
                        internal_fp_repo_dir,
                        reserved_testbed,
                        test_name,
                        test_path,
                        test_repo_url=PUBLIC_FP_REPO_URL,
                        test_branch='main',
                        test_revision=None,
                        test_pr=None,
                        test_args=None,
                        test_timeout=0,
                        fp_pre_tests=[],
                        fp_post_tests=[],
                        internal_test=False,
                        test_debug=False,
                        test_verbose=True,
                        test_html_report=False,
                        release_ixia_ports=True,
                        collect_debug_files=True,
                        override_test_args_from_env=True,
                        testbed=None,
                        **kwargs):
    
    test_repo_dir = os.path.join(ws, 'go_pkgs', 'openconfig', 'featureprofiles')
    if internal_test:
        test_repo_url = internal_fp_repo_url

    chain = InjectArgs(ws=ws,
                    testsuite_id=testsuite_id,
                    internal_fp_repo_dir=internal_fp_repo_dir,
                    reserved_testbed=reserved_testbed,
                    test_repo_dir=test_repo_dir,
                    test_name=test_name,
                    test_path=test_path,
                    test_args=test_args,
                    test_timeout=test_timeout,
                    test_debug=test_debug,
                    test_verbose=test_verbose,
                    collect_debug_files=collect_debug_files,
                    override_test_args_from_env=override_test_args_from_env,
                    **kwargs)

    chain |= CloneRepo.s(repo_url=test_repo_url,
                    repo_branch=test_branch,
                    repo_rev=test_revision,
                    repo_pr=test_pr,
                    target_dir=test_repo_dir)

    #chain |= GoTidy.s(repo=test_repo_dir)

    if test_debug:
        chain |= InstallGoDelve.s()

    if release_ixia_ports:
        chain |= ReleaseIxiaPorts.s(binding_file=reserved_testbed['ate_binding_file'])

    reserved_testbed['binding_file'] = reserved_testbed['ate_binding_file']
    if 'otg' in test_path and not reserved_testbed.get('sim', False) :
        reserved_testbed['binding_file'] = reserved_testbed['otg_binding_file']
        chain |= BringupIxiaController.s()

    if fp_pre_tests:
        for pt in fp_pre_tests:
            for k, v in pt.items():
                chain |= RunGoTest.s(test_repo_dir=internal_fp_repo_dir, test_path = v['test_path'], test_args = v.get('test_args'))

    chain |= RunGoTest.s(test_repo_dir=test_repo_dir, test_path = test_path, test_args = test_args, test_timeout = test_timeout)

    if fp_post_tests:
        for pt in fp_post_tests:
            for k, v in pt.items():
                chain |= RunGoTest.s(test_repo_dir=internal_fp_repo_dir, test_path = v['test_path'], test_args = v.get('test_args'))

    if 'otg' in test_path and not reserved_testbed.get('sim', False):
        chain |= TeardownIxiaController.s()

    if cflow and testbed:
        chain |= CollectCoverageData.s(pyats_testbed=_resolve_path_if_needed(internal_fp_repo_dir, testbed))
    return chain

# noinspection PyPep8Naming
@app.task(bind=True, base=FireXRunnerBase)
@flame('log_file', lambda p: get_link(p, 'Test Output'))
@flame('test_log_directory_path', lambda p: get_link(p, 'All Logs'))
@returns('cflow_dat_dir', 'xunit_results', 'log_file', "start_time", "stop_time")
def RunGoTest(self: FireXTask, ws, testsuite_id, test_log_directory_path, xunit_results_filepath,
        test_repo_dir, internal_fp_repo_dir, reserved_testbed, 
        test_name, test_path, test_args=None, test_timeout=0, collect_debug_files=False, 
        override_test_args_from_env=True, test_debug=False, test_verbose=False, testbed_info_path=None,
        test_ignore_aborted=False, test_skip=False, test_fail_skipped=False, test_show_skipped=False):

    logger.print('Running Go test...')
    logger.print('----- env start ----')
    for name, value in os.environ.items():
        logger.print("{0}: {1}".format(name, value))
    logger.print('----- env end ----')
    
    # json_results_file = Path(test_log_directory_path) / f'go_logs.json'
    xml_results_file = Path(test_log_directory_path) / f'ondatra_logs.xml'
    test_logs_dir_in_ws = Path(ws) / f'{testsuite_id}_logs'

    check_output(f'rm -rf {test_logs_dir_in_ws}')
    silent_mkdir(test_logs_dir_in_ws)

    shutil.copyfile(reserved_testbed['binding_file'],
            os.path.join(test_log_directory_path, "ondatra_binding.txt"))
    shutil.copyfile(reserved_testbed['testbed_file'],
            os.path.join(test_log_directory_path, "ondatra_testbed.txt"))

    if os.path.exists(reserved_testbed['testbed_info_file']):
        shutil.copyfile(reserved_testbed['testbed_info_file'],
            os.path.join(test_log_directory_path, "testbed_info.txt"))
        
    go_args = ''
    test_args = test_args or ''

    if override_test_args_from_env:
        extra_args_env_vars = {}
        if 'mtls_cert_file' in reserved_testbed:
            extra_args_env_vars["MTLS_CERT_FILE"] = reserved_testbed['mtls_cert_file']
        if 'mtls_key_file' in reserved_testbed:
            extra_args_env_vars["MTLS_KEY_FILE"] = reserved_testbed['mtls_key_file']      
        test_args = _update_test_args_from_env(test_args, extra_args_env_vars)

    test_args = f'{test_args} ' \
        f'-log_dir {test_logs_dir_in_ws}'

    test_args += f' -binding {reserved_testbed["binding_file"]} -testbed {reserved_testbed["testbed_file"]} -xml "{xml_results_file}" '
    if test_verbose:
        test_args += f'-v 5 ' \
            f'-alsologtostderr'

    inactivity_timeout = 1800
    if test_timeout == 0: test_timeout = inactivity_timeout
    if test_timeout > 0: inactivity_timeout = 2*test_timeout

    go_args_prefix = '-'
    if test_debug:
        go_args_prefix = '-test.'

    go_args = f'{go_args} ' \
                f'{go_args_prefix}v ' \
                f'{go_args_prefix}timeout {test_timeout}s'

    if test_debug:
        dlv_bin = os.path.join(_get_go_bin_path(), 'dlv')
        cmd = f'{dlv_bin} test ./{test_path} -- {go_args} {test_args}'
    else:
        cmd = f'{GO_BIN} test -p 1 ./{test_path} {go_args} -args {test_args}'

    start_time = self.get_current_time()
    start_timestamp = int(time.time())

    try:
        if not test_skip:
            self.run_script(cmd,
                            inactivity_timeout=inactivity_timeout,
                            ok_nonzero_returncodes=(1,),
                            extra_env_vars=_get_go_env(ws),
                            cwd=test_repo_dir)
        stop_time = self.get_current_time()
    finally:
        # if self.console_output_file and Path(self.console_output_file).is_file():
        #     shutil.copyfile(self.console_output_file, json_results_file)
        logger.debug("After running script")
        suite = _get_testsuite_from_xml(xml_results_file)
        logger.info(f"suite: {suite}")
        if suite: 
            shutil.copyfile(xml_results_file, xunit_results_filepath)
            logger.print(f" xml_results_file passing to CollectDebugFiles: {xml_results_file}, xunit_results_filepath: {xunit_results_filepath}, collect_debug_files [{collect_debug_files}], suite.attrib['failures'] = [{suite.attrib['failures']}]")
            if collect_debug_files and suite.attrib['failures'] != '0':
                logger.info(f"there were no failures detected suite.attrib['failures'] = [{suite.attrib['failures']}]")
                res = self.enqueue_child_and_get_results(CollectDebugFiles.s(
                    ws=ws,
                    internal_fp_repo_dir=internal_fp_repo_dir, 
                    reserved_testbed=reserved_testbed, 
                    test_log_directory_path=test_log_directory_path,
                    timestamp=start_timestamp,
                    core=False,
                    xunit_results_filepath=xunit_results_filepath
                ))
                logger.info(res)
            else:
                logger.info(f"there were failures detected suite.attrib['failures'] = [{suite.attrib['failures']}]")
                res = self.enqueue_child_and_get_results(CollectDebugFiles.s(
                    ws=ws,
                    internal_fp_repo_dir=internal_fp_repo_dir, 
                    reserved_testbed=reserved_testbed, 
                    test_log_directory_path=test_log_directory_path,
                    timestamp=start_timestamp,
                    core=True,
                    xunit_results_filepath=xunit_results_filepath
                ))
                logger.info(res)

        elif test_ignore_aborted or test_skip:
            logger.debug("elif in ")
            _write_dummy_xml_output(test_name, xunit_results_filepath, test_skip and test_fail_skipped)
        copy_test_logs_dir(test_logs_dir_in_ws, test_log_directory_path)
        logger.info(f"xunit_results_filepath {xunit_results_filepath}")
       
        if not Path(xunit_results_filepath).is_file():
            logger.warn('Test did not produce expected xunit result')
        elif not test_show_skipped: 
            check_output(f"sed -i 's|skipped|disabled|g' {xunit_results_filepath}")
        return None, xunit_results_filepath, self.console_output_file, start_time, stop_time

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

def _write_otg_binding(ws, internal_fp_repo_dir, reserved_testbed):
    if 'otg' not in reserved_testbed:
        shutil.copyfile(reserved_testbed["ate_binding_file"], reserved_testbed["otg_binding_file"])
        return

    otg_info = reserved_testbed['otg']

    # convert binding to json
    with tempfile.NamedTemporaryFile() as of:
        outFile = of.name
        cmd = f'{GO_BIN} run ' \
            f'./exec/utils/binding/tojson ' \
            f'-binding {reserved_testbed["ate_binding_file"]} ' \
            f'-out {outFile}'

        env = dict(os.environ)
        env.update(_get_go_env(ws))
        
        check_output(cmd, env=env, cwd=internal_fp_repo_dir)
        with open(outFile, 'r') as fp:
            j = json.load(fp)

    #TODO: support multiple ates
    for ate in j.get('ates', []):
        for p in ate.get('ports', []):
            parts = p['name'].split('/')
            p['name'] = '{chassis};{card};{port}'.format(chassis=ate['name'], card=parts[0], port=parts[1]) 

        ate['name'] = '{host}:{controller_port}'.format(host=otg_info['host'], controller_port=otg_info['controller_port'])
        ate['options'] = {
            'username': 'admin',
            'password': 'admin'
        }

        ate['otg'] = {
            'target': '{host}:{controller_port}'.format(host=otg_info['host'], controller_port=otg_info['controller_port']),
            'insecure': True,
            'timeout': 200
        }

        ate['gnmi'] = {
            'target': '{host}:{gnmi_port}'.format(host=otg_info['host'], gnmi_port=otg_info['gnmi_port']),
            'skip_verify': True,
            'timeout': 60
        }

        if 'ixnetwork' in ate:
            del ate['ixnetwork']

        break

    # convert binding to prototext
    with tempfile.NamedTemporaryFile() as f:
        tmp_binding_file = f.name
        with open(tmp_binding_file, "w") as outfile:
            outfile.write(json.dumps(j))
            
        cmd = f'{GO_BIN} run ' \
            f'./exec/utils/binding/fromjson ' \
            f'-binding {tmp_binding_file} ' \
            f'-out {reserved_testbed["otg_binding_file"]}'

        check_output(cmd, env=env, cwd=internal_fp_repo_dir)

@app.task(base=FireX, bind=True, returns=('reserved_testbed'))
def GenerateOndatraTestbedFiles(self, ws, testbed_logs_dir, internal_fp_repo_dir, reserved_testbed, test_name, **kwargs):
    logger.print('Generating Ondatra files...')
    ondatra_files_suffix = ''.join(random.choice(string.ascii_letters) for _ in range(8))
    ondatra_testbed_path = os.path.join(ws, f'ondatra_{ondatra_files_suffix}.testbed')
    ondatra_binding_path = os.path.join(ws, f'ondatra_{ondatra_files_suffix}.binding')
    ondatra_otg_binding_path = os.path.join(ws, f'ondatra_otg_{ondatra_files_suffix}.binding')
    testbed_info_path = os.path.join(testbed_logs_dir, f'testbed_{ondatra_files_suffix}_info.txt')
    otg_docker_compose_file = os.path.join(testbed_logs_dir, f'otg-docker-compose.yml')
    pyats_testbed = kwargs.get('testbed', reserved_testbed.get('pyats_testbed', None))
            
    if reserved_testbed.get('sim', False):
        sim_out_dir = os.path.join(testbed_logs_dir, 'bringup_success')
        pyvxr_generator = _resolve_path_if_needed(internal_fp_repo_dir, os.path.join('exec', 'utils', 'pyvxr', 'generate_bindings.py'))
        check_output(f'python3 {pyvxr_generator} {sim_out_dir} {ondatra_testbed_path} {ondatra_binding_path}')

        mgmt_ips = _sim_get_mgmt_ips(testbed_logs_dir)
        if not type(reserved_testbed['baseconf']) is dict:
            reserved_testbed['baseconf'] = {
                'dut': reserved_testbed['baseconf']
            }
                
        reserved_testbed['cli_conf'] = {}
        reserved_testbed['ondatra_baseconf_path'] = {}
        for dut, conf in reserved_testbed['baseconf'].items():
            baseconf_file_path = _resolve_path_if_needed(internal_fp_repo_dir, conf)
            ondatra_baseconf_path = os.path.join(ws, f'ondatra_{ondatra_files_suffix}_{dut}.conf')
            reserved_testbed['ondatra_baseconf_path'][dut] = ondatra_baseconf_path
            shutil.copyfile(baseconf_file_path, ondatra_baseconf_path)
        
            mgmt_ip = mgmt_ips[dut]
            logger.info(f"Found management ip: {mgmt_ip} for dut '{dut}'")
            
            extra_conf = []
            mgmt_vrf = _sim_get_vrf(ondatra_baseconf_path)
            if mgmt_vrf:
                extra_conf.append(f'ipv4 virtual address vrf {mgmt_vrf} {mgmt_ip}/24')
            else:
                extra_conf.append(f'ipv4 virtual address {mgmt_ip}/24')

            with open(ondatra_baseconf_path, 'r') as cli:
                lines = []
                for l in cli.read().splitlines():
                    if l.strip() == 'end':
                        lines.extend(extra_conf)
                    lines.append(l)
                reserved_testbed['cli_conf'][dut] = lines
            
            _cli_to_gnmi_set_file(reserved_testbed['cli_conf'][dut], ondatra_baseconf_path)
            check_output("sed -i 's|id: \"" + dut + "\"|id: \"" + dut + "\"\\nconfig:{\\ngnmi_set_file:\"" + ondatra_baseconf_path + "\"\\n  }|g' " + ondatra_binding_path)
    else:
        testbed_info_path = os.path.join(os.path.dirname(testbed_logs_dir), 
            f'testbed_{reserved_testbed["id"]}_info.txt')

        hw_testbed_file_path = _resolve_path_if_needed(internal_fp_repo_dir, reserved_testbed['testbed'])
        hw_binding_file_path = _resolve_path_if_needed(internal_fp_repo_dir, reserved_testbed['binding'])        
        tb_file = _resolve_path_if_needed(internal_fp_repo_dir, MTLS_DEFAULT_TRUST_BUNDLE_FILE)
        key_file = _resolve_path_if_needed(internal_fp_repo_dir, MTLS_DEFAULT_KEY_FILE)
        cert_file = _resolve_path_if_needed(internal_fp_repo_dir, MTLS_DEFAULT_CERT_FILE)

        shutil.copyfile(hw_testbed_file_path, ondatra_testbed_path)
        shutil.copyfile(hw_binding_file_path, ondatra_binding_path)
        
        if type(reserved_testbed['baseconf']) is dict:
            for dut, conf in reserved_testbed['baseconf'].items():
                baseconf_file_path = _resolve_path_if_needed(internal_fp_repo_dir, conf)
                ondatra_baseconf_path = os.path.join(ws, f'ondatra_{ondatra_files_suffix}_{dut}.conf')
                shutil.copyfile(baseconf_file_path, ondatra_baseconf_path)            
                check_output("sed -i 's|id: \"" + dut + "\"|id: \"" + dut + "\"\\nconfig:{\\ngnmi_set_file:\"" + ondatra_baseconf_path + "\"\\n  }|g' " + ondatra_binding_path)
        else:
            baseconf_file_path = _resolve_path_if_needed(internal_fp_repo_dir, reserved_testbed['baseconf'])
            ondatra_baseconf_path = os.path.join(ws, f'ondatra_{ondatra_files_suffix}.conf')
            shutil.copyfile(baseconf_file_path, ondatra_baseconf_path)    
            check_output(f"sed -i 's|$BASE_CONF_PATH|{ondatra_baseconf_path}|g' {ondatra_binding_path}")

        check_output(f"sed -i 's|$TRUST_BUNDLE_FILE|{tb_file}|g' {ondatra_binding_path}")
        check_output(f"sed -i 's|$CERT_FILE|{cert_file}|g' {ondatra_binding_path}")
        check_output(f"sed -i 's|$KEY_FILE|{key_file}|g' {ondatra_binding_path}")

        reserved_testbed['mtls_key_file'] = key_file
        reserved_testbed['mtls_cert_file'] = cert_file

    reserved_testbed['testbed_file'] = ondatra_testbed_path
    reserved_testbed['testbed_info_file'] = testbed_info_path
    reserved_testbed['pyats_testbed_file'] = pyats_testbed
    reserved_testbed['ate_binding_file'] = ondatra_binding_path
    reserved_testbed['otg_binding_file'] = ondatra_otg_binding_path
    reserved_testbed['otg_docker_compose_file'] = otg_docker_compose_file
    reserved_testbed['binding_file'] = reserved_testbed['ate_binding_file']

    _write_otg_binding(ws, internal_fp_repo_dir, reserved_testbed)
    _write_otg_docker_compose_file(otg_docker_compose_file, reserved_testbed)
    return reserved_testbed

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
@app.task(bind=True, max_retries=2, autoretry_for=[CommandFailed], soft_time_limit=1*90*60, time_limit=1*90*60)
def SoftwareUpgrade(self, ws, lineup, efr, internal_fp_repo_dir, testbed_logs_dir, 
                    reserved_testbed, images, image_url=None, force_install=False):
    logger.print("Performing Software Upgrade...")
    
    if image_url: img = image_url
    else: img = images[0]
    su_command = f'{GO_BIN} test -v ' \
            f'./exec/utils/software_upgrade ' \
            f'-timeout 60m ' \
            f'-args ' \
            f'-testbed {reserved_testbed["testbed_file"]} ' \
            f'-binding {reserved_testbed["binding_file"]} ' \
            f'-imagePath "{img}" ' \
            f'-lineup {lineup} ' \
            f'-efr {efr} '

    if force_install:
        su_command += f'-force'

    env = dict(os.environ)
    env.update(_get_go_env(ws))
    output = check_output(su_command, env=env, cwd=internal_fp_repo_dir)
    #TODO: find a better way?
    if not 'Image already installed' in output:
        self.enqueue_child(ForceReboot.s(
            ws=ws,
            internal_fp_repo_dir=internal_fp_repo_dir, 
            reserved_testbed=reserved_testbed,
        ))

# noinspection PyPep8Naming
@app.task(bind=True, max_retries=3, autoretry_for=[CommandFailed], soft_time_limit=1*60*60, time_limit=1*60*60)
def ForceReboot(self, ws, internal_fp_repo_dir, reserved_testbed):
    logger.print("Rebooting...")
    reboot_command = f'{GO_BIN} test -v ' \
            f'./exec/utils/reboot ' \
            f'-timeout 30m ' \
            f'-args ' \
            f'-testbed {reserved_testbed["testbed_file"]} ' \
            f'-binding {reserved_testbed["binding_file"]}'

    env = dict(os.environ)
    env.update(_get_go_env(ws))
    check_output(reboot_command, env=env, cwd=internal_fp_repo_dir)

# noinspection PyPep8Naming
@app.task(bind=True, max_retries=3, autoretry_for=[CommandFailed], soft_time_limit=1*60*60, time_limit=1*60*60)
def InstallSMUs(self, ws, internal_fp_repo_dir, reserved_testbed, smus):
    logger.print("Installing SMUs...")
    smu_install_cmd = f'{GO_BIN} test -v ' \
            f'./exec/utils/smu_install ' \
            f'-timeout 30m ' \
            f'-args ' \
            f'-testbed {reserved_testbed["testbed_file"]} ' \
            f'-binding {reserved_testbed["binding_file"]} ' \
            f'-smus {smus} '

    env = dict(os.environ)
    env.update(_get_go_env(ws))
    check_output(smu_install_cmd, env=env, cwd=internal_fp_repo_dir)

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
def CollectDebugFiles(self, ws, internal_fp_repo_dir, reserved_testbed, test_log_directory_path, timestamp, core,xunit_results_filepath) -> str:
    logger.print("Collecting debug files...")

    with tempfile.NamedTemporaryFile(delete=False) as f:
        tmp_binding_file = f.name
        shutil.copyfile(reserved_testbed['binding_file'], tmp_binding_file)
        check_output(f"sed -i 's|gnmi_set_file|#gnmi_set_file|g' {tmp_binding_file}")

    # create a directory here to send all logs as firex has a line limit
    if core:
        collect_core_files = f'{GO_BIN} test -v ' \
                f'./exec/utils/debug ' \
                f'-timeout 60m ' \
                f'-args ' \
                f'-testbed {reserved_testbed["testbed_file"]} ' \
                f'-binding {tmp_binding_file} ' \
                f'-outDir {test_log_directory_path}/debug_files ' \
                f'-timestamp {str(timestamp)} ' \
                f'-core=true ' \
                f'-v 5'
    else:
        collect_debug_cmd = f'{GO_BIN} test -v ' \
                f'./exec/utils/debug ' \
                f'-timeout 60m ' \
                f'-args ' \
                f'-testbed {reserved_testbed["testbed_file"]} ' \
                f'-binding {tmp_binding_file} ' \
                f'-outDir {test_log_directory_path}/debug_files ' \
                f'-timestamp {str(timestamp)} ' \
                f'-core=false ' \
                f'-v 5'

    try:
        env = dict(os.environ)
        env.update(_get_go_env(ws))
        if core:
            check_output(collect_core_files, env=env, cwd=internal_fp_repo_dir)
        else:
            check_output(collect_debug_cmd, env=env, cwd=internal_fp_repo_dir)
    except:
        logger.warning(f'Failed to collect testbed information. Ignoring...') 
    finally:
        # collect core files if any
        if core:
            res = self.enqueue_child_and_get_results(CollectCoreFiles.s(
                test_log_directory_path=test_log_directory_path,
                xunit_results_filepath=xunit_results_filepath
            ))
            logger.info(res)
        os.remove(tmp_binding_file)
        return "CollectDebugFiles completed successfully"

# noinspection PyPep8Naming
@app.task(bind=True)
def CollectCoreFiles(self, test_log_directory_path,xunit_results_filepath)->str:
    try:
        logger.print(f'xunit_results_filepath: {xunit_results_filepath}')
        logger.print(f'test_log_directory_path: {test_log_directory_path}')
        arr = os.listdir(f'{test_log_directory_path}/debug_files/dut/CollectDebugFiles/')
        r = re.compile(r'core\b',re.IGNORECASE)
        corefileslist = list(filter(lambda x: r.search(str(x)),arr))
        logger.print(f'Array of core files if any {corefileslist}')
        
        try:
            if os.path.exists(xunit_results_filepath) and os.path.getsize(xunit_results_filepath) > 0:
                logger.warn(f'file exists and its not empty')
                tree = ET.parse(xunit_results_filepath)
                testsuite = tree.find("testsuite")
                prop = testsuite[0] 
                if len(corefileslist) == 0:
                    nsub = ET.SubElement(prop, "property",attrib={"name": "corefile"})
                    nsub.set("value","no corefile(s) found")
                else:
                    for file in corefileslist:
                        nsub = ET.SubElement(prop, "property",attrib={"name":"corefile"})
                        nsub.set("value",file)
                    logger.print(f"setting corefile failure {str(len(corefileslist))}")
                    fe = ET.SubElement(testsuite, "testcase",attrib = {'classname': "",'name':"CoreFileCheck", "time":"1"})
                    ET.SubElement(fe,"failure",attrib={"message":"Failed"}).text = "Core files were found"
                    tree.write(xunit_results_filepath,encoding="utf-8")
                return "CollectCoreFiles exited"
            else:
                if os.path.exists(xunit_results_filepath) == True:
                    logger.warn("File exists but its empty")
                    return "CollectCoreFiles exited"
                else:
                    logger.error("File does not exists")
                    return "CollectCoreFiles exited"
        except Exception as error:
            logger.error(f"XML find was not able to find './testsuite/properties/ with the error: {error}'")
            return "CollectCoreFiles exited with exception"
    except Exception as error:
        logger.warning(f'Failed to collect testbed information. Ignoring... with error: {error}') 
        return "CollectCoreFiles exited with exception"


# noinspection PyPep8Naming
@app.task(bind=True)
def CollectTestbedInfo(self, ws, internal_fp_repo_dir, reserved_testbed):
    if os.path.exists(reserved_testbed['testbed_info_file']):
        return

    logger.print("Collecting testbed info...")
    testbed_info_cmd = f'{GO_BIN} test -v ' \
            f'./exec/utils/testbed ' \
            f'-timeout 10m ' \
            f'-args ' \
            f'-testbed {reserved_testbed["testbed_file"]} ' \
            f'-binding {reserved_testbed["binding_file"]} ' \
            f'-outFile {reserved_testbed["testbed_info_file"]}'
    try:
        env = dict(os.environ)
        env.update(_get_go_env(ws))
        check_output(testbed_info_cmd, env=env, cwd=internal_fp_repo_dir)
    except:
        logger.warning(f'Failed to collect testbed information. Ignoring...')

# noinspection PyPep8Naming
@app.task(bind=True, returns=('certs_dir'))
def GenerateCertificates(self, ws, internal_fp_repo_dir, reserved_testbed):
    logger.print("Generating Certificates...")
    
    certs_dir = os.path.join(ws, "certificates")
    gen_command = f'{GO_BIN} test -v ' \
            f'./exec/utils/certgen ' \
            f'-args ' \
            f'-testbed {reserved_testbed["testbed_file"]} ' \
            f'-binding {reserved_testbed["binding_file"]} ' \
            f'-outDir "{certs_dir}" '

    env = dict(os.environ)
    env.update(_get_go_env(ws))
    check_output(gen_command, env=env, cwd=internal_fp_repo_dir)
    return certs_dir

# noinspection PyPep8Naming
@app.task(bind=True)
def SimEnableMTLS(self, ws, internal_fp_repo_dir, reserved_testbed, certs_dir):
    # parser_cmd = f'{PYTHON_BIN} exec/utils/confparser/sim_add_mtls_conf.py ' \
    #     f'{" ".join(reserved_testbed["baseconf"].values())}'
    # logger.print(f'Executing confparser cmd {parser_cmd}')
    # logger.print(
    #     check_output(parser_cmd, cwd=internal_fp_repo_dir)
    # )
    
    for dut, baseconf in reserved_testbed['ondatra_baseconf_path'].items():
        new_conf = []
        for l in reserved_testbed['cli_conf'][dut]:
            new_conf.append(l)
            if l == 'grpc':
                new_conf.append('tls-mutual')
                new_conf.append('certificate-authentication')
        reserved_testbed['cli_conf'][dut] = new_conf
        _cli_to_gnmi_set_file(reserved_testbed['cli_conf'][dut], baseconf)

    # convert binding to json
    with tempfile.NamedTemporaryFile() as of:
        out_file = of.name
        cmd = f'{GO_BIN} run ' \
            f'./exec/utils/binding/tojson ' \
            f'-binding {reserved_testbed["binding_file"]} ' \
            f'-out {out_file}'

        env = dict(os.environ)
        env.update(_get_go_env(ws))
        
        check_output(cmd, env=env, cwd=internal_fp_repo_dir)
        with open(out_file, 'r') as fp:
            j = json.load(fp)

    logger.print(json.dumps(j))
    
    glob_username = j.get('options', {}).get('username', "") 
    glob_password = j.get('options', {}).get('password', "")   
    j['options'] = {}
    
    for dut in j.get('duts', []):
        dut_id = dut['id']
        dut_username = dut.get('options', {}).get('username', glob_username)
        dut_password = dut.get('options', {}).get('password', glob_password)
        dut['options'] = {}

        dut['config'] = {
            'gnmi_set_file': [reserved_testbed['ondatra_baseconf_path'][dut_id]]
        }
        
        for s in ['gnmi', 'gnoi', 'gnsi', 'gribi', 'p4rt']:
            target = dut.get(s, {}).get('target', '')
            if target:
                username = dut[s].get('username', dut_username)
                password = dut[s].get('password', dut_password)
                dut[s] = {
                    'target': target,
                    'username': username,
                    'password': password,
                    'mutual_tls': True,
                    'trust_bundle_file': os.path.join(certs_dir, dut_id, 'ca.cert.pem'),
                    'cert_file': os.path.join(certs_dir, dut_id, f'ems.cert.pem'),
                    'key_file': os.path.join(certs_dir, dut_id, f'ems.key.pem')
                }
        
        for s in ['ssh']:
            target = dut.get(s, {}).get('target', '')
            if target:
                username = dut[s].get('username', dut_username)
                password = dut[s].get('password', dut_password)
                dut[s] = {
                    'target': target,
                    'username': username,
                    'password': password,
                    'skip_verify': True,
                }

    # convert binding to prototext
    with tempfile.NamedTemporaryFile() as f:
        tmp_binding_file = f.name
        with open(tmp_binding_file, "w") as outfile:
            outfile.write(json.dumps(j))
        
        logger.print(json.dumps(j))
        
        cmd = f'{GO_BIN} run ' \
            f'./exec/utils/binding/fromjson ' \
            f'-binding {tmp_binding_file} ' \
            f'-out {reserved_testbed["binding_file"]}'

        check_output(cmd, env=env, cwd=internal_fp_repo_dir)

# noinspection PyPep8Naming
@app.task(bind=True)
def GoTidy(self, ws, repo):
    env = dict(os.environ)
    env.update(_get_go_env(ws))
    logger.print(
        check_output(f'{GO_BIN} mod tidy', env=env, cwd=repo)
    )

# noinspection PyPep8Naming
@app.task(bind=True)
def InstallGoDelve(self, ws, repo):
    env = dict(os.environ)
    env.update(_get_go_env(ws))
    logger.print(
        check_output(f'{GO_BIN} install github.com/go-delve/delve/cmd/dlv@latest', env=env, cwd=repo)
    )
        
# noinspection PyPep8Naming
@app.task(bind=True)
def ReleaseIxiaPorts(self, ws, binding_file):
    logger.print("Releasing ixia ports...")
    with tempfile.NamedTemporaryFile() as f:
        #FIXME: remove once release script is updated to new binding proto
        tmp_binding_file = f.name
        shutil.copyfile(binding_file, tmp_binding_file)
        cmd = "sed -i 's|mutual_tls|#mutual_tls|g;"
        cmd += "s|trust_bundle_file|#trust_bundle_file|g;"
        cmd += "s|cert_file|#cert_file|g;"
        cmd += "s|key_file|#key_file|g' "
        cmd += f"{tmp_binding_file}"
        check_output(cmd)
        
        try:
            logger.print(
                check_output(f'{IXIA_RELEASE_BIN} {tmp_binding_file}')
            )
        except:
            logger.warning(f'Failed to release ixia ports. Ignoring...')

# noinspection PyPep8Naming
@app.task(bind=True, max_retries=3, autoretry_for=[AssertionError])
def BringupIxiaController(self, reserved_testbed):
    # TODO: delete this line
    logger.print(f"reserved_testbed [{reserved_testbed}]")
    pname = reserved_testbed["id"].lower()
    docker_file = reserved_testbed["otg_docker_compose_file"]
    cmd = f'/usr/local/bin/docker-compose -p {pname} --file {docker_file} up -d --force-recreate'
    remote_exec(cmd, hostname=reserved_testbed['otg']['host'], shell=True)

# noinspection PyPep8Naming
@app.task(bind=True, max_retries=3, autoretry_for=[AssertionError])
def TeardownIxiaController(self, reserved_testbed):
    pname = reserved_testbed["id"].lower()
    docker_file = reserved_testbed["otg_docker_compose_file"]
    cmd = f'/usr/local/bin/docker-compose -p {pname} --file {docker_file} down'
    remote_exec(cmd, hostname=reserved_testbed['otg']['host'], shell=True)

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
