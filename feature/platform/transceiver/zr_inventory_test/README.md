# TRANSCEIVER-7: Telemetry: 400ZR module inventory information. 

## Summary

Validate 400ZR modules report inventory information part number and serial
number.

## Procedure

*   Plug in the ZR module in the host port and make sure the transceiver 
    state is enabled and host is able to detect the module.
*   With the module recognized verify it reports correct inventory
    information by subscribing ON_CHANGE to the following telemetry paths.

    *   /platform/components/component/state/serial-no
    *   /platform/components/component/state/part-no

*   Validate the streamed inventory information data is of type String.

*   Verify that the modules inventory information is reported correctly after
    an optic software reset.

    *   With ZR module plugged in the host and properly recognized 
    *   Verify the ZR optics inventory is correctly reported via the 
        streaming telemetry paths above.
    *   Reset the optic by enabling and disabling the transceiver state
        through /components/component/transceiver/config/enabled.
    *   Wait atleast 20 seconds in between toggling transceiver state.
    *   Verify the ZR optics still reports correct inventory information.
    *   Telemetry subscription should be ON_CHANGE and streamed data should
        be of type String.

*   Verify that the modules inventory information is reported correctly when
    interface state is disabled.

    *   With ZR module plugged in the host and properly recognized
    *   Use /interfaces/interface/config/enabled to disable the module
        interface state, wait 20 seconds. 
    *   Verify the ZR optics inventory information is correctly reported via
        the streaming telemetry paths above in this state.
    *   Telemetry subscription should be ON_CHANGE and streamed data should
        be of type String.

*   Verify the module behaviour when transceiver state is set to disabled.

    *   With ZR module plugged in the host and properly recognized.
    *   Use /components/component/transceiver/config/enabled to disable the
        module transceiver state, wait 20 seconds. 
    *   Verify the ZR module is powered off and no inventory information
        reported via the streaming telemetry paths above in this state.
    *   Telemetry subscription should be ON_CHANGE and there should be no
        streamed inventory data in this state.

*   Verify the module inventory information updates when transceiver under test
    is swapped with a different one.
    *   Make sure ZR module plugged in the host and properly recognized.
    *   Verify module is reporting valid inventory information.
    *   Swap the module with a different one and validate that the new
        inventory information is correctly streamed now.  
    *   Telemetry subscription should be ON_CHANGE and streamed data should
        be of type String.

*   Verify the module inventory information updates when transceiver is
    physically removed from the router port.
    *   Make sure ZR module plugged in the host and properly recognized.
    *   Verify module is reporting valid inventory information.
    *   Physically remove the module from the router port and verify the
        relevant component inventory leaves for the transceiver port under test
        are no longer streamed.
    *   When a component is physically removed explicit deletes for the
        relevant entity leaves should be streamed to clear any stale data.
    *   Telemetry subscription should be ON_CHANGE and streamed data should
        be of type String.

## Config Parameter coverage

*   /components/component/transceiver/config/enabled
*   /interfaces/interface/config/enabled

## Telemetry Parameter coverage

*   /platform/components/component/state/serial-no
*   /platform/components/component/state/part-no
