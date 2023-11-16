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
    *   Reset the optic through software.
    *   Verify the ZR optics still reports correct inventory information.
    *   Telemetry subscription should be ON_CHANGE and streamed data should
        be of type String.

*   Verify that the modules inventory information is reported correctly when
    interface and transceiver states are disabled.

    *   With ZR module plugged in the host and properly recognized
    *   Use /components/component/transceiver/config/enabled and 
        /components/component/transceiver/config/enabled to disable the
        transceiver and inetrface state, wait 20 seconds. 
    *   Verify the ZR optics inventory information is correctly reported via
        the streaming telemetry path above in this state.
    *   Telemetry subscription should be ON_CHANGE and streamed data should
        be of type String.

## Config Parameter coverage

*   /components/component/transceiver/config/enabled
*   /interfaces/interface/config/enabled

## Telemetry Parameter coverage

*   /platform/components/component/state/serial-no
*   /platform/components/component/state/part-no
