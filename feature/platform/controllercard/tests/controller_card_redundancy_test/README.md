# gNMI-1.17: Controller card redundanacy test

## Summary
- collect inventory data for each controller card
- Verify last restart time is updated

## Procedure

### test 1 Contyroller Card inventory

* collect following attributes for each component of CONTROLLER_CARD type and verify corectness (mostly non-empty string)
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

* store list of present components of CONTROLLER_CARD type

### test 2 switchover
* Verify that all controller_cards have `switchover-ready=TRUE`
* Collect and store `redundant-role` for each controller_card as "previous-role"
* Initiate controller-card switchover
* Try periodicaly (60 sec interval)  get `state/redundant-role` and `state/switchover-ready` of both CONTROLLER_CARDS  untill sucesfully recived responce, but no longer then 20 min.
  * Collect `redundant-role` for each controller_card. Compare it with "previous-role"
    * for controller_card of **current** "PRIMARY" role, **previous** role must be "SECONDARY"
    * for controller_card of **current** "SECONDARY" role, **previous** role must be "PRIMARY"
* Keep periodicly get `state/switchover-ready` until (`switchover-ready=TRUE` on all controller_cards OR `last-switchover-time` is moret then 20min ago)
  * Wait(5min)
  * Verify that all controller_cards has `switchover-ready=TRUE`; if so test PASSED

### test 3 Redundancy
* Verify that all controller_cards has `switchover-ready=TRUE`
* Select component with `redundant-role=PRIMARY`, store name as "previous_primary"
* Perfom Controller_Card switchover and then power down "previous_primary" component. Wait 5s.
* Collect `redundant-role` and `oper-status` from all components of CONTROLLER_CARD type as collected in test 1;
  * verify that "previous_primary" controller `oper-status` is **not** `ACTIVE` and/or its
`power-admin-state` is `POWER_DISABLED`; 
  * verify that at exectly one controller_card has `redundant-role=PRIMARY` and `oper-status=ACTIVE`
  * Depending on implementation, above leaves may be not returned for "previous_primary" controller_card.
    This satisfy condition of this controller's `oper-status` is **not** `ACTIVE`, and it's `redundant-role`
    is not `PRIMARY`
  * if gNMI client can get this information, it is asumed controller card redundancy works. 
    More torough tests of failover are part of forwarding tests.
* Power up "previous_primary" controller card
* Wait untill all controller_cards has `switchover-ready=TRUE` (cleanup)
 
### test 4 last reboot time
* Select component with `redundant-role=SECONDARY`
* store last-reboot-time for this component as "previous-reboot-time"
* Power down this component, wait 60 sec.
* Power up this component
* Wait
* get last-reboot-time and compare with "previous-reboot-time"
  * "previous-reboot-time" must be smaller (earlier) then recently collected last-reboot-time

## Config Parameter coverage

*   /components/component/controller_card/config/power-admin-state

## Telemetry Parameter coverage

*   /components/component/controller-card/state/power-admin-state
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

## OpenConfig Path and RPC Coverage

```yaml
paths:
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
