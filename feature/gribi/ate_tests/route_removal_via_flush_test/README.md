# TE-6.1: Route Removal via Flush

## Summary

Validate that Flush RPC in gRIBI removes routes as expected.

## Procedure

*   Connect ATE port-1 to DUT port-1, and ATE port-2 to DUT port-2.
*   Connect gRIBI client (gRIBI-A) with `election_id=10` to DUT, with
    persistence set to PRESERVE and SINGLE_PRIMARY redundancy specified. Connect
    a second client (gRIBI-B) using the same parameters, but with
    `election_id=9` .
*   Inject an entry into the default network instance pointing to ATE port-2.
    *   Ensure that traffic can be forwarded between ATE port-1 and ATE port-2
        for destinations in 198.51.100.0/24.
    *   Issue Flush RPC from gRIBI-A, and ensure that packets can no longer be
        forwarded for destinations in 192.0.2.0/2. Ensure that AFT telemetry
        reflects the entry being removed.
    *   Re-inject entry for 198.51.100.0/24 from gRIBI-A.
    *   Issue Flush RPC from gRIBI-B. Ensure that entries are not removed via
        packet forwarding and AFT telemetry; expect a NOT_PRIMARY RPC response
        error.
    *   Increase `gRIBI-B`’s `election_id` to 11 by sending a `ModifyRequest`
        with `election_id=11`.
    *   Issue Flush from gRIBI-B, ensure that entries are removed via packet
        forwarding and telemetry.
*   Inject an IPv4Entry for 198.51.100.0/24 into a non default L3VRF named
    VRF-1.
*   Use a VRF policy to match on 198.51.100.0/24 and redirect them to VRF-1
    ([RT-3] for reference)
    *   Ensure that packets can be forwarded between ATE port-1 and port-2 for
        destinations within 198.51.100.0/24.
    *   Issue Flush RPC from gRIBI-B for VRF-1, ensure that packets can no
        longer be forwarded for destinations within 198.51.100.0/24.
    *   Re-inject entry for 198.51.100.0/24 in VRF-1 from gRIBI-B Issue Flush
        from gRIBI-A, ensure that entries are not removed via forwarding and
        telemetry; expect a NOT_PRIMARY RPC response error.
    *   Increase `gRIBI-A`’s `election_id` to 12 by sending a `ModifyRequest`
        with `election_id=12`
    *   Issue Flush from gRIBI-A for VRF-1, ensure that entries are removed via
        packet forwarding and telemetry.

## Config Parameter coverage

N/A

## Telemetry Parameter coverage

N/A

## Protocol/RPC Parameter coverage

*   gRIBI
    *   Flush

## Minimum DUT platform requirement

vRX
