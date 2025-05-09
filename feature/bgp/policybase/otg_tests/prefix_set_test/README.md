# RT-1.33: BGP Policy with prefix-set matching

## Summary

BGP policy configuration with prefix-set matching

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

## Procedure
* Establish eBGP sessions between:
  * ATE port-1 and DUT port-1
  * ATE port-2 and DUT port-2
  * Configure Route-policy under BGP neighbor/session address-family

* For IPv4:
  * Create three prefix-sets as below:
  * IPv4-prefix-set-1  - exact match on 10.23.15.0/26
  * IPv4-prefix-set-2  - match on 10.23.0.0/16
  * [TODO] IPv4-prefix-set-3  - match on 10.23.15.0/26, 10.23.17.0/26
    ```
    {
      "openconfig-routing-policy:routing-policy": {
        "defined-sets": {
          "prefix-sets": {
            "prefix-set": [
              {
                "name": "IPV4-prefix-set-1",
                "config": {
                  "name": "IPV4-prefix-set-1",
                  "mode": "IPV4"
                },
                "prefixes": {
                  "prefix": [
                    {
                      "ip-prefix": "10.23.15.0/26",
                      "masklength-range": "exact",
                      "config": {
                        "ip-prefix": "10.23.15.0/26",
                        "masklength-range": "exact"
                      }
                    }
                  ]
                }
              },
              {
                "name": "IPV4-prefix-set-2",
                "config": {
                  "name": "IPV4-prefix-set-2",
                  "mode": "IPV4"
                },
                "prefixes": {
                  "prefix": [
                    {
                      "ip-prefix": "10.23.0.0/16",
                      "masklength-range": "exact",
                      "config": {
                        "ip-prefix": "10.23.0.0/16",
                        "masklength-range": "exact"
                      }
                    }
                  ]
                }
              },
              {
                "name": "IPV4-prefix-set-3",
                "config": {
                  "name": "IPV4-prefix-set-3",
                  "mode": "IPV4"
                },
                "prefixes": {
                  "prefix": [
                    {
                      "ip-prefix": "10.23.15.0/26",
                      "masklength-range": "exact",
                      "config": {
                        "ip-prefix": "10.23.15.0/26",
                        "masklength-range": "exact"
                      }
                    },
                    {
                      "ip-prefix": "10.23.17.0/26",
                      "masklength-range": "exact",
                      "config": {
                        "ip-prefix": "10.23.17.0/26",
                        "masklength-range": "exact"
                      }
                    }
                  ]
                }
              }
            ]
          }
        }
      }
    }
    ```
* For IPv6:
  * Create three prefix-sets as below:
  * IPv6-prefix-set-1  - exact match on 2001:4860:f804::/48
  * IPv6-prefix-set-2  - 65-128 match on ::/0
  * [TODO] IPv6-prefix-set-3  - exact match on 2001:4860:f804::/48, 2001:4860:f806::/48
    ```
    {
      "openconfig-routing-policy:routing-policy": {
        "defined-sets": {
          "prefix-sets": {
            "prefix-set": [
              {
                "name": "IPV6-prefix-set-1",
                "config": {
                  "name": "IPV6-prefix-set-1",
                  "mode": "IPV6"
                },
                "prefixes": {
                  "prefix": [
                    {
                      "ip-prefix": "2001:4860:f804::/48",
                      "masklength-range": "exact",
                      "config": {
                        "ip-prefix": "2001:4860:f804::/48",
                        "masklength-range": "exact"
                      }
                    }
                  ]
                }
              },
              {
                "name": "IPV6-prefix-set-2",
                "config": {
                  "name": "IPV6-prefix-set-2",
                  "mode": "IPV6"
                },
                "prefixes": {
                  "prefix": [
                    {
                      "ip-prefix": "::/0",
                      "masklength-range": "65..128",
                      "config": {
                        "ip-prefix": "::/0",
                        "masklength-range": "65..128"
                      }
                    }
                  ]
                }
              },
              {
                "name": "IPV6-prefix-set-3",
                "config": {
                  "name": "IPV6-prefix-set-3",
                  "mode": "IPV6"
                },
                "prefixes": {
                  "prefix": [
                    {
                      "ip-prefix": "2001:4860:f804::/48",
                      "masklength-range": "exact",
                      "config": {
                        "ip-prefix": "2001:4860:f804::/48",
                        "masklength-range": "exact"
                      }
                    },
                    {
                      "ip-prefix": "2001:4860:f806::/48",
                      "masklength-range": "exact",
                      "config": {
                        "ip-prefix": "2001:4860:f806::/48",
                        "masklength-range": "exact"
                      }
                    }
                  ]
                }
              }
            ]
          }
        }
      }
    }
    ```
### RT-1.33.1 mach with option ANY  
* For IPv4 and IPv6:
  * Configure BGP policy on DUT to allow routes based on IPv4-prefix-set-2 and reject routes based on IPv4-prefix-set-1 
  *	Configure BGP policy on DUT to allow routes based on IPv6-prefix-set-1
  *	and reject routes based on IPv6-prefix-set-2 
  *	Validate that the prefixes are accepted after policy application.
  *	DUT conditionally advertises prefixes received from ATE port-1 to ATE port-2 after policy application. Ensure that multiple routes are accepted and advertised to the neighbor on ATE port-2.

### [TODO] RT-1.33.2 match with option INVERT in ingress policy
* Test configuration
  * Generate new policies (bgpInvertIPv4, bgpInvertPv6)
    * Configure BGP policy on DUT to reject IPv4 routes that are NOT covered in IPv4-prefix-set-3 using `INVERT` match-type-option; Allow any other IPv4 route.
      ```
      {
        "openconfig-routing-policy:routing-policy": {
          "policy-definitions": {
            "policy-definition": [
              {
                "name": "bgpInvertIPv4",
                "config": {
                  "name": "bgpInvertIPv4"
                },
                "statements": {
                  "statement": [
                    {
                      "name": "stm1",
                      "openconfig-routing-policy:config": {
                        "name": "stm1"
                      },
                      "openconfig-routing-policy:conditions": {
                        "match-prefix-set": {
                          "config": {
                            "prefix-set": "IPV4-prefix-set-3",
                            "match-set-options": "INVERT"
                          }
                        }
                      },
                      "openconfig-routing-policy:actions": {
                        "config": {
                          "policy-result": "REJECT_ROUTE"
                        }
                      }
                    },
                    {
                      "name": "stm2",
                      "openconfig-routing-policy:config": {
                        "name": "stm2"
                      },
                      "openconfig-routing-policy:actions": {
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
    * Configure BGP policy on DUT to reject IPv6 routes that are NOT covered in IPv6-prefix-set-3 using `INVERT` match-type-option; Allow any other IPv6 route.
      ```
      {
        "openconfig-routing-policy:routing-policy": {
          "policy-definitions": {
            "policy-definition": [
              {
                "name": "bgpInvertIPV6",
                "config": {
                  "name": "bgpInvertIPV6"
                },
                "statements": {
                  "statement": [
                    {
                      "name": "stm1",
                      "openconfig-routing-policy:config": {
                        "name": "stm1"
                      },
                      "openconfig-routing-policy:conditions": {
                        "match-prefix-set": {
                          "config": {
                            "prefix-set": "IPV6-prefix-set-3",
                            "match-set-options": "INVERT"
                          }
                        }
                      },
                      "openconfig-routing-policy:actions": {
                        "config": {
                          "policy-result": "REJECT_ROUTE"
                        }
                      }
                    },
                    {
                      "name": "stm2",
                      "openconfig-routing-policy:config": {
                        "name": "stm2"
                      },
                      "openconfig-routing-policy:actions": {
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
  * Attach bgpInvertIPv4, bgpInvertIPv6 as import policies to DUT port-1 eBGP session
    ```
    {
      "openconfig-network-instance:network-instances": {
        "network-instance": [
          {
            "name": "DEFAULT",
            "config": {
              "name": "DEFAULT"
            },
            "protocols": {
              "protocol": [
                {
                  "identifier": "openconfig-policy-types:BGP",
                  "name": "DEFAULT",
                  "config": {
                    "identifier": "openconfig-policy-types:BGP",
                    "name": "DEFAULT"
                  },
                  "bgp": {
                    "neighbors": {
                      "neighbor": [
                        {
                          "neighbor-address": "<IPv4-port1-ATE-address>",
                          "config": {
                            "neighbor-address": "<IPv4-port1-ATE-address>"
                          },
                          "afi-safis": {
                            "afi-safi": [
                              {
                                "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                                "config": {
                                  "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST"
                                },
                                "apply-policy": {
                                  "config": {
                                    "import-policy": "bgpInvertIPv4",
                                    "default-import-policy": "ACCEPT_ROUTE",
                                    "defaul-export-policy": "ACCEPT_ROUTE"
                                  }
                                }
                              }
                            ]
                          }
                        },
                        {
                          "neighbor-address": "<IPv6-port1-ATE-address>",
                          "config": {
                            "neighbor-address": "<IPv6-port1-ATE-address>"
                          },
                          "afi-safis": {
                            "afi-safi": [
                              {
                                "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                                "config": {
                                  "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST"
                                },
                                "apply-policy": {
                                  "config": {
                                    "import-policy": "bgpInvertIPv6",
                                    "default-import-policy": "ACCEPT_ROUTE",
                                    "defaul-export-policy": "ACCEPT_ROUTE"
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
              ]
            }
          }
        ]
      }
    }
    ```
  * Push the generated configuration to the DUT using gnmi.Set with replace option.
  * Advertise from OTG port-1 BGP prefixes:
    * 10.23.15.0/26, 10.23.16.0/26, 10.23.17.0/26
    * 2001:4860:f804::/48, 2001:4860:f805::/48, 2001:4860:f806::/48
* Behaviour validation
  * validate that prefixes 10.23.15.0/26, 10.23.17.0/26, 2001:4860:f804::/48, 2001:4860:f806::/48 are received by OTG port-2 BGP speaker
  * validate that prefixes 10.23.16.0/26, 2001:4860:f805::/48 are **NOT** received by OTG port-2 BGP speaker

### [TODO] RT-1.33.3 match with option INVERT in egress policy
* Test configuration
  * Generate the same config as for RT-1.33.2 above, with following modification:
  * Attach bgpInvertIPv4, bgpInvertIPv6 as export policies to DUT port-1 eBGP session
    ```
    {
      "openconfig-network-instance:network-instances": {
        "network-instance": [
          {
            "name": "DEFAULT",
            "config": {
              "name": "DEFAULT"
            },
            "protocols": {
              "protocol": [
                {
                  "identifier": "openconfig-policy-types:BGP",
                  "name": "DEFAULT",
                  "config": {
                    "identifier": "openconfig-policy-types:BGP",
                    "name": "DEFAULT"
                  },
                  "bgp": {
                    "neighbors": {
                      "neighbor": [
                        {
                          "neighbor-address": "<IPv4-port1-ATE-address>",
                          "config": {
                            "neighbor-address": "<IPv4-port1-ATE-address>"
                          },
                          "afi-safis": {
                            "afi-safi": [
                              {
                                "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                                "config": {
                                  "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST"
                                },
                                "apply-policy": {
                                  "config": {
                                    "import-policy": "bgpInvertIPv4",
                                    "default-import-policy": "ACCEPT_ROUTE",
                                    "defaul-export-policy": "ACCEPT_ROUTE"
                                  }
                                }
                              }
                            ]
                          }
                        },
                        {
                          "neighbor-address": "<IPv6-port1-ATE-address>",
                          "config": {
                            "neighbor-address": "<IPv6-port1-ATE-address>"
                          },
                          "afi-safis": {
                            "afi-safi": [
                              {
                                "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                                "config": {
                                  "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST"
                                },
                                "apply-policy": {
                                  "config": {
                                    "import-policy": "bgpInvertIPv6",
                                    "default-import-policy": "ACCEPT_ROUTE",
                                    "defaul-export-policy": "ACCEPT_ROUTE"
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
              ]
            }
          }
        ]
      }
    }
    ```
  * Push the generated configuration to the DUT using gnmi.Set with replace option.
* Behaviour validation
  * use the same validation as for RT-1.33.2 above

## Telemetry Parameter coverage
N/A
Protocol/RPC Parameter coverage
N/A
Minimum DUT platform requirement
vRX

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml 
paths:
  ## Config paths
  /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode:
  /routing-policy/defined-sets/prefix-sets/prefix-set/config/name:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set:

  ## State paths
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/installed:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received-pre-policy:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/sent:
  /routing-policy/policy-definitions/policy-definition/statements/statement/state/name:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```
