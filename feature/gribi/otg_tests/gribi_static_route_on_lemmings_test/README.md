# GRIBI static route on lemmings test

## Summary

Ensure the GRIBI injecting the route entry on lemmings (Control plane), which is brought up in KNE topology

## Topology

ATE port-1 <------> port-1 DUT
DUT port-2 <------> port-2 ATE


## Baseline

```
### Install the following gRIBI AFTs.

- IPv4Entry {198.50.100.64/32 (DEFAULT)} -> NHG#2 (DEFAULT VRF) -> {
    {NH#2, DEFAULT VRF, weight:1}, interface-ref:dut-port-2-interface,
  }

```

## Procedure

The DUT should be reset to the baseline after each of the following tests.

Test-1, Configure the route entry on lemmings in KNE topology

```
1. Configure the DUT and OTG.
2. Configure Nh, NHG and IPv4 entry through gRIBI route injection.
3. Validate the acknowledgement for FIB installation success.

```

## OpenConfig Path and RPC Coverage
```yaml
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
  gribi:
    gRIBI.Get:
    gRIBI.Modify:
    gRIBI.Flush:
```
