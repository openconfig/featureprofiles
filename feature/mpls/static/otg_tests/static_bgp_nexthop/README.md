# MPLS-2.2: MPLS forwarding via static LSP to BGP next-hop.

## Summary

Validate static LSP functionality with BGP resolved next-hop.

## Testbed type

*  [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Configuration

1) Create the topology below:

    ```
                         |         | ---- | ATE Port 2 | ---- [eBGP peer]
    [ ATE Port 1 ] ----  |   DUT   |      |            |
                         |         | ---- | ATE Port 3 |
    ```

2)  Configure eBGP peer on ATE Port 2 interface and advertise BGP-NH-V4 and BGP-NH-V6
3)  Configure discard static routes for BGP-NH-V4 and BGP-NH-V6 to Null0
4)  Enable MPLS forwarding.
5)  Create egress static LSP for IPv4 traffic to pop the label and resolve the next-hop BGP-NH
    *  Match incoming label (1000004)
    *  Set the action to pop label
    *  Set IPv4 next-hop and resolve NH action
6)  Create egress static LSP for IPv6 traffic to pop the label and resolve the next-hop BGP-NH
    *  Match incoming label (1000006)
    *  Set the action to pop label
    *  Set IPv4 next-hop and resolve NH action
7)  Configure cover static routes to IPV4-DST and IPV6-DST to ATE Port 3.

**TODO:** OC model does not support resolve next-hop option for LSPs.

### MPLS-2.2.1: Verify IPv4 MPLS forwarding

*   Push the above DUT configuration.
*   Start traffic flow with MPLS[lbl-1000004] and IPv4 destined to IPV4-DST.
*   Verify that traffic arrives to ATE Port 2.

### MPLS-2.2.2: Verify IPv6 MPLS forwarding

*   Push the above DUT configuration.
*   Start traffic flow with MPLS[lbl-1000006] and IPv4 destined to IPV6-DST.
*   Verify that traffic arrives to ATE Port 2.

### MPLS-2.2.3: Verify IPv4 traffic discard when BGP-NH is not available.

*   Withdraw BGP-NH-V4 advertisement.    
*   Push the above DUT configuration.
*   Start traffic flow with MPLS[lbl-1000004] and IPv4 destination set to IPV4-DST.
*   Verify that traffic is discarded.

### MPLS-2.2.4: Verify IPv6 traffic discard when BGP-NH is not available.

*   Withdraw BGP-NH-V6 advertisement.    
*   Push the above DUT configuration.
*   Start traffic flow with MPLS[lbl-1000006] and IPv6 destination set to IPV6-DST.
*   Verify that traffic is discarded.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  ## Config paths
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/config/next-hop:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/config/incoming-label:

  ## Telemetry for drop counter.
  /components/component/integrated-circuit/pipeline-counters/drop/lookup-block/state/no-route:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
```