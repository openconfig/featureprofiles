# TE-3.1: Base Hierarchical Route Installation

## Summary

Validate IPv4 AFT support in gRIBI with recursion.

## Procedure

1.  Connect ATE port-1 to DUT port-1 and ATE port-2 to DUT port-2.
    *   TODO: create a non-default VRF (VRF-1) that includes DUT port-1.
    *   TODO: connect ATE port-3 to DUT port-3.
2.  Establish gRIBI client connection with DUT using default parameters.
3.  Using gRIBI Modify RPC install the following IPv4Entry set:
    *   198.51.100.0/24 to NextHopGroup containing one NextHop, specified to be
        203.0.113.1/32
    *   203.0.113.1/32 to NextHopGroup containing one NextHop specified to be
        the address of ATE port-2.
    *   TODO: ensure installing the above 2 sets in the following ordering, and
        receive [FIB_PROGRAMMED] for all the AFTOperations.
        1.  203.0.113.1/32 to NextHopGroup containing one NextHop specified to
            be the address of ATE port-2.
        2.  198.51.100.0/24 to NextHopGroup containing one NextHop, specified to
            be 203.0.113.1/32
4.  Forward packets between ATE port-1 and ATE port-2 (destined to
    198.51.100.0/24) and determine that packets are forwarded successfully.
5.  Validate that both routes are shown as installed via AFT telemetry.
6.  Ensure that removing the IPEntry 203.0.113.1/32 with a DELETE operation
    results in traffic loss, and removal from AFT.

TODO: Validate error reporting. * Repeat step 1-3 as above but with the
following (table) scenarios: * Replace 203.0.113.1/32 with a syntax invalid IP
address. * Missing NextHopGroup for the IPv4Entry 203.0.113.1/32. * Empty
NextHopGroup for the IPv4Entry 203.0.113.1/32. * Empty NextHop for the IPv4Entry
203.0.113.1/32. * Invalid IPv4 address in NextHop for the IPv4Entry
203.0.113.1/32. * Ensure [FAILED] returned for the related IPv4Entry, NHG and NH
in the above scenarios. * Ensure [RIB_PROGRAMMED] but not [FIB_PROGRAMMED] is
returned for the IPv4Entry 198.51.100.0/24 in all the scenarios.

TODO: Validate in-place update: * Repeat step 1-5 above. * Use the Modify RPC to
[ADD] a new NH with next-hop pointing to ATE port-3, and [ADD] the same NHG (for
198.51.100.0/24) but pointing to the new added NH. * Validate that routes are
pointing to ATE port-3 via AFT, and ensure traffic are now being forwarded to
ATE port-3.

[ADD]: https://github.com/openconfig/gribi/blob/08d53dffce45e942c6e7f07521c58b557984e4b7/v1/proto/service/gribi.proto#L171
[FAILED]: https://github.com/openconfig/gribi/blob/08d53dffce45e942c6e7f07521c58b557984e4b7/v1/proto/service/gribi.proto#L265
[RIB_PROGRAMMED]: https://github.com/openconfig/gribi/blob/08d53dffce45e942c6e7f07521c58b557984e4b7/v1/proto/service/gribi.proto#L269
[FIB_PROGRAMMED]: https://github.com/openconfig/gribi/blob/08d53dffce45e942c6e7f07521c58b557984e4b7/v1/proto/service/gribi.proto#L285

## Config Parameter coverage

No configuration relevant.

## Telemetry Parameter coverage

### For prefix:

*   /network-instances/network-instance/afts/

### Parameters:

*   ipv4-unicast/ipv4-entry/state
*   ipv4-unicast/ipv4-entry/state/next-hop-group
*   ipv4-unicast/ipv4-entry/state/origin-protocol
*   ipv4-unicast/ipv4-entry/state/prefix
*   next-hop-groups/next-hop-group/id
*   next-hop-groups/next-hop-group/next-hops/next-hop/index
*   next-hop-groups/next-hop-group/next-hops/next-hop/state
*   next-hop-groups/next-hop-group/next-hops/next-hop/state/index
*   next-hop-groups/next-hop-group/state/id
*   next-hop-groups/next-hop-group/state/programmed-id
*   next-hops/next-hop/index
*   next-hops/next-hop/interface-ref/state/interface
*   next-hops/next-hop/interface-ref/state/subinterface (not supported)
*   next-hops/next-hop/state/index
*   next-hops/next-hop/state/state/programmed-id (not supported)
*   next-hops/next-hop/state/ip-address
*   next-hops/next-hop/state/mac-address (not supported)

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
