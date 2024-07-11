# TE-9.2: MPLS based forwarding Static LSP

## Summary

Validate static lsp functionality.

## Procedure

*  Create topology ATE1–DUT1-ATE2
*  Enable MPLS forwarding and create egress static LSP to pop the label and forward to ATE2:
*  Match incoming label (1000001)
*  Set IP next-hop
*  Set egress interface
*  Set the action to pop label
*  Start 2 traffic flows with specified MPLS tags IPv4-MPLS[1000002]-MPLS[1000001]
*  Verify that traffic is received at ATE2 with MPLS label [1000001] removed

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml
paths:
  ## Config paths
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/config/next-hop:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/config/incoming-label:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/config/push-label:

  ## State paths
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/state/next-hop:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/state/incoming-label:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/state/push-label:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/state/metric:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/state/interface:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/state/subinterface:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```
