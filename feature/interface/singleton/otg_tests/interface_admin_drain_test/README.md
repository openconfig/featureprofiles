# RT-5.17: Physical Interface Drain via Admin Down

## Summary

Validates that setting a physical interface's administrative state (`enabled`)
to `false` via gNMI causes associated routing protocols (BGP peering sessions
and IS-IS Level-2 adjacencies) to tear down cleanly, ceases all traffic
forwarding across the drained interface, and ensures pass-through traffic
across non-drained interfaces remains completely unaffected. Furthermore,
validates that re-enabling the interface restores routing protocol sessions
and traffic forwarding.

## Testbed type

* https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed

## Topology

```text
               +------------+
  ATE Port 1 --| Port 1     |-- (Drained Physical Interface Under Test)
  ATE Port 2 --| Port 2 DUT |-- (Ingress Traffic Generator Port)
  ATE Port 3 --| Port 3     |-- (Pass-through / Egress Port)
  ATE Port 4 --| Port 4     |-- (Auxiliary Transit / Monitoring Port)
               +------------+
```

* **DUT Port 1** (Drained Interface): Connected to **ATE Port 1**.
  * IPv4: `198.51.100.1/31`, IPv6: `2001:db8:1::1/127`
  * ATE IPv4: `198.51.100.0/31`, ATE IPv6: `2001:db8:1::0/127`
  * BGP Peer: eBGP between DUT AS `64500` and ATE AS `64501`
  * IS-IS: Level-2 point-to-point adjacency
* **DUT Port 2** (Ingress Source): Connected to **ATE Port 2**.
  * IPv4: `198.51.100.3/31`, IPv6: `2001:db8:1::3/127`
  * ATE IPv4: `198.51.100.2/31`, ATE IPv6: `2001:db8:1::2/127`
* **DUT Port 3** (Pass-through Egress): Connected to **ATE Port 3**.
  * IPv4: `198.51.100.5/31`, IPv6: `2001:db8:1::5/127`
  * ATE IPv4: `198.51.100.4/31`, ATE IPv6: `2001:db8:1::4/127`
  * BGP Peer: eBGP between DUT AS `64500` and ATE AS `64503`
  * IS-IS: Level-2 point-to-point adjacency
* **DUT Port 4** (Auxiliary Transit): Connected to **ATE Port 4**.
  * IPv4: `198.51.100.7/31`, IPv6: `2001:db8:1::7/127`
  * ATE IPv4: `198.51.100.6/31`, ATE IPv6: `2001:db8:1::6/127`

## Procedure

### Test environment setup

1. Establish Layer 3 interface configurations on DUT Ports 1, 2, 3, and 4
   with dual-stack IPv4 and IPv6 subnets.
2. Establish eBGP sessions:
   * DUT Port 1 to ATE Port 1: ATE advertises prefix `203.0.113.0/24` (IPv4)
     and `2001:db8:2::/64` (IPv6).
   * DUT Port 3 to ATE Port 3: ATE advertises prefix `203.0.113.128/25` (IPv4)
     and `2001:db8:3::/64` (IPv6).
3. Establish IS-IS Level-2 point-to-point adjacencies over DUT Port 1 and
   DUT Port 3 with standard metrics.
4. Verify routing protocol session states:
   * `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=BGP][name=bgp]/bgp/neighbors/neighbor[neighbor-address=198.51.100.0]/state/session-state`
     is `ESTABLISHED`.
   * `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=BGP][name=bgp]/bgp/neighbors/neighbor[neighbor-address=2001:db8:1::0]/state/session-state`
     is `ESTABLISHED`.
   * `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=BGP][name=bgp]/bgp/neighbors/neighbor[neighbor-address=198.51.100.4]/state/session-state`
     is `ESTABLISHED`.
   * `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=BGP][name=bgp]/bgp/neighbors/neighbor[neighbor-address=2001:db8:1::4]/state/session-state`
     is `ESTABLISHED`.
   * IS-IS Level-2 adjacencies on Port 1 and Port 3 are in `UP` state via
     `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=DEFAULT]/isis/interfaces/interface/levels/level[level-number=2]/adjacencies/adjacency/state/adjacency-state`.
5. Start continuous bidirectional traffic streams from ATE Port 2:
   * **Stream 1**: Ingress ATE Port 2 -> Destination `203.0.113.10` /
     `2001:db8:2::10` (Egress DUT Port 1).
   * **Stream 2**: Ingress ATE Port 2 -> Destination `203.0.113.130` /
     `2001:db8:3::10` (Egress DUT Port 3).
6. Verify baseline forwarding: both Stream 1 and Stream 2 experience 0%
   packet loss at line rate.

### RT-5.17.1 - Physical Interface Admin Down Drain

1. Administratively disable DUT Port 1 via gNMI.Set:
   * Set `/interfaces/interface[name=port1]/config/enabled` to `false`.

#### Canonical OC

```json
{
  "openconfig-interfaces:interfaces": {
    "interface": [
      {
        "config": {
          "enabled": false,
          "name": "port1"
        },
        "name": "port1"
      }
    ]
  }
}
```

2. Validate Interface State via telemetry:
   * `/interfaces/interface[name=port1]/state/enabled` is `false`.
   * `/interfaces/interface[name=port1]/state/admin-status` transitions to `DOWN`.
   * `/interfaces/interface[name=port1]/state/oper-status` transitions to `DOWN`.

3. Validate Routing Protocol Reaction:
   * Verify BGP neighbor sessions over Port 1 (`198.51.100.0` and
     `2001:db8:1::0`) transition away from `ESTABLISHED` (to `IDLE` or `ACTIVE`)
     within the hold-time / fast-down interval.
   * Verify IS-IS Level-2 adjacency on Port 1 transitions to `DOWN` and is
     removed from the active IS-IS neighbor table.
   * Verify BGP neighbor sessions and IS-IS adjacencies over Port 3 remain
     continuously `ESTABLISHED` and `UP` without flapping.

4. Validate Data Plane Drain & Pass-Through Traffic Isolation:
   * Verify traffic for Stream 1 (destined to Port 1 prefixes) immediately
     ceases to egress DUT Port 1;
     `/interfaces/interface[name=port1]/state/counters/out-pkts` stops
     incrementing.
   * Verify pass-through traffic for Stream 2 (destined to Port 3 prefixes)
     continues forwarding through DUT Port 3 with 0% packet loss and no rate
     degradation.

5. Pass/Fail Criteria:
   * **Pass**:
     * DUT Port 1 `admin-status` and `oper-status` are `DOWN`.
     * BGP sessions and IS-IS adjacencies on Port 1 tear down.
     * Forwarding on Port 1 completely ceases.
     * Stream 2 on Port 3 forwards with 0% packet loss and 0 protocol flaps.
   * **Fail**:
     * DUT Port 1 `oper-status` remains `UP`.
     * BGP or IS-IS neighbor sessions over Port 1 remain `ESTABLISHED` or `UP`.
     * Packets continue to be transmitted out of Port 1.
     * Pass-through traffic on Port 3 experiences packet loss or session
       interruption.

### RT-5.17.2 - Physical Interface Un-drain and Traffic Restoration

1. Administratively re-enable DUT Port 1 via gNMI.Set:
   * Set `/interfaces/interface[name=port1]/config/enabled` to `true`.

#### Canonical OC

```json
{
  "openconfig-interfaces:interfaces": {
    "interface": [
      {
        "config": {
          "enabled": true,
          "name": "port1"
        },
        "name": "port1"
      }
    ]
  }
}
```

2. Validate Interface Operational State via telemetry:
   * `/interfaces/interface[name=port1]/state/enabled` is `true`.
   * `/interfaces/interface[name=port1]/state/admin-status` transitions to `UP`.
   * `/interfaces/interface[name=port1]/state/oper-status` transitions to `UP`.

3. Validate Routing Protocol Restoration:
   * Verify BGP neighbor sessions over Port 1 (`198.51.100.0` and
     `2001:db8:1::0`) re-establish to `ESTABLISHED`.
   * Verify IS-IS Level-2 adjacency over Port 1 re-establishes to `UP`.
   * Verify learned prefixes (`203.0.113.0/24` and `2001:db8:2::/64`) are
     re-installed into the FIB.

4. Validate Traffic Flow Recovery:
   * Verify Stream 1 resumes line-rate forwarding across DUT Port 1 with 0%
     steady-state packet loss.
   * Verify Stream 2 on Port 3 remains stable with 0% packet loss.

5. Pass/Fail Criteria:
   * **Pass**:
     * DUT Port 1 `oper-status` returns to `UP`.
     * BGP neighbor sessions and IS-IS adjacencies on Port 1 return to
       `ESTABLISHED` / `UP`.
     * Stream 1 resumes 100% throughput with 0% steady-state packet loss.
   * **Fail**:
     * DUT Port 1 remains `DOWN`.
     * BGP or IS-IS sessions fail to re-establish on Port 1.
     * Forwarding on Port 1 fails to restore or experiences packet loss.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths and RPCs intended to be covered by this
test.

```yaml
paths:
  ## Interface configuration and operational state
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/name:
  /interfaces/interface/state/admin-status:
  /interfaces/interface/state/enabled:
  /interfaces/interface/state/name:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/state/counters/in-octets:
  /interfaces/interface/state/counters/in-pkts:
  /interfaces/interface/state/counters/out-octets:
  /interfaces/interface/state/counters/out-pkts:

  ## Routing Protocol State (BGP & IS-IS)
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/adjacency-state:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Required DUT platform

* FFF
* MFF
