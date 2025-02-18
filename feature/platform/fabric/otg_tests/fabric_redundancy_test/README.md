# gNMI-1.16: Fabric redundnacy test

## Summary
- collect inventory data for each fabric card
- Verify last restart time is updated
- verify traffic could be forwarded with one of Fabric Card inactive.

## Procedure
### topology and basic setup
*  Connect OTG port1 to DUT port1 and OTG port2 to DUT port2
*  Configure IPv6 addresses on both links
### test 1 Fabric inventory

* collect following attributes for each component of FABRIC_CARD type and verify corectness (mostly non-empty string)
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
* Power down exectly one component of fabric type
* Run traffic between ATE por1 and port 2 for 16 millions of packets on 100kpps rate and using 4000B packets.
* verify loss-lessness (with 10E-6 tolerance); Since remaining fabric is not overloaded in any form
  with above traffic pattern 0 losses is expected
* Power up all components

### test 3 last reboot time
* Select one component from list of available
* store last-reboot-time for this component as "PREVIOUS_REBOOT_TIME"
* Power down this component, wait 60 sec.
* Power up this component
* Wait
* get last-reboot-time and compare with "PREVIOUS_REBOOT_TIME". The "PREVIOUS_REBOOT_TIME" must be smaller (earlier) then recently collected last-reboot-time
    
## OpenConfig Path and RPC Coverage

```yaml
paths:
   ## Config Parameter coverage

   /components/component/fabric/config/power-admin-state:
      platform_type: [ "FABRIC" ]

   ## Telemetry Parameter coverage

   /components/component/fabric/state/power-admin-state:
      platform_type: [ "FABRIC" ]
   /components/component/state/description:
      platform_type: [ "FABRIC" ]
   /components/component/state/hardware-version:
      platform_type: [ "FABRIC" ]
   /components/component/state/id:
      platform_type: [ "FABRIC" ]
   /components/component/state/mfg-name:
      platform_type: [ "FABRIC" ]
   /components/component/state/name:
      platform_type: [ "FABRIC" ]
   /components/component/state/oper-status:
      platform_type: [ "FABRIC" ]
   /components/component/state/parent:
      platform_type: [ "FABRIC" ]
   /components/component/state/part-no:
      platform_type: [ "FABRIC" ]
   /components/component/state/serial-no:
      platform_type: [ "FABRIC" ]
   /components/component/state/type:
      platform_type: [ "FABRIC" ]
   /components/component/state/location:
      platform_type: [ "FABRIC" ]
   /components/component/state/last-reboot-time:
      platform_type: [ "FABRIC" ]

rpcs:
    gnmi:
        gNMI.Set:
        gNMI.Subscribe:
```

## Minimum DUT platform requirement
*   MFF
