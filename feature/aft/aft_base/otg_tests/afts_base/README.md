# AFT-1.1: AFTs Base

## Summary

IPv4/IPv6 unicast routes next hop group and next hop.

## Testbed

* atedut_4.testbed

## Test Setup

### Generate DUT and ATE Configuration

Configure DUT:port 1, port 2, port 3 for IS-IS session with ATE:port 1, port 2, port 3

*   IS-IS must be level 2 only with wide metric.
*   IS-IS must be point to point.
*   Send 1000 ipv4 and 1000 ipv6 IS-IS prefixes from ATE:port 3 to DUT:port 3.

Establish eBGP sessions between ATE:port 1, port 2 and DUT:port 1, port 2 and another
between ATE:port 3 and DUT:port 3.

*   Configure eBGP over the interface ip.
*   eBGP must be multipath.
*   Advertise 1000 ipv4,ipv6 prefixes from ATE port 1, port 2 observe received prefixes at DUT.
*   Validate total number of entries of AFT for IPv4 and IPv6.
*   Each prefix must have 2 next hops pointing to ATE port 1, port 2.
*   Advertise 100 ipv4,ipv6 from ATE port 3 observe received prefixes at DUT.

Establish RSVP Sessions between ATE:port 3 and SUT:port 3.

*   Configure mpls and rsvp sessions.
*   Configure 2 ingress TE tunnels from DUT:port 3 to ATE:port 3.
*   Tunnel destination is interface ip of ATE:port 3.
*   Configure explicit null and ipv6 tunneling.
*   BGP advertised routes from ATE:port 3 must be pointing to the 2 tunnels in the DUT.

### Procedure

*   Use gNMI.Set with REPLACE option to push the Test Setup configuration to the DUT.
*   ATE configuration must be pushed.

### Verifications

*   BGP route advertised from ATE:port 1, port 2 must have 2 nexthops.
*   IS-IS route advertised from ATE:port 3 must have one next hop.
*   BGP route advertised from ATE:port 3 must have 2 next hops pointing to tunnels.
*   Use gnmi Subscribe with ON_CHANGE option to /network-instances/network-instance/afts.
*   For verifying prefix, nexthop groups, next hop use the leaves mentioned in the path section.
*   Verify afts prefix advertised by BGP,ISIS.
*   Verify its next hop group, number of next hop and its interfaces.
*   Verify the number of next hop is same as expected.
*   Verify all other leaves mentioned in the path section.


## AFT-1.1.1: AFT Base Link Down scenario 1

### Procedure

Bring down the link between ATE:port 2 and DUT:port 2 using OTG api.

### Verifications

*   BGP routes advertised from ATE:port 1, port 2 must have 1 nexthop.
*   IS-IS routes advertised from ATE:port 3 must have one next hop.
*   BGP routes advertised from ATE:port 3 must have 2 next hops pointing to tunnels.
*   For verifying prefix, nexthop groups, next hop use the leaves mentioned in the path section.
*   Verify afts prefix advertised by BGP,ISIS.
*   Verify its next hop group, number of next hop and its interfaces.
*   Verify the number of next hop is same as expected.

## AFT-1.1.2: AFT Base Link Down scenario 2

### Procedure

Bring down both links between ATE:port 1, port 2 and DUT:port 1, port 2 using OTG api.

### Verifications

*   BGP routes advertised from ATE:port 1, port 2 must be removed from RIB,FIB of the DUT, query results nil.
*   IS-IS routes advertised from ATE:port 3 must have one next hop.
*   BGP routes advertised from ATE:port 3 must have 2 next hops pointing to tunnels.
*   For verifying prefix, nexthop groups, next hop use the leaves mentioned in the path section.
*   Verify afts prefix advertised by BGP,ISIS.
*   Verify its next hop group, number of next hop and its interfaces.
*   Verify the number of next hop is same as expected.

## AFT-1.1.3: AFT Base Link Up scenario 1

### Procedure

Bring up link between ATE:port 1 and DUT:port 1 using OTG api.

### Verifications

*   BGP routes advertised from ATE:port 1, port 2 must have one next hop.
*   IS-IS routes advertised from ATE:port 3 must have one next hop.
*   BGP routes advertised from ATE:port 3 must have 2 next hops pointing to tunnels.
*   Verify afts prefix advertised by BGP,ISIS.
*   For verifying prefix, nexthop groups, next hop use the leaves mentioned in the path section.
*   Verify its next hop group, number of next hop and its interfaces.
*   Verify the number of next hop is same as expected.

## AFT-1.1.4: AFT Base Link Up scenario 2

### Procedure

Bring up both link between ATE:port 1, port 2 and DUT:port 1, port 2 using OTG api.

### Verifications

*   BGP routes advertised from ATE:port 1, port 2 must have 2 next hops.
*   IS-IS routes advertised from ATE:port 3 must have one next hop.
*   BGP routes advertised from ATE:port 3 must have 2 next hops pointing to tunnels.
*   For verifying prefix, nexthop groups, next hop use the leaves mentioned in the path section.
*   Verify afts prefix advertised by BGP, ISIS.
*   Verify its next hop group, number of next hop and its interfaces.
*   Verify the number of next hop is same as expected.

## Telemetry Parameter Coverage

*   network-instances/network-instance[instance=<instance>]/afts/next-hops/next-hop[hop_id=<hop_id>]
*   network-instances/network-instance[instance=<instance>]/afts/next-hops/next-hop[hop_id=<hop_id>]/index
*   network-instances/network-instance[instance=<instance>]/afts/next-hops/next-hop[hop_id=<hop_id>]/interface-ref/state/interface
*   network-instances/network-instance[instance=<instance>]/afts/next-hops/next-hop[hop_id=<hop_id>]/state/ip-address
*   network-instances/network-instance[instance=<instance>]/afts/next-hop-groups/next-hop-group[group_id=<group_id>]
*   network-instances/network-instance[instance=<instance>]/afts/next-hop-groups/next-hop-group[group_id=<group_id>]/state/id
*   network-instances/network-instance[instance=<instance>]/afts/next-hop-groups/next-hop-group[group_id=<group_id>]/id
*   network-instances/network-instance[instance=<instance>]/afts/next-hop-groups/next-hop-group[group_id=<group_id>]/next-hops/next-hop[hop_id=<hop_id>]/index
*   network-instances/network-instance[instance=<instance>]/afts/next-hop-groups/next-hop-group[group_id=<group_id>]/next-hops/next-hop[hop_id=<hop_id>]/state/weight
*   network-instances/network-instance[instance=<instance>]/afts/ipv4-unicast/ipv4-entry[ipv4_prefix=<ipv4_prefix>]
*   network-instances/network-instance[instance=<instance>]/afts/ipv4-unicast/ipv4-entry[ipv4_prefix=<ipv4_prefix>]/state/prefix
*   network-instances/network-instance[instance=<instance>]/afts/ipv4-unicast/ipv4-entry[ipv4_prefix=<ipv4_prefix>]/prefix
*   network-instances/network-instance[instance=<instance>]/afts/ipv4-unicast/ipv4-entry[ipv4_prefix=<ipv4_prefix>]/state/next-hop-group
*   network-instances/network-instance[instance=<instance>]/afts/ipv6-unicast/ipv6-entry[ipv6_prefix=<ipv6_prefix>]
*   network-instances/network-instance[instance=<instance>]/afts/ipv6-unicast/ipv6-entry[ipv6_prefix=<ipv6_prefix>]/state/prefix
*   network-instances/network-instance[instance=<instance>]/afts/ipv6-unicast/ipv6-entry[ipv6_prefix=<ipv6_prefix>]/prefix
*   network-instances/network-instance[instance=<instance>]/afts/ipv6-unicast/ipv6-entry[ipv6_prefix=<ipv6_prefix>]/state/next-hop-group

## OpenConfig Path and RPC Coverage

```yaml
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Subscribe:
```

## Control Protocol Coverage

BGP
IS-IS
