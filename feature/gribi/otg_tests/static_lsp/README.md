# TE-9.1: MPLS based forwarding Static LSP

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


## Config Parameter coverage

*   /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/config
*   /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/config/next-hop
*   /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/config/incoming-label
*   /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/config/push-label
*   /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/state