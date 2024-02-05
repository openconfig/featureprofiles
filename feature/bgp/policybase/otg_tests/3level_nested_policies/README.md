# RT-1.31: BGP 3 levels of nested import/export policy with match-set-options

## Summary

- AS-path prepending by more the 10 repetitions
- Recursive policy subroutines (multi-level nesting). At least 3 levels
- match-set-options of ANY, INVERT for match-prefix-set conditions
- Applicable to both IPv4 and IPv6 BGP neighbors

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

## Procedure

### Applying configuration

For each section of configuration below, prepare a gnmi.SetBatch  with all the configuration items appended to one SetBatch.  Then apply the configuration to the DUT in one gnmi.Set using the `replace` option

#### Initial Setup:

*   Connect DUT port-1, 2 to ATE port-1, 2
*   Configure IPv4/IPv6 addresses on the ports
*   Create an IPv4 networks i.e. ```ipv4-network-1 = 192.168.10.0/24``` attached to ATE port-1
*   Create an IPv6 networks i.e. ```ipv6-network-1 = 2024:db8:128:128::/64``` attached to ATE port-1
*   Create an IPv4 networks i.e. ```ipv4-network-2 = 192.168.20.0/24``` attached to ATE port-2
*   Create an IPv6 networks i.e. ```ipv6-network-2 = 2024:db8:64:64::/64``` attached to ATE port-2
*   Configure IPv4 and IPv6 eBGP between DUT Port-1 and ATE Port-1
    *   Note: Nested policies will be applied to this eBGP session later in the test to validate the results
    *   /network-instances/network-instance/protocols/protocol/bgp/global/config
    *   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/
    *   Advertise ```ipv4-network-1 = 192.168.10.0/24``` and ```ipv6-network-1 = 2024:db8:128:128::/64``` from ATE to DUT over the IPv4 and IPv6 eBGP session on port-1
    *   Configure DUT to advertise standard communities to ATE 
        *   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/send-community-type = ```STANDARD```
*   Configure IPv4 and IPv6 eBGP between DUT Port-2 and ATE Port-2
    *   Note: This eBGP session is only used to advertise prefixes to DUT and receive prefixes from DUT
    *   /network-instances/network-instance/protocols/protocol/bgp/global/config
    *   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/
    *   Advertise ```ipv4-network-2 = 192.168.20.0/24``` and ```ipv6-network-2 = 2024:db8:64:64::/64``` from ATE to DUT over the IPv4 and IPv6 eBGP session on port-2
    *   Set default import and export policy to ```ACCEPT_ROUTE``` for this eBGP session only
        *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
        *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy

##### Parent Route Policy: Configure a route-policy to inverse match any prefix in the given prefix-set (INVERT)
*   Note: This parent policy will be applied to both import and export route policy on the neighbor.
*   This policy will call unique nested policies for both import and export scenarios defined in the sub-tests
*   Configure an IPv4 route-policy definition with the name ```invert-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```invert-policy-v4``` configure a statement with the name ```invert-statement-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```invert-policy-v4``` statement ```invert-statement-v4``` set policy-result as ```NEXT_STATEMENT```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure a prefix-set for route filtering/matching
*   Configure a prefix-set with the name ```prefix-set-v4``` and mode ```IPV4```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/name
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode
*   For prefix-set ```prefix-set-v4``` set the ip-prefix to ```10.0.0.0/8``` and masklength to ```exact```
    *   Our intention is to allow the prefix that does not match 10.0.0.0/8 (inverse the match result)
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range
##### Attach the prefix-set to route-policy
*   For routing-policy ```invert-policy-v4``` statement ```invert-statement-v4``` set match options to ```INVERT```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options
*   For routing-policy ```invert-policy-v4``` statement ```invert-statement-v4``` set prefix set to ```prefix-set-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set


### RT-1.31.1 [TODO: https://github.com/openconfig/featureprofiles/issues/2612]
#### IPv4 BGP 3 levels of nested import policy with match-prefix-set conditions
---

##### 2nd Route Policy: Configure a route-policy to match the a prefix in the given prefix-set (ANY)
*   Configure an IPv4 route-policy definition with the name ```match-import-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```match-import-policy-v4``` configure a statement with the name ```match-statement-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```match-import-policy-v4``` statement ```match-statement-v4``` set policy-result as ```NEXT_STATEMENT```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure a prefix-set for route filtering/matching
*   Configure a prefix-set with the name ```prefix-set-v4``` and mode ```IPV4```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/name
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode
*   For prefix-set ```prefix-set-v4``` set the ip-prefix to ```ipv4-network-1``` i.e. ```192.168.10.0/24``` and masklength to ```exact```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range
##### Attach the prefix-set to route-policy
*   For routing-policy ```match-import-policy-v4``` statement ```match-statement-v4``` set match options to ```ANY```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options
*   For routing-policy ```match-import-policy-v4``` statement ```match-statement-v4``` set prefix set to ```prefix-set-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set


##### 3rd Route Policy: Configure a route-policy to set the bgp local preference
*   Configure an IPv4 route-policy definition with the name ```lp-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```lp-policy-v4``` configure a statement with the name ```lp-statement-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```lp-policy-v4``` statement ```lp-statement-v4``` set policy-result as ```NEXT_STATEMENT```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure BGP actions to set local-pref
*   For routing-policy ```lp-policy-v4``` statement ```lp-statement-v4``` set local-preference to ```200```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-local-pref


##### 4th Route Policy: Configure a route-policy to set the bgp community
*   Configure an IPv4 route-policy definition with the name ```community-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```community-policy-v4``` configure a statement with the name ```community-statement-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```community-policy-v4``` statement ```community-statement-v4``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure a community-set
*   Configure a community set with name ```community-set-v4```
    *   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-set-name
*   For community set ```community-set-v4``` configure a community member value to ```64512:100```
    *   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-member
##### Attach the community-set to route-policy
*   For routing-policy ```community-policy-v4``` statement ```community-statement-v4``` reference the community set ```community-set-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-community/reference/config/community-set-ref


##### Configure policy nesting
*   For Parent routing-policy ```invert-policy-v4``` and statement ```invert-statement-v4``` call the policy ```match-import-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/config/call-policy
*   For routing-policy ```match-import-policy-v4``` and statement ```match-statement-v4``` call the policy ```lp-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/config/call-policy
*   For routing-policy ```lp-policy-v4``` and statement ```lp-statement-v4``` call the policy ```community-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/config/call-policy


##### Configure the parent bgp import policy for the DUT BGP neighbor on ATE Port-1
*   Set default import policy to ```REJECT_ROUTE``` (Note: even though this is the OC default, the DUT should still accept this configuration)
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
*   Apply the parent policy ```invert-policy-v4``` to the BGP neighbor
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy


##### Verification
*   Verify that the parent ```invert-policy-v4``` policy is successfully applied to the DUT BGP neighbor on ATE Port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy
*   Verify that the parent ```invert-policy-v4``` policy has a child policy ```match-import-policy-v4``` attached
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/state/call-policy
*   Verify that the sub-parent ```match-import-policy-v4``` policy has a child policy ```lp-policy-v4``` attached
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/state/call-policy
*   Verify that the sub-parent ```lp-policy-v4``` policy has a child policy ```community-policy-v4``` attached
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/state/call-policy


##### Validate test results
*   Validate that the DUT receives the prefix ```ipv4-network-1``` i.e. ```192.168.10.0/24``` from BGP neighbor on ATE Port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix
*   Validate that the prefix ```ipv4-network-1``` i.e. ```192.168.10.0/24``` from BGP neighbor on ATE Port-1 has local preference of ```200```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med
*   Validate that the prefix ```ipv4-network-1``` i.e. ```192.168.10.0/24``` from BGP neighbor on ATE Port-1 has community of  ```64512:100```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/community-index
*   Initiate traffic from ATE Port-2 towards the DUT destined to ```ipv4-network-1``` i.e. ```192.168.10.0/24```
    *   Validate that the traffic is received on ATE Port-1


### RT-1.31.2 [TODO: https://github.com/openconfig/featureprofiles/issues/2612]
#### IPv4 BGP 3 levels of nested export policy with match-prefix-set conditions
---

##### 2nd Route Policy: Configure a route-policy to match the a prefix in the given prefix-set (ANY)
*   Configure an IPv4 route-policy definition with the name ```match-export-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```match-export-policy-v4``` configure a statement with the name ```match-statement-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```match-export-policy-v4``` statement ```match-statement-v4``` set policy-result as ```NEXT_STATEMENT```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure a prefix-set for route filtering/matching
*   Configure a prefix-set with the name ```prefix-set-v4``` and mode ```IPV4```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/name
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode
*   For prefix-set ```prefix-set-v4``` set the ip-prefix to ```ipv4-network-2``` i.e. ```192.168.20.0/24``` and masklength to ```exact```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range
##### Attach the prefix-set to route-policy
*   For routing-policy ```match-export-policy-v4``` statement ```match-statement-v4``` set match options to ```ANY```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options
*   For routing-policy ```match-export-policy-v4``` statement ```match-statement-v4``` set prefix set to ```prefix-set-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set


##### 3rd Route Policy: Configure a route-policy to prepend AS-PATH
*   Configure an IPv4 route-policy definition with the name ```asp-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```asp-policy-v4``` configure a statement with the name ```asp-statement-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```asp-policy-v4``` statement ```asp-statement-v4``` set policy-result as ```NEXT_STATEMENT```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure BGP actions to prepend AS by more than 10 times
*   For routing-policy ```asp-policy-v4``` statement ```asp-statement-v4``` set AS-PATH prepend to the ASN of the DUT
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/asn
*   For routing-policy ```asp-policy-v4``` statement ```asp-statement-v4``` set the prepended ASN to repeat ```15``` times
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/repeat-n


##### 4th Route Policy: Configure a route-policy to set the MED
*   Configure an IPv4 route-policy definition with the name ```med-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```med-policy-v4``` configure a statement with the name ```med-statement-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```med-policy-v4``` statement ```med-statement-v4``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure BGP actions to set MED
*   For routing-policy ```med-policy-v4``` statement ```med-statement-v4``` set MED to ```1000```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-med


##### Configure policy nesting
*   For Parent routing-policy ```invert-policy-v4``` and statement ```invert-statement-v4``` call the policy ```match-export-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/config/call-policy
*   For routing-policy ```match-export-policy-v4``` and statement ```match-statement-v4``` call the policy ```asp-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/config/call-policy
*   For routing-policy ```asp-policy-v4``` and statement ```asp-statement-v4``` call the policy ```med-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/config/call-policy


##### Configure the parent bgp export policy for the DUT BGP neighbor on ATE Port-1
*   Set default export policy to ```REJECT_ROUTE``` (Note: even though this is the OC default, the DUT should still accept this configuration)
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Apply the parent policy ```invert-export-policy-v4``` to the BGP neighbor
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy


##### Verification
*   Verify that the parent ```invert-policy-v4``` policy is successfully applied to the DUT BGP neighbor on ATE Port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/export-policy
*   Verify that the parent ```invert-policy-v4``` policy has a child policy ```match-export-policy-v4``` attached
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/state/call-policy
*   Verify that the parent ```match-export-policy-v4``` policy has a child policy ```asp-policy-v4``` attached
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/state/call-policy
*   Verify that the parent ```asp-policy-v4``` policy has a child policy ```med-policy-v4``` attached
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/state/call-policy


##### Validate test results
*   Validate that the ATE receives the prefix ```ipv4-network-2``` i.e. ```192.168.20.0/24``` from BGP neighbor on DUT Port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix
*   Validate that the prefix ```ipv4-network-2``` i.e. ```192.168.20.0/24``` on ATE from BGP neighbor on DUT Port-1 has AS-PATH with the ASN of DUT occuring more than 10 times
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/member
*   Validate that the prefix ```ipv4-network-2``` i.e. ```192.168.20.0/24``` from BGP neighbor on DUT Port-1 has MED set to ```1000```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med
*   Initiate traffic from ATE Port-1 towards the DUT destined ```ipv4-network-2``` i.e. ```192.168.20.0/24```
    *   Validate that the traffic is received on ATE Port-2


### RT-1.31.3 [TODO: https://github.com/openconfig/featureprofiles/issues/2612]
#### IPv6 BGP 3 levels of nested import policy with match-prefix-set conditions
---

##### 2nd Route Policy: Configure a route-policy to match the a prefix in the given prefix-set (ANY)
*   Configure an IPv6 route-policy definition with the name ```match-import-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```match-import-policy-v6``` configure a statement with the name ```match-statement-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```match-import-policy-v6``` statement ```match-statement-v6``` set policy-result as ```NEXT_STATEMENT```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure a prefix-set for route filtering/matching
*   Configure a prefix-set with the name ```prefix-set-v6``` and mode ```IPV6```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/name
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode
*   For prefix-set ```prefix-set-v6``` set the ip-prefix to ```ipv6-network-1``` i.e. ```2024:db8:128:128::/64``` and masklength to ```exact```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range
##### Attach the prefix-set to route-policy
*   For routing-policy ```match-import-policy-v6``` statement ```match-statement-v6``` set match options to ```ANY```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options
*   For routing-policy ```match-import-policy-v6``` statement ```match-statement-v6``` set prefix set to ```prefix-set-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set


##### 3rd Route Policy: Configure a route-policy to set the bgp local preference
*   Configure an IPv6 route-policy definition with the name ```lp-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```lp-policy-v6``` configure a statement with the name ```lp-statement-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```lp-policy-v6``` statement ```lp-statement-v6``` set policy-result as ```NEXT_STATEMENT```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure BGP actions to set local-pref
*   For routing-policy ```lp-policy-v6``` statement ```lp-statement-v6``` set local-preference to ```200```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-local-pref

##### 4th Route Policy: Configure a route-policy to set the bgp community
*   Configure an IPv6 route-policy definition with the name ```community-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```community-policy-v6``` configure a statement with the name ```community-statement-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```community-policy-v6``` statement ```community-statement-v6``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure a community-set
*   Configure a community set with name ```community-set-v6```
    *   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-set-name
*   For community set ```community-set-v6``` configure a community member value to ```64512:100```
    *   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-member
##### Attach the community-set to route-policy
*   For routing-policy ```community-policy-v6``` statement ```community-statement-v6``` reference the community set ```community-set-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-community/reference/config/community-set-ref


##### Configure policy nesting
*   For Parent routing-policy ```invert-policy-v6``` and statement ```invert-statement-v6``` call the policy ```match-import-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/config/call-policy
*   For routing-policy ```match-import-policy-v6``` and statement ```match-statement-v6``` call the policy ```lp-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/config/call-policy
*   For routing-policy ```lp-policy-v6``` and statement ```lp-statement-v6``` call the policy ```community-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/config/call-policy


##### Configure the parent bgp import policy for the DUT BGP neighbor on ATE Port-1
*   Set default import policy to ```REJECT_ROUTE``` (Note: even though this is the OC default, the DUT should still accept this configuration)
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
*   Apply the parent policy ```invert-policy-v6``` to the BGP neighbor
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy


##### Verification
*   Verify that the parent ```invert-policy-v6``` policy is successfully applied to the DUT BGP neighbor on ATE Port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy
*   Verify that the parent ```invert-policy-v6``` policy has a child policy ```match-import-policy-v6``` attached
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/state/call-policy
*   Verify that the sub-parent ```match-import-policy-v6``` policy has a child policy ```lp-policy-v6``` attached
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/state/call-policy
*   Verify that the sub-parent ```lp-policy-v6``` policy has a child policy ```community-policy-v6``` attached
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/state/call-policy


##### Validate test results
*   Validate that the DUT receives the prefix ```ipv6-network-1``` i.e. ```2024:db8:128:128::/64``` from BGP neighbor on ATE Port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix
*   Validate that the prefix ```ipv6-network-1``` i.e. ```2024:db8:128:128::/64``` from BGP neighbor on ATE Port-1 has local preference of ```200```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med
*   Validate that the prefix ```ipv6-network-1``` i.e. ```2024:db8:128:128::/64``` from BGP neighbor on ATE Port-1 has community of  ```64512:100```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/state/community-index
*   Initiate traffic from ATE Port-2 towards the DUT destined to ```ipv6-network-1``` i.e. ```2024:db8:128:128::/64```
    *   Validate that the traffic is received on ATE Port-1


### RT-1.31.4 [TODO: https://github.com/openconfig/featureprofiles/issues/2612]
#### IPv6 BGP 3 levels of nested export policy with match-prefix-set conditions
---

##### 2nd Route Policy: Configure a route-policy to match the a prefix in the given prefix-set (ANY)
*   Configure an IPv6 route-policy definition with the name ```match-export-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```match-export-policy-v6``` configure a statement with the name ```match-statement-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```match-export-policy-v6``` statement ```match-statement-v6``` set policy-result as ```NEXT_STATEMENT```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure a prefix-set for route filtering/matching
*   Configure a prefix-set with the name ```prefix-set-v6``` and mode ```IPV6```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/name
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode
*   For prefix-set ```prefix-set-v6``` set the ip-prefix to ```ipv6-network-2``` i.e. ```2024:db8:64:64::/64``` and masklength to ```exact```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range
##### Attach the prefix-set to route-policy
*   For routing-policy ```match-export-policy-v6``` statement ```match-statement-v6``` set match options to ```ANY```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options
*   For routing-policy ```match-export-policy-v6``` statement ```match-statement-v6``` set prefix set to ```prefix-set-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set


##### 3rd Route Policy: Configure a route-policy to prepend AS-PATH
*   Configure an IPv6 route-policy definition with the name ```asp-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```asp-policy-v6``` configure a statement with the name ```asp-statement-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```asp-policy-v6``` statement ```asp-statement-v6``` set policy-result as ```NEXT_STATEMENT```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure BGP actions to prepend AS by more than 10 times
*   For routing-policy ```asp-policy-v6``` statement ```asp-statement-v6``` set AS-PATH prepend to the ASN of the DUT
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/asn
*   For routing-policy ```asp-policy-v6``` statement ```asp-statement-v6``` set the prepended ASN to repeat ```15``` times
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/repeat-n


##### 4th Route Policy: Configure a route-policy to set the MED
*   Configure an IPv6 route-policy definition with the name ```med-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```med-policy-v6``` configure a statement with the name ```med-statement-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```med-policy-v6``` statement ```med-statement-v6``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure BGP actions to set MED
*   For routing-policy ```med-policy-v6``` statement ```med-statement-v6``` set MED to ```1000```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-med


##### Configure policy nesting
*   For Parent routing-policy ```invert-policy-v6``` and statement ```invert-statement-v6``` call the policy ```match-export-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/config/call-policy
*   For routing-policy ```match-export-policy-v6``` and statement ```match-statement-v6``` call the policy ```asp-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/config/call-policy
*   For routing-policy ```asp-policy-v6``` and statement ```asp-statement-v6``` call the policy ```med-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/config/call-policy


##### Configure the parent bgp export policy for the DUT BGP neighbor on ATE Port-1
*   Set default export policy to ```REJECT_ROUTE``` (Note: even though this is the OC default, the DUT should still accept this configuration)
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Apply the parent policy ```invert-export-policy-v6``` to the BGP neighbor
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy


##### Verification
*   Verify that the parent ```invert-policy-v6``` policy is successfully applied to the DUT BGP neighbor on ATE Port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/export-policy
*   Verify that the parent ```invert-policy-v6``` policy has a child policy ```match-export-policy-v6``` attached
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/state/call-policy
*   Verify that the parent ```match-export-policy-v6``` policy has a child policy ```asp-policy-v6``` attached
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/state/call-policy
*   Verify that the parent ```asp-policy-v6``` policy has a child policy ```med-policy-v6``` attached
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/state/call-policy


##### Validate test results
*   Validate that the ATE receives the prefix ```ipv6-network-2``` i.e. ```2024:db8:64:64::/64``` from BGP neighbor on DUT Port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix
*   Validate that the prefix ```ipv6-network-2``` i.e. ```2024:db8:64:64::/64``` on ATE from BGP neighbor on DUT Port-1 has AS-PATH with the ASN of DUT occuring more than 10 times
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/member
*   Validate that the prefix ```ipv6-network-2``` i.e. ```2024:db8:64:64::/64``` from BGP neighbor on DUT Port-1 has MED set to ```1000```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med
*   Initiate traffic from ATE Port-1 towards the DUT destined ```ipv6-network-2``` i.e. ```2024:db8:64:64::/64```
    *   Validate that the traffic is received on ATE Port-2


## Config parameter coverage

*   /network-instances/network-instance/protocols/protocol/bgp/global/config
*   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/
*   /routing-policy/policy-definitions/policy-definition/config/name
*   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
*   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/config/call-policy
*   /routing-policy/defined-sets/prefix-sets/prefix-set/config/name
*   /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode
*   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
*   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range
*   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options
*   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set
*   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-med
*   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/asn
*   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/repeat-n
*   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/send-community-type
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy

## Telemetry parameter coverage

*   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/state/call-policy
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/export-policy
*   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/member
*   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix
*   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix
*   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med
*   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix
*   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/community-index

## Protocol/RPC Parameter Coverage

* gNMI
  * Get
  * Set

## Required DUT platform

* vRX
