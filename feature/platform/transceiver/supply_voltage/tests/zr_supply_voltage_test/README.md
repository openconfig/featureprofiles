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

## Testbed Type
*   Typical test setup for this test is a DUT1 with 2 ports to 2 ATE ports or 2
    ports to a second DUT2. For most tests this setup should be sufficient.
    Ref: [Typical ATE<>DUT Test bed](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)
*   A and Z ends of  the link should have same 400ZR PMD. For this test a
    single DUT ZR port connected to a single ZR ATE port is also sufficient.

Once the ZR link is estabished proceed with the following:
*  verify that the following ZR transceiver telemetry paths exist and are
   streamed for both the ZR optics.
    *   /components/component/transceiver/state/supply-voltage/instant

*   If the modules or the devices are in a boot stage, they must not stream
    any invalid string values like "nil" or "-inf".
*   Reported supply voltage value must always be of type decimal64.
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

## OpenConfig Path and RPC Coverage

```yaml
paths:
    # Config Parameter coverage
    /interfaces/interface/config/enabled:
    # Telemetry Parameter coverage
    /components/component/transceiver/state/supply-voltage/instant:
        platform_type: ["OPTICAL_CHANNEL"]

rpcs:
    gnmi:
        gNMI.Get:
        gNMI.Set:
        gNMI.Subscribe:
```