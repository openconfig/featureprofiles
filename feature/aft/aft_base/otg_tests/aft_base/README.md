# AFT-1.1: AFTs Base

## Summary

IPv4/IPv6 unicast routes next hop group and next hop.

## Test Setup

### Generate DUT and ATE Configuration

Configure DUT:port1,port2,port3 for IS-IS session with ATE:port1,port2,port3
*   IS-IS must be level 2 only with wide metric.
*   IS-IS must be point to point.
*   Send 1000 ipv4 and 1000 ipv6 IS-IS prefixes from ATE:port3 to DUT:port3.


Establish eBGP sessions between ATE:port1,port2 and DUT:port1,port2 and another between ATE:port3 and DUT:port3.
*   Configure eBGP over the interface ip.
*   eBGP must be multipath.
*   Advertise 1000 ipv4,ipv6 prefixes from ATE port1,port2 observe received prefixes at DUT.
*   Validate total number of entries of AFT for IPv4 and IPv6.
*   Each prefix must have 2 next hops pointing to ATE port1,port2.
*   Advertise 100 ipv4,ipv6 from ATE port3 observe received prefixes at DUT.

Establish RSVP Sessions between ATE:port3 and SUT:port3.
*   Configure mpls and rsvp sessions.
*   Configure 2 ingress TE tunnels from DUT:port3 to ATE:port3.
*   Tunnel destination is interface ip of ATE:port3.
*   Configure explicit null and ipv6 tunneling.
*   BGP advertised routes from ATE:port3 must be pointing to the 2 tunnels in the DUT.

## Procedure

*   Gnmi set with REPLACE option to push the configuration DUT.
*   ATE configuration must be pushed.

## Verifications

*   BGP 2000 routes advertised from ATE:port1,port2 must have 2 nexthops.
*   IS-IS 2000 routes advertised from ATE:port3 must have one next hop.
*   BGP 100 routes advertised from ATE:port3 must have 2 next hops pointing to tunnels.
*   Use gnmi Subscribe with ON_CHANGE option to /network-instances/network-instance/afts.
*   Verify afts prefix advertised by BGP,ISIS.
*   Verify its next hop group, number of next hop and its interfaces.
*   Verify the number of next hop is same as expected.
*   Verify all other leaves mentioned in the path section.

## AFT-1.1.1: AFT Base Link Down scenario 1

### Procedure

Bring down the link between ATE:port2 and DUT:port2

### Verifications

*   BGP routes advertised from ATE:port1,port2 must have 1 nexthop.
*   IS-IS routes advertised from ATE:port3 must have one next hop.
*   BGP routes advertised from ATE:port3 must have 2 next hops pointing to tunnels.
*   Verify afts prefix advertised by BGP,ISIS.
*   Verify its next hop group, number of next hop and its interfaces.
*   Verify the number of next hop is same as expected.

## AFT-1.1.2: AFT Base Link Down scenario 2

### Procedure

Bring down both links between ATE:port1,port2 and DUT:port1,port2

### Verifications

*   BGP routes advertised from ATE:port1,port2 must be removed from RIB,FIB of the DUT, query results nil.
*   IS-IS routes advertised from ATE:port3 must have one next hop.
*   BGP routes advertised from ATE:port3 must have 2 next hops pointing to tunnels.
*   Verify afts prefix advertised by BGP,ISIS.
*   Verify its next hop group, number of next hop and its interfaces.
*   Verify the number of next hop is same as expected.

## AFT-1.1.3: AFT Base Link Up scenario 1

### Procedure

Bring up link between ATE:port1 and DUT:port1

### Verifications

*   BGP routes advertised from ATE:port1,port2 must have one next hop.
*   IS-IS routes advertised from ATE:port3 must have one next hop.
*   BGP routes advertised from ATE:port3 must have 2 next hops pointing to tunnels.
*   Verify afts prefix advertised by BGP,ISIS.
*   Verify its next hop group, number of next hop and its interfaces.
*   Verify the number of next hop is same as expected.

## AFT-1.1.4: AFT Base Link Up scenario 2

### Procedure

Bring up both link between ATE:port1,port2 and DUT:port1,port2

### Verifications

*   BGP routes advertised from ATE:port1,port2 must have 2 next hops.
*   IS-IS routes advertised from ATE:port3 must have one next hop.
*   BGP routes advertised from ATE:port3 must have 2 next hops pointing to tunnels.
*   Verify afts prefix advertised by BGP,ISIS.
*   Verify its next hop group, number of next hop and its interfaces.
*   Verify the number of next hop is same as expected.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

```yaml
paths:
  ## Config Paths ##

  /network-instances/network-instance/protocols/protocol/bgp/global/config/as:
  /network-instances/network-instance/protocols/protocol/isis/global/config/instance-id:
  

  ## State Paths ##
 
  /network-instances/network-instance/afts/ethernet/mac-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/origin-protocol:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/origin-protocol:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/aft-summaries/ipv4-unicast/protocols/protocol/state/origin-protocol:
  /network-instances/network-instance/afts/aft-summaries/ipv4-unicast/protocols/protocol/state/counters/aft-entries:
  /network-instances/network-instance/afts/aft-summaries/ipv6-unicast/protocols/protocol/state/origin-protocol:
  /network-instances/network-instance/afts/aft-summaries/ipv6-unicast/protocols/protocol/state/counters/aft-entries:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/id:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/index:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/weight:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/backup-next-hop-group:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hops/next-hop/index:
  /network-instances/network-instance/afts/next-hops/next-hop/interface-ref/state/interface:
  /network-instances/network-instance/afts/next-hops/next-hop/interface-ref/state/subinterface:
  /network-instances/network-instance/afts/next-hops/next-hop/state/encapsulate-header:
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address:
  /network-instances/network-instance/afts/next-hops/next-hop/state/mac-address:
  /network-instances/network-instance/afts/next-hops/next-hop/state/origin-protocol:
  /network-instances/network-instance/afts/state-synced/state/ipv4-unicast:
  /network-instances/network-instance/afts/state-synced/state/ipv6-unicast:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/entry-metadata:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group-network-instance:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/origin-network-instance:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/entry-metadata:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group-network-instance:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/origin-network-instance:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/prefix:

rpcs:
  gnmi:
    gNMI.Subscribe:
```

## Control Protocol Coverage

BGP
IS-IS
RSVP
MPLS

## Minimum DUT Platform Requirement

vRX
