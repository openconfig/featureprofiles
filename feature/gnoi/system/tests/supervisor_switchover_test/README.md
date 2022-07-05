# gNOI-3.3: Supervisor Switchover

## Summary

Validate that the active supervisor can be switched.

## Procedure

*   Issue gnoi.SwitchControlProcessor to the chassis with dual supervisor,
    specifying the path to choose the standby RE/SUP.
*   Ensure the SwitchControlProcessorResponse has the new active supervisor as
    the one specified in the request.
*   Validate the standby RE/SUP becomes the active after switchover
*   Validate that all connected ports are re-enabled.

## Config Parameter Coverage

N/A

## Telemetry Parameter Coverage

*   /system/state/current-datetime
*   /components/component[name=<supervisor>]/state/last-switchover-time
*   /components/component[name=<supervisor>]/state/last-switchover-reason/trigger
*   /components/component[name=<supervisor>]/state/last-switchover-reason/details

## Protocol/RPC Parameter Coverage

*   gNOI
    *   System
        *   SwitchControlProcessor
