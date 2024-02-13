# RT-1.8: BGP Route Reflector Test at scale
## Summary
*   BGP route reflector capabilities check
*   Ensure functionality of different OC paths for "supported-capabilities", "BGP peer-type", "BGP Neighbor details" and "BGP transport session parameters"

## Topology
ATE (Port1) <-EBGP-> (Port1) DUT (Port2) <-IS-IS/IBGP-> (Port2) ATE, DUT (Port3) <-IS-IS/IBGP-> (Port3) ATE
Connect ATE Port1 to DUT port1 (EBGP peering)
*   Connect ATE Port2 to DUT port2 (For IS-IS adjacency and IBGP peer reachaility)
*   Connect ATE Port3 to DUT port3 (For IS-IS adjacency and IBGP peer reachaility)

## Test Procedure
* Establish IS-IS adjacency between ATE Port2 <-> DUT Port2, ATE Port3 <-> DUT Port3.
* Establish BGP sessions as follows between ATE and DUT.
  * ATE port2 and ATE port3 are emulating RR clients peered with the DUT acting as the RR server. DUT's loopback address should be used  for the IBGP peering and "transport/config/local-address" OC path should be used on DUT to configure BGP transport address for IBGP peering. ATE addresses used for the IBGP peering (different from ATE Port1 and ATE Port2 addreses) and DUT loopback addresses should be reachable via IS-IS. Each of the ATEs should advertise 5M IPv4 and 2M IPv6 prefixes to the RR over the IBGP peering, with different path attributes like protocol next-hops, local-preference, communitiy and MED. Among these, 2M v4 and 1M v6 prefixes are unique. Among the 2M IPv4 prefixes as well, 1.5M are non RFC1918 or RFC6598 addresses and represent Internet prefixes. Similarly, 600k IPv6 prefixes will represent internet prefixes. These prefixes should be common between the 2xDUTs with different path-attributes for protocol next-hop, AS-Path and community. Balance 500k IPv4 and 400k IPv6 prefixes represent internal subnets and should include some prefixes that are unique to each of the ATEs. Remaining prefixes in the mix need to be common between the 2xATEs and should have identical path attributes except for the protocol next-hops.
  * The DUT Port1 has eBGP peering with ATE Port 1 and is receiving 10M IPv4 and 3M IPv6 Internet routes with different path attributes like protocol next-hops, AS-Path, Origin, MED and communities. Out of these, only 1.5M IPv4 and 600k IPv6 prefixes are unique and will be programmed to the FIB.
    * The 1.5M IPv4 and 600k IPv6 unique prefixes representing internet addresses should be the same as the ones learnt by the DUT over the IBGP peering but with different path attributes like protocol Next-hops, communities etc.
    * The DUT should automatically determine the BGP transport source address based on the nearest interface. Hence, the OC path "transport/config/local-address" shouldnt be used.
    * The DUT should advertise these EBGP learnt prefixes over the 2xIBGP peering. [*] Expectations are that the DUT will prefer EBGP learnt internet routes over the IBGP learnt routes. For the IBGP learnt internal prefixes, the DUT should advertise the same over the EBGP peering. [**] Expectations are that the DUT would advertise all internal IPv4 and IPv6 prefixes to the EBGP peers. 
  * Validate session state on ATE ports and DUT using telemetry.
    * The check should also include accurately receiving values for the path "transport/state/local-address" for RRCs as well as for the EBGP peering.
    * Validate accuracy of the peer-type leaf (neighbor/config/peer-type) for EBGP and IBGP peering.
  * Validate session state and capabilities received on DUT using telemetry.
    * For capabilities, just make sure that the length of the list "neighbor/state/supported-capabilities" is non-zero
  * Validate route receipt.
    * Ensure that the DUT advertises all the IBGP learnt routes to the EBGP peer. Pass expectation covered in context in [**] above
    * Ensure that the DUT advertises all the EBGP learnt routes to the IBGP peers. Pass expectation covered in context in [*] above
    * Ensure that the DUT as RR server advertises routes learnt from each of the RRC to the other.
  * Validate BGP route/path attributes below for each of the EBGP and IBGP learnt routes
    * Next-Hop
    * Local Pref
    * Communities
    * AS-Path

## Config Parameter Coverage
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/route-reflector
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/route-reflector/config
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/route-reflector/config/route-reflector-cluster-id
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/route-reflector/config/route-reflector-client
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-type
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/config/local-address

## Telemetry Parameter Coverage
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/route-reflector/state/route-reflector-cluster-id
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/route-reflector/state/route-reflector-client
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/peer-type
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/supported-capabilities
