# TRANSCEIVER-13: Configuration: 400ZR Transceiver Low Power Mode Setting.

## Summary

Validate 400ZR transceiver is able to move to low power consumption mode when
the interface/config/enabled state is set to "False"

**NOTE:** The Module Power Mode dictates the maximum electrical power that the
module is permitted to consume while operating in that Module Power Mode.
The Module Power Mode is a function of the state of the Module State Machine.
Two Module Power Modes are defined:
  * In Low Power Mode (characteristic of all MSM steady states except
    ModuleReady) the maximum module power consumption is defined in the form
    factor-specific hardware specification.
  * In High Power Mode (characteristic of the MSM state ModuleReady) the
    implementation dependent maximum module power consumption is advertised in
    the MaxPower Byte 00h:201. More details in the CMIS link below.

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
*   Verify that the following ZR transceiver OC path when set to False is able
    to move to the low power mode as defined in the CMIS.
    
    *   /interfaces/interface/config/enabled

*   In low power mode the module's Management interface should be available,
    entire paged management memory should be accessible. During this state,
    the host may configure the module using the management interface to read
    from and write to the management Memory Map.

*   The Data Path State of all lanes is still DPDeactivated in the ModuleLowPwr
    state.

*   With module in low power mode verify that the module is still able to
    report inventory information through the following OC paths.

    *   /platform/components/component/state/serial-no
    *   /platform/components/component/state/part-no
    *   /platform/components/component/state/type
    *   /platform/components/component/state/description
    *   /platform/components/component/state/mfg-name
    *   /platform/components/component/state/mfg-date
    *   /platform/components/component/state/hardware-version
    *   /platform/components/component/state/firmware-version

*  With module in low power mode verify that the module laser is squelched
   and it is no longer able to report output-power under the following OC
   paths.
    *   /components/component/optical-channel/state/output-power/instant
    *   /components/component/optical-channel/state/output-power/avg
    *   /components/component/optical-channel/state/output-power/min
    *   /components/component/optical-channel/state/output-power/max

*   Set the interface/config/enabled state to True 

    *   Verify module is able to transition into High Power Mode.
    *   In this state module is still able to report all the inventory
        information as verified above.
    *   In this state verify module is able to report a valid output power
        through the following OC paths as provisioned earlier. 

        *   /components/component/optical-channel/state/output-power/instant
        *   /components/component/optical-channel/state/output-power/avg
        *   /components/component/optical-channel/state/output-power/min
        *   /components/component/optical-channel/state/output-power/max

    *   Verify the ZR optics TX output power telemetry values are updated to
        the value in the normal range again.
        *  Typical min/max value range for TX Output Power -13 to -9 dbm.
        *  values must always be of type decimal64.
        *  When link interfaces are in down state 0 must be reported as a valid
           value.

    *   When the modules or the devices are still in a boot stage, they must not
        stream any invalid string values like "nil" or "-inf" until valid values
        are available for streaming.

## OpenConfig Path and RPC Coverage

```yaml
paths:
    # Configure parameter
    /interfaces/interface/config/enabled:
    # Telemetry Parameter coverage
    /components/component/state/serial-no:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/state/part-no:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/state/type:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/state/description:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/state/mfg-name:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/state/mfg-date:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/state/hardware-version:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/state/firmware-version:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/output-power/instant:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/output-power/avg:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/output-power/min:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/output-power/max:
        platform_type: ["OPTICAL_CHANNEL"]

rpcs:
    gnmi:
        gNMI.Get:
        gNMI.Set:
        gNMI.Subscribe:
```