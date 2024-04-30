# ACCTZ-2.1 - gNSI.acctz.v1 (Accounting) Test Record Subscribe Partial

## Summary
Test RecordSubscribe for records since a non-zero timestamp

## Procedure
- For each of the supported service types in gnsi.acctz.v1.GrpcService.GrpcServiceType:
	- Alternate connecting to the IPv4 and Ipv6 addresses of the DUT, recording the local and remote IP addresses and port numbers,
	- Call a few RPCs that will generate accounting records and that, by authorization configuration, should be permitted and a few that should be denied, and some that include arbitrary arguments (eg: interface description), pause for 1 second after the first RPC of this test to ensure its timestamp differs from subsequent RPCs.
- Establish gNSI connection to the DUT.
- Call gnsi.acctz.v1.Acctz.RecordSubscribe with RecordRequest.timestamp = 0, record the timestamp of the first record.
- Call gnsi.acctz.v1.Acctz.RecordSubscribe with RecordRequest.timestamp = to the timestamp retained in the previous step.
- Verify, as in the [ACCTZ-1.1 - Record Subscribe Full](../RecordSubscribeFull) test, that accurate accounting records are returned for the second and subsequent commands run in that test, and that a record is NOT returned for the first command (ie: with the same timestamp as in the request).

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

TODO(OCRPC): Record may not be complete

```yaml
paths:
    ### Prefix:
    # Accounting does not currently support any telemetry; see https://github.com/openconfig/gnsi/issues/97 where it might become /system/aaa/acctz/XXX
rpcs:
  gnsi:
    acctz.v1.Acctz.RecordSubscribe:
      "RecordRequest.timestamp!=0": true
      "RecordResponse.service_request = GrpcService": true
```

## Minimum DUT
vRX
