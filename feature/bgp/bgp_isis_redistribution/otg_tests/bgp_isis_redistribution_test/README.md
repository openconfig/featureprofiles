# RT-1.28: BGP to IS-IS redistribution

## Summary

- Source-protocol: BGP, destination-protocol: ISIS L2
- Using Community (Source-protocol: BGP)
- Using defined prefix set

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed

## Procedure

#### Initial Setup:

*   Connect DUT port-1, 2 to ATE port-1, 2
*   Configure IPv4/IPv6 addresses on the ports
*   Create an IPv4 networks i.e. ```ipv4-network = 192.168.10.0/24``` attached to ATE port-2
*   Create an IPv6 networks i.e. ```ipv6-network = 2024:db8:128:128::/64``` attached to ATE port-2
*   Configure IPv4 and IPv6 eBGP between DUT Port-2 and ATE Port-2
    *   /network-instances/network-instance/protocols/protocol/bgp/global/config
    *   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/
    *   Advertise ```ipv4-network = 192.168.10.0/24``` and ```ipv6-network = 2024:db8:128:128::/64``` from ATE to DUT with community ```64512:100```
*   Configure IPv4 and IPv6 IS-IS L2 adjacency between ATE port-1 and DUT port-1
    *   /network-instances/network-instance/protocols/protocol/isis/global/afi-safi
    *   Set level-capability to ```LEVEL_2```
        *   /network-instances/network-instance/protocols/protocol/isis/global/config/level-capability
    *   Set metric-style to ```WIDE_METRIC```
        *   /network-instances/network-instance/protocols/protocol/isis/levels/level/config/metric-style

### RT-1.28.1 [TODO: https://github.com/openconfig/featureprofiles/issues/2570]
#### Non matching IPv4 BGP prefixes in a prefix-set should not be redistributed to IS-IS
---
##### Configure a route-policy
*   Configure an IPv4 route-policy definition with the name ```route-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```route-policy-v4``` configure a statement with the name ```statement-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```route-policy-v4``` statement ```statement-v4``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
*   For routing-policy ```route-policy-v4``` statement ```statement-v4``` set IS-IS level to ```2```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/config/set-level
##### Configure a prefix-set with a prefix that does not match BGP route for ```ipv4-network = 192.168.10.0/24```
*   Configure a prefix-set with the name ```prefix-set-v4``` and mode ```IPV4```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/name
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode
*   For prefix-set ```prefix-set-v4``` set the ip-prefix to ```192.168.20.0/24``` and masklength to ```exact```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range
##### Attach prefix-set to route-policy
*   For routing-policy ```route-policy-v4``` statement ```statement-v4``` set match options to ```ANY```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options
*   For routing-policy ```route-policy-v4``` statement ```statement-v4``` set prefix set to ```prefix-set-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set
##### Configure BGP to IS-IS L2 redistribution
*   Set address-family to ```IPV4```
    *   /network-instances/network-instance/table-connections/table-connection/config/address-family
*   Configure source protocol to ```BGP```
    *   /network-instances/network-instance/table-connections/table-connection/config/src-protocol
*   Configure destination protocol to ```ISIS```
    *   /network-instances/network-instance/table-connections/table-connection/config/dst-protocol
*   Disable metric propogation by setting it to ```false```
    *   /network-instances/network-instance/table-connections/table-connection/config/disable-metric-propagation
##### Set default-import-policy to reject
*   Configure default import policy to ```REJECT_ROUTE```
    *   /network-instances/network-instance/table-connections/table-connection/config/default-import-policy
##### Attach the route-policy to import-policy
*   Apply routing policy ```route-policy-v4``` for redistribution to IS-IS
    *   /network-instances/network-instance/table-connections/table-connection/config/import-policy
##### Verification
*   Verify IPv4 route-policy definition is configured with the name ```route-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/state/name
*   Verify for routing-policy ```route-policy-v4``` a statement with the name ```statement-v4``` is configured
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/state/name
*   Verify for routing-policy ```route-policy-v4``` statement ```statement-v4``` policy-result is set to ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/state/policy-result
*   Verify for routing-policy ```route-policy-v4``` statement ```statement-v4``` IS-IS level is set to ```2```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/state/set-level
*   Verify prefix-set with the name ```prefix-set-v4``` and mode ```IPV4``` is configured
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/state/name
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/state/mode
*   Verify for prefix-set ```prefix-set-v4``` the ip-prefix is set to ```192.168.20.0/24``` and masklength is set to ```exact```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/masklength-range
*   Verify for routing-policy ```route-policy-v4``` statement ```statement-v4``` match options is set to ```ANY```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/state/match-set-options
*   Verify for routing-policy ```route-policy-v4``` statement ```statement-v4``` prefix-set is set to ```prefix-set-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/state/prefix-set
*   Verify routing policy ```route-policy-v4``` is applied as import policy for redistribution to IS-IS
    *   /network-instances/network-instance/table-connections/table-connection/state/import-policy
*   Verify the address-family is set to ```IPV4```
    *   /network-instances/network-instance/table-connections/table-connection/state/address-family
*   Verify source protocol is set to ```BGP```
    *   /network-instances/network-instance/table-connections/table-connection/state/src-protocol
*   Verify destination protocol is set to ```ISIS```
    *   /network-instances/network-instance/table-connections/table-connection/state/dst-protocol
*   Verify disable metric propogation is set to ```false```
    *   /network-instances/network-instance/table-connections/table-connection/state/disable-metric-propagation
*   Verify default import policy is set to ```REJECT_ROUTE```
    *   /network-instances/network-instance/table-connections/table-connection/state/default-import-policy
*   Verify routing policy ```route-policy-v4``` is applied for redistribution to IS-IS
    *   /network-instances/network-instance/table-connections/table-connection/config/import-policy
##### Validate test results
*   Validate that the IS-IS on ATE does not receives the redistributed BGP route for network ```ipv4-network``` i.e. ```192.168.10.0/24```
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv4-reachability/prefixes/prefix/state/prefix

### RT-1.28.2 [TODO: https://github.com/openconfig/featureprofiles/issues/2570]
#### Matching IPv4 BGP prefixes in a prefix-set should be redistributed to IS-IS
---
##### Replace the previously configured prefix and mask in prefix-set configured in RT-1.28.1
*   For prefix-set ```prefix-set-v4``` replace the ip-prefix to ```192.168.10.0/24``` and masklength is set to ```exact```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range
##### Verification
*   Verify for prefix-set ```prefix-set-v4``` the ip-prefix is set to ```192.168.10.0/24``` and masklength is set to ```exact```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/masklength-range
##### Validate test results
*   Validate that the IS-IS on ATE receives the redistributed BGP route for network ```ipv4-network``` i.e. ```192.168.10.0/24```
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv4-reachability/prefixes/prefix/state/prefix
*   Initiate traffic from ATE port-1 to the DUT and destined to ```ipv4-network``` i.e. ```192.168.10.0/24```
*   Validate that the traffic is received on ATE port-2

### RT-1.28.3 [TODO: https://github.com/openconfig/featureprofiles/issues/2570]
#### IPv4: Non matching BGP community in a community-set should not be redistributed to IS-IS
---
##### Configure a community-set
*   Configure a community-set with name ```community-set-v4```
    *   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-set-name
*   For community set ```community-set-v4``` configure a community member value to ```64599:200```
    *   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-member
##### Attach community-set to the route-policy
*   For routing-policy ```route-policy-v4``` statement ```statement-v4``` reference the community set ```community-set-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/match-community-set
##### Verification
*   Verity a community set with name ```community-set-v4``` exists
    *   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-set-name
*   Verify for community set ```community-set-v4``` a community member value of ```64599:200``` is configured
    *   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-member
##### Validate test results
*   Validate that the IS-IS on ATE does not receives the redistributed BGP route for network ```ipv4-network``` i.e. ```192.168.10.0/24```
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv4-reachability/prefixes/prefix/state/prefix

### RT-1.28.4 [TODO: https://github.com/openconfig/featureprofiles/issues/2570]
#### IPv4: Matching BGP community in a community-set should be redistributed to IS-IS
---
##### Replace the previously configured community member value in RT-1.28.3
*   For community set ```community-set-v4``` replece the community member value to ```64512:100```
    *   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-member
##### Verification
*   Verify for community set ```community-set-v4``` a community member value of ```64512:100``` is configured
    *   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-member
##### Validate test results
*   Validate that the IS-IS on ATE receives the redistributed BGP route for network ```ipv4-network``` i.e. ```192.168.10.0/24```
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv4-reachability/prefixes/prefix/state/prefix
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv4-reachability/prefixes/prefix/state/metric
*   Initiate traffic from ATE port-1 to the DUT and destined to ```ipv4-network``` i.e. ```192.168.10.0/24```
*   Validate that the traffic is received on ATE port-2

### RT-1.28.5 [TODO: https://github.com/openconfig/featureprofiles/issues/2570]
#### Non matching IPv6 BGP prefixes in a prefix-set should not be redistributed to IS-IS
---
##### Configure a route-policy
*   Configure an IPv6 route-policy definition with the name ```route-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```route-policy-v6``` configure a statement with the name ```statement-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```route-policy-v6``` statement ```statement-v6``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
*   For routing-policy ```route-policy-v6``` statement ```statement-v6``` set IS-IS level to ```2```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/config/set-level
##### Configure a prefix-set with a prefix that does not match BGP route for ```ipv6-network = 2024:db8:128:128::/64```
*   Configure a prefix-set with the name ```prefix-set-v6``` and mode ```IPv6```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/name
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode
*   For prefix-set ```prefix-set-v6``` set the ip-prefix to ```2024:db8:64:64::/64``` and masklength to ```exact```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range
##### Attach prefix-set to route-policy
*   For routing-policy ```route-policy-v6``` statement ```statement-v6``` set match options to ```ANY```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options
*   For routing-policy ```route-policy-v6``` statement ```statement-v6``` set prefix set to ```prefix-set-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set
##### Configure BGP to IS-IS L2 redistribution
*   Set address-family to ```IPV6```
    *   /network-instances/network-instance/table-connections/table-connection/config/address-family
*   Configure source protocol to ```BGP```
    *   /network-instances/network-instance/table-connections/table-connection/config/src-protocol
*   Configure destination protocol to ```ISIS```
    *   /network-instances/network-instance/table-connections/table-connection/config/dst-protocol
*   Disable metric propogation by setting it to ```false```
    *   /network-instances/network-instance/table-connections/table-connection/config/disable-metric-propagation
##### Set default-import-policy to reject
*   Configure default import policy to ```REJECT_ROUTE```
    *   /network-instances/network-instance/table-connections/table-connection/config/default-import-policy
##### Attach the route-policy to import-policy
*   Apply routing policy ```route-policy-v6``` for redistribution to IS-IS
    *   /network-instances/network-instance/table-connections/table-connection/config/import-policy
##### Verification
*   Verify IPv6 route-policy definition is configured with the name ```route-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/state/name
*   Verify for routing-policy ```route-policy-v6``` a statement with the name ```statement-v6``` is configured
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/state/name
*   Verify for routing-policy ```route-policy-v6``` statement ```statement-v6``` policy-result is set to ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/state/policy-result
*   Verify for routing-policy ```route-policy-v6``` statement ```statement-v6``` IS-IS level is set to ```2```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/state/set-level
*   Verify prefix-set with the name ```prefix-set-v6``` and mode ```IPv6``` is configured
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/state/name
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/state/mode
*   Verify for prefix-set ```prefix-set-v6``` the ip-prefix is set to ```2024:db8:64:64::/64``` and masklength is set to ```exact```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/masklength-range
*   Verify for routing-policy ```route-policy-v6``` statement ```statement-v6``` match options is set to ```ANY```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/state/match-set-options
*   Verify for routing-policy ```route-policy-v6``` statement ```statement-v6``` prefix-set is set to ```prefix-set-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/state/prefix-set
*   Verify routing policy ```route-policy-v6``` is applied as import policy for redistribution to IS-IS
    *   /network-instances/network-instance/table-connections/table-connection/state/import-policy
*   Verify the address-family is set to ```IPV6```
    *   /network-instances/network-instance/table-connections/table-connection/state/address-family
*   Verify source protocol is set to ```BGP```
    *   /network-instances/network-instance/table-connections/table-connection/state/src-protocol
*   Verify destination protocol is set to ```ISIS```
    *   /network-instances/network-instance/table-connections/table-connection/state/dst-protocol
*   Verify disable metric propogation is set to ```false```
    *   /network-instances/network-instance/table-connections/table-connection/state/disable-metric-propagation
*   Verify default import policy is set to ```REJECT_ROUTE```
    *   /network-instances/network-instance/table-connections/table-connection/state/default-import-policy
*   Verify routing policy ```route-policy-v6``` is applied for redistribution to IS-IS
    *   /network-instances/network-instance/table-connections/table-connection/config/import-policy
##### Validate test results
*   Validate that the IS-IS on ATE does not receives the redistributed BGP route for network ```ipv6-network``` i.e. ```2024:db8:128:128::/64```
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv6-reachability/prefixes/prefix/state/prefix

### RT-1.28.6 [TODO: https://github.com/openconfig/featureprofiles/issues/2570]
#### Matching IPv6 BGP prefixes in a prefix-set should be redistributed to IS-IS
---
##### Replace the previously configured prefix and mask in prefix-set configured in RT-1.28.5
*   For prefix-set ```prefix-set-v6``` replace the ip-prefix to ```2024:db8:128:128::/64``` and masklength is set to ```exact```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range
##### Verification
*   Verify for prefix-set ```prefix-set-v6``` the ip-prefix is set to ```2024:db8:128:128::/64``` and masklength is set to ```exact```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/masklength-range
##### Validate test results
*   Validate that the IS-IS on ATE receives the redistributed BGP route for network ```ipv6-network``` i.e. ```2024:db8:128:128::/64```
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv6-reachability/prefixes/prefix/state/prefix
*   Initiate traffic from ATE port-1 to the DUT and destined to ```ipv6-network``` i.e. ```2024:db8:128:128::/64```
*   Validate that the traffic is received on ATE port-2

### RT-1.28.7 [TODO: https://github.com/openconfig/featureprofiles/issues/2570]
#### IPv6: Non matching BGP community in a community-set should not be redistributed to IS-IS
---
##### Configure a community-set
*   Configure a community-set with name ```community-set-v6```
    *   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-set-name
*   For community set ```community-set-v6``` configure a community member value to ```64599:200```
    *   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-member
##### Attach community-set to the route-policy
*   For routing-policy ```route-policy-v6``` statement ```statement-v6``` reference the community set ```community-set-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/match-community-set
##### Verification
*   Verity a community set with name ```community-set-v6``` exists
    *   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-set-name
*   Verify for community set ```community-set-v6``` a community member value of ```64599:200``` is configured
    *   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-member
##### Validate test results
*   Validate that the IS-IS on ATE does not receives the redistributed BGP route for network ```ipv6-network``` i.e. ```2024:db8:128:128::/64```
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv6-reachability/prefixes/prefix/state/prefix

### RT-1.28.8 [TODO: https://github.com/openconfig/featureprofiles/issues/2570]
#### IPv6: Matching BGP community in a community-set should be redistributed to IS-IS
---
##### Replace the previously configured community member value in RT-1.28.7
*   For community set ```community-set-v6``` replece the community member value to ```64512:100```
    *   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-member
##### Verification
*   Verify for community set ```community-set-v6``` a community member value of ```64512:100``` is configured
    *   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-member
##### Validate test results
*   Validate that the IS-IS on ATE receives the redistributed BGP route for network ```ipv6-network``` i.e. ```2024:db8:128:128::/64```
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv6-reachability/prefixes/prefix/state/prefix
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv6-reachability/prefixes/prefix/state/metric
*   Initiate traffic from ATE port-1 to the DUT and destined to ```ipv6-network``` i.e. ```2024:db8:128:128::/64```
*   Validate that the traffic is received on ATE port-2

## Config parameter coverage

*   /network-instances/network-instance/protocols/protocol/bgp/global/config
*   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/

*   /network-instances/network-instance/protocols/protocol/isis/global/afi-safi
*   /network-instances/network-instance/protocols/protocol/isis/global/config/level-capability
*   /network-instances/network-instance/protocols/protocol/isis/levels/level/config/metric-style

*   /routing-policy/policy-definitions/policy-definition/config/name
*   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
*   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/config/set-level

*   /routing-policy/defined-sets/prefix-sets/prefix-set/config/name
*   /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode
*   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
*   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range

*   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options
*   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set

*   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-set-name
*   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-member
*   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-community/reference/config/community-set-ref

*   /network-instances/network-instance/table-connections/table-connection/config/address-family
*   /network-instances/network-instance/table-connections/table-connection/config/src-protocol
*   /network-instances/network-instance/table-connections/table-connection/config/dst-protocol
*   /network-instances/network-instance/table-connections/table-connection/config/disable-metric-propagation
*   /network-instances/network-instance/table-connections/table-connection/config/import-policy

## Telemetry parameter coverage

*   /routing-policy/policy-definitions/policy-definition/state/name
*   /routing-policy/policy-definitions/policy-definition/statements/statement/state/name
*   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/state/policy-result
*   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/state/set-level

*   /routing-policy/defined-sets/prefix-sets/prefix-set/state/name
*   /routing-policy/defined-sets/prefix-sets/prefix-set/state/mode
*   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/ip-prefix
*   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/masklength-range

*   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/state/match-set-options
*   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/state/prefix-set

*   /network-instances/network-instance/table-connections/table-connection/state/import-policy
*   /network-instances/network-instance/table-connections/table-connection/state/address-family
*   /network-instances/network-instance/table-connections/table-connection/state/src-protocol
*   /network-instances/network-instance/table-connections/table-connection/state/dst-protocol
*   /network-instances/network-instance/table-connections/table-connection/state/disable-metric-propagation

*   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-set-name
*   /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-member

*   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv4-reachability/prefixes/prefix/state/prefix
*   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv6-reachability/prefixes/prefix/state/prefix

## Protocol/RPC Parameter Coverage

* gNMI
  * Get
  * Set

## Required DUT platform

* FFF

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml 
paths:
  ## Config paths
  /network-instances/network-instance/protocols/protocol/isis/global/config/level-capability:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/config/metric-style:
  /routing-policy/policy-definitions/policy-definition/config/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/config/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/config/set-level:
  /routing-policy/defined-sets/prefix-sets/prefix-set/config/name:
  /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set:
  /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-set-name:
  /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-member:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-community/reference/config/community-set-ref:
  /network-instances/network-instance/table-connections/table-connection/config/address-family:
  /network-instances/network-instance/table-connections/table-connection/config/src-protocol:
  /network-instances/network-instance/table-connections/table-connection/config/dst-protocol:
  /network-instances/network-instance/table-connections/table-connection/config/disable-metric-propagation:
  /network-instances/network-instance/table-connections/table-connection/config/import-policy:

  ## State paths
  /routing-policy/policy-definitions/policy-definition/state/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/state/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/state/policy-result:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/isis-actions/state/set-level:
  /routing-policy/defined-sets/prefix-sets/prefix-set/state/name:
  /routing-policy/defined-sets/prefix-sets/prefix-set/state/mode:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/ip-prefix:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/masklength-range:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/state/match-set-options:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/state/prefix-set:
  /network-instances/network-instance/table-connections/table-connection/state/import-policy:
  /network-instances/network-instance/table-connections/table-connection/state/address-family:
  /network-instances/network-instance/table-connections/table-connection/state/src-protocol:
  /network-instances/network-instance/table-connections/table-connection/state/dst-protocol:
  /network-instances/network-instance/table-connections/table-connection/state/disable-metric-propagation:
  /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-set-name:
  /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-member:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```
