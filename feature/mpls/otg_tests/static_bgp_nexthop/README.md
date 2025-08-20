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

2)  Configure eBGP peer on ATE Port 2 interface and advertise `BGP-NH-V4=198.51.100.0/24` and `BGP-NH-V6=2001:DB8:100::0/64`
3)  Enable MPLS forwarding.
4)  Create egress static LSP for IPv4 and IPV6 traffic to pop the label and resolve the next-hop BGP-NH-V4 and BGP-NH-V6 respectivelly

*  Set resolve NH action for both LSPs.

**TODO:** OC model does not support resolve next-hop option for LSPs.

7)  Configure static routes i.e. `IPV4-DST = 198.51.100.0/24` and `IPV6-DST = 2001:DB8:100::0/64` to ATE Port 3 with administrative distance (preference) 254.

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
*   Verify that traffic is forwarded to ATE Port 3.

### MPLS-2.2.4: Verify IPv6 traffic discard when BGP-NH is not available.

*   Withdraw BGP-NH-V6 advertisement.    
*   Push the above DUT configuration.
*   Start traffic flow with MPLS[lbl-1000006] and IPv6 destination set to IPV6-DST.
*   Verify that traffic is forwarded to ATE Port 3.

## Canonical OC
OC for a static MPLS LSP is provided here.

```json
{
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "DEFAULT"
        },
        "mpls": {
          "lsps": {
            "static-lsps": {
              "static-lsp": [
                {
                  "config": {
                    "name": "lspv4"
                  },
                  "egress": {
                    "config": {
                      "incoming-label": 10004,
                      "next-hop": "203.0.200.1",
                      "push-label": "IMPLICIT_NULL"
                    }
                  },
                  "name": "lspv4"
                },
                {
                  "config": {
                    "name": "lspv6"
                  },
                  "egress": {
                    "config": {
                      "incoming-label": 10006,
                      "next-hop": "2001:db8:128:200::1",
                      "push-label": "IMPLICIT_NULL"
                    }
                  },
                  "name": "lspv6"
                }
              ]
            }
          }
        },
        "name": "DEFAULT"
      }
    ]
  }
}
```
## OpenConfig Path and RPC Coverage

```yaml
paths:
  ## Config paths
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/config/incoming-label:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/config/next-hop:
  /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/index:


rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
```
