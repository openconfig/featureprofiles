# TRANSCEIVER-12: Telemetry: 400ZR Transceiver Supply Voltage streaming.

## Summary

Validate 400ZR transceivers report module level internally measured input supply
voltage in 100 µV increments as defined in the CMIS.

Link to CMIS:
https://www.oiforum.com/wp-content/uploads/CMIS5p0_Third_Party_Spec.pdf

## Procedure

*   Connect two ZR optics using a duplex LC fiber jumper such that TX
    output power of one is the RX input power of the other module.
*   To establish a point to point ZR link ensure the following:
      * Both transceivers state is enabled.
      * Both transceivers are set to a valid target TX output power
        example -10 dBm.
      * Both transceivers are tuned to a valid centre frequency
        example 193.1 THz.
*   With the ZR link established as explained above, verify that the
    following ZR transceiver telemetry paths exist and are streamed for both
    the ZR optics.
    *   /components/component/transceiver/state/supply-voltage/instant
    *   /components/component/transceiver/state/supply-voltage/min
    *   /components/component/transceiver/state/supply-voltage/max
    *   /components/component/transceiver/state/supply-voltage/avg
*   For reported data check for validity min <= avg/instant <= max

*   If the modules or the devices are in a boot stage, they must not stream
    any invalid string values like "nil" or "-inf".
*   Reported supply voltage value must always be of type decimal64.


**Note:** For min, max, and avg values, 10 second sampling is preferred. If the
          min, max average values or the 10 seconds sampling is not supported,
          the sampling interval used must be specified and this must be
          captured by adding a deviation to the test.


*   Verify the module supply voltage is reported correctly with optics
    interface in disabled state.

    *   Use /interfaces/interface/config/enabled to disable the interfaces and
        wait 120 seconds before taking the supply voltage reading again.
    *   Verify the module is able to stream the supply voltage data in this
        state.
    *   For reported data check for validity min <= avg/instant <= max
    *   If the modules or the devices are in a boot stage, they must not stream
        any invalid string values like "nil" or "-inf".
    *   Reported supply voltage value must always be of type decimal64. 

## Config Parameter coverage

*   /interfaces/interface/config/enabled

## Telemetry Parameter coverage

*   /platform/components/component/state/supply-voltage/instant
*   /platform/components/component/state/supply-voltage/min
*   /platform/components/component/state/supply-voltage/max
*   /platform/components/component/state/supply-voltage/avg