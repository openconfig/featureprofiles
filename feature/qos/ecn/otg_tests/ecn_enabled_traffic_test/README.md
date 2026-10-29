# DP-1.12: ECN enabled traffic test

## Summary

Verify that DUT set ECN (ECT and CE) bits to 1 for all ECN capable packets >
minimum threshold.

## Testbed type

*   [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

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

### Test environment setup

*   **Topology and Port Speed**:
    *   Connect `DUT port-1`, `DUT port-2`, `DUT port-3`, and `DUT port-4` to
        `ATE port-1`, `ATE port-2`, `ATE port-3`, and `ATE port-4` respectively
        at the testbed's dynamic port speed (`100G`, `400G`, or `800G`).
    *   Configure `/interfaces/interface/ethernet/config/port-speed` to match
        the testbed's dynamic port speed (`SPEED_100GB`, `SPEED_400GB`, or
        `SPEED_800GB` for unbroken 800GZR/ZRP optics) and set
        `/interfaces/interface/config/mtu = 9236`.
    *   Use the vendor-neutral OpenConfig LACP aggregate interface name `LAG_1`
        for Port-Channel subtests (`DP-1.12.3` through `DP-1.12.5`), with `DUT
        port-3` (and `DUT port-4` during 2-member LAG failover) as LACP member
        ports.
*   **Direct L3 Addressing (IPv4 `/31` and IPv6 `/126`)**:
    *   **Ingress 1**: `DUT port-1` (`198.51.100.0/31`, `2001:db8::1/126`) <->
        `ATE port-1` (`198.51.100.1/31`, `2001:db8::2/126`)
    *   **Ingress 2**: `DUT port-2` (`198.51.100.2/31`, `2001:db8::5/126`) <->
        `ATE port-2` (`198.51.100.3/31`, `2001:db8::6/126`)
    *   **Egress**: `DUT port-3` or `LAG_1` (`198.51.100.4/31`,
        `2001:db8::9/126`) <-> `ATE port-3` (`198.51.100.5/31`,
        `2001:db8::a/126`)
*   **QoS Queues, Forwarding Groups, and Input DSCP Classifiers**:
    *   Configure the 7 QoS queues (`NC1`, `AF4`, `AF3`, `AF2`, `AF1`, `BE0`,
        `BE1`) under `/qos/queues/queue/config/name`.
    *   Configure forwarding groups (`target-group-NC1`..`target-group-BE1`)
        under `/qos/forwarding-groups/forwarding-group/config/name` and
        `/qos/forwarding-groups/forwarding-group/config/output-queue`.
    *   Configure dual-stack input DSCP classifiers
        (`dscp_based_classifier_ipv4` and `dscp_based_classifier_ipv6` under
        `/qos/classifiers/classifier/config/name` and
        `/qos/classifiers/classifier/config/type`) via
        `/qos/classifiers/classifier/terms/term/config/id`,
        `/qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set`,
        `/qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set`,
        and
        `/qos/classifiers/classifier/terms/term/actions/config/target-group`:
        *   `BE1`: DSCP `0..3`
        *   `BE0`: DSCP `4..7`
        *   `AF1`: DSCP `8..11`
        *   `AF2`: DSCP `16..19`
        *   `AF3`: DSCP `24..27`
        *   `AF4`: DSCP `32..35`
        *   `NC1`: DSCP `48..59`
*   **Egress Scheduler Policy (`scheduler`)**:
    *   Configure `/qos/scheduler-policies/scheduler-policy/config/name` as
        `scheduler` with queue inputs under
        `/qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/id`,
        `input-type`, `queue`, and `weight`:
        *   **Strict Priority (`sequence = 0`)**: Configure
            `/qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/sequence`
            as `0` and
            `/qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/priority`
            as `STRICT` for `NC1` (weight `7`) and `AF4` (weight `6`).
        *   **Weighted Round-Robin (`sequence = 1`)**: Configure `sequence = 1`
            and
            `/qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/type`
            for WRR queues `AF3` (weight `32`), `AF2` (weight `16`), `AF1`
            (weight `8`), `BE0` (weight `4`), and `BE1` (weight `1`).
*   **WRED ECN Queue-Management Profile**:
    *   Configure the ECN profile (`queue-management-profile` under
        `/qos/queue-management-profiles/queue-management-profile/config/name`,
        using `800GECNProfile` as the profile for `800G` ports) and apply it to
        the output queues of the egress interface with the following WRED
        uniform parameters:
        *   `min-threshold`
            (`/qos/queue-management-profiles/queue-management-profile/wred/uniform/config/min-threshold`):
            `80000` (80 KB)
        *   `max-threshold`
            (`/qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-threshold`):
            `18446744073709551615` (`2^64-1`, or `3000000` bytes / 3 MB on
            platforms with lower `max-threshold` limits)
        *   `enable-ecn`
            (`/qos/queue-management-profiles/queue-management-profile/wred/uniform/config/enable-ecn`):
            `true`
        *   `drop`
            (`/qos/queue-management-profiles/queue-management-profile/wred/uniform/config/drop`):
            `false`
        *   `weight`
            (`/qos/queue-management-profiles/queue-management-profile/wred/uniform/config/weight`):
            `not set` (`0`)
        *   `max-drop-probability-percent`
            (`/qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-drop-probability-percent`):
            `1`
*   **Telemetry and Traffic Verification Baseline**:
    *   Verify all state transitions and post-traffic queue counters using
        streaming `gNMI.Subscribe` (`ON_CHANGE`) without static sleeps, and
        confirm IPv4 ARP and IPv6 ND neighbor resolution before starting OTG
        traffic (`1000`-byte UDP packets towards `ATE port-3` at `198.51.100.5`
        and `2001:db8::a`, tracking egress DSCP bits `0..5` and ECN bits `6..7`:
        `0b00` Not-ECT, `0b01` ECT(1), `0b10` ECT(0), `0b11` CE).
    *   For oversubscribed traffic tests across queues `NC1`, `AF4`, `AF3`,
        `AF2`, `AF1`, `BE1`, and `BE0`, send `101%` aggregate line rate (`51%`
        from `ATE port-1` and `50%` from `ATE port-2`) towards `ATE port-3`:
        *   When `ECT` or `CE` bits are set to `1`, verify that egress ECN bits
            are marked `CE` (`0b11`) with packet drop counter `dropped-pkts`
            incremented as applicable.
        *   When both `ECT` and `CE` bits are cleared (`0b00` Not-ECT), verify
            that egress ECN bits are NOT set (`0b00`) and packet drop counter
            `dropped-pkts` is incremented.

### DP-1.12.1 - Validate Physical Interface QoS Profile State and Non-Congested Traffic Baseline

*   Step 1 - Attach input DSCP classifiers
    (`/qos/interfaces/interface/input/classifiers/classifier/config/name`) on
    ingress interfaces `DUT port-1` and `DUT port-2`, and attach egress
    scheduler policy `scheduler`
    (`/qos/interfaces/interface/output/scheduler-policy/config/name`) and the
    ECN queue-management profile (using `800GECNProfile` as the profile for
    `800G` ports, `/qos/interfaces/interface/output/queues/queue/config/name`
    and
    `/qos/interfaces/interface/output/queues/queue/config/queue-management-profile`)
    across all 7 queues on physical egress interface `DUT port-3`.
*   Step 2 - Push configuration to DUT using `gNMI.Set` with `REPLACE` option.
*   Step 3 - Validate operational state via `gNMI.Subscribe` (`ON_CHANGE`):
    *   `DUT port-3` reports `/interfaces/interface/ethernet/state/port-speed`
        matching the testbed's dynamic port speed (`100G`, `400G`, or `800G`,
        i.e., `SPEED_100GB`, `SPEED_400GB`, or `SPEED_800GB`).
    *   `DUT port-1` and `DUT port-2` report
        `/qos/interfaces/interface/input/classifiers/classifier/state/name` as
        `dscp_based_classifier_ipv4` and `dscp_based_classifier_ipv6`.
    *   `DUT port-3` reports
        `/qos/interfaces/interface/output/scheduler-policy/state/name` as
        `scheduler` and
        `/qos/interfaces/interface/output/queues/queue/state/queue-management-profile`
        as the configured ECN profile (e.g., `800GECNProfile` for `800G` ports)
        across all 7 queues.
    *   The WRED profile reports
        `/qos/queue-management-profiles/queue-management-profile/wred/uniform/state/min-threshold`
        as `80000`, `max-threshold`, `enable-ecn` as `true`, `drop` as `false`,
        `max-drop-probability-percent` as `1`, and `weight` as `0`.
*   Step 4 - Send sub-threshold IPv4 and IPv6 traffic (`80%` aggregate line rate
    = `40%` from `ATE port-1` and `40%` from `ATE port-2`) with `ECT(0)`
    (`0b10`) across all 7 queues towards `ATE port-3` (`198.51.100.5` and
    `2001:db8::a`).
*   Step 5 - Verify non-congested baseline counters and ECN bits:
    *   `/qos/interfaces/interface/output/queues/queue/state/dropped-pkts` and
        `dropped-octets` are `0`.
    *   `/qos/interfaces/interface/output/queues/queue/state/transmit-pkts` and
        `transmit-octets` match ATE Rx counters.
    *   `100%` of received IPv4 and IPv6 packets on `ATE port-3` retain `ECN =
        0b10` with `0` packets marked `CE` (`0b11`).

### DP-1.12.2 - Validate Physical Interface Oversubscribed Per-Queue ECN Marking

*   Step 1 - Testing one queue at a time (`NC1`, `AF4`, `AF3`, `AF2`, `AF1`,
    `BE0`, and `BE1`), send oversubscribed IPv4 and IPv6 traffic (`101%` of line
    rate = `51%` from `ATE port-1` and `50%` from `ATE port-2`) with `ECT(0)`
    (`0b10`) and `ECT(1)` (`0b01`) towards `ATE port-3`.
*   Step 2 - Verify per-queue ECN marking and counters:
    *   Once queue depth exceeds `min-threshold` (`80000` bytes, after the
        initial ~80-packet queue fill), $\ge 99\%$ of received IPv4 and IPv6
        packets on `ATE port-3` have `ECN = 0b11` (`CE`) and DSCP is unchanged.
    *   Queue
        `/qos/interfaces/interface/output/queues/queue/state/transmit-pkts`,
        `transmit-octets`, `dropped-pkts`, and `dropped-octets` counters match
        ATE counters.

### DP-1.12.3 - Validate LACP Port-Channel (LAG_1) ECN Marking and Member Failover

*   Step 1 - Configure `LAG_1` (`ieee8023adLag`) with LACP (`ACTIVE`, `FAST`),
    assign `DUT port-3` to `LAG_1` via
    `/interfaces/interface/ethernet/config/aggregate-id`, move the egress L3
    subinterface (`198.51.100.4/31`, `2001:db8::9/126`) to `LAG_1`, and attach
    `scheduler`
    (`/qos/interfaces/interface/output/scheduler-policy/config/name`) and the
    ECN queue-management profile (using `800GECNProfile` as the profile for
    `800G` ports,
    `/qos/interfaces/interface/output/queues/queue/config/queue-management-profile`)
    to `LAG_1`.
*   Step 2 - Push configuration to DUT and validate via `gNMI.Subscribe`:
    *   `/interfaces/interface/aggregation/state/lag-speed` matches the
        testbed's dynamic single-member port speed in Mbps (`100000` for `100G`,
        `400000` for `400G`, or `800000` for `800G`).
    *   `/qos/interfaces/interface/output/queues/queue/state/queue-management-profile`
        reports the configured ECN profile (e.g., `800GECNProfile` for `800G`
        ports) for all 7 queues on `LAG_1`.
*   Step 3 - Repeat the per-queue (`101%` of line rate) oversubscribed IPv4 and
    IPv6 traffic streams from `DP-1.12.2` over `LAG_1` and verify `CE` (`0b11`)
    marking and queue counter accuracy on `LAG_1`.
*   Step 4 - Add `DUT port-4` (connected to `ATE port-4`) as a second member of
    `LAG_1`:
    *   Verify `/interfaces/interface/aggregation/state/lag-speed` matches `2x`
        the testbed's dynamic port speed in Mbps (`200000` for `2 x 100G`,
        `800000` for `2 x 400G`, or `1600000` for `2 x 800G`).
    *   Send `150%` of single-member line rate (`75%` from `ATE port-1` + `75%`
        from `ATE port-2`) IPv4 and IPv6 `ECT(0)` (`0b10`) traffic in queue
        `AF3`, and verify `0` drops and `0` `CE` marks across the uncongested
        2-member LAG.
*   Step 5 - Bring `ATE port-4` link down to trigger LAG member failover:
    *   Verify `/interfaces/interface/aggregation/state/lag-speed` drops to `1x`
        the testbed's dynamic port speed in Mbps (`100000` for `100G`, `400000`
        for `400G`, or `800000` for `800G`) on surviving member `DUT port-3`.
    *   Verify that the active `150%` flow now oversubscribes the single
        surviving member, exceeds `min-threshold` (`80000` bytes), and marks
        `ECN = 0b11` (`CE`) on `ATE port-3`.
    *   Restore `ATE port-4` and detach `DUT port-4` from `LAG_1` so `LAG_1`
        returns to `1x` the testbed's dynamic port speed (`100000`, `400000`, or
        `800000`).

### DP-1.12.4 - Validate Non-ECN Capable (Not-ECT) Traffic Under Congestion (Negative)

*   Step 1 - Send oversubscribed (`101%` of line rate = `51%` from `ATE port-1`
    and `50%` from `ATE port-2`) IPv4 and IPv6 traffic in queue `AF3` with ECN
    bits set to `Not-ECT` (`0b00`) towards `LAG_1`.
*   Step 2 - Verify that `100%` of received IPv4 and IPv6 packets on `ATE
    port-3` retain `ECN = 0b00` (`0` packets marked `CE`) and excess traffic is
    tail-dropped
    (`/qos/interfaces/interface/output/queues/queue/state/dropped-pkts > 0`).

### DP-1.12.5 - Validate Missing ECN Profile / Disabled ECN Behavior and Recovery (Negative)

*   Step 1 - Remove the ECN profile (e.g., `800GECNProfile` for `800G` ports)
    from queue `AF3` on `LAG_1` (or set
    `/qos/queue-management-profiles/queue-management-profile/wred/uniform/config/enable-ecn`
    to `false`) using `gNMI.Set`, while keeping the ECN profile enabled on
    `AF2`.
*   Step 2 - Send oversubscribed (`101%` of line rate) IPv4 and IPv6 `ECT(0)`
    (`0b10`) traffic to `AF3` and `AF2`.
*   Step 3 - Verify that `AF3` tail-drops excess traffic
    (`/qos/interfaces/interface/output/queues/queue/state/dropped-pkts > 0`)
    with `0` `CE` (`0b11`) marks (`100%` remain `0b10`), while `AF2` continues
    marking $\ge 99\%$ of received packets with `CE` (`0b11`).
*   Step 4 - Re-apply the ECN profile (`enable-ecn = true`, e.g.,
    `800GECNProfile` for `800G` ports) to `AF3`, push to DUT, and verify `CE`
    (`0b11`) marking is restored.

### DP-1.12.6 - Teardown and Cleanup Verification

*   Step 1 - Detach the ECN profile (e.g., `800GECNProfile` for `800G` ports)
    and `scheduler` from `LAG_1`, delete the ECN profile and `LAG_1`, and
    restore `DUT port-1`..`DUT port-4` and `ATE port-1`..`ATE port-4` to their
    baseline state using `gNMI.Set`.
*   Step 2 - Verify via `gNMI.Get` and `gNMI.Subscribe` that
    `/qos/interfaces/interface/output/queues/queue/state/queue-management-profile`
    no longer returns the ECN profile (e.g., `800GECNProfile`) and all ports
    report `/interfaces/interface/state/oper-status` as `UP`.

## Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "name": "LAG_1",
          "type": "iana-if-type:ieee8023adLag"
        },
        "name": "LAG_1"
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
            "interface-id": "LAG_1"
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
          "interface-id": "LAG_1",
          "interface-ref": {
            "config": {
              "interface": "LAG_1"
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
