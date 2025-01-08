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
  /components/component/storage/state/counters/reallocated-sectors:
  /components/component/storage/state/counters/end-to-end-error:
  /components/component/storage/state/counters/offline-uncorrectable-sectors-count:
  /components/component/storage/state/counters/life-left:
  /components/component/storage/state/counters/percentage-used:


rpcs:
  gnmi:
    gNMI.Get:
```
## Required DUT platform
Single DUT
