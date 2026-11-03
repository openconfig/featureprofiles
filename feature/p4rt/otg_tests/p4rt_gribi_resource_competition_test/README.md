# P4RT-3.5: P4RT vs. gRIBI Hardware Resource Competition

## Summary

This test validates graceful hardware resource contention handling, failure
isolation, and telemetry accuracy when forwarding entities programmed by
`P4Runtime` (`P4RT`) and `gRIBI` compete for finite ASIC hardware tables (such
as shared TCAM, LPM, or NextHop banks). It verifies that exhausting hardware
capacity through one control-plane API produces deterministic resource
exhaustion responses (`RESOURCE_EXHAUSTED` in `P4RT` or `FIB_FAILED` in `gRIBI`
when sharing a hardware pool, or `OK` / `FIB_PROGRAMMED` when hardware
partitions are isolated), preserves all previously programmed forwarding and
punt entries with zero packet loss, maintains control-plane daemon stability,
accurately reports `/components/component/integrated-circuit/utilization/`
telemetry, and cleanly reclaims capacity upon entry deletion.

## Testbed type

* [`TESTBED_DUT_ATE_4LINKS`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

```text
                             +---------+
  [ ATE port1 ] ------------ |         | ------------ [ ATE port2 ] (Protected Egress)
  (Traffic & Punt Src)       |   DUT   |
                             | (ASIC   | ------------ [ ATE port3 ] (Scale Egress 1)
                             | Tables) |
                             |         | ------------ [ ATE port4 ] (Scale Egress 2)
                             +---------+
```

* Connect 4 interfaces between the DUT and ATE and configure dual-stack IPv4 and
  IPv6 addresses in the `DEFAULT` network instance:
  * **Link 1 (Traffic & Punt Ingress)**: DUT `port1` (`192.0.2.1/30`,
    `2001:db8:1::1/126`, P4RT `id = 1`) <-> ATE `port1` (`192.0.2.2/30`,
    `2001:db8:1::2/126`).
  * **Link 2 (Protected Baseline Egress)**: DUT `port2` (`192.0.2.5/30`,
    `2001:db8:2::1/126`, P4RT `id = 2`) <-> ATE `port2` (`192.0.2.6/30`,
    `2001:db8:2::2/126`).
  * **Link 3 (Scale Test Egress 1)**: DUT `port3` (`192.0.2.9/30`,
    `2001:db8:3::1/126`, P4RT `id = 3`) <-> ATE `port3` (`192.0.2.10/30`,
    `2001:db8:3::2/126`).
  * **Link 4 (Scale Test Egress 2)**: DUT `port4` (`192.0.2.13/30`,
    `2001:db8:4::1/126`, P4RT `id = 4`) <-> ATE `port4` (`192.0.2.14/30`,
    `2001:db8:4::2/126`).
* Verify all 4 DUT interfaces transition to `oper-status = UP` via
  `gNMI.Subscribe` (`ON_CHANGE`) on `/interfaces/interface/state/oper-status`.
* Configure `/components/component/integrated-circuit/config/node-id` (`1`) on
  the target forwarding ASIC (`IntegratedCircuit0`), interface
  `/interfaces/interface/config/id` (`1`–`4`), and system-wide hardware
  utilization watermarks (`used-threshold-upper = 80`,
  `used-threshold-upper-clear = 70`) under
  `/system/utilization/resources/resource/config/` for the target routing and
  ACL/TCAM hardware resources via `gNMI.Set`.
* Establish a `P4Runtime` client session (`device_id = 1`, `election_id = {high:
  0, low: 1}`), complete primary controller arbitration via
  `P4Runtime.StreamChannel`, and push the forwarding pipeline via
  `P4Runtime.SetForwardingPipelineConfig` (`VERIFY_AND_COMMIT`).
* Establish a `gRIBI` client session in `SINGLE_PRIMARY` mode with `PRESERVE`
  persistence and `FIB_ACK` requested (`election_id = {high: 0, low: 1}`), and
  flush `DEFAULT` via `gRIBI.Flush`.
* Establish persistent `gNMI.Subscribe` sessions (`ON_CHANGE` and `1s` `SAMPLE`)
  monitoring
  `/components/component/integrated-circuit/utilization/resources/resource/state/`
  (`used`, `free`, `max-limit`, `used-threshold-upper-exceeded`),
  `/network-instances/network-instance/afts/`, and
  `/interfaces/interface/state/counters/`.
* Start OTG protocols and explicitly resolve IPv4 ARP and IPv6 Neighbor
  Discovery (ND) across all 4 links before starting traffic.

### P4RT-3.5.1 - Baseline Protected Entries, Traffic, and Resource Telemetry

* **Step 1 - Record Initial Hardware Resource Utilization**:
  * Query the integrated-circuit resource telemetry path
    `/components/component/integrated-circuit/utilization/resources/resource/state/`
    on `IntegratedCircuit0` via `gNMI.Get` to record baseline `used`, `free`,
    and `max-limit` counts for all P4RT-programmable and gRIBI-programmable
    hardware tables, confirming `used-threshold-upper-exceeded = false`.
* **Step 2 - Program Protected gRIBI and P4RT Baseline Entries**:
  * Via `gRIBI.Modify` (`ADD`, `FIB_ACK`), install protected IPv4
    (`198.51.100.0/24`) and IPv6 (`2001:db8:100::/64`) routes in `DEFAULT`
    pointing to `NextHopGroup 10` (`NextHop 1` = `192.0.2.6`, `NextHop 2` =
    `2001:db8:2::2` on ATE `port2`). Confirm installation via `gRIBI.Get` and
    `gNMI.Subscribe` (`ON_CHANGE`).
  * Via `P4Runtime.Write` (`INSERT`), install protected high-priority ingress
    entries:
    * Punt rule matching LLDP (`ether_type == 0x88cc`, action
      `PuntToController`).
    * Permit rule matching protected transit destination `198.51.100.0/24` and
      `2001:db8:100::/64`.
* **Step 3 - Start Continuous Protected Traffic**:
  * From ATE `port1`, start continuous OTG streams (kept active across all
    subtests):
    * `Flow-Protected-v4`: `512`-byte IPv4 packets at `5,000 fps` to
      `198.51.100.1`.
    * `Flow-Protected-v6`: `512`-byte IPv6 packets at `5,000 fps` to
      `2001:db8:100::1`.
    * `Flow-Protected-LLDP`: LLDP frames at `100 fps`.
  * Verify `100%` of `Flow-Protected-v4` and `Flow-Protected-v6` packets arrive
    on ATE `port2` (`0%` loss) and `100%` of `Flow-Protected-LLDP` frames arrive
    as `PacketIn` messages on `P4Runtime.StreamChannel`.

### P4RT-3.5.2 - P4RT Table Exhaustion Followed by gRIBI Contention

* **Step 1 - Scale P4RT Table Entries to Hardware Exhaustion**:
  * Using batched `P4Runtime.Write` (`INSERT`) requests, iteratively program
    unique P4RT table entries matching non-overlapping test prefixes across
    `198.18.0.0/15` and `2001:db8:2000::/48` until the ASIC table reaches full
    capacity and `P4Runtime.Write` returns gRPC `RESOURCE_EXHAUSTED` for entry
    `N_p4rt + 1`.
  * Verify via `gNMI.Subscribe` that
    `/components/component/integrated-circuit/utilization/resources/resource/state/used`
    increases monotonically with installed entries, `free` approaches `0` (or
    the table reservation boundary), and `used-threshold-upper-exceeded`
    transitions to `true` once utilization exceeds `80%`.
* **Step 2 - Attempt gRIBI Route Programming Under P4RT Exhaustion**:
  * Issue `gRIBI.Modify` (`ADD`, `FIB_ACK`) for `200` new unique routes (`100`
    IPv4 `/32` routes `100.64.10.1/32`–`100.64.10.100/32` and `100` IPv6 `/128`
    routes `2001:db8:3000::1/128`–`2001:db8:3000::64/128`) referencing unique
    `NextHopGroups` and `NextHops` pointing to ATE `port3` (`192.0.2.10` /
    `2001:db8:3::2`):
    * **Shared Hardware Resource Pool**: Where the P4RT table and gRIBI AFT
      compete for the same exhausted hardware table/TCAM bank, verify
      `gRIBI.Modify` deterministically returns `FIB_FAILED` (without hanging or
      dropping the gRIBI stream), and confirm OTG probe packets sent to a
      `FIB_FAILED` destination (`100.64.10.1`) are dropped (`0` packets
      forwarded to `port3`).
    * **Dedicated Hardware Partition**: Where the ASIC enforces strict hardware
      partition isolation between P4RT TCAM and gRIBI LPM/NextHop tables, verify
      all `200` gRIBI routes return `FIB_PROGRAMMED` and forward OTG probe
      packets to ATE `port3` with `0%` loss despite `100%` P4RT exhaustion.
* **Step 3 - Stability and Isolation Pass/Fail Criteria**:
  * Protected baseline flows (`Flow-Protected-v4`, `Flow-Protected-v6`, and
    `Flow-Protected-LLDP`) sustain `0%` packet loss throughout P4RT table
    exhaustion and gRIBI contention.
  * `P4Runtime.Read` and `gRIBI.Get` remain responsive and match programmed
    hardware state without control-plane process crashes or session resets.

### P4RT-3.5.3 - P4RT Resource Reclamation and gRIBI Recovery

* **Step 1 - Reclaim 50% of Scaled P4RT Entries**:
  * Issue `P4Runtime.Write` (`DELETE`) to remove `50%` of the scaled P4RT
    entries installed in `P4RT-3.5.2`.
  * Verify via `gNMI.Subscribe` on
    `/components/component/integrated-circuit/utilization/resources/resource/state/`
    that `used` decreases by the deleted entry count, `free` increases
    correspondingly, and `used-threshold-upper-exceeded` clears to `false` once
    utilization drops below `70%` (`used-threshold-upper-clear`).
* **Step 2 - Verify gRIBI Programming Succeeds After Reclamation**:
  * Issue `gRIBI.Modify` (`ADD` or `REPLACE`, `FIB_ACK`) for the `200` gRIBI
    test routes (`100` IPv4 `/32` routes `100.64.10.1/32`–`100.64.10.100/32` and
    `100` IPv6 `/128` routes `2001:db8:3000::1/128`–`2001:db8:3000::64/128`).
  * Verify `100%` of the routes receive `FIB_PROGRAMMED`, appear in `gRIBI.Get`
    and `gNMI` AFT state, and forward OTG traffic from ATE `port1` to ATE
    `port3` with `0%` packet loss.
* **Step 3 - Reset Scale Entries for Reverse Direction Test**:
  * Delete the remaining non-baseline scaled P4RT entries and the `200`
    temporary gRIBI routes while keeping the protected baseline entries and
    protected traffic flows active.

### P4RT-3.5.4 - Reverse Direction: gRIBI FIB Exhaustion Followed by P4RT Contention

* **Step 1 - Scale gRIBI AFT Entries to Hardware Exhaustion**:
  * Continuously inject unique hierarchical gRIBI entries via `gRIBI.Modify`
    (`ADD`, `FIB_ACK`) across `100.65.0.0/16` (`/32` prefixes) and
    `2001:db8:4000::/48` (`/128` prefixes) referencing unique `NextHopGroups`
    and `NextHops` resolving via ATE `port3` and ATE `port4` until the first
    `FIB_FAILED` response is received at entry `M_gribi + 1`.
  * Verify via `gNMI.Subscribe` that FIB/NextHop hardware utilization `used`
    reaches capacity and `used-threshold-upper-exceeded` transitions to `true`.
  * Send OTG probe traffic to a `FIB_PROGRAMMED` gRIBI prefix (verifying `100%`
    forwarding to ATE `port3`/`port4`) and to the `FIB_FAILED` prefix (verifying
    `0` forwarded packets).
* **Step 2 - Attempt P4RT Programming Under gRIBI Exhaustion**:
  * Issue `P4Runtime.Write` (`INSERT`) for `100` new P4RT table entries matching
    `198.19.0.1`–`198.19.0.100`:
    * On shared hardware pools, verify `P4Runtime.Write` cleanly returns
      per-entry `RESOURCE_EXHAUSTED` without dropping `P4Runtime.StreamChannel`.
    * On isolated hardware partitions, verify `P4Runtime.Write` succeeds (`OK`)
      and enforces matching actions accurately.
  * Verify protected baseline flows (`Flow-Protected-v4`, `Flow-Protected-v6`,
    and `Flow-Protected-LLDP`) sustain `0%` packet loss.
* **Step 3 - Reclaim gRIBI Resources and Verify P4RT Recovery**:
  * Delete the scaled `100.65.0.0/16` and `2001:db8:4000::/48` gRIBI entries via
    `gRIBI.Modify` (`DELETE`), verify FIB utilization drops below
    `used-threshold-upper-clear` (`used-threshold-upper-exceeded = false`), and
    confirm a subsequent `P4Runtime.Write` (`INSERT`) for the `100` P4RT entries
    completes with `OK` status.

### Cleanup

* Delete all P4RT table entries via `P4Runtime.Write` (`DELETE`) and execute
  `gRIBI.Flush` on `DEFAULT`.
* Stop all OTG traffic streams and protocol sessions.
* Revert `/system/utilization/resources/`, integrated-circuit `node-id`, and
  interface configurations to the pre-test baseline, and verify hardware
  utilization `used` counters return to baseline levels.

## Canonical OC

```json
{
  "components": {
    "component": [
      {
        "config": {
          "name": "IntegratedCircuit0"
        },
        "integrated-circuit": {
          "config": {
            "node-id": "1"
          }
        },
        "name": "IntegratedCircuit0"
      }
    ]
  },
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "Traffic Ingress Port 1",
          "enabled": true,
          "id": 1,
          "name": "port1",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port1",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.1",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.1"
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
          "description": "Protected Egress Port 2",
          "enabled": true,
          "id": 2,
          "name": "port2",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port2",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.5",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.5"
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
                        "ip": "2001:db8:2::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8:2::1"
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
          "description": "Scale Egress 1 Port 3",
          "enabled": true,
          "id": 3,
          "name": "port3",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port3",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.9",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.9"
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
                        "ip": "2001:db8:3::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8:3::1"
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
          "description": "Scale Egress 2 Port 4",
          "enabled": true,
          "id": 4,
          "name": "port4",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port4",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.13",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.13"
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
                        "ip": "2001:db8:4::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8:4::1"
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
  },
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "DEFAULT",
          "type": "openconfig-network-instance-types:DEFAULT_INSTANCE"
        },
        "name": "DEFAULT"
      }
    ]
  },
  "system": {
    "utilization": {
      "resources": {
        "resource": [
          {
            "config": {
              "name": "FIB",
              "used-threshold-upper": 80,
              "used-threshold-upper-clear": 70
            },
            "name": "FIB"
          },
          {
            "config": {
              "name": "ACL",
              "used-threshold-upper": 80,
              "used-threshold-upper-clear": 70
            },
            "name": "ACL"
          }
        ]
      }
    }
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /components/component/integrated-circuit/config/node-id:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/utilization/resources/resource/state/free:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/utilization/resources/resource/state/max-limit:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/utilization/resources/resource/state/name:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/utilization/resources/resource/state/used:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/utilization/resources/resource/state/used-threshold-upper:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/utilization/resources/resource/state/used-threshold-upper-clear:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/utilization/resources/resource/state/used-threshold-upper-exceeded:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/id:
  /interfaces/interface/state/counters/in-pkts:
  /interfaces/interface/state/counters/out-pkts:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address:
  /system/utilization/resources/resource/config/name:
  /system/utilization/resources/resource/config/used-threshold-upper:
  /system/utilization/resources/resource/config/used-threshold-upper-clear:
  /system/utilization/resources/resource/state/name:
  /system/utilization/resources/resource/state/used-threshold-upper:
  /system/utilization/resources/resource/state/used-threshold-upper-clear:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
      on_change: true
  gribi:
    gRIBI.Flush:
    gRIBI.Get:
    gRIBI.Modify:
```

## Required DUT platform

* FFF
