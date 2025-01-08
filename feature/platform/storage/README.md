# Storage-1.1: Storage File System Check

## Summary

Storage File System Check

## Testbed type

*    dut.testbed

## Procedure

* For each mountpoint collect and verify if the following path returns valid values

  *  /components/component[name=STORAGE]/state/counters/soft-read-error-rate:
  *  /components/component[name=STORAGE]/state/counters/reallocated-sectors:
  *  /components/component[name=STORAGE]/state/counters/end-to-end-error:
  *  /components/component[name=STORAGE]/state/counters/offline-uncorrectable-sectors-count:
  *  /components/component[name=STORAGE]/state/counters/life-left:
  *  /components/component[name=STORAGE]/state/counters/percentage-used:

  
## OpenConfig Path and RPC Coverage

```yaml

paths:

  /components/component[name=STORAGE]/storage/state/counters/soft-read-error-rate:
  /components/component[name=STORAGE]/storage/state/counters/reallocated-sectors:
  /components/component[name=STORAGE]/storage/state/counters/end-to-end-error:
  /components/component[name=STORAGE]/state/counters/offline-uncorrectable-sectors-count:
  /components/component[name=STORAGE]/state/counters/life-left:
  /components/component[name=STORAGE]/state/counters/percentage-used:


rpcs:
  gnmi:
    gNMI.Get:
```
## Required DUT platform
Single DUT
