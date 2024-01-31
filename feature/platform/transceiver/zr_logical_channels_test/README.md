# TRANSCEIVER-11: Telemetry: 400ZR Optics logical channels provisioning and related telemetry.

## Summary

Routing devices that support transceivers with built-in DSPs like 400ZR consume
the [OC-terminal-device model](https://openconfig.net/projects/models/schemadocs/jstree/openconfig-terminal-device.html)
model.
The ZR signal in these transceivers traverses through a series of
terminal-device/logical-channels. The series of logical-channel utilizes the
assignment/optical-channel leaf to create the relationship to
OPTICAL_CHANNEL. For 400ZR 1x400GE mode this heirarchy looks like:
400GE Eth. Logical Channel => 400G Coherent Logical Channel => OPTICAL_CHANNEL
Purpose of this test is to verify the logical channel provisning and related
telemetry.

## Procedure

*   Connect two ZR interfaces using a duplex LC fiber jumper such that TX
    output power of one is the RX input power of the other module.
*   To establish a point to point ZR link ensure the following:
      * Provision the ZR modules using appropriate 
        terminal-device(logical-channels) and
        platforms-comonents(Optical-channel) configuration through the
        following OC paths:
        * /terminal-device/logical-channels/channel/config/admin-state
        * /terminal-device/logical-channels/channel/config/description
        * /terminal-device/logical-channels/channel/config/index
        * /terminal-device/logical-channels/channel/config/logical-channel-type
        * /terminal-device/logical-channels/channel/config/rate-class
        * /terminal-device/logical-channels/channel/config/trib-protocol
        * /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/allocation
        * /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/assignment-type
        * /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/description
        * /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/index
        * /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/logical-channel
        * /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/optical-channel

*   Also ensure optical channel related tunable parameters are set through the
    following OC paths such that
      * Both transceivers state is enabled
      * Both transceivers related optical channel tunable parameters are set
        to a valid target TX output power example -10 dBm
      * Both transceivers are tuned to a valid centre frequency
        example 193.1 THz
        * /components/component/transceiver/config/enabled
        * /components/component/optical-channel/config/frequency
        * /components/component/optical-channel/config/target-output-power

*   Each logical-channel created above must be assigned an integer value. These
    interger values can be anything as long as these do not overlap between
    configuration on two different ports.

*   For Ethernet Logical Channel verify the following parameters are streamed:
      * logical-channel-type: PROT_ETHERNET
      * trib-protocol: PROT_400GE
      * rate-class: TRIB_RATE_400G
      * admin-state: ENABLED
      * description: ETH Logical Channel
      * index: 40000 (unique integer value)

*   For Coherent LogicalChannel verify the following parameters are streamed:
      * logical-channel-type: PROT_OTN
      * admin-state: ENABLED
      * description: Coherent Logical Channel
      * index: 40004(unique integer value)

## Config Parameter coverage

*   /components/component/transceiver/config/enabled
*   /interfaces/interface/config/enabled 
*   /terminal-device/logical-channels/channel/config/admin-state
*   /terminal-device/logical-channels/channel/config/description
*   /terminal-device/logical-channels/channel/config/index
*   /terminal-device/logical-channels/channel/config/logical-channel-type
*   /terminal-device/logical-channels/channel/config/rate-class
*   /terminal-device/logical-channels/channel/config/trib-protocol
*   /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/allocation
*   /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/assignment-type
*   /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/description
*   /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/index
*   /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/logical-channel
*   /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/optical-channel

## Telemetry Parameter coverage

*   /components/component/transceiver/config/enabled
*   /interfaces/interface/config/enabled 
*   /terminal-device/logical-channels/channel/state/admin-state
*   /terminal-device/logical-channels/channel/state/description
*   /terminal-device/logical-channels/channel/state/index
*   /terminal-device/logical-channels/channel/state/logical-channel-type
*   /terminal-device/logical-channels/channel/state/rate-class
*   /terminal-device/logical-channels/channel/state/trib-protocol
*   /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/allocation
*   /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/assignment-type
*   /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/description
*   /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/index
*   /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/logical-channel
*   /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/optical-channel