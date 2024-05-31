# RT-1.8: BGP Route Reflector Test

## Summary

*   BGP route reflector capabilities check.
*   Ensure functionality of different OC paths for "supported-capabilities", "BGP peer-type", "BGP
    Neighbor details" and "BGP transport session parameters". 

## Topology

    DUT (Port1)   <- EBGP ->   (Port1) ATE
    DUT (Port2) <-IS-IS/IBGP-> (Port2) ATE
    DUT (Port3) <-IS-IS/IBGP-> (Port3) ATE

*   Connect ATE Port1 to DUT port1 (EBGP peering)
*   Connect ATE Port2 to DUT port2 (For IS-IS adjacency and IBGP peer reachability)
*   Connect ATE Port3 to DUT port3 (For IS-IS adjacency and IBGP peer reachability)

## Procedure

*   Establish IS-IS adjacency between ATE Port2 <-> DUT Port2, ATE Port3 <-> DUT Port3.

*   Establish BGP sessions as follows between ATE and DUT.

    *   ATE port 2 and ATE port3 are emulating RR clients peered with the DUT acting as the RR server.
    *   DUT's loopback address should be used for the IBGP peering and "transport/config/local-address"
        OC path should be used on DUT to configure BGP transport address for IBGP peering.
    *   ATE addresses used for the IBGP peering (different from ATE Port1 and ATE Port2 addreses) and
        DUT loopback addresses should be reachable via IS-IS.
    *   Each of RR clients should advertise 500k unique ipv4 routes and 200k ipv6 routes. These prefixes
        represent internal subnets and should include some prefixes that are unique to each of the ATEs.
        Remaining prefixes in the mix need to be common between the 2xATEs and should have identical path
        attributes except for the protocol next-hops.
    *   RR clients and eBGP Peer should advertise 1M overlapping ipv4 routes and 600k ipv6 routes. These
        1M are non RFC1918 or RFC6598 addresses and represent Internet prefixes.Similarly, 600k IPv6
        prefixes will represent internet prefixes. These prefixes should be common between the RR clients
        with different path-attributes for protocol next-hop, AS-Path and community.
    *   The DUT Port1 has eBGP peering with ATE Port 1 and is receiving 1M IPv4 and 400k IPv6 routes.
        DUT should automatically determine the BGP transport source address based on the nearest interface.
        Hence, the OC path "transport/config/local-address" shouldnt be used.

*   Validate session state on ATE ports and DUT using telemetry.
    *   The check should also include accurately receiving values for the path 
        "transport/state/local-address" for RRCs as well as for the EBGP peering.
    *   Validate accuracy of the peer-type leaf (neighbor/config/peer-type) for EBGP and IBGP peering.
    *   Validate session state and capabilities received on DUT using telemetry.
    *   Validate route receipt.
        *   Ensure that the DUT advertises all the IBGP learnt routes to the EBGP peer.
        *   Ensure that the DUT advertises all the EBGP learnt routes to the IBGP peers.
        *   Ensure that the DUT as RR server advertises routes learnt from each of the RRC to the other.

*   Validate BGP route/path attributes below for each of the EBGP and IBGP learnt routes
    *   Next-Hop
    *   Local Pref
    *   Communities
    *   AS-Path

## Config Parameter Coverage
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/route-reflector
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/route-reflector/config
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/route-reflector/
    config/route-reflector-cluster-id
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/route-reflector/
    config/route-reflector-client
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-type
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/config/
    local-address

## Telemetry Parameter Coverage
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/route-reflector/state/
    route-reflector-cluster-id
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/route-reflector/state/
    route-reflector-client
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/peer-type
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/
    supported-capabilities
*   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/
    neighbors/neighbor/adj-rib-in-pre/routes/route/state/attr-index
*   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/
    local-pref
*   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/next-hop
*   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/
    as-segment/state/

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

```yaml
paths:
  rpcs:
    gnmi:
      gNMI.Subscribe:
```
