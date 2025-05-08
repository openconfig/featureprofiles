# RT-5.11: LACP Intervals

## Summary

Validate link operational status of LACP LAG and also validate lacp timers

## Procedure

*   Connect DUT-1 ports 1-4 to DUT-2 ports 1-4, Configure both
      DUTs ports 1-4 to be part of a LAG.

*   With LACP:
    *   Ensure that LAG is successfully negotiated, verifying port status for
        each of DUT ports 1-4 reflects expected LAG state via DUT telemetry.
    *   Verify LACP LAG intervals with period type SLOW and FAST


## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
  ## Config Paths ##
   /interfaces/interface/ethernet/config/port-speed:
   /interfaces/interface/ethernet/config/duplex-mode:
   /interfaces/interface/ethernet/config/aggregate-id:
   /interfaces/interface/aggregation/config/lag-type:
   /lacp/interfaces/interface/config/name:
   /lacp/interfaces/interface/config/interval:
   /lacp/interfaces/interface/config/lacp-mode:

  ## State Paths ##
   /lacp/interfaces/interface/state/name:
   /lacp/interfaces/interface/members/member/state/interface:
   /lacp/interfaces/interface/members/member/state/port-num:
   /interfaces/interface/ethernet/state/aggregate-id:
   /lacp/interfaces/interface/state/interval:
rpcs:
  gnmi:
    gNMI.Get:
```
