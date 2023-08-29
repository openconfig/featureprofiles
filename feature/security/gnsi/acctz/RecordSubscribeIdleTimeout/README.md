# gNSI.acctz.v1 (Accounting) Test RecordSubscribe Idle Timeout - client becomes silent

Test RecordSubscribe connection termination after idle timeout following 1 RecprdSubscribe RPC and 1 idle timeout refresh RPC

## Procedure

- Establish gNSI connection
- Call gnsi.acctz.v1.Acctz.RecordSubscribe with RecordRequest.timestamp = now()
- Discard received records
- wait 100 seconds
- Call gnsi.acctz.v1.Acctz.RecordSubscribe with RecordRequest.timestamp = now()
- Discard received records
- Verify that the DUT closes the gNSI connection after ~ 120 seconds of idle time.

## Config Parameter
### Prefix:
/gnsi/acctz/v1/Acctz/RecordSubscribe

### Parameter:

## Telemetry Coverage
gnsi.acctz.v1

## Minimum DUT
vRX
