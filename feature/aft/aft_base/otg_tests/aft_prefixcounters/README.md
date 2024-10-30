# AFT-2.1: AFTs Prefix Counters

## Summary

IPv4/IPv6 prefix counters

## Testbed

* atedut_12.binding

## Test Setup

### Generate DUT and ATE Configuration

Configure DUT:port1 for IS-IS session with ATE:port1
*   IS-IS must be level 2 only with wide metric.
*   IS-IS must be point to point.
*   Send 1000 ipv4 and 1000 ipv6 IS-IS prefixes from ATE:port1 to DUT:port1.

Establish eBGP sessions between ATE:port1 and DUT:port1.
*   Configure eBGP over the interface ip.
*   Advertise 1000 ipv4,ipv6 prefixes from ATE port1 observe received prefixes at DUT.

### Procedure

*   Gnmi set with REPLACE option to push the configuration DUT.
*   ATE configuration must be pushed.

### verifications

*   BGP routes advertised from ATE:port1 must have 1 nexthop.
*   IS-IS routes advertised from ATE:port1 must have one next hop.
*   Use gnmi Subscribe with ON_CHANGE option to /network-instances/network-instance/afts.
*   Verify afts prefix entries using the following paths with in a timeout of 30s.

/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix,
/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix



## AFT-2.1.1: AFT Prefix Counters ipv4 packets forwarded, ipv4 octets forwarded IS-IS route.

### Procedure

From ATE:port2 send 10000 packets to one of the ipv4 prefix advertise by IS-IS.

### Verifications

*  Before the traffic measure the initial counter value.
*  After the traffic measure the final counter value.
*  The difference between final and initial value must match with the counter value in ATE then the test is marked as passed.
*  Verify afts ipv4 forwarded packets and ipv4 forwarded octets counter entries using the path mentioned in the paths section of this test plan.

## AFT-2.1.2: AFT Prefix Counters ipv4 packets forwarded, ipv4 octets forwarded BGP route.

### Procedure

From ATE:port2 send 10000 packets to one of the ipv4 prefix advertise by BGP.

### Verifications

*  Before the traffic measure the initial counter value.
*  After the traffic measure the final counter value.
*  The difference between final and initial value must match with the counter value in ATE, then the test is marked as passed.
*  Verify afts ipv4 forwarded packets and ipv4 forwarded octets counter entries using the path mentioned in the paths section of this test plan.


## AFT-2.1.3: AFT Prefix Counters ipv6 packets forwarded, ipv6 octets forwarded IS-IS route.

### Procedure

From ATE:port2 send 10000 packets to one of the ipv6 prefix advertise by IS-IS.

### Verifications

*  Before the traffic measure the initial counter value.
*  After the traffic measure the final counter value.
*  The difference between final and initial value must match with the counter value in ATE, then the test is marked as passed.
*  Verify afts ipv6 forwarded packets and ipv6 forwarded octets counter entries using the path mentioned in the paths section of this test plan.

## AFT-2.1.4: AFT Prefix Counters ipv6 packets forwarded, ipv6 octets forwarded BGP route.

### Procedure

From ATE:port2 send 10000 packets to one of the ipv6 prefix advertise by BGP.

### Verifications

*  Before the traffic measure the initial counter value.
*  After the traffic measure the final counter value.
*  The difference between final and initial value must match with the counter value in ATE, then the test is marked as passed.
*  Verify afts ipv6 forwarded packets and ipv6 forwarded octets counter entries using the path mentioned in the paths section of this test plan.

## AFT-2.1.5: AFT Prefix Counters withdraw the ipv4 prefix.

### Procedure

*  From ATE:port1 withdraw some prefixes of BGP and IS-IS.
*  Send 10000 packets from ATE:port2 to DUT:port2 for one of the withdrawn ipv4 prefix.
*  The traffic must blackhole.

### Verifications

* The counters must not send incremental value as the prefix is not present in RIB/FIB. The test fails if the counter shows incremental values.
* Verify afts ipv4 forwarded packet counter entries using the path mentioned in the paths section of this test plan.

## AFT-2.1.6: AFT Prefix Counters add the ipv4 prefix back.

### Procedure

*  From ATE:port1 add the prefixes of BGP and IS-IS back.
*  Send 10000 packets from ATE:port2 to DUT:port2 for one of the added ipv4 prefix.
*  The traffic must flow end to end.

### Verifications

*  Before the traffic measure the initial counter value.
*  After the traffic measure the final counter value.
*  The difference between final and initial value must match with the counter value in ATE, then the test is marked as passed.
*  Verify afts counter entries using the path mentioned in the paths section of this test plan.

## AFT-2.1.7: AFT Prefix Counters withdraw the ipv6 prefix.

### Procedure

*  From ATE:port1 withdraw some prefixes of BGP and IS-IS.
*  Send 10000 packets from ATE:port2 to DUT:port2 for one of the withdrawn ipv6 prefix.
*  The traffic must blackhole.

### Verifications

* The counters must not send incremental value as the prefix is not present in RIB/FIB. The test fails if the counter shows incremental values.
*  Verify afts counter entries using the path mentioned in the paths section of this test plan.

## AFT-2.1.8: AFT Prefix Counters add the ipv6 prefix back.

### Procedure

*  From ATE:port1 add the prefixes of BGP and IS-IS back.
*  Send 10000 packets from ATE:port2 to DUT:port2 for one of the added ipv6 prefix.
*  The traffic must flow end to end.

### Verifications

*  Before the traffic measure the initial counter value.
*  After the traffic measure the final counter value.
*  The difference between final and initial value must match with the counter value in ATE, then the test is marked as passed.
*  Verify afts counter entries using the path mentioned in the paths section of this test plan.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

```yaml
paths:
  ## Config Paths ##

  ## State Paths ##
 
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/counters/octets-forwarded:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/counters/packets-forwarded:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/counters/octets-forwarded:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/counters/packets-forwarded:
  

rpcs:
  gnmi:
    gNMI.Subscribe:
```

## Control Protocol Coverage

BGP
IS-IS

## Minimum DUT Platform Requirement

vRX