# RT-2.12: Static route to IS-IS redistribution

## Summary

- Static metric to IS-IS Extended IP Reachability / IPv6 IP Reachability (TLV135 / TLV236)
  - IS-IS wide metric set to value of metric of static route (metric propagation)
- Redistribution to IS-IS L2 based on combination of prefix-set and set-tag

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed

## Procedure

#### Initial Setup:

*   Connect DUT port-1 and port-2 to ATE port-1 and port-2 respectively

*   Configure IPv4 and IPv6 addresses on DUT and ATE ports as shown below
    *   DUT port-1 IPv4 address ```dp1-v4 = 192.168.1.1/30```
    *   ATE port-1 IPv4 address ```ap1-v4 = 192.168.1.2/30```

    *   DUT port-2 IPv4 address ```dp2-v4 = 192.168.1.5/30```
    *   ATE port-2 IPv4 address ```ap2-v4 = 192.168.1.6/30```

    *   DUT port-1 IPv6 address ```dp1-v6 = 2001:DB8::1/126```
    *   ATE port-1 IPv6 address ```ap1-v6 = 2001:DB8::2/126```

    *   DUT port-2 IPv6 address ```dp2-v6 = 2001:DB8::4/126```
    *   ATE port-2 IPv6 address ```ap2-v6 = 2001:DB8::5/126```

*   Create an IPv4 network i.e. ```ipv4-network = 192.168.10.0/24``` attached to ATE port-2

*   Create an IPv6 network i.e. ```ipv6-network = 2024:db8:128:128::/64``` attached to ATE port-2

*   Configure IPv4 and IPv6 IS-IS L2 adjacency between ATE port-1 and DUT port-1
    *   /network-instances/network-instance/protocols/protocol/isis/global/afi-safi
    *   Set level-capability to ```LEVEL_2```
        *   /network-instances/network-instance/protocols/protocol/isis/global/config/level-capability
    *   Set metric-style to ```WIDE_METRIC```
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/config/metric-style

*   On the DUT advertise networks of ```dp2-v4``` i.e. ```192.168.1.4/30``` and ```dp2-v6``` i.e. ```2001:DB8::0/126``` through the IS-IS adjacency between DUT port-1 and ATE port-1
    *   Do not configure IS-IS between DUT port-2 and ATE port-2
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

### RT-2.12.1 [TODO: https://github.com/openconfig/featureprofiles/issues/2494]
#### Redistribute IPv4 static route to IS-IS with metric propogation diabled

*   Redistribute ```ipv4-route``` to IS-IS
*   Set address-family to ```IPV4```  
    *   /network-instances/network-instance/table-connections/table-connection/config/address-family
*   Configure source protocol to ```STATIC```
    *   /network-instances/network-instance/table-connections/table-connection/config/src-protocol
*   Configure destination protocol to ```ISIS```
    *   /network-instances/network-instance/table-connections/table-connection/config/dst-protocol
*   Configure default import policy to ```ACCEPT_ROUTE```
    *   /network-instances/network-instance/table-connections/table-connection/config/default-import-policy
*   Disable metric propogation by setting it to ```true```
    *   /network-instances/network-instance/table-connections/table-connection/config/disable-metric-propagation
*   Verify the address-family is set to ```IPV4```
    *   /network-instances/network-instance/table-connections/table-connection/state/address-family
*   Verify source protocol is set to ```STATIC```
    *   /network-instances/network-instance/table-connections/table-connection/state/src-protocol
*   Verify destination protocol is set to ```ISIS```
    *   /network-instances/network-instance/table-connections/table-connection/state/dst-protocol
*   Verify default import policy is set to ```ACCEPT_ROUTE```
    *   /network-instances/network-instance/table-connections/table-connection/state/default-import-policy
*   Verify disable metric propogation is set to ```true```
    *   /network-instances/network-instance/table-connections/table-connection/state/disable-metric-propagation
*   Validate that the ATE receives the redistributed static route ```ipv4-route``` with default metric of ```0``` and not ```104```
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv4-reachability/prefixes/prefix/state/prefix
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv4-reachability/prefixes/prefix/state/metric

### RT-2.12.2 [TODO: https://github.com/openconfig/featureprofiles/issues/2494]
#### Redistribute IPv4 static route to IS-IS with metric propogation enabled
*   Enable metric propogation by setting disable-metric-propagation to ```false```
    *   /network-instances/network-instance/table-connections/table-connection/config/disable-metric-propagation
*   Verify disable metric propogation is now ```false```
    *   /network-instances/network-instance/table-connections/table-connection/state/disable-metric-propagation
*   Validate that the ATE receives the redistributed static route ```ipv4-route``` with metric```104```
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv4-reachability/prefixes/prefix/state/prefix
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv4-reachability/prefixes/prefix/state/metric

### RT-2.12.3 [TODO: https://github.com/openconfig/featureprofiles/issues/2494]
#### Redistribute IPv6 static route to IS-IS with metric propogation diabled

*   Redistribute ```ipv6-route``` to IS-IS
*   Set address-family to ```IPV6```  
    *   /network-instances/network-instance/table-connections/table-connection/config/address-family
*   Configure source protocol to ```STATIC```
    *   /network-instances/network-instance/table-connections/table-connection/config/src-protocol
*   Configure destination protocol to ```ISIS```
    *   /network-instances/network-instance/table-connections/table-connection/config/dst-protocol
*   Configure default import policy to ```ACCEPT_ROUTE```
    *   /network-instances/network-instance/table-connections/table-connection/config/default-import-policy
*   Disable metric propogation by setting it to ```true```
    *   /network-instances/network-instance/table-connections/table-connection/config/disable-metric-propagation
*   Verify the address-family is set to ```IPV6```
    *   /network-instances/network-instance/table-connections/table-connection/state/address-family
*   Verify source protocol is set to ```STATIC```
    *   /network-instances/network-instance/table-connections/table-connection/state/src-protocol
*   Verify destination protocol is set to ```ISIS```
    *   /network-instances/network-instance/table-connections/table-connection/state/dst-protocol
*   Verify default import policy is set to ```ACCEPT_ROUTE```
    *   /network-instances/network-instance/table-connections/table-connection/state/default-import-policy
*   Verify disable metric propogation is set to ```true```
    *   /network-instances/network-instance/table-connections/table-connection/state/disable-metric-propagation
*   Validate that the ATE receives the redistributed static route ```ipv6-route``` with default metric of ```0``` and not ```106```
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-reachability/prefixes/prefix/state/prefix
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-reachability/prefixes/prefix/state/metric

### RT-2.12.4 [TODO: https://github.com/openconfig/featureprofiles/issues/2494]
#### Redistribute IPv6 static route to IS-IS with metric propogation enabled

*   Enable metric propogation by setting it to ```false```
    *   /network-instances/network-instance/table-connections/table-connection/config/disable-metric-propagation
*   Verify disable metric propogation is now ```false```
    *   /network-instances/network-instance/table-connections/table-connection/state/disable-metric-propagation
*   Validate that the ATE receives the redistributed static route ```ipv6-route``` with metric ```106```
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-reachability/prefixes/prefix/state/prefix
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-reachability/prefixes/prefix/state/metric

### RT-2.12.5 [TODO: https://github.com/openconfig/featureprofiles/issues/2494]
#### Redistribute IPv4 and IPv6 static route to IS-IS with default-import-policy set to reject

*   Configure default import policy to ```REJECT_ROUTE```
    *   /network-instances/network-instance/table-connections/table-connection/config/default-import-policy
*   Verify default import policy is set to ```REJECT_ROUTE```
    *   /network-instances/network-instance/table-connections/table-connection/state/default-import-policy
*   Validate that the ATE does not receives the redistributed static route ```ipv4-route``` and ```ipv6-route```
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv4-reachability/prefixes/prefix/state/prefix
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-reachability/prefixes/prefix/state/prefix

### RT-2.12.6 [TODO: https://github.com/openconfig/featureprofiles/issues/2494]
#### Redistribute IPv4 static route to IS-IS matching a prefix using a route-policy

*   Configure an IPv4 route-policy definition with the name ```route-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```route-policy-v4``` configure a statement with the name ```statement-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```route-policy-v4``` statement ```statement-v4``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
*   For routing-policy ```route-policy-v4``` statement ```statement-v4``` set level to ```2```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/config/set-level
*   For routing-policy ```route-policy-v4``` statement ```statement-v4``` set metric to ```1000```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/config/set-metric
*   For routing-policy ```route-policy-v4``` statement ```statement-v4``` set metric style to ```WIDE_METRIC```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/config/set-metric-style-type
*   Configure a prefix-set with the name ```prefix-set-v4``` and mode ```IPV4```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/name
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode
*   For prefix-set ```prefix-set-v4``` set the ip-prefix to ```ipv4-network``` i.e. ```192.168.10.0/24``` and masklength to ```exact```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range
*   For routing-policy ```route-policy-v4``` statement ```statement-v4``` set match options to ```ANY```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options
*   For routing-policy ```route-policy-v4``` statement ```statement-v4``` set prefix set to ```prefix-set-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set
*   Apply routing policy ```route-policy-v4``` for redistribution to IS-IS
    *   /network-instances/network-instance/table-connections/table-connection/config/import-policy
*   Verify IPv4 route-policy definition is configured with the name ```route-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/state/name
*   Verify for routing-policy ```route-policy-v4``` a statement with the name ```statement-v4``` is configured
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/state/name
*   Verify for routing-policy ```route-policy-v4``` statement ```statement-v4``` policy-result is set to ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/state/policy-result
*   verify for routing-policy ```route-policy-v4``` statement ```statement-v4``` level is set to ```2```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/state/set-level
*   Verify for routing-policy ```route-policy-v4``` statement ```statement-v4``` metric is set to ```1000```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/state/set-metric
*   verify for routing-policy ```route-policy-v4``` statement ```statement-v4``` metric style is set to ```WIDE_METRIC```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/state/set-metric-style-type
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
*   Verify routing policy ```route-policy-v4``` is applied as import policy for redistribution to IS-IS
    *   /network-instances/network-instance/table-connections/table-connection/state/import-policy
*   Verify that the redistibuted static routes ```ipv4-route``` is being received by the ATE
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv4-reachability/prefixes/prefix/state/prefix
*   Verify that the ATE receives the redistributed static route ```ipv4-route``` with metric of ```1000```
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv4-reachability/prefixes/prefix/state/metric
*   Initiate traffic from ATE port-1 to the DUT and destined to ```ipv4-network``` i.e. ```192.168.10.0/24```
*   Validate that the traffic is received on ATE port-2

### RT-2.12.7 [TODO: https://github.com/openconfig/featureprofiles/issues/2494]
#### Redistribute IPv4 static route to IS-IS matching a tag

*   Configure a tag-set with name ```tag-set-v4```
    *   /routing-policy/defined-sets/tag-sets/tag-set/config/name
*   Configure tag-set ```tag-set-v4``` with a tag value of ```100```
    *   /routing-policy/defined-sets/tag-sets/tag-set/config/tag-value
    *   here we are setting incorrect tag value of 100 to validate that the route is not redistributed
*   For routing-policy ```route-policy-v4``` statement ```statement-v4``` configure match-set-tag condition to ```tag-set-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-tag-set/config/tag-set
*   For routing-policy ```route-policy-v4``` statement ```statement-v4``` configure match options to ```ANY```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-tag-set/config/match-set-options
*   Verify a tag-set with name ```tag-set-v4``` is configured
    *   /routing-policy/defined-sets/tag-sets/tag-set/state/name
*   Verify tag-set ```tag-set-v4``` with a tag value of ```100``` is configured
    *   /routing-policy/defined-sets/tag-sets/tag-set/state/tag-value
*   Verify for routing-policy ```route-policy-v4``` statement ```statement-v4``` tag-set is set to ```tag-set-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-tag-set/state/tag-set
*   Verify for routing-policy ```route-policy-v4``` statement ```statement-v4``` match-set-options is set to ```ANY```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-tag-set/state/match-set-options
*   Verify that the ATE does not receives the redistributed static route ```ipv4-route```
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv4-reachability/prefixes/prefix/state/metric
*   Configure tag-set ```tag-set-v4``` with a tag value of ```40```
    *   /routing-policy/defined-sets/tag-sets/tag-set/config/tag-value
    *   here we are setting correct tag value of 40, as defined in initial setup of this test, to validate that the route is now redistributed
*   Initiate traffic from ATE port-1 to the DUT and destined to ```ipv4-network``` i.e. ```192.168.10.0/24```
*   Validate that the traffic is received on ATE port-2

### RT-2.12.8 [TODO: https://github.com/openconfig/featureprofiles/issues/2494]
#### Redistribute IPv6 static route to IS-IS matching a prefix using a route-policy

*   Configure an IPv6 route-policy definition with the name ```route-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```route-policy-v6``` configure a statement with the name ```statement-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```route-policy-v6``` statement ```statement-v6``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
*   For routing-policy ```route-policy-v6``` statement ```statement-v6``` set level to ```2```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/config/set-level
*   For routing-policy ```route-policy-v6``` statement ```statement-v6``` set metric to ```1000```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/config/set-metric
*   For routing-policy ```route-policy-v6``` statement ```statement-v6``` set metric style to ```WIDE_METRIC```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/config/set-metric-style-type
*   Configure a prefix-set with the name ```prefix-set-v6``` and mode ```IPV6``` for the routing policy ```route-policy-v6```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/name
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode
*   For prefix-set ```prefix-set-v6``` set the ip-prefix to ```ipv6-network``` i.e. ```2024:db8:128:128::/64``` and masklength to ```exact```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range
*   For routing-policy ```route-policy-v6``` statement ```statement-v6``` set match options to ```ANY```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options
*   For routing-policy ```route-policy-v6``` statement ```statement-v6``` set prefix set to ```prefix-set-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set
*   Apply routing policy ```route-policy-v6``` for redistribution to IS-IS
    *   /network-instances/network-instance/table-connections/table-connection/config/import-policy
*   Verify for routing-policy ```route-policy-v6``` a statement with the name ```statement-v6``` is configured
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/state/name
*   Verify for routing-policy ```route-policy-v6``` statement ```statement-v6``` policy-result is set to ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/state/policy-result
*   verify for routing-policy ```route-policy-v6``` statement ```statement-v6``` level is set to ```2```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/state/set-level
*   Verify for routing-policy ```route-policy-v6``` statement ```statement-v6``` metric is set to ```1000```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/state/set-metric
*   verify for routing-policy ```route-policy-v6``` statement ```statement-v6``` metric style is set to ```WIDE_METRIC```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/state/set-metric-style-type
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
*   Verify routing policy ```route-policy-v6``` is applied as import policy for redistribution to IS-IS
    *   /network-instances/network-instance/table-connections/table-connection/state/import-policy
*   Verify that the redistibuted static routes ``ipv6-route``` is being received by the ATE
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-reachability/prefixes/prefix/state/prefix
*   Verify that the ATE receives the redistributed static route ```ipv6-route``` with metric of ```1000```
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-reachability/prefixes/prefix/state/metric
*   Initiate traffic from ATE port-1 to the DUT and destined to ```ipv6-network``` i.e. ```2024:db8:128:128::/64```
*   Validate that the traffic is received on ATE port-2

### RT-2.12.9 [TODO: https://github.com/openconfig/featureprofiles/issues/2494]
#### Redistribute IPv6 static route to IS-IS matching a tag

*   Configure a tag-set with name ```tag-set-v6```
    *   /routing-policy/defined-sets/tag-sets/tag-set/config/name
*   Configure tag-set ```tag-set-v6``` with a tag value of ```100```
    *   /routing-policy/defined-sets/tag-sets/tag-set/config/tag-value
    *   here we are setting incorrect tag value of 100 to validate that the route is not redistributed
*   For routing-policy ```route-policy-v6``` statement ```statement-v6``` configure tag-set to ```tag-set-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-tag-set/config/tag-set
*   For routing-policy ```route-policy-v6``` statement ```statement-v6``` configure match options to ```ANY```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-tag-set/config/match-set-options
*   Verify a tag-set with name ```tag-set-v6``` is configured
    *   /routing-policy/defined-sets/tag-sets/tag-set/state/name
*   Verify tag-set ```tag-set-v6``` with a tag value of ```100``` is configured
    *   /routing-policy/defined-sets/tag-sets/tag-set/state/tag-value
*   Verify for routing-policy ```route-policy-v6``` statement ```statement-v6``` match-set-tag is set to ```tag-set-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-tag-set/state/tag-set
*   Verify for routing-policy ```route-policy-v6``` statement ```statement-v6``` match-set-options is set to ```ANY```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-tag-set/state/match-set-options
*   Verify that the ATE does not receives the redistributed static route ```ipv6-route```
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/pv6-reachability/prefixes/prefix/state/metric
*   Configure tag-set ```tag-set-v6``` with a tag value of ```60```
    *   /routing-policy/defined-sets/tag-sets/tag-set/config/tag-value
    *   here we are setting correct tag value of 60, as defined in initial setup of this test, to validate that the route is now redistributed
*   Initiate traffic from ATE port-1 to the DUT and destined to ```ipv6-network``` i.e. ```2024:db8:128:128::/64```
*   Validate that the traffic is received on ATE port-2

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

```yaml
paths:
  ## Config Paths ##
  #/network-instances/network-instance/table-connections/table-connection/config:
  /network-instances/network-instance/table-connections/table-connection/config/address-family:
  /network-instances/network-instance/table-connections/table-connection/config/src-protocol:
  /network-instances/network-instance/table-connections/table-connection/config/dst-protocol:
  /network-instances/network-instance/table-connections/table-connection/config/default-import-policy:
  /network-instances/network-instance/table-connections/table-connection/config/import-policy:
  /network-instances/network-instance/table-connections/table-connection/config/disable-metric-propagation:
  /routing-policy/policy-definitions/policy-definition/config/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/config/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result:
  /routing-policy/defined-sets/prefix-sets/prefix-set/config/name:
  /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set:
  /routing-policy/defined-sets/tag-sets/tag-set/config/name:
  /routing-policy/defined-sets/tag-sets/tag-set/config/tag-value:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-tag-set/config/match-set-options:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-tag-set/config/tag-set:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/config/set-level:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/config/set-metric:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/config/set-metric-style-type:

  ## State Paths ##
  /network-instances/network-instance/table-connections/table-connection/state/address-family:
  /network-instances/network-instance/table-connections/table-connection/state/default-import-policy:
  /network-instances/network-instance/table-connections/table-connection/state/disable-metric-propagation:
  /network-instances/network-instance/table-connections/table-connection/state/dst-protocol:
  /network-instances/network-instance/table-connections/table-connection/state/import-policy:
  /network-instances/network-instance/table-connections/table-connection/state/src-protocol:
  /routing-policy/policy-definitions/policy-definition/state/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/state/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/state/policy-result:
  /routing-policy/defined-sets/prefix-sets/prefix-set/state/mode:
  /routing-policy/defined-sets/prefix-sets/prefix-set/state/name:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/ip-prefix:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/masklength-range:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/state/match-set-options:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/state/prefix-set:
  /routing-policy/defined-sets/tag-sets/tag-set/state/name:
  /routing-policy/defined-sets/tag-sets/tag-set/state/tag-value:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-tag-set/state/match-set-options:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-tag-set/state/tag-set:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/state/set-level:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/state/set-metric:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/state/set-metric-style-type:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv4-reachability/prefixes/prefix/state/prefix:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-reachability/prefixes/prefix/state/prefix:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv4-reachability/prefixes/prefix/state/metric:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-reachability/prefixes/prefix/state/metric:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
```

## Required DUT platform

* FFF
