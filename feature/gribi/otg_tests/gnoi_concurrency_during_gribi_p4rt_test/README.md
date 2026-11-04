# TE-1.24: gNOI Operations During gRIBI and P4RT Programming

## Summary

This test validates control-plane and data-plane resilience when operational
management RPCs via `gNOI` (`System`, `File`, `Healthz`, and `PacketLinkQual` /
`Diag` on non-transit interfaces) execute concurrently with high-rate `gRIBI`
Abstract Forwarding Table (AFT) route modifications and `P4Runtime` (`P4RT`)
Packet-I/O punt (`wbb.p4info.pb.txt` `acl_wbb_ingress_table`) and `PacketOut`
operations. It ensures that gNOI operations complete accurately without
destabilizing, starving, or dropping active `gRIBI`, `P4RT`, or `gNMI` sessions,
and that transit data-plane forwarding and Packet-In punting sustain zero packet
loss.

## Testbed type

* [`TESTBED_DUT_ATE_4LINKS`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

```text
                             +---------+
  [ ATE port1 ] ------------ |         | ------------ [ ATE port2 ] (Transit Egress 1)
  (Transit & Punt Src)       |   DUT   | ------------ [ ATE port3 ] (Transit Egress 2)
                             |         |
                             |         | ------------ [ ATE port4 ] (Non-Transit gNOI Port)
                             +---------+
```

* Connect 4 interfaces between the DUT and ATE and configure dual-stack IPv4 and
  IPv6 addresses in the `DEFAULT` network instance:
  * **Link 1 (Transit & Packet-In Ingress)**: DUT `port1` (`192.0.2.1/30`,
    `2001:db8:1::1/126`, P4RT `id = 1`) <-> ATE `port1` (`192.0.2.2/30`,
    `2001:db8:1::2/126`).
  * **Link 2 (Transit Egress 1)**: DUT `port2` (`192.0.2.5/30`,
    `2001:db8:2::1/126`, P4RT `id = 2`) <-> ATE `port2` (`192.0.2.6/30`,
    `2001:db8:2::2/126`).
  * **Link 3 (Transit Egress 2)**: DUT `port3` (`192.0.2.9/30`,
    `2001:db8:3::1/126`, P4RT `id = 3`) <-> ATE `port3` (`192.0.2.10/30`,
    `2001:db8:3::2/126`).
  * **Link 4 (Dedicated Non-Transit Port)**: DUT `port4` (`192.0.2.13/30`,
    `2001:db8:4::1/126`, P4RT `id = 4`) <-> ATE `port4` (`192.0.2.14/30`,
    `2001:db8:4::2/126`).
* Verify all 4 DUT interfaces report `oper-status = UP` via `gNMI.Subscribe`
  (`ON_CHANGE`) on `/interfaces/interface/state/oper-status`.
* Configure `/components/component/integrated-circuit/config/node-id` (e.g.,
  `device_id = 1`) for the target forwarding integrated circuit and interface
  `/interfaces/interface/config/id` values (`1`–`4`) via `gNMI.Set`.
* Establish a `P4Runtime` client session (`device_id = 1`, `election_id = {high:
  0, low: 1}`), complete primary controller arbitration via
  `P4Runtime.StreamChannel`, and push the WBB forwarding pipeline
  (`feature/p4rt/data/wbb.p4info.pb.txt`, defining `acl_wbb_ingress_table` with
  `size = 8`, actions `acl_wbb_ingress_copy` and `acl_wbb_ingress_trap`, and
  `packet_in` / `packet_out` metadata) via
  `P4Runtime.SetForwardingPipelineConfig` (`VERIFY_AND_COMMIT`).
* Establish a `gRIBI` client session in `SINGLE_PRIMARY` mode with `PRESERVE`
  persistence and `FIB_ACK` requested, elect the client as leader, and flush the
  `DEFAULT` network instance via `gRIBI.Flush`.
* Start OTG protocols and explicitly resolve IPv4 ARP and IPv6 ND on all 4 links
  before starting traffic.

### TE-1.24.1 - Baseline Forwarding, Packet-In, and Programming Latency

* **Step 1 - Program Protected Baseline Entries**:
  * Via `gRIBI.Modify` (`ADD`, `FIB_ACK`), program protected steady-state routes
    in `DEFAULT` (`198.51.100.0/24` and `2001:db8:100::/64`) pointing to
    `NextHopGroup 10` (`NextHop 1` = `192.0.2.6` / `NextHop 2` = `2001:db8:2::2`
    on ATE `port2`).
  * Via `P4Runtime.Write` (`INSERT`, `Atomicity = CONTINUE_ON_ERROR`), program
    the protected `acl_wbb_ingress_table` punt rule for LLDP frames
    (`ether_type == 0x88cc & 0xffff`, `priority = 1`, action
    `acl_wbb_ingress_trap` or `acl_wbb_ingress_copy`), and verify presence via
    `P4Runtime.Read`.
  * Confirm baseline AFT state via `gRIBI.Get` and `gNMI.Subscribe`
    (`ON_CHANGE`).
* **Step 2 - Start Protected Transit and Packet-In Traffic**:
  * Start continuous IPv4 (`TTL = 64`) and IPv6 (`HopLimit = 64`) OTG transit
    flows (`512`-byte frames, `5,000 fps` each) from ATE `port1` to
    `198.51.100.1` and `2001:db8:100::1`.
  * Start continuous LLDP injection (`100 fps`) on ATE `port1` and verify `100%`
    of punted frames arrive on `P4Runtime.StreamChannel` as `PacketIn` messages
    while `100%` of transit packets egress ATE `port2` (`0%` loss).
* **Step 3 - Measure Baseline Control-Plane Programming Rate**:
  * Measure the baseline completion time `T_base` to program and delete `2,000`
    gRIBI IPv4/IPv6 routes (`100.64.0.0/24`–`100.71.207.0/24`) and execute `100`
    `P4Runtime.Write` (`INSERT` -> `MODIFY` -> `DELETE`) cycles of the `7`
    non-baseline WBB `acl_wbb_ingress_table` punt entries (`ether_type ==
    0x6007 & 0xffff` ND/GDP and `6` IPv4/IPv6 Traceroute `ttl == 0, 1, 2 &
    0xff` rules) plus `PacketOut` frames (`egress_port = "2"`) without
    concurrent gNOI load.

### TE-1.24.2 - System and Healthz gNOI Operations Under gRIBI/P4RT Churn

* **Step 1 - Launch Concurrent gRIBI and P4RT Churn**:
  * While the protected transit flows (`10,000 fps` aggregate) and LLDP
    `PacketIn` stream (`100 fps`) remain active on `port1 -> port2`,
    continuously stream:
    * `gRIBI.Modify` (`ADD` -> `REPLACE` -> `DELETE` cycles) across `10,000`
      IPv4 `/32` routes (`100.64.0.1/32`–`100.64.39.16/32`) and `10,000` IPv6
      `/128` routes (`2001:db8:1000::1/128`–`2001:db8:1000::2710/128`) toggling
      ECMP NextHopGroups across ATE `port2` and ATE `port3`.
    * `P4Runtime.Write` (`INSERT` -> `MODIFY` -> `DELETE`, `Atomicity =
      CONTINUE_ON_ERROR`) batch loops cycling the `7` non-baseline
      `wbb.p4info.pb.txt` `acl_wbb_ingress_table` punt entries (`ether_type ==
      0x6007 & 0xffff` ND/GDP and `6` IPv4/IPv6 Traceroute `ttl == 0, 1, 2 &
      0xff` rules while keeping the protected `0x88cc` LLDP punt entry active)
      plus continuous `PacketOut` frame injection (`100 fps`, `egress_port =
      "2"`).
* **Step 2 - Execute Concurrent System & Healthz gNOI RPCs**:
  * Concurrently issue 20 back-to-back iterations of:
    * `gnoi.system.System.Time` (validating monotonic timestamp responses).
    * `gnoi.system.System.Ping` from the DUT to `192.0.2.14` and `2001:db8:4::2`
      on non-transit ATE `port4` (`count = 5` per call).
    * `gnoi.system.System.Traceroute` to `192.0.2.14` on ATE `port4`.
    * `gnoi.healthz.Healthz.List` and `gnoi.healthz.Healthz.Get` across active
      `CHASSIS` and `CONTROLLER_CARD` components.
* **Step 3 - Pass/Fail Criteria**:
  * `100%` of `gRIBI.Modify` operations return `FIB_PROGRAMMED` without gRPC
    stream disconnects, unbounded stalls relative to `T_base`, or
    `DEADLINE_EXCEEDED` errors.
  * `100%` of `P4Runtime.Write` batches succeed and `P4Runtime.Read` matches
    expected table state.
  * `100%` of `gNOI` `Time`, `Ping`, `Traceroute`, and `Healthz` RPCs succeed.
  * Protected transit traffic on `port2` and LLDP `PacketIn` messages sustain
    `0%` packet loss.

### TE-1.24.3 - File Service gNOI Operations During gRIBI/P4RT Programming

* **Step 1 - Execute Large File Transfers Under Control-Plane Churn**:
  * With the `gRIBI.Modify` and `P4Runtime.Write` churn loops and OTG transit
    streams still running, execute 5 consecutive file lifecycle cycles via
    `gnoi.file.File`:
    1. Stream a `10 MB` synthetic diagnostic payload to `/tmp/fnt_gnoi_blob.bin`
       using `gnoi.file.File.Put` (`64 KB` chunks) and finalize with its MD5 /
       SHA-256 hash.
    2. Query `/tmp/fnt_gnoi_blob.bin` via `gnoi.file.File.Stat` and verify exact
       byte size (`10,485,760` bytes).
    3. Download the file via `gnoi.file.File.Get` and verify byte-for-byte hash
       integrity.
    4. Delete `/tmp/fnt_gnoi_blob.bin` via `gnoi.file.File.Remove` and verify
       removal via `Stat` (`NOT_FOUND`).
* **Step 2 - Pass/Fail Criteria**:
  * All 5 `Put` / `Stat` / `Get` / `Remove` cycles succeed with `100%` checksum
    integrity.
  * Concurrent `gRIBI` and `P4RT` programming continues with `0` failed RPCs,
    `0` session drops, and `0%` transit or `PacketIn` packet loss.

### TE-1.24.4 - Non-Transit Port gNOI Diagnostic Isolation

* **Step 1 - Run Intrusive Port-Scoped gNOI Operations on Non-Transit `port4`**:
  * While transit traffic (`port1 -> port2` and `port3`) and `gRIBI` / `P4RT`
    programming churn remain active on transit ports `port1`–`port3`, execute
    port-level `gNOI` link qualification
    (`gnoi.packet_link_qualification.LinkQualification.Create`, `Get`, `Delete`)
    or port diagnostic loopback/BERT operations exclusively on non-transit
    `port4`.
* **Step 2 - Pass/Fail Criteria**:
  * Verify via `gNMI.Subscribe` (`SAMPLE`) and OTG flow metrics that transit
    ports `port1`, `port2`, and `port3` remain `UP` with `0%` packet loss.
  * Verify `100%` of concurrent `gRIBI.Modify` (`FIB_PROGRAMMED`) and
    `P4Runtime.Write` operations on transit ports complete cleanly without
    error.
  * Verify `/system/processes/process/state/` and `/components/component/state/`
    show no process crashes or unexpected restarts.

### Cleanup

* Ensure `/tmp/fnt_gnoi_blob.bin` is removed via `gnoi.file.File.Remove` and any
  active link qualification session on `port4` is deleted via `Delete` so
  `port4` returns to normal `UP` forwarding state.
* Delete all P4RT table entries via `P4Runtime.Write` (`DELETE`) and flush all
  gRIBI AFT entries via `gRIBI.Flush` on `DEFAULT`.
* Stop OTG traffic and protocols and restore baseline DUT configuration.

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
          "description": "Transit Ingress Port 1",
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
          "description": "Transit Egress 1 Port 2",
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
          "description": "Transit Egress 2 Port 3",
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
          "description": "Non-Transit gNOI Port 4",
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
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /components/component/integrated-circuit/config/node-id:
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
  /system/processes/process/state/name:
  /system/processes/process/state/pid:
  /system/processes/process/state/start-time:
  /system/state/current-datetime:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
      on_change: true
  gnoi:
    file.File.Get:
    file.File.Put:
    file.File.Remove:
    file.File.Stat:
    healthz.Healthz.Get:
    healthz.Healthz.List:
    packet_link_qualification.LinkQualification.Create:
    packet_link_qualification.LinkQualification.Delete:
    packet_link_qualification.LinkQualification.Get:
    system.System.Ping:
    system.System.Time:
    system.System.Traceroute:
  gribi:
    gRIBI.Flush:
    gRIBI.Get:
    gRIBI.Modify:
```

## Required DUT platform

* FFF
