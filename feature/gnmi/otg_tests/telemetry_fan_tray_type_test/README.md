# gNMI-2.1: Telemetry: Fan Tray Type Test

## Summary

Validate the type of fan tray components.

## Procedure

*   For every fan tray component on a device (regardless of whether or not the fans in the fan tray are considered FRU fans):

    *   Validate that the string in /components/component/state/type is equal to either "FAN_TRAY" or "openconfig-platform-types:FAN_TRAY" (for devices that use this prefix).

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

```yaml
paths:
  ## State Paths ##
  /components/component/state/type:
    platform_type: ["FAN_TRAY"]
rpcs:
  gnmi:
    gNMI.Subscribe:
        Mode: [ "ON_CHANGE", "ONCE" ]
    gNMI.Get:
```


## Minimum DUT platform requirement

MMF
