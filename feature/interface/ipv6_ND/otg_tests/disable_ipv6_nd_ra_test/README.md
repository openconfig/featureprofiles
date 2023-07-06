# RT-5.x: Interface Loopback mode
[TODO] assign test number 

## Summary

Validate IPv6 ND Router Advertisement (RA) could be completely disabled.

## Procedure
*   Connect DUT porr-1 to OTG port-1
*   Configure IPv6 address on DUT port-1
*   set IPv6 ND RA transmission interval on DUT port-1 to 5 seconds
*   disable IPv6 ND RA transmission on DUT port-1

### TestCase-1: No periodical Router Advertisement

*   Observe over period of 10 seconds if IPv6 ND RA arrives on OTG Port-1 ([rfc4861 section 6.2.1](https://datatracker.ietf.org/doc/html/rfc4861#section-6.2.1))
*   Observe IPv6 ND RA configuration state on DUT Port-1

### TestCase-2: No Router Advertisement in response to Router Solicitation

*   Send IPv6 ND Router Solicitation from OTG Port-1
*   Observe over period of 1 seconds if IPv6 ND RA arrives on OTG Port-1  ([rfc4861 section 6.2.6](https://datatracker.ietf.org/doc/html/rfc4861#section-6.2.6))
*   Observe IPv6 ND RA configuration state on DUT Port-1

## Config Parameter Coverage

*   /interfaces/interface/subinterfaces/subinterface/ipv6/router-advertisement/config/suppress
*   /interfaces/interface/subinterfaces/subinterface/ipv6/router-advertisement/config/interval

## Telemetry Parameter Coverage

*   /interfaces/interface/subinterfaces/subinterface/ipv6/router-advertisement/state/suppress
*   /interfaces/interface/subinterfaces/subinterface/ipv6/router-advertisement/state/interval

## Protocol/RPC Parameter Coverage

None

## Minimum DUT Platform Requirement

vRX
