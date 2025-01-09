# Storage-1.1: Storage File System Check

## Summary

Storage File System Check

## Testbed type

*    dut.testbed

## Procedure

* For each mountpoint collect and verify if the following path returns valid values

  *  /components/component/storage/state/counters/soft-read-error-rate:
     The response returned must ideally be 0, throw a warning message for any other floating point number
  *  /components/component/storage/state/counters/reallocated-sectors:
     Check if the response is an integer 
  *  /components/component/storage/state/counters/end-to-end-error:
     Check if the response is an integer  
  *  /components/component/storage/state/counters/offline-uncorrectable-sectors-count:
     The value returned must ideally be 0, thorw a warning message for any other other integer
  *  /components/component/storage/state/counters/life-left:
     Check if the response is an integer 
  *  /components/component/storage/state/counters/percentage-used:
     The response returned must be an integer, thorw a warning message if utilization is greater than the threshold

  
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
