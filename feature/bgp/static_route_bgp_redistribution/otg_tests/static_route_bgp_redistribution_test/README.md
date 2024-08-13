# RT-1.27: Static route to BGP redistribution

## Summary

- Static routes selected for redistribution base on combination of: prefix-set, set-tag
- MED set to value of metric of static route (metric propagation)
- AS-Path prepend to contain AS with value provided in configuration (repeat prepend 'n' times)
- Local-Preference to a value provided in configuration
- Community list set to defined community set
- BGP protocol next-hop set to value provided in configuration
- Redstribute static-route with "DROP" as the next-hop

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed

## Procedure

#### Initial Setup:

*   Connect DUT port-1, 2 and 3 to ATE port-1, 2 and 3 respectively

*   Configure IPv4 and IPv6 addresses on DUT and ATE ports as shown below
    *   DUT port-1 IPv4 address ```dp1-v4 = 192.168.1.1/30```
    *   ATE port-1 IPv4 address ```ap1-v4 = 192.168.1.2/30```

    *   DUT port-2 IPv4 address ```dp2-v4 = 192.168.1.5/30```
    *   ATE port-2 IPv4 address ```ap2-v4 = 192.168.1.6/30```

    *   DUT port-3 IPv4 address ```dp3-v4 = 192.168.1.9/30```
    *   ATE port-3 IPv4 address ```ap3-v4 = 192.168.1.10/30```

    *   DUT port-1 IPv6 address ```dp1-v6 = 2001:DB8::1/126```
    *   ATE port-1 IPv6 address ```ap1-v6 = 2001:DB8::2/126```

    *   DUT port-2 IPv6 address ```dp2-v6 = 2001:DB8::5/126```
    *   ATE port-2 IPv6 address ```ap2-v6 = 2001:DB8::6/126```

    *   DUT port-3 IPv6 address ```dp3-v6 = 2001:DB8::9/126```
    *   ATE port-3 IPv6 address ```ap3-v6 = 2001:DB8::10/126```

*   Create two IPv4 networks i.e. ```ipv4-network = 192.168.10.0/24``` and ```ipv4-drop-network = 192.168.20.0/24``` attached to ATE port-2

*   Create two IPv6 networks i.e. ```ipv6-network = 2024:db8:128:128::/64``` and ```ipv6-drop-network = 2024:db8:64:64::/64``` attached to ATE port-2

*   Configure IPv4 and IPv6 eBGP session between ATE port-1 and DUT port-1
    *   ATE ASN = 64511
    *   DUT ASN = 64512
    *   /network-instances/network-instance/protocols/protocol/bgp/global/config
    *   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/
    *   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/send-community-type = ```STANDARD```

*   Configure IPv4 and IPv6 iBGP session between ATE port-3 and DUT port-3
    *   ATE ASN = 64512
    *   DUT ASN = 64512
    *   /network-instances/network-instance/protocols/protocol/bgp/global/config
    *   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/
    *   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/send-community-type = ```STANDARD```

*   On the DUT advertise networks of ```dp2-v4``` i.e. ```192.168.1.4/30``` and ```dp2-v6``` i.e. ```2001:DB8::0/126``` through the BGP session between DUT port-1 and ATE port-1
    *   Do not configure BGP between DUT port-2 and ATE port-2
    *   Do not advertise ```ipv4-network 192.168.10.0/24``` or ```ipv6-network 2024:db8:128:128::/64```

*   Configure an IPv4 static route ```ipv4-route``` on DUT destined to the ```ipv4-network``` i.e. ```192.168.10.0/24``` with the next hop set to the IPv4 address of ATE port-2 ```ap2-v4``` i.e. ```192.168.1.6/30```
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop
    *   Set the metric of the ```ipv4-route``` to 104
        *   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/metric
    *   Set a tag on the ```ipv4-route``` to 40
        *   /network-instances/network-instance/protocols/protocol/static-routes/static/config/set-tag

*   Configure an IPv6 static route on DUT destined to the ```ipv6-network``` i.e. ```2024:db8:128:128::/64``` with the next hop set to the IPv6 address of ATE port-2 ```ap2-v6``` i.e. ```2001:DB8::5/126```
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop
    *   Set the metric of the ```ipv6-route``` to 106
        *   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/metric
    *   Set a tag on the ```ipv6-route``` to 60
        *   /network-instances/network-instance/protocols/protocol/static-routes/static/config/set-tag


### RT-1.27.1 [TODO: https://github.com/openconfig/featureprofiles/issues/2568]
#### Redistribute IPv4 static routes to BGP with default-import-policy set to reject
---
##### Configure default policy to reject routes
*   Configure default import policy to ```REJECT_ROUTE```
    *   /network-instances/network-instance/table-connections/table-connection/config/default-import-policy
##### Verification
*   Verify default import policy is set to ```REJECT_ROUTE```
    *   /network-instances/network-instance/table-connections/table-connection/state/default-import-policy
##### Validate test results
*   Validate that the ATE does not receives the redistributed static route ```ipv4-route```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix

### RT-1.27.2 [TODO: https://github.com/openconfig/featureprofiles/issues/2568]
#### Redistribute IPv4 static routes to BGP matching a prefix using a route-policy
---
##### Configure a route-policy
*   Configure an IPv4 route-policy definition with the name ```route-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```route-policy-v4``` configure a statement with the name ```statement-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```route-policy-v4``` statement ```statement-v4``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure a prefix-set for route-filtering/matching
*   Configure a prefix-set with the name ```prefix-set-v4``` and mode ```IPV4```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/name
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode
*   For prefix-set ```prefix-set-v4``` set the ip-prefix to ```ipv4-network``` i.e. ```192.168.10.0/24``` and masklength to ```exact```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range
*   For prefix-set ```prefix-set-v4``` set another ip-prefix to ```ipv4-drop-network``` i.e. ```192.168.20.0/24``` and masklength to ```exact```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range
##### Attach the prefix-set to route-policy
*   For routing-policy ```route-policy-v4``` statement ```statement-v4``` set match options to ```ANY```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options
*   For routing-policy ```route-policy-v4``` statement ```statement-v4``` set prefix set to ```prefix-set-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set
##### Attach the route-policy to the redistribution import-policy
*   Apply routing policy ```route-policy-v4``` for redistribution to BGP
    *   /network-instances/network-instance/table-connections/table-connection/config/import-policy
##### Verification
*   Verify IPv4 route-policy definition is configured with the name ```route-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/state/name
*   Verify for routing-policy ```route-policy-v4``` a statement with the name ```statement-v4``` is configured
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/state/name
*   Verify for routing-policy ```route-policy-v4``` statement ```statement-v4``` policy-result is set to ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/state/policy-result
*   Verify prefix-set with the name ```prefix-set-v4``` and mode ```IPV4``` is configured
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/state/name
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/state/mode
*   Verify for prefix-set ```prefix-set-v4``` the ip-prefix is set to ```192.168.10.0/24``` and masklength is set to ```exact```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/masklength-range
*   Verify for routing-policy ```route-policy-v4``` statement ```statement-v4``` match options is set to ```ANY```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/state/match-set-options
*   Verify for routing-policy ```route-policy-v4``` statement ```statement-v4``` prefix-set is set to ```prefix-set-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/state/prefix-set
*   Verify routing policy ```route-policy-v4``` is applied as import policy for redistribution to BGP
    *   /network-instances/network-instance/table-connections/table-connection/state/import-policy
##### Validate the test results
*   Validate that the ATE receives the redistributed static route ```ipv4-route``` with MED of ```104```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med
*   Initiate traffic from ATE port-1 to the DUT and destined to ```ipv4-network``` i.e. ```192.168.10.0/24```
*   Validate that the traffic is received on ATE port-2

### RT-1.27.3 [TODO: https://github.com/openconfig/featureprofiles/issues/2568]
#### Redistribute IPv4 static routes to BGP with metric propogation diabled
---
##### Configure redistribution 
*   Disable metric propogation by setting it to ```true```
    *   /network-instances/network-instance/table-connections/table-connection/config/disable-metric-propagation
##### Verification
*   Verify disable metric propogation is set to ```true```
    *   /network-instances/network-instance/table-connections/table-connection/state/disable-metric-propagation
##### Validate test results
*   Validate that the ATE receives the redistributed static route ```ipv4-route``` with MED either having no value (missing) or ```0``` but not ```104```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med

### RT-1.27.4 [TODO: https://github.com/openconfig/featureprofiles/issues/2568]
#### Redistribute IPv4 static routes to BGP with metric propogation enabled
---
##### Configure static route metric to be copied to MED
*   Enable metric propogation by setting disable-metric-propagation to ```false```
    *   /network-instances/network-instance/table-connections/table-connection/config/disable-metric-propagation
##### Verification
*   Verify disable metric propogation is now ```false```
    *   /network-instances/network-instance/table-connections/table-connection/state/disable-metric-propagation
##### Validate test results
*   Validate that the ATE receives the redistributed static route ```ipv4-route``` with MED of ```104```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med

### RT-1.27.5 [TODO: https://github.com/openconfig/featureprofiles/issues/2568]
#### Redistribute IPv4 static routes to BGP with AS-PATH prepend
---
##### Configure BGP actions to prepend AS
*   For routing-policy ```route-policy-v4``` statement ```statement-v4``` set AS-PATH prepend to the ASN ```64599```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/asn
*   For routing-policy ```route-policy-v4``` statement ```statement-v4``` set the prepended ASN to repeat ```3``` times
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/repeat-n
##### Verification
*   Verify for routing-policy ```route-policy-v4``` statement ```statement-v4``` AS-PATH prepend is set to the ASN ```64599```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/state/asn
*   Verify for routing-policy ```route-policy-v4``` statement ```statement-v4``` the prepended ASN ```64599``` repeats ```3``` times
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/state/repeat-n
##### Validate the test results
*   Validate that the ATE receives the redistributed static route ```ipv4-route``` with AS-PATH of ```64599 64599 64599 64512```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/member

### RT-1.27.6 [TODO: https://github.com/openconfig/featureprofiles/issues/2568]
#### Redistribute IPv4 static routes to BGP with MED set to ```1000```
---
##### Configure BGP actions to set MED
*   For routing-policy ```route-policy-v4``` statement ```statement-v4``` set MED to ```1000```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-med
##### Verification
*   Verify for routing-policy ```route-policy-v4``` statement ```statement-v4``` MED is set to ```1000```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/state/set-med
##### Validate test results
*   validate that the ATE receives the redistributed static route ```ipv4-route``` with MED of ```1000```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med

### RT-1.27.7 [TODO: https://github.com/openconfig/featureprofiles/issues/2568]
#### Redistribute IPv4 static routes to BGP with Local-Preference set to ```100```
---
##### Configure BGP actions to set local-pref
*   For routing-policy ```route-policy-v4``` statement ```statement-v4``` set local-preference to ```100```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-local-pref
##### Verification
*   Verify for routing-policy ```route-policy-v4``` statement ```statement-v4``` local-preference is set to ```100```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/state/set-local-pref
##### Validate test results
*   Validate that the ATE receives the redistributed static route ```ipv4-route``` with MED of ```1000``` on the iBGP session between DUT-ATE port 3
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/local-pref

### RT-1.27.8 [TODO: https://github.com/openconfig/featureprofiles/issues/2568]
#### Redistribute IPv4 static routes to BGP with community set to ```64512:100```
---
##### Configure a community-set
*   Configure a community set with name ```community-set-v4```
    *   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-set-name
*   For community set ```community-set-v4``` configure a community member value to ```64512:100```
    *   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-member
##### Attach the community-set to route-policy
*   For routing-policy ```route-policy-v4``` statement ```statement-v4``` reference the community set ```community-set-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-community/reference/config/community-set-ref
##### Verification
*   Verify a community set with name ```community-set-v4``` exists
    *   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-set-name
*   Verify for community set ```community-set-v4``` a community member value of ```64512:100``` is configured
    *   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-member
##### Validate test results
*   Validate that the ATE receives the redistributed static route ```ipv4-route``` with a community value of ```64512:100```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/communities/community/state/index
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/community-index

### RT-1.27.9 [TODO: https://github.com/openconfig/featureprofiles/issues/2568]
#### Redistribution of IPv4 static routes to BGP that does not match a conditional tag should not happen
---
##### Configure a tag-set with incorrect tag value to validate that the route is not redistributed
*   Configure a tag-set with name ```tag-set-v4```
    *   /routing-policy/defined-sets/tag-sets/tag-set/config/name
*   Configure tag-set ```tag-set-v4``` with a tag value of ```100```
    *   /routing-policy/defined-sets/tag-sets/tag-set/config/tag-value
##### Attach the tag-set to route-policy conditions
*   For routing-policy ```route-policy-v4``` statement ```statement-v4``` configure match-set-tag condition to ```tag-set-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-tag-set/config/tag-set
*   For routing-policy ```route-policy-v4``` statement ```statement-v4``` configure match options to ```ANY```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-tag-set/config/match-set-options
##### Verification
*   Verify a tag-set with name ```tag-set-v4``` is configured
    *   /routing-policy/defined-sets/tag-sets/tag-set/state/name
*   Verify tag-set ```tag-set-v4``` with a tag value of ```100``` is configured
    *   /routing-policy/defined-sets/tag-sets/tag-set/state/tag-value
*   Verify for routing-policy ```route-policy-v4``` statement ```statement-v4``` tag-set is set to ```tag-set-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-tag-set/state/tag-set
*   Verify for routing-policy ```route-policy-v4``` statement ```statement-v4``` match-set-options is set to ```ANY```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-tag-set/state/match-set-options
##### Validate test results
*   Verify that the ATE does not receives the redistributed static route ```ipv4-route```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix

### RT-1.27.10 [TODO: https://github.com/openconfig/featureprofiles/issues/2568]
#### Redistribution of IPv4 static routes to BGP that matches a conditional tag should happen
---
##### Configure a tag-set with correct tag value to validate that the route is redistributed
*   Configure tag-set ```tag-set-v4``` with a tag value of ```40```
    *   /routing-policy/defined-sets/tag-sets/tag-set/config/tag-value
##### Verification
*   Verify tag-set ```tag-set-v4``` with a tag value of ```40``` is configured
    *   /routing-policy/defined-sets/tag-sets/tag-set/state/tag-value
##### Validate test results
*   Verify that the ATE receives the redistributed static route ```ipv4-route```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix
*   Initiate traffic from ATE port-1 to the DUT and destined to ```ipv4-network``` i.e. ```192.168.10.0/24```
*   Validate that the traffic is received on ATE port-2

### RT-1.27.11 [TODO: https://github.com/openconfig/featureprofiles/issues/2568]
#### Redistribute a NULL IPv4 static routes to BGP with a next-hop configured through route-policy
---
##### Configure a NULL static route
*   Configure an IPv4 static route ```ipv4-drop-route``` on DUT destined to ```ipv4-drop-network``` i.e. ```192.168.20.0/24``` with the next hop set to ```DROP```
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop
##### Configure a tag on the static route
*   Set a tag on the ```ipv4-drop-route``` to ```40```
        *   /network-instances/network-instance/protocols/protocol/static-routes/static/config/set-tag
##### Configure BGP actions to set a next-hop
*   For routing-policy ```route-policy-v4``` statement ```statement-v4``` set next-hop to ```192.168.1.9```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-next-hop
##### Verification
*   Verify for routing-policy ```route-policy-v4``` statement ```statement-v4``` next-hop is set to ```192.168.1.9```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/state/set-next-hop
##### Validate the test results
*   Validate that the ATE receives the redistributed static route ```ipv4-drop-route``` on the iBGP session between DUT-ATE port 3
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix
*   Initiate traffic from ATE port-3 to the DUT and destined to ```ipv4-drop-network``` i.e. ```192.168.20.0/24```
*   Validate that the traffic is received on ATE port-2



### RT-1.27.12 [TODO: https://github.com/openconfig/featureprofiles/issues/2568]
#### Redistribute IPv6 static routes to BGP with default-import-policy set to reject
---
##### Configure default policy to reject routes
*   Configure default import policy to ```REJECT_ROUTE```
    *   /network-instances/network-instance/table-connections/table-connection/config/default-import-policy
##### Verification
*   Verify default import policy is set to ```REJECT_ROUTE```
    *   /network-instances/network-instance/table-connections/table-connection/state/default-import-policy
##### Validate test results
*   Validate that the ATE does not receives the redistributed static route ```ipv4-route``` and ```ipv6-route```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix

### RT-1.27.13 [TODO: https://github.com/openconfig/featureprofiles/issues/2568]
#### Redistribute IPv6 static routes to BGP matching a prefix using a route-policy
---
##### Configure a route-policy
*   Configure an IPv6 route-policy definition with the name ```route-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```route-policy-v6``` configure a statement with the name ```statement-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```route-policy-v6``` statement ```statement-v6``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure a prefix-set for route-filtering/matching
*   Configure a prefix-set with the name ```prefix-set-v6``` and mode ```IPV6``` for the routing policy ```route-policy-v6```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/name
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode
*   For prefix-set ```prefix-set-v6``` set the ip-prefix to ```ipv6-network``` i.e. ```2024:db8:128:128::/64``` and masklength to ```exact```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range
*   For prefix-set ```prefix-set-v6``` set another ip-prefix to ```ipv6-drop-network``` i.e. ```2024:db8:64:64::/64``` and masklength to ```exact```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range
##### Attach the prefix-set to route-policy
*   For routing-policy ```route-policy-v6``` statement ```statement-v6``` set match options to ```ANY```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options
*   For routing-policy ```route-policy-v6``` statement ```statement-v6``` set prefix set to ```prefix-set-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set
##### Attach the route-policy to the redistribution import-policy
*   Apply routing policy ```route-policy-v6``` for redistribution to BGP
    *   /network-instances/network-instance/table-connections/table-connection/config/import-policy
##### Verification
*   Verify for routing-policy ```route-policy-v6``` a statement with the name ```statement-v6``` is configured
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/state/name
*   Verify for routing-policy ```route-policy-v6``` statement ```statement-v6``` policy-result is set to ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/state/policy-result
*   Verify for routing-policy ```route-policy-v6``` statement ```statement-v6``` AS-PATH prepend is set to the ASN ```64512```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/state/asn
*   Verify prefix-set with the name ```prefix-set-v6``` and mode ```IPV6``` is configured
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/state/name
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/state/mode
*   Verify for prefix-set ```prefix-set-v6``` the ip-prefix is set to ```2024:db8:128:128::/64``` and masklength is set to ```exact```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/masklength-range
*   Verify for routing-policy ```route-policy-v6``` statement ```statement-v6``` match options is set to ```ANY```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/state/match-set-options
*   Verify for routing-policy ```route-policy-v6``` statement ```statement-v6``` prefix-set is set to ```prefix-set-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/state/prefix-set
*   Verify routing policy ```route-policy-v6``` is applied as import policy for redistribution to BGP
    *   /network-instances/network-instance/table-connections/table-connection/state/import-policy
##### Validate test results
*   Validate that the ATE receives the redistributed static route ```ipv6-route``` with MED of ```1000```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med
*   Initiate traffic from ATE port-1 to the DUT and destined to ```ipv6-network``` i.e. ```2024:db8:128:128::/64```
*   Validate that the traffic is received on ATE port-2


### RT-1.27.14 [TODO: https://github.com/openconfig/featureprofiles/issues/2568]
#### Redistribute IPv6 static routes to BGP with metric propogation diabled
---
##### Disable metric propogation
*   Disable metric propogation by setting it to ```true```
    *   /network-instances/network-instance/table-connections/table-connection/config/disable-metric-propagation
##### Verification
*   Verify disable metric propogation is set to ```true```
    *   /network-instances/network-instance/table-connections/table-connection/state/disable-metric-propagation
##### Validate test results
*   Validate that the ATE receives the redistributed static route ```ipv6-route``` with MED either having no value (missing) or ```0``` but not ```106```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med

### RT-1.27.15 [TODO: https://github.com/openconfig/featureprofiles/issues/2568]
#### Redistribute IPv6 static routes to BGP with metric propogation enabled
---
##### Configure static route metric to be copied to MED
*   Enable metric propogation by setting it to ```false```
    *   /network-instances/network-instance/table-connections/table-connection/config/disable-metric-propagation
##### Verification
*   Verify disable metric propogation is now ```false```
    *   /network-instances/network-instance/table-connections/table-connection/state/disable-metric-propagation
##### Validate test results
*   Validate that the ATE receives the redistributed static route ```ipv6-route``` with MED ```106```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med

### RT-1.27.16 [TODO: https://github.com/openconfig/featureprofiles/issues/2568]
#### Redistribute IPv6 static routes to BGP with AS-PATH prepend
---
##### Configure BGP actions to prepend AS
*   For routing-policy ```route-policy-v6``` statement ```statement-v6``` set AS-PATH prepend to the ASN ```64512```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/asn
##### Verification
*   Verify for routing-policy ```route-policy-v6``` statement ```statement-v6``` AS-PATH prepend is set to the ASN ```64512```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/state/asn
##### Validate test results
*   Validate that the ATE receives the redistributed static route ```ipv6-route``` with AS-PATH of ```64512 64512```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/member

### RT-1.27.17 [TODO: https://github.com/openconfig/featureprofiles/issues/2568]
#### Redistribute IPv6 static routes to BGP with MED set to ```1000```
---
##### Configure BGP actions to set MED
*   For routing-policy ```route-policy-v6``` statement ```statement-v6``` set MED to ```1000```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-med
##### Verification
*   Verify for routing-policy ```route-policy-v6``` statement ```statement-v6``` MED is set to ```1000```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/state/set-med
##### Validate test results
*   Validate that the ATE receives the redistributed static route ```ipv6-route``` with MED of ```1000```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med

### RT-1.27.18 [TODO: https://github.com/openconfig/featureprofiles/issues/2568]
#### Redistribute IPv6 static routes to BGP with Local-Preference set to ```100```
---
##### Configure BGP actions to set local-pref
*   For routing-policy ```route-policy-v4``` statement ```statement-v4``` set local-preference to ```100```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-local-pref
##### Verification
*   Verify for routing-policy ```route-policy-v4``` statement ```statement-v4``` local-preference is set to ```100```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/state/set-local-pref
##### Validate test results
*   Validate that the ATE receives the redistributed static route ```ipv4-route``` with MED of ```1000``` on the iBGP session between DUT-ATE port 3
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/local-pref

### RT-1.27.19 [TODO: https://github.com/openconfig/featureprofiles/issues/2568]
#### Redistribute IPv6 static routes to BGP with community set to ```64512:100```
---
##### Configure a community-set
*   Configure a community set with name ```community-set-v6```
    *   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-set-name
*   For community set ```community-set-v6``` configure a community member value to ```64512:100```
    *   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-member
##### Attach the community-set to route-policy
*   For routing-policy ```route-policy-v6``` statement ```statement-v6``` reference the community set ```community-set-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-community/reference/config/community-set-ref
##### Verification
*   Verity a community set with name ```community-set-v6``` exists
    *   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-set-name
*   Verify for community set ```community-set-v6``` a community member value of ```64512:100``` is configured
    *   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-member
##### Validate test results
*   Validate that the ATE receives the redistributed static route ```ipv6-route``` with a community value of ```64512:100```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/communities/community/state/index
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/state/community-index

### RT-1.27.20 [TODO: https://github.com/openconfig/featureprofiles/issues/2568]
#### Redistribution of IPv6 static routes to BGP that does not match a conditional tag should not happen
---
##### Configure a tag-set with incorrect tag value to validate that the route is not redistributed
*   Configure a tag-set with name ```tag-set-v6```
    *   /routing-policy/defined-sets/tag-sets/tag-set/config/name
*   Configure tag-set ```tag-set-v6``` with a tag value of ```100```
    *   /routing-policy/defined-sets/tag-sets/tag-set/config/tag-value
##### Attach the tag-set to route-policy conditions
*   For routing-policy ```route-policy-v6``` statement ```statement-v6``` configure tag-set to ```tag-set-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-tag-set/config/tag-set
*   For routing-policy ```route-policy-v6``` statement ```statement-v6``` configure match options to ```ANY```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-tag-set/config/match-set-options
##### Verification
*   Verify a tag-set with name ```tag-set-v6``` is configured
    *   /routing-policy/defined-sets/tag-sets/tag-set/state/name
*   Verify tag-set ```tag-set-v6``` with a tag value of ```100``` is configured
    *   /routing-policy/defined-sets/tag-sets/tag-set/state/tag-value
*   Verify for routing-policy ```route-policy-v6``` statement ```statement-v6``` match-set-tag is set to ```tag-set-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-tag-set/state/tag-set
*   Verify for routing-policy ```route-policy-v6``` statement ```statement-v6``` match-set-options is set to ```ANY```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-tag-set/state/match-set-options
##### Validate test results
*   Verify that the ATE does not receives the redistributed static route ```ipv6-route```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix

### RT-1.27.21 [TODO: https://github.com/openconfig/featureprofiles/issues/2568]
#### Redistribution of IPv6 static routes to BGP that matches a conditional tag should happen
---
##### Configure a tag-set with correct tag value to validate that the route is redistributed
*   Configure tag-set ```tag-set-v6``` with a tag value of ```60```
    *   /routing-policy/defined-sets/tag-sets/tag-set/config/tag-value
    *   here we are setting correct tag value of 60, as defined in initial setup of this test, to validate that the route is now redistributed
##### Verification
*   Verify tag-set ```tag-set-v6``` with a tag value of ```60``` is configured
    *   /routing-policy/defined-sets/tag-sets/tag-set/state/tag-value
##### Validate test results
*   Verify that the ATE receives the redistributed static route ```ipv6-route```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix
*   Initiate traffic from ATE port-1 to the DUT and destined to ```ipv6-network``` i.e. ```2024:db8:128:128::/64```
*   Validate that the traffic is received on ATE port-2

### RT-1.27.22 [TODO: https://github.com/openconfig/featureprofiles/issues/2568]
#### Redistribute a NULL IPv6 static routes to BGP with a next-hop configured through route-policy
---
##### Configure a NULL static route
*   Configure an IPv6 static route ```ipv6-drop-route``` on DUT destined to ```ipv6-drop-network``` i.e. ```2024:db8:64:64::/64``` with the next hop set to ```DROP```
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop
##### Configure a tag on the static route
*   Set a tag on the ```ipv6-drop-route``` to 60
        *   /network-instances/network-instance/protocols/protocol/static-routes/static/config/set-tag
##### Configure BGP actions to set a next-hop
*   For routing-policy ```route-policy-v6``` statement ```statement-v6``` set next-hop to ```2001:DB8::9```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-next-hop
##### Verification
*   Verify for routing-policy ```route-policy-v4``` statement ```statement-v4``` next-hop is set to ```2001:DB8::9```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/state/set-next-hop
##### Validate the test results
*   Validate that the ATE receives the redistributed static route ```ipv4-drop-route``` on the iBGP session between DUT-ATE port 3
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix
*   Initiate traffic from ATE port-3 to the DUT and destined to ```ipv4-drop-network``` i.e. ```2024:db8:64:64::/64```
*   Validate that the traffic is received on ATE port-2

## Config parameter coverage

*   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/
*   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/send-community-type
*   /network-instances/network-instance/protocols/protocol/bgp/global/config

*   /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix
*   /network-instances/network-instance/protocols/protocol/static-routes/static/config/set-tag
*   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/metric
*   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop

*   /network-instances/network-instance/table-connections/table-connection/config/address-family
*   /network-instances/network-instance/table-connections/table-connection/config/default-import-policy
*   /network-instances/network-instance/table-connections/table-connection/config/disable-metric-propagation
*   /network-instances/network-instance/table-connections/table-connection/config/dst-protocol
*   /network-instances/network-instance/table-connections/table-connection/config/import-policy
*   /network-instances/network-instance/table-connections/table-connection/config/src-protocol

*   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-member
*   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-set-name

*   /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode
*   /routing-policy/defined-sets/prefix-sets/prefix-set/config/name
*   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
*   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range

*   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/repeat-n

*   /routing-policy/defined-sets/tag-sets/tag-set/config/name
*   /routing-policy/defined-sets/tag-sets/tag-set/config/tag-value

*   /routing-policy/policy-definitions/policy-definition/config/name
*   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
*   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-local-pref
*   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-med
*   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-next-hop
*   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/asn
*   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-community/reference/config/community-set-ref

*   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options
*   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set

*   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-tag-set/config/match-set-options
*   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-tag-set/config/tag-set

## Telemetry parameter coverage

*  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix
*  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix

*  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/community-index
*  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/state/community-index

*  /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/member
*  /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/local-pref
*  /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med

*  /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-member
*  /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-set-name

*  /network-instances/network-instance/protocols/protocol/bgp/rib/communities/community/state/index

*  /network-instances/network-instance/table-connections/table-connection/state/address-family
*  /network-instances/network-instance/table-connections/table-connection/state/default-import-policy
*  /network-instances/network-instance/table-connections/table-connection/state/disable-metric-propagation
*  /network-instances/network-instance/table-connections/table-connection/state/dst-protocol
*  /network-instances/network-instance/table-connections/table-connection/state/import-policy
*  /network-instances/network-instance/table-connections/table-connection/state/src-protocol

*  /routing-policy/defined-sets/prefix-sets/prefix-set/state/mode
*  /routing-policy/defined-sets/prefix-sets/prefix-set/state/name
*  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/ip-prefix
*  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/masklength-range

*  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/state/repeat-n

*  /routing-policy/defined-sets/tag-sets/tag-set/state/name
*  /routing-policy/defined-sets/tag-sets/tag-set/state/tag-value

*  /routing-policy/policy-definitions/policy-definition/state/name
*  /routing-policy/policy-definitions/policy-definition/statements/statement/state/name
*  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/state/policy-result

*  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/state/asn
*  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/state/set-local-pref
*  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/state/set-med
*  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/state/set-next-hop

*  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/state/match-set-options
*  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/state/prefix-set

*  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-tag-set/state/match-set-options
*  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-tag-set/state/tag-set

## OpenConfig Path and RPC Coverage
```yaml
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
```

## Required DUT platform

* FFF
