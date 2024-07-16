# TRANSCEIVER-4: Telemetry: 400ZR RX input and TX output power telemetry values streaming. 

## Summary

Validate 400ZR optics modules report accurate RX input and TX output power
telemetry values.

As per CMIS ZR modules report two types of RX input power and one TX output
power.
* RX Signal Power
  * Reports the actual signal power after filtering out any extra noise.
  * Is mapped to /component/optical-channel/ full path shown below
* RX Total Power
  * Reports RX Signal Power plus noise without any filtering.
  * Is mapped to /component/transceiver/physical-channel full path shown below
* TX Output Power
  * This is the total TX output power
  * Is mapped to component/optical-channel/ full path shown below


## TRANSCEIVER-4.1

*   Connect two ZR interfaces using a duplex LC fiber jumper such that TX
    output power of one is the RX input power of the other module. Connection
    between the modules should pass through an optical switch that can be
    controlled through automation to simulate a fiber cut.  
*   To establish a point to point ZR link ensure the following:
      * Both transceivers states are enabled
      * Both transceivers are set to a valid target TX output power
        example -9 dBm
      * Both transceivers are tuned to a valid centre frequency
        example 193.1 THz
*   With the ZR link is established as explained above, verify that the
    following ZR transceiver telemetry paths exist and are streamed for both
    the ZR optics
    *   /components/component/optical-channel/state/input-power/instant
    *   /components/component/optical-channel/state/input-power/avg
    *   /components/component/optical-channel/state/input-power/min
    *   /components/component/optical-channel/state/input-power/max
    *   /components/component/optical-channel/state/output-power/instant
    *   /components/component/optical-channel/state/output-power/avg
    *   /components/component/optical-channel/state/output-power/min
    *   /components/component/optical-channel/state/output-power/max
    *   /components/component/transceiver/physical-channel/channel/state/input-power/instant
    *   /components/component/transceiver/physical-channel/channel/state/input-power/min
    *   /components/component/transceiver/physical-channel/channel/state/input-power/max
    *   /components/component/transceiver/physical-channel/channel/state/input-power/avg

## TRANSCEIVER-4.2

*   When the modules or the devices are still in a boot stage, they must not
    stream any invalid string values like "nil" or "-inf" until valid values
    are available for streaming.

*   RX Input and TX output power values must always be of type decimal64.
    When link interfaces are in down state RX Input power of -40 dbm must be
    reported as a valid value.

**Note:** For min, max, and avg values, 10 second sampling is preferred. If 
          10 seconds is not supported, the sampling interval used must be
          communicated.

## TRANSCEIVER-4.3

*   Verify that the optics RX input and TX output power is updated after the
    interface flaps.

    *   Enable a pair of ZR interfaces on the DUT as explained above.
    *   Verify the ZR optics RX input and TX output power telemetry values are
        in the normal range.
    *   Verify that RX Signal Power is less than the RX Total Power.
    *   Disable or shut down the interface on the DUT.
    *   Verify with interfaces in down state both optics are streaming decimal64 0
        value for both RX input and TX output power.
    *   Re-enable the interfaces on the DUT.
    *   Verify the ZR optics RX input and TX output power telemetry values are
        updated to the value in the normal range again.
        * Typical min/max value range for RX Signal Power -14 to 0 dbm.
        * Typical min/max value range for TX Output Power -10 to -6 dbm.

## TRANSCEIVER-4.4

*   Verify that the optics RX input and TX output power is updated after a
    fiber cut.

    *   Enable a pair of ZR interfaces on the DUT as explained above.
    *   Verify the ZR optics RX input and TX output power telemetry values are
        in the normal range.
    *   Verify that RX Signal Power is less than the RX Total Power.
    *   Simulate a fiber cut using the optical switch that sits in-between the
        DUT ports.
    *   Verify with link in down state due to fiber cut both optics are streaming
        decimal64 0 value for both RX input and TX output power.
    *   Re-enable the optical switch connection to clear the fiber cut fault.
    *   Verify the ZR optics RX input and TX output power telemetry values are
        updated to the value in the normal range again.
        * Typical min/max value range for RX Signal Power -14 to 0 dbm.
        * Typical min/max value range for TX Output Power -10 to -6 dbm.

## OpenConfig Path and RPC Coverage

```yaml
paths:
    # Config Parameter coverage
    /interfaces/interface/config/enabled:
    # Telemetry Parameter coverage
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
    /components/component/transceiver/physical-channels/channel/state/input-power/instant:
        platform_type: [ "TRANSCEIVER" ]
    /components/component/transceiver/physical-channels/channel/state/input-power/min:
        platform_type: [ "TRANSCEIVER" ]
    /components/component/transceiver/physical-channels/channel/state/input-power/max:
        platform_type: [ "TRANSCEIVER" ]
    /components/component/transceiver/physical-channels/channel/state/input-power/avg:
        platform_type: [ "TRANSCEIVER" ]

rpcs:
    gnmi:
        gNMI.Get:
        gNMI.Set:
        gNMI.Subscribe:
```