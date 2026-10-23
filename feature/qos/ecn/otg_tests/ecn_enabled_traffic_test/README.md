# DP-1.12: ECN enabled traffic test

## Summary

Verify that DUT set ECN (ECT and CE) bits to 1 for all ECN capable packets >
minimum threshold.

## Testbed type

*   [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)
*   [`featureprofiles/topologies/dut_800zr.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut_800zr.testbed)

## QoS traffic test setup:

*   Topology:

    *   2 input interfaces and 1 output interface with the same port speed. The
        interface can be a physical interface or LACP bundle interface with the
        same aggregated speed.

    ```
      ATE port 1
          |
         DUT--------ATE port 3
          |
      ATE port 2
    ```

*   Traffic classes:

    *   We will use 7 traffic classes NC1, AF4, AF3, AF2, AF1, BE1 and BE0.

*   Traffic types:

    *   All the traffic tests apply to both IPv4 and IPv6 traffic.

*   Queue types:

    *   NC1 will have strict priority queues
        *   AF4/AF3/AF2/AF1/BE1/BE0 will use WRR queues.
    *   NC1 and AF4 will have strict priority queues with NC1 having higher
        priority.
        *   AF3, AF2, AF1, BE1 and BE0 will use WRR queues.

*   Test results should be independent of the location of interfaces. For
    example, 2 input interfaces and output interface could be located on

    *   Same ASIC-based forwarding engine
    *   Different ASIC-based forwarding engine on same line card
    *   Different ASIC-based forwarding engine on different line cards

*   Test results should be the same for port speeds 100G, 400G, and 800G
    (including 800GZR/ZRP physical interfaces and 800G LACP Port-Channel
    trunks).

*   Counters should be also verified for each test case:

    *   /qos/interfaces/interface/output/queues/queue/state/transmit-pkts
    *   /qos/interfaces/interface/output/queues/queue/state/dropped-pkts
    *   transmit-pkts should be equal to the number of Rx pkts on Ixia port
    *   dropped-pkts should be equal to diff between the number of Tx and the
        number Rx pkts on Ixia ports

*   Latency:

    *   Should be < 100000ns

## Procedure

*   Connect DUT port-1 to ATE port-1, DUT port-2 to ATE port-2 and DUT port-3 to
    ATE port-3.

*   Configuration:

    *   ECN profile (`queue-management-profile`, such as `800GECNProfile` for
        800G interfaces) can be created for different queues and applied to the
        output side of interfaces with the following WRED uniform parameters:
        *   `min-threshold`: `80000` (80 KB)
        *   `max-threshold`: `18446744073709551615` (`2^64-1`, or `3000000`
            bytes / 3 MB on platforms with lower `max-threshold` limits)
        *   `enable-ecn`: `true`
        *   `drop`: `false`
        *   `weight`: `not set` (`0`)
        *   `max-drop-probability-percent`: `1`

*   Send oversubscribed traffic (`101%` aggregate line rate) with `ECT` or `CE`
    set to `1` from 2 input interfaces to trigger ECN bits to be set to `1`:

    *   Traffic class: `NC1`
    *   Interface 1 ingress rate: `51%` of line rate
    *   Interface 2 ingress rate: `50%` of line rate
    *   Expected egress ECN bits: `1` (`CE` = `0b11`)

*   Verify that ECN bits are set with packet drop counter dropped-pkts
    incremented.

*   Repeat the above test cases for other traffic classes:

    *   AF4
    *   AF3
    *   AF2
    *   AF1
    *   BE1
    *   BE0

*   Repeat the above case by sending oversubscribed traffic with both ECT and CE
    cleared instead of one of them being set to 1 from 2 input interfaces.

*   Verify that ECN bits are NOT set and packet drop counter dropped-pkts is
    incremented.

### Test environment setup

*   Connect `DUT port-1`, `DUT port-2`, `DUT port-3`, and `DUT port-4` to `ATE
    port-1`, `ATE port-2`, `ATE port-3`, and `ATE port-4` respectively at `800G`
    (`/interfaces/interface/ethernet/config/port-speed = SPEED_800GB`,
    `/interfaces/interface/config/mtu = 9236`, unbroken 800GZR/ZRP optics).
    Resolve the vendor-neutral OpenConfig LACP aggregate interface name
    (`LAG_1`, represented as `Port-Channel547` in the example configuration).
*   Configure IPv4 and IPv6 addresses on DUT and ATE interfaces (`DUT port-1`
    `192.0.2.1/30`, `2001:db8::1/126` <-> `ATE port-1` `192.0.2.2/30`,
    `2001:db8::2/126`; `DUT port-2` `192.0.2.5/30`, `2001:db8::5/126` <-> `ATE
    port-2` `192.0.2.6/30`, `2001:db8::6/126`; egress `DUT port-3` or `LAG_1`
    `192.0.2.9/30`, `2001:db8::9/126` <-> `ATE port-3` `192.0.2.10/30`,
    `2001:db8::a/126`; `DUT port-4` `192.0.2.13/30`, `2001:db8::d/126` <-> `ATE
    port-4` `192.0.2.14/30`, `2001:db8::e/126`).
*   Configure dual-stack eBGP between DUT (`AS 65001`) and `ATE port-3` (`AS
    65002`) advertising `1,000` IPv4 prefixes (`198.51.100.0/24` through
    `198.54.231.0/24`) and `1,000` IPv6 prefixes (`2001:db8:100::/64` through
    `2001:db8:4e7::/64`).
*   Configure the 7 QoS queues (`NC1`, `AF4`, `AF3`, `AF2`, `AF1`, `BE0`,
    `BE1`), forwarding groups (`target-group-NC1`..`target-group-BE1`), and
    dual-stack input DSCP classifiers (`dscp_based_classifier_ipv4` and
    `dscp_based_classifier_ipv6`) mapping DSCP `0..3` to `BE1`, `4..7` to `BE0`,
    `8..11` to `AF1`, `16..19` to `AF2`, `24..27` to `AF3`, `32..35` to `AF4`,
    and `48..59` to `NC1`.
*   Configure egress scheduler policy `scheduler` (`sequence = 0` Strict
    Priority for `NC1` weight `7` and `AF4` weight `6`; `sequence = 1` WRR for
    `AF3` weight `32`, `AF2` `16`, `AF1` `8`, `BE0` `4`, `BE1` `1`) and WRED
    queue-management profile `800GECNProfile` using the ECN profile parameters
    defined in `Procedure` (`min-threshold = 80000`, `max-threshold =
    18446744073709551615` or `3000000`, `enable-ecn = true`, `drop = false`,
    `weight = 0`, `max-drop-probability-percent = 1`).
*   Verify all state transitions and post-traffic queue counters using streaming
    `gNMI.Subscribe` (`ON_CHANGE`) without static sleeps, and confirm IPv4 ARP,
    IPv6 ND, and eBGP prefix convergence before starting OTG traffic
    (`1000`-byte UDP packets tracking egress DSCP bits `0..5` and ECN bits
    `6..7`: `0b00` Not-ECT, `0b01` ECT(1), `0b10` ECT(0), `0b11` CE on `ATE
    port-3`).

### DP-1.12.1 - Validate 800G Physical (800GZR) QoS Profile State and Non-Congested Traffic Baseline

*   Step 1 - Attach input DSCP classifiers on `DUT port-1`, `DUT port-2`, and
    `DUT port-4`, and attach egress scheduler policy `scheduler`
    (`/qos/interfaces/interface/output/scheduler-policy/config/name`) and
    `800GECNProfile`
    (`/qos/interfaces/interface/output/queues/queue/config/queue-management-profile`)
    across all 7 queues on 800G physical egress interface `DUT port-3`.
*   Step 2 - Push configuration to DUT using `gNMI.Set` with `REPLACE` option.
*   Step 3 - Validate via `gNMI.Subscribe` (`ON_CHANGE`) that `DUT port-3`
    reports `/interfaces/interface/ethernet/state/port-speed` as `SPEED_800GB`,
    `/qos/interfaces/interface/output/scheduler-policy/state/name` as
    `scheduler`,
    `/qos/interfaces/interface/output/queues/queue/state/queue-management-profile`
    as `800GECNProfile` for all 7 queues,
    `/qos/queue-management-profiles/queue-management-profile/wred/uniform/state/min-threshold`
    as `80000`,
    `/qos/queue-management-profiles/queue-management-profile/wred/uniform/state/enable-ecn`
    as `true`, and accurate hardware resource utilization under
    `/components/component/integrated-circuit/utilization/resources/resource/state/used`
    and `free`.
*   Step 4 - Send sub-threshold IPv4 and IPv6 traffic (`80%` aggregate line rate
    = `40%` from `ATE port-1` and `40%` from `ATE port-2`) with `ECT(0)`
    (`0b10`) across all 7 queues towards the advertised destinations on `ATE
    port-3`.
*   Step 5 - Verify that `dropped-pkts` is `0`, `transmit-pkts` matches ATE Rx
    packets, and `100%` of received packets on `ATE port-3` retain `ECN = 0b10`
    with `0` packets marked `CE` (`0b11`).

### DP-1.12.2 - Validate 800G Physical (800GZR) Oversubscribed Per-Queue and Multi-Queue ECN Marking

*   Step 1 - For each queue (`NC1` through `BE1`), send oversubscribed IPv4 and
    IPv6 traffic (`101%` of 800G line rate = `51%` from `ATE port-1` and `50%`
    from `ATE port-2`) with `ECT(0)` (`0b10`) and `ECT(1)` (`0b01`) towards `ATE
    port-3`.
*   Step 2 - Verify that once queue depth exceeds `min-threshold` (`80000`
    bytes, after the initial ~80-packet queue fill), $\ge 99\%$ of received
    packets on `ATE port-3` have `ECN = 0b11` (`CE`), DSCP is unchanged, and
    queue `transmit-pkts` and `dropped-pkts` counters match ATE counters.
*   Step 3 - Send simultaneous 7-queue traffic from `ATE port-1`, `ATE port-2`,
    and `ATE port-4` towards `DUT port-3` at `177%` aggregate load with `ECT(0)`
    (`0b10`) (uncongested Strict Priority: `NC1` `1%`, `AF4` `50%`;
    oversubscribed WRR: `AF3` `60%`, `AF2` `40%`, `AF1` `16%`, `BE0` `8%`, `BE1`
    `2%`).
*   Step 4 - Verify that Strict Priority queues `NC1` and `AF4` have `0` drops
    and `0` `CE` marks (`NC1` latency `< 100000ns`), while congested WRR queues
    (`AF3`..`BE1`) share the remaining `49%` egress bandwidth according to their
    WRR weights (`32:16:8:4:1`) with $\ge 99\%$ of received packets marked `CE`
    (`0b11`).

### DP-1.12.3 - Validate 800G LACP Port-Channel (LAG_1) ECN Marking and Member Failover

*   Step 1 - Configure `LAG_1` (`ieee8023adLag`, e.g., `Port-Channel547`) with
    LACP (`ACTIVE`, `FAST`), assign `DUT port-3` to `LAG_1` via
    `/interfaces/interface/ethernet/config/aggregate-id`, move the egress L3
    subinterface and eBGP session to `LAG_1`, and attach `scheduler` and
    `800GECNProfile` to `LAG_1`.
*   Step 2 - Push configuration to DUT and validate via `gNMI.Subscribe` that
    `/interfaces/interface/aggregation/state/lag-speed` is `800000` and
    `/qos/interfaces/interface/output/queues/queue/state/queue-management-profile`
    is `800GECNProfile` for all 7 queues on `LAG_1`.
*   Step 3 - Repeat the per-queue (`101%`) and simultaneous 7-queue
    oversubscribed traffic streams from `DP-1.12.2` over `LAG_1` and verify `CE`
    (`0b11`) marking and queue counter accuracy on `LAG_1`.
*   Step 4 - Add `DUT port-4` to `LAG_1` (`lag-speed = 1600000`), send `150%` of
    800G (`75%` from `ATE port-1` + `75%` from `ATE port-2`) `ECT(0)` traffic in
    `AF3`, and verify `0` drops and `0` `CE` marks across the 2-member LAG.
*   Step 5 - Bring `ATE port-4` link down (`lag-speed` drops to `800000` on
    surviving member `DUT port-3`) and verify that the active `150%` flow
    exceeds `min-threshold` (`80000` bytes) and marks `ECN = 0b11` (`CE`) on
    `ATE port-3`; then restore `ATE port-4` and detach `DUT port-4` from `LAG_1`
    so `LAG_1` returns to `lag-speed = 800000` (`1 x 800G`).

### DP-1.12.4 - Validate Non-ECN Capable (Not-ECT) Traffic Under 800G Congestion (Negative)

*   Step 1 - Send oversubscribed (`101%` of 800G = `51%` from `ATE port-1` and
    `50%` from `ATE port-2`) IPv4 and IPv6 traffic in queue `AF3` with ECN bits
    set to `Not-ECT` (`0b00`) towards `LAG_1`.
*   Step 2 - Verify that `100%` of received packets on `ATE port-3` retain
    `ECN = 0b00` (`0` packets marked `CE`) and excess traffic is tail-dropped
    (`dropped-pkts > 0`).

### DP-1.12.5 - Validate Missing 800GECNProfile / Disabled ECN Behavior and Recovery (Negative)

*   Step 1 - Remove `800GECNProfile` from queue `AF3` on `LAG_1` (or set
    `/qos/queue-management-profiles/queue-management-profile/wred/uniform/config/enable-ecn`
    to `false`) using `gNMI.Set`, while keeping `800GECNProfile` enabled on
    `AF2`.
*   Step 2 - Send oversubscribed (`101%` of 800G) `ECT(0)` (`0b10`) traffic to
    `AF3` and `AF2`.
*   Step 3 - Verify that `AF3` tail-drops excess traffic with `0` `CE` (`0b11`)
    marks (`100%` remain `0b10`), while `AF2` continues marking $\ge 99\%$ of
    received packets with `CE` (`0b11`).
*   Step 4 - Re-apply `800GECNProfile` (`enable-ecn = true`) to `AF3`, push to
    DUT, and verify `CE` (`0b11`) marking is restored.

### DP-1.12.6 - Validate WRED Probabilistic Marking Between Min and Max Thresholds (Corner)

*   Step 1 - With `min-threshold = 80000` bytes and `max-threshold = 3000000`
    bytes on `800GECNProfile`, send a micro-burst at `101%` of 800G with
    `ECT(0)` (`0b10`) in queue `AF3` so peak queue occupancy stays between
    `80000` bytes and `3000000` bytes.
*   Step 2 - Verify that `dropped-pkts` is `0` (as `drop = false`) and `ATE
    port-3` receives both unmarked `ECT(0)` (`0b10`) and marked `CE` (`0b11`)
    packets.

### DP-1.12.7 - Validate Invalid WRED Configuration Rejection and Resource Contention Resilience (Negative)

*   Step 1 - Attempt via `gNMI.Set` to configure `800GECNProfile` with
    `min-threshold = 80000 > max-threshold = 40000` or out-of-range
    `max-drop-probability-percent = 101`, assign a non-existent profile
    (`NON_EXISTENT_PROFILE`) to `/qos/interfaces/interface/output/queues/queue`,
    or delete `800GECNProfile` while actively attached to `LAG_1`.
*   Step 2 - Verify that the DUT rejects each invalid mutation, and that if
    hardware classifier/forwarding table resources are exhausted by control
    plane entries (`RESOURCE_EXHAUSTED`), existing `800GECNProfile` state,
    hardware utilization telemetry, and `CE` (`0b11`) marking remain stable and
    unaffected.

### DP-1.12.8 - Teardown and Cleanup Verification

*   Step 1 - Detach `800GECNProfile` and `scheduler` from `LAG_1`, delete
    `800GECNProfile` and `LAG_1`, and restore `DUT port-1`..`DUT port-4` and
    `ATE port-1`..`ATE port-4` to their baseline state using `gNMI.Set`.
*   Step 2 - Verify via `gNMI.Get` and `gNMI.Subscribe` that
    `/qos/interfaces/interface/output/queues/queue/state/queue-management-profile`
    no longer returns `800GECNProfile` and all ports report
    `/interfaces/interface/state/oper-status` as `UP`.

## Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "name": "Port-Channel547",
          "type": "iana-if-type:ieee8023adLag"
        },
        "name": "Port-Channel547"
      }
    ]
  },
  "qos": {
    "classifiers": {
      "classifier": [
        {
          "config": {
            "name": "dscp_based_classifier_ipv4",
            "type": "IPV4"
          },
          "name": "dscp_based_classifier_ipv4",
          "terms": {
            "term": [
              {
                "actions": {
                  "config": {
                    "target-group": "target-group-NC1"
                  }
                },
                "conditions": {
                  "ipv4": {
                    "config": {
                      "dscp-set": [
                        48,
                        49,
                        50,
                        51,
                        52,
                        53,
                        54,
                        55,
                        56,
                        57,
                        58,
                        59
                      ]
                    }
                  }
                },
                "config": {
                  "id": "6"
                },
                "id": "6"
              }
            ]
          }
        }
      ]
    },
    "forwarding-groups": {
      "forwarding-group": [
        {
          "config": {
            "name": "target-group-NC1",
            "output-queue": "NC1"
          },
          "name": "target-group-NC1"
        }
      ]
    },
    "interfaces": {
      "interface": [
        {
          "config": {
            "interface-id": "Port-Channel547"
          },
          "input": {
            "classifiers": {
              "classifier": [
                {
                  "config": {
                    "name": "dscp_based_classifier_ipv4",
                    "type": "IPV4"
                  },
                  "type": "IPV4"
                }
              ]
            }
          },
          "interface-id": "Port-Channel547",
          "interface-ref": {
            "config": {
              "interface": "Port-Channel547"
            }
          },
          "output": {
            "queues": {
              "queue": [
                {
                  "config": {
                    "name": "NC1",
                    "queue-management-profile": "800GECNProfile"
                  },
                  "name": "NC1"
                }
              ]
            },
            "scheduler-policy": {
              "config": {
                "name": "scheduler"
              }
            }
          }
        }
      ]
    },
    "queue-management-profiles": {
      "queue-management-profile": [
        {
          "config": {
            "name": "800GECNProfile"
          },
          "name": "800GECNProfile",
          "wred": {
            "uniform": {
              "config": {
                "drop": false,
                "enable-ecn": true,
                "max-drop-probability-percent": 1,
                "max-threshold": "18446744073709551615",
                "min-threshold": "80000",
                "weight": 0
              }
            }
          }
        }
      ]
    },
    "queues": {
      "queue": [
        {
          "config": {
            "name": "NC1",
            "queue-id": 7
          },
          "name": "NC1"
        }
      ]
    },
    "scheduler-policies": {
      "scheduler-policy": [
        {
          "config": {
            "name": "scheduler"
          },
          "name": "scheduler",
          "schedulers": {
            "scheduler": [
              {
                "config": {
                  "priority": "STRICT",
                  "sequence": 0
                },
                "inputs": {
                  "input": [
                    {
                      "config": {
                        "id": "NC1",
                        "input-type": "QUEUE",
                        "queue": "NC1",
                        "weight": "7"
                      },
                      "id": "NC1"
                    }
                  ]
                },
                "sequence": 0
              }
            ]
          }
        }
      ]
    }
  }
}
```

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml
paths:
  ## Config Paths ##
  /interfaces/interface/ethernet/config/aggregate-id:
  /interfaces/interface/ethernet/config/port-speed:
  /qos/classifiers/classifier/config/name:
  /qos/classifiers/classifier/config/type:
  /qos/classifiers/classifier/terms/term/actions/config/target-group:
  /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set:
  /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set:
  /qos/classifiers/classifier/terms/term/config/id:
  /qos/forwarding-groups/forwarding-group/config/name:
  /qos/forwarding-groups/forwarding-group/config/output-queue:
  /qos/queues/queue/config/name:
  /qos/interfaces/interface/input/classifiers/classifier/config/name:
  /qos/interfaces/interface/output/queues/queue/config/name:
  /qos/interfaces/interface/output/scheduler-policy/config/name:
  /qos/interfaces/interface/output/queues/queue/config/queue-management-profile:
  /qos/scheduler-policies/scheduler-policy/config/name:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/priority:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/sequence:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/id:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/input-type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/weight:
  /qos/queue-management-profiles/queue-management-profile/config/name:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/min-threshold:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-threshold:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/enable-ecn:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/weight:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/drop:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-drop-probability-percent:

  ## State Paths ##
  /components/component/integrated-circuit/utilization/resources/resource/state/free:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/utilization/resources/resource/state/used:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /interfaces/interface/aggregation/state/lag-speed:
  /interfaces/interface/ethernet/state/port-speed:
  /interfaces/interface/state/oper-status:
  /qos/interfaces/interface/input/classifiers/classifier/state/name:
  /qos/interfaces/interface/output/queues/queue/state/queue-management-profile:
  /qos/interfaces/interface/output/queues/queue/state/transmit-pkts:
  /qos/interfaces/interface/output/queues/queue/state/transmit-octets:
  /qos/interfaces/interface/output/queues/queue/state/dropped-pkts:
  /qos/interfaces/interface/output/queues/queue/state/dropped-octets:
  /qos/interfaces/interface/output/scheduler-policy/state/name:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/state/min-threshold:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/state/max-threshold:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/state/enable-ecn:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/state/drop:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/state/max-drop-probability-percent:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/state/weight:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Subscribe:
      on_change: true
    gNMI.Set:
      Replace:
```

## Required DUT platform

*   FFF
*   MFF
