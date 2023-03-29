# TE-4.1: Base Leader Election

## Summary

Validate Election ID is accepted from a gRIBI client.

## Procedure

*   Connect DUT port-1 to ATE port-1, DUT port-2 to ATE port-2, DUT port-3 to
    ATE port-3. Assign IPv4 addresses to all ports.

*   Establish two gRIBI clients to the DUT (referred to as `gRIBI-A` and
    `gRIBI-B`).

*   Connect `gRIBI-A` to DUT specifying `PRESERVE` persistent mode,
    `SINGLE_PRIMARY` client redundancy in the SessionParameters request. Ensure
    that no error is reported from the gRIBI server.

*   Connect `gRIBI-B` to DUT specifying `PRESERVE` persistent mode,
    `SINGLE_PRIMARY` client redundancy and make it become leader.

*   Add an `IPv4Entry` for `198.51.100.0/24` pointing to ATE port-3 via
    `gRIBI-B`, ensure that the entry is active through AFT telemetry and
    traffic.

*   Add an `IPv4Entry` for `198.51.100.0/24` pointing to ATE port-2 via
    `gRIBI-A`, ensure that the entry is ignored by the DUT.

*   Make `gRIBI-A` become leader, followed by a `ModifyRequest` updating
    `198.51.100.0/24` pointing to ATE port-2, ensure that routing is updated to
    receive packets for `198.51.100.0/24` at ATE port-2.

## Protocol/RPC Parameter Coverage

*   gRIBI
    *   ModifyRequest
        *   SessionParameters:
            *   redundancy
