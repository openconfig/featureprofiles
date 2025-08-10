# TRANSCEIVER-101: Telemetry: ZR platform OC paths streaming.

## Summary

Validate ZR optics module reports telemetry data for all leaves in

```yaml
    platform/components/component/:
        platform_type: ["PORT"]
    platform/components/component/:
        platform_type: ["TRANSCEIVER"]
    platform/components/component/:
        platform_type: ["OPTICAL_CHANNEL"]
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

    *   platform/components/component/state/name
    *   platform/components/component/state/location
    *   platform/components/component/state/type
    *   platform/components/component/port/breakout-mode/groups/group/state/index
    *   platform/components/component/port/breakout-mode/groups/group/state/breakout-speed
    *   platform/components/component/port/breakout-mode/groups/group/state/num-breakouts
    *   platform/components/component/port/breakout-mode/groups/group/state/num-physical-channels
    *   platform/components/component/state/name
    *   platform/components/component/state/parent
    *   platform/components/component/state/location
    *   platform/components/component/state/removable
    *   platform/components/component/state/type
    *   platform/components/component/state/oper-status
    *   platform/components/component/state/temperature/instant
    *   platform/components/component/state/firmware-version
    *   platform/components/component/state/hardware-version
    *   platform/components/component/state/serial-no
    *   platform/components/component/state/part-no
    *   platform/components/component/state/mfg-name
    *   platform/components/component/state/mfg-date
    *   platform/components/component/transceiver/state/form-factor
    *   platform/components/component/transceiver/state/present
    *   platform/components/component/transceiver/state/connector-type
    *   platform/components/component/transceiver/state/supply-voltage/instant
    *   platform/components/component/transceiver/physical-channels/channel/state/index
    *   platform/components/component/transceiver/physical-channels/channel/state/output-power/instant
    *   platform/components/component/transceiver/physical-channels/channel/state/input-power/instant
    *   platform/components/component/transceiver/physical-channels/channel/state/input-power/avg
    *   platform/components/component/transceiver/physical-channels/channel/state/input-power/min
    *   platform/components/component/transceiver/physical-channels/channel/state/input-power/max
    *   platform/components/component/state/name
    *   platform/components/component/state/parent
    *   platform/components/component/state/type
    *   platform/components/component/optical-channel/state/operational-mode
    *   platform/components/component/optical-channel/state/frequency
    *   platform/components/component/optical-channel/state/target-output-power
    *   platform/components/component/optical-channel/state/laser-bias-current/instant
    *   platform/components/component/optical-channel/state/input-power/instant
    *   platform/components/component/optical-channel/state/input-power/avg
    *   platform/components/component/optical-channel/state/input-power/min
    *   platform/components/component/optical-channel/state/input-power/max
    *   platform/components/component/optical-channel/state/output-power/instant
    *   platform/components/component/optical-channel/state/output-power/avg
    *   platform/components/component/optical-channel/state/output-power/min
    *   platform/components/component/optical-channel/state/output-power/max
    *   platform/components/component/optical-channel/state/chromatic-dispersion/instant
    *   platform/components/component/optical-channel/state/chromatic-dispersion/avg
    *   platform/components/component/optical-channel/state/chromatic-dispersion/min
    *   platform/components/component/optical-channel/state/chromatic-dispersion/max
    *   platform/components/component/optical-channel/state/carrier-frequency-offset/instant
    *   platform/components/component/optical-channel/state/carrier-frequency-offset/avg
    *   platform/components/component/optical-channel/state/carrier-frequency-offset/min
    *   platform/components/component/optical-channel/state/carrier-frequency-offset/max

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
                        "target-output-power": -7,
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
                        "logical-channel-type": "openconfig-transport-types:PROT_OTN",
                    },
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

## OpenConfig Path and RPC Coverage:

```yaml
paths:
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
    platform/components/component/state/name:
        platform_type: ["PORT"]
    platform/components/component/state/location:
        platform_type: ["PORT"]
    platform/components/component/state/type:
        platform_type: ["PORT"]
    platform/components/component/port/breakout-mode/groups/group/state/index:
        platform_type: ["PORT"]
    platform/components/component/port/breakout-mode/groups/group/state/breakout-speed:
        platform_type: ["PORT"]
    platform/components/component/port/breakout-mode/groups/group/state/num-breakouts:
        platform_type: ["PORT"]
    platform/components/component/port/breakout-mode/groups/group/state/num-physical-channels:
        platform_type: ["PORT"]
    platform/components/component/state/name:
        platform_type: ["Tranceiver"]
    platform/components/component/state/parent:
        platform_type: ["Tranceiver"]
    platform/components/component/state/location:
        platform_type: ["Tranceiver"]
    platform/components/component/state/removable:
        platform_type: ["Tranceiver"]
    platform/components/component/state/type:
        platform_type: ["Tranceiver"]
    platform/components/component/state/oper-status:
        platform_type: ["Tranceiver"]
    platform/components/component/state/temperature/instant:
        platform_type: ["Tranceiver"]
    platform/components/component/state/firmware-version:
        platform_type: ["Tranceiver"]
    platform/components/component/state/hardware-version:
        platform_type: ["Tranceiver"]
    platform/components/component/state/serial-no:
        platform_type: ["Tranceiver"]
    platform/components/component/state/part-no:
        platform_type: ["Tranceiver"]
    platform/components/component/state/mfg-name:
        platform_type: ["Tranceiver"]
    platform/components/component/state/mfg-date:
        platform_type: ["Tranceiver"]
    platform/components/component/transceiver/state/form-factor:
        platform_type: ["Tranceiver"]
    platform/components/component/transceiver/state/present:
        platform_type: ["Tranceiver"]
    platform/components/component/transceiver/state/connector-type:
        platform_type: ["Tranceiver"]
    platform/components/component/transceiver/state/supply-voltage/instant:
        platform_type: ["Tranceiver"]
    platform/components/component/transceiver/physical-channels/channel/state/index:
        platform_type: ["Tranceiver"]
    platform/components/component/transceiver/physical-channels/channel/state/output-power/instant:
        platform_type: ["Tranceiver"]
    platform/components/component/transceiver/physical-channels/channel/state/input-power/instant:
        platform_type: ["Tranceiver"]
    platform/components/component/transceiver/physical-channels/channel/state/input-power/avg:
        platform_type: ["Tranceiver"]
    platform/components/component/transceiver/physical-channels/channel/state/input-power/min:
        platform_type: ["Tranceiver"]
    platform/components/component/transceiver/physical-channels/channel/state/input-power/max:
        platform_type: ["Tranceiver"]
    platform/components/component/state/name:
        platform_type: ["OPTICAL_CHANNEL"]
    platform/components/component/state/parent:
        platform_type: ["OPTICAL_CHANNEL"]
    platform/components/component/state/type:
        platform_type: ["OPTICAL_CHANNEL"]
    platform/components/component/optical-channel/state/operational-mode:
        platform_type: ["OPTICAL_CHANNEL"]
    platform/components/component/optical-channel/state/frequency:
        platform_type: ["OPTICAL_CHANNEL"]
    platform/components/component/optical-channel/state/target-output-power:
        platform_type: ["OPTICAL_CHANNEL"]
    platform/components/component/optical-channel/state/laser-bias-current/instant:
        platform_type: ["OPTICAL_CHANNEL"]
    platform/components/component/optical-channel/state/input-power/instant:
        platform_type: ["OPTICAL_CHANNEL"]
    platform/components/component/optical-channel/state/input-power/avg:
        platform_type: ["OPTICAL_CHANNEL"]
    platform/components/component/optical-channel/state/input-power/min:
        platform_type: ["OPTICAL_CHANNEL"]
    platform/components/component/optical-channel/state/input-power/max:
        platform_type: ["OPTICAL_CHANNEL"]
    platform/components/component/optical-channel/state/output-power/instant:
        platform_type: ["OPTICAL_CHANNEL"]
    platform/components/component/optical-channel/state/output-power/avg:
        platform_type: ["OPTICAL_CHANNEL"]
    platform/components/component/optical-channel/state/output-power/min:
        platform_type: ["OPTICAL_CHANNEL"]
    platform/components/component/optical-channel/state/output-power/max:
        platform_type: ["OPTICAL_CHANNEL"]
    platform/components/component/optical-channel/state/chromatic-dispersion/instant:
        platform_type: ["OPTICAL_CHANNEL"]
    platform/components/component/optical-channel/state/chromatic-dispersion/avg:
        platform_type: ["OPTICAL_CHANNEL"]
    platform/components/component/optical-channel/state/chromatic-dispersion/min:
        platform_type: ["OPTICAL_CHANNEL"]
    platform/components/component/optical-channel/state/chromatic-dispersion/max:
        platform_type: ["OPTICAL_CHANNEL"]
    platform/components/component/optical-channel/state/carrier-frequency-offset/instant:
        platform_type: ["OPTICAL_CHANNEL"]
    platform/components/component/optical-channel/state/carrier-frequency-offset/avg:
        platform_type: ["OPTICAL_CHANNEL"]
    platform/components/component/optical-channel/state/carrier-frequency-offset/min:
        platform_type: ["OPTICAL_CHANNEL"]
    platform/components/component/optical-channel/state/carrier-frequency-offset/max:
        platform_type: ["OPTICAL_CHANNEL"]

rpcs:
    gnmi:
        gNMI.Get:
        gNMI.Set:
        gNMI.Subscribe:
```


