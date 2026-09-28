# System-1.4: System time test

## Summary

Ensures the device support basic system requirements for a device supporting g* APIs.

### Procedure

Each test will require the DUT configured with a basic service configuration that
should be provided as part of the basic configuration.  This setup should also include
any security setup for connecting to the services.

### Tests

1. Configure DUT with service configurations for all required services
2. Each test will then create a client to those services and valid each service is properly
   listening on the standard port.
3. Verify if the current date and time state path can be parsed as RFC3339 time format.
4. Verify if the timestamp that the system was last restarted can be read and is not an unreasonable value.
5. Verify if the timezone-name config values can be read and set.
6. Verify that `/system/state/last-configuration-timestamp` is present, returns a valid timestamp in nanoseconds since the Unix epoch, and monotonically increases after applying a configuration change (e.g., updating `/system/config/domain-name`).

#### Canonical OC

```json
{
  "system": {
    "config": {
      "domain-name": "test.name.example"
    }
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
   /system/state/current-datetime:
   /system/state/boot-time:
   /system/state/last-configuration-timestamp:
   /system/clock/state/timezone-name:
   /system/config/domain-name:
   /system/state/domain-name:

rpcs:
   gnmi:
      gNMI.Get:
      gNMI.Set:
         replace: true
         delete: true
      gNMI.Subscribe:
```
