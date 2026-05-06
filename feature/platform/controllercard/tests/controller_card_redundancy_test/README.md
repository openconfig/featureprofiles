# gNMI-1.17: Controller card redundancy test

## Summary

- Collect inventory data for each controller card.
- Verify that the last restart time is updated.

## Procedure

### gNMI-1.17.1: Controller Card Inventory Test

* Collect the following attributes for each component of `CONTROLLER_CARD` type and verify correctness (mostly non-empty strings):
  *   /components/component/state/empty
  *   /components/component/state/location
  *   /components/component/state/oper-status
  *   /components/component/state/switchover-ready
  *   /components/component/state/redundant-role
  *   /components/component/state/last-switchover-time
  *   /components/component/state/last-switchover-reason/trigger
  *   /components/component/state/last-switchover-reason/details
  *   /components/component/state/last-reboot-time
  *   /components/component/state/last-reboot-reason
  *   /components/component/state/description
  *   /components/component/state/hardware-version
  *   /components/component/state/id
  *   /components/component/state/mfg-name
  *   /components/component/state/name
  *   /components/component/state/parent
  *   /components/component/state/part-no
  *   /components/component/state/serial-no
  *   /components/component/state/type

* Store the list of present components of `CONTROLLER_CARD` type.

### gNMI-1.17.2: Controller Card Switchover Test

* Verify that all controller cards have `switchover-ready=TRUE`.
* Collect and store the `redundant-role` for each controller card as "previous-role".
* Initiate controller card switchover.
* Periodically (60 sec interval) attempt to get `state/redundant-role` and `state/switchover-ready` for both CONTROLLER_CARDS until a successful response is received, but for no longer than 20 min.
  * Collect `redundant-role` for each controller card. Compare it with the "previous-role".
    * For the controller card with the **current** "PRIMARY" role, the **previous** role must be "SECONDARY".
    * For the controller card with the **current** "SECONDARY" role, the **previous** role must be "PRIMARY".
* Periodically check `state/switchover-ready` until (`switchover-ready=TRUE` on all controller cards OR `last-switchover-time` is more than 20 min ago).
  * Wait (5 min).
  * Verify that all controller cards have `switchover-ready=TRUE`; if so, the test PASSED.

### gNMI-1.17.3: Controller Card Redundancy Test

* Verify that all controller cards have `switchover-ready=TRUE`.
* Select the component with `redundant-role=PRIMARY` and store its name as "previous_primary".
* Verify and power down the "previous_primary" component which should have already been switch over in the previous sub-test. Wait 5s.
* Collect `redundant-role` and `oper-status` from all components of `CONTROLLER_CARD` type as collected in test 1.
  * Verify that the "previous_primary" controller `oper-status` is **not** `ACTIVE` and/or its `power-admin-state` is `POWER_DISABLED`.
  * Verify that exactly one controller card has `redundant-role=PRIMARY` and `oper-status=ACTIVE`.
  * Depending on the implementation, the above leaves may not be returned for the "previous_primary" controller card. This satisfies the condition that this controller's `oper-status` is **not** `ACTIVE`, and its `redundant-role` is not `PRIMARY`.
  * If the gNMI client can get this information, it is assumed that controller card redundancy works. More thorough tests of failover are part of forwarding tests.
* Power up the "previous_primary" controller card.
* Wait until all controller cards have `switchover-ready=TRUE` (cleanup).

### gNMI-1.17.4: Controller Card Verify Last Reboot Test

* Select the component with `redundant-role=SECONDARY`.
* Store the `last-reboot-time` for this component as "previous-reboot-time".
* Power down this component, wait 60 sec.
* Power up this component.
* Wait.
* Get the `last-reboot-time` and compare it with the "previous-reboot-time".
  * The "previous-reboot-time" must be smaller (earlier) than the recently collected `last-reboot-time`.

## Canonical OC
```json
{}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  # Telemetry Parameter coverage
    /components/component/controller-card/state/power-admin-state:
      platform_type: ["CONTROLLER_CARD"]
    /components/component/state/empty:
      platform_type: ["CONTROLLER_CARD"]
    /components/component/state/location:
      platform_type: ["CONTROLLER_CARD"]
    /components/component/state/oper-status:
      platform_type: ["CONTROLLER_CARD"]
    /components/component/state/switchover-ready:
      platform_type: ["CONTROLLER_CARD"]
    /components/component/state/redundant-role:
      platform_type: ["CONTROLLER_CARD"]
    /components/component/state/last-switchover-time:
      platform_type: ["CONTROLLER_CARD"]
    /components/component/state/last-switchover-reason/trigger:
      platform_type: ["CONTROLLER_CARD"]
    /components/component/state/last-switchover-reason/details:
      platform_type: ["CONTROLLER_CARD"]
    /components/component/state/last-reboot-time:
      platform_type: ["CONTROLLER_CARD"]
    /components/component/state/last-reboot-reason:
      platform_type: ["CONTROLLER_CARD"]
    /components/component/state/description:
      platform_type: ["CONTROLLER_CARD"]
    /components/component/state/hardware-version:
      platform_type: ["CONTROLLER_CARD"]
    /components/component/state/id:
      platform_type: ["CONTROLLER_CARD"]
    /components/component/state/mfg-name:
      platform_type: ["CONTROLLER_CARD"]
    /components/component/state/name:
      platform_type: ["CONTROLLER_CARD"]
    /components/component/state/parent:
      platform_type: ["CONTROLLER_CARD"]
    /components/component/state/part-no:
      platform_type: ["CONTROLLER_CARD"]
    /components/component/state/serial-no:
      platform_type: ["CONTROLLER_CARD"]
    /components/component/state/type:
      platform_type: ["CONTROLLER_CARD"]

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
  gnoi:
    system.System.SwitchControlProcessor:
    system.System.Reboot:
```

## Minimum DUT platform requirement

*   MFF
