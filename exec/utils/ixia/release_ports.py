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
    from ixnetwork_restpy import SessionAssistant
    from google.protobuf import text_format
    import binding_pb2

    print(f'Parsing binding file {sys.argv[1]}')
    with open(sys.argv[1], 'rb') as fp:
        binding = text_format.Parse(fp.read(), binding_pb2.Binding())
        for device in binding.ates:
            chassis = device.name
            ixiaNet = device.ixnetwork
            targetPorts = [p.name for p in device.ports]
            print(f'Checking ports: {targetPorts} on chassis {chassis}')

            if ixiaNet and ixiaNet.target:
                ip, port = '127.0.0.1', 443
                username, password = 'admin', 'admin'

                if ':' in ixiaNet.target:
                    ip, port = ixiaNet.target.split(':')
                else:
                    ip = ixiaNet.target

                if ixiaNet.username:
                    username = ixiaNet.username
                if ixiaNet.password:
                    password = ixiaNet.password

                session_assistant = SessionAssistant(IpAddress=ip,
                    RestPort=port,
                    UserName=username,
                    Password=password,
                    LogLevel=SessionAssistant.LOGLEVEL_INFO, 
                    ClearConfig=True)

                port_map = session_assistant.PortMapAssistant()
                for pname in targetPorts:
                    slot, port = pname.split('/')
                    port_map.Map(chassis, slot, port)
                port_map.Connect(ForceOwnership=True)
                port_map.Disconnect()
                session_assistant.Session.remove()
except ModuleNotFoundError:
    ixiaVenv = IxiaEnv('ixia_venv')
    ixiaVenv.run_in_venv([__file__] + sys.argv[1:])
