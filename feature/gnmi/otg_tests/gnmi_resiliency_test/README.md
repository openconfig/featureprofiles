# gNMI-1.7: gNMI Resiliency Test

## Summary

This test verifies the performance, reliability, and resilience of `gNMI.Set` operations on a modular multi-linecard chassis under heavy telemetry load during a Linecard (LC) soft Online Insertion and Removal (OIR). 

gNMI Set operations replacing configuration trees can time out (domain convergence failure) when the gNMI agent is busy serving periodic `gNMI.Get` requests and continuous `gNMI.Subscribe` streams while processing internal state changes triggered by resetting multiple Linecards. This test ensures that a device can successfully commit and acknowledge a `gNMI.Set` request within the controller timeout limit (typically 10 minutes) under these conditions.

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_8.testbed

## Procedure

### Test environment setup

1. Connect DUT port 1 through 8 to ATE port 1 through 8 and ensure ports are spread equally across multiple linecards
1. Assign IP Addresses to the DUT and ATE ports 1 through 8
1. Start traffiic from ATE ports 1 through 8, the traffic should continue to be generated through out the test
1. Establish a `gNMI.Subscribe` STREAM SAMPLE session with `sample_interval` of 10 seconds for the `/interfaces/interface/state/counters` container for DUT ports 1 through 8
1. Establish a `gNMI.Subscribe` ON_CHANGE session for the `/components/component/state/oper-status` leaf
1. Establish a concurrent background goroutine that issues a `gNMI.Get` request with `data_type=CONFIG` for the entire tree every 60 seconds.

### gNMI-1.7.1 - Verify gNMI Set completes successfully during LC soft OIR under load

1. Generate a configuration that applies an extensive update across multiple interfaces to simulate a production-grade configuration replace, guideline below:
    1. Configuring 8 Interfaces (Ethernet1/1 to Ethernet1/8):
       - Configure description and enable the interface.
       - Enable IPv4 and set static IP `10.1.i.1/24` for `i` from 1 to 8.
    
    1. Create 1000-Statement ACL:
       - Set up an IPv4 ACL Set named `"DenySubnets"`.
       - Using a loop, populate `1000` sequential ACL entries (IDs `1` to `1000`), each configured with a forwarding action of     `oc.Acl_FORWARDING_ACTION_DROP` and dropping traffic originating from subnet `110.x.y.0/24` (where octet `x` runs from 1 to 4 and octet `y` runs dynamically modulo 256).
       - Append a final `permit any any` entry at ID `1001` using the forwarding action `oc.Acl_FORWARDING_ACTION_ACCEPT` matching source and     destination `0.0.0.0/0`.
    
    1. Apply ACL to 8 Interfaces:
       - For each of the 8 interfaces, attach the `"DenySubnets"` ACL as an Ingress ACL Set.
       - Define the InterfaceRef referencing the interface name and subinterface 0.
    
    1. Configure BGP and ISIS:
       - Retrieve/create the default Network Instance `default` with type `oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE`.
       - Enable BGP under network instance routing protocols:
         - Establish Router ID `10.1.1.1` and local AS `65001`.
         - Configures IPv4 Unicast AFI/SAFI globally.
         - For each interface, configure a static BGP Neighbor `10.1.i.2` under peer AS `65002` and enable IPv4 Unicast.
       - Enable ISIS under network instance routing protocols:
         - Establish instance name `"DEFAULT"` and typical network entity title `[]string{"49.0001.0000.0000.0001.00"}`.
         - Enable IPv4 Unicast AFI/SAFI globally.
         - Enable ISIS on each interface `Ethernet1/i` in point-to-point mode (`oc.Isis_CircuitType_POINT_TO_POINT`).
         - Enable IPv4 Unicast AFI/SAFI on the ISIS interface.
         - Enable Level 2 on the ISIS interface and assign metric `10`.

#### Canonical OC

```json
{
  "acl": {
    "acl-sets": {
      "acl-set": [
        {
          "acl-entries": {
            "acl-entry": [
              {
                "actions": {
                  "config": {
                    "forwarding-action": "DROP"
                  }
                },
                "config": {
                  "sequence-id": 1
                },
                "ipv4": {
                  "config": {
                    "source-address": "110.1.0.0/24"
                  }
                },
                "sequence-id": 1
              },
              {
                "actions": {
                  "config": {
                    "forwarding-action": "ACCEPT"
                  }
                },
                "config": {
                  "sequence-id": 10
                },
                "ipv4": {
                  "config": {
                    "destination-address": "0.0.0.0/0",
                    "source-address": "0.0.0.0/0"
                  }
                },
                "sequence-id": 10
              },
              {
                "actions": {
                  "config": {
                    "forwarding-action": "DROP"
                  }
                },
                "config": {
                  "sequence-id": 2
                },
                "ipv4": {
                  "config": {
                    "source-address": "110.1.1.0/24"
                  }
                },
                "sequence-id": 2
              },
              {
                "actions": {
                  "config": {
                    "forwarding-action": "DROP"
                  }
                },
                "config": {
                  "sequence-id": 3
                },
                "ipv4": {
                  "config": {
                    "source-address": "110.1.2.0/24"
                  }
                },
                "sequence-id": 3
              },
              {
                "actions": {
                  "config": {
                    "forwarding-action": "DROP"
                  }
                },
                "config": {
                  "sequence-id": 4
                },
                "ipv4": {
                  "config": {
                    "source-address": "110.1.3.0/24"
                  }
                },
                "sequence-id": 4
              },
              {
                "actions": {
                  "config": {
                    "forwarding-action": "DROP"
                  }
                },
                "config": {
                  "sequence-id": 5
                },
                "ipv4": {
                  "config": {
                    "source-address": "110.1.4.0/24"
                  }
                },
                "sequence-id": 5
              },
              {
                "actions": {
                  "config": {
                    "forwarding-action": "DROP"
                  }
                },
                "config": {
                  "sequence-id": 6
                },
                "ipv4": {
                  "config": {
                    "source-address": "110.1.5.0/24"
                  }
                },
                "sequence-id": 6
              },
              {
                "actions": {
                  "config": {
                    "forwarding-action": "DROP"
                  }
                },
                "config": {
                  "sequence-id": 7
                },
                "ipv4": {
                  "config": {
                    "source-address": "110.1.6.0/24"
                  }
                },
                "sequence-id": 7
              },
              {
                "actions": {
                  "config": {
                    "forwarding-action": "DROP"
                  }
                },
                "config": {
                  "sequence-id": 8
                },
                "ipv4": {
                  "config": {
                    "source-address": "110.1.7.0/24"
                  }
                },
                "sequence-id": 8
              },
              {
                "actions": {
                  "config": {
                    "forwarding-action": "DROP"
                  }
                },
                "config": {
                  "sequence-id": 9
                },
                "ipv4": {
                  "config": {
                    "source-address": "110.1.8.0/24"
                  }
                },
                "sequence-id": 9
              }
            ]
          },
          "config": {
            "name": "DenySubnets",
            "type": "ACL_IPV4"
          },
          "name": "DenySubnets",
          "type": "ACL_IPV4"
        }
      ]
    },
    "interfaces": {
      "interface": [
        {
          "config": {
            "id": "Ethernet1/1"
          },
          "id": "Ethernet1/1",
          "ingress-acl-sets": {
            "ingress-acl-set": [
              {
                "config": {
                  "set-name": "DenySubnets",
                  "type": "ACL_IPV4"
                },
                "set-name": "DenySubnets",
                "type": "ACL_IPV4"
              }
            ]
          },
          "interface-ref": {
            "config": {
              "interface": "Ethernet1/1",
              "subinterface": 0
            }
          }
        }
      ]
    },
    "state": {
      "counter-capability": "AGGREGATE_ONLY"
    }
  },
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "description for Ethernet1/1",
          "enabled": true,
          "name": "Ethernet1/1",
          "type": "ethernetCsmacd"
        },
        "name": "Ethernet1/1",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "enabled": true,

                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "10.1.1.1",
                        "prefix-length": 24
                      },
                      "ip": "10.1.1.1"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              }
            }
          ]
        }
      }
    ]
  },
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "default",
          "type": "DEFAULT_INSTANCE"
        },
        "name": "default",
        "protocols": {
          "protocol": [
            {
              "bgp": {
                "global": {
                  "afi-safis": {
                    "afi-safi": [
                      {
                        "afi-safi-name": "IPV4_UNICAST",
                        "config": {
                          "afi-safi-name": "IPV4_UNICAST",
                          "enabled": true
                        }
                      }
                    ]
                  },
                  "config": {
                    "as": 65001,
                    "router-id": "10.1.1.1"
                  }
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "IPV4_UNICAST",
                            "config": {
                              "afi-safi-name": "IPV4_UNICAST",
                              "enabled": true
                            }
                          }
                        ]
                      },
                      "config": {
                        "enabled": true,
                        "neighbor-address": "10.1.1.2",
                        "peer-as": 65002
                      },
                      "neighbor-address": "10.1.1.2"
                    }
                  ]
                }
              },
              "config": {
                "enabled": true,
                "identifier": "BGP",
                "name": "BGP"
              },
              "identifier": "BGP",
              "name": "BGP"
            },
            {
              "config": {
                "enabled": true,
                "identifier": "ISIS",
                "name": "DEFAULT"
              },
              "identifier": "ISIS",
              "isis": {
                "global": {
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
                    "level-capability": "LEVEL_2",
                    "net": [
                      "49.0001.0000.0000.0001.00"
                    ]
                  }
                },
                "interfaces": {
                  "interface": [
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
                        "circuit-type": "POINT_TO_POINT",
                        "enabled": true,
                        "interface-id": "Ethernet1/1"
                      },
                      "interface-id": "Ethernet1/1",
                      "levels": {
                        "level": [
                          {
                            "afi-safi": {
                              "af": [
                                {
                                  "afi-name": "IPV4",
                                  "config": {
                                    "afi-name": "IPV4",
                                    "metric": 10,
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
                }
              },
              "name": "DEFAULT"
            }
          ]
        }
      }
    ]
  }
}
```

1. Trigger soft OIR on one linecard by setting `/components/component/linecard[name=<LC_NAME>]/config/power-admin-state` to `POWER_DISABLED`.
1. Use `gnmi.Watch` with `.Await` to confirm the operational status `/components/component/state/oper-status` of the affected linecards transitions to `DISABLED` or `INACTIVE`.
1. Re-enable the linecards by setting `/components/component/linecard[name=<LC_NAME>]/config/power-admin-state` to `POWER_ENABLED`.
1. Immediately push the large configuration generated in Step 1 to the DUT using `gNMI.Set` with the `REPLACE` option.
1. Validation with pass/fail criteria:
    1. The `gNMI.Set` request MUST succeed without throwing a deadline-exceeded or timeout error.
    1. The execution time of the `gNMI.Set` operation MUST be under the 10-minute threshold.
1. Check that `/system/state/last-configuration-timestamp` updates correctly after the set operation.
1. Use `gnmi.Watch` with `.Await` to verify that the interfaces come back up and the applied configuration is reflected in the DUT state.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml 
paths:
  /components/component/linecard/config/power-admin-state:
    platform_type: ["LINECARD"]
  /components/component/linecard/state/power-admin-state:
    platform_type: ["LINECARD"]
  /system/state/last-configuration-timestamp:
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  /interfaces/interface/ethernet/config/port-speed:


rpcs:
  gnmi:
    gNMI.Set:
      replace: true
    gNMI.Get:
    gNMI.Subscribe:
      stream: true
      sample: true
```

## Required DUT platform

* FFF
