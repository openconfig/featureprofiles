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

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml
paths:
  ## State Paths ##
  /system/state/current-datetime:
  /components/component/state/last-switchover-time:
    platform_type: [ "CONTROLLER_CARD" ]
  /components/component/state/last-switchover-reason/trigger:
    platform_type: [ "CONTROLLER_CARD" ]
  /components/component/state/last-switchover-reason/details:
    platform_type: [ "CONTROLLER_CARD" ]

rpcs:
  gnmi:
    gNMI.Subscribe:
  gnoi:
    system.System.SwitchControlProcessor:
```
