# RT-7.9: BGP ECMP for iBGP with IS-IS protocol nexthop

## Summary

This test is run to make sure that the BGP resolved over IS-IS next-hop and the
traffic is load balance on these resolved nexthops while forwarding traffic.

Implementations can be configured for equal cost multipath (ECMP) routing for 
iBGP which is routed using IS-IS protocol nexthop.

## Testbed type

[TESTBED_DUT_ATE_4LINKS](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Topolgy

Each Interface below is made up of 1x100G ports.

```mermaid
graph LR;
PORTA [ATE PORT1] <-- IPv4-IPv6 --> B[DUT PORT1];
PORTB [ATE:PORT2] <-- IS-IS (iBGP-Session1 with next-hop via IS-IS)--> D[DUT PORT2];
PORTC [ATE:PORT3] <-- IS-IS (iBGP-Session2 with next-hop via IS-IS)--> D[DUT PORT3];
PORTD [ATE:PORT4] <-- IS-IS (iBGP-Session3 with next-hop via IS-IS)--> D[DUT PORT4];
```

## Procedure

## Test environment setup
### Connection Network IP's and IS-IS/BGP Sessions
  * ATE:Port1 (192.1.1.2/24) - DUT:Port1 (192.1.1.1/24)
  * ATE:Port2 (192.1.2.2/24) - DUT:Port2 (192.1.2.1/24)
  * ATE:Port3 (192.1.3.2/24) - DUT:Port3 (192.1.3.1/24)
  * ATE:Port4 (192.1.4.2/24) - DUT:Port4 (192.1.4.1/24)

  * ATE:Port1 (192.1.1.2::2/64) - DUT:Port1 (192.1.1.1::1/64)
  * ATE:Port2 (192.1.2.2::2/64) - DUT:Port2 (192.1.2.1::1/64)
  * ATE:Port3 (192.1.3.2::2/64) - DUT:Port3 (192.1.3.1::1/64)
  * ATE:Port4 (192.1.4.2::2/64) - DUT:Port4 (192.1.4.1::1/64)

  * Routes advertised via IS-IS as below from PORTB,C,D are as below
      *PORTB-R1 - 193.1.1.1/24 (IPv4), 193.1.1.1::1/128 (IPv6)
      *PORTC-R2 - 193.1.1.2/24 (IPv4), 193.1.1.2::1/128 (IPv6)
      *PORTD-R3 - 193.1.1.3/24 (IPv4), 193.1.1.3::1/128 (IPv6)
  * ATE:Port2:iBGP-Session1(193.1.1.1/24)  - DUT:Loopback0:iBGP-Session1 (10.10.10.1)
  * ATE:Port3:iBGP-Session2(193.1.1.2/24)  - DUT:Loopback0:iBGP-Session2 (10.10.10.1)
  * ATE:Port4:iBGP-Session3(193.1.1.3/24)  - DUT:Loopback0:iBGP-Session3 (10.10.10.1)
  * ATE:Port2:iBGP-Session1(193.1.1.1::1/128)  - DUT:Loopback0:iBGP-Session1 (10.10.10.1::1/128)
  * ATE:Port3:iBGP-Session2(193.1.1.2::2/128)  - DUT:Loopback0:iBGP-Session2 (10.10.10.1::1/128)
  * ATE:Port4:iBGP-Session3(193.1.1.3::3/128)  - DUT:Loopback0:iBGP-Session3 (10.10.10.1::1/128)
  * AS Path: 64501

  * Routes advertised from the iBGP peer from all 3 ports PORTB, PORTC, PORTD
    * IPv4 -> 100.1.1.1/24
    * IPv6 -> 100.1.1.1::1/128

### Source port for sending traffic
  * Configure PORTA to send IPv4 traffic such that the flows have below properties
    Source: 192.1.1.2/24
    Destination: 100.1.1.1/24
    Packet Rate: 30000 packets per second
    Packet Header: UDP/TCP ports of Range 3000-15000
  * Configure PORTA to send IPv6 traffic such that the flows have below properties
    Source: 192.1.1.2::2/64
    Destination: 100.1.1.1::1/128
    Packet Rate: 30000 packets per second
    Packet Header: UDP/TCP ports of Range 3000-15000
  * Configure ATE Port2, Port3, Port4 as receiver ports such that in case of
    ECMP disabled, traffic is not distributed to the 3 ports (PORTB, PORTC,
    PORTD) and instead only one of the Ports should be receiving all the traffic
  * Configure ATE Port2, Port3, Port4 as receiver ports such that in case of 
    ECMP enabled traffic is distributed to the 3 ports (Port2, Port3, Port4)
    equally with 10000 packets per port with tolerance of 5%.

## Test cases

### RT-7.9.1: Traffic with best path selection IPv4
#### Staging
    * Configure IS-IS on the PORTb, PORTC, PORTD
    * Configure routes to be advertised from ATE as below
      * PORTB:R1, PORTC:R2, PORTD:R3
    * Configure iBGP sesssion1 on PORTB, session2 on PORTC, session3 on
      PORTD with the BGP parameters as below
      * AS Number: 64501
      * Multipath Enabled: False
      * Keepalive: Default (60)
      * Holdtime: Default (180)
      * Policy: Default policy to Allow all routes
      * Route Advertised: 100.1.1.1/24
    * Configure the traffic flows as mentioned in test environment setup.

#### Verification
  * Verify that route summary for 100.1.1.1/24 shows only one of the ports
    PORTB/PORTC/PORTD as forwarding.
  * Verify that the traffic (30k PPS) sent from PORTA is received by only one
    of the receiver ports PORTB/PORTC/PORTD.

### RT-7.9.2: Traffic with Multipath enabled and equal distribution of traffic IPv4

#### Staging
    * Configure IS-IS on the PORTb, PORTC, PORTD
    * Configure routes to be advertised from ATE as below
      * PORTB:R1, PORTC:R2, PORTD:R3
    * Configure iBGP sesssion1 on PORTB, session2 on PORTC, session3 on
      PORTD with the BGP parameters as below
      * AS Number: 64501
      * Multipath Enabled: True
      * Keepalive: Default (60)
      * Holdtime: Default (180)
      * Policy: Default policy to Allow all routes
      * Route Advertised: 100.1.1.1/24
    * Configure the traffic flows as mentioned in test environment setup

#### Verification
  * Verify that route summary for 100.1.1.1/24 shows all the ports
    PORTB/PORTC/PORTD as forwarding.
  * Verify that the traffic (30k PPS) sent from PORTA is received by PORTB,
    PORTC and PORTD equally such that each port receives 10k traffic each with
    a tolerance of 5%

### RT-7.9.3: Traffic with best path selection IPv6
#### Staging
    * Configure IS-IS on the PORTb, PORTC, PORTD
    * Configure routes to be advertised from ATE as below
      * PORTB:R1, PORTC:R2, PORTD:R3
    * Configure iBGP session with the BGP parameters as below
      * AS Number: 64501
      * Multipath Enabled: False
      * Keepalive: Default (60)
      * Holdtime: Default (180)
      * Policy: Default policy to Allow all routes
      * Route Advertised: 100.1.1.1::1/128
    * Configure the traffic flows as mentioned in test environment setup.

#### Verification
  * Verify that route summary for 100.1.1.1/24 shows only one of the ports
    PORTB/PORTC/PORTD as forwarding.
  * Verify that the traffic (30k PPS) sent from PORTA is received by only one
    of the receiver ports PORTB/PORTC/PORTD.

### RT-7.9.4: Traffic with Multipath enabled and equal distribution of traffic IPv6

#### Staging
    * Configure IS-IS on the PORTb, PORTC, PORTD
    * Configure routes to be advertised from ATE as below
      * PORTB:R1, PORTC:R2, PORTD:R3
    * Configure iBGP session1 on PORTB, session2 on PORTC, session 3 on
      PORTD with the BGP parameters as below
      * AS Number: 64501
      * Multipath Enabled: False
      * Keepalive: Default (60)
      * Holdtime: Default (180)
      * Policy: Default policy to Allow all routes
      * Route Advertised: 100.1.1.1::1/128
    * Configure the traffic flows as mentioned in test environment setup

#### Verification
  * Verify that route summary for 100.1.1.1::1/128 shows all the ports
    PORTB/PORTC/PORTD as forwarding.
  * Verify that the traffic (30k PPS) sent from PORTA is received by PORTB,
    PORTC and PORTD equally such that each port receives 10k traffic each with
    a tolerance of 5%

#### Canonical OC

This section should contain a JSON formatted stanza representing the
canonical OC to configure BGP add-paths. (See the
[README Template](https://github.com/openconfig/featureprofiles/blob/main/doc/test-requirements-template.md#procedure))

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
                "identifier": "openconfig-policy-types:ISIS",
                "name": "DEFAULT"
              },
              "identifier": "openconfig-policy-types:ISIS",
              "isis": {
                "global": {
                  "afi-safi": {
                    "af": [
                      {
                        "afi-name": "openconfig-isis-types:IPV4",
                        "config": {
                          "afi-name": "openconfig-isis-types:IPV4",
                          "enabled": true,
                          "metric": 10,
                          "safi-name": "openconfig-isis-types:UNICAST"
                        },
                        "safi-name": "openconfig-isis-types:UNICAST"
                      },
                      {
                        "afi-name": "openconfig-isis-types:IPV6",
                        "config": {
                          "afi-name": "openconfig-isis-types:IPV6",
                          "enabled": true,
                          "metric": 10,
                          "safi-name": "openconfig-isis-types:UNICAST"
                        },
                        "safi-name": "openconfig-isis-types:UNICAST"
                      }
                    ]
                  },
                  "config": {
                    "hello-padding": "DISABLE",
                    "level-capability": "LEVEL_2",
                    "net": [
                      "49.0001.1920.0000.2001.00"
                    ]
                  },
                  "mpls": {
                    "igp-ldp-sync": {
                      "config": {
                        "enabled": false
                      }
                    }
                  },
                  "timers": {
                    "config": {
                      "lsp-lifetime-interval": 65535,
                      "lsp-refresh-interval": 65218
                    },
                    "spf": {
                      "config": {
                        "spf-first-interval": "200",
                        "spf-hold-interval": "2000"
                      }
                    }
                  }
                },
                "interfaces": {
                  "interface": [
                    {
                      "afi-safi": {
                        "af": [
                          {
                            "afi-name": "openconfig-isis-types:IPV4",
                            "config": {
                              "afi-name": "openconfig-isis-types:IPV4",
                              "enabled": true,
                              "safi-name": "openconfig-isis-types:UNICAST"
                            },
                            "safi-name": "openconfig-isis-types:UNICAST"
                          },
                          {
                            "afi-name": "openconfig-isis-types:IPV6",
                            "config": {
                              "afi-name": "openconfig-isis-types:IPV6",
                              "enabled": true,
                              "safi-name": "openconfig-isis-types:UNICAST"
                            },
                            "safi-name": "openconfig-isis-types:UNICAST"
                          }
                        ]
                      },
                      "config": {
                        "circuit-type": "POINT_TO_POINT",
                        "enabled": true,
                        "interface-id": "Ethernet2/1"
                      },
                      "interface-id": "Ethernet2/1",
                      "levels": {
                        "level": [
                          {
                            "config": {
                              "enabled": false,
                              "level-number": 1
                            },
                            "level-number": 1
                          },
                          {
                            "afi-safi": {
                              "af": [
                                {
                                  "afi-name": "openconfig-isis-types:IPV4",
                                  "config": {
                                    "afi-name": "openconfig-isis-types:IPV4",
                                    "metric": 10,
                                    "safi-name": "openconfig-isis-types:UNICAST"
                                  },
                                  "safi-name": "openconfig-isis-types:UNICAST"
                                },
                                {
                                  "afi-name": "openconfig-isis-types:IPV6",
                                  "config": {
                                    "afi-name": "openconfig-isis-types:IPV6",
                                    "metric": 10,
                                    "safi-name": "openconfig-isis-types:UNICAST"
                                  },
                                  "safi-name": "openconfig-isis-types:UNICAST"
                                }
                              ]
                            },
                            "config": {
                              "enabled": true,
                              "level-number": 2
                            },
                            "level-number": 2,
                            "timers": {
                              "config": {
                                "hello-multiplier": 6
                              }
                            }
                          }
                        ]
                      },
                      "mpls": {
                        "igp-ldp-sync": {
                          "config": {
                            "enabled": false
                          }
                        }
                      },
                      "timers": {
                        "config": {
                          "lsp-pacing-interval": "50"
                        }
                      }
                    },
                    {
                      "afi-safi": {
                        "af": [
                          {
                            "afi-name": "openconfig-isis-types:IPV4",
                            "config": {
                              "afi-name": "openconfig-isis-types:IPV4",
                              "enabled": true,
                              "safi-name": "openconfig-isis-types:UNICAST"
                            },
                            "safi-name": "openconfig-isis-types:UNICAST"
                          },
                          {
                            "afi-name": "openconfig-isis-types:IPV6",
                            "config": {
                              "afi-name": "openconfig-isis-types:IPV6",
                              "enabled": true,
                              "safi-name": "openconfig-isis-types:UNICAST"
                            },
                            "safi-name": "openconfig-isis-types:UNICAST"
                          }
                        ]
                      },
                      "config": {
                        "circuit-type": "POINT_TO_POINT",
                        "enabled": true,
                        "interface-id": "Ethernet2/2"
                      },
                      "interface-id": "Ethernet2/2",
                      "levels": {
                        "level": [
                          {
                            "config": {
                              "enabled": false,
                              "level-number": 1
                            },
                            "level-number": 1
                          },
                          {
                            "afi-safi": {
                              "af": [
                                {
                                  "afi-name": "openconfig-isis-types:IPV4",
                                  "config": {
                                    "afi-name": "openconfig-isis-types:IPV4",
                                    "metric": 10,
                                    "safi-name": "openconfig-isis-types:UNICAST"
                                  },
                                  "safi-name": "openconfig-isis-types:UNICAST"
                                },
                                {
                                  "afi-name": "openconfig-isis-types:IPV6",
                                  "config": {
                                    "afi-name": "openconfig-isis-types:IPV6",
                                    "metric": 10,
                                    "safi-name": "openconfig-isis-types:UNICAST"
                                  },
                                  "safi-name": "openconfig-isis-types:UNICAST"
                                }
                              ]
                            },
                            "config": {
                              "enabled": true,
                              "level-number": 2
                            },
                            "level-number": 2,
                            "timers": {
                              "config": {
                                "hello-multiplier": 6
                              }
                            }
                          }
                        ]
                      },
                      "mpls": {
                        "igp-ldp-sync": {
                          "config": {
                            "enabled": false
                          }
                        }
                      },
                      "timers": {
                        "config": {
                          "lsp-pacing-interval": "50"
                        }
                      }
                    },
                    {
                      "afi-safi": {
                        "af": [
                          {
                            "afi-name": "openconfig-isis-types:IPV4",
                            "config": {
                              "afi-name": "openconfig-isis-types:IPV4",
                              "enabled": true,
                              "safi-name": "openconfig-isis-types:UNICAST"
                            },
                            "safi-name": "openconfig-isis-types:UNICAST"
                          },
                          {
                            "afi-name": "openconfig-isis-types:IPV6",
                            "config": {
                              "afi-name": "openconfig-isis-types:IPV6",
                              "enabled": true,
                              "safi-name": "openconfig-isis-types:UNICAST"
                            },
                            "safi-name": "openconfig-isis-types:UNICAST"
                          }
                        ]
                      },
                      "config": {
                        "circuit-type": "POINT_TO_POINT",
                        "enabled": true,
                        "interface-id": "Loopback0"
                      },
                      "interface-id": "Loopback0",
                      "levels": {
                        "level": [
                          {
                            "config": {
                              "enabled": false,
                              "level-number": 1
                            },
                            "level-number": 1
                          },
                          {
                            "afi-safi": {
                              "af": [
                                {
                                  "afi-name": "openconfig-isis-types:IPV4",
                                  "config": {
                                    "afi-name": "openconfig-isis-types:IPV4",
                                    "metric": 10,
                                    "safi-name": "openconfig-isis-types:UNICAST"
                                  },
                                  "safi-name": "openconfig-isis-types:UNICAST"
                                },
                                {
                                  "afi-name": "openconfig-isis-types:IPV6",
                                  "config": {
                                    "afi-name": "openconfig-isis-types:IPV6",
                                    "metric": 10,
                                    "safi-name": "openconfig-isis-types:UNICAST"
                                  },
                                  "safi-name": "openconfig-isis-types:UNICAST"
                                }
                              ]
                            },
                            "config": {
                              "enabled": true,
                              "level-number": 2
                            },
                            "level-number": 2,
                            "timers": {
                              "config": {
                                "hello-multiplier": 6
                              }
                            }
                          }
                        ]
                      },
                      "mpls": {
                        "igp-ldp-sync": {
                          "config": {
                            "enabled": false
                          }
                        }
                      },
                      "timers": {
                        "config": {
                          "lsp-pacing-interval": "50"
                        }
                      }
                    }
                  ]
                },
                "levels": {
                  "level": [
                    {
                      "config": {
                        "level-number": 2,
                        "metric-style": "WIDE_METRIC"
                      },
                      "level-number": 2
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

## OpenConfig Path and RPC Coverage

```yaml
paths:
  ## Config Paths ##
  /network-instances/network-instance/protocols/protocol/isis/global/afi-safi/af/config/afi-name:
  /network-instances/network-instance/protocols/protocol/isis/global/afi-safi/af/config/safi-name:
  /network-instances/network-instance/protocols/protocol/isis/global/afi-safi/af/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/global/config/level-capability:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/config/metric-style:
  /network-instances/network-instance/protocols/protocol/isis/global/config/weighted-ecmp:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/weighted-ecmp/config/load-balancing-weight:
  /network-instances/network-instance/protocols/protocol/isis/global/config/max-ecmp-paths:
    value: 3
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range:
    value: exact
  /routing-policy/policy-definitions/policy-definition/config/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/config/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result:
    value: ACCEPT_ROUTE
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/use-multiple-paths/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/use-multiple-paths/ebgp/config/allow-multiple-as:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/use-multiple-paths/ebgp/config/maximum-paths:

  ## State Paths ##
  /network-instances/network-instance/protocols/protocol/isis/global/state/weighted-ecmp:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/weighted-ecmp/state/load-balancing-weight:
  /interfaces/interface/state/counters/out-pkts:
  /interfaces/interface/state/counters/in-pkts:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
```

## Minimum DUT platform requirement
* FFF

