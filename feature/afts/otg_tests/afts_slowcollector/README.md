# AFT-1.2: AFTs slow collector 

## Summary
Reset one of the gNMI collectors, which are collecting the IPv4/IPv6 unicast routes' next-hop groups and next hops.

## Testbed
* atedut_2.testbed

## Test Setup

### Generate DUT and ATE Configuration

Configure DUT:port1,port2 for IS-IS session with ATE:port1,port2.

* Let `X` be the number of IPv4 prefixes to be advertised by eBGP. **(User Adjustable Value)**
* Let `Y` be the number of IPv6 prefixes to be advertised by eBGP. **(User Adjustable Value)**
* Let `Z` be the number of prefixes to be advertised by IS-IS. **(User Adjustable Value)**
* IS-IS must be level 2 only with wide metric.
* IS-IS must be point to point.
* Send `Z` IPv4 and `Z` IPv6 prefixes from ATE:port1 to DUT:port1.
Establish eBGP multipath sessions between ATE:port1,port2 and DUT:port1,port2
* Configure eBGP over the interface IP between ATE:port1,port2 and DUT:port1,port2.
* Advertise `X` IPv4 and `Y` IPv6 prefixes from ATE port1,port2.
* Each prefix advertised by eBGP must have 2 next hops pointing to ATE port1 and ATE port2.
* Each prefix advertised by IS-IS must have one next hop pointing to ATE port1.

### Procedure
* Use gNMI.UPDATE option to push the Test Setup configuration to the DUT.
* ATE configuration must be pushed.

### Verifications
* eBGP routes advertised from ATE:port1,port2 must have 2 next hops.
* Use gNMI Subscribe with `ON_CHANGE` option to `/network-instances/network-instance/afts`.
* Verify AFTs prefixes advertised by eBGP and IS-IS.
* Verify their next hop group, number of next hops, and the name of the interfaces.
* Verify the number of next hops is 2 for eBGP advertised prefixes.
* Verify the number of next hops is 1 for IS-IS advertised prefixes.
* Verify the prefixes are pointing to the correct egress interface(s).
* Verify all other leaves mentioned in the path section have the data populated correctly.

## AFT-1.2.1: AFT slow collector (base case)
* have a slow gNMI collector out of two gNMI collectors.

### Verifications
* Verify AFTs prefixes advertised by eBGP and IS-IS on both the collectors.
* Verify their next hop group, number of next hops, and the name of the interfaces.
* Stop the second collector and take a system memory usage in the DUT by subscribing "openconfig:system/processes/process/state/memory-utilization" and "openconfig:components/component/cpu/utilization/state/instant"
* Recreate a gNMI session for the second collector and verify the convergence time for the second collector.
* Also check the memory usage in the DUT after the second collector is added and converged. The memory usage for BGP and the Routing Engine (RE) should not increase before the first step and also monitor for any abnormal CPU utilization (more than 80%).

## OpenConfig Path and RPC Coverage
The below YAML defines the OC paths intended to be covered by this test.
OC paths used for test setup are not listed here.

## AFT-1.2.2: AFT route withdrawal from one BGP peer scenario 1

### Procedure

Withdraw the routes from bgp neighbor from ATE:port2 using OTG API.

### Verifications

* eBGP routes advertised from ATE:port1,port2 must have 1 nexthop (pointing to ATE:port1).
* IS-IS routes advertised from ATE:port1 must have one next hop.
* Verify AFTs prefixes advertised by eBGP and IS-IS.
* Verify their next hop group, number of next hops, and the name of the interfaces.
* Verify the number of next hop per prefix must be 1.
* Verify the memory and CPU util. as mentioned in AFT-1.2.1,

## AFT-1.2.3: AFT Withdraw prefixes from both the BGP neighbor scenario 2

### Procedure

Withdraw the routes from bgp neighbor from ATE:port2 using OTG API.

### Verifications

* eBGP routes advertised from ATE:port1,port2 must be removed from RIB and FIB of the DUT (query results should be nil).
* ISIS routes advertised from ATE:port1 must present from RIB and FIB of the DUT (query result should be nil).
* Verify the memory and CPU util. as mentioned in AFT-1.2.1.

## AFT-1.2.4: AFT readvertise the prefixes from port1 bgp neighbor scenario 3

### Procedure

Readvertise the BGP (both IPv4 and IPv6) prefixes from the BGP neighbor(s) of ATE:port1 and DUT:port1 using OTG API.

### Verifications

* eBGP routes advertised from ATE:port1 must have one next hop (pointing to ATE:port1).
* IS-IS routes advertised from ATE:port1 must have one next hop.
* Verify AFTs prefixes advertised by eBGP and ISIS.
* Verify their next hop group, number of next hops, and the name of the interfaces.
* Verify the number of next hop per prefix is 1.
* Verify the memory and CPU util. as mentioned in AFT-1.2.1.

## AFT-1.2.5: AFT Readvertise the BGP prefixes from both the neighbors scenario 4

### Procedure

Readvertise the BGP (both IPv4 and IPv6) prefixes from the BGP neighbor(s) of ATE:port1,port2 and DUT:port1,port2 using OTG API.

### Verifications

* eBGP routes advertised from ATE:port1,port2 must have 2 next hops.
* IS-IS routes advertised from ATE:port1 must have one next hop.
* Verify AFTs prefixes advertised by eBGP and ISIS.
* Verify their next hop group, number of next hops, and the name of the interfaces.
* Verify the memory and CPU util. as mentioned in AFT-1.2.1.

## OpenConfig Path and RPC Coverage
The below YAML defines the OC paths intended to be covered by this test.
OC paths used for test setup are not listed here.

```yaml
paths:
  ## State Paths ##

  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/weight:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hops/next-hop/interface-ref/state/interface:
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address:
rpcs:
  gnmi:
    gNMI.Subscribe:
```
## Canonical OC

```json
{
  "network-instances": {
    "network-instance": [
      {
        "name": "DEFAULT",
        "protocols": {
          "protocol": [
            {
              "identifier": "BGP",
              "name": "BGP",
              "config": {
                "identifier": "BGP",
                "name": "BGP"
              },
              "bgp": {
                "global": {
                  "config": {
                    "as": 65501,
                    "router-id": "192.0.2.1"
                  },
                  "afi-safis": {
                    "afi-safi": [
                      {
                        "afi-safi-name": "IPV4_UNICAST",
                        "config": {
                          "afi-safi-name": "IPV4_UNICAST",
                          "enabled": true
                        },
                        "use-multiple-paths": {
                          "ebgp": {
                            "config": {
                              "maximum-paths": 2
                            }
                          }
                        }
                      },
                      {
                        "afi-safi-name": "IPV6_UNICAST",
                        "config": {
                          "afi-safi-name": "IPV6_UNICAST",
                          "enabled": true
                        },
                        "use-multiple-paths": {
                          "ebgp": {
                            "config": {
                              "maximum-paths": 2
                            }
                          }
                        }
                      }
                    ]
                  }
                },
                "peer-groups": {
                  "peer-group": [
                    {
                      "peer-group-name": "BGP-PEER-GROUP-V4-P1",
                      "config": {
                        "peer-group-name": "BGP-PEER-GROUP-V4-P1",
                        "peer-as": 65511
                      },
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "IPV4_UNICAST",
                            "config": {
                              "afi-safi-name": "IPV4_UNICAST",
                              "enabled": true
                            },
                            "apply-policy": {
                              "config": {
                                "import-policy": [
                                  "ALLOW"
                                ],
                                "export-policy": [
                                  "ALLOW"
                                ]
                              }
                            },
                            "use-multiple-paths": { "config": { "enabled": true } }
                          }
                        ]
                      }
                    },
                    {
                      "peer-group-name": "BGP-PEER-GROUP-V6-P1",
                      "config": {
                        "peer-group-name": "BGP-PEER-GROUP-V6-P1",
                        "peer-as": 200
                      },
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "IPV6_UNICAST",
                            "config": {
                              "afi-safi-name": "IPV6_UNICAST",
                              "enabled": true
                            },
                            "apply-policy": {
                              "config": {
                                "import-policy": [
                                  "ALLOW"
                                ],
                                "export-policy": [
                                  "ALLOW"
                                ]
                              }
                            },
                            "use-multiple-paths": { "config": { "enabled": true } }
                          }
                        ]
                      }
                    },
                    {
                      "peer-group-name": "BGP-PEER-GROUP-V4-P2",
                      "config": {
                        "peer-group-name": "BGP-PEER-GROUP-V4-P2",
                        "peer-as": 200
                      },
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "IPV4_UNICAST",
                            "config": {
                              "afi-safi-name": "IPV4_UNICAST",
                              "enabled": true
                            },
                            "apply-policy": {
                              "config": {
                                "import-policy": [
                                  "ALLOW"
                                ],
                                "export-policy": [
                                  "ALLOW"
                                ]
                              }
                            },
                            "use-multiple-paths": { "config": { "enabled": true } }
                          }
                        ]
                      }
                    },
                    {
                      "peer-group-name": "BGP-PEER-GROUP-V6-P2",
                      "config": {
                        "peer-group-name": "BGP-PEER-GROUP-V6-P2",
                        "peer-as": 200
                      },
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "IPV6_UNICAST",
                            "config": {
                              "afi-safi-name": "IPV6_UNICAST",
                              "enabled": true
                            },
                            "apply-policy": {
                              "config": {
                                "import-policy": [
                                  "ALLOW"
                                ],
                                "export-policy": [
                                  "ALLOW"
                                ]
                              }
                            },
                            "use-multiple-paths": { "config": { "enabled": true } }
                          }
                        ]
                      }
                    }
                  ]
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "neighbor-address": "192.0.2.2",
                      "config": {
                        "neighbor-address": "192.0.2.2",
                        "peer-as": 200,
                        "enabled": true,
                        "peer-group": "BGP-PEER-GROUP-V4-P1"
                      },
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
                      }
                    },
                    {
                      "neighbor-address": "2001:db8::2",
                      "config": {
                        "neighbor-address": "2001:db8::2",
                        "peer-as": 200,
                        "enabled": true,
                        "peer-group": "BGP-PEER-GROUP-V6-P1"
                      },
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "IPV6_UNICAST",
                            "config": {
                              "afi-safi-name": "IPV6_UNICAST",
                              "enabled": true
                            }
                          }
                        ]
                      }
                    },
                    {
                      "neighbor-address": "192.0.2.6",
                      "config": {
                        "neighbor-address": "192.0.2.6",
                        "peer-as": 200,
                        "enabled": true,
                        "peer-group": "BGP-PEER-GROUP-V4-P2"
                      },
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
                      }
                    },
                    {
                      "neighbor-address": "2001:db8::6",
                      "config": {
                        "neighbor-address": "2001:db8::6",
                        "peer-as": 200,
                        "enabled": true,
                        "peer-group": "BGP-PEER-GROUP-V6-P2"
                      },
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "IPV6_UNICAST",
                            "config": {
                              "afi-safi-name": "IPV6_UNICAST",
                              "enabled": true
                            }
                          }
                        ]
                      }
                    }
                  ]
                }
              }
            },
            {
              "identifier": "ISIS",
              "name": "ISIS",
              "config": {
                "identifier": "ISIS",
                "name": "ISIS"
              },
              "isis": {
                "global": {
                  "config": {
                    "hello-padding": "DISABLE"
                  }
                },
                "interfaces": {
                  "interface": [
                    {
                      "interface-id": "port1",
                      "config": {
                        "interface-id": "port1",
                        "enabled": true
                      },
                      "levels": {
                        "level": [
                          {
                            "level-number": 2,
                            "config": {
                              "level-number": 2
                            }
                          }
                        ]
                      }
                    },
                    {
                      "interface-id": "port2",
                      "config": {
                        "interface-id": "port2",
                        "enabled": true
                      },
                      "levels": {
                        "level": [
                          {
                            "level-number": 2,
                            "config": {
                              "level-number": 2
                            }
                          }
                        ]
                      }
                    }
                  ]
                }
              }
            }
          ]
        }
      }
    ]
  },
  "routing-policy": {
    "policy-definitions": {
      "policy-definition": [
        {
          "name": "ALLOW",
          "config": {
            "name": "ALLOW"
          },
          "statements": {
            "statement": [
              {
                "name": "20",
                "config": {
                  "name": "20"
                },
                "actions": {
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                }
              }
            ]
          }
        }
      ]
    }
  }
}
```

