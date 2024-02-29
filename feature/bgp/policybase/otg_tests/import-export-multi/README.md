# RT-7.11: BGP Policy - Import/Export Policy Action Using Multiple Criteria

## Summary

The purpose of this test is to verify a combination of bgp conditions using matching and policy nesting as well as and actions in a single BGP import policy.  Additional combinations may be added in the future as subtests.

## Testbed type

* [2 port ATE to DUT](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Testbed common configuration

* Testbed configuration - Setup BGP sessions and prefixes
  * Generate config for 2 DUT and ATE ports where
    * DUT port 1 to ATE port 1.
    * DUT port 2 to ATE port 2.
  * Configure ATE port 1 with an external type BGP session to DUT port 1
    * Advertise ipv4 and ipv6 prefixes to DUT port 1 using the following communities:
    * prefix-set-1 with 2 routes with communities `[100:1]`
    * prefix-set-2 with 2 routes with communities `[200:1]`
    * prefix-set-3 with 2 routes with communities `[300:1]`
    * prefix-set-4 with 2 routes with communities `[400:1]`
    * prefix-set-5 with 2 routes with communities `[500:1]`

* Validate bgp sessions and traffic
  * For IPv4 and IPv6 prefixes:
    * Observe received prefixes at ATE port-2.
  * Generate traffic from ATE port-2 to ATE port-1.
  * Validate that traffic can be received on ATE port-1 for all installed
    routes.

* Configure ext-community-sets on DUT using OC
  * Configure the following community sets
    (prefix: `routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set`)
    on the DUT.
    * Create a community-set named 'reject-communities' with members as follows:
      * community-member = [ "100:1" ]
    * Create a community-set named 'accept-communities' with members as follows:
      * community-member = [ "200:1" ]
    * Create a community-set named 'regex-community' with members as follows:
      * community-member = [ "^300:.*$" ]
    * Create a community-set named 'add-communities' with members as follows:
      * community-member = [ "400:1", "400:2" ]
    * Create a community-set named 'my_community' with members as follows:
      * community-member = [ "500:1" ]
    * Create a community-set named 'add_comm_one' with members as follows:
      * community-member = [ "600:1" ]
    * Create a community-set named 'add_comm_two' with members as follows:
      * community-member = [ "700:1" ]

## Subtests

RT-7.11.3 Create a single bgp policy containing the following conditions and actions:

* Reject route matching any communities in a community-set.
* Reject route matching another policy (nested) and not matching a community-set.
* Add a community-set if missing that same community-set.
* Add two communities if matching a community and prefix-set.
* Reject route matching another policy (nested) and matching a community-set.

[ TODO: Clean up formatting for below policies ]

  "policy-definitions/policy-definition/config/name: "core_from_cluster_dual_stacked_v4"
        "statements/statement"
  
          config/name: "reject_route_community"
              actions/config/policy-result: "REJECT_ROUTE"
              conditions/bgp-conditions/match-community-set/config
                  "community-set": "reject-communities"
                  "match-set-options": "ANY"

          config/name: "nested_reject_route_community"
              actions/config/policy-result: "REJECT_ROUTE"
              conditions/config/call-policy: "nested_policy_accept_regex"
              bgp-conditions/match-community-set/config/community-set: "accept-communities"
                      "match-set-options": "INVERT"

          config/name: "add_communities_if_missing"
              actions/config/policy-result: "NEXT_STATEMENT"
                bgp-actions/set-community/config/method: "REFERENCE"
                      "option": "ADD",
                      "community-set-refs": [
                        "add-communities"
              conditions/bgp-conditions/match-community-set/config": {
                      "community-set": "add-communities"
                      "match-set-options": "INVERT"

          config/name: "add_2_community_sets"
              actions/config/policy-result: "NEXT_STATEMENT"
              bgp-actions/set-community/config: 
                      method: "REFERENCE",
                      option: "ADD",
                      community-set-refs: "add_comm_one", "add_comm_two"

              conditions/bgp-conditions/match-community-set/config:
                "community-set": "my_community"
                "match-set-options": "ANY"
              conditions/bgp-conditions/match-prefix-set/config: 
                "prefix-set": "prefix-set-5"
                "match-set-options": "ANY"


  policy-definitions/policy-definition/config/name: "nested_policy_accept_regex"
        statements/statement:
            config/name: "accept-community-regex"
              actions/config/policy-result: "ACCEPT_ROUTE"
              conditions/bgp-conditions/match-community-set/config/community-set: "regex-community"
                  match-set-options: "ANY"


  * For each policy-definition created, run a subtest (RT-7.11.3.x-<policy_name_here>) to
    * Use gnmi Set REPLACE option for:
      * `/routing-policy/policy-definitions` to configure the policy
      * Use `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy`
        to apply the policy on the DUT bgp neighbor to the ATE port 1.
    * Verify expected communities are present in ATE.
    * Verify expected communities are present in DUT state.
      * Do not fail test if this path is not supported, only log results
      * `/network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/ext-community-index`
      * `/network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/ext-community-index`

    * [ TODO: Add Expected routes and communities for each policy ]

[ TODO: Update expected paths to be used below ]
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

## Minimum DUT Required

vRX - Virtual Router Device
