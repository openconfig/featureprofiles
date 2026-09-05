# RT-5.5: Interface hold-time

## Summary

Verify configurability of interface hold-time down of 300ms and hold-time up of 180 sec (3 min).
Verify oper-state behavior and hardware-level traffic loss duration.

## Procedure
*   Configure DUT port-1 to OTG port-1
*   Configure static LAG on DUT and OTG with port-1 as member
*   Configure hold-time down 300ms and hold-time up 180000ms (3 minutes)
*   Configure traffic flow from ATE port-1 to ATE port-2 over the LAG interface.
### TC1 - configuration verification:
*   Get hold-time state from device and check if it matches what was send in configuration. (some implementation may round-up/round-down values)
### TC2 - long down:
*   Read timestamp of last oper-status change  form DUT port-1 
*   Start sending Ethernet Remote Fault (RF) from OTG port-1 (or other mean which disable laser on OTG); read and store the current time from DUT.
*   wait 500 ms
*   Read timestamp of last oper-status change  form DUT port-1 (DUT_LAST_CHANGE_TS)
*   Verify that DUT LAG:
  * oper-status is DOWN
  * oper-status last change time has changed 
  * DUT_LAST_CHANGE_TS = OTG_STATE_CHANGE_TS + 300ms +/- tolerance; Use tolerance of 700ms. (Ideal tolerance value is 200ms. But since we are reading the current time from DUT in step 2 instead of OTG we are accounting for processing delays)
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
*   wait 185 seconds
*   Read timestamp of last oper-status change  form DUT port-1 (DUT_LAST_CHANGE_TS)
*   Verify that DUT LAG:
  * oper-status is UP
  * oper-status last change time has changed
  * DUT_LAST_CHANGE_TS = OTG_STATE_CHANGE_TS + 180000ms +/- tolerance; Use tolerance of 200ms.

### TC5 - short down:
*   Read timestamp of last oper-status change   
*   Start sending Ethernet Remote Fault (RF) from OTG port-1 for **200ms** 
*   Verify that DUT LAG:
  * oper-status is UP
  * oper-status last change time has NOT changed
*   Stop sending Ethernet Remote Fault (RF) from OTG port-1 

### TC6 - traffic loss duration:
*   Start continuous traffic flow from ATE port-1 to ATE port-2 over the LAG interface.
*   Shutdown/disable the member link DUT port-1 (e.g., by sending Ethernet Remote Fault (RF) from OTG port-1).
*   Measure the duration of traffic loss (packet loss) on the traffic flow at the receiver.
*   Verify that the traffic loss duration \(t_{\text{loss}}\) satisfies the relation:
  * \(t_{\text{loss}} \le \text{hold-time down} + \text{failover tolerance}\)
  * Specifically, \(t_{\text{loss}} \le 300\text{ ms} + 100\text{ ms} = 400\text{ ms}\).

## Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "name": "Ethernet1",
        "config": {
          "name": "Ethernet1"
        },
        "hold-time": {
          "config": {
            "up": 180000,
            "down": 300
          }
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths and RPC intended to be covered by this test.

```yaml
paths:
  /interfaces/interface/hold-time/config/up:
  /interfaces/interface/hold-time/config/down:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/state/last-change:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT Platform Requirement

FFF

