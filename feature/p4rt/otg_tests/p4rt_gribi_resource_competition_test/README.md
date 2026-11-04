# P4RT-3.5: P4RT vs. gRIBI Hardware Resource Competition

## Summary

This test validates capacity admission control, failure isolation,
protocol-compliant error reporting, and telemetry accuracy when `P4Runtime`
(`P4RT`) Packet-I/O punt entries (`wbb.p4info.pb.txt` `acl_wbb_ingress_table`,
`size = 8`) and `gRIBI` Abstract Forwarding Table (AFT) entries operate
concurrently at table capacity limits. It verifies that:

* Saturating `acl_wbb_ingress_table` to its `P4Info` capacity (`8` valid WBB
  punt entries: LLDP `0x88cc`, ND/GDP `0x6007`, and IPv4/IPv6 Traceroute
  `ttl = 0, 1, 2`) and attempting an overflow/out-of-profile `N + 1` update via
  `P4Runtime.Write` (`Atomicity = CONTINUE_ON_ERROR`) returns outer gRPC status
  `UNKNOWN` wrapping per-update `p4.v1.Error` details (`canonical_code` =
  `RESOURCE_EXHAUSTED` when exceeding table/hardware capacity or
  `INVALID_ARGUMENT` / `UNIMPLEMENTED` when rejected by `@entry_restriction`)
  without dropping `P4Runtime.StreamChannel`, while `gRIBI.Modify` returns
  `FIB_PROGRAMMED` on isolated partitions (or `FIB_FAILED` on shared hardware
  pools).
* Exhausting hardware FIB/NextHop capacity via `gRIBI.Modify` deterministically
  returns `AFTResult.status = FIB_FAILED` inside `ModifyResponse` over an intact
  `OK` `gRIBI` gRPC stream (never a transport disconnect or top-level gRPC
  `RESOURCE_EXHAUSTED`), reports integrated-circuit FIB utilization via
  `/components/component/integrated-circuit/utilization/`, preserves all
  protected `gRIBI` forwarding and `P4RT` `PacketIn`/`PacketOut` operations with
  zero packet loss, allows isolated `P4RT` Packet-I/O punt rules to remain
  programmable (or returns per-update `RESOURCE_EXHAUSTED` if sharing hardware
  memory), and cleanly reclaims capacity upon entry deletion.

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
  `P4Runtime.StreamChannel`, and push the WBB forwarding pipeline
  (`feature/p4rt/data/wbb.p4info.pb.txt`, defining `acl_wbb_ingress_table` with
  `size = 8`, actions `acl_wbb_ingress_copy` and `acl_wbb_ingress_trap`, and
  `packet_in` / `packet_out` controller metadata) via
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
    and `max-limit` counts for all reported routing/FIB/NextHop hardware tables
    (and `ACL` tables where exposed by the platform), confirming
    `used-threshold-upper-exceeded = false`.
* **Step 2 - Program Protected gRIBI Routes and P4RT Punt Entry**:
  * Via `gRIBI.Modify` (`ADD`, `FIB_ACK`), install protected IPv4
    (`198.51.100.0/24`) and IPv6 (`2001:db8:100::/64`) routes in `DEFAULT`
    pointing to `NextHopGroup 10` (`NextHop 1` = `192.0.2.6`, `NextHop 2` =
    `2001:db8:2::2` on ATE `port2`). Confirm `AFTResult.status = FIB_PROGRAMMED`
    and verify installation via `gRIBI.Get` and `gNMI.Subscribe` (`ON_CHANGE`).
  * Via `P4Runtime.Write` (`INSERT`, `Atomicity = CONTINUE_ON_ERROR`), install
    the protected `acl_wbb_ingress_table` punt entry matching LLDP frames
    (`ether_type == 0x88cc & 0xffff`, `priority = 1`, action
    `acl_wbb_ingress_trap` or `acl_wbb_ingress_copy`), and verify presence via
    `P4Runtime.Read`.
* **Step 3 - Start Continuous Protected Traffic and Validate Packet-I/O**:
  * From ATE `port1`, start continuous OTG streams (kept active across all
    subtests):
    * `Flow-Protected-v4`: `512`-byte IPv4 packets (`TTL = 64`) at `5,000 fps`
      to `198.51.100.1`.
    * `Flow-Protected-v6`: `512`-byte IPv6 packets (`HopLimit = 64`) at `5,000
      fps` to `2001:db8:100::1`.
    * `Flow-Protected-LLDP`: LLDP frames (`ether_type = 0x88cc`) at `100 fps`.
  * Verify `100%` of `Flow-Protected-v4` and `Flow-Protected-v6` packets arrive
    on ATE `port2` (`0%` loss), `100%` of `Flow-Protected-LLDP` frames arrive as
    `PacketIn` messages on `P4Runtime.StreamChannel` (`ingress_port = "1"`), and
    a baseline `PacketOut` frame injected via `P4Runtime.StreamChannel`
    (`egress_port = "2"`) is received on ATE `port2`.

### P4RT-3.5.2 - P4RT Punt Table Saturation Followed by gRIBI Contention

* **Step 1 - Saturate `acl_wbb_ingress_table` and Verify Overflow Rejection**:
  * Using `P4Runtime.Write` (`INSERT`, `Atomicity = CONTINUE_ON_ERROR`), install
    the remaining `7` valid WBB punt entries permitted by `wbb.p4info.pb.txt` in
    `acl_wbb_ingress_table` so all `8` table slots (`size = 8`) are occupied (or
    fill up to the pipeline's advertised table `size` if an extended hardware
    table `P4Info` is loaded):
    * ND/GDP punt entry (`ether_type == 0x6007 & 0xffff`, `priority = 1`).
    * `3` IPv4 Traceroute punt entries (`is_ipv4 == 0x1`, `ttl == 0 & 0xff`,
      `ttl == 1 & 0xff`, and `ttl == 2 & 0xff`, `priority = 1`).
    * `3` IPv6 Traceroute punt entries (`is_ipv6 == 0x1`, `ttl == 0 & 0xff`,
      `ttl == 1 & 0xff`, and `ttl == 2 & 0xff`, `priority = 1`).
  * Confirm via `P4Runtime.Read` that all `8` punt entries are active.
  * Issue an additional `P4Runtime.Write` (`INSERT`, `Atomicity =
    CONTINUE_ON_ERROR`) attempting entry `N_p4rt + 1` beyond the `size = 8`
    capacity / `@entry_restriction` boundary:
    * Verify `P4Runtime.Write` rejects the `N_p4rt + 1` update cleanly with
      outer gRPC status `UNKNOWN` (`codes.Unknown`) containing per-update
      `p4.v1.Error` details (`canonical_code = RESOURCE_EXHAUSTED` when rejected
      for table/hardware capacity, or `canonical_code = INVALID_ARGUMENT` /
      `UNIMPLEMENTED` when rejected by `@entry_restriction` admission control),
      and confirm it never returns a session/transport failure (`UNAVAILABLE`,
      `DEADLINE_EXCEEDED`, `CANCELED`, or `PERMISSION_DENIED`) or drops
      `P4Runtime.StreamChannel`.
    * On platforms where `acl_wbb_ingress_table` consumes a hardware-reported
      `/components/component/integrated-circuit/utilization/resources/resource/state/`
      counter, verify `used` reflects the installed entries without exceeding
      `max-limit` (noting that `size = 8` Packet-I/O punt implementations do not
      trigger the `80%` bulk ASIC `ACL` watermark).
* **Step 2 - Attempt gRIBI Programming Under Full P4RT Punt Table Saturation**:
  * Issue `gRIBI.Modify` (`ADD`, `FIB_ACK`) for `200` new unique routes (`100`
    IPv4 `/32` routes `100.64.10.1/32`–`100.64.10.100/32` and `100` IPv6 `/128`
    routes `2001:db8:3000::1/128`–`2001:db8:3000::64/128`) referencing unique
    `NextHopGroups` and `NextHops` pointing to ATE `port3` (`192.0.2.10` /
    `2001:db8:3::2`):
    * **Dedicated / Isolated Partition (Standard WBB Packet-I/O Behavior)**:
      Where `P4RT` punt entries and `gRIBI` LPM/NextHop tables use independent
      partitions, verify all `200` `gRIBI.Modify` operations return
      `AFTResult.status = FIB_PROGRAMMED`, OTG probe packets (`TTL = 64`) to the
      `200` prefixes forward to ATE `port3` with `0%` loss, and IPv4/IPv6
      Traceroute probe packets (`TTL = 1`) injected on ATE `port1` arrive as
      `PacketIn` messages on `P4Runtime.StreamChannel`.
    * **Shared Hardware Resource Pool**: Where an extended hardware-backed
      `P4RT` table and `gRIBI` AFT compete for the same exhausted ASIC bank,
      verify `gRIBI.Modify` deterministically returns `AFTResult.status =
      FIB_FAILED` inside `ModifyResponse` while keeping the `gRIBI` gRPC stream
      `OK`, and confirm OTG probe packets (`TTL = 64`) sent to a `FIB_FAILED`
      destination (`100.64.10.1`) are dropped (`0` packets forwarded to
      `port3`).
* **Step 3 - Stability and Isolation Pass/Fail Criteria**:
  * Protected baseline flows (`Flow-Protected-v4`, `Flow-Protected-v6`, and
    `Flow-Protected-LLDP`) sustain `0%` packet loss throughout `P4RT` table
    saturation and `gRIBI` route programming.
  * `P4Runtime.Read` and `gRIBI.Get` remain responsive and match programmed
    state without control-plane process crashes or session resets.

### P4RT-3.5.3 - P4RT Punt Entry Reclamation and gRIBI Verification

* **Step 1 - Reclaim Non-Baseline P4RT Punt Entries**:
  * Issue `P4Runtime.Write` (`DELETE`, `Atomicity = CONTINUE_ON_ERROR`) to
    remove the `7` non-baseline WBB punt entries installed in `P4RT-3.5.2`
    (ND/GDP `0x6007` and the `6` IPv4/IPv6 Traceroute `ttl = 0, 1, 2` entries),
    retaining only the protected LLDP (`0x88cc`) punt entry.
  * Confirm via `P4Runtime.Read` that only the single protected LLDP entry
    remains in `acl_wbb_ingress_table` (and where an integrated-circuit `ACL`
    utilization counter incremented in `P4RT-3.5.2`, verify `used` decreases
    correspondingly).
* **Step 2 - Verify gRIBI Programming Succeeds After Reclamation**:
  * Issue `gRIBI.Modify` (`ADD` or `REPLACE`, `FIB_ACK`) for the `200` gRIBI
    test routes (`100` IPv4 `/32` routes `100.64.10.1/32`–`100.64.10.100/32` and
    `100` IPv6 `/128` routes `2001:db8:3000::1/128`–`2001:db8:3000::64/128`).
  * Verify `100%` of the routes receive `AFTResult.status = FIB_PROGRAMMED`,
    appear in `gRIBI.Get` and `gNMI` AFT state, and forward OTG traffic (`TTL =
    64`) from ATE `port1` to ATE `port3` with `0%` packet loss.
* **Step 3 - Reset Temporary Scale Entries for Reverse Direction Test**:
  * Delete the `200` temporary gRIBI routes via `gRIBI.Modify` (`DELETE`,
    `FIB_ACK`) while keeping the protected baseline gRIBI routes, protected LLDP
    punt rule, and continuous protected traffic flows active.

### P4RT-3.5.4 - gRIBI Hardware FIB Exhaustion Followed by P4RT Contention

* **Step 1 - Scale gRIBI AFT Entries to Hardware Exhaustion (`FIB_FAILED`)**:
  * Continuously inject unique hierarchical gRIBI entries via `gRIBI.Modify`
    (`ADD`, `FIB_ACK`) across `100.65.0.0/16` (`/32` prefixes) and
    `2001:db8:4000::/48` (`/128` prefixes) referencing unique `NextHopGroups`
    and `NextHops` resolving via ATE `port3` and ATE `port4` until hardware
    FIB/NextHop capacity is reached and `gRIBI.Modify` returns
    `AFTResult.status = FIB_FAILED` inside `ModifyResponse` at entry
    `M_gribi + 1` (confirming the `gRIBI` gRPC stream stays `OK` rather than
    returning a gRPC transport error or top-level `RESOURCE_EXHAUSTED`).
  * Verify via `gNMI.Subscribe` on
    `/components/component/integrated-circuit/utilization/resources/resource/state/`
    that FIB/NextHop hardware utilization `used` reaches capacity and
    `used-threshold-upper-exceeded` transitions to `true` once utilization
    exceeds `80%`.
  * Send OTG probe traffic (`TTL = 64`) to a `FIB_PROGRAMMED` gRIBI prefix
    (verifying `100%` forwarding to ATE `port3`/`port4`) and to the `FIB_FAILED`
    prefix (verifying `0` forwarded packets).
* **Step 2 - Validate P4RT Programming and Packet-I/O Under gRIBI Exhaustion**:
  * While hardware FIB remains exhausted (`FIB_FAILED`), issue `P4Runtime.Write`
    (`INSERT`, `Atomicity = CONTINUE_ON_ERROR`) for the `7` remaining valid
    `wbb.p4info.pb.txt` `acl_wbb_ingress_table` punt entries (ND/GDP `0x6007`
    and the `6` IPv4/IPv6 Traceroute `ttl = 0, 1, 2` entries):
    * **Dedicated / Isolated Partition**: Where `P4RT` Packet-I/O punt entries
      are isolated from hardware LPM/NextHop FIB tables, verify
      `P4Runtime.Write` succeeds (`OK`), `P4Runtime.Read` returns all `8` punt
      entries, IPv4/IPv6 Traceroute probe packets (`TTL = 1`) injected on ATE
      `port1` arrive as `PacketIn` messages on `P4Runtime.StreamChannel`, and
      `PacketOut` frames (`egress_port = "2"`) egress DUT `port2` with `0%`
      loss.
    * **Shared Hardware Resource Pool**: Where `P4RT` table entries compete for
      the same exhausted ASIC memory pool as `gRIBI` AFT entries, verify
      `P4Runtime.Write` cleanly returns outer gRPC `UNKNOWN` wrapping per-update
      `p4.v1.Error` (`canonical_code = RESOURCE_EXHAUSTED`) without dropping
      `P4Runtime.StreamChannel`.
  * Verify protected baseline flows (`Flow-Protected-v4`, `Flow-Protected-v6`,
    and `Flow-Protected-LLDP`) sustain `0%` packet loss.
* **Step 3 - Reclaim gRIBI Resources and Verify Full Recovery**:
  * Delete the scaled `100.65.0.0/16` and `2001:db8:4000::/48` gRIBI entries via
    `gRIBI.Modify` (`DELETE`, `FIB_ACK`), verify FIB utilization drops below
    `used-threshold-upper-clear` (`70%`, `used-threshold-upper-exceeded =
    false`), and confirm both a subsequent `gRIBI.Modify` (`ADD`, `FIB_ACK`)
    probe route (`FIB_PROGRAMMED`) and `P4Runtime.Write` (`INSERT` / `MODIFY`)
    for the WBB punt entries complete with `OK` status.

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
