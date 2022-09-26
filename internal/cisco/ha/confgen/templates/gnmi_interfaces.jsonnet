local utils = import '../utils/utils.jsonnet';

local bundle_ids = std.range(121, 127) + [225];
local subinterfaces_index = std.range(100, 196);

local create_subinterface(bundle, index, vlan=false) =
  {
    local subi = self,
    index: index,
    config: {
      index: $.index,
    },
    'openconfig-if-ip:ipv4': {
      addresses: {
        address: [
          {
            local addr = self,
            ip: '100.' + bundle + '.' + subi.index + '.1',
            config: {
              ip: addr.ip,
              'prefix-length': 30,
            },
          },
        ],
      },
    },
    'openconfig-if-ip:ipv6': {
      addresses: {
        address: [
          {
            local addr = self,
            ip: '4001:' + subi.index + ':' + bundle + ':1:1',
            config: {
              ip: addr.ip,
              'prefix-length': 126,
            },
          },
        ],
      },
    },
    'openconfig-vlan:vlan': {
      match: {
        'single-tagged': {
          config: {
            'vlan-id': index,
          },
        },
      },
    },
  };

local create_bundle(id) =
  {
    name: 'Bundle-Ether' + id,
    config: {
      name: $.name,
      type: 'iana-if-type:ieee8023adLag',
    },
    subinterfaces: {
      subinterface: [
        create_subinterface(id, index)
        for index in subinterfaces_index
      ],
    },
  };


local generated = {
  'openconfig-interfaces:interfaces': {
    interface+: [create_bundle(id) for id in bundle_ids],
  },
};

local base = import 'gnmi_interfaces.json';
utils.mergePatch(base, generated)
