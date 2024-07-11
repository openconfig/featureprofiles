# gNMI-1.3: Benchmarking: Drained Configuration Convergence Time

## Summary

Measure performance of drained configuration being applied.

## Procedure

Configure DUT with maximum number of IS-IS adjacencies, and BGP
peers - with physical interfaces between ATE and DUT for IS-IS
peers.

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
    *   BGP AS_PATH prepend:
        *   At t=0, send Set to DUT changing BGP policy for each session
            to prepend AS_PATH.
        *   Measure time between t=0 and all BGP received routes on ATE
            to report change in as path.
    *   TODO: BGP MED manipulation.   
        *   At t=0, send Set to DUT changing BGP policy for each session to
            set MED to non-default value.
        *   Measure time between t=0 and all BGP received routes on ATE to
            report changed metric.

## Config Parameter coverage

    *   BGP
        *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-med
        *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/repeat-n
        *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/as-number

    *   ISIS
        *   /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/afi-safi/af/state/metric
        *   /network-instances/network-instance/protocols/protocol/isis/global/lsp-bit/overload-bit/state/set-bit

## Telemetry Parameter coverage
    
    *   ISIS
        *   /interfaces/interfaces/levels/level/adjacencies/adjacency/state/adjacency-state
    *   BGP    
        *   /afi-safis/afi-safi/state/prefixes/sent
        *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor
