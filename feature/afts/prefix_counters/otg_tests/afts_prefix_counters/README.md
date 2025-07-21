# AFT-2.1: AFTs Prefix Counters

## Summary

IPv4/IPv6 prefix counters

## Testbed

* atedut_2.testbed

## Test Setup

### Generate DUT and ATE Configuration

Configure DUT:port1 for IS-IS session with ATE:port1
*   IS-IS must be level 2 only with wide metric.
*   IS-IS must be point to point.
*   Send 1000 ipv4 and 1000 ipv6 IS-IS prefixes from ATE:port1 to DUT:port1.

Establish eBGP sessions between ATE:port1 and DUT:port1.
*   Configure eBGP over the interface ip.
*   Advertise 1000 ipv4,ipv6 prefixes from ATE port1 observe received prefixes at DUT.

### Procedure

*   Gnmi set with REPLACE option to push the configuration DUT.
*   ATE configuration must be pushed.

### verifications

*   BGP routes advertised from ATE:port1 must have 1 nexthop.
*   IS-IS routes advertised from ATE:port1 must have one next hop.
*   Use gnmi Subscribe with ON_CHANGE option to /network-instances/network-instance/afts.
*   Verify afts prefix entries using the following paths with in a timeout of 30s.

/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix,
/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix



## AFT-2.1.1: AFT Prefix Counters ipv4 packets forwarded, ipv4 octets forwarded IS-IS route.

### Procedure

From ATE:port2 send 10000 packets to one of the ipv4 prefix advertise by IS-IS.

### Verifications

*  Before the traffic measure the initial counter value.
*  After the traffic measure the final counter value.
*  The difference between final and initial value must match with the counter value in ATE then the test is marked as passed.
*  Verify afts ipv4 forwarded packets and ipv4 forwarded octets counter entries using the path mentioned in the paths section of this test plan.

## AFT-2.1.2: AFT Prefix Counters ipv4 packets forwarded, ipv4 octets forwarded BGP route.

### Procedure

From ATE:port2 send 10000 packets to one of the ipv4 prefix advertise by BGP.

### Verifications

*  Before the traffic measure the initial counter value.
*  After the traffic measure the final counter value.
*  The difference between final and initial value must match with the counter value in ATE, then the test is marked as passed.
*  Verify afts ipv4 forwarded packets and ipv4 forwarded octets counter entries using the path mentioned in the paths section of this test plan.


## AFT-2.1.3: AFT Prefix Counters ipv6 packets forwarded, ipv6 octets forwarded IS-IS route.

### Procedure

From ATE:port2 send 10000 packets to one of the ipv6 prefix advertise by IS-IS.

### Verifications

*  Before the traffic measure the initial counter value.
*  After the traffic measure the final counter value.
*  The difference between final and initial value must match with the counter value in ATE, then the test is marked as passed.
*  Verify afts ipv6 forwarded packets and ipv6 forwarded octets counter entries using the path mentioned in the paths section of this test plan.

## AFT-2.1.4: AFT Prefix Counters ipv6 packets forwarded, ipv6 octets forwarded BGP route.

### Procedure

From ATE:port2 send 10000 packets to one of the ipv6 prefix advertise by BGP.

### Verifications

*  Before the traffic measure the initial counter value.
*  After the traffic measure the final counter value.
*  The difference between final and initial value must match with the counter value in ATE, then the test is marked as passed.
*  Verify afts ipv6 forwarded packets and ipv6 forwarded octets counter entries using the path mentioned in the paths section of this test plan.

## AFT-2.1.5: AFT Prefix Counters withdraw the ipv4 prefix.

### Procedure

*  From ATE:port1 withdraw some prefixes of BGP and IS-IS.
*  Send 10000 packets from ATE:port2 to DUT:port2 for one of the withdrawn ipv4 prefix.
*  The traffic must blackhole.

### Verifications

* The counters must not send incremental value as the prefix is not present in RIB/FIB. The test fails if the counter shows incremental values.
* Verify afts ipv4 forwarded packet counter entries using the path mentioned in the paths section of this test plan.

## AFT-2.1.6: AFT Prefix Counters add the ipv4 prefix back.

### Procedure

*  From ATE:port1 add the prefixes of BGP and IS-IS back.
*  Send 10000 packets from ATE:port2 to DUT:port2 for one of the added ipv4 prefix.
*  The traffic must flow end to end.

### Verifications

*  Before the traffic measure the initial counter value.
*  After the traffic measure the final counter value.
*  The difference between final and initial value must match with the counter value in ATE, then the test is marked as passed.
*  Verify afts counter entries using the path mentioned in the paths section of this test plan.

## AFT-2.1.7: AFT Prefix Counters withdraw the ipv6 prefix.

### Procedure

*  From ATE:port1 withdraw some prefixes of BGP and IS-IS.
*  Send 10000 packets from ATE:port2 to DUT:port2 for one of the withdrawn ipv6 prefix.
*  The traffic must blackhole.

### Verifications

* The counters must not send incremental value as the prefix is not present in RIB/FIB. The test fails if the counter shows incremental values.
*  Verify afts counter entries using the path mentioned in the paths section of this test plan.

## AFT-2.1.8: AFT Prefix Counters add the ipv6 prefix back.

### Procedure

*  From ATE:port1 add the prefixes of BGP and IS-IS back.
*  Send 10000 packets from ATE:port2 to DUT:port2 for one of the added ipv6 prefix.
*  The traffic must flow end to end.

### Verifications

*  Before the traffic measure the initial counter value.
*  After the traffic measure the final counter value.
*  The difference between final and initial value must match with the counter value in ATE, then the test is marked as passed.
*  Verify afts counter entries using the path mentioned in the paths section of this test plan.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

```yaml
paths:
  ## State Paths ##

  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/counters/octets-forwarded:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/counters/packets-forwarded:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/counters/octets-forwarded:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/counters/packets-forwarded:
  
rpcs:
  gnmi:
    gNMI.Subscribe:
```

## Control Protocol Coverage

BGP
IS-IS

## Minimum DUT Platform Requirement

vRX

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
