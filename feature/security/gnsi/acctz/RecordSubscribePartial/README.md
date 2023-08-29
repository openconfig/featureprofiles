# gNSI.acctz.v1 (Accounting) Test Record Subscribe Partial

Test RecordSubscribe for records since a non-zero timestamp

- Establish gNSI connection
- Call gnsi.acctz.v1.Acctz.RecordSubscribe with RecordRequest.timestamp = to the timestamp retained in the [Record Subscribe Full](../RecordSubscribeFull) test.
- Verify, as in the [Record Subscribe Full](../RecordSubscribeFull) test, that accurate accounting records are returned for the second and subsequent commands run in that test, and that a record is not returned for the first command (ie: with the same timestamp as in the request).
- Retain a copy of the Record.timestamp of the last returned record for the [Record Subscribe Non-gRPC](../RecordSubscribeNongrpc) test.

## Config Parameter
### Prefix:
/gnsi/acctz/v1/Acctz/RecordSubscribe

### Parameter:
RecordRequest.timestamp!=0
RecordResponse.service_request = GrpcService

## Telemetry Coverage
### Prefix:
Accounting does not currently support any telemetry; see https://github.com/openconfig/gnsi/issues/97 where it might become /system/aaa/acctz/XXX

## Protocol/RPC
gnsi.acctz.v1

## Minimum DUT
vRX
