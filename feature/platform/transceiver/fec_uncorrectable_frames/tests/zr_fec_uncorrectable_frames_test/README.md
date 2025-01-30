# TRANSCEIVER-10: Telemetry: 400ZR Optics FEC(Forward Error Correction) Uncorrectable Frames Streaming.

## Summary

Validate 400ZR optics module reports uncorrectable FEC frames count.

This observable represents the number of uncorrectable FEC frames,
measured as RS(544,514) equivalent frames, in a short interval.
This is a post-FEC decoder error metric.

## Procedure

*   Connect two ZR interfaces using a duplex LC fiber jumper such that TX
    output power of one is the RX input power of the other module.
*   To establish a point to point ZR link ensure the following:
      * Both transceivers state is enabled
      * Both transceivers are set to a valid target TX output power
        example -10 dBm
      * Both transceivers are tuned to a valid centre frequency
        example 193.1 THz
*   With the ZR link established as explained above, verify that the
    following ZR transceiver telemetry path exist and is streamed for both
    the ZR optics.
    *   /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks
*   Verify that the reported data should be of type yang:counter64.
*   When the modules or the devices are still in a boot stage, they must not
    stream any invalid string values like "nil" or "-inf".
*   Toggle the interface state using /interfaces/interface/config/enabled and
    verify relevant FEC uncorrectable frame count is streamed. If there are no
    errors a value of 0 should be streamed for no FEC uncorrectable frames. 

## OpenConfig Path and RPC Coverage

```yaml
paths:
    # Config Parameter coverage
    /interfaces/interface/config/enabled:
    /components/component/transceiver/config/enabled:
        platform_type: ["OPTICAL_CHANNEL"]
    # Telemetry Parameter coverage
    /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks:

rpcs:
    gnmi:
        gNMI.Get:
        gNMI.Set:
        gNMI.Subscribe:
```