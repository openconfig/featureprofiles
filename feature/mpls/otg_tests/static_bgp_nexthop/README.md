# MPLS-2.2: MPLS forwarding via static LSP to BGP next-hop.

## Summary

Validate static LSP functionality with BGP resolved next-hop. This test verifies that the DUT can forward MPLS traffic based on a static LSP that uses a next-hop resolved via BGP.

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

2)  Configure eBGP peer on ATE Port 2 interface and advertise `BGP-NH-V4= 203.0.200.0/24` and `BGP-NH-V6= 2001:db8:128:200::/64`
3) Configure static routes on the DUT to discard traffic destined for BGP-NH-V4 and BGP-NH-V6. These routes should point to a Null0  with an administrative distance of 254 to ensure they are less preferred than the BGP routes. This prevents the DUT from using its IGP to reach the BGP next-hops.
4)  Enable MPLS forwarding.
5)  Create egress static LSP for IPv4 and IPV6 traffic to pop the label and resolve the next-hop BGP-NH-V4 and BGP-NH-V6 respectivelly

```yaml
network-instances:  
  - network-instance:  
    mpls:  
      lsps:  
        static-lsps:  
          - static-lsp:  
            config:  
              name: "lsp-egress-v4"  
            egress:  
              next-hop: 203.0.200.1 
              incoming-label: 1000004  
          - static-lsp:  
            config:  
              name: "lsp-egress-v6"  
            egress:  
              next-hop: 2001:db8:128:200::1  
              incoming-label: 1000006
```
    *  Set resolve NH action for both LSPs.

**TODO:** OC model does not support resolve next-hop option for LSPs.

7)  Configure static routes i.e. `IPV4-DST = 203.0.113.0/24` and `IPV6-DST = 2001:db8:128:128::/64` to ATE Port 3.
```yaml
network-instances:
  - network-instance:
    protocols:
      - protocol:
        static-routes:
          - static:
            config:
              prefix: "203.0.113.0/24"
            next-hops:
              - next-hop:
                config:
                  index:  1
                  next-hop: "ATE PORT 3"
          - static:
            config:
              prefix: "2001:db8:128:128::/64"
            next-hops:
              - next-hop:
                config:
                  index:  1
                  next-hop: "ATE PORT 3"
```

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


rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
```