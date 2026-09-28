# RT-10.2: Non-default Route Generation based on 192.168.2.2/32 Presence in ISIS

## Testcase summary

### Testbed type 

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

### Topology

*  Connect ATE Port-1 to DUT Port-1

*  Preconditions:
    * A ISIS speaker DUT is configured.

    * ISIS peering is established and stable between DUT and at least one neighbor ATE-1.

    * A policy to generate default route 192.168.2.0/30 upon receiving a isis route 192.168.2.2/32

    * Initial routing tables are verified to be free of any 192.168.2.0/30 or 192.168.2.2/32 .




### Procedure for generate non-default route

### Scenario: 192.168.2.2/32 route is present		

1.1	On ATE-1, advertise a isis route 192.168.2.2/32 towards DUT.	

Expected result: The 192.168.2.2/32 route is visible in DUT's routing table.

1.2	Wait for isis to converge.	

Expected result: The isis session remains in the UP state.

1.3	On DUT, inspect the routing table.	

Expected result: A non-default route (192.168.2.0/30) is now generated in DUT.

1.4	On ATE-1, stop advertising the route 192.168.2.2/32.	

Expected result: The 192.168.2.2/32 route is removed from DUT's routing table.

1.5	Wait for isis to converge.	

Expected result: The isis session remains in the UP state.

1.6	On DUT, inspect the routing table.	

Expected result: The non-default route 192.168.2.0/30 is withdrawn and is no longer generated in DUT.



### Conclusion:

Pass: The test case passes if all expected results are achieved.

Fail: The test case fails if any of the following occur:

A non-default route is generated and when the 192.168.2.2/32 route is not present.
A non-default route is not generated and advertised when the 192.168.2.2/32 route is present.
The isis session drops or gets stuck in an intermediate state.

### Canonical OC 

TODO:

1. Add advertise-aggregate path to the OpenConfig public data models. https://github.com/openconfig/public/issues/1368
   ```
   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/advertise-aggregate = `"LOCAL-AGG"
   ```
2. Add  a new OC path for attaching a routing-policy to ISIS to match the route.
3. Add OC path for state verification as well.



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
              "config": {
                "identifier": "ISIS",
                "name": "DEFAULT-ISIS"
              },
              "identifier": "ISIS",
              "isis": {
                "global": {
                  "config": {
                    "net": [
                      "49.0001.0000.0000.0001.00"
                    ]
                  }
                },
                "interfaces": {
                  "interface": [
                    {
                      "config": {
                        "circuit-type": "POINT_TO_POINT",
                        "interface-id": "eth1.0"
                      },
                      "interface-id": "eth1.0",
                      "levels": {
                        "level": [
                          {
                            "afi-safi": {
                              "af": [
                                {
                                  "afi-name": "IPV4",
                                  "config": {
                                    "afi-name": "IPV4",
                                    "enabled": true,
                                    "safi-name": "UNICAST"
                                  },
                                  "safi-name": "UNICAST"
                                }
                              ]
                            },
                            "config": {
                              "enabled": true,
                              "level-number": 2
                            },
                            "level-number": 2
                          }
                        ]
                      }
                    }
                  ]
                },
                "levels": {
                  "level": [
                    {
                      "config": {
                        "enabled": true,
                        "level-number": 2
                      },
                      "level-number": 2
                    }
                  ]
                }
              },
              "name": "DEFAULT-ISIS"
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
                      "discard": true,
                      "prefix": "192.168.2.0/30"
                    },
                    "prefix": "192.168.2.0/30"
                  }
                ]
              },
              "name": "DEFAULT-AGG"
            }
          ]
        },
        "table-connections": {
          "table-connection": [
            {
              "address-family": "IPV4",
              "config": {
                "address-family": "IPV4",
                "default-import-policy": "REJECT_ROUTE",
                "dst-protocol": "ISIS",
                "import-policy": [
                  "SEND-DEF-IF-NOT-FOUND-IPV4"
                ],
                "src-protocol": "LOCAL_AGGREGATE"
              },
              "dst-protocol": "ISIS",
              "src-protocol": "LOCAL_AGGREGATE"
            }
          ]
        },
        "tables": {
          "table": [
            {
              "address-family": "IPV4",
              "config": {
                "address-family": "IPV4",
                "protocol": "ISIS"
              },
              "protocol": "ISIS"
            },
            {
              "address-family": "IPV4",
              "config": {
                "address-family": "IPV4",
                "protocol": "LOCAL_AGGREGATE"
              },
              "protocol": "LOCAL_AGGREGATE"
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
              "name": "SEND-DEF-IF-NOT-FOUND-IPV4"
            },
            "name": "SEND-DEF-IF-NOT-FOUND-IPV4",
            "prefixes": {
              "prefix": [
                {
                  "config": {
                    "ip-prefix": "192.168.2.2/32",
                    "masklength-range": "exact"
                  },
                  "ip-prefix": "192.168.2.2/32",
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
            "name": "SEND-DEF-IF-NOT-FOUND-IPV4"
          },
          "name": "SEND-DEF-IF-NOT-FOUND-IPV4",
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
                      "prefix-set": "SEND-DEF-IF-NOT-FOUND-IPV4"
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

## OpenConfig Path and RPC Coverage
```yaml
paths:
  /interfaces/interface/config/description:
  /interfaces/interface/config/name:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /network-instances/network-instance/protocols/protocol/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/global/afi-safi/af/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/global/config/instance:
  /network-instances/network-instance/protocols/protocol/isis/global/config/level-capability:
  /network-instances/network-instance/protocols/protocol/isis/global/config/net:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/config/circuit-type:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/interface-ref/config/interface:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/interface-ref/config/subinterface:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/adjacency-state:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/afi-safi/af/config/metric:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/config/metric-style:
  /network-instances/network-instance/table-connections/table-connection/config/address-family:
  /network-instances/network-instance/table-connections/table-connection/config/dst-protocol:
  /network-instances/network-instance/table-connections/table-connection/config/import-policy:
  /network-instances/network-instance/table-connections/table-connection/config/src-protocol:
  /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set:
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

FFF
