# TE-8.2: Supervisor Failure

## Summary

Ensure that gRIBI entries are persisted over supervisor failure.

## Testbed type

* https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

## Procedure

### Test environment setup:

* Connect DUT port-1 to ATE port-1, DUT port-2 to ATE port-2.
* Assign IPv4 addresses to all ports.

### TE-8.2.1 - FIB Programming and Switchover Validation

* Connect gRIBI client to DUT specifying persistence mode PRESERVE,
`SINGLE_PRIMARY` client redundancy in the SessionParameters request, and
make it become leader. Ensure that no error is reported from the gRIBI
server.

* Add 50 `IPv4Entry`s (starting at `203.0.113.1/32`) and 50 `IPv6Entry`s
(starting at `2001:db8:203:0:113::1/128`) pointing to ATE port-2 via
`gRIBI-A`. Ensure that the entries are active through AFT telemetry and correct ACK is received.

* Send traffic from ATE port-1 to the 100 prefixes (50 IPv4 and 50 IPv6), and ensure traffic
flows 100% and reaches ATE port-2.

* Validate: Supervisor switchover is triggered using gNOI `SwitchControlProcessor`.
Ensure gRIBI entries persist over switchover and traffic contniues to be forwarded.

* Following reconnection of a gRIBI client to the new master supervisor, ensure
the 100 prefixes pointing to ATE port-2 are present and traffic
flows 100% from ATE port-1 to ATE port-2.

### TE-8.2.2 - Post Switchover FIB Programming Validation

* Add another 50 `IPv4Entry`s (starting at `203.0.114.1/32`) and 50
`IPv6Entry`s (starting at `2001:db8:203:0:114::1/128`) pointing to ATE
port-2. Ensure that these new entries are active through AFT telemetry and correct ACKs are received.

* Send traffic to all 200 prefixes (100 initial + 100 post-switchover) and
ensure traffic flows 100% from ATE port-1 to ATE port-2.

### TE-8.2.3 - Post Switchover gRIBI Get RPC Validation

A gRIBI client acting as a routing controller uses the gRIBI `Get` RPC to
reconcile its view of the programmed forwarding state with the DUT. This is
typically done periodically and with a fixed RPC deadline. After a supervisor
switchover, the gRIBI session of the client that was connected before the
switchover is torn down, and a new client session is established with the new
primary controller card. This subtest validates that state associated with the
disconnected pre-switchover client session does not degrade the `Get` RPC. The
newly connected client must receive a complete and accurate view of the
programmed entries, without duplicate or stale entries, within the configured
deadline. The validation is performed after a switchover in each direction
between the two controller cards.

This subtest starts from the state left by TE-8.2.2: the 100 `IPv4Entry`s and
the 100 `IPv6Entry`s programmed in TE-8.2.1 and TE-8.2.2, all pointing to ATE
port-2, are present in the default network instance.

Test parameters:

* `get_deadline`: gRPC deadline applied to each gRIBI `Get` RPC. It is
configurable through a test flag and defaults to `10s`.

* Step 1 - Record baseline controller card and gRIBI state
    * Use gNMI Get on `/components/component/state/redundant-role` for all
      `CONTROLLER_CARD` components to identify the current primary controller
      card (referred to as `SUP-A`) and the standby controller card (referred
      to as `SUP-B`).
    * Use gNMI Subscribe `ON_CHANGE` to wait until
      `/components/component/state/switchover-ready` reports `true` for
      `SUP-B`, so that the switchover is not requested while the standby
      controller card is still synchronizing state.
    * Verify through AFT telemetry that all 200 prefixes are present in the
      default network instance, using
      `/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix`
      and
      `/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix`.
      These 100 IPv4 prefixes, 100 IPv6 prefixes, and the next-hop-group and
      next-hop entries that they reference are the expected gRIBI state for
      the validations below.
    * Record the highest election ID used by any gRIBI client so far in the
      test.

* Step 2 - Trigger a supervisor switchover from `SUP-A` to `SUP-B`
    * Issue gNOI `System.SwitchControlProcessor` with `control_processor` set
      to `SUP-B`. The gRIBI session of the currently connected client is
      expected to be terminated by the switchover, and that client must not
      send any further RPCs on that session.
    * Use gNMI Subscribe to wait until `SUP-B` reports
      `/components/component/state/redundant-role` as `PRIMARY` and `SUP-A`
      reports `SECONDARY`.
    * Verify that `/components/component/state/last-switchover-time` of
      `SUP-B` is later than the time at which the switchover was requested,
      and that `/components/component/state/last-switchover-reason/trigger` is
      `USER_INITIATED`.
    * Send IPv4 and IPv6 traffic from ATE port-1 to all 200 prefixes and
      ensure that 100% of the traffic is received on ATE port-2, confirming
      that the gRIBI entries persisted over the switchover.

* Step 3 - Connect a new gRIBI client to the new primary controller card
    * Connect a new gRIBI client to the DUT specifying persistence mode
      `PRESERVE`, `SINGLE_PRIMARY` client redundancy and `FIB_ACK`
      acknowledgement mode in the `SessionParameters` request.
    * Send a `ModifyRequest` with an election ID strictly greater than the
      election ID recorded in Step 1 so that the new client becomes leader.
    * Pass: the `SessionParameters` request is acknowledged without error, and
      the `ModifyResponse` reports the new election ID, confirming that the
      new client is the leader. Any error returned by the gRIBI server is a
      failure.

* Step 4 - Validate gRIBI Get RPC latency and contents
    * From the new client, issue a gRIBI `Get` RPC with `network_instance`
      set to all network instances and `aft` set to `ALL`, using a gRPC
      deadline equal to `get_deadline`.
    * Measure the `Get` latency as the time elapsed between sending the
      `GetRequest` and receiving the end of the `GetResponse` stream, and
      record it in the test log.
    * Pass/fail criteria:
        * The RPC completes with gRPC status `OK` before `get_deadline`
          expires. A `DEADLINE_EXCEEDED` status, or any other non-`OK`
          status, is a failure.
        * Across all `GetResponse` messages, the returned entries exactly
          match the expected state recorded in Step 1: 100 `IPv4Entry`s and
          100 `IPv6Entry`s with the expected prefixes, and the
          next-hop-group and next-hop entries that they reference.
        * Each prefix, next-hop-group and next-hop is returned exactly once
          per network instance. Any duplicate entry, or any entry that was not
          programmed by the test, such as stale state retained from the
          disconnected pre-switchover client session, is a failure.
        * Every returned entry reports `fib_status` as `PROGRAMMED`.
    * Repeat the `Get` RPC 2 more times at 30 second intervals, emulating
      periodic controller reconciliation, and apply the same pass/fail
      criteria to each response. This confirms that `Get` latency and
      contents remain stable after the switchover rather than only on the
      first request.

* Step 5 - Trigger a reverse supervisor switchover from `SUP-B` to `SUP-A`
    * Use gNMI Subscribe `ON_CHANGE` to wait until
      `/components/component/state/switchover-ready` reports `true` for
      `SUP-A`.
    * Issue gNOI `System.SwitchControlProcessor` with `control_processor` set
      to `SUP-A`, and validate `redundant-role`, `last-switchover-time` and
      `last-switchover-reason/trigger` as described in Step 2, with `SUP-A` as
      the new primary controller card.
    * Send IPv4 and IPv6 traffic from ATE port-1 to all 200 prefixes and
      ensure that 100% of the traffic is received on ATE port-2.
    * Repeat Step 3 with another new gRIBI client, using an election ID
      strictly greater than the election ID used in Step 3.
    * Repeat Step 4 with the same pass/fail criteria.

* Step 6 - Cleanup
    * From the leader gRIBI client, issue a gRIBI `Flush` RPC for the default
      network instance to remove all entries programmed by the test, and
      verify through AFT telemetry that the 200 prefixes are removed.
    * Close all gRIBI client sessions.
    * Verify that `SUP-A`, the primary controller card at the start of this
      subtest, is the primary controller card and that `SUP-B` reports
      `switchover-ready` as `true`, leaving the DUT with fully redundant
      controller cards.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

```yaml
paths:
    ## Config Parameter coverage

    ## Telemetry Parameter coverage

    /components/component/state/last-reboot-time:
     platform_type: ["CHASSIS", "CONTROLLER_CARD"]
    /components/component/state/last-reboot-reason:
     platform_type: ["CHASSIS", "CONTROLLER_CARD" ]
    /components/component/state/redundant-role:
     platform_type: [ "CONTROLLER_CARD" ]
    /components/component/state/last-switchover-time:
     platform_type: [ "CONTROLLER_CARD" ]
    /components/component/state/last-switchover-reason/trigger:
     platform_type: [ "CONTROLLER_CARD" ]
    /components/component/state/last-switchover-reason/details:
     platform_type: [ "CONTROLLER_CARD" ]
    /components/component/state/switchover-ready:
     platform_type: [ "CONTROLLER_CARD" ]
    /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
    /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:

rpcs:
    gnmi:
        gNMI.Set:
        gNMI.Get:
        gNMI.Subscribe:
    gnoi:
        system.System.SwitchControlProcessor:
    gribi:
        gRIBI.Flush:
        gRIBI.Get:
        gRIBI.Modify:
```

## Minimum DUT Required

* MFF


## Canonical OC
```json
{}
```   
