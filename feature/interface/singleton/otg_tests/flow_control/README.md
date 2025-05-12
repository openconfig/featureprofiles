# RT-5.13: Flow control test

## Summary

Validate state of flow control for a interface

## Procedure

*   Connect DUT-1 port 1 to ATE-1 port 1.

*   Enable flow control on the interface and check the state of the flow-control
  for the port. it should return true
*   Disable flow control on the interface and check the state of the
  flow-control for the port. it should return false


## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
  ## Config Paths ##
   /interfaces/interface/ethernet/config/port-speed:
   /interfaces/interface/ethernet/config/duplex-mode:
   /interfaces/interface/ethernet/config/enable-flow-control:


  ## State Paths ##
   /interfaces/interface/ethernet/state/enable-flow-control:
rpcs:
  gnmi:
    gNMI.Get:
```
## Minimum DUT platform requirement
FFF
