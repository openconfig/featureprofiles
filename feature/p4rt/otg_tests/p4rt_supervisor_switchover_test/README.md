# P4RT-1.4: P4RT PacketIn Continuity across Supervisor Switchover

## Summary

Validate that P4RT packet punting keeps working after a supervisor (controller
card) switchover on a modular DUT. A P4RT controller relies on the DUT punting
control protocol packets, such as LLDP, Google Discovery Protocol (GDP) and
traceroute packets, to the controller as PacketIn messages. If the DUT stops
punting after a switchover, the controller loses neighbor discovery on every
affected port even though the P4RT session, the forwarding pipeline and the
table entries appear to be correctly programmed, and PacketOut still works.

This test verifies that after a supervisor switchover triggered with gNOI
`System.SwitchControlProcessor`, a P4RT controller that reconnects to the DUT
and re-programs its forwarding pipeline and punt table entries receives
PacketIn messages for every punted packet type, on a standalone port and on
each member of a LACP aggregate interface (LAG), with the correct ingress port
metadata. The check is performed after a switchover in each direction between
the two controller cards, and for a port that is enabled immediately after a
switchover. Data plane traffic forwarded by gRIBI entries must continue to be
forwarded throughout.

## Testbed type

* [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

The DUT and ATE are connected with 4 links. DUT port-1 is a standalone port,
DUT port-2 and DUT port-3 are the two members of a LACP aggregate interface
`lag1`, and DUT port-4 is the egress port for data plane traffic programmed by
gRIBI.

```text
ATE port-1 <------> DUT port-1      (standalone)
ATE port-2 <------> DUT port-2 --+
                                 +-- lag1 (LACP)
ATE port-3 <------> DUT port-3 --+
ATE port-4 <------> DUT port-4      (gRIBI egress)
```

* Configure the DUT interfaces as follows:
    * DUT port-1: IPv4 `192.0.2.1/30` and IPv6 `2001:db8::1/126`. ATE port-1
      uses `192.0.2.2/30` and `2001:db8::2/126`.
    * DUT `lag1` with DUT port-2 and DUT port-3 as members
      (`/interfaces/interface/ethernet/config/aggregate-id`), LAG type `LACP`
      (`/interfaces/interface/aggregation/config/lag-type`) and LACP mode
      `ACTIVE` (`/lacp/interfaces/interface/config/lacp-mode`): IPv4
      `192.0.2.5/30` and IPv6 `2001:db8::5/126`. The ATE configures a matching
      LACP LAG on ATE port-2 and ATE port-3 with `192.0.2.6/30` and
      `2001:db8::6/126`.
    * DUT port-4: IPv4 `192.0.2.9/30` and IPv6 `2001:db8::9/126`. ATE port-4
      uses `192.0.2.10/30` and `2001:db8::a/126`.
* Assign a P4RT interface ID to each DUT interface using
  `/interfaces/interface/config/id`: DUT port-1 = `1`, DUT port-2 = `2`, DUT
  port-3 = `3`, DUT port-4 = `4` and `lag1` = `10`. Verify that
  `/interfaces/interface/state/id` reports the same values.
* Configure a P4RT node ID using
  `/components/component/integrated-circuit/config/node-id` on every
  `INTEGRATED_CIRCUIT` component that hosts one of the DUT ports above. Where
  the DUT ports are hosted on more than one integrated circuit, each integrated
  circuit gets a unique node ID (for example `1` and `2`).
* Disable on-box LLDP processing by setting `/lldp/config/enabled` to `false`,
  so that LLDP packets are only handled through P4RT punting.
* Verify that DUT port-1, DUT port-2, DUT port-3, DUT port-4 and `lag1` report
  `/interfaces/interface/state/oper-status` `UP`, and that both LAG members
  report `/lacp/interfaces/interface/members/member/state/synchronization`
  `IN_SYNC`.
* Connect a gRIBI client to the DUT with persistence mode `PRESERVE`,
  `SINGLE_PRIMARY` client redundancy and `FIB_ACK`, and make it leader.
  Program an `IPv4Entry` for `198.51.100.0/24` and an `IPv6Entry` for
  `2001:db8:2::/64` in the default network instance, each pointing to a
  next-hop-group containing a single next hop towards ATE port-4. Verify that
  both entries are reported through AFT telemetry
  (`/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix`
  and
  `/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix`)
  and acknowledged as `FIB_PROGRAMMED`.
* Connect a P4RT client to each P4RT node ID. Send a
  `MasterArbitrationUpdate` on `P4Runtime.StreamChannel` with election ID
  `{high: 0, low: 1}` and verify that the client becomes primary.
* Push the forwarding pipeline configuration with
  `P4Runtime.SetForwardingPipelineConfig` (action `VERIFY_AND_COMMIT`).
* Install the following 4 punt entries in `acl_wbb_ingress_table` with action
  `acl_wbb_ingress_trap` using `P4Runtime.Write` (`INSERT`):
    * LLDP: `ether_type` = `0x88cc`.
    * GDP: `ether_type` = `0x6007`.
    * IPv4 traceroute: `is_ipv4` = `1` and `ttl` = `1`.
    * IPv6 traceroute: `is_ipv6` = `1` and `ttl` (hop limit) = `1`.
* Verify with `P4Runtime.Read` that exactly these 4 entries are installed.

### PacketIn and PacketOut validation procedure

The following validation is referenced by each subtest below.

* From each of ATE port-1, ATE port-2 and ATE port-3 (each LAG member port
  individually), send 10 LLDP packets (destination MAC `01:80:c2:00:00:0e`)
  and 10 GDP packets (destination MAC `00:0a:da:f0:f0:f0`).
* From ATE port-1 and from the ATE LAG, send 10 IPv4 packets with TTL `1` to
  `198.51.100.1`, and 10 IPv6 packets with hop limit `1` to `2001:db8:2::1`.
* Pass criteria:
    * The primary P4RT client receives one PacketIn message for every packet
      sent, for each packet type and each ATE port, within 60 seconds of the
      P4RT client completing the `P4Runtime.Write` of the punt entries,
      without any manual recovery action on the DUT. A PacketIn count of `0`,
      or any count lower than the number of packets sent, is a failure.
    * Every PacketIn message carries the `ingress_singleton_port` metadata,
      and its value equals the interface ID of the DUT port the packet was
      received on (`1`, `2` or `3`). For traceroute packets received on
      `lag1`, the value is the interface ID of the LAG member (`2` or `3`)
      that received the packet.
    * Every traceroute PacketIn message also carries the
      `egress_singleton_port` metadata with the interface ID of DUT port-4
      (`4`), the egress port of the gRIBI route for the destination.
* Send 10 LLDP and 10 GDP PacketOut messages from the primary P4RT client
  with `egress_singleton_port` set to `1` (DUT port-1), and 10 of each with
  `egress_singleton_port` set to `2` (DUT port-2). Verify that ATE port-1 and
  ATE port-2 receive exactly 20 packets each.

### P4RT-1.4.1 - PacketIn after supervisor switchover with controller reconnect

This subtest emulates a P4RT controller that reconnects to the new primary
controller card as soon as possible after a switchover and immediately
re-programs its pipeline and punt entries, which is the most demanding timing
for the DUT.

* Step 1 - Baseline
    * Run the PacketIn and PacketOut validation procedure and verify that it
      passes.
    * Use gNMI Get on `/components/component/state/redundant-role` for all
      `CONTROLLER_CARD` components to identify the primary controller card
      (referred to as `SUP-A`) and the standby controller card (referred to as
      `SUP-B`).
    * Use gNMI Subscribe `ON_CHANGE` to wait until
      `/components/component/state/switchover-ready` reports `true` for
      `SUP-B`.
    * Start continuous IPv4 traffic from ATE port-1 to `198.51.100.0/24` and
      IPv6 traffic from ATE port-1 to `2001:db8:2::/64`, at 1000 packets per
      second each.

* Step 2 - Trigger supervisor switchover
    * Record the current time and issue gNOI `System.SwitchControlProcessor`
      with `control_processor` set to `SUP-B`.
    * Use gNMI Subscribe to wait until `SUP-B` reports
      `/components/component/state/redundant-role` as `PRIMARY` and `SUP-A`
      reports `SECONDARY`.
    * Verify that `/components/component/state/last-switchover-time` of
      `SUP-B` is later than the recorded time and that
      `/components/component/state/last-switchover-reason/trigger` is
      `USER_INITIATED`.

* Step 3 - Reconnect the P4RT client and re-program punt entries
    * As soon as the DUT accepts P4RT connections, reconnect a P4RT client to
      each P4RT node ID with election ID `{high: 0, low: 2}`, which is higher
      than the election ID used before the switchover, and verify that it
      becomes primary.
    * Push the forwarding pipeline configuration again with
      `P4Runtime.SetForwardingPipelineConfig` (action `VERIFY_AND_COMMIT`) and
      write the 4 punt entries again. An `ALREADY_EXISTS` error for an entry
      that persisted across the switchover is acceptable.
    * Verify with `P4Runtime.Read` that exactly the 4 punt entries are
      installed.

* Step 4 - Validation with pass/fail criteria
    * Run the PacketIn and PacketOut validation procedure and verify that it
      passes, with the PacketIn count for every packet type on every port,
      including both LAG members, equal to the number of packets sent.
    * Verify that `/interfaces/interface/state/id` still reports `1`, `2`,
      `3`, `4` and `10` for DUT port-1, DUT port-2, DUT port-3, DUT port-4 and
      `lag1`.
    * Verify that all DUT ports and `lag1` report `oper-status` `UP` and that
      both LAG members report `synchronization` `IN_SYNC`.
    * Stop the data plane traffic and verify that IPv4 and IPv6 traffic to
      the gRIBI programmed prefixes is received on ATE port-4 with no packet
      loss after the switchover completed.

### P4RT-1.4.2 - PacketIn after reverse supervisor switchover

The DUT must behave the same way regardless of which controller card becomes
primary, so the switchover is repeated in the reverse direction.

* Use gNMI Subscribe `ON_CHANGE` to wait until
  `/components/component/state/switchover-ready` reports `true` for `SUP-A`.
* Start the same continuous IPv4 and IPv6 data plane traffic as in
  P4RT-1.4.1.
* Issue gNOI `System.SwitchControlProcessor` with `control_processor` set to
  `SUP-A`, and validate `redundant-role`, `last-switchover-time` and
  `last-switchover-reason/trigger` as described in P4RT-1.4.1 Step 2, with
  `SUP-A` as the new primary controller card.
* Repeat P4RT-1.4.1 Step 3 with election ID `{high: 0, low: 3}`.
* Repeat P4RT-1.4.1 Step 4 with the same pass/fail criteria.

### P4RT-1.4.3 - PacketIn on a port enabled immediately after switchover

An interface that comes up while the DUT is still completing a switchover must
also punt packets once the P4RT controller has re-programmed its entries.

* Set `/interfaces/interface/config/enabled` to `false` on DUT port-3 and
  verify that DUT port-3 reports `oper-status` `DOWN` and that `lag1` stays
  `UP` with DUT port-2 as its only active member.
* Use gNMI Subscribe `ON_CHANGE` to wait until
  `/components/component/state/switchover-ready` reports `true` for `SUP-B`.
* Issue gNOI `System.SwitchControlProcessor` with `control_processor` set to
  `SUP-B`, and validate `redundant-role`, `last-switchover-time` and
  `last-switchover-reason/trigger` as described in P4RT-1.4.1 Step 2.
* As soon as the DUT accepts gNMI `Set` requests on the new primary
  controller card, and before the P4RT client reconnects, set
  `/interfaces/interface/config/enabled` to `true` on DUT port-3.
* Repeat P4RT-1.4.1 Step 3 with election ID `{high: 0, low: 4}`.
* Verify that DUT port-3 reports `oper-status` `UP` and that both LAG members
  report `synchronization` `IN_SYNC`.
* Run the PacketIn and PacketOut validation procedure and verify that it
  passes. In particular, the PacketIn count for LLDP and GDP packets sent from
  ATE port-3 must equal the number of packets sent, with
  `ingress_singleton_port` set to `3`.

### Cleanup

* Delete the 4 punt entries with `P4Runtime.Write` (`DELETE`), verify with
  `P4Runtime.Read` that no entries remain, and close the P4RT client
  sessions.
* Remove the gRIBI entries with `gRIBI.Flush` for the default network
  instance and close the gRIBI client session.
* Restore the original DUT configuration for DUT port-1, DUT port-2, DUT
  port-3, DUT port-4, `lag1`, the P4RT interface IDs, the integrated circuit
  node IDs and `/lldp/config/enabled`, and verify that all DUT ports report
  `oper-status` `UP`.
* Verify that the standby controller card reports `switchover-ready` `true`,
  leaving the DUT with fully redundant controller cards.

## Canonical OC

```json
{
  "components": {
    "component": [
      {
        "config": {
          "name": "P4RT_NODE1"
        },
        "integrated-circuit": {
          "config": {
            "node-id": "1"
          }
        },
        "name": "P4RT_NODE1"
      },
      {
        "config": {
          "name": "P4RT_NODE2"
        },
        "integrated-circuit": {
          "config": {
            "node-id": "2"
          }
        },
        "name": "P4RT_NODE2"
      }
    ]
  },
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "DUT port-1 to ATE port-1",
          "enabled": true,
          "id": 1,
          "name": "port1",
          "type": "ethernetCsmacd"
        },
        "name": "port1",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "enabled": true,
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
                        "ip": "2001:db8::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8::1"
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
          "description": "DUT port-2 member of lag1",
          "enabled": true,
          "id": 2,
          "name": "port2",
          "type": "ethernetCsmacd"
        },
        "ethernet": {
          "config": {
            "aggregate-id": "lag1"
          }
        },
        "name": "port2"
      },
      {
        "config": {
          "description": "DUT port-3 member of lag1",
          "enabled": true,
          "id": 3,
          "name": "port3",
          "type": "ethernetCsmacd"
        },
        "ethernet": {
          "config": {
            "aggregate-id": "lag1"
          }
        },
        "name": "port3"
      },
      {
        "config": {
          "description": "DUT port-4 to ATE port-4",
          "enabled": true,
          "id": 4,
          "name": "port4",
          "type": "ethernetCsmacd"
        },
        "name": "port4",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "enabled": true,
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
                        "ip": "2001:db8::9",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8::9"
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
        "aggregation": {
          "config": {
            "lag-type": "LACP"
          }
        },
        "config": {
          "description": "LAG to ATE port-2 and ATE port-3",
          "enabled": true,
          "id": 10,
          "name": "lag1",
          "type": "ieee8023adLag"
        },
        "name": "lag1",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "enabled": true,
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
                        "ip": "2001:db8::5",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8::5"
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
  "lacp": {
    "interfaces": {
      "interface": [
        {
          "config": {
            "interval": "FAST",
            "lacp-mode": "ACTIVE",
            "name": "lag1"
          },
          "name": "lag1"
        }
      ]
    }
  },
  "lldp": {
    "config": {
      "enabled": false
    }
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  ## Config paths
  /components/component/integrated-circuit/config/node-id:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /interfaces/interface/config/id:
  /interfaces/interface/config/enabled:
  /interfaces/interface/ethernet/config/aggregate-id:
  /interfaces/interface/aggregation/config/lag-type:
  /lacp/interfaces/interface/config/lacp-mode:
  /lldp/config/enabled:

  ## State paths
  /interfaces/interface/state/id:
  /interfaces/interface/state/oper-status:
  /lacp/interfaces/interface/members/member/state/synchronization:
  /components/component/state/redundant-role:
    platform_type: ["CONTROLLER_CARD"]
  /components/component/state/switchover-ready:
    platform_type: ["CONTROLLER_CARD"]
  /components/component/state/last-switchover-time:
    platform_type: ["CONTROLLER_CARD"]
  /components/component/state/last-switchover-reason/trigger:
    platform_type: ["CONTROLLER_CARD"]
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
  gnoi:
    system.System.SwitchControlProcessor:
  gribi:
    gRIBI.Flush:
    gRIBI.Modify:
```

## Required DUT platform

* MFF
