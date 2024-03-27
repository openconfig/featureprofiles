from enum import Enum
import argparse
import yaml
import os
import re

class Device():
    def __init__(self, id, username, password, ports_map):
        self.id = id
        self.username = username
        self.password = password
        self.ports_map = ports_map
        self.data_ports = {}

    def get_id(self):
        return self.id
    
    def get_username(self):
        return self.username
    
    def add_data_port(self, name):
        self.data_ports[name] = 'port' + str(len(self.data_ports) + 1)

    def has_data_port(self, name):
        return name in self.data_ports

    def get_data_port_abstract_name(self, p):
        return self.data_ports[p]
    
    def get_port_redir(self, p):
        return self.ports_map[self.id]['xr_redir' + str(p)]

    def get_host_agent(self):
        return self.ports_map[self.id]['HostAgent']

    def to_testbed_entry(self):
        return {
            'id': self.id,
            'ports': [{'id': p} for p in self.data_ports.values()]
        }
        
    def to_binding_entry(self):
        return {
            'id': self.id,
            'ports': [{'id': v, 'name': k} for k, v in self.data_ports.items()]
        }
        
class DUT(Device):
    class Vendor(Enum):
        CISCO = 1

    def __init__(self, id, platform, hostname, username, password, base_conf, certs_dir, ports_map):
        super().__init__(id, username, password, ports_map)
        self.platform = platform
        self.hostname = hostname
        self.mtls = False
        self.insecure = False
        self.grpc_port = 57400
        if base_conf: self._parse_baseconf(base_conf)
        if certs_dir: self._get_certificates(certs_dir)

    def _parse_baseconf(self, base_conf):
        with open(base_conf, 'r') as fp:
            lines = fp.readlines()
        
        # skip to grpc
        i = 0
        for l in lines:
            if l.startswith('grpc'): break
            i+=1
        lines = lines[i:]
        
        for l in lines:
            if l.startswith('!'): break
            if 'tls' in l: self.insecure = False
            if 'no-tls' in l: self.insecure = True
            if 'tls-mutual' in l: self.mtls = True

            if 'port' in l: 
                for p in re.compile(r'[\d]+').finditer(l):
                    self.grpc_port = int(p.group(0))
             
    def _get_certificates(self, certs_dir):
        self.cert_file = os.path.join(certs_dir, self.get_id(), self.get_user() + '.cert.pem')
        self.key_file = os.path.join(certs_dir, self.get_id(), self.get_user() + '.key.pem')
        self.trust_bundle_file = os.path.join(certs_dir, self.get_id(), 'ca.cert.pem')

    def get_model(self):
        if platform == 'spitfire_d':
            return 'CISCO-8808'
        return 'CISCO-8201'

    def to_binding_entry(self):
        e = super().to_binding_entry()  
        e.update({
            'name': self.hostname,
            'vendor': DUT.Vendor.CISCO,
            'hardware_model': self.get_model(),
            'options': {
                'username': self.username,
                'password': self.password,
                'insecure': self.insecure,
                'skip_verify': not self.mtls,
            },
            'ssh': {
                'target': self.get_host_agent() + ':' + str(self.get_port_redir(22)),
            }
        })
        
        if self.mtls:
            e['options'].update({
                'mutual_tls': self.mtls,
                'trust_bundle_file': self.trust_bundle_file,
                'cert_file': self.cert_file,
                'key_file': self.key_file,
            })

        for s in ['gnmi', 'gnoi', 'gribi', 'gnsi', 'p4rt']:
            e[s] = {
                 'target': self.get_host_agent() + ':' + str(self.get_port_redir(self.grpc_port)),
            }
        return e

class ATE(Device):
    def __init__(self, id, ports_map):
        super().__init__(id, 'admin', 'admin', ports_map)
          
class IxiaWeb(ATE):
    def __init__(self, id, ports_map):
        super().__init__(id, ports_map)

    def get_port_redir(self, p):
        return self.ports_map[self.id + '_gui']['redir' + str(p)]

    def get_host_agent(self):
        return self.ports_map[self.id + '_gui']['HostAgent']
    
    def to_binding_entry(self):
        e = super().to_binding_entry()
        e.update({
            'name': '20.0.0.2',
            'ixnetwork': {
                'target': self.get_host_agent() + ':' + str(self.get_port_redir(443)),
                'username': self.username,
                'password': self.password,
                'skip_verify': True
            }
        })
        return e

class IxiaOTG(ATE):
    def __init__(self, id, ports_map):
        super().__init__(id, ports_map)
    
    def to_binding_entry(self):
        e = super().to_binding_entry()
        e.update({
            'name': self.id,
            'options': {
                'username': self.username,
                'password': self.password,
            },
            'gnmi': {
                'target': self.get_host_agent() + ':' + str(self.get_port_redir(31003)),
                'skip_verify': True,
                'timeout': 60
            },
            'otg': {
                'target': self.get_host_agent() + ':' + str(self.get_port_redir(31002)),
                'insecure': True,
                'timeout': 100
            }
        })
        return e
        
class ConnectionEnd():
    def __init__(self, device, port):
        device.add_data_port(port)
        self.device = device
        self.port = port

    def get_device(self):
        return self.device
    
    def get_port(self):
        return self.port

class Connection():
    def __init__(self, a, b):
        self.a = a
        self.b = b
    
    def to_testbed_entry(self):
        return {
            'a': self.a.get_device().get_id() + ':' + self.a.get_device().get_data_port_abstract_name(self.a.get_port()),
            'b': self.b.get_device().get_id() + ':' + self.b.get_device().get_data_port_abstract_name(self.b.get_port())
        }

class ProtoPrinter():
    def __init__(self):
        self._ind = 0
    
    def _indent(self):
        self._ind += 2
        return self._insert_indent()
    
    def _deindent(self):
        self._ind -= 2
        return self._insert_indent()

    def _insert_indent(self):
        return ' ' * self._ind
    
    def _to_proto_val(self, v):
        if isinstance(v, str): return '"' + str(v) + '"'
        if isinstance(v, bool): return str(v).lower()
        if isinstance(v, Enum): return v.name
        return str(v)

    def _to_dict_entry(self, k, v):
        self._indent()
        vproto = self._to_proto_generic(k, v).split('\n')
        vproto = [self._insert_indent() + l + '\n' for l in vproto]
        return str(k) + ': ' + '{\n' + "".join(vproto) + self._deindent() + '\n}\n'
            
    def _to_proto_list(self, k, l):
        s = ""
        for v in l:
            s += self._to_dict_entry(k, v)
        return s
     
    def _to_proto_generic(self, k, v):
        if isinstance(v, list): return self._to_proto_list(k, v)
        if isinstance(v, dict): return self.dict_to_proto(v)
        return self._to_proto_val(v)

    def dict_to_proto(self, d):
        s = ""
        for k, v in d.items():
            if isinstance(v, list):
                s += self._to_proto_generic(k, v)
            elif isinstance(v, dict):
                s += self._to_dict_entry(k, v)
            else:
                s += str(k) + ': ' + self._to_proto_generic(k, v) + '\n'
        return s.strip()
    
def parse_connection_end(devices, c):
    parts = c.split('.')
    return ConnectionEnd(devices[parts[0]], ''.join(parts[1:]))

def is_otg(e):
    d = e.get('disks', [])
    if len(d) == 1 and isinstance(d[0], dict):
        return d[0].get('hda_ref', {}).get('file', '') == '/auto/vxr/vxr_images/ixia/ixia-c.qcow2'
    return False
    
parser = argparse.ArgumentParser(description='Generate Ondatra bindings for PyVXR')
parser.add_argument('vxr_out_dir', help="path to PyVXR vxr.out directory")
parser.add_argument('testbed_file', help="output testbed file")
parser.add_argument('binding_file', help="output binding file")
parser.add_argument('-certs_dir', default=None, help="certificates directory for mTLS")
args = parser.parse_args()

vxr_conf_file = os.path.join(args.vxr_out_dir, 'sim-config.yaml')
vxr_ports_file = os.path.join(args.vxr_out_dir, 'sim-ports.yaml')

with open(vxr_conf_file, "r") as fp:
    vxr_conf = yaml.safe_load(fp)

with open(vxr_ports_file, "r") as fp:
    vxr_ports = yaml.safe_load(fp)
    
devices = {}
connections = []

for name, entry in vxr_conf.get('devices', {}).items():
    platform = entry.get('platform', '')
    if platform in ['spitfire_f', 'spitfire_d']:
        d = DUT(name, platform, entry.get('xr_hostname'), entry.get('xr_username'), 
                entry.get('xr_password'), entry.get('cvac', None), args.certs_dir, vxr_ports)
    elif platform == 'ixia':
        d = IxiaWeb(name, vxr_ports)
    elif is_otg(entry):
        d = IxiaOTG(name, vxr_ports)
    else: continue

    devices[name] = d

for name, entry in vxr_conf.get('connections', {}).get('hubs', {}).items():
    if len(entry) < 2:
        continue
    connections.append(Connection(parse_connection_end(devices, entry[0]), 
                                    parse_connection_end(devices, entry[1])))

for name, entry in vxr_conf.get('devices', {}).items():
    for p in entry.get('data_ports', []):
        if not devices[name].has_data_port(p):
            devices[name].add_data_port(p)
        
testbed = {
    'duts': [d.to_testbed_entry() for d in devices.values() if isinstance(d, DUT)],
    'ates': [d.to_testbed_entry() for d in devices.values() if isinstance(d, ATE)],
    'links': [c.to_testbed_entry() for c in connections]
}

binding = {
    'duts': [d.to_binding_entry() for d in devices.values() if isinstance(d, DUT)],
    'ates': [d.to_binding_entry() for d in devices.values() if isinstance(d, ATE)],
}

with open(args.testbed_file, "w") as fp:
    fp.write(ProtoPrinter().dict_to_proto(testbed))
    
with open(args.binding_file, "w") as fp:
    fp.write(ProtoPrinter().dict_to_proto(binding))
