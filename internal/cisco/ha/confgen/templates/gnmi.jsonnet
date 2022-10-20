local utils = import '../utils/utils.jsonnet';

local bundleConf = std.extVar('bundles');

local create_subinterface(bundleId, index) =
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
            ip: '100.' + bundleId + '.' + subi.index + '.1',
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
            ip: '4001:' + subi.index + '::' + bundleId + ':1:1',
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

local create_bundle(bundle) =
  {
    name: 'Bundle-Ether' + bundle.Id,
    config: {
      name: $.name,
      type: 'iana-if-type:ieee8023adLag',
    },
  } + if bundle.SubInterfaceRange != null then {
    subinterfaces: {
      subinterface: [
        create_subinterface(bundle.Id, index)
        for index in std.range(bundle.SubInterfaceRange[0], bundle.SubInterfaceRange[1])
      ],
    },
  } else {};

local map_interface(bundleId, interface) = {
  name: interface,
  config: {
    name: $.name,
    type: 'iana-if-type:ethernetCsmacd',
  },
  'openconfig-if-ethernet:ethernet': {
    config: {
      'auto-negotiate': false,
      'openconfig-if-aggregate:aggregate-id': 'Bundle-Ether' + bundleId,
    },
  },
};

local create_afi_safi(afi, safi) =
  {
    'afi-name': afi,
    'safi-name': safi,
    config: {
      'afi-name': afi,
      'safi-name': safi,
      enabled: true,
    },
  };

local create_afi_safi_compact(afiSafi) =
  {
    'afi-safi-name': afiSafi,
    config: {
      'afi-safi-name': afiSafi,
      enabled: true,
    },
  };

local create_isis_interface(iface, subiface=null) =
  {
    local interfaceId = if subiface != null then iface.name + '.' + subiface.index else iface.name,
    'interface-id': interfaceId,
    config: {
      'interface-id': interfaceId,
      enabled: true,
      passive: false,
      'hello-padding': 'LOOSE',
      'circuit-type': 'POINT_TO_POINT',
    },
    'interface-ref': {
      config: {
        interface: iface.name,
      } + if subiface != null then {
        subinterface: subiface.index,
      } else {},
    },
    timers: {
      config: {
        'csnp-interval': 5,
      },
    },
    'afi-safi': {
      af: [
        create_afi_safi('openconfig-isis-types:IPV4', 'openconfig-isis-types:UNICAST'),
        create_afi_safi('openconfig-isis-types:IPV6', 'openconfig-isis-types:UNICAST'),
      ],
    },
  };

local create_isis(bundles) =
  {
    identifier: 'openconfig-policy-types:ISIS',
    name: 'B4',
    config: {
      identifier: $.identifier,
      name: $.name,
    },
    isis: {
      global: {
        config: {
          'level-capability': 'LEVEL_2',
          net: [
            '47.0001.0000.0000.0001.00',
          ],
        },
        nsr: {
          config: {
            enabled: true,
          },
        },
        timers: {
          config: {
            'lsp-refresh-interval': 10,
            'lsp-lifetime-interval': 60,
          },
          spf: {
            config: {
              'spf-hold-interval': '6000',
              'spf-first-interval': '60',
            },
          },
        },
        'afi-safi': {
          af: [
            create_afi_safi('openconfig-isis-types:IPV4', 'openconfig-isis-types:UNICAST')
            + {
              config: {
                metric: 10,
              },
            },
            create_afi_safi('openconfig-isis-types:IPV4', 'openconfig-isis-types:MULTICAST'),
            create_afi_safi('openconfig-isis-types:IPV6', 'openconfig-isis-types:UNICAST') + {
              config: {
                metric: 10,
              },
              'multi-topology': {
                config: {
                  'afi-name': 'openconfig-isis-types:IPV6',
                  'safi-name': 'openconfig-isis-types:UNICAST',
                },
              },
            },
            create_afi_safi('openconfig-isis-types:IPV6', 'openconfig-isis-types:MULTICAST'),
          ],
        },
      },
      interfaces: {
        interface: [
          create_isis_interface(iface)
          for iface in bundles
        ] + [
          create_isis_interface(iface, subiface)
          for iface in bundles
          if std.objectHas(iface, 'subinterfaces')
          for subiface in iface.subinterfaces.subinterface
        ],
      },
    },
  };

local create_bgp_neighbor(bundleId, subifaceIndex) = {
  'neighbor-address': '100.' + bundleId + '.' + subifaceIndex + '.2',
  config: {
    'neighbor-address': '100.' + bundleId + '.' + subifaceIndex + '.2',
    'peer-group': 'B4-EBGP',
  },
};

local create_bgp(bundles) = {
  identifier: 'openconfig-policy-types:BGP',
  name: 'default',
  config: {
    identifier: 'openconfig-policy-types:BGP',
    name: 'default',
  },
  bgp: {
    global: {
      config: {
        as: 66001,
        'router-id': '1.1.1.1',
      },
      'graceful-restart': {
        config: {
          enabled: true,
        },
      },
      'afi-safis': {
        'afi-safi': [
          create_afi_safi_compact('openconfig-bgp-types:IPV4_UNICAST'),
          create_afi_safi_compact('openconfig-bgp-types:IPV6_UNICAST')
        ],
      },
    },
    'peer-groups': {
      'peer-group': [
        {
          'peer-group-name': 'B4-EBGP',
          config: {
            'peer-group-name': 'B4-EBGP',
            'peer-as': 64001,
            'local-as': 63001,
          },
          'ebgp-multihop': {
            config: {
              enabled: true,
              'multihop-ttl': 255,
            },
          },
          timers: {
            config: {
              'keepalive-interval': '10',
              'hold-time': '40',
            },
          },
          'graceful-restart': {
            config: {
              'restart-time': 10,
              'stale-routes-time': '15',
            },
          },
          'afi-safis': {
            'afi-safi': [
              create_afi_safi_compact('openconfig-bgp-types:IPV4_UNICAST') +
              {
                'ipv4-unicast': {
                  'prefix-limit': {
                    config: {
                      'max-prefixes': 100000,
                      'warning-threshold-pct': 75,
                      'prevent-teardown': false,
                    },
                  },
                },
                'apply-policy': {
                  config: {
                    'import-policy': [
                      'ALLOW',
                    ],
                    'export-policy': [
                      'ALLOW',
                    ],
                  },
                },
              },
              create_afi_safi_compact('openconfig-bgp-types:IPV6_UNICAST') +
              {
                'ipv6-unicast': {
                  'prefix-limit': {
                    config: {
                      'max-prefixes': 100000,
                      'warning-threshold-pct': 75,
                      'prevent-teardown': false,
                    },
                  },
                },
                'apply-policy': {
                  config: {
                    'import-policy': [
                      'ALLOW',
                    ],
                    'export-policy': [
                      'ALLOW',
                    ],
                  },
                },
              },
            ],
          },
        },
      ],
    },
    neighbors: {
      neighbor: [
        create_bgp_neighbor(bundle.Id, index)
        for bundle in bundleConf
        if bundle.SubInterfaceRange != null
        for index in std.range(bundle.SubInterfaceRange[0], bundle.SubInterfaceRange[1])
      ],
    },
  },
};

local bundles = [create_bundle(bundle) for bundle in bundleConf];

{
  'openconfig-interfaces:interfaces': {
    interface: bundles +
               [
                 map_interface(bundle.Id, interface)
                 for bundle in bundleConf
                 if bundle.Interfaces != null
                 for interface in bundle.Interfaces
               ],
  },
  'openconfig-network-instance:network-instances': {
    'network-instance': [
      {
        name: 'default',
        protocols: {
          protocol: [
            create_isis(bundles),
            create_bgp(bundles),
          ],
        },
      },
    ],
  },
}
