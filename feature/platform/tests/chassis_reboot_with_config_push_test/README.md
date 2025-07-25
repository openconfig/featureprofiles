# gNOI-3.6: Complete Chassis Reboot with large config push

## Summary

Validate gNOI chassis reboot with Large config push is successful without device
health getting affected

## Procedure

*   Test gnoi.system Reboot with Config push.
    *   Issue a Large Set config push to the device
    *   Issue gnoi.system Reboot status request RPC to chassis.
    *   Validate that the reboot status is not active before sending reboot request.
    *   Validate the reboot status after sending reboot request.
        *   The reboot status is active.
        *   The reason from reboot status response matches reboot message.
        *   The wait time from reboot status response matches reboot delay.
        *   Validate that there are no cores formed on the device post the reboot

## Canonical OC
```json
{}
```
## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test
OC paths used for test setup are not listed here.
```yaml
rpcs:
  gnoi:
    system.System.Reboot:
    system.System.RebootStatus:
```


