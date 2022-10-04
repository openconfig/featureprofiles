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
        print(command)
        return subprocess.check_call(command)

try:
    from ixnetwork_restpy import TestPlatform
    from google.protobuf import text_format
    import binding_pb2

    with open(sys.argv[1], 'rb') as fp:
        binding = text_format.Parse(fp.read(), binding_pb2.Binding())
        for device in binding.ates:
            ixia = device.ixnetwork
            if ixia and ixia.target:
                platform = TestPlatform(ixia.target)
                if ixia.username and ixia.password:
                    platform.Authenticate(ixia.username, ixia.password)
                
                vport = platform.Sessions.find() \
                    .Ixnetwork.Vport.find()
                vport.ReleasePort()
except ModuleNotFoundError:
    ixiaVenv = IxiaEnv('ixia_venv')
    ixiaVenv.run_in_venv([__file__] + sys.argv[1:])