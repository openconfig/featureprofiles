# Storage-1.1: Storage File System Check

## Summary

Storage File System Check

## Testbed type

*    dut.testbed

## Procedure

* For each storage file system collect and verify if the following path are not empty and returns valid values. Log the values returned. 

  *  /components/component/storage/state/counters/soft-read-error-rate:
     Check if the response is a counter64
  *  /components/component/storage/state/counters/reallocated-sectors:
     Check if the response is a counter64
  *  /components/component/storage/state/counters/end-to-end-error:
     Check if the response is a counter64
  *  /components/component/storage/state/counters/offline-uncorrectable-sectors-count:
     Check if the response is a counter64
  *  /components/component/storage/state/counters/life-left:
     Check if the response is an integer 
  *  /components/component/storage/state/counters/percentage-used:
     Check if the response is an integer 
  
## OpenConfig Path and RPC Coverage

```yaml

paths:
## State paths
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
