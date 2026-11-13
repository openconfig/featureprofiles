# P4RT-3.3: P4RT Packet-Out Interaction with gRIBI

## Summary

This test verifies that P4Runtime Packet-Out operations (the controller
injecting packets onto the data plane via the switch) function correctly and
are not disrupted by concurrent gRIBI route modifications. The test ensures that
packets injected via P4RT Packet-Out are successfully transmitted out of the
specified port with correct headers, and simultaneous high-rate gRIBI
programming (Add/Delete/Replace) does not cause packet drops or corrupt headers
for Packet-Out traffic, while gRIBI routing table updates continue to succeed
under the Packet-Out load.

## Testbed type

* [`TESTBED_DUT_ATE_4_LINKS`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

* Configure four interfaces on the DUT connected to ATE port 1 through ATE port 4.
  * ATE port 1 (`port1`) IPv4: `192.0.2.2/30`, IPv6: `2001:db8:1::2/126`, MAC: `02:00:00:00:00:01`
  * DUT port 1 (`Ethernet1/1`) IPv4: `192.0.2.1/30`, IPv6: `2001:db8:1::1/126`, MAC: `00:1A:11:00:00:01`
  * ATE port 2 (`port2`) IPv4: `192.0.2.6/30`, IPv6: `2001:db8:2::2/126`, MAC: `02:00:00:00:00:02`
  * DUT port 2 (`Ethernet1/2`) IPv4: `192.0.2.5/30`, IPv6: `2001:db8:2::1/126`, MAC: `00:1A:11:00:00:02`
  * ATE port 3 (`port3`) IPv4: `192.0.2.10/30`, IPv6: `2001:db8:3::2/126`, MAC: `02:00:00:00:00:03`
  * DUT port 3 (`Ethernet1/3`) IPv4: `192.0.2.9/30`, IPv6: `2001:db8:3::1/126`, MAC: `00:1A:11:00:00:03`
  * ATE port 4 (`port4`) IPv4: `192.0.2.14/30`, IPv6: `2001:db8:4::2/126`, MAC: `02:00:00:00:00:04`
  * DUT port 4 (`Ethernet1/4`) IPv4: `192.0.2.13/30`, IPv6: `2001:db8:4::1/126`, MAC: `00:1A:11:00:00:04`
* Bring up the interfaces and verify they are `UP` using telemetry
  `/interfaces/interface/state/oper-status`.
* Establish gRIBI and P4RT client connections to the DUT.

### P4RT-3.3.1 - Baseline gRIBI Forwarding and P4RT Packet-Out

* Step 1 - Program baseline gRIBI routes
  * Program a gRIBI IPv4 route in network instance `DEFAULT` for destination
    prefix `198.51.100.0/24`.
  * Program a gRIBI IPv6 route in network instance `DEFAULT` for destination
    prefix `2001:db8:100::/64`.
  * Create Next-Hop (index `10`) pointing to IP `192.0.2.6` and
    Next-Hop (index `11`) pointing to IP `2001:db8:2::2` (ATE port 2).
  * Create Next-Hop Group (index `100`) containing Next-Hop `10`, and
    Next-Hop Group (index `101`) containing Next-Hop `11`.
  * Validate route installation using `gNMI.Subscribe` (ON_CHANGE) or
    `gNMI.Get` on paths:
    * `/network-instances/network-instance[name=DEFAULT]/afts/ipv4-unicast/ipv4-entry[prefix=198.51.100.0/24]/state/prefix`
    * `/network-instances/network-instance[name=DEFAULT]/afts/ipv6-unicast/ipv6-entry[prefix=2001:db8:100::/64]/state/prefix`

* Step 2 - Send baseline P4RT Packet-Out
  * Inject 1500-byte IPv4 packets via P4RT Packet-Out directed to egress out of DUT port 2 (`Ethernet1/2`).
  * Inject 1500-byte IPv6 packets via P4RT Packet-Out directed to egress out of DUT port 2 (`Ethernet1/2`).
  * Receive packets on ATE port 2 and verify headers and payload match the
    injected packets exactly (no TTL decrement or MAC rewrite for Packet-Out).
  * Validate DUT egress transmission using gNMI path `/interfaces/interface[name=Ethernet1/2]/state/counters/out-pkts`.

* Step 3 - Send baseline data plane traffic
  * Send IPv4 traffic from ATE port 1 to destination IP `198.51.100.1`.
  * Send IPv6 traffic from ATE port 1 to destination IP `2001:db8:100::1`.
  * Verify 0% packet loss for both streams using OTG flow tracking to confirm baseline gRIBI forwarding is active.
  * Verify TTL is decremented by 1 and Source/Destination MAC addresses are rewritten correctly by the router.

### P4RT-3.3.2 - Concurrent gRIBI and P4RT Packet-Out operations

*Note: This subtest must build sequentially upon the state, configuration, and sessions established in P4RT-3.3.1. Do not tear down sessions or reset the device between subtests.*

* Step 1 - Start concurrent operations
  * Start continuous injection of 1500-byte IPv4 and IPv6 packets via P4RT
    Packet-Out directed to egress out of DUT port 2 (`Ethernet1/2`) at a constant rate (e.g., `1000 packets per second`).
  * Simultaneously start gRIBI programming (Add, Delete, Replace) for
    `100,000` different IPv4 (`198.51.x.y/32`) and `100,000` IPv6 (`2001:db8:x::y/128`)
    routes. Point these routes to ATE port 3 and ATE port 4 via new Next-Hop Groups.

* Step 2 - Validation with pass/fail criteria
  * Rely on `gNMI.Subscribe` (ON_CHANGE) or polling on AFT state paths to verify route installation without using arbitrary wait times. Ensure the exact count of `100,000` routes is reached.
  * Verify P4RT Packet-Out packets are successfully received by ATE port 2 using OTG flow tracking, with
    0% drop, no corrupted headers, and no packets erroneously leaking to other ATE ports.
  * Verify gRIBI routing table updates continue to succeed under P4RT
    Packet-Out load by verifying route installation via telemetry on AFT state.
  * Send test data plane traffic to the dynamically programmed `100,000` routes and
    verify successful forwarding (0% loss) out of ATE port 3 and 4 with correct TTL decrement and MAC rewrite.

### P4RT-3.3.3 - Negative Scenarios under Concurrent Load

* Step 1 - Invalid Egress Port ID
  * Send P4RT Packet-Outs directed to an invalid or non-existent egress port ID while concurrent gRIBI route programming is active.
  * *Validation*: Verify that the P4RT controller receives an appropriate error and the concurrent gRIBI operations continue uninterrupted.

* Step 2 - Port State Down
  * Administratively disable the egress port (`/interfaces/interface[name=Ethernet1/2]/config/enabled` = `false`) via gNMI while concurrent Packet-Out and gRIBI programming is occurring.
  * *Validation*: Verify that Packet-Out traffic to that port drops deterministically, but gRIBI route programming for other healthy ports completes successfully.

#### Canonical OC

```json
{
  "network-instances": {
    "network-instance": [
      {
        "name": "DEFAULT",
        "config": {
          "name": "DEFAULT"
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/config/enabled:
  /interfaces/interface/state/counters/out-pkts:
  /interfaces/interface/state/counters/in-pkts:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/config/name:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

* FFF
