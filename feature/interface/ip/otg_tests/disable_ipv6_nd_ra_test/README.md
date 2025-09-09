# RT-5.9: Disable IPv6 ND Router Arvetisment

## Summary

Validate IPv6 ND Router Advertisement (RA) could be completely disabled.

## Procedure
*   Connect DUT port-1 to OTG port-1
*   Configure IPv6 address on DUT port-1
*   Set IPv6 ND RA transmission interval on DUT port-1 to 5 seconds
*   Disable IPv6 ND RA transmission on DUT port-1

### TestCase-1: No periodical Router Advertisement

*   Verify over period of 10 seconds that IPv6 ND RA **doesn't** arrives on OTG Port-1 ([rfc4861 section 6.2.1](https://datatracker.ietf.org/doc/html/rfc4861#section-6.2.1))
*   Observe IPv6 ND RA configuration state on DUT Port-1

### TestCase-2: No Router Advertisement in response to Router Solicitation

*   Send IPv6 ND Router Solicitation from OTG Port-1
*   Verify over period of 1 seconds that IPv6 ND RA **doesn't** arrives on OTG Port-1  ([rfc4861 section 6.2.6](https://datatracker.ietf.org/doc/html/rfc4861#section-6.2.6))
*   Observe IPv6 ND RA configuration state on DUT Port-1

## Config Parameter Coverage

*   /interfaces/interface/subinterfaces/subinterface/ipv6/router-advertisement/config/interval
*   /interfaces/interface/subinterfaces/subinterface/ipv6/router-advertisement/config/enable: FALSE
*   /interfaces/interface/subinterfaces/subinterface/ipv6/router-advertisement/config/mode:   ALL (default)
  
## Telemetry Parameter Coverage

*   /interfaces/interface/subinterfaces/subinterface/ipv6/router-advertisement/state/interval
*   /interfaces/interface/subinterfaces/subinterface/ipv6/router-advertisement/config/enable: FALSE
*   /interfaces/interface/subinterfaces/subinterface/ipv6/router-advertisement/config/mode:   ALL (default)

## Protocol/RPC Parameter Coverage

None

## Minimum DUT Platform Requirement

vRX


## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml
paths:
## Config paths
   /interfaces/interface/subinterfaces/subinterface/ipv6/router-advertisement/config/interval:
   /interfaces/interface/subinterfaces/subinterface/ipv6/router-advertisement/config/enable:
   /interfaces/interface/subinterfaces/subinterface/ipv6/router-advertisement/config/mode:
  ##State paths
   /interfaces/interface/subinterfaces/subinterface/ipv6/router-advertisement/state/interval:

rpcs:
  gnmi:
    gNMI.Set:
      Replace:
```