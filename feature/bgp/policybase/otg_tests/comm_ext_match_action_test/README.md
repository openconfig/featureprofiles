# RT-7.9: BGP Policy Match and Set Extended Communities Import/Export Policy

## Summary

Configure bgp policy to add communities to routes by matching on the following
criteria.

* RT-7.9.1 Validate test environment
* RT-7.9.2 Validate policy to set standard community for all routes using OC release 3.x
* RT-7.9.3 Validate policy to set ext-community for various match criteria using OC release 3.x

## Testbed type

* https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

## Procedure

* Testbed configuration - Setup BGP sessions and prefixes.
  * Generate config for 2 DUT and ATE ports where:
    * DUT port 1 to ATE port 1.
    * DUT port 2 to ATE port 2.
  * Configure ATE port 1 with an external type BGP session to DUT port 1.
    * Advertise ipv4 and ipv6 prefixes to DUT port 1 using the following communities:
      * prefix-set-1 with 2 ipv6 and 2 ipv4 routes without communities.
      * prefix-set-2 with 2 ipv6 and 2 ipv4 routes with communities `[5:5, 6:6 ]`.
      * prefix-set-3 with 2 ipv6 and 2 ipv4 routes with extended communities `[50:500000, 60:600000]`.

* RT-7.9.1 - Validate test environment
  * For IPv4 and IPv6 prefixes:
    * Observe received prefixes at ATE port-2.
  * Send traffic from ATE port-2 to all prefix-sets-1,2,3,4.
    * Verify traffic is received on ATE port 1 for accepted prefixes.
    * Verify traffic is not received on ATE port 1 for rejected prefixes.
    * Stop traffic

* RT-7.9.2 - Create policy to set standard community for all routes using OC release 3.x
  * Configure the following community sets on the DUT.
    (prefix: `/routing-policy/defined-sets/bgp-defined-sets/`)
    * Create a `community-sets/community-set` named 'match_std_comms' with members as follows:
      * community-member = [ "5:5" ]
    * Create a `community-sets/community-set` named 'add_std_comms' with members as follows:
      * community-member = [ "10:10", "20:20", "30:30" ]

  * Create `/routing-policy/policy-definitions/policy-definition/policy-definition[name='add_std_comms']/`
    with the following `statements/`
    * statement[name='add_std_comms']/
      * actions/bgp-actions/set-community/reference/config/community-set-refs =
          /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set[name='add_std_comms']
      * actions/bgp-actions/set-community/config/options = ADD
      * actions/bgp-actions/set-community/config/method = REFERENCE
      * actions/config/policy-result = NEXT_STATEMENT
    * statement[name='accept_all_routes']/
      * actions/config/policy-result = ACCEPT_ROUTE

  * Create a `/routing-policy/policy-definitions/policy-definition/policy-definition[name='match_and_add_comms'/`
    with the following `statements/`
    * statement[name='match_and_add_std_comms']/
      * conditions/bgp-conditions/match-community-set/config/community-set =
        /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set[name='match_std_comms']
      * conditions/bgp-conditions/match-ext-community-set/config/match-set-options = ANY
      * actions/bgp-actions/set-community/reference/config/community-set-refs =
          /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set[name='add_std_comms']
      * actions/bgp-actions/set-community/config/options = ADD
      * actions/bgp-actions/set-community/config/method = REFERENCE
      * actions/config/policy-result = NEXT_STATEMENT
    * statement[name='accept_all_routes']/
      * actions/config/policy-result = ACCEPT_ROUTE

  * For each policy-definition created, run a subtest (RT-7.9.2.x-neighbor-<policy_name_here>) to
    * Use gnmi Set REPLACE option for:
      * `/routing-policy/policy-definitions` to configure the policy
      * Use `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy`
        to apply the policy on the DUT bgp neighbor to the ATE port 1.
    * Verify routes are received on ATE port 1 for all prefixes (since all routes are accepted by policies).
    * Verify expected communities are present in ATE.
    * Verify expected communities are present in DUT state.
      * Do not fail test if this path is not supported, only log results
      * `/network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/ext-community-index`
      * `/network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/ext-community-index`

  * For each policy-definition created, run a sub-test (RT-7.9.2.x-peer-group-<policy_name_here>) to
    * Use gnmi Set REPLACE option for:
      * `/routing-policy/policy-definitions` to configure the policy
      * Use `/network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/import-policy`
        to apply the policy on the DUT bgp neighbor to the ATE port 1.
    * Verify expected communities are present in ATE.
    * Verify expected communities are present in DUT state.
      * Do not fail test if this path is not supported, only log results
      * `/network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/ext-community-index`
      * `/network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/ext-community-index`

    * Expected communities
      |              | add_std_comms                                 | match_and_add_std_comms           |
      | ------------ | --------------------------------------------- | --------------------------------- |
      | prefix-set-1 | [ 10:10, 20:20, 30:30 ]                       | none                              |
      | prefix-set-2 | [ 10:10, 20:20, 30:30, 5:5, 6:6 ]             | [ 10:10, 20:20, 30:30, 5:5, 6:6 ] |
      | prefix-set-3 | [ 10:10, 20:20, 30:30, 50:500000, 60:600000 ] | [ 50:500000, 60:600000 ]          |

* RT-7.9.3 - Create policy to set ext-community for various match criteria using OC release 3.x
  * Note, this particular subtest only covers  <2b AS>:<4b value> per RFC4360 section 3.1
  * Configure the following community sets on the DUT.
    (prefix: `/routing-policy/defined-sets/bgp-defined-sets/`)
    * Create  `ext-community-sets/ext-community-set['match_ext_comms]` with members as follows:
      * community-member = [ "50:500000" ]
    * Create `ext-community-sets/ext-community-set[name='add_ext_comms']` with members as follows:
      * community-member = [ "1:100000", "2:200000" ]

  * Create a `/routing-policy/policy-definitions/policy-definition/policy-definition/`
    named 'add_ext_comms' with the following `statements/`
    * statement[name='add_ext_comms']/
      * actions/bgp-actions/set-ext-community/reference/config/ext-community-set-refs =
          /routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set[name='add_ext_comms']
      * actions/bgp-actions/set-ext-community/config/options = ADD
      * actions/config/policy-result = NEXT_STATEMENT
    * statement[name='accept_all_routes']/
      * actions/config/policy-result = ACCEPT_ROUTE

  * Create a `/routing-policy/policy-definitions/policy-definition/policy-definition/`
    named 'match_and_add_ext_comms' with the following `statements/`
    * statement[name='match_and_add_ext_comms']/
      * conditions/bgp-conditions/match-ext-community-set/config/ext-community-set =
          /routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set[name='match_ext_comms']
      * conditions/bgp-conditions/match-ext-community-set/config/match-set-options = ANY
      * actions/bgp-actions/set-ext-community/reference/config/ext-community-set-refs =
          /routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set[name='add_ext_comms']
      * actions/bgp-actions/set-ext-community/config/options = ADD
      * actions/config/policy-result = NEXT_STATEMENT
    * statement[name='accept_all_routes']/
      * actions/config/policy-result = ACCEPT_ROUTE

  * For each policy-definition created, run a subtest (RT-7.9.3.x-<policy_name_here>) to
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
      |              | add_ext_comms                                | match_and_add_ext_comms                      |
      | ------------ | -------------------------------------------- | -------------------------------------------- |
      | prefix-set-1 | [ 1:100000, 2:200000 ]                       | none                                         |
      | prefix-set-2 | [ 1:100000, 2:200000, 5:5, 6:6 ]             | [ 5:5, 6:6 ]                                 |
      | prefix-set-3 | [ 1:100000, 2:200000, 50:500000, 60:600000 ] | [ 1:100000, 2:200000, 50:500000, 60:600000 ] |

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
