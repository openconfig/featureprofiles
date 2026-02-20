# TRANSCEIVER-101: Telemetry: ZR platform OC paths streaming.

## Summary

Validate ZR optics module reports telemetry data for all leaves in

```yaml
    /components/component/:
        platform_type: ["PORT"]
    /components/component/:
        platform_type: ["TRANSCEIVER"]
    /components/component/:
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
    *   platform/components/component/state/parent
    *   platform/components/component/state/location
    *   platform/components/component/state/type
    *   platform/components/component/state/firmware-version
    *   platform/components/component/state/hardware-version
    *   platform/components/component/state/serial-no
    *   platform/components/component/state/part-no
    *   platform/components/component/state/mfg-name
    *   platform/components/component/state/mfg-date
    *   platform/components/component/state/temperature/instant
    *   platform/components/component/port/breakout-mode/groups/group/state/index
    *   platform/components/component/port/breakout-mode/groups/group/state/breakout-speed
    *   platform/components/component/port/breakout-mode/groups/group/state/num-breakouts
    *   platform/components/component/port/breakout-mode/groups/group/state/num-physical-channels
    *   platform/components/component/transceiver/state/form-factor
    *   platform/components/component/transceiver/state/connector-type
    *   platform/components/component/transceiver/state/supply-voltage/instant
    *   platform/components/component/transceiver/physical-channels/channel/state/index
    *   platform/components/component/transceiver/physical-channels/channel/state/output-power/instant
    *   platform/components/component/transceiver/physical-channels/channel/state/input-power/instant
    *   platform/components/component/transceiver/physical-channels/channel/state/input-power/avg
    *   platform/components/component/transceiver/physical-channels/channel/state/input-power/min
    *   platform/components/component/transceiver/physical-channels/channel/state/input-power/max
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
    /components/component/state/name:
        platform_type: ["PORT", "TRANSCEIVER", "OPTICAL_CHANNEL"]
    /components/component/state/location:
        platform_type: ["PORT", "TRANSCEIVER", "OPTICAL_CHANNEL"]
    /components/component/state/type:
        platform_type: ["PORT", "TRANSCEIVER", "OPTICAL_CHANNEL"]
    /components/component/state/parent:
        platform_type: ["PORT", "TRANSCEIVER", "OPTICAL_CHANNEL"]
    /components/component/port/breakout-mode/groups/group/state/index:
        platform_type: ["PORT"]
    /components/component/port/breakout-mode/groups/group/state/breakout-speed:
        platform_type: ["PORT"]
    /components/component/port/breakout-mode/groups/group/state/num-breakouts:
        platform_type: ["PORT"]
    /components/component/port/breakout-mode/groups/group/state/num-physical-channels:
        platform_type: ["PORT"]
    /components/component/state/oper-status:
        platform_type: ["TRANSCEIVER"]
    /components/component/state/temperature/instant:
        platform_type: ["TRANSCEIVER"]
    /components/component/state/firmware-version:
        platform_type: ["TRANSCEIVER"]
    /components/component/state/hardware-version:
        platform_type: ["TRANSCEIVER"]
    /components/component/state/serial-no:
        platform_type: ["TRANSCEIVER"]
    /components/component/state/part-no:
        platform_type: ["TRANSCEIVER"]
    /components/component/state/mfg-name:
        platform_type: ["TRANSCEIVER"]
    /components/component/state/mfg-date:
        platform_type: ["TRANSCEIVER"]
    /components/component/transceiver/state/form-factor:
        platform_type: ["TRANSCEIVER"]
    /components/component/transceiver/state/connector-type:
        platform_type: ["TRANSCEIVER"]
    /components/component/transceiver/state/supply-voltage/instant:
        platform_type: ["TRANSCEIVER"]
    /components/component/transceiver/physical-channels/channel/state/index:
        platform_type: ["TRANSCEIVER"]
    /components/component/transceiver/physical-channels/channel/state/output-power/instant:
        platform_type: ["TRANSCEIVER"]
    /components/component/transceiver/physical-channels/channel/state/input-power/instant:
        platform_type: ["TRANSCEIVER"]
    /components/component/transceiver/physical-channels/channel/state/input-power/avg:
        platform_type: ["TRANSCEIVER"]
    /components/component/transceiver/physical-channels/channel/state/input-power/min:
        platform_type: ["TRANSCEIVER"]
    /components/component/transceiver/physical-channels/channel/state/input-power/max:
        platform_type: ["TRANSCEIVER"]
    /components/component/optical-channel/state/operational-mode:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/frequency:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/target-output-power:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/laser-bias-current/instant:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/input-power/instant:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/input-power/avg:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/input-power/min:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/input-power/max:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/output-power/instant:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/output-power/avg:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/output-power/min:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/output-power/max:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/chromatic-dispersion/instant:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/chromatic-dispersion/avg:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/chromatic-dispersion/min:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/chromatic-dispersion/max:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/carrier-frequency-offset/instant:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/carrier-frequency-offset/avg:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/carrier-frequency-offset/min:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/carrier-frequency-offset/max:
        platform_type: ["OPTICAL_CHANNEL"]

rpcs:
    gnmi:
        gNMI.Get:
        gNMI.Set:
        gNMI.Subscribe:
```


