import sys
import subprocess
import venv

class IxiaEnv(venv.EnvBuilder):
    def __init__(self, env_name, *args, **kwargs):
        self.context = None
        super().__init__(with_pip=True, *args, **kwargs)
        super().create(env_name)

    def post_setup(self, context):
        self.context = context
        self.run_in_venv(['-m', 'pip', 'install', 'ixnetwork_restpy', 'protobuf'])

    def run_in_venv(self, command):
        command = [self.context.env_exe] + command
        subprocess.run(command)

try:
    from ixnetwork_restpy import TestPlatform
    from google.protobuf import text_format
    import binding_pb2

    print(f'Parsing binding file {sys.argv[1]}')
    with open(sys.argv[1], 'rb') as fp:
        binding = text_format.Parse(fp.read(), binding_pb2.Binding())
        for device in binding.ates:
            hostname = device.name
            ixiaNet = device.ixnetwork
            targetPorts = [p.name for p in device.ports]
            print(f'Checking ports: {targetPorts} on chassis {hostname}')

            if ixiaNet and ixiaNet.target:
                platform = TestPlatform(ixiaNet.target)
                if ixiaNet.username and ixiaNet.password:
                    platform.Authenticate(ixiaNet.username, ixiaNet.password)
                
                for session in platform.Sessions.find():
                    for port in session.Ixnetwork.Vport.find():
                        if not port.AssignedTo:
                             continue
                        chassis, card, port = port.AssignedTo.split(':')
                        pname = card + '/' + port
                        if chassis == hostname and pname in targetPorts:
                            print(f'Port {pname} assigned; releasing...')
                            port.ReleasePort()
except ModuleNotFoundError:
    ixiaVenv = IxiaEnv('ixia_venv')
    ixiaVenv.run_in_venv([__file__] + sys.argv[1:])