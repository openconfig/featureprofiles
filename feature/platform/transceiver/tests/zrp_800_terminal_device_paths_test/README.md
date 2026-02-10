# TRANSCEIVER-104: Telemetry: ZR Plus terminal-device OC paths streaming.

## Summary

Validate ZR Plus optics module reports telemetry data for all leaves in

```yaml
    /terminal-device/logical-channels/channel/:
```

## Procedure

*   Connect two ZR Plus interfaces using a duplex LC fiber jumper such that TX
    output power of one is the RX input power of the other module.

*   To establish a point to point ZR Plus link ensure the following:
      * Both transceivers state is enabled
      * Both transceivers are set to a valid operational mode
        example 1.      
      * Both transceivers are set to a valid target TX output power
        example -7 dBm.
      * Both transceivers are tuned to a valid centre frequency
        example 196.1 THz.

*   With the ZR Plus link established as explained above, wait until
    both interfaces oper-status are UP and all the min/avg/max values are 
    populated. Then verify that the following ZR Plus transceiver telemetry paths 
    exist and are streamed valid values for both ZR Plus optics.

*   Emulate flaps with the following procedure:
    *   Enable a pair of ZR Plus interfaces on the DUT as explained above.
    *   Disable interface and wait at least one sample interval.
    *   Enable interface.

*   Verify that all the static leaves (e.g., breakout-speed, part-no, ...) 
    are present and reports valid strings or enums before, during and after flap.

*   Verify all the other leaves reports valid value of decimal64 before, 
    during and after flap. For leaves with stats, ensure min <= avg/instant <= max.

    *  terminal-device/logical-channels/channel/state/index
    *  terminal-device/logical-channels/channel/state/description
    *  terminal-device/logical-channels/channel/state/logical-channel-type
    *  terminal-device/logical-channels/channel/state/loopback-mode
    *  terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/index
    *  terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/logical-channel
    *  terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/optical-channel
    *  terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/description
    *  terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/allocation
    *  terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/assignment-type
    *  terminal-device/logical-channels/channel/otn/state/q-value/instant
    *  terminal-device/logical-channels/channel/otn/state/q-value/avg
    *  terminal-device/logical-channels/channel/otn/state/q-value/min
    *  terminal-device/logical-channels/channel/otn/state/q-value/max
    *  terminal-device/logical-channels/channel/otn/state/esnr/instant
    *  terminal-device/logical-channels/channel/otn/state/esnr/avg
    *  terminal-device/logical-channels/channel/otn/state/esnr/min
    *  terminal-device/logical-channels/channel/otn/state/esnr/max
    *  terminal-device/logical-channels/channel/otn/state/pre-fec-ber/instant
    *  terminal-device/logical-channels/channel/otn/state/pre-fec-ber/avg
    *  terminal-device/logical-channels/channel/otn/state/pre-fec-ber/min
    *  terminal-device/logical-channels/channel/otn/state/pre-fec-ber/max
    *  terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks
    *  terminal-device/logical-channels/channel/ingress/state/interface
    *  terminal-device/logical-channels/channel/ingress/state/transceiver

**Note:** For min, max, and avg values, 10 second sampling is preferred. If 
          10 seconds is not supported, the sampling interval used must be
          specified by adding a deviation to the test.


### Canonical OC
```json
{
    "openconfig-interfaces:interfaces": {
        "interface": [
            {
                "config": {
                    "name": "Ethernet4/1/1",
                    "type": "ethernetCsmacd"
                },
                "name": "Ethernet4/1/1",
                "openconfig-if-ethernet:ethernet": {
                    "config": {
                        "duplex-mode": "FULL",
                        "port-speed": "SPEED_800GB"
                    }
                }
            }
        ]
    },
    "openconfig-platform:components": {
        "component": [
            {
                "config": {
                    "name": "Ethernet4/1-Port"
                },
                "name": "Ethernet4/1-Port",
                "openconfig-platform-port:port": {
                    "breakout-mode": {
                        "groups": {
                            "group": [
                                {
                                    "config": {
                                        "breakout-speed": "openconfig-if-ethernet:SPEED_800GB",
                                        "index": 1,
                                        "num-breakouts": 1,
                                        "num-physical-channels": 8
                                    },
                                    "index": 1
                                }
                            ]
                        }
                    }
                }
            },
            {
                "config": {
                    "name": "Ethernet4/1"
                },
                "name": "Ethernet4/1"
            },
            {
                "config": {
                    "name": "Ethernet4/1-Optical0"
                },
                "name": "Ethernet4/1-Optical0",
                "openconfig-terminal-device:optical-channel": {
                    "config": {
                        "operational-mode": 1,            
                        "frequency": "196000000",
                        "target-output-power": "-7" 
                    }
                }
            }
        ]
    },
    "openconfig-terminal-device:terminal-device": {
        "logical-channels": {
            "channel": [
                {
                    "logical-channel-assignments": {
                        "assignment": [
                            {
                                "config": {
                                    "allocation": "800",
                                    "assignment-type": "OPTICAL_CHANNEL",
                                    "description": "OTN to optical channel assignment",
                                    "index": 1,
                                    "optical-channel": "Ethernet4/1-Optical0"
                                }
                            }
                        ]
                    },
                    "config": {
                        "admin-state": "ENABLED",
                        "description": "OTN Logical Channel",
                        "index": 8000,
                        "logical-channel-type": "openconfig-transport-types:PROT_OTN"
                    }
                },
                {
                    "ingress": {
                        "config": {
                            "interface": "Ethernet4/1/1",
                            "transceiver": "Ethernet4/1"
                        }
                    },
                    "logical-channel-assignments": {
                        "assignment": [
                            {
                                "config": {
                                    "allocation": "800",
                                    "assignment-type": "LOGICAL_CHANNEL",
                                    "description": "ETH to OTN assignment",
                                    "index": 1,
                                    "logical-channel": 8000
                                }
                            }
                        ]
                    },
                    "config": {
                        "admin-state": "ENABLED",
                        "description": "ETH Logical Channel",
                        "index": 80000,
                        "logical-channel-type": "openconfig-transport-types:PROT_ETHERNET",
                        "rate-class": "openconfig-transport-types:TRIB_RATE_800G",
                        "trib-protocol": "openconfig-transport-types:PROT_800GE"
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
    # Config Parameter coverage
    /interfaces/interface/config/enabled:
    /interfaces/interface/config/type:
    /interfaces/interface/ethernet/config/port-speed:
    /interfaces/interface/ethernet/config/duplex-mode:
    /components/component/port/breakout-mode/groups/group/config/breakout-speed:
        platform_type: ["PORT"]
    /components/component/port/breakout-mode/groups/group/config/num-breakouts:
        platform_type: ["PORT"]
    /components/component/port/breakout-mode/groups/group/config/num-physical-channels:
        platform_type: ["PORT"]
    /components/component/optical-channel/config/operational-mode:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/config/frequency:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/config/target-output-power:
        platform_type: ["OPTICAL_CHANNEL"]
    /terminal-device/logical-channels/channel/config/admin-state:
    /terminal-device/logical-channels/channel/config/description:
    /terminal-device/logical-channels/channel/config/logical-channel-type:
    /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/description:
    /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/assignment-type:
    /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/allocation:
    /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/optical-channel:
    /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/logical-channel:
    /terminal-device/logical-channels/channel/ingress/config/transceiver:
    /terminal-device/logical-channels/channel/ingress/config/interface:
    
    # Telemetry Parameter coverage
    /terminal-device/logical-channels/channel/state/index:
    /terminal-device/logical-channels/channel/state/description:
    /terminal-device/logical-channels/channel/state/logical-channel-type:
    /terminal-device/logical-channels/channel/state/loopback-mode:
    /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/index:
    /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/description:
    /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/allocation:
    /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/assignment-type:
    /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/optical-channel:
    /terminal-device/logical-channels/channel/otn/state/q-value/instant:
    /terminal-device/logical-channels/channel/otn/state/q-value/avg:
    /terminal-device/logical-channels/channel/otn/state/q-value/min:
    /terminal-device/logical-channels/channel/otn/state/q-value/max:
    /terminal-device/logical-channels/channel/otn/state/esnr/instant:
    /terminal-device/logical-channels/channel/otn/state/esnr/avg:
    /terminal-device/logical-channels/channel/otn/state/esnr/min:
    /terminal-device/logical-channels/channel/otn/state/esnr/max:
    /terminal-device/logical-channels/channel/otn/state/pre-fec-ber/instant:
    /terminal-device/logical-channels/channel/otn/state/pre-fec-ber/avg:
    /terminal-device/logical-channels/channel/otn/state/pre-fec-ber/min:
    /terminal-device/logical-channels/channel/otn/state/pre-fec-ber/max:
    /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks:
    /terminal-device/logical-channels/channel/ingress/state/interface:
    /terminal-device/logical-channels/channel/ingress/state/transceiver:
    /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/logical-channel:

rpcs:
    gnmi:
        gNMI.Get:
        gNMI.Set:
        gNMI.Subscribe:
```


