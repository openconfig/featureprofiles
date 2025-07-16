# RT-2.9: IS-IS  metric style wide enabled

## Summary

*  Base IS-IS functionality and adjacency establishment.
*  Verifies route metric with wide metric enabled on DUT.

## Procedure

    *	Configure IS-IS for ATE port-1 and DUT port-1.
    *	Enable wide metric style on ATE and DUT.
    *	Advertise ISIS prefixes from ATE with wide metrics (value > 63).
    *	Verify that IS-IS adjacency for IPv4 and IPV6 address family is coming up.
    *	Verify that IPv4 and IPv6 prefixes that are advertised by ATE correctly installed into DUTs route and forwarding table.
    *	TODO-Verify that the metrics of the IPv4 and IPv6 prefixes is  correctly reflected
    *	Ensure that IPv4 and IPv6 prefixes that are advertised as part of an (emulated) neighboring system are installed into the DUT routing table, and validate that packets are sent and received to them.


## OpenConfig Path and RPC Coverage

```yaml
paths:
  # isis config
  /network-instances/network-instance/protocols/protocol/isis/global/config/authentication-check:
  /network-instances/network-instance/protocols/protocol/isis/global/config/net:
  /network-instances/network-instance/protocols/protocol/isis/global/config/level-capability:
  /network-instances/network-instance/protocols/protocol/isis/global/config/hello-padding:
  /network-instances/network-instance/protocols/protocol/isis/global/afi-safi/af/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/config/level-number:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/config/metric-style:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/authentication/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/authentication/config/auth-mode:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/authentication/config/auth-password:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/authentication/config/auth-type:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/config/interface-id:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/config/circuit-type:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/config/passive:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/timers/config/csnp-interval:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/timers/config/lsp-pacing-interval:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/config/level-number:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/config/passive:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/timers/config/hello-interval:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/timers/config/hello-multiplier:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/hello-authentication/config/auth-mode:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/hello-authentication/config/auth-password:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/hello-authentication/config/auth-type:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/hello-authentication/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/afi-safi/af/config/afi-name:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/afi-safi/af/config/safi-name:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/afi-safi/af/config/enabled:
  # isis telemetry
  /network-instances/network-instance/protocols/protocol/isis/levels/level/state/metric-style:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/adjacency-state:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/neighbor-ipv4-address:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/neighbor-ipv6-address:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/system-id:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/area-address:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/dis-system-id:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/local-extended-circuit-id:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/multi-topology:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/neighbor-circuit-type:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/neighbor-extended-circuit-id:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/neighbor-snpa:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/nlpid:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/priority:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/restart-status:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/restart-support:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/restart-suppress:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/afi-safi/af/state/afi-name:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/afi-safi/af/state/metric:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/afi-safi/af/state/safi-name:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/auth-fails:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/auth-type-fails:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/corrupted-lsps:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/database-overloads:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/exceed-max-seq-nums:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/id-len-mismatch:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/lsp-errors:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/manual-address-drop-from-areas:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/max-area-address-mismatches:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/own-lsp-purges:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/part-changes:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/seq-num-skips:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/spf-runs:
  
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:  
```
