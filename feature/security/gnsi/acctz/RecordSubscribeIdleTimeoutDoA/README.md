# ACCTZ-6.1 - gNSI.acctz.v1 (Accounting) Test RecordSubscribe Idle Timeout - DoA client

## Summary
Test RecordSubscribe connection termination after idle timeout without making RecordSubscribe idle timeout refresh RPCs

## Procedure

- Establish gNSI connection
- Call gnsi.acctz.v1.Acctz.RecordSubscribe with RecordRequest.timestamp = now()
- Wait at least longer than the idletimeout period (default: 120s)
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
