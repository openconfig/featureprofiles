# gNMI-1.10 Fabric dedundnacy test

## Summary
- collect inventory data for each fabric card
- Verify last restart time is updated
- verify trassic could be forwarded with only one Fabric Card active.

## Procedure
### topology and basic setup
*  Connect OTG port1 to DUT port1 and OTG port2 to DUT port2
*  Configure IPv6 addresses on both links
### test 1 Fabric inventory

> NOTE: Theis test is practically identical to Fabric part of
> https://github.com/openconfig/featureprofiles/tree/main/feature/platform/tests/telemetry_inventory_test;
> Hence code could be reused and then removed form telemetry_inventory_test.

* collect folloing attributes for each component of FABRIC_CARD type and verify corectness (mostly non-empty string)
  *   /components/component/state/description             
  *   /components/component/state/hardware-version
  *   /components/component/state/id
  *   /components/component/state/mfg-name
  *   /components/component/state/name
  *   /components/component/state/oper-status
  *   /components/component/state/parent
  *   /components/component/state/part-no
  *   /components/component/state/serial-no
  *   /components/component/state/type

  *   /components/component/state/location
  *   /components/component/state/last-reboot-time
* store list of present components of FABRIC_CARD type

### test 2 redundancy
* Power down all but one component
* Run traffic between ATE por1 and port 2 for 16 millions of packets
* verify loss-lessness (10<sup>-6</sup> tolerance)
* Power up all components

### test 3 last reboot time
* Select one component from list of available
* store last-reboot-time for this component as "No_want"
* Power down this component, wait 60 sec.
* Power up this component
* Wait
* get last-reboot-timea and compare with "want"
  * "No_want" must be smaller (earlier) then recently collectrd last-reboot-time
    

## Config Parameter coverage

*   /components/component/fabric/config/power-admin-state

## Telemetry Parameter coverage

*   /components/component/fabric/state/power-admin-state
*   /components/component/state/description             
*   /components/component/state/hardware-version
*   /components/component/state/id
*   /components/component/state/mfg-name
*   /components/component/state/name
*   /components/component/state/oper-status
*   /components/component/state/parent
*   /components/component/state/part-no
*   /components/component/state/serial-no
*   /components/component/state/type
*   /components/component/state/location
*   /components/component/state/last-reboot-time

## Minimum DUT platform requirement
*   MFF