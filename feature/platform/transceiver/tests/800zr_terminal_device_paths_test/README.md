# TRANSCEIVER-102: Telemetry: ZR terminal-device OC paths streaming.

## Summary

Validate ZR optics module reports telemetry data for all leaves in

```yaml
    terminal-device/logical-channels/channel/:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/:
        logical_channel_type: ["PROT_ETHERNET"]
```

## Procedure

*   Connect two ZR interfaces using a duplex LC fiber jumper such that TX
    output power of one is the RX input power of the other module.

*   To establish a point to point ZR link ensure the following:
      * Both transceivers state is enabled
      * Both transceivers are set to a valid operational mode
        example 1.      
      * Both transceivers are set to a valid target TX output power
        example -7 dBm.
      * Both transceivers are tuned to a valid centre frequency
        example 196.1 THz.

*   With the ZR link established as explained above, wait until
    both interfaces oper-status are UP and all the min/avg/max values are 
    populated. Then verify that the following ZR transceiver telemetry paths 
    exist and are streamed valid values for both ZR optics.

*   Emulate flaps with the following procedure:
    *   Enable a pair of ZR interfaces on the DUT as explained above.
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
    *  terminal-device/logical-channels/channel/state/index
    *  terminal-device/logical-channels/channel/state/description
    *  terminal-device/logical-channels/channel/state/logical-channel-type
    *  terminal-device/logical-channels/channel/state/loopback-mode
    *  terminal-device/logical-channels/channel/ingress/state/interface
    *  terminal-device/logical-channels/channel/ingress/state/transceiver
    *  terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/index
    *  terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/logical-channel
    *  terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/allocation
    *  terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/assignment-type

**Note:** For min, max, and avg values, 10 second sampling is preferred. If 
          10 seconds is not supported, the sampling interval used must be
          specified by adding a deviation to the test.


### Canonical OC
```json
{
    "components": {
        "component": [
            {
                "config": {
                    "name": "Ethernet4/1-Port"
                },
                "name": "Ethernet4/1-Port",
                "port": {
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
                    "name": "Ethernet4/1-Optical0"
                },
                "name": "Ethernet4/1-Optical0",
                "optical-channel": {
                    "config": {
                        "operational-mode": 1,            
                        "frequency": 192800000,
                        "target-output-power": -7
                    }
                }
            }
        ]
    },
    "terminal-device": {
        "logical-channels": {
            "channel": [
                {
                    "logical-channel-assignments": {
                        "assignment": [
                            {
                                "config": {
                                    "allocation": 400,
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
                                    "allocation": 400,
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
                        "rate-class": "openconfig-transport-types:TRIB_RATE_400G",
                        "trib-protocol": "openconfig-transport-types:PROT_400GE"
                    }
                }
            ]
        }
    }
}
```

## OpenConfig Path and RPC Coverag:

```yaml 
    # Config Parameter coverage
    interfaces/interface/enabled/config:
    interfaces/interface/type/config:
    interfaces/interface/ethernet/port-speed/config:
    interfaces/interface/ethernet/duplex-mode/config:
    platform/components/component/port/breakout-mode/groups/group/breakout-speed/config:
        platform_type: ["PORT"]
    platform/components/component/port/breakout-mode/groups/group/num-breakouts/config:
        platform_type: ["PORT"]
    platform/components/component/port/breakout-mode/groups/group/num-physical-channels/config:
        platform_type: ["PORT"]
    platform/components/component/optical-channel/operational-mode/config:
        platform_type: ["OPTICAL_CHANNEL"]
    platform/components/component/optical-channel/frequency/config:
        platform_type: ["OPTICAL_CHANNEL"]
    platform/components/component/optical-channel/target-output-power/config:
        platform_type: ["OPTICAL_CHANNEL"]
    terminal-device/logical-channels/channel/admin-state/cofig:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/description/cofig:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/logical-channel-type/cofig:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/logical-channel-assignments/assignment/description/cofig:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/logical-channel-assignments/assignment/assignment-type/cofig:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/logical-channel-assignments/assignment/optical-channel/cofig:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/logical-channel-assignments/assignment/allocation/cofig:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/admin-state/cofig:
        logical_channel_type: ["PROT_ETHERNET"]
    terminal-device/logical-channels/channel/description/cofig:
        logical_channel_type: ["PROT_ETHERNET"]
    terminal-device/logical-channels/channel/logical-channel-type/cofig:
        logical_channel_type: ["PROT_ETHERNET"]
    terminal-device/logical-channels/channel/ingress/transceiver/cofig:
        logical_channel_type: ["PROT_ETHERNET"]
    terminal-device/logical-channels/channel/ingress/interface/cofig:
        logical_channel_type: ["PROT_ETHERNET"]
    terminal-device/logical-channels/channel/logical-channel-assignments/assignment/description/cofig:
        logical_channel_type: ["PROT_ETHERNET"]
    terminal-device/logical-channels/channel/logical-channel-assignments/assignment/assignment-type/cofig:
        logical_channel_type: ["PROT_ETHERNET"]
    terminal-device/logical-channels/channel/logical-channel-assignments/assignment/logical-channel/cofig:
        logical_channel_type: ["PROT_ETHERNET"]
    terminal-device/logical-channels/channel/logical-channel-assignments/assignment/allocation/cofig:
        logical_channel_type: ["PROT_ETHERNET"]
    
    # Telemetry Parameter coverage
    terminal-device/logical-channels/channel/state/index:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/state/description:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/state/logical-channel-type:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/state/loopback-mode:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/index:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/optical-channel:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/description:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/allocation:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/assignment-type:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/otn/state/q-value/instant:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/otn/state/q-value/avg:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/otn/state/q-value/min:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/otn/state/q-value/max:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/otn/state/esnr/instant:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/otn/state/esnr/avg:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/otn/state/esnr/min:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/otn/state/esnr/max:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/otn/state/pre-fec-ber/instant:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/otn/state/pre-fec-ber/avg:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/otn/state/pre-fec-ber/min:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/otn/state/pre-fec-ber/max:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks:
        logical_channel_type: ["PROT_OTN"]
    terminal-device/logical-channels/channel/state/index:
        logical_channel_type: ["PROT_ETHERNET"]
    terminal-device/logical-channels/channel/state/description:
        logical_channel_type: ["PROT_ETHERNET"]
    terminal-device/logical-channels/channel/state/logical-channel-type:
        logical_channel_type: ["PROT_ETHERNET"]
    terminal-device/logical-channels/channel/state/loopback-mode:
        logical_channel_type: ["PROT_ETHERNET"]
    terminal-device/logical-channels/channel/ingress/state/interface:
        logical_channel_type: ["PROT_ETHERNET"]
    terminal-device/logical-channels/channel/ingress/state/transceiver:
        logical_channel_type: ["PROT_ETHERNET"]
    terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/index:
        logical_channel_type: ["PROT_ETHERNET"]
    terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/logical-channel:
        logical_channel_type: ["PROT_ETHERNET"]
    terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/allocation:
        logical_channel_type: ["PROT_ETHERNET"]
    terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/assignment-type:
        logical_channel_type: ["PROT_ETHERNET"]

rpcs:
    gnmi:
        gNMI.Get:
        gNMI.Set:
        gNMI.Subscribe:
```


