import sys
import subprocess
import venv

class ConfParserEnv(venv.EnvBuilder):
    def __init__(self, env_name, *args, **kwargs):
        self.context = None
        super().__init__(with_pip=True, *args, **kwargs)
        super().create(env_name)

    def post_setup(self, context):
        self.context = context
        self.run_in_venv(['-m', 'pip', 'install', 'ciscoconfparse'])

    def run_in_venv(self, command):
        command = [self.context.env_exe] + command
        subprocess.run(command)

try:
    from ciscoconfparse import CiscoConfParse
    for conf_file in sys.argv[1:]:
        print(f'Processing conf file {conf_file}')
        parser = CiscoConfParse(conf_file, syntax='ios')
        parser.insert_after(r'grpc', 'tls_mutual\n', new_val_indent=1)
        parser.insert_after(r'grpc', 'certificate-authentication\n', new_val_indent=1)
        parser.save_as(conf_file)
except ModuleNotFoundError:
    ixiaVenv = ConfParserEnv('confparser_venv')
    ixiaVenv.run_in_venv([__file__] + sys.argv[1:])
