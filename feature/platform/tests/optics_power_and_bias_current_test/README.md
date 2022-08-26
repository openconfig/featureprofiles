# gNMI-1.13: Telemetry: Optics Power and Bias Current

## Summary

Validate optics input power, output power and bias current.

## Procedure

*   Connect at least one optical ethernet interface to ATE.
*   Verify that the following optics telemetry paths exist for the installed optics.
    *   /components/component/transceiver/physical-channels/channel/state/input-power/instant
    *   /components/component/transceiver/physical-channels/channel/state/output-power/instant
    *   /components/component/transceiver/physical-channels/channel/state/laser-bias-current/instant
*   Verify the optics power is updated after the interface is flapped.
    *   Enable an interface on the DUT.
    *   Verify the optics input and output power are in the normal range.
    *   Disable or shut down the interface on the DUT.
    *   Verify the optics output power is updated to very low value.
    *   Re-enable the interface on the DUT
    *   Verify the optics output power is updated to the value in the normal range again
    
## Config Parameter coverage

*   /interfaces/interface/config/enabled

## Telemetry Parameter coverage

*   /components/component/transceiver/physical-channels/channel/state/input-power/instant
*   /components/component/transceiver/physical-channels/channel/state/output-power/instant
*   /components/component/transceiver/physical-channels/channel/state/laser-bias-current/instant
