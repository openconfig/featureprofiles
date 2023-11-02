import subprocess
import tempfile
import argparse
import yaml
import json
import os

GO_BIN = 'go'
TESTBEDS_FILE = 'exec/testbeds.yaml'

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

def _otg_docker_compose_template(control_port, gnmi_port):
    return f"""
version: "2"
services:
  controller:
    image: ghcr.io/open-traffic-generator/keng-controller:0.1.0-3
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
    image: ghcr.io/open-traffic-generator/keng-layer23-hw-server:0.13.0-2
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
    image: ghcr.io/open-traffic-generator/otg-gnmi-server:1.13.0
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

def _write_otg_binding(fp_repo_dir, reserved_testbed, otg_binding_file):
    otg_info = reserved_testbed['otg']

    # convert binding to json
    with tempfile.NamedTemporaryFile() as of:
        outFile = of.name
        cmd = f'{GO_BIN} run ' \
            f'./exec/utils/binding/tojson ' \
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
            'timeout': 30
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
            f'./exec/utils/binding/fromjson ' \
            f'-binding {tmp_binding_file} ' \
            f'-out {otg_binding_file}'

        check_output(cmd, cwd=fp_repo_dir)

parser = argparse.ArgumentParser(description='Manage OTG container for a testbed')
command_parser = parser.add_subparsers(title="command", dest="command", help="command to run", required=True)
start_parser = command_parser.add_parser("start", help="start OTG container")
start_parser.add_argument('testbed', help="testbed id")
stop_parser = command_parser.add_parser("stop", help="stop OTG container")
stop_parser.add_argument('testbed', help="testbed id")
binding_parser = command_parser.add_parser("binding", help="generate OTG binding")
binding_parser.add_argument('testbed', help="testbed id")
binding_parser.add_argument('out_file', help="output file")
args = parser.parse_args()

testbed_id = args.testbed
command = args.command

fp_repo_dir = os.getenv('FP_REPO_DIR', '.')
reserved_testbed = _get_testbed_by_id(fp_repo_dir, testbed_id)
pname = reserved_testbed['id'].lower()

with tempfile.NamedTemporaryFile(prefix='otg-docker-compose-', suffix='.yml') as f:
    kne_host = reserved_testbed['otg']['host']

    if command == "start":
        docker_compose_file_path = f.name
        docker_compose_file_name = os.path.basename(docker_compose_file_path)
        _write_otg_docker_compose_file(docker_compose_file_path, reserved_testbed)
        check_output(
            f'scp -q -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null {docker_compose_file_path} {kne_host}:/tmp/{docker_compose_file_name}'
        )
        check_output(
            f'ssh -q -q -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null {kne_host} /usr/local/bin/docker-compose -p {pname} --file /tmp/{docker_compose_file_name} up -d --force-recreate'
        )

    elif command == "stop":
        check_output(
            f'ssh -q -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null {kne_host} /usr/local/bin/docker-compose -p {pname} down'
        )
        
    elif command == "binding":
        out_file = _resolve_path_if_needed(os.getcwd(), args.out_file)
        _write_otg_binding(FP_REPO_DIR, reserved_testbed, out_file)