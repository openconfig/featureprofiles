# TE-8.3: gRIBI Get RPC Validation after Supervisor Switchover

## Summary

Validate that the gRIBI `Get` RPC remains responsive and accurate after a
supervisor (controller card) switchover on a modular DUT.

A gRIBI client acting as a routing controller uses the gRIBI `Get` RPC to
reconcile its view of the programmed forwarding state with the DUT. This is
typically done periodically and with a fixed RPC deadline. After a supervisor
switchover, the gRIBI session of the client that was connected before the
switchover is torn down, and a new client session is established with the new
primary controller card. If state associated with the disconnected client
session slows down or pollutes the `Get` response, the controller can no longer
reconcile its state even though forwarding is healthy.

This test verifies that, after a switchover triggered with gNOI
`System.SwitchControlProcessor` in each direction between the two controller
cards, a newly connected gRIBI client receives a complete and accurate view of
the programmed entries across network instances, without duplicate or stale
entries, within a configurable deadline. It also verifies that a client using
a pre-switchover election ID cannot change the forwarding state, and that a
switchover requested while the standby controller card is not ready does not
disturb gRIBI state.

## Testbed type

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

### Test environment setup

The DUT and ATE are connected with 2 links. DUT port-1 receives traffic from
the ATE, and DUT port-2 is the egress port for all gRIBI programmed entries.

```text
ATE port-1 <------> DUT port-1      (traffic ingress)
ATE port-2 <------> DUT port-2      (gRIBI next hop / traffic egress)
```

* Configure the DUT as follows:
    * DUT port-1: IPv4 `192.0.2.1/30` and IPv6 `2001:db8::1/126` on
      subinterface `0`.
    * DUT port-2: IPv4 `192.0.2.5/30` and IPv6 `2001:db8::5/126` on
      subinterface `0`.
    * DUT port-1 and DUT port-2 (subinterface `0`) are in the default network
      instance.
    * A non-default network instance `VRF-1`, with
      `/network-instances/network-instance/config/type` set to `L3VRF`. No
      interfaces are assigned to `VRF-1`. Its gRIBI entries resolve through
      next-hop-groups in the default network instance and are validated
      through gRIBI `Get` and AFT telemetry only; no traffic is sent to them.
      `VRF-1` makes sure that a `Get` for all network instances aggregates
      entries from more than one network instance.
* Configure the ATE as follows:
    * ATE port-1: MAC `02:00:01:01:01:01`, IPv4 `192.0.2.2/30` and IPv6
      `2001:db8::2/126`, with DUT port-1 as the gateway.
    * ATE port-2: MAC `02:00:02:01:01:01`, IPv4 `192.0.2.6/30` and IPv6
      `2001:db8::6/126`, answering ARP and ND for these addresses.
    * Disable any other protocol emulation on the ATE ports.
* Use gNMI `Await` to wait until DUT port-1 and DUT port-2 report
  `/interfaces/interface/state/oper-status` `UP`.
* Use gNMI Get on `/components/component/state/redundant-role` for all
  `CONTROLLER_CARD` components. Verify that exactly one controller card reports
  `PRIMARY` (referred to as `SUP-A`) and exactly one reports `SECONDARY`
  (referred to as `SUP-B`).

Test parameters (all configurable through test flags):

* `get_deadline`: gRPC deadline applied to each gRIBI `Get` RPC. Default
  `10s`.
* `get_repeat_interval`: interval between repeated `Get` RPCs after a
  switchover, emulating a controller's periodic reconciliation. Default `30s`.
  This is an intentional test interval, not a wait for state to settle.
* `switchover_timeout`: maximum time to re-establish the gNMI connection and
  observe the new controller card roles after a switchover. Default
  `20 minutes`.
* `gribi_reconnect_timeout`: maximum time to establish a gRIBI session with the
  new primary controller card after a switchover. Default `10 minutes`.

Traffic definition (referred to as the **traffic validation**):

* `Flow-IPv4`: from ATE port-1, source `192.0.2.2`, destination incrementing
  from `198.51.100.1` to `198.51.100.100` (100 values), 512-byte frames,
  1000 packets per second, fixed count of 10,000 packets.
* `Flow-IPv6`: from ATE port-1, source `2001:db8::2`, destination incrementing
  from `2001:db8:100::1` to `2001:db8:100::64` (the last address is
  hexadecimal `0x64`, so the range contains 100 values), 512-byte frames,
  1000 packets per second, fixed count of 10,000 packets.
* After starting protocols and before starting traffic, wait for ARP
  resolution (IPv4) and ND resolution (IPv6) on both ATE ports.
* Pass criteria: for each flow, the number of packets received on ATE port-2
  equals the number of packets transmitted (0% loss), and no packets of the
  flow are received on ATE port-1.

### TE-8.3.1 - Baseline gRIBI programming and Get validation

This subtest programs the gRIBI entries and records the expected state that
the subsequent subtests validate against. TE-8.3.2 and TE-8.3.3 do not
re-program any entry, so any duplicate or stale entry observed later can only
originate from the DUT.

* Step 1 - Verify that no route exists before programming
    * Send `Flow-IPv4` and `Flow-IPv6` and verify that 0 packets are received
      on ATE port-2. This proves that later delivery comes from the gRIBI
      entries and not from a pre-existing default or static route.

* Step 2 - Program gRIBI entries
    * Connect a gRIBI client (referred to as `gRIBI-A`) to the DUT specifying
      persistence mode `PRESERVE`, `SINGLE_PRIMARY` client redundancy and
      `FIB_ACK` acknowledgement mode in the `SessionParameters` request.
      Send a `ModifyRequest` with election ID `{high: 0, low: 1}` and verify
      that `gRIBI-A` becomes leader.
    * In the default network instance, program using `gRIBI.Modify`:
        * Next hop index `1` with IP address `192.0.2.6` (ATE port-2) and
          next-hop-group ID `1` containing next hop `1`.
        * Next hop index `2` with IP address `2001:db8::6` (ATE port-2) and
          next-hop-group ID `2` containing next hop `2`.
        * 100 `IPv4Entry`s from `198.51.100.1/32` to `198.51.100.100/32`,
          each pointing to next-hop-group `1`.
        * 100 `IPv6Entry`s from `2001:db8:100::1/128` to
          `2001:db8:100::64/128` (the last address is hexadecimal `0x64`, so
          the range contains 100 entries), each pointing to next-hop-group
          `2`.
    * In network instance `VRF-1`, program 50 `IPv4Entry`s from
      `203.0.113.1/32` to `203.0.113.50/32`, each pointing to next-hop-group
      `1` in the default network instance.
    * Verify that every operation is acknowledged with `FIB_PROGRAMMED`.

* Step 3 - Verify forwarding state
    * Use gNMI `Await` to verify through AFT telemetry that:
        * all 100 IPv4 prefixes and 100 IPv6 prefixes are present in the
          default network instance, using
          `/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix`
          and
          `/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix`;
        * all 50 IPv4 prefixes are present in `VRF-1`;
        * next-hop-groups `1` and `2` are present, using
          `/network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id`;
        * the next hops `192.0.2.6` and `2001:db8::6` are present, using
          `/network-instances/network-instance/afts/next-hops/next-hop/state/ip-address`;
        * each IPv4 entry references a next-hop-group, using
          `/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group`;
        * each IPv6 entry references a next-hop-group, using
          `/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group`.
    * Run the **traffic validation** and verify that it passes.

* Step 4 - Baseline Get validation
    * From `gRIBI-A`, issue a gRIBI `Get` RPC with `network_instance` set to
      all network instances and `aft` set to `ALL`, using a gRPC deadline
      equal to `get_deadline`. Measure the `Get` latency as the time elapsed
      between sending the `GetRequest` and receiving the end of the
      `GetResponse` stream, and record it in the test log. The latency is a
      recorded metric only; the pass/fail criterion is `get_deadline`.
    * Pass/fail criteria (referred to as the **Get validation criteria**):
        * The RPC completes with gRPC status `OK` before `get_deadline`
          expires. A `DEADLINE_EXCEEDED` status, or any other non-`OK`
          status, is a failure.
        * Across all `GetResponse` messages, the returned entries are exactly:
          in the default network instance, the 100 `IPv4Entry`s, the 100
          `IPv6Entry`s, next-hop-groups `1` and `2`, and next hops `1` and
          `2`; in `VRF-1`, the 50 `IPv4Entry`s.
        * Each prefix, next-hop-group and next hop is returned exactly once
          per network instance. Any duplicate entry, or any entry that was
          not programmed by the test, is a failure.
        * Every returned entry reports `fib_status` as `PROGRAMMED`.

### TE-8.3.2 - Get validation after supervisor switchover

* Step 1 - Trigger a supervisor switchover from `SUP-A` to `SUP-B`
    * Use gNMI Subscribe `ON_CHANGE` to wait until
      `/components/component/state/switchover-ready` reports `true` for
      `SUP-B`, so that the switchover is not requested while the standby
      controller card is still synchronizing state.
    * Record the current time and issue gNOI `System.SwitchControlProcessor`
      with `control_processor` set to `SUP-B`. The gRIBI session of
      `gRIBI-A` is expected to be terminated by the switchover, and
      `gRIBI-A` must not send any further RPCs on that session.
    * The gNMI connection is expected to drop during the switchover. Retry
      the gNMI connection until it succeeds or `switchover_timeout` expires,
      then use gNMI Subscribe `ON_CHANGE` to wait until `SUP-B` reports
      `/components/component/state/redundant-role` as `PRIMARY` and `SUP-A`
      reports `SECONDARY`.
    * Verify that `/components/component/state/last-switchover-time` of
      `SUP-B` is later than the recorded time, and that
      `/components/component/state/last-switchover-reason/trigger` is
      `USER_INITIATED`.

* Step 2 - Verify forwarding persisted
    * Use gNMI `Await` to wait until DUT port-1 and DUT port-2 report
      `oper-status` `UP`.
    * Use gNMI `Await` to verify through AFT telemetry that all 200 prefixes
      in the default network instance and all 50 prefixes in `VRF-1` are
      still present.
    * Run the **traffic validation** and verify that it passes.

* Step 3 - Connect a new gRIBI client to the new primary controller card
    * Connect a new gRIBI client (referred to as `gRIBI-B`) to the DUT
      specifying persistence mode `PRESERVE`, `SINGLE_PRIMARY` client
      redundancy and `FIB_ACK` acknowledgement mode. Retry the connection
      until it succeeds or `gribi_reconnect_timeout` expires.
    * Send a `ModifyRequest` with election ID `{high: 0, low: 2}`, which is
      higher than the election ID used by `gRIBI-A`, and verify that the
      `ModifyResponse` reports the new election ID, confirming that
      `gRIBI-B` is the leader. Any error returned by the gRIBI server is a
      failure.

* Step 4 - Validate Get latency and contents
    * From `gRIBI-B`, issue the same gRIBI `Get` RPC as in TE-8.3.1 Step 4
      and record its latency. The response must meet the **Get validation
      criteria**. In particular, no entry may be duplicated or retained from
      the disconnected `gRIBI-A` session.
    * Repeat the `Get` RPC 2 more times, `get_repeat_interval` apart, and
      apply the same criteria to each response. This confirms that `Get`
      latency and contents remain stable after the switchover rather than
      only on the first request.

### TE-8.3.3 - Get validation after reverse supervisor switchover

The DUT must behave the same way regardless of which controller card becomes
primary, so the switchover is repeated in the reverse direction. This subtest
starts from the state left by TE-8.3.2.

* Use gNMI Subscribe `ON_CHANGE` to wait until
  `/components/component/state/switchover-ready` reports `true` for `SUP-A`.
* Issue gNOI `System.SwitchControlProcessor` with `control_processor` set to
  `SUP-A`, and validate the gNMI reconnection, `redundant-role`,
  `last-switchover-time` and `last-switchover-reason/trigger` as described in
  TE-8.3.2 Step 1, with `SUP-A` as the new primary controller card.
* Repeat TE-8.3.2 Step 2.
* Repeat TE-8.3.2 Step 3 with a new gRIBI client (referred to as `gRIBI-C`)
  using election ID `{high: 0, low: 3}`.
* Repeat TE-8.3.2 Step 4 from `gRIBI-C` with the same pass/fail criteria.

### TE-8.3.4 - Stale client is rejected after switchover

A client that reconnects with the election ID used before the switchover must
not be able to change the forwarding state. This subtest starts from the state
left by TE-8.3.3, with `gRIBI-C` as leader.

* Connect a gRIBI client (referred to as `gRIBI-X`) specifying persistence
  mode `PRESERVE`, `SINGLE_PRIMARY` client redundancy and `FIB_ACK`
  acknowledgement mode, using election ID `{high: 0, low: 1}` (the election ID
  used by `gRIBI-A` before the first switchover).
* From `gRIBI-X`, send a `ModifyRequest` that adds `IPv4Entry`
  `198.51.100.200/32` pointing to next-hop-group `1` in the default network
  instance.
* Pass/fail criteria:
    * The operation is rejected because `gRIBI-X` is not the primary client
      (gRPC status `FAILED_PRECONDITION` with reason `NOT_PRIMARY`). Any
      `FIB_PROGRAMMED` or `RIB_PROGRAMMED` acknowledgement is a failure.
    * AFT telemetry does not contain `198.51.100.200/32`.
    * A `Get` from `gRIBI-C` meets the **Get validation criteria** and does
      not contain `198.51.100.200/32`.
* Close the `gRIBI-X` session.

### TE-8.3.5 - Switchover is not performed when the standby is not ready

A switchover requested while the standby controller card is not ready must be
rejected or handled without a role change, and must not affect gRIBI state.
This subtest starts from the state left by TE-8.3.4, with `SUP-A` as primary.

* Use gNMI Subscribe `ON_CHANGE` to wait until `switchover-ready` reports
  `true` for `SUP-B`, then issue gNOI `System.SwitchControlProcessor` with
  `control_processor` set to `SUP-B`, and validate it as described in
  TE-8.3.2 Step 1.
* Immediately after the gNMI connection is re-established, read
  `/components/component/state/switchover-ready` for `SUP-A` (the new
  standby). If it reports `true`, the standby became ready before the request
  could be sent; log this and skip the remaining negative check to avoid a
  timing-dependent result. If it reports `false`, issue gNOI
  `System.SwitchControlProcessor` with `control_processor` set to `SUP-A`.
* Pass/fail criteria:
    * The request returns an error, or it returns without a role change:
      subscribing to `/components/component/state/redundant-role` for
      2 minutes shows `SUP-B` remaining `PRIMARY` throughout.
    * Repeat TE-8.3.2 Step 2, Step 3 with a new gRIBI client (referred to as
      `gRIBI-D`) using election ID `{high: 0, low: 4}`, and Step 4 from
      `gRIBI-D`, with the same pass/fail criteria.

### Cleanup

* From the leader gRIBI client (`gRIBI-D`), issue a gRIBI `Flush` RPC for the
  default network instance and for `VRF-1` to remove all entries programmed by
  the test, and use gNMI `Await` to verify through AFT telemetry that the
  prefixes are removed from both network instances.
* Close all gRIBI client sessions.
* Delete network instance `VRF-1`, and delete the IPv4 and IPv6 addresses on
  subinterface `0` of DUT port-1 and DUT port-2.
* Use gNMI Subscribe `ON_CHANGE` to wait until the standby controller card
  reports `switchover-ready` as `true`, leaving the DUT with fully redundant
  controller cards. Either controller card may be primary at the end of the
  test.

## Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "DUT port-1 to ATE port-1",
          "enabled": true,
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
          "description": "DUT port-2 to ATE port-2",
          "enabled": true,
          "name": "port2",
          "type": "ethernetCsmacd"
        },
        "name": "port2",
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
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "VRF-1",
          "type": "L3VRF"
        },
        "name": "VRF-1"
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  ## Config paths
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/name:
  /interfaces/interface/config/type:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /network-instances/network-instance/config/type:

  ## State paths
  /interfaces/interface/state/oper-status:
  /components/component/state/redundant-role:
    platform_type: ["CONTROLLER_CARD"]
  /components/component/state/switchover-ready:
    platform_type: ["CONTROLLER_CARD"]
  /components/component/state/last-switchover-time:
    platform_type: ["CONTROLLER_CARD"]
  /components/component/state/last-switchover-reason/trigger:
    platform_type: ["CONTROLLER_CARD"]
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
  gnoi:
    system.System.SwitchControlProcessor:
  gribi:
    gRIBI.Flush:
    gRIBI.Get:
    gRIBI.Modify:
```

## Required DUT platform

* MFF
