# System-1.5: System software-version test

## Summary

Ensures the device support basic system requirements for a device supporting g* APIs.

### Procedure

Each test will require the DUT configured with a basic service configuration that
should be provided as part of the basic configuration. This setup should also include
any security setup for connecting to the services.

### Tests

1. Configure DUT with service configurations for all required services
2. The test will verify if the software-version state path can be read and is non-empty.

#### Canonical OC
```json
{
  "system": {
    "state": {
      "software-version": "1.4.2"
    }
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
   /system/state/software-version:

rpcs:
   gnmi:
      gNMI.Subscribe:
```
