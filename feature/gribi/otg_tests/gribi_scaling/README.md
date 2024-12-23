# TE-14.1: gRIBI Scaling

## Summary

Validate gRIBI scaling requirements.

## Procedure

*   Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2.
*   Create 64 L3 sub-interfaces under DUT port-2 and corresponding 64 L3
    sub-interfaces on ATE port-2
*   On DUT port-1 and ATE port-1 create a single L3 interface
*   On DUT, create a policy-based forwarding rule to redirect all traffic received from DUT port-1 into VRF-1 (based on src. IP match 111.111.111.111)
*   On DUT, create a policy-based forwarding rule to redirect all traffic received from DUT port-1 into VRF-2 (based on src. IP match 222.222.222.222)
*   Establish gRIBI client connection with DUT negotiating FIBACK as the
    requested ack_type and make it become leader.
*   TODO: Using gRIBI Modify RPC install the following IPv4Entry sets, and validate
    the specified behaviours:
    *   <Default VRF>
        * A) Install 400 NextHops, egressing out different interfaces.
        * B) Install 200 NextHopGroups.  Each points at 8 NextHops from the first 200 entries of A) with equal weight.
        * C) Install 200 IPv4 Entries, each pointing at a unique NHG (1:1) from B.
        * D.1) Install 100 NextHops.  Each will redirect to an IP from C).
        * D.2) Install 100 NextHops.  Each will redirect to an IP from C).
        * E) Install 100 NextHopGroups.  Each will contain 1 NextHops from D.1 with weights 1 and 1 NextHop from D.1 with weight 31. The backup next_hop_group will be to redirect to VRF3.
        * F) Install 100 NextHopGroups.  Each will contain 2 NextHops from D.1 with weights 1 abd 1 NextHop from D.2 with weight 31. The backup next_hop_group will be to decap and redirect to DEFAULT vrf.
        * G) Install 700 NextHops.  Each will decaps + reencap to an IP in VRF2.
        * H) Install 700 NextHopGroups.  Each will point to a NextHop from G) and have a backup next_hop_group to decap and redirect to DEFAULT vrf.
    *   <VRF1>
        *   Install 9000 IPv4Entries.  Each points to a NextHopGroup from E).
    *   <VRF2>
        *   Install 9000 IPv4Entries (Same IPAddress as VRF1).  Each points to a NextHopGroup from F).
    *   <VRF3>
        *   Install 9000 IPv4Entries (Same IPAddress as VRF1).  Each points to a NextHopGroup from H).
*   Validate that each entry above are installed as FIB_PROGRAMMED.
*   TODO: Add flows destinating to IPBlocks and ensure ATEPort2 receives it with
    no loss and proper weights

## OpenConfig Path and RPC Coverage
```yaml
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
  gribi:
    gRIBI.Get:
    gRIBI.Modify:
    gRIBI.Flush:
```
