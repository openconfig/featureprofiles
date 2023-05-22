# TE-3.5: Ordering: ACK Received

## Summary

Ensure that acknowledgements are sent as is expected by gRIBI controller.

## Procedure

*   Configure ATE port-1 connected to DUT port-1, and ATE port-2 to DUT port-2.
*   Connect to the gRIBI server running on DUT, negotiating `RIB_AND_FIB_ACK` as
    the requested `ack_type` and persistence mode `PRESERVE`. Make it become
    leader. Flush all entries after each case.
*   Install the following entries and determine whether the expected result is
    observed:
    *   A `NextHopGroup` referencing a `NextHop` is responded to with FIB ACK,
        and is reported through the AFT telemetry.
    *   A single `ModifyRequest` with the following ordered operations is
        responded to with an error:
        *   An `AFTOperation` containing an `IPv4Entry` referencing
            `NextHopGroup` 10.
        *   An `AFTOperation` containing a `NextHopGroup id=10`.
    *   A single `ModifyRequest` with the following ordered operations is
        installed (verified through telemetry and traffic):
        *   An `AFTOperation` containing a `NextHopGroup` 10 pointing to a
            `NextHop` to ATE port-2.
        *   An `AFTOperation` containing a `IPv4Entry` referencing
            `NextHopGroup` 10.
    *   A single `ModifyRequest` with the following ordered operations is
        installed (verified through telemetry and traffic):
        *   An AFT entry adding `IPv4Entry 203.0.113.0/24`.
        *   An AFT entry deleting `IPv4Entry 203.0.113.0/24`.
        *   An AFT entry adding `IPv4Entry 203.0.113.0/24`.

If the device supports it, repeat this test with gRIBI client persistence mode
`DELETE` without flushing entries between cases.

## Config Parameter coverage

N/A

## Telemetry Parameter coverage

N/A

## Protocol/RPC Parameter coverage

*   gRIBI
    *   ModifyRequest:
        *   SessionParameters:
            *   ack_type
