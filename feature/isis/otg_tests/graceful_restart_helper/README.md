# RT-2.16: IS-IS Graceful Restart Helper

## Summary

- test verify isis garceful restarts support in helper role

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

## Procedure

#### Initial Setup:

*   Connect:
  *  DUT port-1 to ATE port-1
  *  DUT port-2 to ATE port-2

*   Configure IPv4 and IPv6 addresses on DUT and ATE ports as shown below
    *   DUT port-1 IPv4 address ```dp1-v4 = 192.168.1.1/30```
    *   ATE port-1 IPv4 address ```ap1-v4 = 192.168.1.2/30```

    *   DUT port-2 IPv4 address ```dp2-v4 = 192.168.1.5/30```
    *   ATE port-2 IPv4 address ```ap2-v4 = 192.168.1.6/30```

    *   DUT port-1 IPv6 address ```dp1-v6 = 2001:DB8::1/126```
    *   ATE port-1 IPv6 address ```ap1-v6 = 2001:DB8::2/126```

    *   DUT port-2 IPv6 address ```dp2-v6 = 2001:DB8::5/126```
    *   ATE port-2 IPv6 address ```ap2-v6 = 2001:DB8::6/126```

*   Create an "target IPv4" network i.e. ```ipv4-network = 192.168.10.0/24``` attached to ATE port-2 and inject it to ISIS.

*   Create an "target IPv6" network i.e. ```ipv6-network = 2024:db8:128:128::/64``` attached to ATE port-2 and inject it to ISIS.

*   Configure ISIS
    * Configure separate ISIS emulated routers, one on each  ATE ports-1, port-2 
    * Enable IPv4 and IPv6 IS-IS L2 adjacency between ATE port-1 and DUT port-1, DUT port-2 and ATE port-2 in point-to-point mode.

        ```json
        {
            "network-instances": {
                "network-instance": [
                    {
                        "name": "DEFAULT",
                        "protocols": {
                            "protocol": [
                                {
                                    "identifier": "ISIS",
                                    "name": "DEFAULT",
                                    "config": {
                                        "name": "DEFAULT",
                                        "identifier": "ISIS"
                                    },
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
                                                    },
                                                    {
                                                        "afi-name": "IPV6",
                                                        "config": {
                                                            "afi-name": "IPV6",
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
                                                    "<NET address of this WBB>"
                                                ]
                                            }
                                        },
                                        "interfaces": {
                                            "interface": [
                                                {
                                                    "config": {
                                                        "passive": true,
                                                        "enabled": true,
                                                        "interface-id": "Loopback0"
                                                    },
                                                    "interface-id": "Loopback0",
                                                    "interface-ref": {
                                                        "config": {
                                                            "interface": "loopback0",
                                                            "subinterface": 0
                                                        }
                                                    },
                                                    "levels": {
                                                        "level": [
                                                            {
                                                                "config": {
                                                                    "level-number": 2,
                                                                    "enabled": true
                                                                },
                                                                "level-number": 2
                                                            }
                                                        ]
                                                    }
                                                },
                                                {
                                                    "config": {
                                                        "circuit-type": "POINT_TO_POINT",
                                                        "enabled": true,
                                                        "interface-id": "<Interface_ID>"
                                                    },
                                                    "interface-id": "<Interface_ID>",
                                                    "interface-ref": {
                                                        "config": {
                                                            "interface": "<Interface name>",
                                                            "subinterface": 0
                                                        }
                                                    },
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
                                                                        },
                                                                        {
                                                                            "afi-name": "IPV6",
                                                                            "config": {
                                                                                "afi-name": "IPV6",
                                                                                "metric": 10,
                                                                                "safi-name": "UNICAST"
                                                                            },
                                                                            "safi-name": "UNICAST"
                                                                        }
                                                                    ]
                                                                },
                                                                "config": {
                                                                    "level-number": 2,
                                                                    "enabled": true
                                                                },
                                                                "level-number": 2,
                                                                "timers": {
                                                                    "config": {
                                                                        "hello-interval": 10,
                                                                        "hello-multiplier": 6
                                                                    }
                                                                }
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
                                                        "level-number": 2,
                                                        "metric-style": "WIDE_METRIC",
                                                        "enabled": true
                                                    },
                                                    "level-number": 2
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
    * Enable IPv4 and IPv6 IS-IS L2 adjacency between ATE port-1 and DUT port-1, DUT port-2 and ATE port-2 in point-to-point mode.\
    *   Set ISIS graceful restart helper mode on DUT

        ```json
        {
            "network-instances": {
                "network-instance": [
                    {
                        "name": "DEFAULT",
                        "protocols": {
                            "protocol": [
                                {
                                    "identifier": "ISIS",
                                    "name": "DEFAULT",
                                    "isis": {
                                        "global": {
                                            "graceful-restart": {
                                                "config": {
                                                    "enabled": true,
                                                    "helper-only": true,
                                                    "restart-time": 30
                                                }
                                            }
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

### RT-2.16.1 [TODO: https://github.com/openconfig/featureprofiles/issues/2494]
#### GR helper

*   Generate traffic form ATE port-1 to "target IPv4" and "target IPv6" networks (ATE port-2)
*   Verify traffic is recived on ATE port-2
*   Restart ISIS on ATE port-2 (Alternativly: using set_control_state to "down" for emulated isis router. Wait (restart-time - 10) sec and set it back to "up")
*   Verify traffic is recived on ATE port-2 during restart time ( no losses )
*   Disable ISIS on ATE port-2 (set_control_state to "down"). Wait restart-time seconds
*   Verify traffic is NOT recived on ATE port-2 (after restart-time expires)

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

```yaml
paths:
  ## Config Paths ##
  /network-instances/network-instance/protocols/protocol/isis/global/graceful-restart/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/global/graceful-restart/config/helper-only:
  /network-instances/network-instance/protocols/protocol/isis/global/graceful-restart/config/restart-time:
  
rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
```

## Required DUT platform

* FFF