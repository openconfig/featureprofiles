from firexapp.engine.celery import app
from celery.utils.log import get_task_logger
from celery.utils.log import get_task_logger
from microservices.workspace_tasks import Warn
from firexapp.firex_subprocess import check_output
from firexkit.task import FireXTask
from firexapp.common import silent_mkdir
from firexapp.submit.arguments import whitelist_arguments
from microservices.testbed_tasks import register_testbed_file_generator
from microservices.firex_base import returns, flame, InjectArgs, FireX
from services.cflow.code_coverage_tasks import CollectCoverageData
from microservices.runners.runner_base import FireXRunnerBase
from test_framework import register_test_framework_provider
from ci_plugins.vxsim import GenerateGoB4TestbedFile
from html_helper import get_link 
from helper import CommandFailed, remote_exec, scp_to_remote, scp_from_remote
from getpass import getuser
from pathlib import Path
import xml.etree.ElementTree as ET
import shutil
import socket
import random
import string
import tempfile
import hashlib
import glob
import uuid
import time
import json
import yaml
import git
import os
import re

logger = get_task_logger(__name__)

GO_BIN = '/auto/firex/bin/go'
PYTHON_BIN = '/auto/firex/sw/python/3.9.10/bin/python3.9'
HIBA_BIN_PATH = '/auto/b4ws/hiba' # https://github.com/google/hiba/blob/main/README.md

PUBLIC_FP_REPO_URL = 'https://github.com/openconfig/featureprofiles.git'
INTERNAL_FP_REPO_URL = 'git@wwwin-github.cisco.com:B4Test/featureprofiles.git'

TESTBEDS_FILE = 'exec/testbeds.yaml'

MTLS_DEFAULT_TRUST_BUNDLE_FILE = 'internal/cisco/security/cert/keys/CA/ca.cert.pem'
MTLS_DEFAULT_CERT_FILE = 'internal/cisco/security/cert/keys/clients/cafyauto.cert.pem'
MTLS_DEFAULT_KEY_FILE = 'internal/cisco/security/cert/keys/clients/cafyauto.key.pem'

DOCKER_KENG_CONTROLLER = 'ghcr.io/open-traffic-generator/keng-controller'
DOCKER_KENG_LAYER23 = 'ghcr.io/open-traffic-generator/keng-layer23-hw-server'
DOCKER_OTG_GNMI = 'ghcr.io/open-traffic-generator/otg-gnmi-server'

whitelist_arguments([
    'test_html_report',
    'release_ixia_ports',
    'test_ignore_aborted',
    'test_skip',
    'test_fail_skipped',
    'test_show_skipped',
    'test_repo_url',
    'test_branch',
    'test_revision',
    'test_pr',
    'sim_use_mtls',
    'sim_config_bootz',
    'collect_dut_info',
    'cflow_over_ssh',
    'testbed_checks'
    'otg_keng_controller'
    'otg_keng_layer23_hw_server'
    'otg_gnmi_server'
    'otg_controller_command'
    'testbed_checks',
    'testbeds_exclude',
    'violet_export'
])

def _get_user_nobackup_path(ws=None):
    p = os.path.join('/nobackup', getuser())
    if os.access(p, os.W_OK | os.X_OK):
        return p
    return ws
        
def _get_go_path():
    return os.path.join(_get_user_nobackup_path(), 'go')

def _get_go_bin_path():
    return os.path.join(_get_go_path(), 'bin')

def _get_venv_path(ws=None):
    if os.path.exists("/auto/b4ws/firex/venv"):
        # Use the shared venv if it exists
        return "/auto/b4ws/firex/venv"
    return os.path.join(_get_user_nobackup_path(ws), 'b4_firex_venv')

def _get_venv_python_bin(ws):
    return os.path.join(_get_venv_path(ws), 'bin', 'python')

def _get_venv_pip_bin(ws):
    return os.path.join(_get_venv_path(ws), 'bin', 'pip')

def _get_go_env(ws=None):
    PATH = "{}:{}:{}".format(
        HIBA_BIN_PATH, os.path.dirname(GO_BIN), os.environ["PATH"]
    ) 

    nobackup_path = _get_user_nobackup_path(ws)
    gocache = os.path.join(nobackup_path, '.gocache')
    os.makedirs(gocache, exist_ok=True)

    return {
        'GOPATH': _get_go_path(),
        'GOCACHE': gocache,
        'GOTMPDIR': gocache,
        'GOROOT': '/auto/firex/sw/go',
        'GOPROXY': "https://proxy.golang.org,direct",
        'PATH': PATH
    }

def _gobool(b=False):
    if b: return "true"
    return "false"

def _resolve_path_if_needed(dir, path):
    """
    Resolve a relative path to an absolute path if needed.

    Args:
        dir (str): The base directory to resolve the path against.
        path (str): The path to check and resolve if it is relative.

    Returns:
        str: The absolute path if the input path is relative, otherwise the original path.
    """
    # Check if the path is relative (does not start with '/')
    if path[0] != '/':
        # Join the base directory with the relative path to form an absolute path
        return os.path.join(dir, path)
    # Return the original path if it is already absolute
    return path

def _uuid_from_str(s):
    """
    Generate a UUID based on the MD5 hash of a given string.

    Args:
        s (str): The input string to generate the UUID from.

    Returns:
        uuid.UUID: A UUID object created from the MD5 hash of the input string.
    """
    # Compute the MD5 hash of the input string.
    hex_string = hashlib.md5(s.encode("UTF-8")).hexdigest()
    # Create and return a UUID object from the hash.
    return uuid.UUID(hex=hex_string)
    
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

def _otg_docker_compose_template(control_port, gnmi_port, rest_port, otg_keng_controller,otg_keng_layer23_hw_server,otg_gnmi_server,otg_controller_command,version):
   controller_version,layer23_version,gnmi_version = otg_keng_controller,otg_keng_layer23_hw_server,otg_gnmi_server
   if version["controller"] != '1.3.0-2':
       controller_version = version["controller"]
   if version["hw"] !=  '1.3.0-4':
       layer23_version = version["hw"]
   if version["gnmi"] !=  '1.13.15':
       gnmi_version = version["gnmi"]
    # check for controller_commands
   if otg_controller_command:
        # Remove the enclosing brackets and split the command into a list
        otg_controller_command = otg_controller_command[0].strip('[]').split(", ")
        controller_command_formatted = ""
        for i in otg_controller_command:
            controller_command_formatted = controller_command_formatted + f"\n      - \"{i}\""
   else:
        controller_command_formatted = ""
   dockerFile = f"""
version: "2.1"
services:
  controller:
    image: {DOCKER_KENG_CONTROLLER}:{controller_version}
    restart: always
    ports:
      - "{control_port}:40051"
      - "{rest_port}:8443"
    depends_on:
      layer23-hw-server:
        condition: service_started
    command:
      - "--accept-eula"
      - "--debug"
      - "--keng-layer23-hw-server"
      - "layer23-hw-server:5001"
      {controller_command_formatted}
    environment:
      - LICENSE_SERVERS=10.85.70.247
    logging:
      driver: "local"
      options:
        max-size: "100m"
        max-file: "10"
        mode: "non-blocking"
  layer23-hw-server:
    image: {DOCKER_KENG_LAYER23}:{layer23_version}
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
    image: {DOCKER_OTG_GNMI}:{gnmi_version}
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
   logger.info(f"dockerFile: {dockerFile}")
   return dockerFile

def _write_otg_docker_compose_file(docker_file, reserved_testbed, otg_keng_controller,otg_keng_layer23_hw_server,otg_gnmi_server,otg_controller_command,version):
    logger.info(f"_write_otg_docker_compose_file")
    if not 'otg' in reserved_testbed:
        return
    otg_info = reserved_testbed['otg']
    with open(docker_file, 'w') as fp:
        res = fp.write(_otg_docker_compose_template(otg_info['controller_port'], otg_info['gnmi_port'], otg_info['rest_port'], otg_keng_controller,otg_keng_layer23_hw_server,otg_gnmi_server,otg_controller_command,version))
    logger.info(f"docker-compose file written: {res}")

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
    """
    Extract the VRF (Virtual Routing and Forwarding) configuration from a base configuration file.
    Args:
        base_conf_file (str): The path to the base configuration file.
    Returns:
        str or None: The extracted VRF name if found, otherwise None.
    """
    # Regular expression to match the VRF configuration in the interface section
    intf_re = r'interface.*?MgmtEth0\/RP0\/CPU0/0(.|\n)*?(\bvrf(\b.*\b))(.|\n)*?!'
    # Open the base configuration file and search for the VRF pattern
    with open(base_conf_file, 'r') as fp:
        matches = re.search(intf_re, fp.read())
        # If a match is found, return the VRF name (group 3) after stripping whitespace
        if matches and matches.group(3) is not None:
            return matches.group(3).strip()
    return None # Return None if no VRF configuration is found

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

def _sim_get_port_redir(testbed_logs_dir):
    vxr_ports_file = os.path.join(testbed_logs_dir, "bringup_success", "sim-ports.yaml")
    with open(vxr_ports_file, "r") as fp:
        try:
            vxr_ports = yaml.safe_load(fp)
        except yaml.YAMLError:
            logger.warning("Failed to parse vxr ports file...")
            return
    
    devices = {}
    for d, e in vxr_ports.items():
        devices[d] = {
            'host': vxr_ports[d]['HostAgent'],
            'ports': {}
        }
        for k, v in e.items():
            try:
                if k.startswith('redir'):
                    devices[d]['ports'][int(k[5:])] = v
                elif k.startswith('xr_redir'):
                    devices[d]['ports'][int(k[8:])] = v
            except: continue
    return devices

def _sim_get_data_ports(testbed_logs_dir):
    vxr_conf_file = os.path.join(testbed_logs_dir, "bringup_success", "sim-config.yaml")
    with open(vxr_conf_file, "r") as fp:
        try:
            vxr_conf = yaml.safe_load(fp)
        except yaml.YAMLError:
            logger.warning("Failed to parse vxr ports file...")
            return
    data_ports = {}
    for dut, entry in vxr_conf.get('devices').items():
        data_ports[dut] = entry.get('data_ports', [])
    return data_ports

def _cli_to_gnmi_set_file(cli_lines, gnmi_file, extra_conf=[]):
    gnmi_set = _gnmi_set_file_template(cli_lines)
    with open(gnmi_file, 'w') as gnmi:
        gnmi.write(gnmi_set)

    with open(gnmi_file, 'w') as gnmi:
        gnmi.write(gnmi_set)

def _get_fixes_from_testbed_info(testbed_info_file):
    ddts_re = r'CSC[a-z][a-z]\d{5}'
    with open(testbed_info_file, 'r',encoding="utf8", errors='ignore') as fp:
        info = fp.read()
        return re.findall(ddts_re, info)

def _add_extra_properties_to_xml(ts, test_name, reserved_testbed, core_files=[]):
    """
    Add extra properties and core file information to a JUnit XML test suite element.

    Args:
        ts (xml.etree.ElementTree.Element): The test suite XML element.
        test_name (str): The name of the test plan.
        reserved_testbed (dict): Dictionary containing testbed information.
        core_files (list, optional): List of core files found during the test. Defaults to [].

    This function adds the following properties to the XML:
        - test.plan_id: The test plan name.
        - b4.fixes_active: Comma-separated list of active DDTS fixes.
        - b4.num_core_files: Number of core files found.
        - b4.testbed: The testbed ID.

    If core files are present, a testcase with a failure message listing the core files is added.
    """
    props = ts.find('./properties')
    if not props:
        props = ET.SubElement(ts, 'properties')

    has_test_plan_id = False
    for p in props:
        has_test_plan_id = p.get('name', '') == 'test.plan_id'
        if has_test_plan_id: break

    if not has_test_plan_id:
        ET.SubElement(props, 'property', attrib={
            'name': 'test.plan_id',
            'value': test_name
        })

    fixes = []
    if os.path.exists(reserved_testbed['testbed_info_file']):
        fixes = _get_fixes_from_testbed_info(reserved_testbed['testbed_info_file'])

    # Add a property to the XML with the active fixes as a comma-separated string.
    ET.SubElement(props, 'property', attrib={
        'name': 'b4.fixes_active',
        'value': ','.join(fixes)
    })
    # Add a property to the XML with the number of core files found
    ET.SubElement(props, 'property', attrib={
        'name': 'b4.num_core_files',
        'value': str(len(core_files))
    })
    # Add a property to the XML with the testbed ID
    ET.SubElement(props, 'property', attrib={
        'name': 'b4.testbed',
        'value': reserved_testbed["id"]
    })
    # If core files are found, add a 'testcase' element to the XML to indicate failure.
    if len(core_files) > 0:
        e = ET.SubElement(ts, 'testcase', attrib = {
            'classname': '',
            'name': 'CoreFileCheck',
            'time': '1'
        })
        # Add a 'failure' element to the 'testcase' with a message listing the core files.
        ET.SubElement(e, 'failure', attrib={
            'message': 'Failed'
        }).text = 'Found core files:\n' + '\n'.join(core_files)

def _generate_dummy_suite(test_name, fail=False, abort=False):
    """
    Generate a dummy JUnit XML test suite element.

    Args:
        test_name (str): The name of the test suite.
        fail (bool, optional): Whether to include a failure in the test case. Defaults to False.
        abort (bool, optional): Whether to include an error in the test case. Defaults to False.

    Returns:
        xml.etree.ElementTree.Element: The root XML element representing the test suite.

    This function creates a JUnit XML structure with a single test case. The test case can
    optionally include a failure or an error element based on the `fail` and `abort` arguments.
    """
    ts = ET.Element('testsuite', attrib={
        'name': test_name,
        'tests': '1',
        'failures': str(int(fail)),
        'errors': str(int(abort)),
        'skipped': '0'
    })

    tc = ET.SubElement(ts, 'testcase', attrib={
        'name': 'dummy',
        'time': '1'
    })

    if fail:
        ET.SubElement(tc, 'failure', attrib={
            'message': 'Failed'
        })
    elif abort:
        ET.SubElement(tc, 'error')
    else:
        ET.SubElement(tc, 'system-out')

    root = ET.Element("testsuites", attrib={
        'tests': '1',
        'failures': str(int(fail)),
        'errors': str(int(abort)),
        'skipped': '0'
    })
    root.append(ts)
    return root

def _write_xml_tree(root, xml_file):
    tree = ET.ElementTree(root)
    with open(xml_file, 'wb') as fp:
        tree.write(fp)

def _get_testsuites_from_xml(file_name):
    try:
        tree = ET.parse(file_name)
        return tree.getroot()
    except Exception as e:
        logger.print(f"Could not parse testsuite xml file {file_name}: {e}")
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

def _trylock_testbed(ws, internal_fp_repo_dir, testbed_id, testbed_logs_dir):
    try:
        testbed = _get_testbed_by_id(internal_fp_repo_dir, testbed_id)
        if testbed.get('sim', False):
            return testbed

        python_bin = _get_venv_python_bin(ws)
        tblock = _resolve_path_if_needed(internal_fp_repo_dir, 'exec/utils/tblock/tblock.py')
        output = _check_json_output(f'{python_bin} {tblock} {_get_testbeds_file(internal_fp_repo_dir)} {_get_locks_dir(testbed_logs_dir)} -j lock {testbed_id}')
        if output['status'] == 'ok':
            for tb in output['testbeds']:
                if tb['id'] == testbed_id:
                    return tb
        return None
    except:
        return None

def _reserve_testbed(ws, testbed_logs_dir, internal_fp_repo_dir, testbeds):
    logger.print('Reserving testbed...')
    reserved_testbed = None
    while not reserved_testbed:
        for t in testbeds:
            if os.path.exists(os.path.join(testbed_logs_dir, f'testbed_{t}_disabled.lock')):
                testbeds.remove(t)
                break
            reserved_testbed = _trylock_testbed(ws, internal_fp_repo_dir, t, testbed_logs_dir)
            if reserved_testbed: break
        time.sleep(random.randint(5,60))
    logger.print(f'Reserved testbed {reserved_testbed["id"]}')
    return reserved_testbed

def _release_testbed(ws, testbed_logs_dir, internal_fp_repo_dir, reserved_testbed):
    if reserved_testbed.get('sim', False):
        return True

    id = reserved_testbed['id']
    logger.print(f'Releasing testbed {id}')
    try:
        python_bin = _get_venv_python_bin(ws)
        tblock = _resolve_path_if_needed(internal_fp_repo_dir, 'exec/utils/tblock/tblock.py')
        output = _check_json_output(f'{python_bin} {tblock} {_get_testbeds_file(internal_fp_repo_dir)} {_get_locks_dir(testbed_logs_dir)} -j release {id}')
        if output['status'] != 'ok':
            logger.warn(f'Cannot release testbed {id}: {output["status"]}')
        return True
    except:
        logger.warn(f'Cannot release testbed {id}')
        return False

def _get_all_ondatra_log_files(ws, test_ws, test_path):
    env = dict(os.environ)
    env.update(_get_go_env(ws))

    output = check_output(f"go list {test_path}", env=env, cwd=test_ws)
    packages = output.splitlines()

    module_name = check_output("go list -m", env=env, cwd=test_ws)
    module_name = module_name.strip()
    packages = [pkg[len(module_name) + 1:] for pkg in packages if pkg.startswith(module_name)]

    prefix = packages[0]
    for p in packages[1:]:
        while not p.startswith(prefix) and prefix:
            prefix = prefix[:-1]
        if prefix:
            prefix = prefix[:prefix.rfind(os.sep) + 1]
        else:
            return []

    logger.print(f'Searching {prefix} for log files')
    pattern = os.path.join(test_ws, prefix, '**', "ondatra_logs.xml")
    log_files = [str(file) for file in glob.glob(pattern, recursive=True)]
    logger.print(f'Found log files: {log_files}')
    return log_files

def _aggregate_ondatra_log_files(log_files, out_file):
    root = _get_testsuites_from_xml(log_files[0])
    if root == None: return

    testsuite = root.find("testsuite")
    if testsuite == None: return

    if len(log_files) > 1:
        tests_attr = int(testsuite.attrib.get('tests', 0))
        failures_attr = int(testsuite.attrib.get('failures', 0))
        errors_attr = int(testsuite.attrib.get('errors', 0))
        skipped_attr = int(testsuite.attrib.get('skipped', 0))
        time_attr = float(testsuite.attrib.get('time', 0))

        for f in log_files[1:]:
            try:
                tree = ET.parse(f)
                for ts in tree.getroot().findall("testsuite"):
                    tests_attr += int(ts.attrib.get('tests', 0))
                    failures_attr += int(ts.attrib.get('failures', 0))
                    errors_attr += int(ts.attrib.get('errors', 0))
                    skipped_attr += int(ts.attrib.get('skipped', 0))
                    time_attr += float(ts.attrib.get('time', 0))
                    for tc in ts.findall("testcase"):
                        testsuite.append(tc)
            except Exception as e:
                logger.print(f"Could not parse testsuite xml file {f}: {e}")
                return

        testsuite.attrib['tests'] = str(tests_attr)
        testsuite.attrib['failures'] = str(failures_attr)
        testsuite.attrib['errors'] = str(errors_attr)
        testsuite.attrib['skipped'] = str(skipped_attr)
        testsuite.attrib['time'] = "{:.3f}".format(time_attr)
    _write_xml_tree(root, out_file)


@app.task(base=FireX, bind=True, soft_time_limit=12*60*60, time_limit=12*60*60)
@returns('internal_fp_repo_url', 'internal_fp_repo_dir', 'reserved_testbed',
        'slurm_cluster_head', 'sim_working_dir', 'slurm_jobid', 'topo_path', 'testbed')
def BringupTestbed(self, ws, testbed_logs_dir, testbeds, test_path,
                        internal_fp_repo_url=INTERNAL_FP_REPO_URL,
                        internal_fp_repo_branch='master',
                        internal_fp_repo_rev=None,
                        collect_tb_info=True,
                        test_requires_tgen=False,
                        test_requires_otg=False,
                        install_image=True,
                        force_install=False,
                        force_reboot=False,
                        sim_use_mtls=False,
                        sim_config_bootz=False,
                        testbed_checks=False,
                        smus=None,
                        testbeds_exclude=[]):
    # Define the directory for the internal feature profiles repository
    internal_fp_repo_dir = os.path.join(ws, 'b4_go_pkgs', 'openconfig', 'featureprofiles')
    # Clone the repository if it does not exist
    if not os.path.exists(internal_fp_repo_dir):
        c = CloneRepo.s(repo_url=internal_fp_repo_url,
                    repo_branch=internal_fp_repo_branch,
                    repo_rev=internal_fp_repo_rev,
                    target_dir=internal_fp_repo_dir)
        self.enqueue_child_and_get_results(c)
    # Create a Python virtual environment if it does not exist
    if not os.path.exists(_get_venv_path(ws)):
        c = CreatePythonVirtEnv.s(ws=ws, internal_fp_repo_dir=internal_fp_repo_dir)
        self.enqueue_child_and_get_results(c)

    # Convert testbeds and testbeds_exclude to lists if they are not already
    if not isinstance(testbeds, list): testbeds = testbeds.split(',')
    if not isinstance(testbeds_exclude, list): testbeds_exclude = testbeds_exclude.split(',')

    # Remove excluded testbeds from the list of testbeds
    for tb in testbeds_exclude:
        if tb in testbeds:
            logger.print(f'Excluding testbed {tb}')
            testbeds.remove(tb)

    # Reserve and configure testbeds while there are testbeds available
    while len(testbeds) > 0:
        reserved_testbed = _reserve_testbed(ws, testbed_logs_dir, internal_fp_repo_dir, testbeds)
        testbeds.remove(reserved_testbed["id"])

        c = InjectArgs(internal_fp_repo_dir=internal_fp_repo_dir,
                    reserved_testbed=reserved_testbed,
                    **self.abog)

        using_sim = reserved_testbed.get('sim', False)
        if using_sim:
            topo_file = _resolve_path_if_needed(internal_fp_repo_dir, reserved_testbed['topology'])
            with open(topo_file, "r") as fp:
                topo_yaml = yaml.safe_load(fp)

            # Ensure the base configuration is a dictionary
            if not type(reserved_testbed['baseconf']) is dict:
                reserved_testbed['baseconf'] = {
                    'dut': reserved_testbed['baseconf']
                }
            # Set a default gateway if not already defined
            if 'gateway' not in reserved_testbed:
                reserved_testbed['gateway'] = "192.168.122.1"
            # Copy base configuration files and update the topology YAML
            for dut, conf in reserved_testbed['baseconf'].items():
                baseconf_file = _resolve_path_if_needed(internal_fp_repo_dir, conf)
                baseconf_file_copy = os.path.join(testbed_logs_dir, f'baseconf_{dut}.conf')
                shutil.copyfile(baseconf_file, baseconf_file_copy)
                topo_yaml['devices'][dut]['cvac'] = baseconf_file_copy
            # Write the updated topology YAML back to the file
            with open(topo_file, "w") as fp:
                fp.write(yaml.dump(topo_yaml))
            c |= self.orig.s(plat='8000', topo_file=topo_file)
        # Generate Ondatra testbed files
        c |= GenerateOndatraTestbedFiles.s()
        if using_sim and sim_use_mtls:
            c |= GenerateCertificates.s()
            c |= SimEnableMTLS.s()

        if sim_config_bootz:
            c |= SimConfigBootz.s()
            
        # Determine if the test requires traffic generators
        is_otg = 'otg' in test_path or test_requires_otg
        is_tgen = 'ate' in test_path or is_otg or test_requires_tgen
        # Release Ixia ports if traffic generators are required
        if is_tgen:
            c |= ReleaseIxiaPorts.s()

        if is_otg:
            try:
                c |= BringupIxiaController.s()
            except Exception as e:
                _release_testbed(ws, testbed_logs_dir, internal_fp_repo_dir, reserved_testbed)
                raise Exception("Could not bringup IXIA Controller") from e
        # Perform additional checks and configurations for non-simulation testbeds
        if not using_sim:
            if testbed_checks:
                c |= CheckTestbed.s(tgen=is_tgen, otg=is_otg)
            if install_image:
                c |= SoftwareUpgrade.s(force_install=force_install)
        if smus:
            c |= InstallSMUs.s(smus=smus)
        if force_reboot:
            c |= ForceReboot.s()
        if collect_tb_info:
            c |= CollectTestbedInfo.s()
        try:
            result = self.enqueue_child_and_get_results(c)
            return (internal_fp_repo_url, internal_fp_repo_dir, result.get("reserved_testbed"),
                result.get("slurm_cluster_head", None), result.get("sim_working_dir", None),
                result.get("slurm_jobid", None), result.get("topo_path", None), result.get("testbed", None))
        except Exception as e:
            _release_testbed(ws, testbed_logs_dir, internal_fp_repo_dir, reserved_testbed)
            logger.warning(f'Failed to bringup testbed {reserved_testbed["id"]}: {e}')

    raise Exception(f'Could not bringup testbed')

@app.task(base=FireX, bind=True)
def CleanupTestbed(self, ws, testbed_logs_dir,
        internal_fp_repo_dir, reserved_testbed=None):
    logger.print('Cleaning up...')
    if reserved_testbed.get('sim', False):
        self.enqueue_child(
            self.orig.s(**self.abog),
            block=True
        )
    elif reserved_testbed:
        _release_testbed(ws, testbed_logs_dir, internal_fp_repo_dir, reserved_testbed)

def max_testbed_requests():
    return int(os.getenv("B4_FIREX_TESTBEDS_COUNT", '10'))

def decommission_testbed_after_tests():
    return os.getenv("B4_FIREX_DECOMMISSION_TESTBED", '1') == '1'

@register_test_framework_provider('b4')
def b4_chain_provider(ws, testsuite_id,
                        test_log_directory_path,
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
                        test_requires_tgen=False,
                        test_requires_otg=False,
                        fp_pre_tests=[],
                        fp_post_tests=[],
                        internal_test=False,
                        test_debug=False,
                        test_verbose=True,
                        collect_debug_files=True,
                        force_collect_debug_files=False,
                        override_test_args_from_env=True,
                        testbed=None,
                        sanitizer=None,
                        cflow=None,
                        cflow_over_ssh=False,
                        otg_keng_controller='1.3.0-2',
                        otg_keng_layer23_hw_server='1.3.0-4',
                        otg_gnmi_server='1.13.15',
                        violet_export=False,
                        **kwargs):

    if internal_test:
        test_repo_url = internal_fp_repo_url

    test_repo_uuid = str(_uuid_from_str(test_repo_url))
    test_repo_dir = os.path.join(ws, 'go_test_pkgs', test_repo_uuid, 'openconfig', 'featureprofiles')

    chain = InjectArgs(ws=ws,
                    testsuite_id=testsuite_id,
                    test_log_directory_path=test_log_directory_path,
                    internal_fp_repo_dir=internal_fp_repo_dir,
                    reserved_testbed=reserved_testbed,
                    test_repo_dir=test_repo_dir,
                    test_name=test_name,
                    test_path=test_path,
                    test_args=test_args,
                    test_timeout=test_timeout,
                    test_debug=test_debug,
                    test_verbose=test_verbose,
                    otg_keng_controller=otg_keng_controller,
                    otg_keng_layer23_hw_server=otg_keng_layer23_hw_server,
                    otg_gnmi_server=otg_gnmi_server,
                    collect_debug_files=collect_debug_files,
                    force_collect_debug_files=force_collect_debug_files,
                    override_test_args_from_env=override_test_args_from_env,
                    **kwargs)

    chain |= CloneRepo.s(repo_url=test_repo_url,
                    repo_branch=test_branch,
                    repo_rev=test_revision,
                    repo_pr=test_pr,
                    target_dir=test_repo_dir)

    if test_debug:
        chain |= InstallGoDelve.s()

    chain |= GoTidy.s(repo=test_repo_dir)

    reserved_testbed['testbed_file'] = reserved_testbed['noate_testbed_file']
    reserved_testbed['binding_file'] = reserved_testbed['noate_binding_file']

    is_otg = 'otg' in test_path or test_requires_otg
    is_tgen = 'ate' in test_path or is_otg or test_requires_tgen

    if is_tgen:
        reserved_testbed['testbed_file'] = reserved_testbed['ate_testbed_file']
        reserved_testbed['binding_file'] = reserved_testbed['ate_binding_file']

    if is_otg:
        reserved_testbed['binding_file'] = reserved_testbed['otg_binding_file']

    if is_tgen and not decommission_testbed_after_tests():
        chain |= ReleaseIxiaPorts.s()
        if is_otg:
            chain |= BringupIxiaController.s()

    if fp_pre_tests:
        for pt in fp_pre_tests:
            for k, v in pt.items():
                chain |= RunGoTest.s(test_repo_dir=internal_fp_repo_dir, test_path = v['test_path'], test_args = v.get('test_args'))

    chain |= RunGoTest.s(test_repo_dir=test_repo_dir, test_path = test_path, test_args = test_args, test_timeout = test_timeout)

    if violet_export:
        chain |= Export2Violet.s()

    if fp_post_tests:
        for pt in fp_post_tests:
            for k, v in pt.items():
                chain |= RunGoTest.s(test_repo_dir=internal_fp_repo_dir, test_path = v['test_path'], test_args = v.get('test_args'))

    if is_otg:
        chain |= CollectIxiaLogs.s(out_dir=os.path.join(test_log_directory_path, "debug_files", "otg"))
        chain |= TeardownIxiaController.s()

    if sanitizer:
        logger.info(f"Sanitizer is set to {sanitizer}. Collect show tech sanitizer from routers")
        chain |= CollectDebugFiles.s(
            ws=ws,
            internal_fp_repo_dir=internal_fp_repo_dir,
            reserved_testbed=reserved_testbed,
            out_dir=os.path.join(test_log_directory_path, "sanitizer_logs"),
            collect_tech=True,
            custom_tech="sanitizer",
        )

    if cflow:
        if cflow_over_ssh: chain |= CollectCoverageDataOverSSH.s()
        else: chain |= CollectCoverageData.s(pyats_testbed=_resolve_path_if_needed(internal_fp_repo_dir, testbed))

    return chain

# noinspection PyPep8Naming
@app.task(bind=True, base=FireXRunnerBase)
@flame('log_file', lambda p: get_link(p, 'Test Output'))
@flame('test_log_directory_path', lambda p: get_link(p, 'All Logs'))
@returns('cflow_dat_dir', 'xunit_results', 'log_file', "start_time", "stop_time")
def RunGoTest(self: FireXTask, ws, uid, skuid, testsuite_id, test_log_directory_path, xunit_results_filepath,
        test_repo_dir, internal_fp_repo_dir, reserved_testbed,
        test_name, test_path, test_args=None, test_timeout=0, collect_debug_files=False, force_collect_debug_files=False,
        collect_dut_info=True, override_test_args_from_env=False, test_debug=False, test_verbose=False,
        test_ignore_aborted=False, test_skip=False, test_fail_skipped=False, test_show_skipped=False,
        test_enable_grpc_logs=False, **kwargs):
    """
        Execute a Go test using the Ondatra framework and handle test-related configurations and logging.

        Args:
            self (FireXTask): The FireX task instance.
            ws (str): The workspace directory for the test execution.
            uid (str): Unique identifier for the test run.
            skuid (str): Secondary unique identifier for the test run.
            testsuite_id (str): Identifier for the test suite being executed.
            test_log_directory_path (str): Path to the directory where test logs will be stored.
            xunit_results_filepath (str): Path to the file where xUnit results will be saved.
            test_repo_dir (str): Directory containing the test repository.
            internal_fp_repo_dir (str): Directory containing the internal feature profiles repository.
            reserved_testbed (dict): Information about the reserved testbed for the test.
            test_name (str): Name of the test being executed.
            test_path (str): Path to the test file or package.
            test_args (str, optional): Additional arguments to pass to the test. Defaults to None.
            test_timeout (int, optional): Timeout for the test execution in seconds. Defaults to 0 (no timeout).
            collect_debug_files (bool, optional): Whether to collect debug files after the test. Defaults to False.
            force_collect_debug_files (bool, optional): Whether to force collection of debug files. Defaults to False.
            collect_dut_info (bool, optional): Whether to collect Device Under Test (DUT) information. Defaults to True.
            override_test_args_from_env (bool, optional): Whether to override test arguments with environment variables. Defaults to False.
            test_debug (bool, optional): Whether to enable debug mode for the test. Defaults to False.
            test_verbose (bool, optional): Whether to enable verbose logging for the test. Defaults to False.
            test_ignore_aborted (bool, optional): Whether to ignore aborted tests. Defaults to False.
            test_skip (bool, optional): Whether to skip the test. Defaults to False.
            test_fail_skipped (bool, optional): Whether to fail the test if it is skipped. Defaults to False.
            test_show_skipped (bool, optional): Whether to show skipped tests in the results. Defaults to False.
            test_enable_grpc_logs (bool, optional): Whether to enable gRPC binary logging. Defaults to False.

        Returns:
            None

        Raises:
            Exception: If the test execution fails or encounters an error.
        """
    logger.print('Running Go test...')
    logger.print('----- env start ----')
    # Print all environment variables for debugging
    for name, value in os.environ.items():
        logger.print("{0}: {1}".format(name, value))
    logger.print('----- env end ----')

    # json_results_file = Path(test_log_directory_path) / f'go_logs.json'
    # Prepare paths for test logs and results
    xml_results_file = Path(test_log_directory_path) / f'ondatra_logs.xml'

    # Copy binding and testbed files to the log directory for reference
    shutil.copyfile(reserved_testbed['binding_file'],
            os.path.join(test_log_directory_path, "ondatra_binding.txt"))
    shutil.copyfile(reserved_testbed['testbed_file'],
            os.path.join(test_log_directory_path, "ondatra_testbed.txt"))

    # Copy testbed info file if it exists
    if os.path.exists(reserved_testbed['testbed_info_file']):
        shutil.copyfile(reserved_testbed['testbed_info_file'],
            os.path.join(test_log_directory_path, "testbed_info.txt"))

    # Update the test list file with the current test's skuid
    with open(reserved_testbed['test_list_file'], "a+") as fp:
        fp.write(f'{skuid}\n')
    shutil.copyfile(reserved_testbed['test_list_file'],
            os.path.join(test_log_directory_path, "testbed_tests_list.txt"))

    gotest_log_dir = os.path.join(test_log_directory_path, "test_logs")
    silent_mkdir(gotest_log_dir)

    go_args = ''
    test_args = test_args or ''

    # Optionally override test arguments with environment variables
    if override_test_args_from_env:
        extra_args_env_vars = {}
        if 'mtls_cert_file' in reserved_testbed:
            extra_args_env_vars["MTLS_CERT_FILE"] = reserved_testbed['mtls_cert_file']
        if 'mtls_key_file' in reserved_testbed:
            extra_args_env_vars["MTLS_KEY_FILE"] = reserved_testbed['mtls_key_file']
        test_args = _update_test_args_from_env(test_args, extra_args_env_vars)

    # Add log directory and required files to test arguments
    test_args = f'{test_args} ' \
        f'-log_dir {gotest_log_dir}'

    test_args += f' -binding {reserved_testbed["binding_file"]} -testbed {reserved_testbed["testbed_file"]} -xml "ondatra_logs.xml" '
    if test_verbose:
        test_args += f'-v 5 ' \
            f'-alsologtostderr'

    if not collect_dut_info:
        test_args += f' -collect_dut_info=false'

    # Set timeouts for test execution
    inactivity_timeout = 1800
    test_timeout = int(test_timeout)
    if test_timeout == 0: test_timeout = inactivity_timeout
    if test_timeout > 0: inactivity_timeout = 2*test_timeout

    # Prepare Go test arguments, using debug mode if requested
    go_args_prefix = '-'
    if test_debug:
        go_args_prefix = '-test.'

    go_args = f'{go_args} ' \
                f'{go_args_prefix}v ' \
                f'{go_args_prefix}timeout {test_timeout}s'

    # Build the command to run the test, using Delve for debugging if needed
    if test_debug:
        dlv_bin = os.path.join(_get_go_bin_path(), 'dlv')
        cmd = f'{dlv_bin} test ./{test_path} -- {go_args} {test_args}'
    else:
        cmd = f'{GO_BIN} test -p 1 ./{test_path} {go_args} -args {test_args}'

    test_ws = test_repo_dir
    test_env = _get_go_env(ws)
    test_env["GRPC_BINARY_LOG_FILTER"] = "*"

    # Set gateway environment variable if available
    tb_gw = reserved_testbed.get('gateway', None)
    if tb_gw:
        test_env["B4_DUT_GW"] = tb_gw

    start_time = self.get_current_time()
    start_timestamp = int(time.time())

    try:
        if not test_skip:
            self.run_script(cmd,
                            inactivity_timeout=inactivity_timeout,
                            ok_nonzero_returncodes=(1,),
                            extra_env_vars=test_env,
                            cwd=test_ws)
    finally:
        stop_time = self.get_current_time()
        
        # Move gRPC binary log file if it exists
        grpc_bin_log_file = os.path.join(test_ws, test_path, "grpc_binarylog.txt")
        if os.path.exists(grpc_bin_log_file):
            shutil.move(grpc_bin_log_file, gotest_log_dir)

        # Aggregate all Ondatra log files found in the workspace
        log_files = _get_all_ondatra_log_files(ws, test_ws, f"./{test_path}")
        if log_files:
            _aggregate_ondatra_log_files(log_files, str(xml_results_file))

        # Parse test results and check for pass/fail
        test_did_pass = True
        xml_root = _get_testsuites_from_xml(xml_results_file)
        if xml_root is None:
            if test_ignore_aborted or test_skip:
                xml_root = _generate_dummy_suite(test_name, fail=test_skip and test_fail_skipped)

        if xml_root:
            suites = xml_root.findall("testsuite")
            for suite in suites:
                test_did_pass = test_did_pass and suite.attrib['failures'] == '0' and suite.attrib['errors'] == '0'

            # Collect debug files if requested or if the test failed
            collect_debug_files = collect_debug_files or force_collect_debug_files
            core_check_only = (test_did_pass and not force_collect_debug_files) or (not test_did_pass and not collect_debug_files)
            core_files = self.enqueue_child_and_extract(CollectDebugFiles.s(
                ws=ws,
                internal_fp_repo_dir=internal_fp_repo_dir,
                reserved_testbed=reserved_testbed,
                out_dir = os.path.join(test_log_directory_path, "debug_files"),
                timestamp=start_timestamp,
                core_check=True,
                collect_tech=not core_check_only,
                collect_snapshot=not core_check_only,
                run_cmds=True,
                split_files_per_dut=True
            )).get('core_files', [])

            for suite in suites:
                _add_extra_properties_to_xml(suite, test_name, reserved_testbed, core_files)
            _write_xml_tree(xml_root, xunit_results_filepath)

            if not test_show_skipped:
                check_output(f"sed -i 's|skipped|disabled|g' {xunit_results_filepath}")

            logger.info(f"xunit_results_filepath {xunit_results_filepath}")
        else:
            logger.warn('Test did not produce expected xunit result')
        return None, xunit_results_filepath, self.console_output_file, start_time, stop_time

def _git_checkout_repo(repo, repo_branch=None, repo_rev=None, repo_pr=None):
    """
    Check out a specific branch, revision, or pull request in a Git repository.

    Args:
        repo (git.Repo): The Git repository object to operate on.
        repo_branch (str, optional): The branch to check out. Defaults to None.
        repo_rev (str, optional): The specific revision (commit hash) to check out. Defaults to None.
        repo_pr (int, optional): The pull request number to check out. Defaults to None.

    Returns:
        None

    Raises:
        git.GitCommandError: If any Git command fails during the checkout process.
    """
    # Reset the repository to a clean state
    repo.git.reset('--hard')
    repo.git.clean('-xdf')

    if repo_rev:
        # Check out a specific revision
        logger.print(f'Checking out revision {repo_rev}...')
        repo.git.checkout(repo_rev)
    elif repo_pr:
        # Check out a specific pull request
        logger.print(f'Checking out pr {repo_pr}...')
        branch_suffix = ''.join(random.choice(string.ascii_letters) for _ in range(6))
        repo.remotes.origin.fetch(f'pull/{repo_pr}/head:firex_{branch_suffix}_pr_{repo_pr}')
        repo.git.checkout(f'firex_{branch_suffix}_pr_{repo_pr}')
    elif repo_branch:
        # Check out a specific branch
        logger.print(f'Checking out branch {repo_branch}...')
        repo.git.checkout(repo_branch)

@app.task(bind=True, max_retries=3, autoretry_for=[git.GitCommandError])
def CloneRepo(self, repo_url, repo_branch, target_dir, repo_rev=None, repo_pr=None):
    """
    Clone a Git repository and check out a specific branch, revision, or pull request.

    Args:
        self: The task instance invoking this method.
        repo_url (str): The URL of the Git repository to clone.
        repo_branch (str): The branch to check out after cloning.
        target_dir (str): The directory where the repository will be cloned.
        repo_rev (str, optional): The specific commit hash to check out. Defaults to None.
        repo_pr (int, optional): The pull request number to check out. Defaults to None.

    Returns:
        None

    Raises:
        git.GitCommandError: If any Git command fails during the cloning or checkout process.
        Exception: If the repository cannot be cloned or checked out.
    """
    try:
        # Set an environment variable to skip downloading large files during the clone process
        os.environ['GIT_LFS_SKIP_SMUDGE'] = '1'
        # Extract the repository name from the URL
        repo_name = repo_url.split("/")[-1].split(".")[0]

        # Clone the repository if the target directory does not already exist
        if not os.path.exists(target_dir):
            logger.print(f'Cloning repo {repo_url} to {target_dir} branch {repo_branch}...')
            repo = git.Repo.clone_from(url=repo_url,
                                    to_path=target_dir)
        else:
            # If the directory exists, initialize the repository object
            repo = git.Repo(target_dir)
        _git_checkout_repo(repo, repo_branch, repo_rev, repo_pr) # Check out the specified branch, revision, or pull request

    except git.GitCommandError as e:
        # Handle permission errors specifically for public key issues
        err = e.stderr or ''
        if 'Permission denied (publickey).' in err.splitlines():
            err_msg = f'It appears you do not have proper access to the {repo_name} repository. Check your access ' \
                      f'permissions and make sure your ssh keys are added to your user profile here:\n' \
                      f'https://wwwin-github.cisco.com/settings/keys'
            self.enqueue_child(Warn.s(err_msg=err_msg), block=True, raise_exception_on_failure=False)
        # Raise a generic exception if cloning fails
        raise Exception("Could not clone repository") from e
    # Retrieve and log the short SHA of the head commit
    head_commit_sha = repo.head.commit.hexsha
    logger.info(f'Head Commit Sha: {head_commit_sha}')
    short_sha = repo.git.rev_parse(head_commit_sha, short=7)
    self.send_flame_html(version=f'{repo_name}: {short_sha}')

def _write_testbed_files(ws, internal_fp_repo_dir, reserved_testbed):
    # convert testbed to json
    with tempfile.NamedTemporaryFile() as of:
        outFile = of.name
        cmd = f'{GO_BIN} run ' \
            f'./exec/utils/proto/testbed/tojson ' \
            f'-testbed {reserved_testbed["ate_testbed_file"]} ' \
            f'-out {outFile}'

        env = dict(os.environ)
        env.update(_get_go_env(ws))

        check_output(cmd, env=env, cwd=internal_fp_repo_dir)
        with open(outFile, 'r') as fp:
            j = json.load(fp)

    j.pop('ates', None)
    j.pop('links', None)

    # convert binding to prototext
    with tempfile.NamedTemporaryFile() as f:
        tmp_testbed_file = f.name
        with open(tmp_testbed_file, "w") as outfile:
            outfile.write(json.dumps(j))

        cmd = f'{GO_BIN} run ' \
            f'./exec/utils/proto/testbed/fromjson ' \
            f'-testbed {tmp_testbed_file} ' \
            f'-out {reserved_testbed["noate_testbed_file"]}'

        check_output(cmd, env=env, cwd=internal_fp_repo_dir)


def _write_binding_files(ws, internal_fp_repo_dir, reserved_testbed):
    # convert binding to json
    with tempfile.NamedTemporaryFile() as of:
        outFile = of.name
        cmd = f'{GO_BIN} run ' \
            f'./exec/utils/proto/binding/tojson ' \
            f'-binding {reserved_testbed["ate_binding_file"]} ' \
            f'-out {outFile}'

        env = dict(os.environ)
        env.update(_get_go_env(ws))

        check_output(cmd, env=env, cwd=internal_fp_repo_dir)
        with open(outFile, 'r') as fp:
            j = json.load(fp)

    if 'otg' in reserved_testbed:
        otg_info = reserved_testbed['otg']
        controller_port = otg_info.get('controller_port_redir', otg_info['controller_port'])
        gnmi_port = otg_info.get('gnmi_port_redir', otg_info['gnmi_port'])

        #TODO: support multiple ates
        for ate in j.get('ates', []):
            for p in ate.get('ports', []):
                parts = p['name'].split('/')
                p['name'] = '{chassis};{card};{port}'.format(chassis=ate['name'], card=parts[0], port=parts[1])

            ate['name'] = '{host}:{controller_port}'.format(host=otg_info['host'], controller_port=controller_port)
            ate['options'] = {
                'username': 'admin',
                'password': 'admin'
            }
            ate['otg'] = {
                'target': '{host}:{controller_port}'.format(host=otg_info['host'], controller_port=controller_port),
                'insecure': True,
                'timeout': 300
            }

            ate['gnmi'] = {
                'target': '{host}:{gnmi_port}'.format(host=otg_info['host'], gnmi_port=gnmi_port),
                'skip_verify': True,
                'timeout': 150
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
                f'./exec/utils/proto/binding/fromjson ' \
                f'-binding {tmp_binding_file} ' \
                f'-out {reserved_testbed["otg_binding_file"]}'

            check_output(cmd, env=env, cwd=internal_fp_repo_dir)

    j.pop('ates', None)

    # convert binding to prototext
    with tempfile.NamedTemporaryFile() as f:
        tmp_binding_file = f.name
        with open(tmp_binding_file, "w") as outfile:
            outfile.write(json.dumps(j))

        cmd = f'{GO_BIN} run ' \
            f'./exec/utils/proto/binding/fromjson ' \
            f'-binding {tmp_binding_file} ' \
            f'-out {reserved_testbed["noate_binding_file"]}'

        check_output(cmd, env=env, cwd=internal_fp_repo_dir)

@app.task(base=FireX, bind=True, returns=('reserved_testbed'))
def GenerateOndatraTestbedFiles(self, ws, testbed_logs_dir, internal_fp_repo_dir, reserved_testbed, test_name, **kwargs):
    logger.print('Generating Ondatra files...')
    ondatra_files_suffix = ''.join(random.choice(string.ascii_letters) for _ in range(8))
    ondatra_testbed_path = os.path.join(ws, f'ondatra_{ondatra_files_suffix}.testbed')
    ondatra_noate_testbed_path = os.path.join(ws, f'ondatra_noate_{ondatra_files_suffix}.testbed')
    ondatra_binding_path = os.path.join(ws, f'ondatra_{ondatra_files_suffix}.binding')
    ondatra_noate_binding_path = os.path.join(ws, f'ondatra_noate_{ondatra_files_suffix}.binding')
    ondatra_otg_binding_path = os.path.join(ws, f'ondatra_otg_{ondatra_files_suffix}.binding')
    testbed_info_path = os.path.join(testbed_logs_dir, f'testbed_{ondatra_files_suffix}_info.txt')
    install_lock_file = os.path.join(testbed_logs_dir, f'testbed_{ondatra_files_suffix}_install.lock')
    disabled_lock_file = os.path.join(testbed_logs_dir, f'testbed_{ondatra_files_suffix}_disabled.lock')
    testbed_test_list_file = os.path.join(testbed_logs_dir, f'testbed_{ondatra_files_suffix}_tests_list.txt')
    pyats_testbed = kwargs.get('testbed', reserved_testbed.get('pyats_testbed', None))

    if reserved_testbed.get('sim', False):
        sim_out_dir = os.path.join(testbed_logs_dir, 'bringup_success')
        pyvxr_generator = _resolve_path_if_needed(internal_fp_repo_dir, os.path.join('exec', 'utils', 'pyvxr', 'generate_bindings.py'))
        # use ws venv python3.9 as we see binding file port mapping issues with <=python3.5
        python_bin = _get_venv_python_bin(ws)
        check_output(f'{python_bin} {pyvxr_generator} {sim_out_dir} {ondatra_testbed_path} {ondatra_binding_path}')

        sim_port_redir = _sim_get_port_redir(testbed_logs_dir)
        if 'ate_gui' in sim_port_redir:
            e = sim_port_redir['ate_gui']
            reserved_testbed['otg'] = {
                'host': e['host'],
                'port': e['ports'][22],
                'username': 'admin',
                'password': 'admin',
                'controller_port': 3389,
                'controller_port_redir': e['ports'][3389],
                'gnmi_port': 11009,
                'gnmi_port_redir': e['ports'][11009],
                'rest_port': 8443,
            }

        data_ports = _sim_get_data_ports(testbed_logs_dir)
        mgmt_ips = _sim_get_mgmt_ips(testbed_logs_dir)

        if not type(reserved_testbed['baseconf']) is dict:
            reserved_testbed['baseconf'] = {
                'dut': reserved_testbed['baseconf']
            }

        reserved_testbed['cli_conf'] = {}
        reserved_testbed['ondatra_baseconf_path'] = {}
        for dut, conf in reserved_testbed['baseconf'].items():
            baseconf_file_path = _resolve_path_if_needed(internal_fp_repo_dir, conf)
            ondatra_baseconf_path = os.path.join(testbed_logs_dir, f'ondatra_{ondatra_files_suffix}_{dut}.conf')
            reserved_testbed['ondatra_baseconf_path'][dut] = ondatra_baseconf_path
            shutil.copyfile(baseconf_file_path, ondatra_baseconf_path)

            extra_conf = []
            if dut in mgmt_ips:
                mgmt_ip = mgmt_ips[dut]
                logger.info(f"Found management ip: {mgmt_ip} for dut '{dut}'")

                mgmt_vrf = _sim_get_vrf(ondatra_baseconf_path)
                if mgmt_vrf: extra_conf.append(f'ipv4 virtual address vrf {mgmt_vrf} {mgmt_ip}/24')
                else: extra_conf.append(f'ipv4 virtual address {mgmt_ip}/24')

            # some FP tests (e.g., gNOI-3.1) expects that
            for d in data_ports.get(dut, []):
                extra_conf.append(f'interface {d} description {d}')

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
        install_lock_file = os.path.join(os.path.dirname(testbed_logs_dir),
            f'testbed_{reserved_testbed["id"]}_install.lock')
        disabled_lock_file = os.path.join(os.path.dirname(testbed_logs_dir),
            f'testbed_{reserved_testbed["id"]}_disabled.lock')
        testbed_test_list_file = os.path.join(os.path.dirname(testbed_logs_dir),
            f'testbed_{reserved_testbed["id"]}_tests_list.txt')

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
                ondatra_baseconf_path = os.path.join(testbed_logs_dir, f'ondatra_{ondatra_files_suffix}_{dut}.conf')
                shutil.copyfile(baseconf_file_path, ondatra_baseconf_path)
                check_output("sed -i 's|id: \"" + dut + "\"|id: \"" + dut + "\"\\nconfig:{\\ngnmi_set_file:\"" + ondatra_baseconf_path + "\"\\n  }|g' " + ondatra_binding_path)
        else:
            baseconf_file_path = _resolve_path_if_needed(internal_fp_repo_dir, reserved_testbed['baseconf'])
            ondatra_baseconf_path = os.path.join(testbed_logs_dir, f'ondatra_{ondatra_files_suffix}.conf')
            shutil.copyfile(baseconf_file_path, ondatra_baseconf_path)
            check_output(f"sed -i 's|$BASE_CONF_PATH|{ondatra_baseconf_path}|g' {ondatra_binding_path}")

        check_output(f"sed -i 's|$TRUST_BUNDLE_FILE|{tb_file}|g' {ondatra_binding_path}")
        check_output(f"sed -i 's|$CERT_FILE|{cert_file}|g' {ondatra_binding_path}")
        check_output(f"sed -i 's|$KEY_FILE|{key_file}|g' {ondatra_binding_path}")

        reserved_testbed['mtls_key_file'] = key_file
        reserved_testbed['mtls_cert_file'] = cert_file

    reserved_testbed['ate_testbed_file'] = ondatra_testbed_path
    reserved_testbed['noate_testbed_file'] = ondatra_noate_testbed_path
    reserved_testbed['testbed_file'] = reserved_testbed['noate_testbed_file']

    reserved_testbed['ate_binding_file'] = ondatra_binding_path
    reserved_testbed['otg_binding_file'] = ondatra_otg_binding_path
    reserved_testbed['noate_binding_file'] = ondatra_noate_binding_path
    reserved_testbed['binding_file'] = reserved_testbed['noate_binding_file']

    reserved_testbed['testbed_info_file'] = testbed_info_path
    reserved_testbed['install_lock_file'] = install_lock_file
    reserved_testbed['disabled_lock_file'] = disabled_lock_file
    reserved_testbed['test_list_file'] = testbed_test_list_file
    reserved_testbed['pyats_testbed_file'] = pyats_testbed

    _write_binding_files(ws, internal_fp_repo_dir, reserved_testbed)
    _write_testbed_files(ws, internal_fp_repo_dir, reserved_testbed)
    return reserved_testbed

@app.task(bind=True, soft_time_limit=1*10*60, time_limit=1*10*60)
def CheckTestbed(self, ws, internal_fp_repo_dir, reserved_testbed, tgen=False, otg=False):
    logger.print("Checking testbed connectivity...")

    testbed_file = reserved_testbed["noate_testbed_file"]
    binding_file = reserved_testbed["noate_binding_file"]

    if tgen:
        testbed_file = reserved_testbed["ate_testbed_file"]
        if otg: binding_file = reserved_testbed["otg_binding_file"]
        else: binding_file = reserved_testbed["ate_binding_file"]

    cmd = f'{GO_BIN} test -v ' \
            f'./exec/utils/tbchecks ' \
            f'-timeout 5m ' \
            f'-args ' \
            f'-collect_dut_info=false ' \
            f'-testbed {testbed_file} ' \
            f'-binding {binding_file} ' \
            f'-otg={_gobool(otg)} '

    env = dict(os.environ)
    env.update(_get_go_env(ws))
    check_output(cmd, env=env, cwd=internal_fp_repo_dir)

# noinspection PyPep8Naming
@app.task(bind=True, soft_time_limit=1*60*60, time_limit=1*60*60)
def SoftwareUpgrade(self, ws, lineup, efr, internal_fp_repo_dir, testbed_logs_dir,
                    reserved_testbed, images, image_url=None, force_install=False):
    if os.path.exists(reserved_testbed['install_lock_file']):
        return

    logger.print("Performing Software Upgrade...")

    if image_url: img = image_url
    else: img = images[0]
    su_command = f'{GO_BIN} test -v ' \
            f'./exec/utils/software_upgrade ' \
            f'-timeout 60m ' \
            f'-args ' \
            f'-collect_dut_info=false ' \
            f'-testbed {reserved_testbed["noate_testbed_file"]} ' \
            f'-binding {reserved_testbed["noate_binding_file"]} ' \
            f'-imagePath "{img}" ' \
            f'-lineup {lineup} ' \
            f'-efr {efr} ' \

    if force_install:
        su_command += f'-force'

    env = dict(os.environ)
    env.update(_get_go_env(ws))
    start_timestamp = int(time.time())
    try:
        check_output(su_command, env=env, cwd=internal_fp_repo_dir)
        Path(reserved_testbed['install_lock_file']).touch()
    except Exception as e:
        self.enqueue_child_and_extract(CollectDebugFiles.s(
            ws=ws,
            internal_fp_repo_dir=internal_fp_repo_dir,
            reserved_testbed=reserved_testbed,
            out_dir = os.path.join(testbed_logs_dir, "debug_files"),
            timestamp=start_timestamp,
            core_check=True,
            collect_tech=True,
            collect_snapshot=True,
            run_cmds=True,
            split_files_per_dut=True
        ))
        raise Exception("Software upgrade failed") from e

# noinspection PyPep8Naming
@app.task(bind=True, max_retries=3, autoretry_for=[CommandFailed], soft_time_limit=1*60*60, time_limit=1*60*60)
def ForceReboot(self, ws, internal_fp_repo_dir, reserved_testbed):
    logger.print("Rebooting...")
    reboot_command = f'{GO_BIN} test -v ' \
            f'./exec/utils/reboot ' \
            f'-timeout 30m ' \
            f'-args ' \
            f'-collect_dut_info=false ' \
            f'-testbed {reserved_testbed["noate_testbed_file"]} ' \
            f'-binding {reserved_testbed["noate_binding_file"]}'

    env = dict(os.environ)
    env.update(_get_go_env(ws))
    check_output(reboot_command, env=env, cwd=internal_fp_repo_dir)

# noinspection PyPep8Naming
@app.task(bind=True, soft_time_limit=1*60*60, time_limit=1*60*60)
def InstallSMUs(self, ws, internal_fp_repo_dir, reserved_testbed, smus):
    logger.print("Installing SMUs...")
    smu_install_cmd = f'{GO_BIN} test -v ' \
            f'./exec/utils/smu_install ' \
            f'-timeout 30m ' \
            f'-args ' \
            f'-collect_dut_info=false ' \
            f'-testbed {reserved_testbed["noate_testbed_file"]} ' \
            f'-binding {reserved_testbed["noate_binding_file"]} ' \
            f'-smus {smus} '

    env = dict(os.environ)
    env.update(_get_go_env(ws))
    check_output(smu_install_cmd, env=env, cwd=internal_fp_repo_dir)

# noinspection PyPep8Naming
@app.task(bind=True)
def CheckoutRepo(self, repo, repo_branch=None, repo_rev=None):
    # Reset the repository to a clean state by discarding all changes
    r = git.Repo(repo)
    r.git.reset('--hard')
    r.git.clean('-xdf')
    # Check out a specific revision if provided
    if repo_rev:
        r.git.checkout(repo_rev)
    # Otherwise, check out the specified branch if provided
    elif repo_branch:
        r.git.checkout(repo_branch)
    # Ensure the repository is reset to a clean state after checkout
    r.git.reset('--hard')
    r.git.clean('-xdf')

# noinspection PyPep8Naming
@app.task(bind=True, max_retries=3, autoretry_for=[CommandFailed], soft_time_limit=1*90*60, time_limit=1*90*60, returns=('core_files'))
def CollectDebugFiles(self, ws, internal_fp_repo_dir, reserved_testbed, out_dir,
                      timestamp=1, core_check=False, collect_tech=False, collect_snapshot=False,
                      run_cmds=False, split_files_per_dut=False, custom_tech="", custom_cmds=""):
    logger.print("Collecting debug files...")

    with tempfile.NamedTemporaryFile(delete=False) as f:
        tmp_binding_file = f.name
        shutil.copyfile(reserved_testbed['noate_binding_file'], tmp_binding_file)
        check_output(f"sed -i 's|gnmi_set_file|#gnmi_set_file|g' {tmp_binding_file}")

    collect_debug_cmd = f'{GO_BIN} test -v ' \
            f'./exec/utils/debug ' \
            f'-timeout 45m ' \
            f'-args ' \
            f'-collect_dut_info=false '\
            f'-testbed {reserved_testbed["noate_testbed_file"]} ' \
            f'-binding {tmp_binding_file} ' \
            f'-outDir {out_dir} ' \
            f'-timestamp {str(timestamp)} ' \
            f'-coreCheck={_gobool(core_check)} ' \
            f'-collectTech={_gobool(collect_tech)} ' \
            f'-runCmds={_gobool(run_cmds)} ' \
            f'-snapshot={_gobool(collect_snapshot)} ' \
            f'-splitPerDut={_gobool(split_files_per_dut)} ' \
            f'-showtechs="{custom_tech}" ' \
            f'-cmds="{custom_cmds}" '

    try:
        env = dict(os.environ)
        env.update(_get_go_env(ws))
        check_output(collect_debug_cmd, env=env, cwd=internal_fp_repo_dir, inactivity_timeout=0)
    except Exception as error:
        logger.warning(f'Failed to collect debug files with error: {error}')
    finally:
        os.remove(tmp_binding_file)

        core_files = []
        if core_check:
            duts = ['dut']
            if type(reserved_testbed['baseconf']) is dict:
                duts = [k for k in reserved_testbed['baseconf']]

            r = re.compile(r'core\b', re.IGNORECASE)
            for dut in duts:
                dutDir = os.path.join(out_dir, dut)
                if not os.path.exists(dutDir): continue
                arr = os.listdir(dutDir)
                dut_core_files = list(filter(lambda x: r.search(str(x)), arr))
                core_files.extend([f'{l} on dut "{dut}"' for l in dut_core_files])
        return core_files

# noinspection PyPep8Naming
@app.task(bind=True, soft_time_limit=1*10*60, time_limit=1*10*60)
def CollectTestbedInfo(self, ws, internal_fp_repo_dir, reserved_testbed):
    if os.path.exists(reserved_testbed['testbed_info_file']):
        return

    logger.print("Collecting testbed info...")
    testbed_info_cmd = f'{GO_BIN} test -v ' \
            f'./exec/utils/testbed ' \
            f'-timeout 5m ' \
            f'-args ' \
            f'-collect_dut_info=false ' \
            f'-testbed {reserved_testbed["noate_testbed_file"]} ' \
            f'-binding {reserved_testbed["noate_binding_file"]} ' \
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
            f'-collect_dut_info=false ' \
            f'-testbed {reserved_testbed["noate_testbed_file"]} ' \
            f'-binding {reserved_testbed["noate_binding_file"]} ' \
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

    # convert binding to json and adjust for mtls
    with tempfile.NamedTemporaryFile() as of:
        out_file = of.name
        cmd = f'{GO_BIN} run ' \
            f'./exec/utils/proto/binding/tojson ' \
            f'-binding {reserved_testbed["binding_file"]} ' \
            f'-out {out_file}'

        env = dict(os.environ)
        env.update(_get_go_env(ws))
        # Execute the command to generate the JSON binding file
        check_output(cmd, env=env, cwd=internal_fp_repo_dir)
        with open(out_file, 'r') as fp:
            j = json.load(fp)
    # Log the modified JSON configuration
    logger.print(json.dumps(j))
    # Extract global username from the JSON configuration
    glob_username = j.get('options', {}).get('username', "")
    glob_password = j.get('options', {}).get('password', "")
    j['options'] = {}

    dut_map = {}
    for dut in j.get('duts', []):
        dut_id = dut['id']
        dut_map[dut_id] = dut

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


    # add mtls params to cli conf
    for dut, cli_conf in reserved_testbed['cli_conf'].items():
        gnmi_username = dut_map[dut]['gnmi']['username']
        new_conf = []

        for l in cli_conf:
            if l == 'grpc':
                new_conf.append('aaa accounting commands default start-stop local')
                new_conf.append(f'aaa map-to username {gnmi_username} spiffe-id any')
                new_conf.append('aaa authorization exec default local')
                new_conf.append(l)
                new_conf.append('  tls-mutual')
                new_conf.append('  certificate-authentication')
            else:
                new_conf.append(l)
        reserved_testbed['cli_conf'][dut] = new_conf

        # apply new cli conf on duts using non-mtls binding
        with tempfile.NamedTemporaryFile() as of:
            cli_conf_file = of.name
            with open(cli_conf_file, 'w') as fp:
                fp.write('\n'.join(reserved_testbed['cli_conf'][dut]) + '\n')

            cmd = f'{GO_BIN} test -v ' \
                f'./exec/utils/setconf ' \
                f'-args ' \
                f'-collect_dut_info=false ' \
                f'-testbed {reserved_testbed["testbed_file"]} ' \
                f'-binding {reserved_testbed["binding_file"]} ' \
                f'-dut {dut} ' \
                f'-conf {cli_conf_file} ' \
                f'-ignore_set_err=true ' \

            env = dict(os.environ)
            env.update(_get_go_env(ws))
            check_output(cmd, env=env, cwd=internal_fp_repo_dir)

    # update gnmi conf set file from cli_conf
    for dut, baseconf in reserved_testbed['ondatra_baseconf_path'].items():
        _cli_to_gnmi_set_file(reserved_testbed['cli_conf'][dut], baseconf)

    # convert binding to prototext
    with tempfile.NamedTemporaryFile() as f:
        tmp_binding_file = f.name
        with open(tmp_binding_file, "w") as outfile:
            outfile.write(json.dumps(j))

        logger.print(json.dumps(j))

        cmd = f'{GO_BIN} run ' \
            f'./exec/utils/proto/binding/fromjson ' \
            f'-binding {tmp_binding_file} ' \
            f'-out {reserved_testbed["binding_file"]}'

        check_output(cmd, env=env, cwd=internal_fp_repo_dir)

# noinspection PyPep8Naming
# For Bootz on sim, dut mgmt interface is connected to bootz linux VM.
# The dhcp server on the VM will point dut to bootz port on VM.
# Add rinetd forwarding entry to forward incoming bootz connection on vm to the test host.
@app.task(bind=True)
def SimConfigBootz(self, testbed_logs_dir):
    vxr_ports_file = os.path.join(testbed_logs_dir, "bringup_success", "sim-ports.yaml")
    with open(vxr_ports_file, "r") as fp:
        try:
            vxr_ports = yaml.safe_load(fp)
        except yaml.YAMLError:
            logger.warning("Failed to parse vxr ports file...")
            return
    
    if not 'bootz' in vxr_ports:
        logger.warning("No bootz device found in vxr ports file...Ignoring")
        return
    
    if not 'xr_redir22' in vxr_ports['bootz']:
        logger.warning("No xr_redir22 port found in vxr ports file...Ignoring")
        return

    conn_args = {
        'username': 'root',
        'password': 'cisco123',
        'port': vxr_ports['bootz']['xr_redir22']
    }

    with tempfile.NamedTemporaryFile(mode='w+t') as of:
        hostname = socket.gethostname()
        of.writelines([
            f"0.0.0.0 15006 {hostname}.cisco.com 15006",    # bootz
        ])
        of.flush()
        scp_to_remote(vxr_ports['HostAgent'], of.name, "/root/bootz_bridge.conf", **conn_args)

    cmd = f'nohup /usr/local/bin/rinetd -c /root/bootz_bridge.conf -f &'
    remote_exec(cmd, vxr_ports['bootz']['HostAgent'], shell=True, **conn_args)

# noinspection PyPep8Naming
@app.task(bind=True)
def GoModReplace(self, ws, repo, pkg, target):
    """
    Replace a Go module dependency with a local path in the `go.mod` file.

    Args:
        self: The task instance invoking this method.
        ws (str): The workspace directory path.
        repo (str): The path to the repository where the `go.mod` file is located.

    Returns:
        None

    Raises:
        Exception: If the `go mod edit` command fails.
    """
    env = dict(os.environ)
    env.update(_get_go_env(ws))
    logger.print(
        check_output(f'{GO_BIN} mod edit -replace {pkg}={target}', env=env, cwd=repo)
    )

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
def InstallGoDelve(self, ws, internal_fp_repo_dir):
    env = dict(os.environ)
    env.update(_get_go_env(ws))
    logger.print(
        check_output(f'{GO_BIN} install github.com/go-delve/delve/cmd/dlv@latest', env=env, cwd=internal_fp_repo_dir)
    )

# noinspection PyPep8Naming
@app.task(bind=True, max_retries=3, autoretry_for=[CommandFailed])
def CreatePythonVirtEnv(self, ws, internal_fp_repo_dir):
    logger.print("Creating python venv...")
    requirements = [
        os.path.join(internal_fp_repo_dir, 'exec/utils/tblock/requirements.txt'),
        os.path.join(internal_fp_repo_dir, 'exec/utils/ixia/requirements.txt')
    ]

    venv_path = _get_venv_path(ws)
    venv_pip_bin = _get_venv_pip_bin(ws)
    venv_python_bin = _get_venv_python_bin(ws)

    if not os.path.exists(venv_python_bin):
        logger.print(check_output(f'{PYTHON_BIN} -m venv {venv_path}'))

    try:
        logger.print(check_output(f'{venv_pip_bin} install -r {" -r ".join(requirements)}'))
    except Exception as e:
        check_output(f'rm -rf {venv_path}')
        raise Exception("Could not initialize Python environment") from e

# noinspection PyPep8Naming
@app.task(bind=True)
def ReleaseIxiaPorts(self, ws, internal_fp_repo_dir, reserved_testbed):
    logger.print("Releasing ixia ports...")
    try:
        python_bin = _get_venv_python_bin(ws)
        ixia_release_bin = _resolve_path_if_needed(internal_fp_repo_dir, 'exec/utils/ixia/release_ports.py')
        logger.print(
            check_output(f'{python_bin} {ixia_release_bin} {reserved_testbed["ate_binding_file"]}')
        )
    except:
        logger.warning(f'Failed to release ixia ports. Ignoring...')

# noinspection PyPep8Naming
@app.task(bind=True, max_retries=3, autoretry_for=[AssertionError])
def BringupIxiaController(self, test_log_directory_path, reserved_testbed,
                        otg_keng_controller='1.3.0-2',
                        otg_keng_layer23_hw_server='1.3.0-4',
                        otg_gnmi_server='1.13.15',
                        otg_controller_command='',
                        otg_version=None):
    """
    Bring up the Ixia controller using Docker Compose with the specified configurations.

    Args:
        self: The task instance invoking this method.
        test_log_directory_path (str): The directory path where test logs are stored.
        reserved_testbed (dict): Information about the reserved testbed.
        otg_keng_controller (str, optional): The version of the OTG Keng Controller image. Defaults to '1.3.0-2'.
        otg_keng_layer23_hw_server (str, optional): The version of the OTG Layer23 hardware server image. Defaults to '1.3.0-4'.
        otg_gnmi_server (str, optional): The version of the OTG gNMI server image. Defaults to '1.13.15'.
        otg_controller_command (str, optional): Additional commands to pass to the OTG controller. Defaults to an empty string.
        otg_version (dict, optional): A dictionary specifying custom versions for the controller, hardware server, and gNMI server. Defaults to None.

    Returns:
        None

    Raises:
        AssertionError: If any assertion fails during the process.
    """
    # Set default OTG version if not provided
    if not otg_version:
        otg_version = {
            "controller": otg_keng_controller,
            "hw": otg_keng_layer23_hw_server,
            "gnmi": otg_gnmi_server,
        }

    # Log the reserved testbed for debugging
    # TODO: delete this line
    logger.print(f"BringupIxiaController, reserved_testbed [{reserved_testbed}]")
    pname = reserved_testbed["id"].lower()
    remote_exec(f'ls -la {test_log_directory_path}')
    # Generate Docker Compose file for OTG controller
    docker_file = os.path.join(test_log_directory_path, f'otg-docker-compose.yml')
    logger.print(f'the controller images are otg_keng_controller: {otg_keng_controller}, keng_layer23: {otg_keng_layer23_hw_server}, otg gnmi: {otg_gnmi_server}, docker_file: {docker_file}')
    _write_otg_docker_compose_file(docker_file, reserved_testbed, otg_keng_controller,otg_keng_layer23_hw_server,otg_gnmi_server,otg_controller_command,otg_version)

    conn_args = {}
    if 'username' in reserved_testbed['otg']:
        conn_args['username'] = reserved_testbed['otg']['username']
        conn_args['password'] = reserved_testbed['otg']['password']
    if 'port' in reserved_testbed['otg']:
        conn_args['port'] = reserved_testbed['otg']['port']

    # sim has no access to /auto/
    if reserved_testbed.get('sim', False):
        docker_file_on_remote = f'/tmp/{os.path.basename(docker_file)}'
        scp_to_remote(reserved_testbed['otg']['host'], docker_file, docker_file_on_remote, **conn_args)
        docker_file = docker_file_on_remote

    cmd = f'/usr/local/bin/docker-compose -p {pname} --file {docker_file} up -d --force-recreate'
    remote_exec(f'cat {docker_file}', reserved_testbed['otg']['host'], shell=True, **conn_args)
    remote_exec(cmd, reserved_testbed['otg']['host'], shell=True, **conn_args)

# noinspection PyPep8Naming
@app.task(bind=True)
def CollectIxiaLogs(self, reserved_testbed, out_dir):
    logger.print("Collecting OTG logs...")
    otg_log_collector_bin = "/auto/tftpboot-ottawa/b4/bin/otg_log_collector" # Path to the OTG log collector binary
    # Prepare a unique project name based on the testbed ID
    pname = reserved_testbed["id"].lower()

    try:
        # sim has no access to /auto/
        if reserved_testbed.get('sim', False):
            conn_args = {}
            if 'username' in reserved_testbed['otg']:
                conn_args['username'] = reserved_testbed['otg']['username']
                conn_args['password'] = reserved_testbed['otg']['password']
            if 'port' in reserved_testbed['otg']:
                conn_args['port'] = reserved_testbed['otg']['port']

            collect_script_on_remote = f'/tmp/{os.path.basename(otg_log_collector_bin)}'
            out_dir_on_remote = f'/tmp/otg_logs'
            scp_to_remote(reserved_testbed['otg']['host'], otg_log_collector_bin, collect_script_on_remote, **conn_args)

            cmd = f'{collect_script_on_remote} {pname} {out_dir_on_remote}'
            remote_exec(cmd, hostname=reserved_testbed['otg']['host'], shell=True, **conn_args)

            scp_from_remote(reserved_testbed['otg']['host'], out_dir_on_remote, out_dir, recursive=True, **conn_args)

        else:
            cmd = f'{otg_log_collector_bin} {pname} {out_dir}'
            remote_exec(cmd, hostname=reserved_testbed['otg']['host'], shell=True)
    except:
        logger.warning(f'Failed to collect OTG logs. Ignoring...')

# noinspection PyPep8Naming
@app.task(bind=True, max_retries=3, autoretry_for=[AssertionError])
def TeardownIxiaController(self, test_log_directory_path, reserved_testbed):
    conn_args = {}
    if 'username' in reserved_testbed['otg']:
        conn_args['username'] = reserved_testbed['otg']['username']
        conn_args['password'] = reserved_testbed['otg']['password']
    if 'port' in reserved_testbed['otg']:
        conn_args['port'] = reserved_testbed['otg']['port']

    docker_file = os.path.join(test_log_directory_path, f'otg-docker-compose.yml')
    if os.path.exists(docker_file):
        pname = reserved_testbed["id"].lower()

        # sim has no access to /auto/
        if reserved_testbed.get('sim', False):
            docker_file_on_remote = f'/tmp/{os.path.basename(docker_file)}'
            cmd = f'/usr/local/bin/docker-compose -p {pname} --file {docker_file_on_remote} down'
        else:
            cmd = f'/usr/local/bin/docker-compose -p {pname} --file {docker_file} down'
        remote_exec(cmd, hostname=reserved_testbed['otg']['host'], shell=True, **conn_args)
        os.remove(docker_file)

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

@app.task(base=FireX, bind=True)
@returns('cflow_dat_dir')
def CollectCoverageDataOverSSH(self, ws, internal_fp_repo_dir, reserved_testbed, cflow_arguments=None):
    if not cflow_arguments:
        cflow_arguments = {}

    cflow_date = cflow_arguments.get('arguments_dict', {}).get('cflow_date', 'no_date')
    cflow_dat_dir = os.path.join(ws, 'cflow', cflow_date)
    os.makedirs(cflow_dat_dir, exist_ok=True)

    c = CollectDebugFiles.s(
        ws=ws,
        internal_fp_repo_dir=internal_fp_repo_dir,
        reserved_testbed=reserved_testbed,
        out_dir=cflow_dat_dir,
        collect_tech=True,
        custom_tech="cflow",
    )
    self.enqueue_child_and_get_results(c)
    return cflow_dat_dir

# noinspection PyPep8Naming
@app.task(bind=True)
def Export2Violet(self, ws, internal_fp_repo_dir, test_log_directory_path):
    env = dict(os.environ)
    env.update(_get_go_env(ws))
    try:
        logger.print(
            check_output(f'{GO_BIN} run ./tools/cisco/violet_export/main.go {test_log_directory_path}', env=env, cwd=internal_fp_repo_dir)
        )
    except:
        logger.warning(f'Failed to export to Violet. Ignoring...')
