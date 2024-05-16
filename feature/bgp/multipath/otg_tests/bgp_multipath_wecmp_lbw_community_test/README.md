# RT-1.52: BGP multipath UCMP support with Link Bandwidth Community

## Summary

Validate BGP in multipath UCMP support with link bandwidth community

## Testbed type

[TESTBED_DUT_ATE_4LINKS](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Setup

*   Connect DUT port 1, 2 and 3 to ATE port 1, 2 and 3 respectively
*   Configure IPv4/IPv6 addresses on the interfaces
*   Establish eBGP sessions between:
    *   ATE port-1 and DUT port-1
    *   ATE port-2 and DUT port-2
    *   ATE port-3 and DUT port-3
*   Enable an Accept-route all import-policy/export-policy for eBGP session
    under the neighbor AFI/SAFI
*   Create an IPv4 internal target network attached to ATE port 2 and 3

### Tests

*   RT-1.52.1: Verify use of community type

    *   Configure ATE port 1, 2 and 3 on different AS
    *   Enable multipath, set maximum-paths limit to 2, enable allow multiple
        AS, and send community type to BOTH (STANDARD and EXTENDED)
        *   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/use-multiple-paths/config/enabled
        *   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/use-multiple-paths/ebgp/config/allow-multiple-as
        *   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/use-multiple-paths/ebgp/config/maximum-paths
        *   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/send-community-type
        *   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/use-multiple-paths/ebgp/link-bandwidth-ext-community/config/enabled
    *   Advertise equal cost paths from port2 and port3 of ATE
    *   Check entries in FIB for advertised prefix, it should have 2 entries
        *   /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops
    *   Initiate traffic from ATE port-1 to the DUT and destined to internal
        target network
    *   Check entire traffic should only be unequally forwarded between DUT
        port2 and port3

## Config Parameter Coverage

*   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/use-multiple-paths/config/enabled
*   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/use-multiple-paths/ebgp/config/allow-multiple-as
*   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/use-multiple-paths/ebgp/config/maximum-paths
*   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/send-community-type
*   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/use-multiple-paths/ebgp/link-bandwidth-ext-community/config/enabled

## Telemetry Parameter Coverage

*   /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state
*   /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group
*   /network-instances/network-instance/afts/next-hop-groups/next-hop-group[id=<id>]/state
*   /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops

## OpenConfig Path and RPC Coverage

```yaml
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Subscribe:
```
## Required DUT platform

*   FFF - Fixed Form Factor

