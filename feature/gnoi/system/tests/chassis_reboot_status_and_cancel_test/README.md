# gNOI-3.4: Chassis Reboot Status and Reboot Cancellation

## Summary

Validate gNOI RPC can get reboot status and cancel the reboot

## Procedure

*   Test gnoi.system Reboot status RPC.
    *   Issue gnoi.system Reboot status request RPC to chassis.
    *   Validate that the reboot status is not active before sending reboot request.
    *   Validate the reboot status after sending reboot request.
        *   The reboot status is active.
        *   The reason from reboot status response matches reboot message.
        *   The wait time from reboot status response matches reboot delay.
*   Test gnoi.system Cancel Reboot RPC.
    *   Issue Cancel reboot request RPC to chassis before the test.
    *   Validate that there is no response error returned.
    *   Issue Reboot request with delay RPC to chassis.
    *   Validate that the reboot status is active.
    *   Issue Cancel reboot request RPC to chassis.
    *   Validate that the reboot status is no longer active.
    
## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

```yaml
rpcs:
  gnoi:
    system.System.CancelReboot:
    system.System.Reboot:
    system.System.RebootStatus:
```

