# RT-5.19: LAG Member-Link Drain

## Summary

Verify that administratively disabling a single member link of a Link
Aggregation Group (LAG) via gNMI causes traffic to redistribute to the remaining
active member links with minimal packet loss. The overall LAG must remain up,
and BGP/IS-IS sessions running over the LAG must remain stable.

Success Metrics:
* Traffic Redistribution: 100% of traffic is successfully carried by the remaining active links post-drain.
* Redistribution Loss: Packet loss duration during the member link disable is under the target milliseconds (e.g., < 10ms depending on platform expectations).

## Testbed type

* `TESTBED_DUT_ATE_4LINKS`

## Procedure

### Test environment setup

*   Connect ATE port-1 to DUT port-1.
*   Connect ATE ports 2 through 4 to DUT ports 2 through 4.
*   Configure DUT ports 2-4 and ATE ports 2-4 to be part of a single LAG (e.g., `PortChannel1`).
*   Configure dual-stack IPv4 and IPv6 addressing on DUT port-1 and ATE port-1.
    *   IPv4: `192.0.2.1/30` (DUT) and `192.0.2.2/30` (ATE).
    *   IPv6: `2001:db8::1/126` (DUT) and `2001:db8::2/126` (ATE).
*   Configure dual-stack IPv4 and IPv6 addressing on the LAG interface.
    *   IPv4: `198.51.100.1/30` (DUT) and `198.51.100.2/30` (ATE).
    *   IPv6: `2001:db8:1::1/126` (DUT) and `2001:db8:1::2/126` (ATE).
*   Establish eBGP and IS-IS sessions between ATE and DUT over both the standalone link (port-1) and the LAG interface.
*   Advertise 10,000 IPv4 and 10,000 IPv6 routes from the ATE over the LAG BGP sessions.

### RT-5.19.1 - Member Link Drain and Restore

*   **Step 1:** Generate DUT configuration and push via gNMI Set.
    *   Verify that `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state` is `ESTABLISHED` for sessions over both the standalone link and the LAG.
    *   Verify that `/network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/adjacency-state` is `UP` for adjacencies over both the standalone link and the LAG.
    *   Verify that the LAG `/interfaces/interface[name=<PortChannel1>]/state/oper-status` is `UP` and all member links have `/interfaces/interface[name=<DUT_port>]/state/oper-status` as `UP`.
*   **Step 2:** Start traffic.
    *   Send continuous IPv4 and IPv6 traffic from ATE port-1 destined to the 10,000 IPv4 and 10,000 IPv6 routes advertised over the LAG. Ensure the traffic generator creates large-scale entropy (e.g., 10,000+ unique flows by varying Source/Destination IP and UDP ports) to allow for effective LAG hashing.
    *   Verify that the traffic is load-balanced across all three member links (DUT ports 2-4) of the LAG. Each link should carry ~33.3% of the traffic (within a ±5% tolerance).
    *   Verify that ATE traffic loss is 0%.
*   **Step 3:** Disable one member link.
    *   Use gNMI to set `/interfaces/interface[name=<DUT_port-2>]/config/enabled` to `false`.

#### Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "name": "Ethernet2",
        "config": {
          "name": "Ethernet2",
          "enabled": false
        }
      }
    ]
  }
}
```

*   **Step 4:** Validate state after link drain.
    *   Verify that `/interfaces/interface[name=<DUT_port-2>]/state/enabled` is `false`.
    *   Verify that `/interfaces/interface[name=<DUT_port-2>]/state/oper-status` transitions to `DOWN`.
    *   Verify that the LAG interface `/interfaces/interface[name=<PortChannel1>]/state/oper-status` remains `UP`.
    *   Verify that the BGP `session-state` and IS-IS `adjacency-state` over the LAG remain stable without flapping.
    *   Verify that traffic from ATE port-1 redistributes evenly to the remaining active member links (DUT ports 3 and 4). Each link should carry ~50% of the traffic (within a ±5% tolerance).
    *   Calculate loss duration = `(Tx frames - Rx frames) / (Tx frame rate)`. Verify loss duration is under the acceptable threshold.
*   **Step 5:** Re-enable the member link.
    *   Use gNMI to set `/interfaces/interface[name=<DUT_port-2>]/config/enabled` to `true`.
*   **Step 6:** Validate state after link restore.
    *   Verify that `/interfaces/interface[name=<DUT_port-2>]/state/enabled` is `true`.
    *   Verify that `/interfaces/interface[name=<DUT_port-2>]/state/oper-status` transitions to `UP`.
    *   Verify that the BGP `session-state` and IS-IS `adjacency-state` over the LAG remain stable without flapping.
    *   Verify that traffic from ATE port-1 redistributes evenly across all three member links (DUT ports 2-4), with each carrying ~33.3% of the traffic (within a ±5% tolerance).
    *   Verify that ATE traffic loss is 0% in steady state.

### RT-5.19.2 - Successive Member Link Drain

*   **Step 1:** From the restored state of RT-5.19.1, disable the second member link.
    *   Use gNMI to set `/interfaces/interface[name=<DUT_port-3>]/config/enabled` to `false`.
*   **Step 2:** Validate state.
    *   Verify `/interfaces/interface[name=<DUT_port-3>]/state/oper-status` transitions to `DOWN`.
    *   Verify LAG `/interfaces/interface[name=<PortChannel1>]/state/oper-status` remains `UP`.
    *   Verify BGP and IS-IS sessions over the LAG remain stable.
    *   Verify that traffic from ATE port-1 redistributes evenly to the remaining active member links (DUT ports 2 and 4) with minimal packet loss. Each link should carry ~50% of the traffic (within a ±5% tolerance).
*   **Step 3:** Re-enable the member link.
    *   Use gNMI to set `/interfaces/interface[name=<DUT_port-3>]/config/enabled` to `true`.
    *   Verify traffic shifts back to load balance across DUT ports 2-4.

### RT-5.19.3 - All Member Links Drain

*   **Step 1:** Disable all active member links.
    *   Use gNMI to set `/interfaces/interface/config/enabled` to `false` for DUT ports 2, 3, and 4.
*   **Step 2:** Validate state.
    *   Verify the LAG `/interfaces/interface[name=<PortChannel1>]/state/oper-status` transitions to `DOWN`.
    *   Verify BGP `session-state` transitions out of `ESTABLISHED` (e.g., `ACTIVE` or `IDLE`) and IS-IS `adjacency-state` transitions to `DOWN`.
    *   Verify 100% traffic loss.
*   **Step 3:** Re-enable all member links.
    *   Use gNMI to set `/interfaces/interface/config/enabled` to `true` for DUT ports 2, 3, and 4.
*   **Step 4:** Validate recovery.
    *   Verify LAG `/interfaces/interface[name=<PortChannel1>]/state/oper-status` transitions to `UP`.
    *   Verify BGP `session-state` returns to `ESTABLISHED` and IS-IS `adjacency-state` returns to `UP`.
    *   Verify traffic forwarding resumes at 100% and is load-balanced across all three member links.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/name:
  /interfaces/interface/state/enabled:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/aggregation/members/member/state/interface:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/adjacency-state:
rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

* vRX
