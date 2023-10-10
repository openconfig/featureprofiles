# gNMI-1.20: Telemetry: 400ZR Optics Chromatic Dispersion(CD) streaming

## Summary

Validate 400ZR optics module reports Chromatic Dispersion.
Chromatic Dispersion is frequency dependent change in signal phase velocity due
to fiber measured in ps/nm 

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
    *   /platform/components/component/optical-channel/state/chromatic-dispersion/instant
    *   /platform/components/component/optical-channel/state/chromatic-dispersion/avg
    *   /platform/components/component/optical-channel/state/chromatic-dispersion/min
    *   /platform/components/component/optical-channel/state/chromatic-dispersion/max


**Note:** For min, max, and avg values, 10 second sampling is preferred. If 
          10 seconds is not supported, the sampling interval used must be
          communicated.


*   Verify that the optics chromatic dispersion is updated after the interface flaps.

    *   Enable a pair of ZR interfaces on the DUT as explained above.
    *   Verify the ZR optics chromatic dispersion PM is in the normal range.
    *   Disable or shut down the interface on the DUT.
    *   Re-enable the interfaces on the DUT.
    *   Verify the ZR optics chromatic dispersion PM is updated to the value in the normal
        range again. Typical expected value range is 0 to 2400 ps/nm.

## Config Parameter coverage

*   /components/component/oc-transceiver:transceiver/oc-transceiver/config/enabled

## Telemetry Parameter coverage

*   /platform/components/component/optical-channel/state/chromatic-dispersion/instant
*   /platform/components/component/optical-channel/state/chromatic-dispersion/avg
*   /platform/components/component/optical-channel/state/chromatic-dispersion/min
*   /platform/components/component/optical-channel/state/chromatic-dispersion/max