# gNOI-3.1: Complete Chassis Reboot

## Summary

Validate gNOI RPC can reboot entire chassis

## Procedure

*   Configure ATE port-1 connected to DUT port-1 with the relevant IPv4 and IPv6
    addresses.
*   Issue gnoi.system Reboot RPC to chassis with method set to COLD and no
    populated delay or subcomponents.
    *   Validate that system uptime is reflected as having rebooted after device
        returns.
        *   TODO: test code currently checks boot-time instead of uptime.
    *   TODO: Validate that all connected ports are disabled and re-enabled.
    *   Validate that the device returns with the expected software version.
*   Issue Reboot RPC to chassis with method set to COLD and a populated delay of
    N seconds.
    *   Validate that system remains reachable for N seconds.
    *   Validate that system uptime is reflected as having rebooted.
        *   TODO: test code currently checks boot-time instead of uptime
    *   TODO: Validate that all connected ports are disabled and re-enabled.
    *   Validate that the device returns with the expected software version

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

TODO(OCPATH): fill in coverage from code already written.

```yaml
paths:
  ## State paths
  /system/state/boot-time:

rpcs:
  gnoi:
    system.System.Reboot:
    system.System.CancelReboot:
```
