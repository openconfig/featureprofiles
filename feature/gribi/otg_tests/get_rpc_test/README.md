# TE-5.1: gRIBI Get RPC

## Summary

Validate gRIBI Get RPC.

## Procedure

*   Connect ATE port-1 to DUT port-1 and ATE port-2 to DUT port-2.

*   Connect gRIBI client to DUT referred to as gRIBI-A, along with a second
    client referred to as gRIBI-B - using `PRESERVE` persistence and
    `SINGLE_PRIMARY` mode, with FIB ACK requested. Make gRIBI-A become leader.

*   Inject IPv4Entry cases for 198.51.100.0/26, 198.51.100.64/26,
    198.51.100.128/26 to ATE port-2 via gRIBI-A. Validate entries are installed
    through AFT telemetry.

*   Issue Get RPC from gRIBI-A, ensure that all entries for 198.51.100.0/26,
    198.51.100.64/26, 198.51.100.128/26 are returned. Measure latency of Get
    RPC.

    *   TODO: ensure all AFTEntry in the GetResponse for the IPv4Entry, NHG and
        NH are returned with [fib_status]=`PROGRAMMED`.

*   Issue Get RPC from gRIBI-B, ensure that all entries for 198.51.100.0/26,
    198.51.100.64/26, 198.51.100.128/26 are returned. Measure latency of Get
    RPC.

    *   TODO: ensure all IPv4Entry, NHG and NH are returned with
        [fib_status]=`PROGRAMMED`.

*   Configure static route for 198.51.100.192/64, issue Get from gRIBI-A and
    ensure that only entries for 198.51.100.0/26, 198.51.100.64/26,
    198.51.100.128/26 are returned, with no entry returned for
    198.51.100.192/64.

    *   TODO: ensure all IPEntry, NHG and NH are returned with
        [fib_status]=`PROGRAMMED`.

*   Inject an entry that cannot be installed into the FIB due to an unresolved
    next-hop (203.0.113.0/24 -> unresolved 192.0.2.254/32). Issue a Get RPC from
    gRIBI-A and ensure that the entry for 203.0.113.0/24 is not returned.

    *   TODO: ensure that the IPEntry for 203.0.113.0/24 is returned with
        [fib_status]=`NOT_PROGRAMMED` and [rib_status]=`PROGRAMMED`

[fib_status]: https://github.com/openconfig/gribi/blob/08d53dffce45e942c6e7f07521c58b557984e4b7/v1/proto/service/gribi.proto#L485
[rib_status]: https://github.com/openconfig/gribi/blob/08d53dffce45e942c6e7f07521c58b557984e4b7/v1/proto/service/gribi.proto#L483

## Config Parameter coverage

No additional configuration parameters.

## Telemetry Parameter coverage

No additional telemetry parameters.

## Protocol/RPC Parameter coverage

*   gRIBI
    *   Get

## Minimum DUT platform requirement

vRX if the vendor implementation supports FIB-ACK simulation, otherwise FFF.
