# SYS-4.1: System Mount Points State Verification

## Summary

Verify system mount points state parameters including name, size, utilized,
available space, and storage component reference.

## Testbed type

* `TESTBED_DUT`

## Procedure

### Test environment setup

* No special setup required. Ensure the DUT is operational.

### System-Mount-Points-1.1.1: Verify Mount Points State

1.  Subscribe/Get `/system/mount-points/mount-point` list.
2.  Verify that the list of mount points is not empty (expecting at least one mount point).
3.  For each mount point, verify `name`, `size`, `utilized`, `available`, and optional `storage-component` are valid:

    *   `size` >= 0
    *   `utilized` <= `size`
    *   `available` <= `size`
    *   Verify `storage-component` string if present.

*Note*: `mount-points` in OpenConfig are `config false` (operational state
only), so we cannot configure them via OC. We verify the state reported by the
system.

#### Canonical OC

```json
{
  "components": {
    "component": [
      {
        "config": {
          "name": "disk0"
        },
        "name": "disk0"
      }
    ]
  },
  "system": {
    "mount-points": {
      "mount-point": [
        {
          "name": "/",
          "state": {
            "available": "50000",
            "name": "/",
            "size": "100000",
            "storage-component": "disk0",
            "utilized": "50000"
          }
        }
      ]
    }
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /system/mount-points/mount-point/state/available:
  /system/mount-points/mount-point/state/name:
  /system/mount-points/mount-point/state/size:
  /system/mount-points/mount-point/state/storage-component:
  /system/mount-points/mount-point/state/utilized:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Get:
```

## Required DUT platform

* FFF
