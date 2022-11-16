# TE-6.2: Route Removal In Non Default VRF

## Summary

Validate that Flush RPC in gRIBI removes routes in non-default VRF as expected.

## Procedure

*   Connect ATE port-1 to DUT port-1, and ATE port-2 to DUT port-2.
*   Create a non-default VRF (VRF-1) that includes DUT port-1.
*   Connect gRIBI client (gRIBI-A) to DUT, with persistence set to `PRESERVE`
    and `SINGLE_PRIMARY` redundancy specified and make it become leader. Connect
    a second client (gRIBI-B) using the same parameters, but with
    `election_id=leader_election_id-1`.
*   Inject an IPv4Entry for 198.51.100.0/24 into VRF-1, with its referenced NHG
    and NH in the default routing-instance pointing to ATE port-2.
    *   Ensure that packets can be forwarded between ATE port-1 and port-2 for
        destinations within 198.51.100.0/24.
    *   Issue Flush RPC from gRIBI-A for VRF-1, ensure that entries are removed
        via validating packet forwarding and telemetry;
    *   Re-inject entry for 198.51.100.0/24 in VRF-1 from gRIBI-A
    *   Issue Flush from gRIBI-B, ensure that entries are not removed via
        validating packet forwarding and telemetry; expect a NOT_PRIMARY RPC
        response error.
    *   Make gRIBI-B become leader.
    *   Issue Flush from gRIBI-B for VRF-1, ensure that entries are removed via
        packet forwarding and telemetry.
    *   Re-inject entry for 198.51.100.0/24 in VRF-1 from gRIBI-B. Re-inject
        entry for 198.51.110.0/24 in default VRF,from gRIBI-B, referencing the
        same NHG and NH pointing to port-2.
    *   Ensure entries’ existence via packet forwarding and telemetry for
        198.51.100.0/24. Ensure entries’ existence via telemetry for
        198.51.110.0/24.
    *   Issue Flush RPC from gRIBI-B for default VRF
    *   Ensure that the gRIBI-B receives
        [`NON_ZERO_REFERENCE_REMAIN`](https://github.com/openconfig/gribi/blob/08d53dffce45e942c6e7f07521c58b557984e4b7/v1/proto/service/gribi.proto#L557).
        Ensure that the IPEntry 198.51.100.0/24 is not removed, by validating
        packet forwarding and telemetry. Ensure that 198.51.110.0/24 has been
        removed by validating telemetry.

## Config Parameter coverage

N/A

## Telemetry Parameter coverage

N/A

## Protocol/RPC Parameter coverage

*   gRIBI
    *   Flush

## Minimum DUT platform requirement

vRX
