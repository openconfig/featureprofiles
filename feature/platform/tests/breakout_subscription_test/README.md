# PLT-1.2: OnChange Subscription Test for Breakout Interfaces

## Summary

OnChange Subscription Test for Breakout Interfaces

## Testbed type

*  TESTBED_DUT_ATE_2LINKS

## Procedure

* Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2 and ATE port-3 to DUT port-3. 
* Configure lacp between DUT with DUT ports 1 and 2 and ATE with ATE ports 1 and 2 as member interfaces.
* Configure DUT port 3 and ATE port 3 as singleton interfaces connected to eachother.
* Send a single `SubscribeRequest` message to the DUT Singleton and Member ports with a SubscriptionList and SubscriptionMode as ONCHANGE for the paths covered in telemetry coverage.

### PLT-1.2.1 Check response after a triggered interface state change

  * Change the admin status of DUT port 1 and port 3 and check if subscription request detects the changes in below paths.
    /interfaces/interface/state/admin-status
    /lacp/interfaces/interface/members/member/interface
    /interfaces/interface/state/hardware-port
    /interfaces/interface/state/id
    /interfaces/interface/state/oper-status
    /components/component/state/oper-status
    /interfaces/interface/state/forwarding-viable
  
  * Bring back DUT port 1 and port 3 to admin up state.
  * Record the responses of all the paths covered in telemetry coverage section. 

### PLT-1.2.2 Check response after a triggered interface flap

  * Disable/Shut DUT port 1 and 3 and verify if operational and admin state change is Down. Enable the interfaces again and verify if the states change to UP. 
  * Repeat this step 5 times and verify if subscription detects the stable state as recorded in subtest 1.2.1

### PLT-1.2.3 Check response after a triggered LC reboot

  * Issue a reboot to the Linecard and check if all the leafs are still being popuated and not empty.
  * Compare if the responses match to the ones recorded in subtest 1.2.1
    
### PLT-1.2.4 Check response after a triggered reboot

  * Issue a reboot to the device and check if all the leafs are still being popuated and not empty.
  * Compare if the responses match to the ones recorded in subtest 1.2.1
 
## OpenConfig Path and RPC Coverage

```yaml

paths:
 ## State paths: SubscriptionMode: ON_CHANGE ##
  /interfaces/interface/state/id:
  /interfaces/interface/state/hardware-port:
  /interfaces/interface/state/admin-status:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/state/forwarding-viable:
  /interfaces/interface/ethernet/state/port-speed:
  /interfaces/interface/ethernet/state/mac-address:
  /lacp/interfaces/interface/members/member/interface:
  /components/component/state/parent:
   platform_type: [ "INTEGRATED_CIRCUIT", "LINECARD" ]
  /components/component/state/oper-status:
   platform_type: [ "INTEGRATED_CIRCUIT", "LINECARD" ]
  /components/component/state/name:
   platform_type: [ "INTEGRATED_CIRCUIT", "LINECARD" ]
  /components/component/integrated-circuit/state/node-id:
   platform_type: [ "INTEGRATED_CIRCUIT" ]


rpcs:
  gnmi:
    gNMI.Subscribe:
      Mode: [ "ON_CHANGE" ]
    gNMI.Set:
  gnoi:
    system.System.Reboot:
    system.System.RebootStatus:
```
## Required DUT platform
Single DUT
