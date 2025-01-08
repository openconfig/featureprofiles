# Storage-1.1: Storage File System Check

## Summary

Storage File System Check

## Testbed type

*    dut.testbed

## Procedure

* For each mountpoint collect and verify if the following path returns valid values

  *  /components/component/storage/state/counters/soft-read-error-rate:
  *  /components/component/storage/state/counters/reallocated-sectors:
  *  /components/component/storage/state/counters/end-to-end-error:
  *  /components/component/storage/state/counters/offline-uncorrectable-sectors-count:
  *  /components/component/storage/state/counters/life-left:
  *  /components/component/storage/state/counters/percentage-used:

  
## OpenConfig Path and RPC Coverage

```yaml

paths:

  /components/component/storage/state/counters/soft-read-error-rate:
   platform_type: [ "STORAGE" ]
  /components/component/storage/state/counters/reallocated-sectors:
   platform_type: [ "STORAGE" ]
  /components/component/storage/state/counters/end-to-end-error:
   platform_type: [ "STORAGE" ]
  /components/component/storage/state/counters/offline-uncorrectable-sectors-count:
   platform_type: [ "STORAGE" ]
  /components/component/storage/state/counters/life-left:
   platform_type: [ "STORAGE" ]
  /components/component/storage/state/counters/percentage-used:
   platform_type: [ "STORAGE" ]


rpcs:
  gnmi:
    gNMI.Get:
```
## Required DUT platform
Single DUT
