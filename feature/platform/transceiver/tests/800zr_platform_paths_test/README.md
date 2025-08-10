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

**Note:** For min, max, and avg values, 10 second sampling is preferred. If 
          10 seconds is not supported, the sampling interval used must be
          specified by adding a deviation to the test.

### Canonical OC

```json
{
    "components": {
        "component": [
            {
                "name": "Ethernet4/1-Port",
                "port": {
                    "breakout-mode": {
                        "groups": {
                            "group": [
                                {
                                    "index": 0,
                                    "state": {
                                        "breakout-speed": "openconfig-if-ethernet:SPEED_800GB",
                                        "index": 0,
                                        "num-breakouts": 1,
                                        "num-physical-channels": 8
                                    }
                                }
                            ]
                        }
                    }
                },
                "state": {
                    "location": "4/1",
                    "name": "Ethernet4/1-Port",
                    "type": "openconfig-platform-types:PORT"
                }
            },
        ]
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

rpcs:
    gnmi:
        gNMI.Get:
        gNMI.Set:
        gNMI.Subscribe:
```


