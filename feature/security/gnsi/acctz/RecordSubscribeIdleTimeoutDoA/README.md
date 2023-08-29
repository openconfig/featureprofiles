# gNSI.acctz.v1 (Accounting) Test RecordSubscribe Idle Timeout - DoA client

Test RecordSubscribe connection termination after idle timeout without making RecordSubscribe idle timeout refresh RPCs

## Procedure

- Establish gNSI connection
- Call gnsi.acctz.v1.Acctz.RecordSubscribe with RecordRequest.timestamp = now()
- Wait 120+ seconds
- Verify that the DUT closes the gNSI connection after ~ 120 seconds of idle time.

## Config Parameter
### Prefix:
/gnsi/acctz/v1/Acctz/RecordSubscribe

### Parameter:

## Telemetry Coverage
gnsi.acctz.v1

## Minimum DUT
vRX
