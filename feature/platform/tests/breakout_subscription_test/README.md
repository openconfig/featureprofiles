# PLT-1.2: OnChange Subscription Test for Breakout Interfaces

## Summary

OnChange Subscription Test for Breakout Interfaces

## Testbed type

*  TESTBED_DUT_ATE_2LINKS

## Procedure

* Connect ATE port-1 to DUT port-1, and ATE port-2 connected to DUT port-2.
* Configure lacp between DUT with DUT ports 1 and 2 and ATE with ATE ports 1 and 2 as member interfaces.
* Send a single `SubscribeRequest` message to the DUT member ports with a SubscriptionList and SubscriptionMode as ONCHANGE for the paths covered in telemetry coverage.

### Check response after a triggered interface state change

  * Change the admin status of DUT port 1 and check if subscription request detects the changes in below paths.
    /interfaces/interface/state/admin-status
    /lacp/interfaces/interface/members/member/interface
    /interfaces/interface/state/hardware-port
    /interfaces/interface/state/id
    /interfaces/interface/state/oper-status
    /components/component/state/oper-status
    /interfaces/interface/state/forwarding-viable
  
  * Record the responses of all the paths covered in telemetry coverage section. 


### Check response after a triggered reboot

  * Issue a reboot to the device and check if all the leafs are still being popuated.
  * Compare if the responses match to the ones recorded in the previous step. 
 
## OpenConfig Path and RPC Coverage

```yaml

paths:
 ## State paths: SubscriptionMode: ON_CHANGE ##
  /interfaces/interface/state/admin-status:
  /lacp/interfaces/interface/members/member/interface:
  /interfaces/interface/ethernet/state/mac-address:
  /interfaces/interface/state/hardware-port:
  /interfaces/interface/state/id:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/ethernet/state/port-speed:
  /components/component/integrated-circuit/state/node-id:
  /components/component/state/parent:
  /components/component/state/oper-status:
  /interfaces/interface/state/forwarding-viable:

rpcs:
  gnmi:
    gNMI.Subscribe:
      Mode: [ "ON_CHANGE" ]
    gNMI.Set:
```
## Required DUT platform
Single DUT
