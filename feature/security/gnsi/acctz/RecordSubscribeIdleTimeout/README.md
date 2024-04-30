# ACCTZ-5.1 - gNSI.acctz.v1 (Accounting) Test RecordSubscribe Idle Timeout - client becomes silent

## Summary
Test RecordSubscribe connection termination after idle timeout following 1 RecordSubscribe RPC and 1 idle timeout refresh RPC

## Procedure

- Establish gNSI connection
- Call gnsi.acctz.v1.Acctz.RecordSubscribe with RecordRequest.timestamp = now()
- Discard received records
- wait until nearly the idletimeout period (default: 120 seconds)
- Call gnsi.acctz.v1.Acctz.RecordSubscribe with RecordRequest.timestamp = now()
- Discard received records
- Wait at least longer than the idletimeout period
- Verify that the DUT closes the gNSI connection at or shortly after the idletimeout period.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

TODO(OCRPC): Record may not be complete

```yaml
paths:
rpcs:
  gnsi:
    acctz.v1.Acctz.RecordSubscribe:
```

## Minimum DUT
vRX
