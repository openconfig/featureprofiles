import subprocess
import platform
import tempfile
import argparse
import shutil
import yaml
import json
import os

GO_BIN = 'go'
TESTBEDS_FILE = 'exec/testbeds.yaml'

MTLS_DEFAULT_TRUST_BUNDLE_FILE = 'internal/cisco/security/cert/keys/CA/ca.cert.pem'
MTLS_DEFAULT_CERT_FILE = 'internal/cisco/security/cert/keys/clients/cafyauto.cert.pem'
MTLS_DEFAULT_KEY_FILE = 'internal/cisco/security/cert/keys/clients/cafyauto.key.pem'

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

def _otg_docker_compose_template(control_port, gnmi_port, rest_port):
    return f"""
version: "2.1"
services:
  controller:
    image: ghcr.io/open-traffic-generator/keng-controller:1.3.0-2
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
    environment:
      - LICENSE_SERVERS=10.85.70.247
    logging:
      driver: "local"
      options:
        max-size: "100m"
        max-file: "10"
        mode: "non-blocking"
  layer23-hw-server:
    image: ghcr.io/open-traffic-generator/keng-layer23-hw-server:1.3.0-4
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
    image: ghcr.io/open-traffic-generator/otg-gnmi-server:1.13.15
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
        fp.write(_otg_docker_compose_template(otg_info['controller_port'], otg_info['gnmi_port'], otg_info['rest_port']))

def _replace_binding_placeholders(fp_repo_dir, baseconf_files, binding_file):
    tb_file = _resolve_path_if_needed(fp_repo_dir, MTLS_DEFAULT_TRUST_BUNDLE_FILE)
    key_file = _resolve_path_if_needed(fp_repo_dir, MTLS_DEFAULT_KEY_FILE)
    cert_file = _resolve_path_if_needed(fp_repo_dir, MTLS_DEFAULT_CERT_FILE)
    with open(binding_file, 'r') as fp:
        data = fp.read()
    data = data.replace('$TRUST_BUNDLE_FILE', tb_file)
    data = data.replace('$CERT_FILE', cert_file)
    data = data.replace('$KEY_FILE', key_file)
    
    with open(binding_file, 'w') as fp:
        fp.write(data)
    
    for dut, baseconf_file in baseconf_files.items():
        check_output("sed -i 's|id: \"" + dut + "\"|id: \"" + dut + "\"\\nconfig:{\\ngnmi_set_file:\"" + baseconf_file + "\"\\n  }|g' " + binding_file)

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
            'timeout': 100
        }

        ate['gnmi'] = {
            'target': '{host}:{gnmi_port}'.format(host=otg_info['host'], gnmi_port=otg_info['gnmi_port']),
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

def _write_ate_binding(fp_repo_dir, reserved_testbed, baseconf_file, ate_binding_file):
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
    
parser = argparse.ArgumentParser(description='Manage OTG container for a testbed')
command_parser = parser.add_subparsers(title="command", dest="command", help="command to run", required=True)
start_parser = command_parser.add_parser("start", help="start OTG container")
start_parser.add_argument('testbed', help="testbed id")
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

testbed_id = args.testbed
command = args.command

fp_repo_dir = os.getenv('FP_REPO_DIR', os.getcwd())
reserved_testbed = _get_testbed_by_id(fp_repo_dir, testbed_id)
pname = reserved_testbed['id'].lower()

if not type(reserved_testbed['baseconf']) is dict:
    reserved_testbed['baseconf'] = {
        'dut': reserved_testbed['baseconf']
    }

if command in ["bindings", "logs"]:
    if args.out_dir:
        out_dir = _resolve_path_if_needed(os.getcwd(), args.out_dir)
    else:
        out_dir = _resolve_path_if_needed(os.getcwd(), f'{pname}_bindings')

    os.makedirs(out_dir, exist_ok=True)
    
    if command == "bindings":
        otg_binding_file = os.path.join(out_dir, 'otg.binding')
        ate_binding_file = os.path.join(out_dir, 'ate.binding')
        testbed_file = os.path.join(out_dir, 'dut.testbed')
        setup_file = os.path.join(out_dir, 'setup.sh')
        
        baseconf_files = {}
        for dut, conf in reserved_testbed['baseconf'].items():
            baseconf_file = os.path.join(out_dir, f'{dut}.baseconf')
            _write_baseconf_file(fp_repo_dir, conf, baseconf_file)
            baseconf_files[dut] = baseconf_file


        _write_testbed_file(fp_repo_dir, reserved_testbed, testbed_file)
        _write_ate_binding(fp_repo_dir, reserved_testbed, baseconf_files, ate_binding_file)
        _write_otg_binding(fp_repo_dir, reserved_testbed, baseconf_files, otg_binding_file)
        _write_setup_script(testbed_id, testbed_file, ate_binding_file, otg_binding_file, setup_file)
        print('You can run the following command to setup your enviroment:')
        print(f'source {setup_file}')
        
    if command == "logs":
        kne_host = reserved_testbed['otg']['host']
        check_output(
            f'ssh -q -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null {kne_host} /auto/tftpboot-ottawa/b4/bin/otg_log_collector {pname} {out_dir}'
        )
 
with tempfile.NamedTemporaryFile(prefix='otg-docker-compose-', suffix='.yml') as f:
    kne_host = reserved_testbed['otg']['host']
    docker_compose_file_path = f.name
    docker_compose_file_name = os.path.basename(docker_compose_file_path)
    _write_otg_docker_compose_file(docker_compose_file_path, reserved_testbed)
    check_output(
        f'scp -q -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null {docker_compose_file_path} {kne_host}:/tmp/{docker_compose_file_name}'
    )

    if command in ["stop", "restart"]:
        kne_host = reserved_testbed['otg']['host']
        check_output(
            f'ssh -q -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null {kne_host} /usr/local/bin/docker-compose -p {pname} --file /tmp/{docker_compose_file_name} down'
        )

    if command in ["start", "restart"]:
        check_output(
            f'ssh -q -q -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null {kne_host} /usr/local/bin/docker-compose -p {pname} --file /tmp/{docker_compose_file_name} up -d --force-recreate'
        )
