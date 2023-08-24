# gNMI-1.15 Controller Card redundnacy test

## Summary
- collect inventory data for each controller card
- Verify last restart time is updated

## Procedure

### test 1 Contyroller Card inventory

> NOTE: Theis test is practically identical to Fabric part of
> https://github.com/openconfig/featureprofiles/tree/main/feature/platform/tests/telemetry_inventory_test;
> Hence code could be reused and then removed form telemetry_inventory_test.

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
* Verify that all controller_cards has `switchover-ready=TRUE`
* Collect and store `redundant-role` for each controller_card as "previous-role"
* Initiate controller-card switchover
* Wait 15 seconds
* Collect `redundant-role` for each controller_card. Compare it wit "previous-role"
  * for controller_card of **current** "PRIMARY" role, **previous** role must be "SECONDARY"
  * for at least one controller_card of **previous** role "SECONDARY" role, **current** role must be "PRIMARY"
  * shall more then 2 controller_cards exist, their previour and current states could be equal and be "SECONDARY"
* Until (`switchover-ready=TRUE` on all controller_cards OR `last-switchover-time` is moret then 20min ago)
  * Wait(5min)
  * Verify that all controller_cards has `switchover-ready=TRUE`; if so test PASSED

### test 2 Redundancy
* Verify that all controller_cards has `switchover-ready=TRUE`
* Select component with `redundant-role=PRIMARY`, store name as "previous_primary"
* Power down this component. Wait 5s.
* Collect `redundant-role` and `oper-status` from all components of CONTROLLER_CARD type as collected in test 1;
  * verify that "previous_primary" controller `oper-status` is **not** `ACTIVE`
  * verify that at exectly one controller_card has `redundant-role=PRIMARY` and `oper-status=ACTIVE`
  * if gNMI client can retrive this information, it is asumed controller card redundancy works. 
    More torough tests of failover are part of forwarding tests.
* Power up "previous_primary" controller card
* Wait untill all controller_cards has `switchover-ready=TRUE` (cleanup)
 
### test 3 last reboot time
* Select component with `redundant-role=SECONDARY`
* store last-reboot-time for this component as "No_want"
* Power down this component, wait 60 sec.
* Power up this component
* Wait
* get last-reboot-timea and compare with "want"
  * "No_want" must be smaller (earlier) then recently collectrd last-reboot-time

### test 4 switchover
* Verify that all controller_cards has `switchover-ready=TRUE`
* Collect and store `redundant-role` for each controller_card as "previous-role"
* Initiate controller-card switchover
* Wait 15 seconds
* Collect `redundant-role` for each controller_card. Compare it wit "previous-role"
  * for controller_card of **current** "PRIMARY" role, **previous** role must be "SECONDARY"
  * for at least one controller_card of **previous** role "SECONDARY" role, **current** role must be "PRIMARY"
  * shall more then 2 controller_cards exist, their previour and current states could be equal and be "SECONDARY"
* Until (`switchover-ready=TRUE` on all controller_cards OR `last-switchover-time` is moret then 20min ago)
  * Wait(5min)
  * Verify that all controller_cards has `switchover-ready=TRUE`; if so test PASSED

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

## Minimum DUT platform requirement
*   MFF
