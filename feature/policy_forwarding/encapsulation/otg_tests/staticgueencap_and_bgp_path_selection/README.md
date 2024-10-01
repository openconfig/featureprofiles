# RT-3.35 Static GUE Encap and BGP path selection

## Summary
This is to
1. Test implementation of Static GUE encap whereby Tunnel endopoint is resolved over EBGP while the Payload's destination is learnt over IBGP
2. Prior to being GUE encaped, the LPM lookup on the payload destination undergoes route selection between different IBGP learnt routes and selects the ones w/ higher Local preference. In the absence of which the backup routes are selected.
3. Encaped traffic also gets the TTL and the TOS bits copied over from the inner header to the outer header. The same are verified at the other end.

## Topology
```mermaid
graph LR; 
subgraph DUT
    B1[Port1]
    B2[Port2]
    B3[Port3]
    B4[Port4]
end
subgraph ATE2
    C1[Port1]
    C2[Port2]
end
subgraph ATE3
    E1[Port1]
    E2[Port2]
    E3[Port3]
end
subgraph ATE4
    D1[Port1]
    D2[Port2]
end
A1[ATE1:Port1] <-- IBGP(ASN100) --> B1; 
B2 <-- IBGP(ASN100) --> C1; 
B3 <-- EBGP(ASN100:ASN200) --> D1;  
B4 <-- IBGP(ASN100) --> E1; 
C2 <-- IBGP(ASN100) --> E2; 
E3 <-- EBGP(ASN100:ASN200) --> D2;
```
