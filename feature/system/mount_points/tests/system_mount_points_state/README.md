# SYS-4.1: System Mount Points State Verification


## Summary

Verify system mount points state parameters including name, size, utilized, and available space.

## Testbed type

* [dut.testbed](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut.testbed)

## Procedure

### Test environment setup

* No special setup required. Ensure the DUT is operational.

### System-Mount-Points-1.1.1: Verify Mount Points State

1.  Subscribe/Get `/system/mount-points/mount-point` list.
2.  Verify that the list of mount points is not empty (expecting at least one mount point).
3.  For each mount point, verify `name`, `size`, `utilized`, and `available` are present and valid:
    *   `size` >= 0
    *   `utilized` <= `size`
    *   `available` <= `size`

*Note*: `mount-points` in OpenConfig are `config false` (operational state only), so we cannot configure them via OC. We verify the state reported by the system.

#### Canonical OC

```json
{
  "system": {
    "mount-points": {
      "mount-point": [
        {
          "name": "/",
          "state": {
            "name": "/",
            "size": "100000",
            "utilized": "50000",
            "available": "50000"
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
  /system/mount-points/mount-point/state/utilized:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Get:
```

## Required DUT platform

* FFF
