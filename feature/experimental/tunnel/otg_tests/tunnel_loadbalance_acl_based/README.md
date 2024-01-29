# TUN-1.1: Filter based IPv4 GRE encapsulation
# TUN-1.2: Filter based IPv6 GRE encapsulation

## Summary

Verify the GRE acl based tunnel encapsulation with Load balancing traffic.

## Procedure

*   Configure DUT with IPv4 and IPv6 address on ingress and egress routed interfaces.
*   Configure LAG on the egress interface. 
*   Configure acl based tunnel configuration with action as encapsulation.
*   Attach the filter on ingress interface.
*   Configure the static route for the tunnel end point destination.
*   Send 1000 IPv4 and IPv6 traffic flows from traffic gerenator
*   Capture packet on ATE on the recieving end(port-2).
*   verify gre encapsulation on the captured packet.
*   verify that no traffic drops in all flows and traffic is load balanced across the interfaces.

## Config Parameter coverage

*   /acl/interfaces/interface/ingress-acl-sets/ingress-acl-set/config/set-name
*   /acl/interfaces/interface/ingress-acl-sets/ingress-acl-set/config/set-type

## Validation coverage

*   No packet drop should be oberserved.
*   Capture the packet on recieving end and verify the gre encapsulation.
    
