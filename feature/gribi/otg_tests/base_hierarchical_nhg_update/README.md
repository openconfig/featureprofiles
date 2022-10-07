# TE-3.7: Base Hierarchical NHG Update

## Summary

Validate NHG update in hierarchical resolution scenario

## Procedure

*   Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2, ATE port-3 to
    DUT port-3.
*   Create a non-default VRF (VRF-1) that includes DUT port-1.
*   Establish gRIBI client connection with DUT.
*   Use Modify RPC to install entries per the following order, and ensure FIB
    ACK is received for each of the AFTOperation:
    *   Add 203.0.113.1/32 (default VRF) to NextHopGroup (default VRF)
        containing one NextHop (default VRF) that that specifies DUT port-2 as
        the egress interface and 00:1A:11:00:00:01 as the destination MAC
        address.
    *   Add 198.51.100.0/24 (VRF-1) to NextHopGroup (default VRF) containing one
        NextHop (default VRF) specified to be 203.0.113.1/32 in the default VRF.
*   TODO: Ensure that ATE port-2 receives the packets with 00:1A:11:00:00:01 as
    the destination MAC address.
*   Use the Modify RPC to add the following entries with an implicit in-place
    replace (ADD operation):
    *   Add a new NH with egress interface that specifies DUT port-3 as the
        egress interface and 00:1A:11:00:00:01 as the destination MAC address.
    *   Add the same NHG (for 203.0.113.1/32) but pointing to the new added NH.
*   TODO: Ensure that ATE port-3 receives the packets with 00:1A:11:00:00:01 as
    the destination MAC address.

## Config Parameter coverage

No configuration relevant.

## Telemetry Parameter coverage

For prefix:

*   /network-instances/network-instance/afts/

Parameters:

*   ipv4-unicast/ipv4-entry/state
*   ipv4-unicast/ipv4-entry/state/next-hop-group
*   ipv4-unicast/ipv4-entry/state/origin-protocol
*   ipv4-unicast/ipv4-entry/state/prefix
*   next-hop-groups/next-hop-group/id
*   next-hop-groups/next-hop-group/next-hops
*   next-hop-groups/next-hop-group/next-hops/next-hop
*   next-hop-groups/next-hop-group/next-hops/next-hop/index
*   next-hop-groups/next-hop-group/next-hops/next-hop/state
*   next-hop-groups/next-hop-group/next-hops/next-hop/state/index
*   next-hop-groups/next-hop-group/state
*   next-hop-groups/next-hop-group/state/id
*   next-hops/next-hop/index
*   next-hops/next-hop/interface-ref
*   next-hops/next-hop/interface-ref/state
*   next-hops/next-hop/interface-ref/state/interface
*   next-hops/next-hop/interface-ref/state/subinterface
*   next-hops/next-hop/state
*   next-hops/next-hop/state/index
*   next-hops/next-hop/state/ip-address
*   next-hops/next-hop/state/mac-address

## Protocol/RPC Parameter coverage

*   gRIBI:
    *   Modify()
        *   ModifyRequest:
            *   AFTOperation:
                *   id
                *   network_instance
                *   op
                *   Ipv4
                    *   Ipv4EntryKey: prefix
                    *   Ipv4Entry: next_hop_group
                *   next_hop_group
                    *   NextHopGroupKey: id
                    *   NextHopGroup: next_hop
                *   next_hop
                    *   NextHopKey: id
                    *   NextHop:
                        *   ip_address
                        *   mac_address
                        *   interface_ref
        *   ModifyResponse:
            *   AFTResult:
                *   id
                *   status

## Minimum DUT platform requirement

vRX if the vendor implementation supports FIB-ACK simulation, otherwise FFF.
