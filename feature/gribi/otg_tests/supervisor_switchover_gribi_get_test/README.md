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
the programmed entries, without duplicate or stale entries, within a
configurable deadline.

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

* Configure the DUT interfaces as follows:
    * DUT port-1: IPv4 `192.0.2.1/30` and IPv6 `2001:db8::1/126`. ATE port-1
      uses `192.0.2.2/30` and `2001:db8::2/126`.
    * DUT port-2: IPv4 `192.0.2.5/30` and IPv6 `2001:db8::5/126`. ATE port-2
      uses `192.0.2.6/30` and `2001:db8::6/126`.
* Verify that DUT port-1 and DUT port-2 report
  `/interfaces/interface/state/oper-status` `UP`.
* Use gNMI Get on `/components/component/state/redundant-role` for all
  `CONTROLLER_CARD` components to identify the primary controller card
  (referred to as `SUP-A`) and the standby controller card (referred to as
  `SUP-B`).

Test parameters:

* `get_deadline`: gRPC deadline applied to each gRIBI `Get` RPC. It is
  configurable through a test flag and defaults to `10s`.

### TE-8.3.1 - Baseline gRIBI programming and Get validation

This subtest programs the gRIBI entries and records the expected state that
the subsequent subtests validate against.

* Step 1 - Program gRIBI entries
    * Connect a gRIBI client (referred to as `gRIBI-A`) to the DUT specifying
      persistence mode `PRESERVE`, `SINGLE_PRIMARY` client redundancy and
      `FIB_ACK` acknowledgement mode in the `SessionParameters` request.
      Send a `ModifyRequest` with election ID `{high: 0, low: 1}` and verify
      that `gRIBI-A` becomes leader.
    * In the default network instance, program using `gRIBI.Modify`:
        * Next hop `NH#1` with IP address `192.0.2.6` (ATE port-2) and
          next-hop-group `NHG#1` containing `NH#1`.
        * Next hop `NH#2` with IP address `2001:db8::6` (ATE port-2) and
          next-hop-group `NHG#2` containing `NH#2`.
        * 100 `IPv4Entry`s from `198.51.100.1/32` to `198.51.100.100/32`,
          each pointing to `NHG#1`.
        * 100 `IPv6Entry`s from `2001:db8:100::1/128` to
          `2001:db8:100::64/128`, each pointing to `NHG#2`.
    * Verify that every operation is acknowledged with `FIB_PROGRAMMED`.

* Step 2 - Verify forwarding state
    * Verify through AFT telemetry that all 100 IPv4 prefixes and 100 IPv6
      prefixes are present in the default network instance, using
      `/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix`
      and
      `/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix`.
    * Send IPv4 traffic from ATE port-1 to the 100 IPv4 prefixes and IPv6
      traffic from ATE port-1 to the 100 IPv6 prefixes, and verify that 100%
      of the traffic is received on ATE port-2.

* Step 3 - Baseline Get validation
    * From `gRIBI-A`, issue a gRIBI `Get` RPC with `network_instance` set to
      all network instances and `aft` set to `ALL`, using a gRPC deadline
      equal to `get_deadline`. Measure the `Get` latency as the time elapsed
      between sending the `GetRequest` and receiving the end of the
      `GetResponse` stream, and record it in the test log.
    * Pass/fail criteria (referred to as the **Get validation criteria**):
        * The RPC completes with gRPC status `OK` before `get_deadline`
          expires. A `DEADLINE_EXCEEDED` status, or any other non-`OK`
          status, is a failure.
        * Across all `GetResponse` messages, the returned entries are exactly
          the 100 `IPv4Entry`s, 100 `IPv6Entry`s, `NHG#1`, `NHG#2`, `NH#1`
          and `NH#2` programmed in Step 1.
        * Each prefix, next-hop-group and next-hop is returned exactly once
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
    * Use gNMI Subscribe to wait until `SUP-B` reports
      `/components/component/state/redundant-role` as `PRIMARY` and `SUP-A`
      reports `SECONDARY`.
    * Verify that `/components/component/state/last-switchover-time` of
      `SUP-B` is later than the recorded time, and that
      `/components/component/state/last-switchover-reason/trigger` is
      `USER_INITIATED`.

* Step 2 - Verify forwarding persisted
    * Verify through AFT telemetry that all 100 IPv4 prefixes and 100 IPv6
      prefixes are still present in the default network instance.
    * Send IPv4 and IPv6 traffic from ATE port-1 to all 200 prefixes and
      verify that 100% of the traffic is received on ATE port-2.

* Step 3 - Connect a new gRIBI client to the new primary controller card
    * Connect a new gRIBI client (referred to as `gRIBI-B`) to the DUT
      specifying persistence mode `PRESERVE`, `SINGLE_PRIMARY` client
      redundancy and `FIB_ACK` acknowledgement mode.
    * Send a `ModifyRequest` with election ID `{high: 0, low: 2}`, which is
      higher than the election ID used by `gRIBI-A`, and verify that the
      `ModifyResponse` reports the new election ID, confirming that
      `gRIBI-B` is the leader. Any error returned by the gRIBI server is a
      failure.

* Step 4 - Validate Get latency and contents
    * From `gRIBI-B`, issue the same gRIBI `Get` RPC as in TE-8.3.1 Step 3
      and record its latency. The response must meet the **Get validation
      criteria**. In particular, no entry may be duplicated or retained from
      the disconnected `gRIBI-A` session.
    * Repeat the `Get` RPC 2 more times at 30 second intervals, emulating
      periodic controller reconciliation, and apply the same criteria to
      each response. This confirms that `Get` latency and contents remain
      stable after the switchover rather than only on the first request.

### TE-8.3.3 - Get validation after reverse supervisor switchover

The DUT must behave the same way regardless of which controller card becomes
primary, so the switchover is repeated in the reverse direction.

* Use gNMI Subscribe `ON_CHANGE` to wait until
  `/components/component/state/switchover-ready` reports `true` for `SUP-A`.
* Issue gNOI `System.SwitchControlProcessor` with `control_processor` set to
  `SUP-A`, and validate `redundant-role`, `last-switchover-time` and
  `last-switchover-reason/trigger` as described in TE-8.3.2 Step 1, with
  `SUP-A` as the new primary controller card.
* Repeat TE-8.3.2 Step 2.
* Repeat TE-8.3.2 Step 3 with a new gRIBI client (referred to as `gRIBI-C`)
  using election ID `{high: 0, low: 3}`.
* Repeat TE-8.3.2 Step 4 from `gRIBI-C` with the same pass/fail criteria.

### Cleanup

* From the leader gRIBI client, issue a gRIBI `Flush` RPC for the default
  network instance to remove all entries programmed by the test, and verify
  through AFT telemetry that the 200 prefixes are removed.
* Close all gRIBI client sessions.
* Restore the original configuration of DUT port-1 and DUT port-2.
* Verify that `SUP-A`, the primary controller card at the start of the test,
  is the primary controller card and that `SUP-B` reports `switchover-ready`
  as `true`, leaving the DUT with fully redundant controller cards.

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
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
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
    gRIBI.Get:
    gRIBI.Modify:
```

## Required DUT platform

* MFF
