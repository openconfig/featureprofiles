# TRANSCEIVER-6: Telemetry: 400ZR Optics performance metrics (pm) streaming.

## Summary

Validate 400ZR optics module reports performance metric (PM) data as defined in
module CMIS VDM(Versatile Diagnostics Monitor):
*   eSNR is defined as the electrical Signal to Noise ratio at the decision sampling point in dB
*   Q-value is the decibel (dB) value representing signal BER.
*   pre-FEC BER bit error rate.

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
    the ZR optics.
    *   /terminal-device/logical-channels/channel/otn/state/esnr/instant
    *   /terminal-device/logical-channels/channel/otn/state/esnr/avg
    *   /terminal-device/logical-channels/channel/otn/state/esnr/min
    *   /terminal-device/logical-channels/channel/otn/state/esnr/max
    *   /terminal-device/logical-channels/channel/otn/state/q-value/instant
    *   /terminal-device/logical-channels/channel/otn/state/q-value/avg
    *   /terminal-device/logical-channels/channel/otn/state/q-value/min
    *   /terminal-device/logical-channels/channel/otn/state/q-value/max
    *   /terminal-device/logical-channels/channel/otn/state/pre-fec-ber/instant
    *   /terminal-device/logical-channels/channel/otn/state/pre-fec-ber/avg
    *   /terminal-device/logical-channels/channel/otn/state/pre-fec-ber/min
    *   /terminal-device/logical-channels/channel/otn/state/pre-fec-ber/max


*   For reported data check for validity min <= avg/instant <= max

*   When the modules or the devices are still in a boot stage, they must not
    stream any invalid string values like "nil" or "-inf" until valid values
    are available for streaming.

*   Q-value, eSNR and pre-Fec BER must always be of type decimal64. When link
    interfaces are in down state 0.0 must be reported as a valid default value.
    *   Typical expected value range for eSNR is 13.5 to 18 dB +/-0.1 dB.
    *   Typical expected value for Pre-FEC BER should be less than 1.2E-2.
    *   Typical expected Q-value should be greater than 7 dB.


**Note:** For min, max, and avg values, 10 second sampling is preferred. If 
          10 seconds is not supported, the sampling interval used must be
          specified by adding a deviation to the test.


*   Verify that the optics PM data is updated after the interface flaps.

    *   Enable a pair of ZR interfaces on the DUT as explained above.
    *   Subscribe SAMPLE to the above PM leafs with a sample rate of 10
        seconds.
    *   Verify the ZR optics PMs are in the normal range.
    *   Use /components/component/transceiver/config/enabled to disable the
        transceiver, wait 10 seconds and then re-enable the transceiver.
    *   Verify that the PM leafs report '0' during the reboot and no value
        of nil or -inf is reported.
    *   Re-enable the interfaces on the DUT.
    *   Verify the ZR optics pre FEC PM is updated to the value in the normal
        range again. 

## OpenConfig Path and RPC Coverage

```yaml
paths:
    # Config Parameter coverage
    /interfaces/interface/config/enabled:
    /components/component/transceiver/config/enabled:
        platform_type: ["OPTICAL_CHANNEL"]
    # Telemetry Parameter coverage
    /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks:
    /terminal-device/logical-channels/channel/otn/state/esnr/instant:
    /terminal-device/logical-channels/channel/otn/state/esnr/avg:
    /terminal-device/logical-channels/channel/otn/state/esnr/min:
    /terminal-device/logical-channels/channel/otn/state/esnr/max:
    /terminal-device/logical-channels/channel/otn/state/q-value/instant:
    /terminal-device/logical-channels/channel/otn/state/q-value/avg:
    /terminal-device/logical-channels/channel/otn/state/q-value/min:
    /terminal-device/logical-channels/channel/otn/state/q-value/max:
    /terminal-device/logical-channels/channel/otn/state/pre-fec-ber/instant:
    /terminal-device/logical-channels/channel/otn/state/pre-fec-ber/avg:
    /terminal-device/logical-channels/channel/otn/state/pre-fec-ber/min:
    /terminal-device/logical-channels/channel/otn/state/pre-fec-ber/max:

rpcs:
    gnmi:
        gNMI.Get:
        gNMI.Set:
        gNMI.Subscribe:
```