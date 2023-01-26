# TE-1.2: My Station MAC

## Summary

Ensure my MAC entries installed on the DUT are honored and used for routing.

## Procedure

*   Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2.
*   Configure My Station MAC per the config shown below.
*   Configure Static ARP on DUT for ATE port-2 so the destination MAC to ATE Port-2 is also the My Station MAC.
*   Install static route for traffic to flow from ATE port-1 to ATE port-2.
*   Verify traffic to flow from ATE port-1 to ATE port-2 such that:
    *   The destination MAC for the flow source is set to My Station MAC.
    *   The destination MAC received at ATE port-2 is also the My Station MAC.

## Config Parameter Coverage

*   My Station MAC: /sytem/mac-address/config/routing-mac
*   Static ARP:  
    *   /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip
    *   /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length
    *   /interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/config/ip
    *   /interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/config/link-layer-address
    *   /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip
    *   /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length
    *   /interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/config/ip
    *   /interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/config/link-layer-address

## Telemetry Parameter Coverage

*   No additional telemetry but ensure that the AFT and Static ARP telemetry is validated.

## Protocol/RPC Parameter Coverage

*  For gRIBI use the same parameters as in TE-2.1.