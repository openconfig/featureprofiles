# RT-7.9: BGP Policy - Import/Export Policy Action Using Extended Communities

## Summary

Configure bgp policy to import/export routes by matching on extended communities.

Matches should be performed against a subset of extended community types

* <2b AS>:<4b value> per RFC4360 section 3.1
* <4b AS>:<2b value> per RFC5668 section 2.
* link-bandwidth:<2 byte asn>:<bandwidth value in bits/sec, optionally with K/M/G suffix>
  per draft-ietf-idr-link-bandwidth-07
* TODO: Additional match types can be added here.  Currently these match types
  cover the use cases needed for RT-7.9

## Testbed type

* https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

* Testbed configuration - Setup BGP sessions and prefixes
  * Generate config for 2 DUT and ATE ports where
    * DUT port 1 to ATE port 1.
    * DUT port 2 to ATE port 2.
  * Configure ATE port 1 with a BGP session to DUT port 1
    * Advertise ipv4 and ipv6 prefixes to DUT port 1 using the following communities:
    * prefix-set-1 with 2 routes with communities `[100:95000, 200000:2, 300000:300000]`
    * prefix-set-2 with 2 routes with communities `[100000:1, 101000:1, 200000:1]`
    * prefix-set-3 with 2 routes with communities `[109000:1]`
    * prefix-set-4 with 2 routes with communities `[400000:1]`
    * prefix-set-5 with 2 routes with communities `[400000:1, link-bandwidth:100:1500000`
    * prefix-set-6 with 2 routes with communities `[400000:1, link-bandwidth:100:1M`

## Subtests

* RT-7.9.1 - Validate bgp sessions and traffic
  * For IPv4 and IPv6 prefixes:
    * Observe received prefixes at ATE port-2.
  * Generate traffic from ATE port-2 to ATE port-1.
  * Validate that traffic can be received on ATE port-1 for all installed
    routes.

* RT-7.9.2 - Validate ext-community-sets and routing-policy using OC
  release 2.x or earlier
  * Configure the following community sets
    (prefix: `routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set`)
    on the DUT.
    * Create a community-set named 'any_my_3_ext_comms' with members as follows:
      * community-member = [ "100:95000", "200000:2", "300000:300000" ]
      * match-set-options = ANY
    * Create a community-set named 'all_3_ext_comms' with members as follows:
      * community-member = [ "100:95000", "200000:2", "300000:300000" ]
      * match-set-options = ALL
    * Create a community-set named 'no_3_ext_comms' with members as follows:
      * community-member = [ "100000:99", "200000:2", "300000:300000" ]
      * match-set-options = INVERT
    * Create a community-set named 'any_my_regex_ext_comms' with members as follows:
      * community-member = [ "10[0-9]000:1" ]
      * match-set-options = ANY
    * Create a community-set named 'any_ext_comms' with members as follows:
      * community-member = [ "^.*$" ]
      * match-set-options = ANY

  * Create a `/routing-policy/policy-definitions/policy-definition/policy-definition`
    named 'any_3_comms' with the following `statements`
    * statement[name='any_3_comms']/
      * conditions/bgp-conditions/match-ext-community-set/config/ext-community-set = 'any_my_3_ext_comms'
      * actions/config/policy-result = ACCEPT_ROUTE
    * statement[name='reject_all']/
      * conditions/bgp-conditions/match-ext-community-set/config/ext-community-set = 'any_ext_comms'
      * actions/config/policy-result = REJECT_ROUTE

  * Create a `/routing-policy/policy-definitions/policy-definition/policy-definition`
    named 'all_3_comms' with the following `statements`
    * statement[name='all_3_comms']/
      * conditions/bgp-conditions/match-ext-community-set/config/ext-community-set = 'all_3_ext_comms'
      * actions/config/policy-result = ACCEPT_ROUTE
    * statement[name='reject_all']/
      * conditions/bgp-conditions/match-ext-community-set/config/ext-community-set = 'any_ext_comms'
      * actions/config/policy-result = REJECT_ROUTE

  * Create a `/routing-policy/policy-definitions/policy-definition/policy-definition`
    named 'no_3_comms' with the following `statements`
    * statement[name='no_3_comms']/
      * conditions/bgp-conditions/match-ext-community-set/config/ext-community-set = 'no_3_ext_comms'
      * conditions/bgp-conditions/match-ext-community-set/config/match-set-options = INVERT
      * actions/config/policy-result = ACCEPT_ROUTE
    * statement[name='reject_all']/
      * conditions/bgp-conditions/match-ext-community-set/config/ext-community-set = 'any_ext_comms'
      * conditions/bgp-conditions/match-ext-community-set/config/match-set-options = ANY
      * actions/config/policy-result = REJECT_ROUTE

  * Create a `/routing-policy/policy-definitions/policy-definition/policy-definition`
    named 'any_my_regex_comms' with the following `statements`
    * statement[name='any_my_regex_comms']/
      * conditions/bgp-conditions/match-ext-community-set/config/ext-community-set = 'any_my_regex_ext_comms'
      * conditions/bgp-conditions/match-ext-community-set/config/match-set-options = ANY
      * actions/config/policy-result = ACCEPT_ROUTE
    * statement[name='reject_all']/
      * conditions/bgp-conditions/match-ext-community-set/config/ext-community-set = 'any_ext_comms'
      * conditions/bgp-conditions/match-ext-community-set/config/match-set-options = ANY
      * actions/config/policy-result = REJECT_ROUTE

  * For each policy-definition created
    * Use gnmi Set REPLACE option for:
      * `/routing-policy/policy-definitions` to configure the policy
      * Use `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/import-policy`
        to configure the policy on the DUT to the ATE port 1 bgp neighbor
    * Send traffic from ATE port-2 to all prefix-sets-1,2,3,4.
    * Verify traffic is received on ATE port 1 for accepted prefixes.
    * Verify traffic is not received on ATE port 1 for rejected prefixes.
    * Stop traffic

    * Expected matches for each policy
      |              | any_my_3_comms | all_3_comms | no_3_comms | any_my_regex_comms |
      | ------------ | -------------- | ----------- | ---------- | ------------------ |
      | prefix-set-1 | accept         | accept      | reject     | accept             |
      | prefix-set-2 | accept         | reject      | reject     | accept             |
      | prefix-set-3 | reject         | reject      | accept     | accept             |
      | prefix-set-4 | reject         | reject      | accept     | reject             |

* RT-7.9.3 - Validate ext-community-sets and routing-policy using OC release 3.x
  * Note, this is the same at RT-7.9.2, but with a change in the location of the
    `match-set-options` leaf which moved to
    `/routing-policy/policy-definitions/policy-definition/policy-definition/bgp-conditions/match-ext-community-set/config/match-set-options`
  * Configure the following community sets
    (prefix: `routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set`)
    on the DUT.
    * Create a community-set named 'any_my_3_ext_comms' with members as follows:
      * community-member = [ "100:95000", "200000:2", "300000:300000" ]
    * Create a community-set named 'all_3_ext_comms' with members as follows:
      * community-member = [ "100:95000", "200000:2", "300000:300000" ]
    * Create a community-set named 'no_3_ext_comms' with members as follows:
      * community-member = [ "100000:99", "200000:2", "300000:300000" ]
    * Create a community-set named 'any_my_regex_ext_comms' with members as follows:
      * community-member = [ "10[0-9]000:1" ]
    * Create a community-set named 'any_ext_comms' with members as follows:
      * community-member = [ "^.*$" ]

  * Create a `/routing-policy/policy-definitions/policy-definition/policy-definition`
    named 'any_3_comms' with the following `statements`
    * statement[name='any_3_comms']/
      * conditions/bgp-conditions/match-ext-community-set/config/ext-community-set = 'any_my_3_ext_comms'
      * conditions/bgp-conditions/match-ext-community-set/config/match-set-options = ANY
      * actions/config/policy-result = ACCEPT_ROUTE
    * statement[name='reject_all']/
      * conditions/bgp-conditions/match-ext-community-set/config/ext-community-set = 'any_ext_comms'
      * conditions/bgp-conditions/match-ext-community-set/config/match-set-options = ANY
      * actions/config/policy-result = REJECT_ROUTE

  * Create a `/routing-policy/policy-definitions/policy-definition/policy-definition`
    named 'all_3_comms' with the following `statements`
    * statement[name='all_3_comms']/
      * conditions/bgp-conditions/match-ext-community-set/config/ext-community-set = 'all_3_ext_comms'
      * conditions/bgp-conditions/match-ext-community-set/config/match-set-options = ALL
      * actions/config/policy-result = ACCEPT_ROUTE
    * statement[name='reject_all']/
      * conditions/bgp-conditions/match-ext-community-set/config/ext-community-set = 'any_ext_comms'
      * conditions/bgp-conditions/match-ext-community-set/config/match-set-options = ANY
      * actions/config/policy-result = REJECT_ROUTE

  * Create a `/routing-policy/policy-definitions/policy-definition/policy-definition`
    named 'no_3_comms' with the following `statements`
    * statement[name='no_3_comms']/
      * conditions/bgp-conditions/match-ext-community-set/config/ext-community-set = 'no_3_ext_comms'
      * conditions/bgp-conditions/match-ext-community-set/config/match-set-options = INVERT
      * actions/config/policy-result = ACCEPT_ROUTE
    * statement[name='reject_all']/
      * conditions/bgp-conditions/match-ext-community-set/config/ext-community-set = 'any_ext_comms'
      * conditions/bgp-conditions/match-ext-community-set/config/match-set-options = ANY
      * actions/config/policy-result = REJECT_ROUTE

  * Create a `/routing-policy/policy-definitions/policy-definition/policy-definition`
    named 'any_my_regex_comms' with the following `statements`
    * statement[name='any_my_regex_comms']/
      * conditions/bgp-conditions/match-ext-community-set/config/ext-community-set = 'any_my_regex_ext_comms'
      * conditions/bgp-conditions/match-ext-community-set/config/match-set-options = ANY
      * actions/config/policy-result = ACCEPT_ROUTE
    * statement[name='reject_all']/
      * conditions/bgp-conditions/match-ext-community-set/config/ext-community-set = 'any_ext_comms'
      * conditions/bgp-conditions/match-ext-community-set/config/match-set-options = ANY
      * actions/config/policy-result = REJECT_ROUTE

  * For each policy-definition
    * Use gnmi Set REPLACE option for:
      * `/routing-policy/policy-definitions` to configure the policy
      * Use `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/import-policy`
        to configure the policy on the DUT to the ATE port 1 bgp neighbor
    * Send traffic from ATE port-2 to all prefix-sets-1,2,3,4.
    * Verify traffic is received on ATE port 1 for accepted prefixes.
    * Verify traffic is not received on ATE port 1 for rejected prefixes.
    * Stop traffic

    * Expected matches for each policy
      |              | any_my_3_comms | all_3_comms | no_3_comms | any_my_regex_comms |
      | ------------ | -------------- | ----------- | ---------- | ------------------ |
      | prefix-set-1 | accept         | accept      | reject     | accept             |
      | prefix-set-2 | accept         | reject      | reject     | accept             |
      | prefix-set-3 | reject         | reject      | accept     | accept             |
      | prefix-set-4 | reject         | reject      | accept     | reject             |

* RT-7.9.4 - Validate link-bandwidth ext-community-sets and matching policy
  using OC model revision 2.x
  * Add prefixes with link-bandwidth community to ATE port 1 to advertise ipv4
    and ipv6 prefixes to DUT port 1 using the following communities:
    * prefix-set-5 with 2 routes with communities `[ link-bandwidth:500000:0, ]`
    * prefix-set-6 with 2 routes with communities `[ link-bandwidth:600000:1M, ]`

  * Configure the following community sets
    (prefix: `routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set`)
    on the DUT.
    * Create an ext-community-set named 'linkbw_ext_comms_0' with:
      * ext-community-member = [ "^link-bandwidth:.*:0$" ]
      * match-set-options = ANY
    * Create an ext-community-set named 'linkbw_ext_comms_1M' with members as follows:
      * ext-community-member = [ "^link-bandwidth:.*:1M$" ]
      * config/match-set-options = ANY
    * Create an ext-community-set named 'linkbw_ext_comms_2G' with members as follows:
      * ext-community-member = [ "^link-bandwidth:.*:2G$" ]
      * match-set-options = ANY
  * Create a `/routing-policy/policy-definitions/policy-definition/policy-definition`
    named 'zero-bandwidth-reject' with the following `statements`
    * statement[name='zero-bandwidth-reject']/
      * conditions/bgp-conditions/match-ext-community-set/config/community-set = 'linkbw_ext_comms_0'
      * actions/config/policy-result = REJECT_ROUTE
    * statement[name='accept_all']/
      * actions/config/policy-result = ACCEPT_ROUTE
  * Create a `/routing-policy/policy-definitions/policy-definition/policy-definition`
    named '1-megabit-match' with the following `statements`
    * statement[name='1-megabit-match']/
      * conditions/bgp-conditions/match-ext-community-set/config/community-set = 'linkbw_ext_comms_1M'
      * actions/config/policy-result = ACCEPT_ROUTE
    * statement[name='reject_all']/
      * actions/config/policy-result = REJECT_ROUTE

  * Create a `/routing-policy/policy-definitions/policy-definition/policy-definition`
    named 'link-bandwidth-match' with the following `statements`
    * statement[name='2-gigabit-match']/
      * conditions/bgp-conditions/match-ext-community-set/config/community-set = 'linkbw_ext_comms_2G'
      * actions/config/policy-result = ACCEPT_ROUTE
    * statement[name='reject_all']/
      * actions/config/policy-result = REJECT_ROUTE

  * For each policy-definition
    * Use gnmi Set REPLACE option for:
      * `/routing-policy/policy-definitions` to configure the policy
      * Use `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/import-policy`
        to configure the policy on the DUT to the ATE port 1 bgp neighbor
    * Send traffic from ATE port-2 to all prefix-sets-5,6.
    * Verify traffic is received on ATE port 1 for accepted prefixes.
    * Verify traffic is not received on ATE port 1 for rejected prefixes.
    * Stop traffic

    * Expected matches for each policy
      |              | zero-bandwidth-reject | 1-megabit | 2-gigabit-match |
      | ------------ | --------------------- | --------- | --------------- |
      | prefix-set-5 | reject                | reject    | reject          |
      | prefix-set-6 | accept                | accept    | reject          |

* RT-7.9.5 - Validate ext-community-sets and matching policy using OC
  release 3.x
  * Note, this is the same at RT-7.9.4, but with a change in the location of the
    `match-set-options` leaf which moved to
    `/routing-policy/policy-definitions/policy-definition/policy-definition/bgp-conditions/match-ext-community-set/config/match-set-options`
  * Add prefixes with link-bandwidth community to ATE port 1 to advertise ipv4
    and ipv6 prefixes to DUT port 1 using the following communities:
    * prefix-set-5 with 2 routes with communities `[ link-bandwidth:500000:0, ]`
    * prefix-set-6 with 2 routes with communities `[ link-bandwidth:600000:1M, ]`

  * Configure the following community sets
    (prefix: `routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set`)
    on the DUT.
    * Create a community-set named 'linkbw_ext_comms_0' with members as follows:
      * `community-member = [ "^link-bandwidth:.*:0$" ]`
    * Create a community-set named 'linkbw_ext_comms_1M' with members as follows:
      * `community-member = [ "^link-bandwidth:.*:1M$" ]`
    * Create a community-set named 'linkbw_ext_comms_2G' with members as follows:
      * `community-member = [ "^link-bandwidth:.*:2G$" ]`

  * Create a `/routing-policy/policy-definitions/policy-definition/policy-definition`
    named 'zero-bandwidth-reject' with the following `statements`
    * statement[name='zero-bandwidth-reject']/
      * conditions/bgp-conditions/match-ext-community-set/config/community-set = 'linkbw_ext_comms_0'
      * conditions/bgp-conditions/match-ext-community-set/config/match-set-options = ANY
      * actions/config/policy-result = REJECT_ROUTE
    * statement[name='accept_all']/
      * actions/config/policy-result = ACCEPT_ROUTE
  * Create a `/routing-policy/policy-definitions/policy-definition/policy-definition`
    named '1-megabit-match' with the following `statements`
    * statement[name='1-megabit-match']/
      * conditions/bgp-conditions/match-ext-community-set/config/community-set = 'linkbw_ext_comms_1M'
      * conditions/bgp-conditions/match-ext-community-set/config/match-set-options = ANY
      * actions/config/policy-result = ACCEPT_ROUTE
    * statement[name='reject_all']/
      * actions/config/policy-result = REJECT_ROUTE

  * Create a `/routing-policy/policy-definitions/policy-definition/policy-definition`
    named 'link-bandwidth-match' with the following `statements`
    * statement[name='2-gigabit-match']/
      * conditions/bgp-conditions/match-ext-community-set/config/community-set = 'linkbw_ext_comms_2G'
      * conditions/bgp-conditions/match-ext-community-set/config/match-set-options = ANY
      * actions/config/policy-result = ACCEPT_ROUTE
    * statement[name='reject_all']/
      * actions/config/policy-result = REJECT_ROUTE

  * For each policy-definition
    * Use gnmi Set REPLACE option for:
      * `/routing-policy/policy-definitions` to configure the policy
      * Use `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/import-policy`
        to configure the policy on the DUT to the ATE port 1 bgp neighbor
    * Send traffic from ATE port-2 to all prefix-sets-5,6.
    * Verify traffic is received on ATE port 1 for accepted prefixes.
    * Verify traffic is not received on ATE port 1 for rejected prefixes.
    * Stop traffic

    * Expected matches for each policy
      |              | zero-bandwidth-reject | 1-megabit | 2-gigabit-match |
      | ------------ | --------------------- | --------- | --------------- |
      | prefix-set-5 | reject                | reject    | reject          |
      | prefix-set-6 | accept                | accept    | reject          |

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
