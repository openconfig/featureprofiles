# PF-1.6 Static GUE Encap and BGP path selection

## Summary
This is to
1. Test implementation of Static GUE encap whereby Tunnel endopoint is resolved over EBGP while the Payload's destination is learnt over IBGP
2. Prior to being GUE encaped, the LPM lookup on the payload destination undergoes route selection between different IBGP learnt routes and selects the ones w/ higher Local preference. In the absence of which, the backup routes are selected.
3. Encaped traffic also gets the TTL and the TOS bits copied over from the inner header to the outer header. The same are verified at the other end.
4. The DUT also performs GUEv1 Decap of the traffic received in the reverse direction.

## Topology
```mermaid
graph LR; 
subgraph DUT [DUT]
    B1[Port1]
    B2[Port2]
    B3[Port3]
    B4[Port4]
end

subgraph ATE2 [ATE2]
    C1[Port1]
    C2[Port2]
    C3[Port3]
end

A1[ATE1:Port1] <-- IBGP(ASN100) --> B1; 
B2 <-- IBGP(ASN100) --> C1; 
B3 <-- IBGP(ASN100) --> C2;
B4 <-- EBGP(ASN100:ASN200) --> C3;
```
