# RT-1.3: BGP Route Propagation

## Summary

BGP Route Propagation

## Procedure

Establish eBGP sessions between:

*   ATE port-1 and DUT port-1
*   ATE port-2 and DUT port-2

Initial Add-Path Configuration:

*   DUT & ATE: Configure both DUT and ATE to enable add-path send and receive capabilities for both IPv4 and IPv6 address families.
*   Verification (Telemetry): Verify that the DUT's telemetry output reflects the enabled Add-Path capabilities.

Route Policy Configuration:

*   DUT: Configure Route-policy under BGP peer-group address-family and specify default accept for received prefixes on DUT.
*   ATE Port 1: Advertise both IPv4 and IPv6 prefixes.

Route Propagation and Specific Capabilities:

*   MRAI:
    *   DUT: Configure the Minimum Route Advertisement Interval (MRAI) for desired behavior.
    *   ATE Port 2: Verify received routes adhere to the MRAI timing.

*   RFC5549:
    *   DUT: Enable RFC5549 support.
    *   ATE Port 1: Advertise IPv4 routes with IPv6 next-hops.
    *   ATE Port 2: Validate correct acceptance and propagation of routes with IPv6 next-hops.

*   Add-Path (Initial State):
    *   ATE Port 1: Advertise multiple routes with distinct path IDs for the same prefix.
    *   ATE Port 2: Confirm that all advertised routes are accepted and propagated by the DUT due to the initially enabled Add-Path.

*   Disabling Add-Path Send:
    *   DUT: Disable Add-Path send for the neighbor connected to ATE Port 2 for both IPv4 and IPv6.
    *   Verification (Telemetry): Confirm that the DUT's telemetry reflects the disabled Add-Path send status.
    *   ATE Port 1: Readvertise multiple paths.
    *   ATE Port 2: Verify that only a single best path is received by ATE Port 2 due to disabled Add-Path send on the DUT.

*   Disabling Add-Path Receive:

    *   DUT: Disable Add-Path receive for the neighbor connected to ATE Port 1 for both IPv4 and IPv6.
    *   Verification (Telemetry): Confirm the disabled Add-Path receive status in telemetry.
    *   ATE Port 1: Readvertise multiple paths.
    *   ATE Port 2: Verify that the DUT does not accept multiple paths and only a single path is propagated.

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
