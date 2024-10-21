# RT-2.2: IS-IS LSP Updates

## Summary

Ensure that IS-IS updates reflect parameter changes on DUT.

## Procedure

*   Configure L2 IS-IS adjacency between ATE port-1 and DUT port-1, and ATE
    port-2 and DUT port-2.

*   Validate that received LSDB on ATE has:

    *   TODO: Overload bit unset by default, change overload bit to set via DUT
        configuration, and ensure that the overload bit is advertised as set (as
        observed by the ATE). Ensure that DUT telemetry reflects the overload
        bit is set.

    *   TODO: Metric is set to the specified value for ATE port-1 facing DUT
        port via configuration, update value in configuration, and ensure that
        ATE and DUT telemetry reflects the change.

## OpenConfig Path and RPC Coverage
```yaml
paths:
  ## Config Parameter Coverage
  /network-instances/network-instance/protocols/protocol/isis/global/lsp-bit/overload-bit/config/set-bit:

  ## Telemetry Parameter Coverage
  /network-instances/network-instance/protocols/protocol/isis/global/lsp-bit/overload-bit/state/set-bit:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/afi-safi/af/state/metric:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
```

## Minimum DUT Platform Requirement

vRX
