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

## Canonical OC

The subinterface health checks require an untagged subinterface to be present on
each DUT port, so the test configures the following before validating telemetry.

```json
{
  "openconfig-interfaces:interfaces": {
    "interface": [
      {
        "config": {
          "description": "Health-1.1 dutPort1",
          "enabled": true,
          "name": "port1",
          "type": "ethernetCsmacd"
        },
        "name": "port1",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "description": "Health-1.1 dutPort1",
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "198.51.100.1",
                        "prefix-length": 30
                      },
                      "ip": "198.51.100.1"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:1::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8:1::1"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              }
            }
          ]
        }
      },
      {
        "config": {
          "description": "Health-1.1 dutPort2",
          "enabled": true,
          "name": "port2",
          "type": "ethernetCsmacd"
        },
        "name": "port2",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "description": "Health-1.1 dutPort2",
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "198.51.100.5",
                        "prefix-length": 30
                      },
                      "ip": "198.51.100.5"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:1::5",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8:1::5"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              }
            }
          ]
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:

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
    /interfaces/interface/subinterfaces/subinterface/state/oper-status:
    /interfaces/interface/subinterfaces/subinterface/state/admin-status:
    /interfaces/interface/subinterfaces/subinterface/state/description:
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
