# RT-2.2: BGP Policy AS Path Set

## Summary

BGP policy configuration for AS Paths and Community Sets

## Procedure

* RT-2.2.1 - Test setup
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

* RT-2.2.2 - Validate as-path-set
  * Configure DUT for each of the following policies
    * Create an as-path-set/name "my_3_aspaths" as follows
      * `{ as-path-set-member = [ "100", "200", "300" ] }`
      * `{ match-set-options=ANY }`
    * Create an as-path-set/name "my_regex_aspath-1" as follows
      * `{ as-path-set-member = [ "^100", "20[0-9]", "200$" ] }`
      * `{ match-set-options=ANY }`
    * Create an as-path-set/name "my_regex_aspath-2" as follows
      * `{ as-path-set-member = ["^100", ".*", "300$" ] }`
      * `{ match-set-options=ANY }`
    * Create an as-path-set/name "all_3_aspaths" as follows
      * `{ as-path-set-member = [ "100", "200", "300" ]}`
      * `{ match-set-options=ALL }`
  * Configure ATE to
    * Advertise routes-set-1 with as path `[100, 200, 300]`
    * Advertise routes-set-2 with as path `[100, 400, 300]`
    * Advertise routes-set-3 with as path `[110]`
    * Advertise routes-set-4 with as path `[400]`
  * For each DUT policy configuration
    * Update the configuration for BGP neighbor policy (`.../apply-policy/config/import-policy`) to the selected as-path-set
      * Verify prefixes sent, received and installed are as expected
    * Send traffic
      * Verify traffic is forwarded for routes with matching policy
      * Verify traffic is not forwarded for routes without matching policy

## Config Parameter Coverage

### Policy definition

* /routing-policy/policy-definitions/policy-definition/config/name
* /routing-policy/policy-definitions/policy-definition/statements/statement/config/name

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
