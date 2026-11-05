# Health-1.1: Generic Health Check

## Summary

Generic Health Check

## Procedure

*   Capture the generic health check of the DUT, used modularly in pre/post and during various different tests:
    *   No system/kernel/process/component coredumps
    *   No high CPU spike or usage on control or forwarding plane
        *   For every controller card and linecard CPU (or, on fixed form
            factor devices without controller cards, every CPU parented by the
            chassis), stream `/components/component/cpu/utilization/state/avg`
            and `/components/component/cpu/utilization/state/instant` for 30
            seconds.
        *   Verify both leaves are streamed during the window.
        *   No `avg` sample may exceed the CPU utilization threshold (default
            80%, configurable with the `cpu_utilization_threshold` flag).
        *   `instant` may exceed the threshold for a single sample (a short
            spike), but fails if it exceeds the threshold for 2 or more
            consecutive samples (sustained high CPU).
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
        *   Fans are spinning
            *   For every installed (non-empty) FAN component, read
                `/components/component/fan/state/speed` and then stream it for
                30 seconds (the initial read is included, since some devices
                only stream fan speed when it changes).
            *   Verify the leaf is streamed for every fan and that no sample
                reports 0 rpm.
            *   If the device has no FAN components (for example, it reports
                fan speed on sensor components under each FAN_TRAY), validate
                every component that streams `fan/state/speed` instead, and
                verify that every installed FAN_TRAY has at least one child
                component streaming fan speed.
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

## Canonical OC
```json
{}
```

## OpenConfig Path and RPC Coverage

```yaml
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Subscribe:

paths:
  ## Config Parameter coverage

    /system/alarms/alarm/state/severity:
    /components/component/cpu/utilization/state/avg:
       platform_type: ["CPU"]
    /components/component/cpu/utilization/state/instant:
       platform_type: ["CPU"]
    /components/component/fan/state/speed:
       platform_type: ["FAN"]
    /components/component/integrated-circuit/pipeline-counters/drop/fabric-block/state/fabric-aggregate:
       platform_type: ["INTEGRATED_CIRCUIT"]
    /components/component/integrated-circuit/pipeline-counters/drop/fabric-block/state/in-high-priority:
       platform_type: ["INTEGRATED_CIRCUIT"]
    /components/component/integrated-circuit/pipeline-counters/drop/fabric-block/state/in-low-priority:
       platform_type: ["INTEGRATED_CIRCUIT"]
    /components/component/integrated-circuit/pipeline-counters/drop/fabric-block/state/lost-packets:
       platform_type: ["INTEGRATED_CIRCUIT"]
    /components/component/integrated-circuit/pipeline-counters/drop/fabric-block/state/out-high-priority:
       platform_type: ["INTEGRATED_CIRCUIT"]
    /components/component/integrated-circuit/pipeline-counters/drop/fabric-block/state/out-low-priority:
       platform_type: ["INTEGRATED_CIRCUIT"]
    /components/component/integrated-circuit/pipeline-counters/drop/host-interface-block/state/host-aggregate:
       platform_type: ["INTEGRATED_CIRCUIT"]
    /components/component/integrated-circuit/pipeline-counters/drop/lookup-block/state/acl-drops:
       platform_type: ["INTEGRATED_CIRCUIT"]
    /components/component/integrated-circuit/pipeline-counters/drop/lookup-block/state/incorrect-software-state:
       platform_type: ["INTEGRATED_CIRCUIT"]
    /components/component/integrated-circuit/pipeline-counters/drop/lookup-block/state/invalid-packet:
       platform_type: ["INTEGRATED_CIRCUIT"]
    /components/component/integrated-circuit/pipeline-counters/drop/lookup-block/state/no-nexthop:
       platform_type: ["INTEGRATED_CIRCUIT"]
    /components/component/integrated-circuit/pipeline-counters/drop/lookup-block/state/oversubscription:
       platform_type: ["INTEGRATED_CIRCUIT"]
    /components/component/integrated-circuit/pipeline-counters/drop/state/congestion-aggregate:
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
