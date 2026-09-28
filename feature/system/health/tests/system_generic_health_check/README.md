# Health-1.1: Generic Health Check

## Summary

Generic Health Check

## Procedure

*   Capture the generic health check of the DUT, used modularly in pre/post and during various different tests:
    *   No system/kernel/process/component coredumps
    *   No high CPU spike or usage on control or forwarding plane
    *   No high memory utilization or usage on control or forwarding plane
    *   No processes/daemons high CPU/Memory utilization
    *   Validate system process state telemetry (process names, PIDs, and start-times)
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

## Canonical OC

```json
{}
```

## Config Parameter Coverage

N/A

## OpenConfig Path and RPC Coverage

```yaml
rpcs:
  gnmi:
    gNMI.Get:

paths:
  ## Config Parameter coverage

    /components/component/cpu/utilization/state/avg:
       platform_type: ["CPU"]
    /components/component/integrated-circuit/pipeline-counters/drop/fabric-block/state/lost-packets:
       platform_type: ["INTEGRATED_CIRCUIT"]
    /components/component/integrated-circuit/pipeline-counters/drop/lookup-block/state/acl-drops:
       platform_type: ["INTEGRATED_CIRCUIT"]
    /components/component/integrated-circuit/pipeline-counters/drop/lookup-block/state/incorrect-software-state:
       platform_type: ["INTEGRATED_CIRCUIT"]
    /components/component/integrated-circuit/pipeline-counters/drop/lookup-block/state/invalid-packet:
       platform_type: ["INTEGRATED_CIRCUIT"]
    /components/component/integrated-circuit/pipeline-counters/drop/lookup-block/state/no-nexthop:
       platform_type: ["INTEGRATED_CIRCUIT"]
    /components/component/state/memory/available:
       platform_type: ["CHASSIS", "CONTROLLER_CARD", "CPU"]
    /components/component/state/memory/utilized:
       platform_type: ["CHASSIS", "CONTROLLER_CARD", "CPU"]
    /system/processes/process/state/name:
    /system/processes/process/state/pid:
    /system/processes/process/state/start-time:
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
    /interfaces/interface/subinterfaces/subinterface/state/counters/out-octets:
    /interfaces/interface/subinterfaces/subinterface/state/admin-status:
    /interfaces/interface/subinterfaces/subinterface/state/description:
    /interfaces/interface/ethernet/state/counters/in-mac-pause-frames:
    /interfaces/interface/ethernet/state/counters/out-mac-pause-frames:
    /interfaces/interface/ethernet/state/counters/in-crc-errors:
    /interfaces/interface/ethernet/state/counters/in-block-errors:
```

## Protocol/RPC Parameter Coverage
