# AFT-3.1: AFTs Atomic Flag Check

## Summary

This test verifies that the `atomic` flag is correctly set in gNMI subscription
notifications for the AFT (Abstract Forwarding Table) paths during network
churn events. The `atomic` flag is expected to be `true` for the initial
synchronization and for updates that occur after a link comes back up, and
`false` for delete notifications when a link goes down.

## Testbed

*   featureprofiles/topologies/atedut_2.testbed

## Test Setup

### Generate DUT and ATE Configuration

#### Variables

*   Let `X` be the number of IPv4 prefixes to be advertised by eBGP. **(User
    Adjustable Value)**
*   Let `Y` be the number of IPv6 prefixes to be advertised by eBGP. **(User
    Adjustable Value)**
*   Let `Z` be the number of IPv4 prefixes to be advertised by IS-IS. **(User
    Adjustable Value)**
*   Let `Z1` be the number of IPv6 prefixes to be advertised by IS-IS. **(User
    Adjustable Value)**

#### Configure IS-IS session.

*   Configure DUT:port1,port2 for IS-IS session with ATE:port1,port2.
*   IS-IS must be level 2 only with wide metric.
*   IS-IS must be point to point.
*   Send `Z` IPv4 and `Z1` IPv6 prefixes from ATE:port1 to DUT:port1.
*   Each prefix advertised by ISIS must have one next hop pointing to ATE port1.

#### Configure eBGP multipath sessions.

*   Configure eBGP over the interface IP between ATE:port1,port2 and
    DUT:port1,port2.
*   eBGP DUT AS is 65501 and peer AS is 200.
*   eBGP is enabled for address family IPv4 and IPv6.
*   Advertise `X` IPv4 and `Y` IPv6 prefixes from ATE port1,port2.
*   Each prefix advertised by eBGP must have 2 next hops pointing to ATE port1
    and ATE port2.

### Procedure

*   Use gNMI.UPDATE option to push the Test Setup configuration to the DUT.
*   ATE configuration must be pushed.
*   After configuration, establish a gNMI `Subscribe` stream for the
    `/network-instances/network-instance/afts` path with `ON_CHANGE` mode.

### Verifications

*   **Initial Synchronization:**
    *   Verify that eBGP sessions are in the `ESTABLISHED` state.
    *   The first set of gNMI notifications received on the subscription stream
        represents the initial synchronization of the AFT state.
    *   Verify that these are all `update` notifications.
    *   Verify that **every** `update` notification in this initial batch has
        the `atomic` flag set to `true`.
    *   Ensure all expected BGP and ISIS routes are present in the AFT once the
        initial sync is complete.

## AFT-3.1.1: AFT Atomic Flag check scenario 1

## Procedure

Bring down the link between ATE:port2 and DUT:port2 using OTG API.

### Verifications

*   Verify all the notifications sent from the DUT for the paths mentioned in
    the path section have atomic flag set to true for updates, and false for
    deletes.

## AFT-3.1.2: AFT Atomic Flag Check Link Down and Up scenario 2

### Procedure

1.  Bring down both links between ATE:port1,port2 and DUT:port1,port2 using OTG
    API.
2.  After verifications, bring up both links using OTG API.

### Verifications

*   **After Bringing Links Down:**
    *   Verify that `delete` notifications are received on the gNMI stream
        remove all previously learned BGP and ISIS routes.
    *   Verify that the `atomic` flag is **not present** for all `delete`
        notifications.
    *   Confirm that the eBGP and ISIS routes are removed from the DUT's RIB and
        FIB (query results should be nil).
*   **After Bringing Links Up:**
    *   Verify that eBGP sessions are re-established.
    *   A new set of `update` notifications should be received for all BGP and
        ISIS routes.
    *   Verify that the `atomic` flag is set to `true` on these `update`
        notifications.
    *   Confirm that all routes are correctly re-installed in the RIB and FIB
        with their respective next-hops.

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

## OpenConfig Path and RPC Coverage

The below YAML defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

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
