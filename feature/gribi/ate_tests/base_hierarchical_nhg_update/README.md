# TE-3.7: Base Hierarchical NHG Update

## Summary

Validate NHG update in hierarchical resolution scenario

## Procedure

*   Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2, ATE port-3 to
    DUT port-3.
*   Create a non-default VRF (VRF-1) that includes DUT port-1.
*   Establish gRIBI client connection with DUT and make it become leader.
*   Use Modify RPC to install entries per the following order, and ensure FIB
    ACK is received for each of the AFTOperation:
    *   Add 203.0.113.1/32 (default VRF) to NextHopGroup (NHG#42 in default VRF)
        containing one NextHop (NH#40 in default VRF) that specifies DUT port-2 as
        the egress interface and 00:1A:11:00:1A:BC as the destination MAC
        address.
    *   Add 198.51.100.0/24 (VRF-1) to NextHopGroup (NHG#44 in default VRF) containing one
        NextHop (NH#43 in default VRF) specified to be 203.0.113.1/32 in the default VRF.
*   Ensure that ATE port-2 receives the packets with 00:1A:11:00:1A:BC as
    the destination MAC address.
*   Use the Modify RPC with ADD operation to test NHG implicit in-place
    replace (step by step as below):
    1. Add a new NH (NH#41) with egress interface that specifies DUT port-3 as the
        egress interface and 00:1A:11:00:1A:BC as the destination MAC address.
    2. Add the same NHG#42 but reference both NH#40 and NH#41.
    3. Validate that both ATE port-2 and ATE port-3 receives the packets with 00:1A:11:00:1A:BC as the destination MAC address.
    4. Add the same NHG#42 but reference only NH#41.
    5. Validate that only ATE port-3 receives the packets.
    6. Add the same NHG#42 but reference only NH#40.
    7. Validate that only ATE port-2 receives the packets

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
                        *   mac_address
                        *   interface_ref
        *   ModifyResponse:
            *   AFTResult:
                *   id
                *   status

## Minimum DUT platform requirement

vRX if the vendor implementation supports FIB-ACK simulation, otherwise FFF.

# TE-3.7: Drain Implementation Test.

## Summary

Validate NHG update in Drain Implementation Test.

## Procedure

*   Steps:
*   Topology
*   [ATE port-1] — [port-1 DUT port-2] — [port-2 ATE]
                                             Port-3]—-[port-3 ATE]
                                             Port-4]—-[port-4 ATE]
*   DUT port-2, port-3 and port-4 are each making a one-member trunk port (trunk-2 and trunk-3, trunk-4).
*   Configure a destination network-a connected to trunk-2, trunk-3 and trunk-4.
*   gRIBI installs the following routing structure (700 IPv4 prefix, NHG and NH numbers stays the same as the             illustration), and expect FIB ACKs:
*   In the DEFAULT VRF:
    VIP1 -> NHG#1 -> [NH#1 {mac: MagicMAC, interface: DUTPort2Trunk}, NH#2 {mac: MagicMAC, interface: DUTPort3Trunk}]
    NHG#10 -> NH#10 {decap, network-instance: DEFAULT VRF}
    NHG#20 -> [ NH#20{ip: VIP1}, backupNH: NHG#10]

*   In a non-defualt VRF, VRF-1:
    IPv4Entries(1000 /32 IPv4 entries) -> NHG#20

*   Send 10K IPinIP traffic flows from ATE port-1 to network-a. 
*   Validate that traffic is going via trunk-2 and trunk-3 and  there is no traffic loss.
*   Send one gRIBI NHG#1 update to replace NH#1 and NH#2 with NH#3 pointing to trunk#4. 
*   Expect FIB ACKs, and validate that all traffic are moved to trunk#4 with no traffic loss.
*   Send one gRIBI NHG#1 update to revert back the changes above. 
*   Expect FIB ACKs and validate that the traffic is moved back to trunk-2 and trunk-3 with less than <xx> ms traffic loss.
