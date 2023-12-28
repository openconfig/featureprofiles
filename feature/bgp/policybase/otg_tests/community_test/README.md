# RT-2.3: BGP Policy Community Set

## Summary

BGP policy configuration for AS Paths and Community Sets

## Subtests

* RT-2.3.1 - Setup BGP sessions
  * Generate config for 2 DUT ports, with DUT port 1 eBGP session to ATE port 1.
  * Generate config for ATE 2 ports, with ATE port 1 eBGP session to DUT port 1.
  * Configure ATE port 1 to advertise ipv4 and ipv6 prefixes to DUT port 1 using the following communities:
    * prefix-set-1 with 2 routes with communities `[100:1, 200:2, 300:3]`
    * prefix-set-2 with 2 routes with communities `[100:1, 101:1, 200:1]`
    * prefix-set-3 with 2 routes with communities `[109:1]`
    * prefix-set-4 with 2 routes with communities `[400:1]`

  * For IPv4 and IPv6 prefixes:
    * Observe received prefixes at ATE port-2.
  * Generate traffic from ATE port-2 to ATE port-1.
  * Validate that traffic can be received on ATE port-1 for all installed
        routes.

* RT-2.3.2 - Validate community-set
  * Configure DUT with a policy-definition for each of the following community-sets.
    * Create a community-set named `any_my_3_comms` with members and match options as follows:
      * `{ community-member = [ "100:1", "200:2", "300:3" ] }`
      * `{ match-set-options=ANY }`
      * with `{ policy-result=ACCEPT_ROUTE }`
    * Create a community-set named `all_3_comms` with members and match options as follows:
      * `{ community-member = [ "100:1", "200:2", "300:3" ] }`
      * `{ match-set-options=ALL }`
      * with `{ policy-result=ACCEPT_ROUTE }`
    * Create a community-set named `no_3_comms` with members and match options as follows:
      * `{ community-member = [ "100:1", "200:2", "300:3" ]}`
      * `{ match-set-options=INVERT }`
      * with `{ policy-result=ACCEPT_ROUTE }`
    * Create a community-set named `any_my_regex_comms` with members and match options as follows:
      * `{ community-member = [ "10[0-9]:1" ] }`
      * `{ match-set-options=ANY }`
      * with `{ policy-result=ACCEPT_ROUTE }`

  * For each DUT policy configuration:
    * Update the configuration for BGP neighbor import policy (`.../apply-policy/config/import-policy`) to the selected community set.
      * Verify prefixes sent, received and installed are as expected.
    * Send traffic from ATE port-2 to all prefix-sets.
      * Verify traffic is forwarded for accepted prefixes.
      * Verify traffic is not forwarded for rejected prefixes.

### Expected community matches

| prefix-set   | any_my_3_comms | all_3_comms | no_3_comms | any_my_regex_comms |
| ------------ | -------------- | ----------- | ---------- | ------------------ |
| prefix-set-1 | accept         | accept      | reject     | accept             |
| prefix-set-2 | accept         | reject      | reject     | accept             |
| prefix-set-3 | reject         | reject      | accept     | accept             |
| prefix-set-4 | reject         | reject      | accept     | reject             |

* TODO: add coverage for link-bandwidth community in separate test.

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

## Telemetry Parameter Coverage

### Policy definition state

* /routing-policy/policy-definitions/policy-definition/state/name
* /routing-policy/policy-definitions/policy-definition/statements/statement/state/name

### Policy for community-set match state

* /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-set-name
* /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-member
* /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/match-set-options
* /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/state/community-set

### Paths to verify policy state

* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/export-policy
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy

### Paths to verify prefixes sent and received

* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/sent
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received-pre-policy
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/installed
