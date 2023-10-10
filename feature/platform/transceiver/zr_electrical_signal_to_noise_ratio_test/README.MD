# gNMI-1.20: Telemetry: 400ZR Optics Electrical Signal to Noise Ratio(eSNR) streaming

## Summary

Validate 400ZR optics module reports accurate eSNR.
eSNR is defined as the electrical Signal to Noise ratio at the decision
sampling point in dB

## Procedure

*   Connect two ZR interfaces using a duplex LC fiber jumper such that TX
    output power of one is the RX input power of the other module.
*   To establish a point to point ZR link ensure the following:
      * Both transceivers state is enabled
      * Both transceivers are set to a valid target TX output power
        example -10 dBm
      * Both transceivers are tuned to a valid centre frequency
        example 193.1 THz
*   With the link ZR link established as explained above, verify that the
    following ZR transceiver telemetry paths exist and are streamed for both
    the ZR optics
    *   /terminal-device/logical-channels/channel/otn/state/esnr/instant
    *   /terminal-device/logical-channels/channel/otn/state/esnr/avg
    *   /terminal-device/logical-channels/channel/otn/state/esnr/min
    *   /terminal-device/logical-channels/channel/otn/state/esnr/max



**Note:** For min, max, and avg values, 10 second sampling is preferred. If 
          10 seconds is not supported, the sampling interval used must be
          communicated.


*   Verify that the optics eSNR is updated after the interface flaps.

    *   Enable a pair of ZR interfaces on the DUT as explained above.
    *   Verify the ZR optics eSNR PM is in the normal range.
    *   Disable or shut down the interface on the DUT.
    *   Re-enable the interfaces on the DUT.
    *   Verify the ZR optics eSNR PM is updated to the value in the normal
        range again. Typical expected value range is 13.5 to 18 dB +/-0.1 dB.

## Config Parameter coverage

*   /components/component/oc-transceiver:transceiver/oc-transceiver/config/enabled

## Telemetry Parameter coverage

*   /terminal-device/logical-channels/channel/otn/state/esnr/instant
*   /terminal-device/logical-channels/channel/otn/state/esnr/avg
*   /terminal-device/logical-channels/channel/otn/state/esnr/min
*   /terminal-device/logical-channels/channel/otn/state/esnr/max