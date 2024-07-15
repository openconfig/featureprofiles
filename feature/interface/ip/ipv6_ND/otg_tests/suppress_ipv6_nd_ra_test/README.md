# RT-5.11: Suppress IPv6 ND Router Advertisement

## Summary

Validate IPv6 ND Router Advertisement (RA) is suppresed. No periodic RA are sent.

## Procedure
*   Connect DUT port-1 to OTG port-1
*   Configure IPv6 address on DUT port-1
*   Enable IPv6 ND RA suppression on DUT port-1

### RT-5.11.1: No periodical Router Advertisement are sent 

*   Verify over period of 10 seconds that IPv6 ND RA **doesn't** arrives on OTG Port-1 ([rfc4861 section 6.2.1](https://datatracker.ietf.org/doc/html/rfc4861#section-6.2.1))

### RT-5.11.2: Router Advertisement response is sent to Router Solicitation  

*   Send IPv6 ND Router Solicitation from OTG Port-1
*   Verify over period of 1 seconds that IPv6 ND RA does arrives on OTG Port-1 ([rfc4861 section 6.2.6](https://datatracker.ietf.org/doc/html/rfc4861#section-6.2.6))

## OpenConfig Path and RPC Coverage

```yaml
paths:
    /interfaces/interface/subinterfaces/subinterface/ipv6/router-advertisement/config/interval:
    /interfaces/interface/subinterfaces/subinterface/ipv6/router-advertisement/config/suppress:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

* FFF