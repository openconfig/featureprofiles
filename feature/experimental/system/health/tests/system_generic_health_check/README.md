# Health-1.1: Generic Health Check

## Summary

Generic Health Check

## Procedure

*   Capture the generic health check of the DUT, used modularly in pre/post and during various different tests:
    *   No system/kernel/process/component coredumps
    *   No high CPU spike or usage on control or forwarding plane
    *   No high memory utilization or usage on control or forwarding plane
    *   No processes/daemons high CPU/Memory utilization
    *   No generic drop counters
        *   QUEUE drops
            *   Interfaces
            *   VOQ
        *   Fabric drops
        *   ASIC drops
    *   No flow control frames tx/rx
    *   No CRC or Layer 1 errors on interfaces
    *   No config commit errors
    *   No system level alarms
    *   In spec hardware should be in proper state
        *   No hardware errors
        *   Major Alarms
    *   No HW component or SW processes crash
*   TODO:
    *   DDOS/COPP violations
    *   No memory leaks
    *   No system errors or logs
    *   No CRC or Layer 1 errors fabric links

## Config Parameter Coverage

N/A

## OpenConfig Path and RPC Coverage

```yaml
rpcs:
  gnmi:
    gNMI.Get:

paths:
  ## Config Parameter coverage

    /system/processes/process/state/cpu-utilization:
    /system/processes/process/state/memory-utilization:
    /qos/interfaces/interface/input/queues/queue/state/dropped-pkts:
    /qos/interfaces/interface/output/queues/queue/state/dropped-pkts:
    /qos/interfaces/interface/input/virtual-output-queues/voq-interface/queues/queue/state/dropped-pkts:
    /interfaces/interface/state/counters/in-discards:
    /interfaces/interface/state/counters/in-errors:
    /interfaces/interface/state/counters/in-multicast-pkts:
    /interfaces/interface/state/counters/in-unknown-protos:
    /interfaces/interface/state/counters/out-discards:
    /interfaces/interface/state/counters/out-errors:
    /interfaces/interface/state/oper-status:
    /interfaces/interface/state/admin-status:
    /interfaces/interface/state/counters/out-octets:
    /interfaces/interface/state/description:
    /interfaces/interface/state/type:
    /interfaces/interface/subinterfaces/subinterface/state/counters/in-discards:
    /interfaces/interface/subinterfaces/subinterface/state/counters/in-errors:
    /interfaces/interface/subinterfaces/subinterface/state/counters/in-unknown-protos:
    /interfaces/interface/subinterfaces/subinterface/state/counters/out-discards:
    /interfaces/interface/subinterfaces/subinterface/state/counters/out-errors:
    /interfaces/interface/ethernet/state/counters/in-mac-pause-frames:
    /interfaces/interface/ethernet/state/counters/out-mac-pause-frames:
    /interfaces/interface/ethernet/state/counters/in-crc-errors:
    /interfaces/interface/ethernet/state/counters/in-block-errors:
```

## Protocol/RPC Parameter Coverage
