# RT-1.63: Internal BGP multipath ECMP

## Summary

Validate internal BGP in multipath scenario

## Testbed type

[TESTBED_DUT_ATE_4LINKS](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Setup iBGP

*   Connect DUT port 1, 2, 3 and 4 to ATE port 1, 2, 3 and 4 respectively
*   Configure IPv4/IPv6 addresses on the interfaces
*   Establish IPv4 and IPv6 eBGP sessions, with IPv4 unicast and IPv6 unicast NLRI suppore respectivly,  between:
    *   ATE port-1 and DUT port-1
    *   Enable an Accept-route all import-policy/export-policy for eBGP session
    under the neighbor AFI/SAFI
*   Establish IPv4 and IPv6 iBGP sessions, with IPv4 unicast and IPv6 unicast NLRI suppore respectivly,  between:
    *   ATE port-2 and DUT port-2
    *   ATE port-3 and DUT port-3
    *   ATE port-4 and DUT port-4
*   Create an IPv4 internal target network attached to ATE port 2, 3 and 4
*   Create an IPv6 internal target network attached to ATE port 2, 3 and 4

### Tests

*   RT-1.63.1: Verify best path

    *   Configure ATE devices(ports) 2-3 on same AS as DUT
    *   Advertise equal cost paths from 3 interfaces of ATE of same AS as DUT for IPv4 and IPv6 target network.
    *   Check entries in FIB for advertised prefix, it should only have 1 entry per prefix
        *   /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops
    *   Initiate traffic from ATE port-1 to the DUT and destined to internal
        target networks
    *   Check entire IPv4 traffic should only be forwarded by one of DUT port2, port3
        or port4
    *   Check entire IPv6 traffic should only be forwarded by one of DUT port2, port3
        or port4

*   RT-1.63.2: Enforcing multipath hence ECMP scope to only one peer AS

    *   Configure ATE devices(ports) on same AS 2-3 on same AS as DUT
    *   Enable multipath on DUT and set maximum-paths limit to 2
        *   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/use-multiple-paths/config/enabled
        *   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/use-multiple-paths/ibgp/config/maximum-paths
    *   Advertise equal cost paths from 3 interfaces of ATE of same AS as DUT for IPv4 and IPv6 target network.
    *   Check entries in FIB for advertised prefix, it should only have 2
        entries
        *   /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops
    *   Initiate traffic from ATE port-1 to the DUT and destined to internal
        target network
    *   Check entire IPv4 traffic should only be equally forwarded by any two among DUT
        port2, port3 or port4
    *   Check entire IPv6 traffic should only be equally forwarded by any two among DUT
        port2, port3 or port4

## Config Parameter Coverage

*   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/use-multiple-paths/config/enabled
*   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/use-multiple-paths/ibgp/config/maximum-paths

## Telemetry Parameter Coverage

*   /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state
*   /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group
*   /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state
*   /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group
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

