# TE-2.1: gRIBI IPv4 Entry

## Summary

Validate IPv4 support in gRIBI.

## Procedure

*   Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2, and ATE port-3
    to DUT port-3.
*   Establish gRIBI client connection with DUT, negotiating `RIB_AND_FIB_ACK` as
    the requested `ack_type` and persistence mode `PRESERVE`. Flush all entries
    after each case.
*   Using gRIBI Modify RPC install the following IPv4Entry sets, and validate
    the specified behaviours:
    *   Single IPv4Entry -> NHG -> NH.
        *   Install 198.51.100.0/24 to NextHopGroup containing one NextHop
            specified to ATE port-2.
        *   Forward packets between ATE port-1 and ATE port-2 (destined to
            198.51.100.0/24 ) and determine that packets are forwarded
            successfully:
    *   Single IPv4Entry -> NHG -> multiple NHs.
        *   Install 198.51.100.0/24 to NextHopGroup containing two NextHop
            entries specified to ATE ports 2 and 3.
        *   Validate that packets forwarded between ATE ports 1 and (2 and 3),
            ensuring that traffic is forwarded.
    *   Single IPv4Entry -> NHG -> non-existent NH.
        *   Send a Modify() containing 2 AFTOperations that install
            198.51.100.0/24 to NextHopGroup containing next-hops that do not
            exist. Validate that FAILED error is received for all the 2
            operations. Ensure that traffic to 198.51.100.0/24 is blackholed.
    *   Single IPv4Entry -> NHG -> NH with down interface
        *   Install 198.51.100.0/24 to NextHopGroup containing a NextHop that
            references (interface\_ref) a down interface and override the
            destination MAC (mac\_address), ensure that `FIB_PROGRAMMED` is
            returned.

If the device supports it, repeat this test with gRIBI client persistence mode
`DELETE` without flushing entries between cases.

## Config Parameter coverage

N/A

## Telemetry Parameter coverage

N/A

## Protocol/RPC Parameter coverage

*   gRIBI
    *   Modify()
        *   ModifyRequest:
            *   AFTOperation:
                *   id
                *   network_instance
                *   op
                *   Ipv4
                    *   Ipv4EntryKey: prefix
                    *   Ipv4Entry: next_hop_group
                *   next_hop_group
                    *   NextHopGroupKey: id
                    *   NextHopGroup: next_hop
                *   next_hop
                    *   NextHopKey: id
                    *   NextHop:
                        *   ip_address
        *   ModifyResponse:
            *   AFTResult:
                *   id
                *   status
