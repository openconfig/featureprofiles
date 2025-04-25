# AFT-1.1: AFTs Base

## Summary

IPv4/IPv6 unicast routes next hop group and next hop.

## Testbed

* atedut_2.testbed

## Test Setup

### Generate DUT and ATE Configuration

Configure DUT:port1,port2 for IS-IS session with ATE:port1,port2.

* Let `X` be the number of prefixes to be advertised by BGP. **(User Adjustable Value)**
* Let `Y` be the number of prefixes to be advertised by IS-IS. **(User Adjustable Value)**
* IS-IS must be level 2 only with wide metric.
* IS-IS must be point to point.
* Send `Y` IPv4 and `Y` IPv6 prefixes from ATE:port1 to DUT:port1.

Establish EBGP sessions between ATE:port1,port2 and DUT:port1,port2

* Configure EBGP over the interface IP between ATE:port1,port2 and DUT:port1,port2.
* Advertise `X` IPv4 and `X` IPv6 prefixes from ATE port1,port2.
* Validate the total number of entries of AFT for IPv4 and IPv6. This should be `X + Y` for both address families.
* Each prefix advertised by BGP must have 2 next hops pointing to ATE port1 and ATE port2.
* Each prefix advertised by ISIS must have one next hop pointing to ATE port1.

### Procedure

* Use gNMI.Set with REPLACE option to push the Test Setup configuration to the DUT.
* ATE configuration must be pushed.

### Verifications

* BGP routes advertised from ATE:port1,port2 must have 2 nexthops.
* Use gNMI Subscribe with `ON_CHANGE` option to `/network-instances/network-instance/afts`.
* Verify AFTs prefixes advertised by BGP and ISIS.
* Verify their next hop group, number of next hops, and their interfaces.
* Verify the number of next hops is 2 for BGP advertised prefixes.
* Verify the number of next hop is 1 for ISIS advertised prefixes.
* Verify the prefixes are pointing to the correct egress interface(s).
* Verify all other leaves mentioned in the path section have the data populated correctly.

## AFT-1.1.1: AFT Base Link Down scenario 1

### Procedure

Bring down the link between ATE:port2 and DUT:port2 using OTG API.

### Verifications

* BGP routes advertised from ATE:port1,port2 must have 1 nexthop (pointing to ATE:port1).
* IS-IS routes advertised from ATE:port1 must have one next hop.
* Verify AFTs prefixes advertised by BGP and ISIS.
* Verify their next hop group, number of next hops, and their interfaces.
* Verify the number of next hop per prefix must be 1.

## AFT-1.1.2: AFT Base Link Down scenario 2

### Procedure

Bring down both links between ATE:port1,port2 and DUT:port1,port2 using OTG API.

### Verifications

* BGP routes advertised from ATE:port1,port2 must be removed from RIB and FIB of the DUT (query results should be nil).
* ISIS routes advertised from ATE:port1 must be removed from RIB and FIB of the DUT (query result should be nil).

## AFT-1.1.3: AFT Base Link Up scenario 1

### Procedure

Bring up the link between ATE:port1 and DUT:port1 using OTG API.

### Verifications

* BGP routes advertised from ATE:port1,port2 must have one next hop (pointing to ATE:port1).
* IS-IS routes advertised from ATE:port1 must have one next hop.
* Verify AFTs prefixes advertised by BGP and ISIS.
* Verify their next hop group, number of next hops, and their interfaces.
* Verify the number of next hop per prefix is 1.

## AFT-1.1.4: AFT Base Link Up scenario 2

### Procedure

Bring up both links between ATE:port1,port2 and DUT:port1,port2 using OTG API.

### Verifications

* BGP routes advertised from ATE:port1,port2 must have 2 next hops.
* IS-IS routes advertised from ATE:port1 must have one next hop.
* Verify AFTs prefixes advertised by BGP and ISIS.
* Verify their next hop group, number of next hops, and their interfaces.


## OpenConfig Path and RPC Coverage

The below YAML defines the OC paths intended to be covered by this test.
OC paths used for test setup are not listed here.

```yaml
paths:
  ## Config Paths ##


  ## State Paths ##

  /network-instances/network-instance/afts/ethernet/mac-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/origin-protocol:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/origin-protocol:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/aft-summaries/ipv4-unicast/protocols/protocol/state/origin-protocol:
  /network-instances/network-instance/afts/aft-summaries/ipv6-unicast/protocols/protocol/state/origin-protocol:
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