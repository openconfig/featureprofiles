# RT-10.1: Default Route Generation based on 192.0.0.0/8 Presence

## Testbed Topology

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

### Test environment setup

*  Connect ATE Port-1 to DUT Port-1
*  Preconditions:
    * A BGP speaker DUT is configured.
    * BGP peering is established and stable between DUT and at least one neighbor ATE-1.
    * A policy to generate default route 0.0.0.0/0 upon receiving a bgp route 192.0.0.0/8
    * Initial routing tables are verified to be free of any 192.0.0.0/8 or 0.0.0.0/0 routes.
* Description of procedure to configure ATE and DUT with pre-requisites making
  it possible to cover the intended paths and RPCs.

### RT-10.1.1 - 192.0.0.0/8 and 0.0.0.0/0 route is present

* On ATE-1, advertise a bgp route 192.0.0.0/8 towards DUT.	
* On DUT, inspect the routing table.	
* Expected result: The 192.0.0.0/8 route is visible in DUT's routing table.
* Expected result: A default route (0.0.0.0/0) is now present in DUT.

### RT-10.1.2 Stop advertising 192.0.0.0/8 and observe 0.0.0.0/0 is removed
* On ATE-1, stop advertising the route 192.0.0.0/8.	
* Expected result: The 192.0.0.0/8 route is removed from DUT's routing table.
* Expected result: The default route (0.0.0.0/0) is withdrawn and is no longer present in DUT.

### Canonical OC 

TODO ([openconfig/public#1368](https://github.com/openconfig/public/issues/1368)): Add advertise-aggregate path to the OpenConfig public data models. 
`/routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/advertise-aggregate = `"LOCAL-AGG"`


```json
{
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "DEFAULT"
        },
        "name": "DEFAULT",
        "protocols": {
          "protocol": [
            {
              "bgp": {
                "neighbors": {
                  "neighbor": [
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "IPV4_UNICAST",
                            "apply-policy": {
                              "config": {
                                "default-import-policy": "REJECT_ROUTE"
                              }
                            },
                            "config": {
                              "afi-safi-name": "IPV4_UNICAST"
                            }
                          }
                        ]
                      },
                      "config": {
                        "neighbor-address": "192.1.1.1"
                      },
                      "neighbor-address": "192.1.1.1"
                    },
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "IPV6_UNICAST",
                            "apply-policy": {
                              "config": {
                                "default-import-policy": "REJECT_ROUTE"
                              }
                            },
                            "config": {
                              "afi-safi-name": "IPV6_UNICAST"
                            }
                          }
                        ]
                      },
                      "config": {
                        "neighbor-address": "2001:db9::1"
                      },
                      "neighbor-address": "2001:db9::1"
                    }
                  ]
                }
              },
              "config": {
                "identifier": "BGP",
                "name": "BGP"
              },
              "identifier": "BGP",
              "name": "BGP"
            },
            {
              "config": {
                "identifier": "LOCAL_AGGREGATE",
                "name": "DEFAULT-AGG"
              },
              "identifier": "LOCAL_AGGREGATE",
              "local-aggregates": {
                "aggregate": [
                  {
                    "config": {
                      "prefix": "0.0.0.0/0"
                    },
                    "prefix": "0.0.0.0/0"
                  }
                ]
              },
              "name": "DEFAULT-AGG"
            }
          ]
        }
      }
    ]
  },
  "routing-policy": {
    "defined-sets": {
      "prefix-sets": {
        "prefix-set": [
          {
            "config": {
              "mode": "IPV4",
              "name": "EBGP-IMPORT-IPV4"
            },
            "name": "EBGP-IMPORT-IPV4",
            "prefixes": {
              "prefix": [
                {
                  "config": {
                    "ip-prefix": "192.0.2.0/32",
                    "masklength-range": "exact"
                  },
                  "ip-prefix": "192.0.2.0/32",
                  "masklength-range": "exact"
                }
              ]
            }
          }
        ]
      }
    },
    "policy-definitions": {
      "policy-definition": [
        {
          "config": {
            "name": "EBGP-IMPORT-IPV4"
          },
          "name": "EBGP-IMPORT-IPV4",
          "statements": {
            "statement": [
              {
                "actions": {
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                },
                "conditions": {
                  "match-prefix-set": {
                    "config": {
                      "match-set-options": "INVERT",
                      "prefix-set": "EBGP-IMPORT-IPV4"
                    }
                  }
                },
                "config": {
                  "name": "10"
                },
                "name": "10"
              }
            ]
          }
        }
      ]
    }
  }
}

```

### OpenConfig Path and RPC Coverage
```yaml
paths:
  /interfaces/interface/config/description:
  /interfaces/interface/config/name:
  /interfaces/interface/config/type:
  /interfaces/interface/ethernet/config/port-speed:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/config/name:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/as:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/config/afi-safi-name:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/config/identifier:
  /network-instances/network-instance/protocols/protocol/config/name:
  /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode:
  /routing-policy/defined-sets/prefix-sets/prefix-set/config/name:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range:
  /routing-policy/policy-definitions/policy-definition/config/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set:
  /routing-policy/policy-definitions/policy-definition/statements/statement/config/name:
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```
