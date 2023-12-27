# ACCTZ-4.1 - gNSI.acctz.v1 (Accounting) Test Record Payload Truncation

## Summary
Test how large payload is handled.

## Procedure

1.  Use gNMI SET to push a large configuration (e.g. contains 100K static routes).
2.  Establish gNSI connection to the DUT.
    1.  Call `gnsi.acctz.v1.Acctz.RecordSubscribe` with `RecordRequest.timestamp = T1`. T1 should be timestamp that covers the above gNMI SET action.
    2.  Verify that the gNMIS SET record are returned either with full payload, or with `payload_istruncated` set to `true`.

## Telemetry Coverage
Accounting does not currently support any telemetry; see https://github.com/openconfig/gnsi/issues/97 where it might become /system/aaa/acctz/XXX

## Protocol/RPC
gnsi.acctz.v1

## Minimum DUT
vRX