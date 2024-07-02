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
Purpose of this test is to verify the logical channel provisioning and related
telemetry.

## Procedure

*   Connect two ZR interfaces using a duplex LC fiber jumper such that TX
    output power of one is the RX input power of the other module. Optics can be
    connected through passive patch panels or an optical switch as needed, as
    long as the overall link loss budget is kept under 2 - 3 dB. There is no
    requirement to deploy a separate line system for the functional tests.

## Testbed Type
*   Typical test setup for this test is a DUT1 with 2 ports to 2 ATE ports or 2
    ports to a second DUT2. For most tests this setup should be sufficient.
    Ref: [Typical ATE<>DUT Test bed](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)
*   A and Z ends of  the link should have same 400ZR PMD. For this test a
    single DUT ZR port connected to a single ZR ATE port is also sufficient. 

Once the ZR link is estabished proceed to configure the following entities:

### TRANSCEIVER 11.1 - Test Optical Channel and Tunable Parameters
*   Ensure optical channel related tunable parameters are set through the
    following OC paths such that
      * Both transceivers state is enabled
      * Both transceivers related optical channel tunable parameters are set
        to a valid target TX output power example -10 dBm
      * Both transceivers are tuned to a valid centre frequency
        example 193.1 THz
        * /components/component/transceiver/config/enabled
        * /components/component/optical-channel/config/frequency
        * /components/component/optical-channel/config/target-output-power
        * /components/component/optical-channel/config/operational-mode

### TRANSCEIVER 11.2 - Test Ethernet Logical Channels 
* Ensure terminal-devic ethernet-logical-channels  are set through the
  following OC paths
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
* Typical Settings for an Ethernet Logical Channel are shown below:
    * logical-channel-type: PROT_ETHERNET
    * trib-protocol: PROT_400GE
    * rate-class: TRIB_RATE_400G
    * admin-state: ENABLED
    * description: ETH Logical Channel
    * index: 40000 (unique integer value)
*   Not that each logical-channel created above must be assigned an integer value that
    is unique across the system.

### TRANSCEIVER 11.3 - Test Coherent Logical Channels 
* Ensure terminal-device coherent-logical-channels are set through the
  following OC paths
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
*   Typical setting for a coherent logical channel are shown below:
      * logical-channel-type: PROT_OTN
      * admin-state: ENABLED
      * description: Coherent Logical Channel
      * index: 40004 (unique integer value)

* With above optical and logical channels configured verify DUT is able to
  stream corresponding telemetry leaves under these logical and optical
  channels. List of such telemetry leaves covered under this test is documented
  below under Telemetry Parameter coverage heading.

**Note**: There are other telemetry and config leaves related to optical and
          logical channelsthat are covered under separately published tests
          under platforms/transceiver.

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

## OpenConfig Path and RPC Coverage
```yaml
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```
