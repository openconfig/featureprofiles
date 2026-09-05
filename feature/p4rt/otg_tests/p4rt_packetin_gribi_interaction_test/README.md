# P4RT-3.4: P4RT Packet-In Interaction with gRIBI

## Summary

This test verifies that P4Runtime Packet-In rules (e.g., for LLDP, Traceroute)
function correctly and are not disrupted by concurrent gRIBI AFT programming
operations. The test ensures that packets matching the P4RT Packet-In rules
are successfully punted to the controller, data plane traffic routed by gRIBI
entries is not affected by the Packet-In process, and gNMI telemetry for
interface counters and AFTs remains accurate.

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
* Bring up the interfaces and verify they are `UP` using telemetry `/interfaces/interface/state/oper-status`.
* Establish gRIBI and P4RT client connections to the DUT.

### P4RT-3.4.1 - Baseline gRIBI Forwarding and P4RT Packet-In

* Step 1 - Program baseline gRIBI routes and P4RT rules
  * Program a gRIBI IPv4 route in network instance `DEFAULT` for destination prefix `198.51.100.0/24`.
  * Program a gRIBI IPv6 route in network instance `DEFAULT` for destination prefix `2001:db8:100::/64`.
  * Create Next-Hop (index `10`) pointing to IP `192.0.2.6` (ATE port 2) and Next-Hop (index `11`) pointing to IP `2001:db8:2::2` (ATE port 2).
  * Create Next-Hop Group (index `100`) containing Next-Hop `10`, and Next-Hop Group (index `101`) containing Next-Hop `11`.
  * Validate route installation using `gNMI.Subscribe` (ON_CHANGE) or `gNMI.Get` on paths:
    * `/network-instances/network-instance[name=DEFAULT]/afts/ipv4-unicast/ipv4-entry[prefix=198.51.100.0/24]/state/prefix`
    * `/network-instances/network-instance[name=DEFAULT]/afts/ipv6-unicast/ipv6-entry[prefix=2001:db8:100::/64]/state/prefix`
  * Program P4RT table entries with Action `PuntToController` to trap the following:
    * LLDP packets (Match: `ether_type == 0x88CC`).
    * IPv4 Traceroute packets (Match: `ipv4_ttl == 1`).
    * BGP packets (Match: `ipv4_dst == 192.0.2.1`, `tcp_dst == 179`).
    * OSPF packets (Match: `ipv4_proto == 89`).

* Step 2 - Send baseline P4RT Packet-In
  * Inject 50,000 LLDP packets (1514 bytes) from ATE port 1 at a constant rate (1000 packets per second).
  * Verify exactly 50,000 LLDP packets are punted to the P4RT controller as `PacketIn` messages with 0% drop and no corrupted headers.
  * Validate DUT ingress transmission using gNMI path `/interfaces/interface[name=Ethernet1/1]/state/counters/in-pkts`.

* Step 3 - Send baseline data plane traffic
  * Send 50,000 IPv4 packets (1514 bytes) from ATE port 1 to destination IP `198.51.100.1` at 1000 pps.
  * Send 50,000 IPv6 packets (1514 bytes) from ATE port 1 to destination IP `2001:db8:100::1` at 1000 pps.
  * Verify 0% packet loss for both streams using OTG flow tracking to confirm baseline gRIBI forwarding is active.
  * Verify TTL is decremented by 1 and Source/Destination MAC addresses are rewritten correctly by the router.
  * Validate DUT egress transmission using gNMI path `/interfaces/interface[name=Ethernet1/2]/state/counters/out-pkts`.

### P4RT-3.4.2 - Concurrent gRIBI and P4RT Packet-In operations

*Note: This subtest must build sequentially upon the state, configuration, and
sessions established in P4RT-3.4.1. Do not tear down sessions or reset the
device between subtests.*

* Step 1 - Start concurrent operations
  * Start continuous injection of LLDP (1514 bytes) and IPv4 Traceroute (TTL=1, 1514 bytes) packets from ATE port 1 at a constant rate (1000 packets per second) to be punted to the controller.
  * Simultaneously start gRIBI programming (Add, Delete, Replace) for `100,000` different IPv4 (`198.51.x.y/32`) and `100,000` IPv6 (`2001:db8:x::y/128`) routes.
  * Distribute these routes across multiple Next-Hop Groups utilizing ECMP across ATE port 3 and ATE port 4.
    * Next-Hops pointing to `192.0.2.10` (ATE port 3) and `192.0.2.14` (ATE port 4).
    * Next-Hops pointing to `2001:db8:3::2` (ATE port 3) and `2001:db8:4::2` (ATE port 4).

* Step 2 - Validation with pass/fail criteria
  * Rely on `gNMI.Subscribe` (ON_CHANGE) on AFT state paths to verify route installation without using arbitrary wait times. Ensure the exact count of `100,000` IPv4 and `100,000` IPv6 routes is reached.
  * Stop Packet-In injection and verify exact packet counts received by the controller match the sent counts (0% drop, no corrupted headers).
  * Verify gRIBI routing table updates continue to succeed under P4RT Packet-In load by verifying route installation via telemetry on AFT state.
  * Send test data plane traffic to the dynamically programmed `100,000` routes and verify successful forwarding (0% loss) out of ATE port 3 and ATE port 4 with correct TTL decrement and MAC rewrite.

### P4RT-3.4.3 - Negative Scenarios under Concurrent Load

*Note: This subtest must build sequentially upon the state established in P4RT-3.4.2 and run while the concurrent load is active.*

* Step 1 - Invalid P4RT Table Entry
  * Attempt to insert an invalid P4RT Packet-In rule while concurrent gRIBI route programming is active.
  * *Validation*: Verify that the P4RT controller receives an appropriate error and the concurrent gRIBI operations continue uninterrupted.

* Step 2 - Port State Down
  * Administratively disable the ingress port (`/interfaces/interface[name=Ethernet1/1]/config/enabled` = `false`) via gNMI while concurrent Packet-In traffic and gRIBI programming is occurring.
  * *Validation*: Verify that Packet-In traffic from that port stops being punted to the controller, but gRIBI route programming for other healthy ports completes successfully.

* Step 3 - Session Disconnect
  * Drop the gRIBI session during active Packet-In traffic and verify P4RT operations are unaffected.
  * Drop the P4RT session during active gRIBI programming and verify gRIBI operations continue uninterrupted.

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
