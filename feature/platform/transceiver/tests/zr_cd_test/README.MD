# TRANSCEIVER-1: Telemetry: 400ZR Chromatic Dispersion(CD) telemetry values streaming

## Summary

Validate 400ZR optics module reports accurate CD telemetry values.

Chromatic Dispersion is frequency dependent change in signal phase velocity due
to fiber measured in ps/nm 

## Procedure

*   Connect two ZR interfaces using a duplex LC fiber jumper such that TX
    output power of one is the RX input power of the other module. Connection
    between the modules should pass through an optical switch that can be
    controlled through automation to simulate a fiber cut.  
*   To establish a point to point ZR link ensure the following:
      * Both transceivers states are enabled
      * Both transceivers are set to a valid target TX output power
        example -10 dBm
      * Both transceivers are tuned to a valid centre frequency
        example 193.1 THz
*   With the ZR link is established as explained above, verify that the
    following ZR transceiver telemetry paths exist and are streamed for both
    the ZR optics
    *   /platform/components/component/optical-channel/state/chromatic-dispersion/instant
    *   /platform/components/component/optical-channel/state/chromatic-dispersion/avg
    *   /platform/components/component/optical-channel/state/chromatic-dispersion/min
    *   /platform/components/component/optical-channel/state/chromatic-dispersion/max

*   When the modules or the devices are still in a boot stage, they must not
    stream any invalid string values like "nil" or "-inf" until valid values
    are available for streaming.

*   CD streamed values must always be of type Decimal64.
    When link interfaces are in down state 0 must be reported as a valid
    value.

**Note:** For min, max, and avg values, 10 second sampling is preferred. If 
          10 seconds is not supported, the sampling interval used must be
          communicated.


*   Verify that the optics CD is updated after the interface flaps.

    *   Enable a pair of ZR interfaces on the DUT as explained above.
    *   Verify the ZR optics CD telemetry values are in the normal range.
    *   Disable or shut down the interface on the DUT.
    *   Verify with interfaces in down state both optics are streaming Decimal64 0
        value for CD.
    *   Re-enable the interfaces on the DUT.
    *   Verify the ZR optics CD telemetry values are updated to the
        value in the normal range again.
        * Typical CD expected value range is 0 to 2400 ps/nm.

*   Verify that the optics CD is updated after a fiber cut.

    *   Enable a pair of ZR interfaces on the DUT as explained above.
    *   Verify the ZR optics CD telemetry values are in the normal
        range.
    *   Simulate a fiber cut using the optical switch that sits in-between the
        DUT ports.
    *   Verify with link in down state due to fiber cut both optics are streaming
        Decimal64 0 value for CD.
    *   Re-enable the optical switch connection to clear the fiber cut fault.
    *   Verify the ZR optics CD telemetry values are updated to the value in the normal
        range again.
        * Typical CD expected value range is 0 to 2400 ps/nm.

## Config Parameter coverage

*   /components/component/transceiver/config/enabled

## Telemetry Parameter coverage

*   /platform/components/component/optical-channel/state/chromatic-dispersion/instant
*   /platform/components/component/optical-channel/state/chromatic-dispersion/avg
*   /platform/components/component/optical-channel/state/chromatic-dispersion/min
*   /platform/components/component/optical-channel/state/chromatic-dispersion/max