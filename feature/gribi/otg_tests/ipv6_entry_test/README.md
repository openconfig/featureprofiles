# TE-2.3: gRIBI IPv6 Entry

## Summary

Validate IPv6 support in gRIBI.

## Procedure

*   Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2, and ATE port-3 to DUT port-3.
*   Establish gRIBI client connection with DUT, negotiating `RIB_AND_FIB_ACK` as the requested `ack_type` and persistence mode `PRESERVE`. make it become leader. Flush all entries after each case.
*   Using gRIBI Modify RPC install the following IPv6Entry sets, and validate the specified behaviours:
    *   Single IPv6Entry -> NHG -> NH.
        *   Install 2001:db8:a::/64 to NextHopGroup containing one NextHop specified to ATE port-2 (IPv6 address).
        *   Forward packets between ATE port-1 and ATE port-2 (destined to 2001:db8:a::/64) and determine that packets are forwarded successfully.

## Canonical OC

This test adds no new OpenConfig path coverage.

```json
{}
```
