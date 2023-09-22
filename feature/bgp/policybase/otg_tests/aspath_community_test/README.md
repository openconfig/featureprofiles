# RT-2.2: BGP Policy using AS Paths and Community sets

## Summary

BGP policy configuration for AS Paths and Community Sets

## Procedure

* Subtest 1
    * Establish eBGP sessions between ATE port-1 and DUT port-1
    * For IPv4 and IPv6 routes:
        * Advertise IPv4 prefixes over IPv4 neighbor from ATE port-1, observe 
            received prefixes at ATE port-2.
    * Advertise IPv6 prefixes over IPv6 neighbor from ATE port-1, 
        observe received prefixes at ATE port-2.
    * Validate that traffic can be forwarded to **all** installed routes
        between ATE port-1 and ATE port-2

* Subtest 2 for as-path-set
    * Create table based tests for each of the following policies
    * Create an as-path-set/name "my_3_aspaths" with members and match options as follows
        *  `{ as-path-set-member = [ "100", "200", "300" ], match-set-options=ANY }`
    * Create an as-path-set/name "my_regex_aspaths" with one member as follows
        * `{ as-path-set-member = [ "10[0-9]" ], match-set-options=ANY }`
    * Create an as-path-set/name "all_3_aspaths" with members and match options as follows
        * `{ as-path-set-member = [ "100", "200", "300" ], match-set-options=ALL }`
  * For each table based test, validate that traffic can be forwarded to
    all installed routes between ATE port-1 and ATE port-2, validate that
    traffic flows between all denied routes cannot be forwarded.
      * Advertise routes in with as path `[100, 200, 300]` 
      * Advertise routes in with as path `[100, 101, 200]` 
      * Advertise routes with as path `[110]` 
      * Advertise routes with as path `[400]` 


* Subtest 3 for community-set
    * Create table based tests for each of the following policies
    * Create a named community-set with members and match options as follows
        * `{ community-member = [ "1000", "2000", "3000" ], match-set-options=ANY }`
    *   Create a named community-set with members and match options as follows
          * `{ community-member = [ "100[0-9]" ], match-set-options=ANY }`
  *   Create a named community-set with members and match options as follows
      * `{ community-member = [ "1000", "2000", "3000" ], match-set-options=ALL }`     
  * For each table based test, validate that traffic can be forwarded to
    all installed routes between ATE port-1 and ATE port-2, validate that
    traffic flows between all denied routes cannot be forwarded.
      * Advertise routes with communities `[1000,2000,3000]` 
      * Advertise routes with communities `[1000,1001,2000]`
      * Advertise routes with communities `[1100]` 
      * Advertise routes with communities `[4000]` 
      * Verify traffic is forwarded for routes with matching policy
      * Verify traffic is not forwarded for routes without matching policy

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
