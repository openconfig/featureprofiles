# RT-1.30: BGP nested import/export policy attachment

## Summary

- A policy calling another policy to be attached to a neighbor's import-policy
- A policy calling another policy to be attached to a neighbor's export-policy
- Applicable to both IPv4 and IPv6 BGP neighbors
- Single level nesting is sufficient

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
*   Configure IPv4 and IPv6 eBGP between DUT Port-2 and ATE Port-2
    *   Note: This eBGP session is only used to advertise prefixes to DUT and receive prefixes from DUT
    *   /network-instances/network-instance/protocols/protocol/bgp/global/config
    *   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/
    *   Advertise ```ipv4-network-2 = 192.168.20.0/24``` and ```ipv6-network-2 = 2024:db8:64:64::/64``` from ATE to DUT over the IPv4 and IPv6 eBGP session on port-2
    *   Set default import and export policy to ```NEXT_STATEMENT``` for this eBGP session only
        *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
        *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy

### RT-1.30.1 [TODO: https://github.com/openconfig/featureprofiles/issues/2608]
#### IPv4 BGP nested import policy test
---
##### Configure a route-policy to set the local preference
*   Configure an IPv4 route-policy definition with the name ```lp-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```lp-policy-v4``` configure a statement with the name ```lp-statement-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```lp-policy-v4``` statement ```lp-statement-v4``` set policy-result as ```NEXT_STATEMENT```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure BGP actions to set local-pref
*   For routing-policy ```lp-policy-v4``` statement ```lp-statement-v4``` set local-preference to ```200```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-local-pref

##### Configure a route-policy to match the prefix
*   Configure an IPv4 route-policy definition with the name ```match-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```match-policy-v4``` configure a statement with the name ```match-statement-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```match-policy-v4``` statement ```match-statement-v4``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure a prefix-set for route filtering/matching
*   Configure a prefix-set with the name ```prefix-set-v4``` and mode ```IPV4```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/name
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode
*   For prefix-set ```prefix-set-v4``` set the ip-prefix to ```ipv4-network-1``` i.e. ```192.168.10.0/24``` and masklength to ```exact```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range
##### Attach the prefix-set to route-policy
*   For routing-policy ```match-policy-v4``` statement ```match-statement-v4``` set match options to ```ANY```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options
*   For routing-policy ```match-policy-v4``` statement ```match-statement-v4``` set prefix set to ```prefix-set-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set

##### Configure a nested policy 
*   For routing-policy ```lp-policy-v4``` call the policy ```match-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/config/call-policy

##### Configure the parent bgp import policy for the DUT BGP neighbor on ATE Port-1
*   Set default import policy to ```REJECT_ROUTE``` (Note: even though this is the OC default, the DUT should still accept this configuration)
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
*   Apply the parent policy ```lp-policy-v4``` to the BGP neighbor
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy

##### Verification
*   Use gNMI `replace` to send the configuration to the DUT.
*   Use gNMI `subscribe` with mode `once` to retrieve the configuration `state` from the DUT.
*   Verify that the parent ```lp-policy-v4``` policy is successfully applied to the DUT BGP neighbor on ATE Port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy
*   Verify that the parent ```lp-policy-v4``` policy has a child policy ```match-policy-v4``` attached
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/state/call-policy

##### Validate test results
*   Validate that the DUT receives the prefix ```ipv4-network-1``` i.e. ```192.168.10.0/24``` from BGP neighbor on ATE Port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix
*   Validate that the prefix ```ipv4-network-1``` i.e. ```192.168.10.0/24``` from BGP neighbor on ATE Port-1 has local preference set to 200
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med
*   Initiate traffic from ATE Port-2 towards the DUT destined to ```ipv4-network-1``` i.e. ```192.168.10.0/24```
    *   Validate that the traffic is received on ATE Port-1

### RT-1.30.2 [TODO: https://github.com/openconfig/featureprofiles/issues/2608]
#### IPv4 BGP nested export policy test
---
##### Configure a route-policy to prepend AS-PATH
*   Configure an IPv4 route-policy definition with the name ```asp-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```asp-policy-v4``` configure a statement with the name ```asp-statement-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```asp-policy-v4``` statement ```asp-statement-v4``` set policy-result as ```NEXT_STATEMENT```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure BGP actions to prepend AS
*   For routing-policy ```asp-policy-v4``` statement ```asp-statement-v4``` set AS-PATH prepend to the ASN of the DUT
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/asn

##### Configure another route-policy to set the MED
*   Configure an IPv4 route-policy definition with the name ```med-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```med-policy-v4``` configure a statement with the name ```med-statement-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```med-policy-v4``` statement ```med-statement-v4``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure BGP actions to set MED
*   For routing-policy ```med-policy-v4``` statement ```med-statement-v4``` set MED to ```1000```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-med

##### Configure a nested policy 
*   For routing-policy ```asp-policy-v4``` attach the policy ```med-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/config/call-policy

##### Configure the parent bgp import policy for the DUT BGP neighbor on ATE Port-1
*   Set default import policy to ```REJECT_ROUTE``` (Note: even though this is the OC default, the DUT should still accept this configuration)
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
*   Apply the parent policy ```asp-policy-v4``` to the BGP neighbor
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy

##### Verification
*   Use gNMI `subscribe` with mode `once` to retrieve the configuration `state` from the DUT.
*   Verify that the parent ```asp-policy-v4``` policy is successfully applied to the DUT BGP neighbor on ATE Port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy
*   Verify that the parent ```asp-policy-v4``` policy has a child policy ```med-policy-v4``` attached
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/state/call-policy

##### Validate test results
*   Validate that the ATE receives the prefix ```ipv4-network-2``` i.e. ```192.168.20.0/24``` from BGP neighbor on DUT Port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix
*   Validate that the prefix ```ipv4-network-2``` i.e. ```192.168.20.0/24``` on ATE from BGP neighbor on DUT Port-1 has AS-PATH with the ASN of DUT occuring twice
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/member
*   Validate that the prefix ```ipv4-network-2``` i.e. ```192.168.20.0/24``` from BGP neighbor on DUT Port-1 has MED set to ```1000```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med
*   Initiate traffic from ATE Port-1 towards the DUT destined ```ipv4-network-2``` i.e. ```192.168.20.0/24```
    *   Validate that the traffic is received on ATE Port-2

### RT-1.30.3 [TODO: https://github.com/openconfig/featureprofiles/issues/2608]
#### IPv6 BGP nested import policy test
---
##### Configure a route-policy to set the local preference
*   Configure an IPv6 route-policy definition with the name ```lp-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```lp-policy-v6``` configure a statement with the name ```lp-statement-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```lp-policy-v6``` statement ```lp-statement-v6``` set policy-result as ```NEXT_STATEMENT```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure BGP actions to set local-pref
*   For routing-policy ```lp-policy-v6``` statement ```lp-statement-v6``` set local-preference to ```200```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-local-pref

##### Configure a route-policy to match the prefix
*   Configure an IPv6 route-policy definition with the name ```match-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```match-policy-v6``` configure a statement with the name ```match-statement-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```match-policy-v6``` statement ```match-statement-v6``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure a prefix-set for route filtering/matching
*   Configure a prefix-set with the name ```prefix-set-v6``` and mode ```IPV6```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/name
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode
*   For prefix-set ```prefix-set-v6``` set the ip-prefix to ```ipv6-network-1``` i.e. ```2024:db8:128:128::/64``` and masklength to ```exact```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range
##### Attach the prefix-set to route-policy
*   For routing-policy ```match-policy-v6``` statement ```match-statement-v6``` set match options to ```ANY```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options
*   For routing-policy ```match-policy-v6``` statement ```match-statement-v6``` set prefix set to ```prefix-set-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set

##### Configure a nested policy 
*   For routing-policy ```lp-policy-v6``` call the policy ```match-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/config/call-policy

##### Configure the parent bgp import policy for the DUT BGP neighbor on ATE Port-1
*   Set default import policy to ```REJECT_ROUTE``` (Note: even though this is the OC default, the DUT should still accept this configuration)
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
*   Apply the parent policy ```lp-policy-v6``` to the BGP neighbor
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy

##### Verification
*   Use gNMI `subscribe` with mode `once` to retrieve the configuration `state` from the DUT.
*   Verify that the parent ```lp-policy-v6``` policy is successfully applied to the DUT BGP neighbor on ATE Port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy
*   Verify that the parent ```lp-policy-v6``` policy has a child policy ```match-policy-v6``` attached
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/state/call-policy

##### Validate test results
*   Validate that the DUT receives the prefix ```ipv6-network-1``` i.e. ```2024:db8:128:128::/64``` from BGP neighbor on ATE Port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix
*   Validate that the prefix ```ipv6-network-1``` i.e. ```2024:db8:128:128::/64``` from BGP neighbor on ATE Port-1 has local preference set to 200
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med
*   Initiate traffic from ATE Port-2 towards the DUT destined to ```ipv6-network-1``` i.e. ```2024:db8:128:128::/64```
    *   Validate that the traffic is received on ATE Port-1

### RT-1.30.4 [TODO: https://github.com/openconfig/featureprofiles/issues/2608]
#### IPv6 BGP nested export policy test
---
##### Configure a route-policy to prepend AS-PATH
*   Configure an IPv6 route-policy definition with the name ```asp-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```asp-policy-v6``` configure a statement with the name ```asp-statement-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```asp-policy-v6``` statement ```asp-statement-v6``` set policy-result as ```NEXT_STATEMENT```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure BGP actions to prepend AS
*   For routing-policy ```asp-policy-v6``` statement ```asp-statement-v6``` set AS-PATH prepend to the ASN of the DUT
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/asn

##### Configure another route-policy to set the MED
*   Configure an IPv6 route-policy definition with the name ```med-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```med-policy-v6``` configure a statement with the name ```med-statement-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```med-policy-v6``` statement ```med-statement-v6``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure BGP actions to set MED
*   For routing-policy ```med-policy-v6``` statement ```med-statement-v6``` set MED to ```1000```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-med

##### Configure a nested policy 
*   For routing-policy ```asp-policy-v6``` call the policy ```med-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/config/call-policy

##### Configure the parent bgp import policy for the DUT BGP neighbor on ATE Port-1
*   Set default import policy to ```REJECT_ROUTE``` (Note: even though this is the OC default, the DUT should still accept this configuration)
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
*   Apply the parent policy ```asp-policy-v6``` to the BGP neighbor
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy

##### Verification
*   Use gNMI `subscribe` with mode `once` to retrieve the configuration `state` from the DUT.
*   Verify that the parent ```asp-policy-v6``` policy is successfully applied to the DUT BGP neighbor on ATE Port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy
*   Verify that the parent ```asp-policy-v6``` policy has a child policy ```med-policy-v6``` attached
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/state/call-policy

##### Validate test results
*   Validate that the ATE receives the prefix ```ipv6-network-2``` i.e. ```2024:db8:64:64::/64``` from BGP neighbor on DUT Port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix
*   Validate that the prefix ```ipv6-network-2``` i.e. ```2024:db8:64:64::/64``` on ATE from BGP neighbor on DUT Port-1 has AS-PATH with the ASN of DUT occuring twice
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/member
*   Validate that the prefix ```ipv6-network-2``` i.e. ```2024:db8:64:64::/64``` from BGP neighbor on DUT Port-1 has MED set to ```1000```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med
*   Initiate traffic from ATE Port-1 towards the DUT destined to ```ipv6-network-1``` i.e. ```2024:db8:64:64::/64```
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

## Protocol/RPC Parameter Coverage

* gNMI
  * Subscribe (ONCE)
  * Set (REPLACE)

## Required DUT platform

* vRX
