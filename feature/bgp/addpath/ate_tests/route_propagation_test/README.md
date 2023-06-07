# RT-1.3: BGP Route Propagation

## Summary

BGP Route Propagation

## Procedure

Establish eBGP sessions between:

*   ATE port-1 and DUT port-1
*   ATE port-2 and DUT port-2
*   Configure Route-policy under BGP peer-group address-family

For IPv4 and IPv6:

*   Advertise prefixes from ATE port-1, observe received prefixes at ATE port-2.
*   TODO: Specify default accept for received prefixes on DUT.
*   TODO: Specify table based neighbor configuration to cover - validating the
    supported capabilities from the DUT.
    *   TODO: MRAI (minimum route advertisement interval), ensuring routes are
        advertised within specified time.
    *   IPv4 routes with an IPv6 next-hop when negotiating RFC5549 - validating
        that routes are accepted and advertised with the specified values.
    *   TODO: With ADD-PATH enabled, ensure that multiple routes are accepted
        from a neighbor when advertised with individual path IDs, and that these
        routes are advertised to ATE port-2.

## Config Parameter Coverage

For prefix:
/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor

Parameters:

*   afi-safis/afi-safi/add-paths/config/receive
*   afi-safis/afi-safi/add-paths/config/send
*   afi-safis/afi-safi/add-paths/config/send-max

## Telemetry Parameter Coverage

/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/supported-capabilities

## Protocol/RPC Parameter Coverage

BGP

*   OPEN
    *   Capabilities (Extended nexthop encoding capability (5), ADD-PATH (69))
*   UPDATE
    *   Extended NLRI Encodings (RFC7911)
    *   Nexthop AFI (RFC5549)
