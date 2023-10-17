# gNMI-1.20: Telemetry: 400ZR Electrical Signal to Noise Ratio(eSNR) and Chromatic Dispersion(CD) PM streaming

## Summary

Validate 400ZR optics module reports accurate eSNR and CD PMs.

eSNR is defined as the electrical Signal to Noise ratio at the decision
sampling point in dB

Chromatic Dispersion is frequency dependent change in signal phase velocity due
to fiber measured in ps/nm 

## Procedure

*   Connect two ZR interfaces using a duplex LC fiber jumper such that TX
    output power of one is the RX input power of the other module. Connection
    between the modules should pass through an optical switch that can be
    controlled through automation to simulate a fiber cut.  
*   To establish a point to point ZR link ensure the following:
      * Both transceivers state is enabled
      * Both transceivers are set to a valid target TX output power
        example -10 dBm
      * Both transceivers are tuned to a valid centre frequency
        example 193.1 THz
*   With the ZR link is established as explained above, verify that the
    following ZR transceiver telemetry paths exist and are streamed for both
    the ZR optics
    *   /terminal-device/logical-channels/channel/otn/state/esnr/instant
    *   /terminal-device/logical-channels/channel/otn/state/esnr/avg
    *   /terminal-device/logical-channels/channel/otn/state/esnr/min
    *   /terminal-device/logical-channels/channel/otn/state/esnr/max
    *   /platform/components/component/optical-channel/state/chromatic-dispersion/instant
    *   /platform/components/component/optical-channel/state/chromatic-dispersion/avg
    *   /platform/components/component/optical-channel/state/chromatic-dispersion/min
    *   /platform/components/component/optical-channel/state/chromatic-dispersion/max

*   When the modules or the devices are still in a boot stage, they should not
    stream any invalid string values like "nil" or "-inf" until valid values
    are available for streaming.

*   eSNR and CD streamed values should always be of type Decimal64.
    When link interfaces are in down state 0 should be reported as a valid
    value.

**Note:** For min, max, and avg values, 10 second sampling is preferred. If 
          10 seconds is not supported, the sampling interval used must be
          communicated.


*   Verify that the optics eSNR and CD is updated after the interface flaps.

    *   Enable a pair of ZR interfaces on the DUT as explained above.
    *   Verify the ZR optics eSNR and CD PM is in the normal range.
    *   Disable or shut down the interface on the DUT.
    *   Re-enable the interfaces on the DUT.
    *   Verify the ZR optics eSNR and CD PM is updated to the value in the normal
        range again.
        * Typical expected value range for eSNR is 13.5 to
          18 dB +/-0.1 dB.
        * Typical CD expected value range is 0 to 2400 ps/nm.

*   Verify that the optics eSNR and CD is updated after a fiber cut.

    *   Enable a pair of ZR interfaces on the DUT as explained above.
    *   Verify the ZR optics eSNR and CD PM is in the normal range.
    *   Simulate a fiber cut using the optical switch that sits in-between the
        DUT ports.
    *   Re-enable the optical switch connection to clear the fiber cut fault.
    *   Verify the ZR optics eSNR and CD PM is updated to the value in the normal
        range again.
        * Typical expected value range for eSNR is 13.5 to
          18 dB +/-0.1 dB.
        * Typical CD expected value range is 0 to 2400 ps/nm.

## Config Parameter coverage

*   /components/component/oc-transceiver:transceiver/oc-transceiver/config/enabled

## Telemetry Parameter coverage

*   /terminal-device/logical-channels/channel/otn/state/esnr/instant
*   /terminal-device/logical-channels/channel/otn/state/esnr/avg
*   /terminal-device/logical-channels/channel/otn/state/esnr/min
*   /terminal-device/logical-channels/channel/otn/state/esnr/max
*   /platform/components/component/optical-channel/state/chromatic-dispersion/instant
*   /platform/components/component/optical-channel/state/chromatic-dispersion/avg
*   /platform/components/component/optical-channel/state/chromatic-dispersion/min
*   /platform/components/component/optical-channel/state/chromatic-dispersion/max