# RT-4.10: AFTs Route Summary

## Summary

IPv4/IPv6 unicast AFTs route summary for ISIS and BGP protocol

## Procedure

Configure DUT:port1 for an IS-IS session with ATE:port1
*   Validate total number of entries of AFT for IPv4 and IPv6

Establish eBGP sessions between ATE:port1 and DUT:port1 and another between ATE:port2 and DUT:port2
*   Configure Route-policy under BGP peer-group address-family
*   Advertise prefixes from ATE port-1, observe received prefixes at ATE port-2 for IPv4 and IPv6
*   Validate total number of entries of AFT for IPv4 and IPv6

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

```yaml
paths:
  ## Config Paths ##

  ## State Paths ##
  /network-instances/network-instance/afts/aft-summaries/ipv4-unicast/protocols/protocol/state/counters/aft-entries:
  /network-instances/network-instance/afts/aft-summaries/ipv6-unicast/protocols/protocol/state/counters/aft-entries:

rpcs:
  gnmi:
    gNMI.Subscribe:
```

## Control Protocol Coverage

BGP
IS-IS

## Minimum DUT Platform Requirement

vRX
