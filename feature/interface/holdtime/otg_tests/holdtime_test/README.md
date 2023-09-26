# RT-5.5: Interface hold-time

## Summary

Verify configurability of interface hold-time down of 300ms  and hold-time up of 5 sec.\
Verify oper-state behaviour

## Procedure
*   Configure DUT port-1 to OTG port-1
*   Configure static LAG on DUT and OTG with port-1 as member
*   Configure hold-time down 300ms and hold-time up 5000ms
### TC1 - configuration verification:
*   Get hold-time state from device and check if it matches what was send in configuration. (some implementation may round-up/round-down values)
### TC2 - long down:
*   Read timestamp of last oper-status change  form DUT port-1 
*   Start sending Ethernet Remote Fault (RF) from OTG port-1 (or other mean which disable laser on OTG); read and store timestamp form OTG of this operation (OTG_STATE_CHANGE_TS).
*   wait 1000 ms
*   Read timestamp of last oper-status change  form DUT port-1 (DUT_LAST_CHANGE_TS)
*   Verify that DUT LAG:
  * oper-status is DOWN
  * oper-status last change time has changed 
  * DUT_LAST_CHANGE_TS = OTG_STATE_CHANGE_TS + 300ms +/- tolerance; Use tolerance of 200ms.
*   Stop sending Ethernet Remote Fault (RF) from OTG port-1 
### TC3 - short up:
*   Start sending Ethernet Remote Fault (RF) from OTG port-1 (or other mean which disable laser on OTG)
*   Read timestamp of last oper-status change   
*   Stop sending Ethernet Remote Fault (RF) from OTG port-1 for 4 seconds and then start send RF again. (or other mean which disable laser on OTG). Read and store timestamp form OTG of last operation (OTG_STATE_CHANGE_TS).
*   Read timestamp of last oper-status change  form DUT port-1 (DUT_LAST_CHANGE_TS)
*   Verify that DUT LAG:
  * oper-status is DOWN
  * oper-status last change time has NOT changed
*   Stop sending Ethernet Remote Fault (RF) from OTG port-1 
### TC4 - long  up:
*   Start sending Ethernet Remote Fault (RF) from OTG port-1 (or other mean which disable laser on OTG)
*   Read timestamp of last oper-status change   
*   Stop sending Ethernet Remote Fault (RF) from OTG port-1 (or other mean which disable laser on OTG). Read and store timestamp form OTG of last operation (OTG_STATE_CHANGE_TS).
*   wait 6 seconds
*   Read timestamp of last oper-status change  form DUT port-1 (DUT_LAST_CHANGE_TS)
*   Verify that DUT LAG:
  * oper-status is UP
  * oper-status last change time has changed
  * DUT_LAST_CHANGE_TS = OTG_STATE_CHANGE_TS + 5000ms +/- tolerance; Use tolerance of 200ms.

### TC5 - short down:
*   Read timestamp of last oper-status change   
*   Start sending Ethernet Remote Fault (RF) from OTG port-1 for **200ms** 
*   Verify that DUT LAG:
  * oper-status is UP
  * oper-status last change time has NOT changed
*   Stop sending Ethernet Remote Fault (RF) from OTG port-1 

## Config Parameter Coverage

*   /interfaces/interface/hold-time/config/up
*   /interfaces/interface/hold-time/config/down

## Telemetry Parameter Coverage

*   /interfaces/interface/hold-time/config/up
*   /interfaces/interface/hold-time/config/down
*   /interfaces/interface/state/oper-status
*   /interfaces/interface/state/last-change

## Protocol/RPC Parameter Coverage

None

## Minimum DUT Platform Requirement

FFF
