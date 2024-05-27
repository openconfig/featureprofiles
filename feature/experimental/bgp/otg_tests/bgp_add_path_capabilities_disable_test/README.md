# RT-1.53: BGP Add Path send/receive disable 

## Summary

BGP Add Path send/receive disable

## Procedure

*   Establish BGP connections between:.
    *   DUT Port1 (AS 65501) ---eBGP 1--- ATE Port1 (AS 65501)
    *   DUT Port2 (AS 65501) ---eBGP 2--- ATE Port2 (AS 65502)
*   Configure both DUT and ATE with add-path send and receive capabilities enabled for both IPv4 and IPv6.
*   Validate Initial Add-Path Capability: 
    *   Verify the telemetry path output to confirm the Add-Path capabilities for DUT.
*   Disable Add-Path Send: On DUT, disable the Add-Path send capability for the neighbor on ATE Port-2 for IPv4 and IPv6.
*   Verify the telemetry path output to confirm the Add-Path send capability for DUT is disabled for both ipv4 and ipv6 for the neighbor on ATE Port-2.
*   Disable Add-Path Receive: On DUT, disable the Add-Path receive capability for the neighbor on ATE Port-1.
*   Verify the telemetry path output to confirm the Add-Path receive capability for DUT is disabled for both ipv4 and ipv6 for the neighbor on ATE Port-1.

## Config Parameter coverage

*   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/config/receive
*   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/config/send
*   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/config/receive
*   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/config/send

## Telemetry Parameter coverage

*   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/state/receive
*   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/state/send
*   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/state/receive
*   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/state/send
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/supported-capabilities

## Protocol/RPC Parameter coverage

*   BGP

    *   OPEN
        *   Capabilities (Extended nexthop encoding capability (5), ADD-PATH (69))
    *   UPDATE
        *   Extended NLRI Encodings (RFC7911)
        *   Nexthop AFI (RFC5549)
