# RT-2.2: BGP Policy AS Path Set

## Summary

BGP policy configuration for AS Paths and Community Sets

## Procedure

* RT-2.2.1 - Test setup
  * Generate config for 2 DUT ports, with DUT port 1 eBGP session to ATE port 1

  * Generate config for ATE 2 ports, with ATE port 1 eBGP session to DUT port 1
  
  * Configure ATE port 1 to advertise ipv4 and ipv6 prefixes using the following as paths
    * routes-set-1 with as path `[100, 200, 300]`
    * routes-set-2 with as path `[100, 400, 300]`
    * routes-set-3 with as path `[110]`
    * routes-set-4 with as path `[400]`
    * routes-set-5 with as path `[100, 300, 200]`
    * routes-set-6 with as path `[1, 100, 200, 300, 400]`

  * Establish eBGP sessions between ATE port-1 and DUT port-1
  * Generate traffic from ATE port-2 to all prefixes
  * Validate that traffic is received on ATE port-1 for all installed routes

* RT-2.2.2 - Validate as-path-set
  * Configure DUT with the following routing policies
    * Create policy-definition named 'any_my_3_aspaths' with the following options
      * create as-path-set named "my_3_aspaths" with members
        * `{ as-path-set-member = [ "100", "200", "300" ] }`
      * conditions/bgp-conditions/match-as-path-set/config `{ match-set-options=ANY }`
      * with `{ policy-result=ACCEPT_ROUTE }` (note: default-policy action = REJECT)
    * Create an as-path-set/name "all_my_3_aspaths" as follows
      * `{ as-path-set-member = [ "100", "200", "300" ]}`
      * `{ match-set-options=ALL }`
      * with `{ policy-result=ACCEPT_ROUTE }`
    * Create an as-path-set/name "not_any_my_3_aspaths" as follows
      * `{ as-path-set-member = [ "100", "200", "300" ]}`
      * `{ match-set-options=INVERT }`
      * with `{ policy-result=ACCEPT_ROUTE }`
    * Create policy-definition named 'any_my_regex_aspath-1' with the following options
      * matches as-path-set named 'my_regex_aspath-1' with members
        * `{ as-path-set-member = [ "^100", "20[0-9]", "200$" ] }`
      * with match option `{ match-set-options=ANY }`
      * with `{ policy-result=ACCEPT_ROUTE }`
    * Create an as-path-set/name "any_my_regex_aspath-2" as follows
      * matches as-path-set `my_regex_aspath-2` `{ as-path-set-member = ["(^100)(.*)+(300$)" ] }`
      * with match option `{ match-set-options=ANY }`
      * with `{ policy-result=ACCEPT_ROUTE }`

  * For each DUT policy-definition configuration
    * Update the configuration for BGP neighbor policy (`.../apply-policy/config/import-policy`) to the selected as-path-set
      * Verify prefixes sent, received and installed are as expected
    * Send traffic
      * Verify traffic is forwarded for routes with matching policy
      * Verify traffic is not forwarded for routes without matching policy

### Expected prefix matches

| routes-set   | any_my_3_aspaths | all_my_3_aspaths | not_any_my_3_aspaths | any_my_regex_aspath-1 | any_my_regex_aspath-2 |
| ------------ | ---------------- | ---------------- | -------------------- | --------------------- | --------------------- |
| routes-set-1 | accept           | accept           | reject               | accept                | accept                |
| routes-set-2 | accept           | reject           | reject               | accept                | accept                |
| routes-set-3 | reject           | reject           | accept               | reject                | reject                |
| routes-set-4 | reject           | reject           | accept               | reject                | reject                |
| routes-set-5 | accept           | accept           | reject               | accept                | reject                |
| routes-set-6 | accept           | reject           | reject               | accept                | reject                |

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
