# RT-7.11: BGP Policy - Import/Export Policy Action Using Multiple Criteria

## Summary

The purpose of this test is to verify a combination of bgp conditions using matching and policy nesting as well as and actions in a single BGP import policy.  Additional combinations may be added in the future as subtests.

## Testbed type

* [2 port ATE to DUT](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Testbed common configuration

* Testbed configuration - Setup eBGP sessions and prefixes
  * Generate config for 2 DUT and ATE ports where
    * DUT port 1 connects to ATE port 1.
    * DUT port 2 connects to ATE port 2.
  * Configure ATE port 1 with an external type BGP session to DUT port 1
    * DUT ASN 65000
    * ATE port 1 ASN 65100
    * ATE port 2 ASN 65200
    * Advertise ipv4 and ipv6 prefixes from ATE port 1 to DUT port 1 using
      the following communities:
    * prefix-set-1 with 2 ipv4 and 2 ipv6 routes with communities [ "10:1" ]
    * prefix-set-2 with 2 ipv4 and 2 ipv6 routes with communities [ "20:1" ]
    * prefix-set-3 with 2 ipv4 and 2 ipv6 routes with communities [ "30:1" ]
    * prefix-set-4 with 2 ipv4 and 2 ipv6 routes with communities [ "20:2", "30:3" ]
    * prefix-set-5 with 2 ipv4 and 2 ipv6 routes with communities [ "40:1" ]
    * prefix-set-6 with 2 ipv4 and 2 ipv6 routes with communities [ "50:1" ]
    * Configure accept_route policy and apply as an export policy on the DUT
      eBGP session to ATE port 2.

* Configure the following community sets on the DUT:
  * /routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set/config
    * name = "reject_communities"
      * community-member = [ "10:1" ]
    * name = "accept_communities"
      * community-member = [ "20:1" ]
    * name = "regex_community"
      * community-member = [ "^30:.*$" ]
    * name = "add_communities"
      * community-member = [ "40:1", "40:2" ]
    * name "my_community"
      * community-member = [ "50:1" ]
    * name = "add_comm_60"
      * community-member = [ "60:1" ]
    * name = "add_comm_70"
      * community-member = [ "70:1" ]

* Create an as-path-set on the DUT as follows
  * /routing-policy/defined-sets/bgp-defined-sets/as-path-sets/as-path-set/config/
    * as-path-set-name = "my_aspath"
    * as-path-set-member = "65100"

* Validate bgp sessions and traffic
  * For IPv4 and IPv6 prefixes:
    * Observe received prefixes at ATE port-2.
  * Generate traffic from ATE port-2 to ATE port-1.
  * Validate that traffic can be received on ATE port-1 for all installed
    routes.

## Procedure

### RT-7.11.2 - Create a bgp policy containing the following conditions and actions

* Summary of this policy
  * Reject route matching any communities in a community-set.
  * Reject route matching another policy (nested) and not matching a community-set.
  * Add a community-set if missing that same community-set.
  * Add two communities and set localpref if matching a community and prefix-set.
  * Set MED if matching an aspath

* policy-definitions/policy-definition/config/name: "match_community_regex"
  * statements/statement/config/name: "match_community_regex"
    * conditions/bgp-conditions/match-community-set/config/
      * community-set: "regex-community"
      * match-set-options: "ANY"

* Create policy-definitions/policy-definition/config/name = "multi_policy"
  * statements/statement/config/name = "reject_route_community"
    * conditions/bgp-conditions/match-community-set/config
      * community-set = "reject_communities"
      * match-set-options = "ANY"
    * actions/config/policy-result = "REJECT_ROUTE"

  * statements/statement/config/name = "if_30:.*_and_not_20:1_nested_reject"
    * conditions/config/call-policy = "match_community_regex"
    * conditions/bgp-conditions/match-community-set/config/
      * community-set = "accept_communities"
      * match-set-options = "INVERT"
    * actions/config/policy-result = "REJECT_ROUTE"

  * statements/statement/config/name = "add_communities_if_missing"
    * conditions/bgp-conditions/match-community-set/config/
      * community-set-refs = "add-communities"
      * match-set-options: "INVERT"
    * actions/bgp-actions/set-community/reference/config/
      * community-set-refs = "add-communities"
      * method = "REFERENCE"
      * option = "ADD"
    * actions/config/policy-result = "NEXT_STATEMENT"

  * statements/statement/config/name: "match_comm_and_prefix_add_2_community_sets"
    * conditions/bgp-conditions/match-community-set/config
      * community-set = "my_community"
      * match-set-options = "ANY"
    * conditions/bgp-conditions/match-prefix-set/config
      * prefix-set = "prefix-set-5"
      * match-set-options = "ANY"
    * actions/bgp-actions/set-community/config
      * method = "REFERENCE"
      * option = "ADD"
      * community-set-refs = "add_comm_60", "add_comm_70"
    * actions/bgp-actions/config/set-local-pref = 5
    * actions/config/policy-result = "NEXT_STATEMENT"

  * statements/statement/config/name: "match_aspath_set_med"
    * conditions/bgp-conditions/match-as-path-set/config/
      * as-path-set = "my_aspath"
      * match-set-options = "ANY"
    * actions/bgp-actions/config/
      * set-med = 100
    * actions/config/policy-result = "NEXT_STATEMENT"

* For each policy-definition created, run a subtest (RT-7.11.2.x-import_<policy_name_here>) to
  * Use gnmi Set REPLACE option for:
    * `/routing-policy/policy-definitions` to configure the policy
    * Use `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy`
        to apply the policy on the DUT bgp neighbor to the ATE port 1.
  * Verify expected attributes are present in ATE.

* For each policy-definition created, run a subtest (RT-7.11.2.x-export_<policy_name_here>) to
  * Use gnmi Set REPLACE option for:
    * `/routing-policy/policy-definitions` to configure the policy
    * Use `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy`
        to apply the policy on the DUT bgp neighbor to the ATE port 1.
  * Verify expected attributes are present in ATE.

* multi_policy results observed on ATE port 2
  |              | Accept | Communities                                | as-path      | lpref | med | notes                                                     |
  | ------------ | ------ | ------------------------------------------ | ------------ | ----- | --- | --------------------------------------------------------- |
  | prefix-set-1 | False  | n/a                                        | n/a          | n/a   | n/a | rejected by statement reject_route_community              |
  | prefix-set-2 |        | [ "20:1", "40:1", "40:2" ]                 | 65000, 65100 | n/a   | 100 | accepted                                                  |
  | prefix-set-3 | False  | n/a                                        | n/a          | n/a   | n/a | rejected by statement if_30:.*_and_not_20:1_nested_reject |
  | prefix-set-4 | False  | n/a                                        | n/a          | n/a   | n/a | rejected by statement if_30:.*_and_not_20:1_nested_reject |
  | prefix-set-6 |        | [ "10:1", "40:1", "40:2", "60:1", "70:1" ] | 65000, 65100 | 5     | 100 | accepted and match_comm_and_prefix_add_2_community_sets   |

## OpenConfig Path Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

```yaml
config_paths:
  # Policy definition
  - /routing-policy/policy-definitions/policy-definition/config/name
  - /routing-policy/policy-definitions/policy-definition/statements/statement/config/name

  # Policy for community-set configuration
  - /routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set/config/community-set-name
  - /routing-policy/defined-sets/bgp-defined-sets/ext-community-sets/ext-community-set/config/community-member

  # Policy for match configuration
  - /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/match-community-set/config/community-set
  - /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/match-community-set/config/match-set-options

  - /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/match-as-path-set/config/as-path-set
  - /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/match-as-path-set/config/match-set-options

  - /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set
  - /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options

  # Policy for bgp actions
  - /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-community/config/method
  - /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-community/config/options
  - /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-community/reference/config/community-set-ref
  - /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-community/reference/config/community-set-refs
  - /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-local-pref
  - /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-med

  # Policy for bgp attachment
  - /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy
  - /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy

state_paths:
  # Policy definition state
  - /routing-policy/policy-definitions/policy-definition/state/name
  - /routing-policy/policy-definitions/policy-definition/statements/statement/state/name

  # Policy for community-set match state
  - /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-set-name
  - /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-member
  - /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/match-ext-community-set/state/match-set-options
  - /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/state/community-set

  # Paths to verify policy state
  - /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/export-policy
  - /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy

  # Paths to verify prefixes sent and received
  - /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/sent
  - /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received-pre-policy
  - /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received
  - /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/installed
```

## Minimum DUT Required

vRX - Virtual Router Device
