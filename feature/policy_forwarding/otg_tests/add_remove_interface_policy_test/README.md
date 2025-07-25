# Add/Remove Interface from Policy Forwarding Policy Test

## Summary

This test verifies that interfaces can be added to and removed from a policy-forwarding policy that uses a next-hop-group as an action.

## Topology

*   ATE port 1 <> DUT port 1 (ingress)
*   ATE port 2 <> DUT port 2 (egress 1)
*   ATE port 3 <> DUT port 3 (egress 2)

## Procedure

1.  Configure a policy-forwarding policy on the DUT to forward traffic to a next-hop-group.
2.  Initially, the next-hop-group contains only DUT port 2.
3.  Send traffic from ATE port 1 and verify it is received on ATE port 2.
4.  Add DUT port 3 to the next-hop-group.
5.  Send traffic from ATE port 1 and verify it is load-balanced between ATE port 2 and ATE port 3.
6.  Remove DUT port 3 from the next-hop-group.
7.  Send traffic from ATE port 1 and verify it is only received on ATE port 2.
