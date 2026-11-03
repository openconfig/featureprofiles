# TE-1.23: gRIBI Tunnel Programming over Active BGP

## Summary

This test validates that gRIBI-injected tunnel routes for specific IPv4 and IPv6
prefixes deterministically override active eBGP-learned routes for those exact
same prefixes in the hardware forwarding plane. It verifies that the network
operating system prioritizes SDN controller traffic-engineering intent (`gRIBI`
`ADD`) over dynamic routing protocols (`BGP`), encapsulates matching traffic
onto the programmed tunnel path without disturbing unrelated BGP-only prefixes,
accurately reports route precedence in both `gRIBI.Get` and `gNMI` Abstract
Forwarding Table (AFT) telemetry, and hitlessly reverts to the underlying active
BGP path when the gRIBI routes are removed (`DELETE`).

## Testbed type

* [`TESTBED_DUT_ATE_4LINKS`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

```text
                         +---------+          eBGP (ASN 65537)
  [ ATE port1 ] -------- |         | -------- [ ATE port2 ] (Native BGP Path)
  (Traffic Src)          |   DUT   |
                         | (65536) | -------- [ ATE port3 ] (gRIBI Tunnel Path 1)
                         |         |          eBGP (ASN 65538)
                         |         | -------- [ ATE port4 ] (gRIBI Tunnel Path 2)
                         +---------+          eBGP (ASN 65539)
```

* Connect 4 interfaces between the DUT and ATE and configure dual-stack IPv4 and
  IPv6 addresses in the `DEFAULT` network instance:
  * **Link 1 (Traffic Ingress)**: Connect DUT `port1` to ATE `port1`.
    * DUT `port1`: IPv4 `192.0.2.1/30`, IPv6 `2001:db8:1::1/126`
    * ATE `port1`: IPv4 `192.0.2.2/30`, IPv6 `2001:db8:1::2/126`
  * **Link 2 (Native BGP Path)**: Connect DUT `port2` to ATE `port2`.
    * DUT `port2`: IPv4 `192.0.2.5/30`, IPv6 `2001:db8:2::1/126`
    * ATE `port2`: IPv4 `192.0.2.6/30`, IPv6 `2001:db8:2::2/126`
  * **Link 3 (Primary Tunnel Egress Path)**: Connect DUT `port3` to ATE `port3`.
    * DUT `port3`: IPv4 `192.0.2.9/30`, IPv6 `2001:db8:3::1/126`
    * ATE `port3`: IPv4 `192.0.2.10/30`, IPv6 `2001:db8:3::2/126`
  * **Link 4 (Secondary Tunnel Egress Path)**: Connect DUT `port4` to ATE
    `port4`.
    * DUT `port4`: IPv4 `192.0.2.13/30`, IPv6 `2001:db8:4::1/126`
    * ATE `port4`: IPv4 `192.0.2.14/30`, IPv6 `2001:db8:4::2/126`
* Verify all 4 DUT interfaces transition to `UP` using `gNMI.Subscribe`
  (`ON_CHANGE`) on `/interfaces/interface/state/oper-status`.
* Configure eBGP on the DUT (`ASN 65536`, router-id `192.0.2.1`) with permissive
  route-policy (`ACCEPT_ROUTE`) for IPv4-Unicast and IPv6-Unicast families:
  * Peer with ATE `port2` (`ASN 65537`, `192.0.2.6` / `2001:db8:2::2`).
  * Peer with ATE `port3` (`ASN 65538`, `192.0.2.10` / `2001:db8:3::2`).
  * Peer with ATE `port4` (`ASN 65539`, `192.0.2.14` / `2001:db8:4::2`).
* Start OTG protocols and explicitly resolve IPv4 ARP and IPv6 Neighbor
  Discovery (ND) across all 4 ports before starting any traffic flows.
* Verify all IPv4 and IPv6 eBGP sessions reach `ESTABLISHED` via
  `gNMI.Subscribe` (`ON_CHANGE`) on
  `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state`.
* Advertise the following prefixes via eBGP:
  * **Group A (Override Target Prefixes — advertised by ATE `port2`)**:
    * `500` IPv4 `/24` prefixes: `100.64.0.0/24` through `100.65.243.0/24`.
    * `500` IPv6 `/64` prefixes: `2001:db8:1000::/64` through
      `2001:db8:11f3::/64`.
  * **Group B (BGP-Only Control Prefixes — advertised by ATE `port2`)**:
    * `500` IPv4 `/24` prefixes: `100.66.0.0/24` through `100.67.243.0/24`.
    * `500` IPv6 `/64` prefixes: `2001:db8:2000::/64` through
      `2001:db8:21f3::/64`.
  * **Tunnel Endpoint Underlay Prefixes**:
    * Advertise tunnel endpoint `198.51.100.10/32` from ATE `port3` (`ASN
      65538`).
    * Advertise tunnel endpoint `198.51.100.20/32` from ATE `port4` (`ASN
      65539`).
* Establish a `gRIBI` client session to the DUT in `SINGLE_PRIMARY` redundancy
  mode with `PRESERVE` persistence and `FIB_ACK` requested, elect the client as
  leader (`election_id = {high: 0, low: 1}`), and execute `gRIBI.Flush` on the
  `DEFAULT` network instance.

### TE-1.23.1 - Baseline Native BGP Forwarding Validation

* **Step 1 - Verify BGP AFT Installation**:
  * Using `gNMI.Subscribe` (`ON_CHANGE`), confirm all `1,000` IPv4 prefixes
    (Groups A and B) and `1,000` IPv6 prefixes (Groups A and B) are installed in
    the `DEFAULT` AFT with `origin-protocol` set to `BGP`.
  * Confirm via `gRIBI.Get` on `DEFAULT` that zero gRIBI entries are currently
    present.
* **Step 2 - Start Baseline Traffic**:
  * From ATE `port1`, start 4 continuous OTG traffic flows (`512`-byte frames,
    `5,000 fps` each, inner IPv4 `TTL = 64` / IPv6 `HopLimit = 64`, `DSCP =
    10`):
    * `Flow-GroupA-v4`: Destined to `100.64.0.1`–`100.65.243.1` (`500` flows).
    * `Flow-GroupA-v6`: Destined to `2001:db8:1000::1`–`2001:db8:11f3::1` (`500`
      flows).
    * `Flow-GroupB-v4`: Destined to `100.66.0.1`–`100.67.243.1` (`500` flows).
    * `Flow-GroupB-v6`: Destined to `2001:db8:2000::1`–`2001:db8:21f3::1` (`500`
      flows).
* **Step 3 - Pass/Fail Criteria**:
  * Verify `100%` of packets for all 4 flows arrive unencapsulated on ATE
    `port2` with `0%` packet loss, and `0` packets arrive on ATE `port3` or ATE
    `port4`.
  * Verify `gNMI.Subscribe` (`SAMPLE`) interface counters (`in-pkts` on `port1`,
    `out-pkts` on `port2`) increment at the expected frame rate.

### TE-1.23.2 - gRIBI Tunnel Injection (ADD) Overriding Active BGP Routes

* **Step 1 - Inject gRIBI Encapsulated Tunnel Routes for Group A**:
  * While all 4 OTG traffic flows remain active, issue `gRIBI.Modify` (`ADD`,
    requesting `FIB_PROGRAMMED` ACK) in `DEFAULT`:
    * Install `NextHop` `index = 10` encapsulating in IPv4 (`encapsulate-header
      = IPV4`) with outer source IP `198.51.100.1`, outer destination IP
      `198.51.100.10` (resolving via ATE `port3`), outer IPv4 `TTL = 64`, and
      outer `DSCP = 10`.
    * Install `NextHop` `index = 11` encapsulating in IPv4 (`encapsulate-header
      = IPV4`) with outer source IP `198.51.100.1`, outer destination IP
      `198.51.100.20` (resolving via ATE `port4`), outer IPv4 `TTL = 64`, and
      outer `DSCP = 10`.
    * Install `NextHopGroup` `id = 100` referencing `NextHop 10` (`weight = 1`)
      and `NextHop 11` (`weight = 1`).
    * Install `500` `IPv4Entry` routes matching the exact Group A prefixes
      (`100.64.0.0/24` through `100.65.243.0/24`) and `500` `IPv6Entry` routes
      matching the exact Group A IPv6 prefixes (`2001:db8:1000::/64` through
      `2001:db8:11f3::/64`), all pointing to `NextHopGroup` `id = 100`.
* **Step 2 - Control-Plane & Telemetry Validation**:
  * Verify every `AFTOperation` receives `FIB_PROGRAMMED` acknowledgment.
  * Issue `gRIBI.Get` on `DEFAULT` and verify all `500` `IPv4Entry` routes,
    `500` `IPv6Entry` routes, `NextHopGroup 100`, and `NextHops 10, 11` are
    returned with `fib_status = PROGRAMMED`.
  * Verify via `gNMI.Subscribe` (`ON_CHANGE`) on
    `/network-instances/network-instance/afts/` that:
    * All `500` Group A IPv4 and `500` Group A IPv6 entries report
      `origin-protocol = GRIBI` and `next-hop-group = 100`.
    * All `500` Group B IPv4 and `500` Group B IPv6 entries continue to report
      `origin-protocol = BGP`.
* **Step 3 - Data-Plane & Encapsulation Validation**:
  * Verify `Flow-GroupA-v4` and `Flow-GroupA-v6` cease egressing ATE `port2`
    (`0` new packets on `port2` after convergence) and are load-balanced `50% /
    50%` (`±2%` tolerance) across ATE `port3` and ATE `port4` as IP-in-IP
    (`IPv4-in-IPv4` protocol `4` and `IPv6-in-IPv4` protocol `41`) encapsulated
    packets.
  * Inspect captured packets on ATE `port3` and `port4` to confirm outer source
    IP `198.51.100.1`, outer destination IP `198.51.100.10` / `198.51.100.20`,
    and preserved inner destination address and inner `DSCP = 10`.
  * Verify `Flow-GroupB-v4` and `Flow-GroupB-v6` experience `0%` packet loss and
    remain `100%` on native BGP path ATE `port2`.

### TE-1.23.3 - BGP Underlay Churn Under Active gRIBI Override

* **Step 1 - Flap Overridden BGP Routes on Link 2**:
  * Withdraw the `500` Group A IPv4 and `500` Group A IPv6 prefixes from ATE
    `port2` while keeping `Flow-GroupA-v4`, `Flow-GroupA-v6`, `Flow-GroupB-v4`,
    and `Flow-GroupB-v6` running.
  * Confirm via `gNMI.Subscribe` on BGP neighbor prefix counters
    (`prefixes/installed`) that ATE `port2` prefix counts decrease by `500` per
    family, while AFT `origin-protocol` for Group A remains `GRIBI`.
  * Re-advertise the `500` Group A IPv4 and `500` Group A IPv6 prefixes from ATE
    `port2` and confirm BGP RIB re-installation.
* **Step 2 - Pass/Fail Criteria**:
  * Verify `0%` packet loss on `Flow-GroupA-v4` and `Flow-GroupA-v6` across ATE
    `port3` and `port4` throughout the BGP withdrawal and re-advertisement
    cycle, proving complete FIB isolation of the active gRIBI override.
  * Verify `0%` packet loss on `Flow-GroupB-v4` and `Flow-GroupB-v6` on ATE
    `port2`.

### TE-1.23.4 - gRIBI Tunnel Deletion (DELETE) and Fallback to Active BGP

* **Step 1 - Delete gRIBI Routes for Group A**:
  * Issue `gRIBI.Modify` (`DELETE`, requesting `FIB_PROGRAMMED` ACK) to remove
    all `500` Group A `IPv4Entry` and `500` Group A `IPv6Entry` routes, followed
    by `NextHopGroup 100` and `NextHops 10, 11`.
* **Step 2 - Telemetry & Forwarding Reversion Validation**:
  * Verify via `gRIBI.Get` that the deleted entries are no longer present in the
    gRIBI RIB/FIB.
  * Verify via `gNMI.Subscribe` (`ON_CHANGE`) on AFT state paths that all `500`
    Group A IPv4 and `500` Group A IPv6 prefixes remain present in the AFT and
    revert `origin-protocol` from `GRIBI` back to `BGP`.
  * Verify `Flow-GroupA-v4` and `Flow-GroupA-v6` transition cleanly back to ATE
    `port2` as native unencapsulated packets (`0` packets arriving on `port3` or
    `port4` post-convergence), with convergence loss `< 1` second of traffic and
    `0%` steady-state loss thereafter.
  * Verify `Flow-GroupB-v4` and `Flow-GroupB-v6` suffer `0%` packet loss during
    the gRIBI route deletion.

### Cleanup

* Stop all OTG traffic streams and protocol sessions.
* Execute `gRIBI.Flush` on network instance `DEFAULT` to ensure zero residual
  gRIBI AFT state.
* Revert DUT interface, routing-policy, and BGP configurations to the baseline
  pre-test state.

## Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "Traffic Ingress Port 1",
          "enabled": true,
          "name": "port1",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port1",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.1",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.1"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:1::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8:1::1"
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
      },
      {
        "config": {
          "description": "Native BGP Egress Port 2",
          "enabled": true,
          "name": "port2",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port2",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.5",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.5"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:2::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8:2::1"
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
      },
      {
        "config": {
          "description": "Primary Tunnel Egress Port 3",
          "enabled": true,
          "name": "port3",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port3",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.9",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.9"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:3::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8:3::1"
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
      },
      {
        "config": {
          "description": "Secondary Tunnel Egress Port 4",
          "enabled": true,
          "name": "port4",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port4",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.13",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.13"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:4::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8:4::1"
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
          "name": "DEFAULT",
          "type": "openconfig-network-instance-types:DEFAULT_INSTANCE"
        },
        "name": "DEFAULT",
        "protocols": {
          "protocol": [
            {
              "bgp": {
                "global": {
                  "afi-safis": {
                    "afi-safi": [
                      {
                        "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                        "config": {
                          "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                          "enabled": true
                        }
                      },
                      {
                        "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                        "config": {
                          "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                          "enabled": true
                        }
                      }
                    ]
                  },
                  "config": {
                    "as": 65536,
                    "router-id": "192.0.2.1"
                  }
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ACCEPT_ALL"
                                ],
                                "import-policy": [
                                  "ACCEPT_ALL"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                              "enabled": true
                            }
                          }
                        ]
                      },
                      "config": {
                        "enabled": true,
                        "neighbor-address": "192.0.2.6",
                        "peer-as": 65537
                      },
                      "neighbor-address": "192.0.2.6"
                    },
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ACCEPT_ALL"
                                ],
                                "import-policy": [
                                  "ACCEPT_ALL"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                              "enabled": true
                            }
                          }
                        ]
                      },
                      "config": {
                        "enabled": true,
                        "neighbor-address": "2001:db8:2::2",
                        "peer-as": 65537
                      },
                      "neighbor-address": "2001:db8:2::2"
                    },
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ACCEPT_ALL"
                                ],
                                "import-policy": [
                                  "ACCEPT_ALL"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                              "enabled": true
                            }
                          }
                        ]
                      },
                      "config": {
                        "enabled": true,
                        "neighbor-address": "192.0.2.10",
                        "peer-as": 65538
                      },
                      "neighbor-address": "192.0.2.10"
                    },
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ACCEPT_ALL"
                                ],
                                "import-policy": [
                                  "ACCEPT_ALL"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                              "enabled": true
                            }
                          }
                        ]
                      },
                      "config": {
                        "enabled": true,
                        "neighbor-address": "2001:db8:3::2",
                        "peer-as": 65538
                      },
                      "neighbor-address": "2001:db8:3::2"
                    },
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ACCEPT_ALL"
                                ],
                                "import-policy": [
                                  "ACCEPT_ALL"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                              "enabled": true
                            }
                          }
                        ]
                      },
                      "config": {
                        "enabled": true,
                        "neighbor-address": "192.0.2.14",
                        "peer-as": 65539
                      },
                      "neighbor-address": "192.0.2.14"
                    },
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ACCEPT_ALL"
                                ],
                                "import-policy": [
                                  "ACCEPT_ALL"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                              "enabled": true
                            }
                          }
                        ]
                      },
                      "config": {
                        "enabled": true,
                        "neighbor-address": "2001:db8:4::2",
                        "peer-as": 65539
                      },
                      "neighbor-address": "2001:db8:4::2"
                    }
                  ]
                }
              },
              "config": {
                "identifier": "openconfig-policy-types:BGP",
                "name": "BGP"
              },
              "identifier": "openconfig-policy-types:BGP",
              "name": "BGP"
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
          "config": {
            "name": "ACCEPT_ALL"
          },
          "name": "ACCEPT_ALL",
          "statements": {
            "statement": [
              {
                "actions": {
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                },
                "config": {
                  "name": "accept_all_stmt"
                },
                "name": "accept_all_stmt"
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
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/name:
  /interfaces/interface/config/type:
  /interfaces/interface/state/counters/in-pkts:
  /interfaces/interface/state/counters/out-pkts:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/origin-protocol:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/origin-protocol:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/weight:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hops/next-hop/ip-in-ip/state/dst-ip:
  /network-instances/network-instance/afts/next-hops/next-hop/ip-in-ip/state/src-ip:
  /network-instances/network-instance/afts/next-hops/next-hop/state/encapsulate-header:
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/as:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/installed:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/neighbor-address:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
      on_change: true
  gribi:
    gRIBI.Flush:
    gRIBI.Get:
    gRIBI.Modify:
```

## Required DUT platform

* FFF
