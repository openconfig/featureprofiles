# TE-3.1: Base Hierarchical Route Installation

## Summary
Validate IPv4 AFT support in gRIBI with recursion.

## Procedure
* Connect ATE port-1 to DUT port-1 and ATE port-2 to DUT port-2.
* Establish gRIBI client connection with DUT using default parameters.
* Using gRIBI Modify RPC install the following IPv4Entry set:
  * 198.51.100.0/24 to NextHopGroup containing one NextHop, specified to be 203.0.113.1/32
  * 203.0.113.1/32 to NextHopGroup containing one NextHop specified to be the address of ATE port-2.
* Forward packets between ATE port-1 and ATE port-2 (destined to 198.51.100.0/24)
  and determine that packets are forwarded successfully.
* Validate that both routes are shown as installed via AFT telemetry.
* Ensure that removing the NextHopGroup containing 203.0.113.1/32 with a DELETE
  operation results in traffic loss, and removal from AFT.
* Repeat above test with the following (table) cases:
  * TODO: Explicitly specified egress interface of DUT port-2.
  * TODO: Explicitly specified MAC address for ATE port-2.
  * TODO: Entry installed in a non-default network instance (with ATE port-1 and port-2 assigned to a non-default NI)
* Ensure that the following cases result in an error being returned:
  * TODO: Invalid IPv4 prefix in the IPv4Entry
  * TODO: Missing NextHopGroup within an IPv4Entry
  * TODO: Empty NextHopGroup
  * TODO: Empty NextHop
  * TODO: Invalid IPv4 address in NextHop.
* Validate that REPLACE operations:
  * TODO: Fail when using 2.0.0.0/8 within the Ipv4EntryKey (does not exist).
  * TODO: After installing a second next-hop-group with a different ID, validate that
    a REPLACE for 1.0.0.0/8 can update (in-place) the NHG to the new value.
    Validate via traffic and telemetry.

## Config Parameter coverage
No configuration relevant.

## Telemetry Parameter coverage

### For prefix:
* /network-instances/network-instance/afts/

### Parameters:
* ipv4-unicast/ipv4-entry/state
* ipv4-unicast/ipv4-entry/state/next-hop-group
* ipv4-unicast/ipv4-entry/state/origin-protocol
* ipv4-unicast/ipv4-entry/state/prefix
* next-hop-groups/next-hop-group/id
* next-hop-groups/next-hop-group/next-hops/next-hop/index
* next-hop-groups/next-hop-group/next-hops/next-hop/state
* next-hop-groups/next-hop-group/next-hops/next-hop/state/index
* next-hop-groups/next-hop-group/state/id
* next-hop-groups/next-hop-group/state/programmed-id
* next-hops/next-hop/index
* next-hops/next-hop/interface-ref/state/interface
* next-hops/next-hop/interface-ref/state/subinterface (not supported)
* next-hops/next-hop/state/index
* next-hops/next-hop/state/state/programmed-id (not supported)
* next-hops/next-hop/state/ip-address
* next-hops/next-hop/state/mac-address (not supported)

## Protocol/RPC Parameter coverage

* gRIBI:
  * Modify()
    * ModifyRequest:
      * AFTOperation:
        * id
        * network_instance
        * op
        * Ipv4
          * Ipv4EntryKey: prefix
          * Ipv4Entry: next_hop_group
        * next_hop_group
          * NextHopGroupKey: id
          * NextHopGroup: next_hop
        * next_hop
          * NextHopKey: id
          * NextHop:
            * ip_address
            * mac_address
            * interface_ref
  * ModifyResponse:
    * AFTResult:
      * id
      * status

## Minimum DUT platform requirement

vRX if the vendor implementation supports FIB-ACK simulation, otherwise FFF.

