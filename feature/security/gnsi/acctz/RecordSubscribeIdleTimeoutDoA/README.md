# ACCTZ-6.1 - gNSI.acctz.v1 (Accounting) Test RecordSubscribe Idle Timeout - DoA client

## Summary
Test RecordSubscribe connection termination after idle timeout without making RecordSubscribe idle timeout refresh RPCs

## Procedure

- Establish gNSI connection
- Call gnsi.acctz.v1.Acctz.RecordSubscribe with RecordRequest.timestamp = now()
- Wait at least longer than the idletimeout period (default: 120s)
- Verify that the DUT closes the gNSI connection at or shortly after the idletimeout period.

## Config Parameter
### Prefix:
/gnsi/acctz/v1/Acctz/RecordSubscribe

### Parameter:

## Telemetry Coverage
gnsi.acctz.v1

## Minimum DUT
vRX
