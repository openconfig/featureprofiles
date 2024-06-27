# gNMI-1.2: Benchmarking: Full Configuration Replace 

## Summary

Measure performance of full configuration replace.

## Procedure

Configure DUT with:
 - The number of interfaces needed for the benchmarking test.
 - One BGP peer per interface.
 - One ISIS adjacency per interface.
Measure time required for Set operation to complete. 
Modify descriptions of a subset of interfaces within the system.
Measure time for Set to complete.

Notes:
This test does not measure the time to an entirely converged state, only to completion of the gNMI update.

## OpenConfig Path and RPC Coverage

```yaml
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:

paths:
  ## Config Parameter coverage
    /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-med:
    /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/repeat-n:
    /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/afi-safi/af/state/metric:
    /network-instances/network-instance/protocols/protocol/isis/global/lsp-bit/overload-bit/state/set-bit:

```

## Minimum DUT Required

vRX - Virtual Router Device


