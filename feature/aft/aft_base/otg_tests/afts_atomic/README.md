# AFT-3.1: AFTs Atomic Flag Check

## Summary

Check the atomic flag in the notification.

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
* Each prefix advertised by ISIS must have one next hop pointing to ATE port1.

### Procedure

* Use gNMI.UPDATE option to push the Test Setup configuration to the DUT.
* ATE configuration must be pushed.

### Verifications

* eBGP must be established state.
* Use gNMI Subscribe with `ON_CHANGE` option to `/network-instances/network-instance/afts`.
* Verify the notifcations send from the DUT for the paths mentioned in the path section must have atomic flag set to true.


## AFT-3.1.1: AFT Atomic Flag check scenario 1

### Procedure

Bring down the link between ATE:port2 and DUT:port2 using OTG API.

### Verifications

* eBGP routes advertised from ATE:port1,port2 must have 1 nexthop (pointing to ATE:port1).
* IS-IS routes advertised from ATE:port1 must have one next hop.
* Verify the notifcations send from the DUT for the paths mentioned in the path section must have atomic flag set to true.

## AFT-3.1.2: AFT Atomic Flag Check Link Down and Up scenario 2

### Procedure

Bring down both links between ATE:port1,port2 and DUT:port1,port2 using OTG API and bring us using OTG API.

### Verifications

* eBGP routes advertised from ATE:port1,port2 must be removed from RIB and FIB of the DUT (query results should be nil).
* ISIS routes advertised from ATE:port1 must be removed from RIB and FIB of the DUT (query result should be nil).
* Bring up the both links using OTG API.
* Verify eBGP must be in stablished state.
* Verify the notifcations send from the DUT for the paths mentioned in the path section must have atomic flag set to true.

## OpenConfig Path and RPC Coverage

The below YAML defines the OC paths intended to be covered by this test.
OC paths used for test setup are not listed here.

```yaml
paths:
  ## State Paths ##

  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group:
  /network-instances/network-instance/afts/next-hops/next-hop:

rpcs:
  gnmi:
    gNMI.Subscribe:
```
## Canonical OC

```json
{
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
                          "import-policy": ["ALLOW"],
                          "export-policy": ["ALLOW"]
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
                          "import-policy": ["ALLOW"],
                          "export-policy": ["ALLOW"]
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
                          "import-policy": ["ALLOW"],
                          "export-policy": ["ALLOW"]
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
                          "import-policy": ["ALLOW"],
                          "export-policy": ["ALLOW"]
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
            },
            "afi": [
              {
                "afi-name": "IPV6",
                "safi-name": "UNICAST",
                "config": {
                  "afi-name": "IPV6",
                  "safi-name": "UNICAST"
                },
                "multi-topology": {
                  "config": {
                    "afi-name": "IPV4",
                    "safi-name": "UNICAST"
                  }
                }
              }
            ]
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
```