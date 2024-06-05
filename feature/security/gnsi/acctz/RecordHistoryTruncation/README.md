# ACCTZ-4.1 - gNSI.acctz.v1 (Accounting) Test Record History Truncation

## Summary
Test Record Response Truncation boolean is set

## Procedure
- For an supported service class in gnsi.acctz.v1.CommandService.CmdServiceType:
	- Run a few commands
	- disconnect
- Establish gNSI connection to the DUT.
- Call gnsi.acctz.v1.Acctz.RecordSubscribe with RecordRequest.timestamp = (openconfig-system.system-global-state.boot-time - 24 hours)
- Verify that RecordResponse.history_istruncated = true.  It should be true because there should be no records in the history equal to nor pre-dating this RecordRequest.timestamp.

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
      "RecordResponse.service_request = CommandService": true
```

## Minimum DUT
vRX
