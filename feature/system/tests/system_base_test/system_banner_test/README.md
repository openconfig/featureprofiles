# System-1.1: System banner testing

## Summary

Ensures the device support basic system requirements for a device supporting g* APIs.

### Procedure

Each test will require the DUT configured with a basic service configuration that
should be provided as part of the basic configuration.  This setup should also include
any security setup for connecting to the services.

1. Configure DUT with service configurations for all required services
2. Verify if MOTD banner configuration paths can be read, updated and deleted.
3. Verify that the Login banner configuration paths can be read, updated, and deleted.

## OpenConfig Path and RPC Coverage

```yaml
paths:
   /system/state/motd-banner:
   /system/state/login-banner:

rpcs:
   gnmi:
      gNMI.Set:
         replace: true
         delete: true
      gNMI.Subscribe:
```
