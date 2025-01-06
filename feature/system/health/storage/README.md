# Health-1.2: Storage Mount-Points Check

## Summary

Storage Mount-Points Check

## Testbed type

dut.testbed

## Procedure



## Config Parameter Coverage

N/A

## OpenConfig Path and RPC Coverage

```yaml

paths:
    /system/mount-points/mount-point/state/name:
    /system/mount-points/mount-point/state/storage-component:
    /system/mount-points/mount-point/state/size:
    /system/mount-points/mount-point/state/available:
    /system/mount-points/mount-point/state/utilized:

rpcs:
  gnmi:
    gNMI.Get:
```

