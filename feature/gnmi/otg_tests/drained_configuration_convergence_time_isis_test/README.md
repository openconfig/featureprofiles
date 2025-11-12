# gNMI-1.3: Benchmarking: Drained Configuration Convergence Time

## Summary

Measure performance of drained configuration being applied.

## Procedure

Configure DUT with maximum number of IS-IS adjacencies with physical
interfaces between ATE and DUT for IS-IS peers.

First port is used as ingress port to send routes from ATE to DUT.

For each of the following configurations, generate complete device
configuration and measure time for the operation to complete (as
defined in the case):
    *   TODO: IS-IS overload:
        *   At t=0, send Set to DUT marking IS-IS overload bit.
        *   Measure time between t=0 and all IS-IS sessions on ATE to
            report DUT as overloaded.
    *   IS-IS metric change:
        *   At t=0, send Set to DUT marking IS-IS metric as changed for
            all IS-IS interfaces.
        *   Measure time between t=0 and all IS-IS sessions on ATE to
            report changed metric.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml
paths:
  ## Config Parameter coverage
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/afi-safi/af/state/metric:
  /network-instances/network-instance/protocols/protocol/isis/global/lsp-bit/overload-bit/state/set-bit:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
```
