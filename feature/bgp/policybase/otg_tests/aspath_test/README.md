# RT-7.3: BGP Policy AS Path Set

## Summary

BGP policy configuration for AS Paths and Community Sets

## Procedure

* RT-7.3.1 - Test setup
  * Generate config for 2 DUT ports, with DUT port 1 eBGP session to ATE port 1

  * Generate config for ATE 2 ports, with ATE port 1 eBGP session to DUT port 1

  * Configure ATE port 1 to advertise ipv4 and ipv6 prefixes using the following as paths
    * prefix-set-1 with as path `[100, 200, 300]`
    * prefix-set-2 with as path `[100, 400, 300]`
    * prefix-set-3 with as path `[110]`
    * prefix-set-4 with as path `[400]`
    * prefix-set-5 with as path `[100, 300, 200]`
    * prefix-set-6 with as path `[1, 100, 200, 300, 400]`

  * Establish eBGP sessions between ATE port-1 and DUT port-1
  * Generate traffic from ATE port-2 to all prefixes
  * Validate that traffic is received on ATE port-1 for all installed prefixes

* RT-7.3.2 - Configure as-path-sets
  * Configure DUT with the following routing policies
    * Create the following /routing-policy/defined-sets/bgp-defined-sets/as-path-sets/as-path-set/
      * Create as-path-set-name = "my_3_aspaths" with members
        * `{ as-path-set-member = [ "100", "200", "300" ] }`
      * Create an as-path-set-name 'my_regex_aspath-1' with members
        * `{ as-path-set-member = [ "^100", "20[0-9]", "200$" ] }`
      * Create an as-path-set-name "my_regex_aspath-2" as follows
        * `{ as-path-set-member = [ "(^100)(.*)+(300$)" ] }`

    * Create /routing-policy/policy-definitions/policy-definition named 'match_any_aspaths' with the following statements
      * statement[name='accept_any_my_3_aspaths']/
        * conditions/bgp-conditions/match-community-set/config/community-set = 'my_3_aspaths'
        * conditions/bgp-conditions/match-community-set/config/match-set-options = ANY
        * actions/config/policy-result = ACCEPT_ROUTE

    * Create /routing-policy/policy-definitions/policy-definition named 'match_not_my_3_aspaths' with the following statements
      * statement[name='accept_not_my_3_aspaths']/
        * conditions/bgp-conditions/match-community-set/config/community-set = 'my_3_aspaths'
        * conditions/bgp-conditions/match-community-set/config/match-set-options = INVERT
        * actions/config/policy-result = ACCEPT_ROUTE

    * Create /routing-policy/policy-definitions/policy-definition named 'match_my_regex_aspath-1' with the following statements
      * statement[name='accept_my_regex_aspath-1']/
        * conditions/bgp-conditions/match-community-set/config/community-set = 'my_regex_aspath-1'
        * conditions/bgp-conditions/match-community-set/config/match-set-options = ANY
        * actions/config/policy-result = ACCEPT_ROUTE

    * Create /routing-policy/policy-definitions/policy-definition named 'match_my_regex_aspath-2' with the following statements
      * statement[name='accept_my_regex_aspath-2']/
        * conditions/bgp-conditions/match-community-set/config/community-set = 'my_regex_aspath-2'
        * conditions/bgp-conditions/match-community-set/config/match-set-options = ANY
        * actions/config/policy-result = ACCEPT_ROUTE

* RT-7.3.3 - Replace /routing-policy DUT config 
  * For each DUT policy-definition
    * Replace the configuration for BGP neighbor policy (`.../apply-policy/config/import-policy`) to the currently tested policy
      * Verify prefixes sent, received and installed are as expected
    * Send traffic
      * Verify traffic is forwarded for prefixes with matching policy
      * Verify traffic is not forwarded for prefixes without matching policy

### Expected as-path matches

| prefix-set   | match_any_aspaths | match_not_my_3_aspaths | match_my_regex_aspath-1 | my_regex_aspath-2 |
| ------------ | ----------------- | ---------------------- | ----------------------- | ----------------- |
| prefix-set-1 | accept            | reject                 | accept                  | accept            |
| prefix-set-2 | accept            | reject                 | accept                  | accept            |
| prefix-set-3 | reject            | accept                 | reject                  | reject            |
| prefix-set-4 | reject            | accept                 | reject                  | reject            |
| prefix-set-5 | accept            | reject                 | accept                  | reject            |
| prefix-set-6 | accept            | reject                 | accept                  | reject            |

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

### OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

```yaml
paths:
  ## Config Paths ##
  /routing-policy/policy-definitions/policy-definition/config/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/config/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy:
## State Paths ##
  /interfaces/interface/ethernet/state/mac-address:

rpcs:
  gnmi:
    gNMI.Get:
```
