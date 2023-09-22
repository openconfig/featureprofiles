# RT-2.2: BGP Policy using AS Paths and Community sets

## Summary

BGP policy configuration for AS Paths and Community Sets

## Procedure

* Subtest 1
    *   Establish eBGP sessions between ATE port-1 and DUT port-1
    *   For IPv4 and IPv6 routes:
        *   Advertise IPv4 prefixes over IPv4 neighbor from ATE port-1, observe received prefixes at ATE port-2.
    *   Similarly advertise IPv6 prefixes over IPv6 neighbor from ATE port-1, observe received prefixes at ATE port-2.
    *   Validate that traffic can be forwarded to **all** installed routes
        between ATE port-1 and ATE port-2
* Subtest 2
    *   Create table based tests to cover policy configuration under peer-group AFI for each of the below policies
        *   as-path with match-set-options ANY
            * `{ as-path-set-member = [ 100, 200, 300 ], match-set-options=ANY }`
        *   as-path using regex with match-set-options ANY
            * `{ as-path-set-member = [ '10[0-9]' ], match-set-options=ANY }`
        *   as-path with match-set-options ALL
              * `{ as-path-set-member = [ 100, 200, 300 ], match-set-options=ALL }`
        *   community-set with match-set-options ANY
            * `{ community-member = [ 1000, 2000, 3000 ], match-set-options=ANY }`
        *   community-set using regex with match-set-options ANY
            * `{ community-member = [ 100[0-9], 2000, 3000 ], match-set-options=ANY }`
        *   community-set with match-set-options ALL
            * `{ community-member = [ 1000, 2000, 3000 ], match-set-options=ALL }`
        
    *   For each table based test, validate that traffic can be forwarded to **all** installed routes
        between ATE port-1 and ATE port-2, validate that flows between all
        denied routes cannot be forwarded.
    *   Validate that traffic is not forwarded to withdrawn routes between ATE
        port-1 and ATE port-2.

## Config Parameter Coverage

* /routing-policy/policy-definitions/policy-definition/config/name
* /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-member	
* /routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set/config/ext-community-member

* /routing-policy/defined-sets/bgp-defined-sets/as-path-sets/as-path-set/config/as-path-set-name
* /routing-policy/defined-sets/bgp-defined-sets/as-path-sets/as-path-set/config/as-path-set-member	


## Telemetry Parameter Coverage

* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor
* /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group

* /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-member
* /routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set/state/ext-community-member	

* /routing-policy/defined-sets/bgp-defined-sets/as-path-sets/as-path-set/state/as-path-set-member	
* /routing-policy/defined-sets/bgp-defined-sets/as-path-sets/as-path-set/config/as-path-set-name

* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy	
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy	

* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/export-policy	
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy	


*   afi-safis/afi-safi/state/prefixes/installed
*   afi-safis/afi-safi/state/prefixes/received
*   afi-safis/afi-safi/state/prefixes/received-pre-policy

*   afi-safis/afi-safi/state/prefixes/sent
