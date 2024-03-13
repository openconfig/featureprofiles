# ACCTZ-4.2 - gNSI.acctz.v1 (Accounting) Test Record Payload Truncation

## Summary

Test how large payload is handled.

## Procedure

1.  Call a method with a payload that will exceed the maximum payload supported by the implementation for `CommandService.{cmd,cmd_args}` or`GrpcService.payloads`` or both, such as adding a large number of static routes. If the implementation supports configuration of this limit, it may be configured to artificially reduce the limit for easier testing.
2.  Establish gNSI connection to the DUT.
    1.  Call `gnsi.acctz.v1.Acctz.RecordSubscribe` with `RecordRequest.timestamp = T1`. T1 should be timestamp that covers the above gNMI SET action.
    2.  Verify that The appropriate boolean should be set; one of `CommandService.{cmd_istruncated,cmd_args_istruncated}` or `GrpcService.payload_istruncated`.
    3.  If an RPC, the contents of the payload field(s) is structured and must remain syntactically parsable.

## Telemetry Coverage

Accounting does not currently support any telemetry; see <https://github.com/openconfig/gnsi/issues/97> where it might become /system/aaa/acctz/XXX

## Protocol/RPC

gnsi.acctz.v1

## Minimum DUT

vRX
