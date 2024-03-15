from ixnetwork_restpy import SessionAssistant
from google.protobuf import text_format
import binding_pb2
import sys

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

            try:
                port_map = session_assistant.PortMapAssistant()
                for pname in targetPorts:
                    slot, port = pname.split('/')
                    port_map.Map(chassis, slot, port)
                port_map.Connect(ForceOwnership=True, HostReadyTimeout=20, LinkUpTimeout=60)
                port_map.Disconnect()
            except Exception as e: 
                print(e)
            finally: 
                session_assistant.Session.remove()