# Health-1.2: File Systems Check

## Summary

File Systems Check

## Testbed type

*    dut.testbed

## Procedure

* For each mountpoint collect and verify if the following path returns valid values
    *    /system/mount-points/mount-point/state/name:
    *    /system/mount-points/mount-point/state/storage-component:
    *    /system/mount-points/mount-point/state/size:
    *    /system/mount-points/mount-point/state/available:
    *    /system/mount-points/mount-point/state/utilized:
    *    /system/mount-points/mount-point/state/counters/io-errors:

* Alert if the utilized memory of a file system is more than the configured threshold value.
  
## OpenConfig Path and RPC Coverage

```yaml

paths:
    /system/mount-points/mount-point/state/name:
    /system/mount-points/mount-point/state/storage-component:
    /system/mount-points/mount-point/state/size:
    /system/mount-points/mount-point/state/available:
    /system/mount-points/mount-point/state/utilized:
    /system/mount-points/mount-point/state/counters/io-errors:

rpcs:
  gnmi:
    gNMI.Get:
```

## Required DUT platform

Single DUT
