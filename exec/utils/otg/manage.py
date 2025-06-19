import subprocess
import platform
import tempfile
import argparse
import shutil
import yaml
import json
import os
import re

import sys
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), '../pyvxr')))

from generate_bindings import generate_bindings

GO_BIN = 'go'
ALT_GO_BIN = '/auto/firex/bin/go'

TESTBEDS_FILE = 'exec/testbeds.yaml'

MTLS_DEFAULT_TRUST_BUNDLE_FILE = 'internal/cisco/security/cert/keys/CA/ca.cert.pem'
MTLS_DEFAULT_CERT_FILE = 'internal/cisco/security/cert/keys/clients/cafyauto.cert.pem'
MTLS_DEFAULT_KEY_FILE = 'internal/cisco/security/cert/keys/clients/cafyauto.key.pem'

DOCKER_KENG_CONTROLLER = 'ghcr.io/open-traffic-generator/keng-controller'
DOCKER_KENG_LAYER23 = 'ghcr.io/open-traffic-generator/keng-layer23-hw-server'
DOCKER_OTG_GNMI = 'ghcr.io/open-traffic-generator/otg-gnmi-server'

def _check_otg_version(otgContainer,version):
    if not re.match(r'^\d+\.\d+\.\d+(-\d+)?$', version):
        print(f'WARNING - The {otgContainer} version might not be valid: {version}, please check the version number format x.y.z or x.y.z-n')
    return 

def check_output(cmd, **kwargs):
    kwargs['shell'] = True
    kwargs['text'] = True
    output = subprocess.check_output(cmd, **kwargs).strip()
    if output != "":
        print(output)
    return output
    
    
def _resolve_path_if_needed(dir, path):
    if path[0] != '/':
        return os.path.join(dir, path)
    return path
    
def _get_testbeds_file(fp_repo_dir):
    return _resolve_path_if_needed(fp_repo_dir, TESTBEDS_FILE)

def _get_testbed_by_id(fp_repo_dir, testbed_id):
    with open(_get_testbeds_file(fp_repo_dir), 'r') as fp:
        tf = yaml.safe_load(fp)
        if testbed_id in tf['testbeds']:
            tb = tf['testbeds'][testbed_id]
            tb['id'] = testbed_id
            return tb
    raise Exception(f'Testbed {testbed_id} not found')

def _otg_docker_compose_template(control_port, gnmi_port, rest_port, controller,layer23,gnmi,controller_command):
    if controller_command:
        # Remove the enclosing brackets and split the command into a list
        controller_command = controller_command[0].strip('[]').split()
        controller_command_formatted = ""
        for i in controller_command:
            controller_command_formatted = controller_command_formatted + f"\n      - \"{i}\""
    else:
        controller_command_formatted = ""
    yamlFile =  f"""
version: "2.1"
services:
  controller:
    image: {DOCKER_KENG_CONTROLLER}:{controller}
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
    image: {DOCKER_KENG_LAYER23}:{layer23}
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
    image: {DOCKER_OTG_GNMI}:{gnmi}
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
    return yamlFile

def _write_otg_docker_compose_file(docker_file, reserved_testbed,controller,layer23,gnmi,controller_command):
    if not 'otg' in reserved_testbed:
        return
    otg_info = reserved_testbed['otg']
    with open(docker_file, 'w') as fp:
        fp.write(_otg_docker_compose_template(otg_info['controller_port'], otg_info['gnmi_port'], otg_info['rest_port'],controller,layer23,gnmi,controller_command))

def _replace_binding_placeholders(fp_repo_dir, baseconf_files, binding_file):
    tb_file = _resolve_path_if_needed(fp_repo_dir, MTLS_DEFAULT_TRUST_BUNDLE_FILE)
    key_file = _resolve_path_if_needed(fp_repo_dir, MTLS_DEFAULT_KEY_FILE)
    cert_file = _resolve_path_if_needed(fp_repo_dir, MTLS_DEFAULT_CERT_FILE)
    with open(binding_file, 'r') as fp:
        data = fp.read()
    data = data.replace('$TRUST_BUNDLE_FILE', tb_file)
    data = data.replace('$CERT_FILE', cert_file)
    data = data.replace('$KEY_FILE', key_file)
    
    for dut, baseconf_file in baseconf_files.items():
        pattern = rf'(id:\s*"{re.escape(dut)}")'
        replacement = rf'\1\n  config: {{\n    gnmi_set_file: "{baseconf_file}"\n  }}'
        data = re.sub(pattern, replacement, data)

    with open(binding_file, 'w') as fp:
        fp.write(data)

def _write_otg_binding(fp_repo_dir, reserved_testbed, baseconf_files, otg_binding_file):
    otg_info = reserved_testbed['otg']

    # convert binding to json
    with tempfile.NamedTemporaryFile() as of:
        outFile = of.name
        cmd = f'{GO_BIN} run ' \
            f'./exec/utils/proto/binding/tojson ' \
            f'-binding {reserved_testbed["binding"]} ' \
            f'-out {outFile}'

        check_output(cmd, cwd=fp_repo_dir)
        
        with open(outFile, 'r') as fp:
            j = json.load(fp)

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
            'timeout': 100
        }

        ate['gnmi'] = {
            'target': '{host}:{gnmi_port}'.format(host=otg_info['host'], gnmi_port=gnmi_port),
            'skip_verify': True,
            'timeout': 30
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
            f'-out {otg_binding_file}'
            
        check_output(cmd, cwd=fp_repo_dir)        
        _replace_binding_placeholders(fp_repo_dir, baseconf_files, otg_binding_file)

def _write_ate_binding(fp_repo_dir, reserved_testbed, baseconf_files, ate_binding_file):
    shutil.copy(_resolve_path_if_needed(fp_repo_dir, reserved_testbed["binding"]), ate_binding_file)
    _replace_binding_placeholders(fp_repo_dir, baseconf_files, ate_binding_file)
        
def _write_testbed_file(fp_repo_dir, reserved_testbed, testbed_file):
    shutil.copy(_resolve_path_if_needed(fp_repo_dir, reserved_testbed["testbed"]), testbed_file)
    
def _write_baseconf_file(fp_repo_dir, conf, baseconf_file):
    shutil.copy(_resolve_path_if_needed(fp_repo_dir, conf), baseconf_file)

def _write_setup_script(testbed_id, testbed_file, ate_binding_file, otg_binding_file, setup_file):
    setup_script = f"""
export TESTBED_ID={testbed_id}
export TESTBED={testbed_file}
export ATE_BINDING={ate_binding_file}
export OTG_BINDING={otg_binding_file}
    """.strip()

    with open(setup_file, 'w') as fp: 
        fp.write(setup_script)
    print('You can run the following command to setup your enviroment:')
    print(f'source {setup_file}')

def _sim_get_port_redir(vxr_out):
    vxr_ports_file = os.path.join(vxr_out, "sim-ports.yaml")
    with open(vxr_ports_file, "r") as fp:
        try:
            vxr_ports = yaml.safe_load(fp)
        except yaml.YAMLError:
            sys.exit("Failed to parse vxr ports file")
    
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

def _sim_edit_topo(topo_file, baseconf_files, image):
    with open(topo_file, "r") as fp:
        try:
            topo = yaml.safe_load(fp)
        except yaml.YAMLError:
            sys.exit("Failed to parse topology file")
    with open(topo_file, "w") as fp:
        for id, node in topo.get("devices", {}).items():
            if node.get("platform", "") in ["spitfire_f", "spitfire_d"]:
                node["image"] = image
                if id in baseconf_files:
                    node["cvac"] = baseconf_files[id]
        yaml.dump(topo, fp, indent=2)

def _generate_baseconf_files(fp_repo_dir, reserved_testbed, out_dir):
    baseconf_files = {}
    for dut, conf in reserved_testbed['baseconf'].items():
        baseconf_file = os.path.join(out_dir, f'{dut}.baseconf')
        _write_baseconf_file(fp_repo_dir, conf, baseconf_file)
        baseconf_files[dut] = baseconf_file
    return baseconf_files    
    
parser = argparse.ArgumentParser(description='Manage OTG container for a testbed')
command_parser = parser.add_subparsers(title="command", dest="command", help="command to run", required=True)

start_parser = command_parser.add_parser("start", help="start OTG container")
start_parser.add_argument('testbed', help="testbed id")
start_parser.add_argument('--vxr_out', default='', help="path to vxr.out directory")
start_parser.add_argument('--image', default='', help="path to xr image")
start_parser.add_argument('--topo', default='', help="path to sim topology file")

# check if there are more args that can modify docker-compose file
start_parser.add_argument('--controller', help='Docker version number for image controller e.g. --controller=1.20.0-6', default='1.3.0-2')
start_parser.add_argument('--layer23', help='Docker version number image for image layer23 e.g. 1.20.0-1', default='1.3.0-4')
start_parser.add_argument('--gnmi', help='Docker version number image for gnmi e.g. 1.20.2', default='1.13.15')
# controller command options
start_parser.add_argument('--controller_command', help='Command line for controller e.g. --controller_command=[--grpc-max-msg-size 500]', nargs='*')

stop_parser = command_parser.add_parser("stop", help="stop OTG container")
stop_parser.add_argument('testbed', help="testbed id")

restart_parser = command_parser.add_parser("restart", help="restart OTG container")
restart_parser.add_argument('testbed', help="testbed id")

bindings_parser = command_parser.add_parser("bindings", help="generate Ondatra bindings")
bindings_parser.add_argument('testbed', help="testbed id")
bindings_parser.add_argument('--out_dir', default='', help="output directory")

logs_parser = command_parser.add_parser("logs", help="collect OTG container logs")
logs_parser.add_argument('testbed', help="testbed id")
logs_parser.add_argument('out_dir', help="output directory")

args = parser.parse_args()

if shutil.which(GO_BIN) is None:
    if os.path.exists(ALT_GO_BIN):
        GO_BIN = ALT_GO_BIN
    else:
        sys.exit(f"Go binary not found. Make sure `go` is in PATH")

testbed_id = args.testbed
command = args.command

if command == "start":
    controller_ver = getattr(args, 'controller', '1.3.0-2')
    _check_otg_version("controller",controller_ver)
    layer23_ver = getattr(args, 'layer23', '1.3.0-4')
    _check_otg_version("layer23",layer23_ver)
    gnmi_ver = getattr(args, 'gnmi', '1.13.15')
    _check_otg_version("gnmi",gnmi_ver)
    controller_command = getattr(args, 'controller_command', [])
else:
    controller_ver = getattr(args, 'controller', None)
    layer23_ver = getattr(args, 'layer23', None)
    gnmi_ver = getattr(args, 'gnmi', None)
    controller_command = getattr(args, 'controller_command', None)

fp_repo_dir = os.getenv('FP_REPO_DIR', os.getcwd())
reserved_testbed = _get_testbed_by_id(fp_repo_dir, testbed_id)
pname = reserved_testbed['id'].lower()

if not type(reserved_testbed['baseconf']) is dict:
    reserved_testbed['baseconf'] = {
        'dut': reserved_testbed['baseconf']
    }

if 'out_dir' in args and args.out_dir:
    out_dir = _resolve_path_if_needed(os.getcwd(), args.out_dir)
else:
    out_dir = _resolve_path_if_needed(os.getcwd(), f'{pname}_bindings')
    
if reserved_testbed.get('sim', False) and command in ["start", "restart"]:
    shutil.rmtree(out_dir, ignore_errors=True)
    os.makedirs(out_dir, exist_ok=True)
    
    baseconf_files = _generate_baseconf_files(fp_repo_dir, reserved_testbed, out_dir)
    
    vxr_out = args.vxr_out
    if not args.vxr_out:
        if not reserved_testbed.get('sim', False): 
            sys.exit(f"Testbed {pname} is not a sim")

        if os.path.exists(out_dir):
            try:
                check_output(f'/auto/vxr/pyvxr/latest/vxr.py stop', cwd=out_dir)
                check_output(f'/auto/vxr/pyvxr/latest/vxr.py clean', cwd=out_dir)
            except: pass

        topo_file = os.path.join(out_dir, 'topology.yaml')
        if args.topo:
            shutil.copy(args.topo, topo_file)
        else:
            if not args.image:
                sys.exit(f"Image not provided. Use --image option.")
            shutil.copy(_resolve_path_if_needed(fp_repo_dir, reserved_testbed['topology']), topo_file)
            _sim_edit_topo(topo_file, baseconf_files, args.image)
        
        if not os.path.exists(topo_file):
            sys.exit(f"Topology file not found")
        
        check_output(f'/auto/vxr/pyvxr/latest/vxr.py start {topo_file}', cwd=out_dir)
        vxr_out = os.path.join(out_dir, 'vxr.out')
        if not os.path.exists(vxr_out):
            sys.exit(f"Sim bringup failed")

    sim_port_redir = _sim_get_port_redir(vxr_out)
    if 'ate_gui' not in sim_port_redir:
        sys.exit("ATE not found in sim ports file")

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

    reserved_testbed['binding'] = os.path.join(out_dir, 'ate.binding')
    reserved_testbed['testbed'] = os.path.join(out_dir, 'dut.testbed')

    otg_binding_file = os.path.join(out_dir, 'otg.binding')
    setup_file = os.path.join(out_dir, 'setup.sh')

    generate_bindings(vxr_out, reserved_testbed['testbed'], reserved_testbed['binding'])
    
    _write_otg_binding(fp_repo_dir, reserved_testbed, baseconf_files, otg_binding_file)
    _write_setup_script(testbed_id, reserved_testbed['testbed'], reserved_testbed['binding'], otg_binding_file, setup_file)

else:
    if command in ["bindings", "logs"]:
        shutil.rmtree(out_dir, ignore_errors=True)
        os.makedirs(out_dir, exist_ok=True)

        if command == "bindings":
            baseconf_files = _generate_baseconf_files(fp_repo_dir, reserved_testbed, out_dir)
            otg_binding_file = os.path.join(out_dir, 'otg.binding')
            ate_binding_file = os.path.join(out_dir, 'ate.binding')
            testbed_file = os.path.join(out_dir, 'dut.testbed')
            setup_file = os.path.join(out_dir, 'setup.sh')

            _write_testbed_file(fp_repo_dir, reserved_testbed, testbed_file)
            _write_ate_binding(fp_repo_dir, reserved_testbed, baseconf_files, ate_binding_file)
            _write_otg_binding(fp_repo_dir, reserved_testbed, baseconf_files, otg_binding_file)
            _write_setup_script(testbed_id, testbed_file, ate_binding_file, otg_binding_file, setup_file)

        if command == "logs":
            kne_host = reserved_testbed['otg']['host']
            check_output(
                f'ssh -q -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null {kne_host} /auto/tftpboot-ottawa/b4/bin/otg_log_collector {pname} {out_dir}'
            )
    
with tempfile.NamedTemporaryFile(prefix='otg-docker-compose-', suffix='.yml') as f:
    kne_host = reserved_testbed['otg']['host']
    kne_port = reserved_testbed['otg'].get('port', 22)

    username = reserved_testbed['otg'].get('username')
    if username:
        kne_host = f"{username}@{kne_host}"
    
    password = reserved_testbed['otg'].get('password', '')
    if password:
        password = f"sshpass -p {password}"

    docker_compose_file_path = f.name
    docker_compose_file_name = os.path.basename(docker_compose_file_path)
    _write_otg_docker_compose_file(docker_compose_file_path, reserved_testbed, controller_ver, layer23_ver, gnmi_ver, controller_command)
    check_output(
        f'{password} scp -P {kne_port} -q -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null {docker_compose_file_path} {kne_host}:/tmp/{docker_compose_file_name}'
    )

    stop_cmd = f'{password} ssh -p {kne_port} -q -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null {kne_host} /usr/local/bin/docker-compose -p {pname} --file /tmp/{docker_compose_file_name} down'
    start_cmd = f'{password} ssh -p {kne_port} -q -q -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null {kne_host} /usr/local/bin/docker-compose -p {pname} --file /tmp/{docker_compose_file_name} up -d --force-recreate'

    if os.path.exists(out_dir):
        with open(os.path.join(out_dir, 'start_otg.sh'), 'w') as fp:
            fp.writelines([
                "#!/bin/sh\n",
                start_cmd + "\n"
            ])
        
        with open(os.path.join(out_dir, 'stop_otg.sh'), 'w') as fp:
            fp.writelines([
                "#!/bin/sh\n",
                stop_cmd + "\n"
            ])

    if command in ["stop", "restart", "sim"]:
        print(f"Stopping OTG")
        check_output(start_cmd)

    if command in ["start", "restart"]:
        print(f"Starting OTG")
        check_output(stop_cmd)
