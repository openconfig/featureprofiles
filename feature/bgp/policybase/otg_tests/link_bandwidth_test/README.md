# RT-7.5: BGP Policy - Set Link Bandwidth Community

## Summary

Configure bgp policy to add statically configured BGP link bandwidth
communities to routes based on a prefix match.

## Testbed type

* https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

## Procedure

* Testbed configuration - Setup BGP sessions and prefixes.
  * Generate config for 2 DUT and ATE ports where:
    * DUT port 1 to ATE port 1.
    * DUT port 2 to ATE port 2.
  * Configure ATE port 1 with a BGP session to DUT port 1.
    * Advertise ipv4 and ipv6 prefixes to DUT port 1 using the following communities:
      * prefix-set-1 with 2 ipv4 and 2 ipv6 routes without communities.
      * prefix-set-2 with 2 ipv4 and 2 ipv6 routes with communities `[ "100:100" ]`.
      * prefix-set-3 with 2 ipv4 and 2 ipv6 routes with extended communities `[ "link-bandwidth:100:0" ]`.

* RT-7.5.1 - Validate bgp sessions and traffic
  * For IPv4 and IPv6 prefixes:
    * Observe received prefixes at ATE port-2.
  * Generate traffic from ATE port-2 to ATE port-1.
  * Validate that traffic can be received on ATE port-1 for all installed
    routes.

* RT-7.5.2 - Validate adding link-bandwidth ext-community-sets using OC model release 2.x
  * Configure the following extended community sets on the DUT:
    (prefix: `routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set`)
    * Create an ext-community-set named 'linkbw_0' with:
      * ext-community-member = [ "link-bandwidth:100:0" ]
    * Create an ext-community-set named 'linkbw_1M' with members as follows:
      * ext-community-member = [ "link-bandwidth:100:1M" ]
    * Create an ext-community-set named 'linkbw_2G' with members as follows:
      * ext-community-member = [ "link-bandwidth:100:2G" ]
    * Create an community-set named 'regex_match_as100' with members as follows:
      * ext-community-member = [ "^100:.*$" ]
    * Create an community-set named 'regex_nomatch_as100' with members as follows:
      * ext-community-member = [ "^100:.*$" ]
      * match-set-options = INVERT
    * Create an ext-community-set named 'linkbw_any' with members as follows:
      * ext-community-member = [ "^link-bandwidth:.*:.*$" ]

  * Create a `/routing-policy/policy-definitions/policy-definition/policy-definition`
    named 'set_linkbw_0' with the following `statements`
    * statement[name='zero_linkbw']/
      * actions/bgp-actions/set-ext-community/reference/config/ext-community-set-refs =
          /routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set[name='linkbw_0']
      * actions/bgp-actions/set-ext-community/config/options = ADD
      * actions/bgp-actions/set-community/config/method = REFERENCE
      * actions/config/policy-result = NEXT_STATEMENT
    * statement[name='accept_all_routes']/
      * actions/config/policy-result = ACCEPT_ROUTE

  * Create a `/routing-policy/policy-definitions/policy-definition/policy-definition`
    named 'match_100_set_linkbw_1M' with the following `statements`
    * statement[name='1-megabit-match']/
      * conditions/bgp-conditions/match-ext-community-set/config/community-set = 'regex_match_as100'
      * actions/bgp-actions/set-ext-community/reference/config/ext-community-set-refs =
          /routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set[name='linkbw_1M']
    * statement[name='accept_all_routes']/
      * actions/config/policy-result = ACCEPT_ROUTE

  * Create a `/routing-policy/policy-definitions/policy-definition/policy-definition`
    named 'nomatch_100_set_linkbw_2G' with the following `statements`
    * statement[name='2-gigabit-match']/
      * conditions/bgp-conditions/match-ext-community-set/config/community-set = 'regex_nomatch_as100'
      * actions/bgp-actions/set-ext-community/reference/config/ext-community-set-refs =
          /routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set[name='linkbw_2G']
    * statement[name='accept_all_routes']/
      * actions/config/policy-result = ACCEPT_ROUTE

  * For each policy-definition created, run a subtest (RT-7.8.3.x-<policy_name_here>) to
    * Use gnmi Set REPLACE option for:
      * `/routing-policy/policy-definitions` to configure the policy
      * Use `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy`
        to apply the policy on the DUT bgp neighbor to the ATE port 1.
    * Verify expected communities are present in ATE.
    * Verify expected communities are present in DUT state.
      * Do not fail test if this path is not supported, only log results
      * `/network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/ext-community-index`
      * `/network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/ext-community-index`

    * Expected matches for each policy
      |              | zero_linkbw              | match_100_set_linkbw_1M        | nomatch_100_set_linkbw_2G         |
      | ------------ | ------------------------ | ------------------------------ | --------------------------------- |
      | prefix-set-5 | [ link-bandwidth:100:0 ] | none                           | [ link-bandwidth:100:2000000000 ] |
      | prefix-set-6 | [ link-bandwidth:100:0 ] | [ link-bandwidth:100:1000000 ] | none                              |

* RT-7.5.3 - Validate adding link-bandwidth ext-community-sets using OC model release 3.x
  * Note, this is the same at RT-7.8.6, but with a change in the location of the
    `match-set-options` leaf which moved to
    `/routing-policy/policy-definitions/policy-definition/policy-definition/bgp-conditions/match-ext-community-set/config/match-set-options`

## Config Parameter Coverage

### Policy for community-set configuration

* /routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set/config/ext-community-set-name
* /routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set/config/community-member

### Policy action configuration

* /routing-policy/policy-definitions/policy-definition/config/name
* /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
* /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-ext-community/reference/config/ext-community-set-refs

### Policy for community-set match configuration

* /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/config/community-set
* /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/match-ext-community-set/config/match-set-options

### Policy attachment point configuration

* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy
* /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/import-policy
* /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/export-policy

## Telemetry Parameter Coverage

* /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/ext-community-index
* /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/ext-community-index

## Minimum DUT Required

vRX - Virtual Router Device
