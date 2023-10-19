# RT-2.4: BGP Policy AS Path Set and Community Set

## Summary

BGP policy configuration for AS Paths and Community Sets

## Procedure

* RT-2.4.1 - Test setup
  * Use cfgLib to generate config for 2 DUT ports
  * Use cfgLib to generate config for DUT port 1 eBGP session

  * Use helper to configure ATE 2 ports
  * Use helper to configure ATE port 1 with 1 eBGP session

  * Establish eBGP sessions between ATE port-1 and DUT port-1
  * For IPv4 and IPv6 routes:
    * Advertise IPv4 prefixes over IPv4 neighbor from ATE port-1, observe
            received prefixes at ATE port-2.
    * Advertise IPv6 prefixes over IPv6 neighbor from ATE port-1,
        observe received prefixes at ATE port-2.
  * Generate traffic from ATE port-2 to ATE port-1
  * Validate that traffic can be received on ATE port-1 for all installed
        routes

* RT-2.4.1 - Validate single routing-policy containing as-path-set and ext-community-set
  * Configure ATE
    * Use paths and routes from RT-2.2.3 and RT-2.2.4 where each route have both aspath and ext-community set.
  * Configure DUT
    * Replace the DUT's bgp import-policy configuration for the bgp neighbor ATE port1 to use as-path-set-name `my_regex_aspaths` and community-set-name `my_regex_comms`
  * Send traffic
    * Verify traffic is forwarded for routes with matching policy
    * Verify traffic is not forwarded for routes without matching policy

* TODO: Add coverage for for route-target, route-origin and color in a separate test.

## Config Parameter Coverage

### Policy definition

* /routing-policy/policy-definitions/policy-definition/config/name
* /routing-policy/policy-definitions/policy-definition/statements/statement/config/name

### Policy for community-set match

* /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-set-name
* /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-member
* /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/match-set-options
* /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/config/community-set
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/
import-policy

### Policy for ext-community-set match

* /routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set/config/ext-community-set-name
* /routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set/config/ext-community-member
* /routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set/config/match-set-options
* /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/config/ext-community-set
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/
import-policy

### Policy for as-path match

* /routing-policy/defined-sets/bgp-defined-sets/as-path-sets/as-path-set/config/as-path-set-name
* /routing-policy/defined-sets/bgp-defined-sets/as-path-sets/as-path-set/config/as-path-set-member
* /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/match-as-path-set/config/as-path-set
* /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/match-as-path-set/config/match-set-options
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/
import-policy

## Telemetry Parameter Coverage

### Policy definition state

* /routing-policy/policy-definitions/policy-definition/state/name
* /routing-policy/policy-definitions/policy-definition/statements/statement/state/name

### Policy for community-set match state

* /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-set-name
* /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-member
* /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/match-set-options
* /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/state/community-set

### Policy for ext-community-set match state

* /routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set/state/ext-community-set-name
* /routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set/state/ext-community-member
* /routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set/state/match-set-options
* /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/state/ext-community-set

### Policy for as-path match state

* /routing-policy/defined-sets/bgp-defined-sets/as-path-sets/as-path-set/state/as-path-set-name
* /routing-policy/defined-sets/bgp-defined-sets/as-path-sets/as-path-set/state/as-path-set-member
* /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/match-as-path-set/state/as-path-set
* /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/match-as-path-set/state/match-set-options

### Paths to verify policy state

* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/export-policy
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy

### Paths to verify prefixes sent and received

* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/sent
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received-pre-policy
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/installed
