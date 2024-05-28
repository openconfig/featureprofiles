# RT-2.15: IS-IS circuit-type point-to-point

## Summary

* IS-IS circuit-type point-to-point Test

## Topology

* ATE:port1 <-> DUT:port1

## Procedure

* Configure IS-IS for ATE port-1 and DUT port-1.
* Configure both DUT and ATE interfaces as ISIS type point-to-point.
    * Verify that IS-IS adjacency is coming up.
    * Verify the output of streaming telemetry path displaying the interface circuit-type as point-to-point.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
  ## Config paths
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/config/circuit-type:

  ## State paths
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/state/circuit-type:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

* MFF - A modular form factor device containing LINECARDs, FABRIC and redundant CONTROLLER_CARD components
* FFF - fixed form factor
