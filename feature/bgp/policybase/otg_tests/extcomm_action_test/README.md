# RT-7.9: BGP Policy - Import/Export Policy Action Using Extended Communities

## Summary

Configure bgp policy to import/export routes by matching on extended communities.

Matches should be performed against a subset of extended community types

* <2b AS>:<4b value> per RFC4360 section 3.1
* <4b AS>:<2b value> per RFC5668 section 2.
* link-bandwidth:<2 byte asn>:<bandwidth value in bits/sec, optionally with K/M/G suffix> per draft-ietf-idr-link-bandwidth-07

## Subtests

* RT-7.9.1 - Setup BGP sessions
  * Generate config for 2 DUT ports, with DUT port 1 eBGP session to ATE port 1.
  * Generate config for ATE 2 ports, with ATE port 1 eBGP session to DUT port 1.
  * Configure ATE port 1 to advertise ipv4 and ipv6 prefixes to DUT port 1 using the following communities:
    * prefix-set-1 with 2 routes with communities `[100:95000, 200000:2, 300000:300000]`
    * prefix-set-2 with 2 routes with communities `[100000:1, 101000:1, 200000:1]`
    * prefix-set-3 with 2 routes with communities `[109000:1]`
    * prefix-set-4 with 2 routes with communities `[400000:1]`
    * prefix-set-5 with 2 routes with communities `[400000:1, link-bandwidth:100:1500000`
    * prefix-set-6 with 2 routes with communities `[400000:1, link-bandwidth:100:1M`

  * For IPv4 and IPv6 prefixes:
    * Observe received prefixes at ATE port-2.
  * Generate traffic from ATE port-2 to ATE port-1.
  * Validate that traffic can be received on ATE port-1 for all installed
        routes.

* RT-7.9.2 - Validate creating ext-community-sets
  * Configure the following community sets
    (prefix: `routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set`)
    on the DUT.
    * Create a community-set named `any_my_3_ext_comms` with members as follows:
      * `{ community-member = [ "100:95000", "200000:2", "300000:300000" ] }`
    * Create a community-set named `all_3_ext_comms` with members as follows:
      * `{ community-member = [ "100:95000", "200000:2", "300000:300000" ] }`
    * Create a community-set named `no_3_ext_comms` with members as follows:
      * `{ community-member = [ "100:1", "200:2", "300:3" ]}`
    * Create a community-set named `any_my_regex_ext_comms` with members as follows:
      * `{ community-member = [ "10[0-9]:1" ] }`
    * Create a community-set named `linkbw_ext_comms_15` with members as follows:
      * `{ community-member = [ "^link-bandwidth:400000:1500000$" ] }`
    * Create a community-set named `linkbw_ext_comms_1M` with members as follows:
      * `{ community-member = [ "^link-bandwidth:400000:1M$" ] }`
    * Create a community-set named `linkbw_ext_comms_2M` with members as follows:
      * `{ community-member = [ "^link-bandwidth:400000:2M$" ] }`

  * Create a `/routing-policy/policy-definitions/policy-definition/policy-definition`
    named 'ext-community-match' with the following `statements`
    * statement[name='accept_any_3_comms']/
      * conditions/bgp-conditions/match-ext-community-set/config/community-set = 'any_my_3_comms'
      * conditions/bgp-conditions/match-ext-community-set/config/match-set-options = ANY
      * actions/config/policy-result = ACCEPT_ROUTE
    * statement[name='accept_all_3_comms']/
      * conditions/bgp-conditions/match-ext-community-set/config/as-path-set = 'all_3_comms'
      * conditions/bgp-conditions/match-ext-community-set/config/match-set-options = ALL
      * actions/config/policy-result = ACCEPT_ROUTE
    * statement[name='accept_no_3_comms']/
      * conditions/bgp-conditions/match-ext-community-set/config/as-path-set = 'no_3_comms'
      * conditions/bgp-conditions/match-ext-community-set/config/match-set-options = INVERT
      * actions/config/policy-result = ACCEPT_ROUTE
    * statement[name='accept_any_my_regex_comms']/
      * conditions/bgp-conditions/match-ext-community-set/config/as-path-set = 'all_3_comms'
      * conditions/bgp-conditions/match-ext-community-set/config/match-set-options = ANY
      * actions/config/policy-result = ACCEPT_ROUTE

  * Send traffic from ATE port-2 to all prefix-sets.
    * Verify traffic is received on ATE port 1 for accepted prefixes.
    * Verify traffic is not received on ATE port 1 for rejected prefixes.

### Expected community matches

| prefix-set   | any_my_3_comms | all_3_comms | no_3_comms | any_my_regex_comms |
| ------------ | -------------- | ----------- | ---------- | ------------------ |
| prefix-set-1 | accept         | accept      | reject     | accept             |
| prefix-set-2 | accept         | reject      | reject     | accept             |
| prefix-set-3 | reject         | reject      | accept     | accept             |
| prefix-set-4 | reject         | reject      | accept     | reject             |

## Config Parameter Coverage

### Policy definition

* /routing-policy/policy-definitions/policy-definition/config/name
* /routing-policy/policy-definitions/policy-definition/statements/statement/config/name

### Policy for community-set configuration

* /routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set/config/ext-community-set-name
* /routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set/config/community-member

### Policy for community-set match configuration

* /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/config/community-set
* /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/match-ext-community-set/config/match-set-options
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy

## Telemetry Parameter Coverage

### Policy definition state

* /routing-policy/policy-definitions/policy-definition/state/name
* /routing-policy/policy-definitions/policy-definition/statements/statement/state/name

### Policy for community-set match state

* /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-set-name
* /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-member
* /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/match-ext-community-set/state/match-set-options
* /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/state/community-set

### Paths to verify policy state

* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/export-policy
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy

### Paths to verify prefixes sent and received

* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/sent
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received-pre-policy
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/installed
