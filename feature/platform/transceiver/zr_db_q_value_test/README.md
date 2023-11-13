# TRANSCEIVER-6: Telemetry: 400ZR Optics Q-value streaming.

## Summary

Validate 400ZR optics module reports Q-value performance data as defined in
module CMIS VDM(Versatile Diagnostics Monitor).
Q-value is the decibel (dB) value representing signal BER.

## Procedure

*   Connect two ZR interfaces using a duplex LC fiber jumper such that TX
    output power of one is the RX input power of the other module.

*   To establish a point to point ZR link ensure the following:
      * Both transceivers state is enabled
      * Both transceivers are set to a valid target TX output power
        example -10 dBm.
      * Both transceivers are tuned to a valid centre frequency
        example 193.1 THz.

*   With the link ZR link established as explained above, verify that the
    following ZR transceiver telemetry paths exist and are streamed for both
    the ZR optics
    *   /terminal-device/logical-channels/channel/otn/state/q-value/instant
    *   /terminal-device/logical-channels/channel/otn/state/q-value/avg
    *   /terminal-device/logical-channels/channel/otn/state/q-value/min
    *   /terminal-device/logical-channels/channel/otn/state/q-value/max

*   For reported data check for validity min <= avg/instant <= max

*   When the modules or the devices are still in a boot stage, they must not
    stream any invalid string values like "nil" or "-inf" until valid values
    are available for streaming.

*   Q-value must always be of type decimal64. When link interfaces are in down
    state 0.0 must be reported as a valid default value.


**Note:** For min, max, and avg values, 10 second sampling is preferred. If 
          10 seconds is not supported, the sampling interval used must be
          communicated.


*   Verify that the optics Q-value is updated after the interface flaps.

    *   Enable a pair of ZR interfaces on the DUT as explained above.
    *   Verify the ZR optics Q-value PMs are in the normal range.
    *   Disable or shut down the interface on the DUT.
    *   Re-enable the interfaces on the DUT.
    *   Verify the ZR optics pre FEC PM is updated to the value in the normal
        range again. Typical expected value should be greater than 7 dB.

## Config Parameter coverage

*   /components/component/oc-transceiver:transceiver/oc-transceiver/config/enabled

## Telemetry Parameter coverage

*   /terminal-device/logical-channels/channel/otn/state/q-value/instant
*   /terminal-device/logical-channels/channel/otn/state/q-value/avg
*   /terminal-device/logical-channels/channel/otn/state/q-value/min
*   /terminal-device/logical-channels/channel/otn/state/q-value/max
