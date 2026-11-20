# gNMI-1.30: gNMI Telemetry Performance under Dynamic Control Plane Churn

## Summary

Verify that gNMI state streaming (specifically interface counters and AFT
states) remains stable and within latency SLOs while the device is actively
processing high-rate routing/gRIBI updates.
This test ensures that the gNMI session does not disconnect or report sync loss
during control plane churn, 100% of telemetry updates are received within the
configured interval without starvation, and interface counters match the actual
traffic volume sent by the ATE.

## Testbed type

* [`TESTBED_DUT_ATE_4LINKS`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

* Configure IPv4 and IPv6 addresses on DUT and ATE on all 4 links:
  * Link 1 (`port1`): ATE 192.0.2.1/30 & 2001:db8:1::1/126, DUT 192.0.2.2/30 & 2001:db8:1::2/126
  * Link 2 (`port2`): ATE 192.0.2.5/30 & 2001:db8:2::1/126, DUT 192.0.2.6/30 & 2001:db8:2::2/126
  * Link 3 (`port3`): ATE 192.0.2.9/30 & 2001:db8:3::1/126, DUT 192.0.2.10/30 & 2001:db8:3::2/126
  * Link 4 (`port4`): ATE 192.0.2.13/30 & 2001:db8:4::1/126, DUT 192.0.2.14/30 & 2001:db8:4::2/126
* Configure BGP on the ATE to generate traffic destined to the routes that will be churned via gRIBI. DUT AS `65001`, ATE AS `65002`.
* Establish a gRIBI session between ATE (acting as gRIBI controller) and DUT using `SINGLE_PRIMARY` redundancy mode, and `PRESERVE` persistence.

### gNMI-1.30.1 - Baseline gNMI Telemetry under gRIBI Churn (Single Link)

* Step 1 - Generate DUT configuration
  * Configure interfaces on the DUT with IPv4 and IPv6 addresses for Link 1 (`port1`) and Link 2 (`port2`).
* Step 2 - Push configuration to DUT using gNMI Set with REPLACE option.
* Step 3 - Event-Driven Validation:
  * Wait for `/interfaces/interface[name=port1]/state/oper-status` and `/interfaces/interface[name=port2]/state/oper-status` to equal `UP` via gNMI Subscribe.
  * Wait for `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state` to equal `ESTABLISHED` via gNMI Subscribe.
* Step 4 - Establish gNMI subscriptions on the DUT for interface counters (`/interfaces/interface/state/counters`) and AFT states (`/network-instances/network-instance/afts`) with a 10-second update interval (`SAMPLE`).
* Step 5 - Inject a baseline batch of 100 IPv4 and 100 IPv6 routes via the gRIBI session on Link 2.
* Step 6 - Wait for the presence of specific prefixes in `/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix` to guarantee routes are fully programmed.
* Step 7 - Start ATE traffic from Link 1 (`port1`) to Link 2 (`port2`) matching the programmed gRIBI routes.
* Step 8 - Continuous gRIBI churn: Inject a continuous stream of gRIBI route modifications (add/delete/update IPv4 and IPv6 NextHopGroups and Entries) for the baseline routes from the ATE gRIBI controller.
  * *Guidance on gRIBI Churn:* To achieve "churn" without disrupting the traffic flow entirely, you can perform gRIBI `Modify` RPCs in a continuous loop:
    1. **NHG Updates:** Modify an existing `NextHopGroup` (NHG) by adding or removing a secondary `NextHop`, or by changing the weight of a `NextHop`.
    2. **Entry Swaps:** Change an `IPv4Entry` or `IPv6Entry` to point to a different, pre-programmed `NextHopGroup` that still routes valid traffic to the ATE.
    3. **Add/Delete Cycles:** Rapidly send `DELETE` and then `ADD` operations for a subset of the programmed prefixes.
* Step 9 - Validation with pass/fail criteria:
  * **Pass Criteria:**
    * Subscription Stability: The gNMI session does not disconnect or report sync loss during control plane churn.
    * On-time Delivery: Verify the `timestamp` in the gNMI `SubscribeResponse` to ensure telemetry updates arrive exactly at the 10-second interval (no starvation).
    * Data Accuracy: The delta of `in-unicast-pkts` and `out-unicast-pkts` reported by gNMI matches the ATE Tx/Rx rate within a 1% tolerance.
    * Lossless Validation: Total ATE Tx packets == ATE Rx packets (0% packet drop).

### gNMI-1.30.2 - Scaled gNMI Telemetry under High gRIBI Churn (4 Links)

* Step 1 - Generate DUT configuration
  * Configure interfaces on the DUT with IPv4 and IPv6 addresses for all 4 links (`port1` through `port4`).
* Step 2 - Push configuration to DUT using gNMI Set with REPLACE option.
* Step 3 - Event-Driven Validation:
  * Wait for `oper-status` to equal `UP` on all 4 interfaces via gNMI Subscribe.
  * Wait for BGP `session-state` to equal `ESTABLISHED` on all 4 links via gNMI Subscribe.
* Step 4 - Establish gNMI subscriptions on the DUT for interface counters and AFT states on all links with a 10-second update interval (`SAMPLE`).
* Step 5 - Scale Up Route Injection: Program 10,000+ IPv4 and 10,000+ IPv6 routes with ECMP next-hops spread across `port2`, `port3`, and `port4` via gRIBI.
* Step 6 - Wait for the presence of the scaled prefixes in `/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix` and `/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix` to guarantee full programming.
* Step 7 - Start ATE traffic from `port1` load-balanced across the scaled gRIBI routes to `port2`, `port3`, and `port4`.
* Step 8 - Scale Up gRIBI Churn: Continuously modify and delete routes at a high rate (e.g., 1,000 operations/sec) from the ATE gRIBI controller.
  * *Guidance on gRIBI Churn:* In this scaled scenario, leverage the ECMP paths across `port2`, `port3`, and `port4`. Send batches of gRIBI `Modify` requests that continuously shift the `NextHopGroup` assignments for thousands of prefixes, or rapidly add/remove `NextHop` members within the NHGs to force massive AFT updates across the DUT's forwarding pipeline.
* Step 9 - Validation with pass/fail criteria:
  * **Pass Criteria:**
    * Same as 1.30.1: No gNMI disconnects, strict 10-second update cadence per timestamp validation, counter deltas match ATE rates within 1%, and zero traffic loss during ECMP churn.

### gNMI-1.30.3 - Negative Testing: Telemetry Stability during gRIBI Session Disconnect

* Step 1 - While the scaled high-rate traffic and gRIBI churn from `gNMI-1.30.2` are actively running, simulate a sudden gRIBI controller crash or network disconnect from the ATE side.
* Step 2 - Observe the active gNMI telemetry subscriptions for interface counters.
* Step 3 - Validation with pass/fail criteria:
  * **Pass Criteria:**
    * Telemetry Isolation: The gNMI telemetry session remains completely unaffected by the gRIBI session drop.
    * No Sync Loss: The gNMI session does not drop, reconnect, or report sync loss.
    * Continuous Accuracy: Interface counters (`in-unicast-pkts`, `out-unicast-pkts`, `in-octets`, `out-octets`) continue to stream at the accurate configured interval without interruption.
    * Route Persistence: Since gRIBI was configured with `PRESERVE` persistence, AFT states and traffic forwarding should remain unaffected during the disconnect.

#### Canonical OC

```json
{
  "openconfig-interfaces:interfaces": {
    "interface": [
      {
        "name": "Ethernet1",
        "config": {
          "type": "iana-if-type:ethernetCsmacd",
          "description": "DUT to ATE link 1",
          "enabled": true
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /interfaces/interface/state/counters/in-unicast-pkts:
  /interfaces/interface/state/counters/out-unicast-pkts:
  /interfaces/interface/state/counters/in-octets:
  /interfaces/interface/state/counters/out-octets:
  /interfaces/interface/state/oper-status:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
      sample: true
```

## Required DUT platform

* FFF
