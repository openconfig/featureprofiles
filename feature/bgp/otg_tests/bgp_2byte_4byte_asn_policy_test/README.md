# RT-1.24: BGP 2-Byte and 4-Byte ASN support with policy

## Summary

BGP 2-Byte and 4-Byte ASN support with policy

## Procedure

*   Establish BGP sessions as follows and verify all the sessions are established:
    *   ATE (2-byte) - DUT (4-byte) - eBGP IPv4 with ASN < 65535 on DUT side
    *   ATE (2-byte) - DUT (4-byte) - eBGP IPv6 with ASN < 65535 on DUT side
    *   ATE (4-byte) - DUT (4-byte) - eBGP IPv4
    *   ATE (4-byte) - DUT (4-byte) - eBGP IPv6
    *   ATE (2-byte) - DUT (4-byte) - iBGP IPv4 with ASN < 65535 on DUT side
    *   ATE (4-byte) - DUT (4-byte) - iBGP IPv6 with ASN < 65535 on DUT side
    *   ATE (4-byte) - DUT (4-byte) - iBGP IPv4
    *   ATE (4-byte) - DUT (4-byte) - iBGP IPv6

*   Configure below policies and verify prefix count:
    *   Policy to reject prefix with prefix filter
    *   Policy to reject prefix with community filter
    *   Policy to reject prefix with regex filter to match as-path

## Config Parameter Coverage

*   /global/config/as
*   /neighbors/neighbor/config/peer-as
*   /neighbors/neighbor/config/local-as

## Telemetry Parameter Coverage

*   /global/config/as
*   /neighbors/neighbor/config/peer-as
*   /neighbors/neighbor/config/local-as

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /network-instances/network-instance/config/type:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/as:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id:
  /network-instances/network-instance/protocols/protocol/bgp/global/state/as:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/local-as:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/local-as:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/peer-as:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/state/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/import-policy:
  /routing-policy/defined-sets/bgp-defined-sets/as-path-sets/as-path-set/config/as-path-set-member:
  /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-member:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/config/community-set:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/match-community-set/config/community-set:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set:
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```