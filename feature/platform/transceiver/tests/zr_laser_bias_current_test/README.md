# TRANSCEIVER-9: Telemetry: 400ZR TX laser bias current telemetry values streaming. 

## Summary

Validate 400ZR optics modules report accurate laser bias current telemetry
values.

As per [CMIS](https://www.oiforum.com/wp-content/uploads/CMIS3p0_Third_Party_Spec.pdf):

Measured Tx laser bias current is represented as a 16-bit unsigned integer with
the current defined as the 12 full 16-bit value (0 to 65535) with LSB equal to
2 uA times the multiplier from Byte 01h:160. For a multiplier of 13 1,
this yields a total measurement range of 0 to 131 mA.

Accuracy must be better than +/-10% of the manufacturer's nominal value over
specified operating temperature and voltage.


## TRANSCEIVER-9.1

*   Connect two ZR interfaces using a duplex LC fiber jumper such that TX
    output power of one is the RX input power of the other module. Connection
    between the modules should pass through an optical switch that can be
    controlled through automation to simulate a fiber cut as needed.
*   To establish a point to point ZR link ensure the following:
      * Both transceivers states are enabled
      * Both transceivers are set to a valid target TX output power
        example -9 dBm
      * Both transceivers are tuned to a valid centre frequency
        example 193.1 THz
*   With the ZR link is established as explained above, verify that the
    following ZR transceiver telemetry paths exist and are streamed for both
    the ZR optics

    *   /components/component/optical-channel/state/laser-bias-current/instant
    *   /components/component/optical-channel/state/laser-bias-current/avg
    *   /components/component/optical-channel/state/laser-bias-current/min
    *   /components/component/optical-channel/state/laser-bias-current/max


## TRANSCEIVER-9.2

*   When the modules or the devices are still in a boot stage, they must not
    stream any invalid string values like "nil" or "-inf".

*   Laser bias current values must always be of type decimal64.
    When laser is in off state 0 must be reported as a valid value.

**Note:** For min, max, and avg values, 10 second sampling is preferred. If the
          min, max average values or the 10 seconds sampling is not supported,
          the sampling interval used must be specified and this must be
          captured by adding a deviation to the test.

## TRANSCEIVER-9.3

*   Verify that the TX laser bias current is updated after an interface
    enable / disable state change.

    *   Enable a pair of ZR interfaces on the DUT as explained above.
    *   Verify the ZR optics TX laser bias current telemetry values are
        in the normal range.
    *   Use /interfaces/interface/config/enabled to disable the interfaces so
        that the TX laser is squelched / turned off.
    *   Verify with interface state disabled and link down, decimal64 0 value
        is streamed for both optics TX laser bias current.
    *   Re-enable the optics using /interfaces/interface/config/enabled.
    *   Verify the ZR optics TX laser bias current telemetry values are
        updated to the value in the normal range again.
        * Typical measurement range 0 to 131 mA.

## TRANSCEIVER-9.4

*   Verify that the TX laser bias current is updated after transceiver power
    ON / OFF state change.

    *   Enable a pair of ZR interfaces on the DUT as explained above.
    *   Verify the ZR optics TX laser bias current telemetry values are
        in the normal range.
    *   Use /components/component/transceiver/config/enabled to power off the
        transceiver so that the TX laser is squelched / turned off.
    *   Verify with transceiver state disabled and link down, no value
        is streamed for both optics TX laser bias current.
    *   Re-enable the optics using
        /components/component/transceiver/config/enabled.
    *   Verify the ZR optics TX laser bias current telemetry values are
        updated to the value in the normal range again.
        * Typical measurement range 0 to 131 mA.

## Config Parameter coverage

*   /components/component/transceiver/config/enabled
*   /interfaces/interface/config/enabled

## Telemetry Parameter coverage

*   /components/component/optical-channel/state/laser-bias-current/instant
*   /components/component/optical-channel/state/laser-bias-current/avg
*   /components/component/optical-channel/state/laser-bias-current/min
*   /components/component/optical-channel/state/laser-bias-current/max

## OpenConfig Path and RPC Coverage
```yaml
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```
