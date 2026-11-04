# Health-1.3: System Process and Controller Card Memory Leak Detection

## Summary

This test validates system process, controller-card memory, and hardware
resource stability on a DUT against unbounded memory leaks within a realistic
`~44-minute` execution budget (`< 50 minutes`). The specification is organized
into five distinct, cross-referenced sections:

- **Section 1 (Test environment setup)**: Defines physical port mappings,
  RFC-compliant IPv4/IPv6 addressing, 3-way ECMP eBGP peers, route sets
  (`Baseline-Routes` and `Churn-Routes`), and sentinel AFT prefixes.
- **Section 2 (OpenConfig Path Groups)**: Groups all OpenConfig leaf paths by
  function (`[OC-Group-A]` through `[OC-Group-E]`) and maps each group to the
  test cases that use it.
- **Section 3 (Positive Test Cases `Health-1.3.1`–`Health-1.3.4`)**: Validates
  baseline memory telemetry (`~3 min`), `15-minute` steady-state per-process and
  controller-card memory polling, `15-minute` accelerated `20,000`-route
  dual-stack eBGP churn (`60,000` BGP paths across 3 peers over `5` cycles =
  `600,000` cumulative path transitions), and concurrent hardware FIB resource
  utilization.
- **Section 4 (Negative Test Cases `Health-1.3.5`–`Health-1.3.7`)**: Validates
  process and controller-card memory stability during BGP max-prefix limit
  violations (`~3 min`), `5` egress link admin-down/up flaps (`~4 min`), and `3`
  abrupt BGP peer session resets (`~3 min`).
- **Section 5 (Cleanup)**: Restores all DUT and ATE state (`~1 min`).

## Testbed type

* [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

* **Physical Links and Interface Addressing** (configured in
  [Canonical OC](#canonical-oc) under `"interfaces"`):
  * `Link 1` (`port1` <-> `atePort1`, Egress ECMP Path 1): DUT `192.0.2.1/30`
    and `2001:db8::1/126` <-> ATE `192.0.2.2/30` and `2001:db8::2/126`.
  * `Link 2` (`port2` <-> `atePort2`, Egress ECMP Path 2): DUT `192.0.2.5/30`
    and `2001:db8::5/126` <-> ATE `192.0.2.6/30` and `2001:db8::6/126`.
  * `Link 3` (`port3` <-> `atePort3`, Egress ECMP Path 3): DUT `192.0.2.9/30`
    and `2001:db8::9/126` <-> ATE `192.0.2.10/30` and `2001:db8::a/126`.
  * `Link 4` (`port4` <-> `atePort4`, Ingress Validation Port): DUT
    `192.0.2.13/30` and `2001:db8::d/126` <-> ATE `192.0.2.14/30` and
    `2001:db8::e/126`.
* **DUT and ATE eBGP Control Plane** (configured in
  [Canonical OC](#canonical-oc) under `"network-instances"` and
  `"routing-policy"`):
  * DUT BGP Global: Network-instance `DEFAULT`, local AS `64500`, router-id
    `192.0.2.1`, eBGP `maximum-paths` = `3`, routing policy `ALLOW-ALL`
    (`ACCEPT_ROUTE`) applied on import and export for `IPV4_UNICAST` and
    `IPV6_UNICAST`.
  * `Peer 1` on `Link 1` (`atePort1`): Neighbor `192.0.2.2` and `2001:db8::2`,
    peer AS `64501`.
  * `Peer 2` on `Link 2` (`atePort2`): Neighbor `192.0.2.6` and `2001:db8::6`,
    peer AS `64502`.
  * `Peer 3` on `Link 3` (`atePort3`): Neighbor `192.0.2.10` and `2001:db8::a`,
    peer AS `64503`.
* **Advertised BGP Route Sets and Sentinel AFT Prefixes on `Peer 1`–`Peer 3`**:
  * `Baseline-Routes` (used in `Health-1.3.1`–`Health-1.3.7`, `500` dual-stack
    routes per peer): `250` IPv4 `/32` prefixes (`203.0.113.1/32`–
    `203.0.113.250/32`, RFC 5737) and `250` IPv6 `/64` prefixes
    (`2001:db8:2:1::/64`–`2001:db8:2:fa::/64`, RFC 3849).
    * `Baseline-Sentinel-Prefixes` (for fast point-lookup AFT validation):
      `203.0.113.1/32`, `203.0.113.250/32`, `2001:db8:2:1::/64`, and
      `2001:db8:2:fa::/64`.
  * `Churn-Routes` (used in `Health-1.3.3`–`Health-1.3.7`, `20,000` unique
    dual-stack routes per peer = `60,000` total eBGP paths across 3 peers):
    `10,000` IPv4 `/32` prefixes (`100.64.0.1/32`–`100.64.39.16/32`, RFC 6598)
    and `10,000` IPv6 `/64` prefixes (`2001:db8:10:1::/64`–
    `2001:db8:10:2710::/64`, RFC 3849).
    * `Churn-Sentinel-Prefixes` (for fast point-lookup AFT validation without
      streaming the full AFT table): `100.64.0.1/32`, `100.64.39.16/32`,
      `2001:db8:10:1::/64`, and `2001:db8:10:2710::/64`.
* **Data-Plane Validation Traffic**:
  * Transmit dual-stack IPv4 and IPv6 packets from ingress `atePort4` (`Link 4`)
    destined to the active route set (`Baseline-Routes` or `Churn-Routes`)
    across egress `atePort1`–`atePort3` (`Link 1`–`Link 3`) at `10,000 pps` per
    family (`512` and `1500` byte frames, UDP source ports `10000`–`10999`).
* **Initial Bring-Up**:
  * Push the [Canonical OC](#canonical-oc) configuration to the DUT using
    `gNMI.Set` (`REPLACE`), start ATE protocols, resolve IPv4 ARP and IPv6 ND
    neighbors (`WaitForARP`), and use `gnmi.Watch` (`ON_CHANGE`) on
    `[OC-Group-C]` to confirm `port1`–`port4` are `UP` and all six eBGP sessions
    (`Peer 1`–`Peer 3`, IPv4 and IPv6) are `ESTABLISHED`.

---

### OpenConfig Path Groups (Referenced by Test Cases)

To prevent path repetition across subtests, all OpenConfig leaves are organized
into five functional groups below and listed in the machine-readable
[OpenConfig Path and RPC Coverage](#openconfig-path-and-rpc-coverage) section:

* **`[OC-Group-A]` — System Process Memory Telemetry** *(Used by:
  `Health-1.3.1`, `Health-1.3.2`, `Health-1.3.3`, `Health-1.3.5`,
  `Health-1.3.6`, `Health-1.3.7`)*:
  * Process discovery: `/system/processes/process/state/pid`,
    `/system/processes/process/state/name`, and
    `/system/processes/process/state/start-time`.
  * Per-process memory and CPU usage (queried for each running `pid`):
    `/system/processes/process/state/memory-usage`,
    `/system/processes/process/state/memory-utilization`, and
    `/system/processes/process/state/cpu-utilization`.
* **`[OC-Group-B]` — Controller Card and Component Memory Telemetry** *(Used by:
  `Health-1.3.1`, `Health-1.3.2`, `Health-1.3.3`, `Health-1.3.6`)*:
  * Component identity and operational state for `CHASSIS`, active and standby
    `CONTROLLER_CARD`, and `CPU`: `/components/component/state/type` and
    `/components/component/state/oper-status`.
  * Component memory state: `/components/component/state/memory/available` and
    `/components/component/state/memory/utilized`.
* **`[OC-Group-C]` — BGP Session, AFT Route, and Interface Counter Telemetry**
  *(Used by: `Health-1.3.1`, `Health-1.3.3`, `Health-1.3.5`, `Health-1.3.6`,
  `Health-1.3.7`)*:
  * BGP session and prefix counters:
    `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state`,
    `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/established-transitions`,
    `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received`,
    and
    `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/installed`.
  * AFT programmed entries (queried on `Baseline-Sentinel-Prefixes` and
    `Churn-Sentinel-Prefixes`) and ECMP next-hops:
    `/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix`,
    `/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix`,
    `/network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id`,
    `/network-instances/network-instance/afts/next-hops/next-hop/state/index`,
    and
    `/network-instances/network-instance/afts/next-hops/next-hop/state/ip-address`.
  * Interface status and packet counters:
    `/interfaces/interface/state/oper-status`,
    `/interfaces/interface/state/counters/in-pkts`, and
    `/interfaces/interface/state/counters/out-pkts`.
* **`[OC-Group-D]` — Hardware Resource Utilization Config and Telemetry** *(Used
  by: `Health-1.3.4`)*:
  * System resource threshold config and state:
    `/system/utilization/resources/resource/config/name`,
    `/system/utilization/resources/resource/config/used-threshold-upper`,
    `/system/utilization/resources/resource/config/used-threshold-upper-clear`,
    `/system/utilization/resources/resource/state/name`,
    `/system/utilization/resources/resource/state/used-threshold-upper`, and
    `/system/utilization/resources/resource/state/used-threshold-upper-clear`.
  * Integrated circuit (`INTEGRATED_CIRCUIT`) FIB resource utilization state:
    `/components/component/integrated-circuit/utilization/resources/resource/state/name`,
    `/components/component/integrated-circuit/utilization/resources/resource/state/used`,
    `/components/component/integrated-circuit/utilization/resources/resource/state/free`,
    `/components/component/integrated-circuit/utilization/resources/resource/state/max-limit`,
    `/components/component/integrated-circuit/utilization/resources/resource/state/committed`,
    `/components/component/integrated-circuit/utilization/resources/resource/state/high-watermark`,
    `/components/component/integrated-circuit/utilization/resources/resource/state/used-threshold-upper`,
    `/components/component/integrated-circuit/utilization/resources/resource/state/used-threshold-upper-clear`,
    and
    `/components/component/integrated-circuit/utilization/resources/resource/state/used-threshold-upper-exceeded`.
* **`[OC-Group-E]` — Negative Test Control-Plane and Link Config/State Paths**
  *(Used by: `Health-1.3.5` and `Health-1.3.6`)*:
  * BGP per-AFI-SAFI prefix limit config and state (`Health-1.3.5`):
    `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit/config/max-prefixes`,
    `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit/config/warning-threshold-pct`,
    `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit/state/prefix-limit-exceeded`,
    `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv6-unicast/prefix-limit/config/max-prefixes`,
    `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv6-unicast/prefix-limit/config/warning-threshold-pct`,
    and
    `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv6-unicast/prefix-limit/state/prefix-limit-exceeded`.
  * Interface administrative state (`Health-1.3.6`):
    `/interfaces/interface/config/enabled`.

---

### Positive Test Cases

### Health-1.3.1 - Validate Baseline System Process and Controller Card Memory Telemetry

* **Connected Setup & OC Groups**: Uses `Link 1`–`Link 4`, `Peer 1`–`Peer 3`,
  `Baseline-Routes` (`250` IPv4 + `250` IPv6), and OpenConfig groups
  `[OC-Group-A]`, `[OC-Group-B]`, and `[OC-Group-C]` (`~3 minutes`).
* Step 1 - Advertise `Baseline-Routes` from `Peer 1`, `Peer 2`, and `Peer 3`,
  and verify via `gnmi.Watch` on `[OC-Group-C]` that `prefixes/received` and
  `prefixes/installed` equal `250` per family on all three peers and that
  `Baseline-Sentinel-Prefixes` resolve to a 3-way ECMP `next-hop-group` in the
  AFT.
* Step 2 - Query `[OC-Group-A]` via a batched `gNMI.Get` to enumerate all
  running process IDs (`pid`) and names (`name`) and record the initial baseline
  `memory-usage`, `memory-utilization`, `cpu-utilization`, and `start-time` for
  each running `pid`.
* Step 3 - Query `[OC-Group-B]` via `gNMI.Get` to record the baseline
  `memory/available` and `memory/utilized` for all `CHASSIS`, active and standby
  `CONTROLLER_CARD`, and `CPU` components.
* Step 4 - Send data-plane validation traffic from `atePort4` to
  `Baseline-Routes` for `15 seconds` and verify via `[OC-Group-C]` counters and
  OTG metrics that packet loss is `0%` and traffic is balanced across `port1`,
  `port2`, and `port3` (`33.3% ± 5%`).
* Step 5 - Pass/Fail Criteria: Pass if all running processes in `[OC-Group-A]`
  and components in `[OC-Group-B]` report valid non-zero baseline telemetry and
  baseline traffic forwards with `0%` loss; Fail otherwise.

### Health-1.3.2 - Validate Steady-State Per-Process Memory Usage Over Time

* **Connected Setup & OC Groups**: Builds on `Health-1.3.1` (`Baseline-Routes`
  installed) and polls `[OC-Group-A]` and `[OC-Group-B]` (`15 minutes` default
  FNT duration; configurable up to `60 minutes` for extended soak runs).
* Step 1 - Poll `[OC-Group-A]` (for every running `pid`) and `[OC-Group-B]`
  (for active and standby `CONTROLLER_CARD` components) every `60 seconds` for a
  total duration of `15 minutes` (`15` samples).
* Step 2 - Compare `memory-usage` in `[OC-Group-A]` across consecutive
  `60 seconds` samples for each process, treating samples `1`–`5` (`5 minutes`)
  as the warm-up window (allowing bounded caches and active/standby
  controller-card state-synchronization history buffers to reach their retention
  threshold) and samples `6`–`15` (`10 minutes`) as the steady-state evaluation
  window.
* Step 3 - Pass/Fail Criteria: Pass if no process restarts (constant `pid` and
  `start-time` in `[OC-Group-A]`) and no process exhibits both a monotonic
  upward linear trend (`R² >= 0.85`) and net `memory-usage` growth exceeding
  `5%` (and `10 MB`) from sample `6` to sample `15`; Fail and report the
  offending process (`pid` and `name`) as a memory leak otherwise.

### Health-1.3.3 - Validate Process Memory Reclamation Under Scaled Dual-Stack eBGP Route Churn

* **Connected Setup & OC Groups**: Builds on `Health-1.3.2`, flapping
  `Churn-Routes` (`10,000` IPv4 + `10,000` IPv6 = `20,000` routes per peer,
  `60,000` total BGP paths across `Peer 1`–`Peer 3`) and monitoring
  `[OC-Group-A]`, `[OC-Group-B]`, and `[OC-Group-C]` (`15 minutes`).
* Step 1 - Advertise `Churn-Routes` alongside `Baseline-Routes` from `Peer 1`,
  `Peer 2`, and `Peer 3`, confirm via `gnmi.Watch` on `[OC-Group-C]` that
  `prefixes/installed` reaches `10,250` per family on all three peers and
  `Churn-Sentinel-Prefixes` are present in the AFT, and verify `0%` traffic loss
  to `Churn-Routes`.
* Step 2 - Hold `Churn-Routes` advertised for `90 seconds`, then withdraw
  `Churn-Routes` from all three peers and hold withdrawn for `90 seconds` (`3
  minutes` per full advertise/withdraw cycle) while keeping `Baseline-Routes`
  active.
* Step 3 - During each withdraw hold, confirm via `gnmi.Watch` on `[OC-Group-C]`
  that `prefixes/installed` returns to `250` per family and
  `Churn-Sentinel-Prefixes` are removed from the AFT. Send traffic to the
  withdrawn `Churn-Routes` for `5 seconds` and verify `100%` packet loss
  (confirming no stale FIB entries remain), while traffic to `Baseline-Routes`
  continues with `0%` loss.
* Step 4 - Repeat Steps 1–3 across `5` consecutive churn cycles over `15
  minutes` (`600,000` cumulative BGP path advertise/withdraw transitions across
  the 3 peers) while polling `[OC-Group-A]` and `[OC-Group-B]` every `60
  seconds`.
* Step 5 - Pass/Fail Criteria: Pass if all six eBGP sessions remain
  `ESTABLISHED` with constant `established-transitions` in `[OC-Group-C]` and,
  using Cycle `1` post-withdrawal memory as the arena-warmed baseline,
  post-withdrawal per-process `memory-usage` in `[OC-Group-A]` across Cycles
  `2`–`5` does not exhibit monotonic growth (`R² >= 0.85` with net unrecovered
  growth `> 5%` and `> 15 MB` above Cycle `1` post-withdrawal baseline); Fail
  and report the leaking process (`pid` and `name`) otherwise.

### Health-1.3.4 - Validate Hardware Resource Utilization Telemetry Under Route Scale

* **Connected Setup & OC Groups**: Runs concurrently with the `Churn-Routes`
  advertise and withdraw phases of `Health-1.3.3` using `[OC-Group-D]` (`0`
  additional minutes).
* Step 1 - Verify the hardware resource utilization thresholds configured in
  [Canonical OC](#canonical-oc) (`name` = `FIB`, `used-threshold-upper` = `80`,
  `used-threshold-upper-clear` = `70`) are reflected in the system resource
  state leaves of `[OC-Group-D]`.
* Step 2 - Query the `INTEGRATED_CIRCUIT` utilization state leaves in
  `[OC-Group-D]` before `Churn-Routes` advertisement, at peak `20,000`-route
  (`60,000`-path) installation, and after `Churn-Routes` withdrawal in Cycle `1`
  and Cycle `5`.
* Step 3 - Pass/Fail Criteria: Pass if hardware `used` increases and `free`
  decreases proportionally when `Churn-Routes` are installed (with
  `used-threshold-upper-exceeded` = `false`), and `used` and `free` return
  within `±1%` of the `Health-1.3.1` baseline after withdrawal without leaking
  hardware FIB entries; Fail otherwise.

---

### Negative Test Cases

### Health-1.3.5 - BGP Max-Prefix Limit Violation (Negative)

* **Connected Setup & OC Groups**: Targets `Peer 1` (`atePort1` on `Link 1`)
  while `Peer 2` and `Peer 3` remain active, using `Churn-Routes` and OpenConfig
  groups `[OC-Group-E]`, `[OC-Group-C]`, and `[OC-Group-A]` (`~3 minutes`).
* Step 1 - Configure `[OC-Group-E]` prefix limits (`max-prefixes` = `8000`,
  `warning-threshold-pct` = `80`) for `IPV4_UNICAST` and `IPV6_UNICAST` on
  `Peer 1` (`192.0.2.2` and `2001:db8::2`) via `gNMI.Set`.
* Step 2 - Advertise `Churn-Routes` (`10,000` IPv4 and `10,000` IPv6 routes)
  from `Peer 1`, `Peer 2`, and `Peer 3`.
* Step 3 - Verify via `gnmi.Watch` on `[OC-Group-E]` and `[OC-Group-C]` that
  `Peer 1` reports `prefix-limit-exceeded` = `true` and transitions to
  `session-state` = `IDLE`, while `Peer 2` and `Peer 3` remain `ESTABLISHED`
  (`prefixes/installed` = `10,250` per family) and forward traffic from
  `atePort4` with `0%` steady-state loss across 2-way ECMP (`50% ± 5%` split).
* Step 4 - Withdraw excess routes on `Peer 1`, remove the `[OC-Group-E]`
  prefix-limit configuration, re-establish `Peer 1` (`prefixes/installed` =
  `10,250` per family across all three peers), and poll `[OC-Group-A]`.
* Step 5 - Pass/Fail Criteria: Pass if `Peer 1` enforces the prefix limit,
  surviving peers forward traffic with `0%` loss, and per-process `memory-usage`
  in `[OC-Group-A]` remains within `<= 5%` (or `<= 10 MB`) of the post-churn
  baseline after session recovery without leaking rejected prefix buffers; Fail
  otherwise.

### Health-1.3.6 - Repeated Egress Interface Admin-Down Flapping Under High Route Scale (Negative)

* **Connected Setup & OC Groups**: Targets DUT `port1` (`Link 1`) with
  `Churn-Routes` (`20,000` dual-stack routes) installed across `Peer 1`–`Peer
  3`, using `[OC-Group-E]`, `[OC-Group-C]`, `[OC-Group-A]`, and `[OC-Group-B]`
  (`~4 minutes`).
* Step 1 - With `Churn-Routes` installed and traffic flowing, flap `port1`
  across `5` consecutive cycles by setting `[OC-Group-E]`
  `/interfaces/interface/config/enabled` to `false` (registering a deferred
  cleanup to restore `enabled` = `true`), awaiting `oper-status` = `DOWN` via
  `gnmi.Watch` on `[OC-Group-C]`, and then setting `enabled` = `true` and
  awaiting `oper-status` = `UP`.
* Step 2 - During each `DOWN` phase, verify via `[OC-Group-C]` and OTG metrics
  that traffic reconverges to `port2` and `port3` (`50% ± 5%` split) with `0%`
  steady-state loss; during each `UP` phase, verify 3-way ECMP restoration
  (`33.3% ± 5%` split) across `port1`–`port3`.
* Step 3 - Pass/Fail Criteria: Pass if querying `[OC-Group-A]` and
  `[OC-Group-B]` after the `5` flap cycles confirms that process `memory-usage`
  and controller-card `memory/utilized` return within `<= 5%` of their pre-flap
  baseline without retaining orphaned next-hop or state-synchronization buffers;
  Fail otherwise.

### Health-1.3.7 - Abrupt BGP Peer Session Reset Under High Route Scale (Negative)

* **Connected Setup & OC Groups**: Targets `Peer 2` (`atePort2` on `Link 2`)
  with `Churn-Routes` (`20,000` dual-stack routes) installed across `Peer 1`–
  `Peer 3`, using `[OC-Group-C]` and `[OC-Group-A]` (`~3 minutes`).
* Step 1 - With `Churn-Routes` (`10,000` IPv4 and `10,000` IPv6 prefixes)
  installed across all three peers, trigger `3` consecutive administrative BGP
  session resets on `Peer 2` (`192.0.2.6` and `2001:db8::6`).
* Step 2 - Verify via `gnmi.Watch` on `[OC-Group-C]` and OTG metrics that
  traffic continues with `0%` steady-state loss on `Peer 1` and `Peer 3` during
  each reset, and that `Peer 2` re-establishes and converges back to
  `prefixes/installed` = `10,250` per family after each reset.
* Step 3 - Pass/Fail Criteria: Pass if querying `[OC-Group-A]` confirms per-PID
  `memory-usage` remains within `<= 5%` (or `<= 10 MB`) of the pre-reset
  baseline with zero monotonic growth across repeated peer RIB flush and
  re-learning cycles; Fail otherwise.

---

### Cleanup

* Stop all active OTG traffic streams and protocols on `atePort1` through
  `atePort4`.
* Revert all BGP, routing-policy, resource-threshold, and interface
  configurations on the DUT using `t.Cleanup()` (`gNMI.Set` with `DELETE` /
  `REPLACE`) and verify via `[OC-Group-C]` that all DUT and ATE ports are
  restored to `UP` (`~1 minute`).

## Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "DUT Port 1 connected to ATE Port 1",
          "enabled": true,
          "name": "port1",
          "type": "iana-if-type:ethernetCsmacd"
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
          "description": "DUT Port 2 connected to ATE Port 2",
          "enabled": true,
          "name": "port2",
          "type": "iana-if-type:ethernetCsmacd"
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
      },
      {
        "config": {
          "description": "DUT Port 3 connected to ATE Port 3",
          "enabled": true,
          "name": "port3",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port3",
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
        "config": {
          "description": "DUT Port 4 connected to ATE Port 4",
          "enabled": true,
          "name": "port4",
          "type": "iana-if-type:ethernetCsmacd"
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
                        "ip": "2001:db8::d",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8::d"
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
          "name": "DEFAULT"
        },
        "name": "DEFAULT",
        "protocols": {
          "protocol": [
            {
              "bgp": {
                "global": {
                  "afi-safis": {
                    "afi-safi": [
                      {
                        "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                        "config": {
                          "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                          "enabled": true
                        }
                      },
                      {
                        "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                        "config": {
                          "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                          "enabled": true
                        }
                      }
                    ]
                  },
                  "config": {
                    "as": 64500,
                    "router-id": "192.0.2.1"
                  },
                  "use-multiple-paths": {
                    "ebgp": {
                      "config": {
                        "maximum-paths": 3
                      }
                    }
                  }
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ALLOW-ALL"
                                ],
                                "import-policy": [
                                  "ALLOW-ALL"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                              "enabled": true
                            }
                          }
                        ]
                      },
                      "config": {
                        "enabled": true,
                        "neighbor-address": "192.0.2.2",
                        "peer-as": 64501
                      },
                      "neighbor-address": "192.0.2.2"
                    },
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ALLOW-ALL"
                                ],
                                "import-policy": [
                                  "ALLOW-ALL"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                              "enabled": true
                            }
                          }
                        ]
                      },
                      "config": {
                        "enabled": true,
                        "neighbor-address": "2001:db8::2",
                        "peer-as": 64501
                      },
                      "neighbor-address": "2001:db8::2"
                    },
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ALLOW-ALL"
                                ],
                                "import-policy": [
                                  "ALLOW-ALL"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                              "enabled": true
                            }
                          }
                        ]
                      },
                      "config": {
                        "enabled": true,
                        "neighbor-address": "192.0.2.6",
                        "peer-as": 64502
                      },
                      "neighbor-address": "192.0.2.6"
                    },
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ALLOW-ALL"
                                ],
                                "import-policy": [
                                  "ALLOW-ALL"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                              "enabled": true
                            }
                          }
                        ]
                      },
                      "config": {
                        "enabled": true,
                        "neighbor-address": "2001:db8::6",
                        "peer-as": 64502
                      },
                      "neighbor-address": "2001:db8::6"
                    },
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ALLOW-ALL"
                                ],
                                "import-policy": [
                                  "ALLOW-ALL"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                              "enabled": true
                            }
                          }
                        ]
                      },
                      "config": {
                        "enabled": true,
                        "neighbor-address": "192.0.2.10",
                        "peer-as": 64503
                      },
                      "neighbor-address": "192.0.2.10"
                    },
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ALLOW-ALL"
                                ],
                                "import-policy": [
                                  "ALLOW-ALL"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                              "enabled": true
                            }
                          }
                        ]
                      },
                      "config": {
                        "enabled": true,
                        "neighbor-address": "2001:db8::a",
                        "peer-as": 64503
                      },
                      "neighbor-address": "2001:db8::a"
                    }
                  ]
                }
              },
              "config": {
                "identifier": "openconfig-policy-types:BGP",
                "name": "BGP"
              },
              "identifier": "openconfig-policy-types:BGP",
              "name": "BGP"
            }
          ]
        }
      }
    ]
  },
  "routing-policy": {
    "policy-definitions": {
      "policy-definition": [
        {
          "config": {
            "name": "ALLOW-ALL"
          },
          "name": "ALLOW-ALL",
          "statements": {
            "statement": [
              {
                "actions": {
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                },
                "config": {
                  "name": "allow-all-stmt"
                },
                "name": "allow-all-stmt"
              }
            ]
          }
        }
      ]
    }
  },
  "system": {
    "utilization": {
      "resources": {
        "resource": [
          {
            "config": {
              "name": "FIB",
              "used-threshold-upper": 80,
              "used-threshold-upper-clear": 70
            },
            "name": "FIB"
          }
        ]
      }
    }
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  ## Config Paths ##
  /interfaces/interface/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/global/use-multiple-paths/ebgp/config/maximum-paths:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit/config/max-prefixes:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit/config/warning-threshold-pct:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv6-unicast/prefix-limit/config/max-prefixes:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv6-unicast/prefix-limit/config/warning-threshold-pct:
  /system/utilization/resources/resource/config/name:
  /system/utilization/resources/resource/config/used-threshold-upper:
  /system/utilization/resources/resource/config/used-threshold-upper-clear:

  ## State Paths ##
  /system/processes/process/state/pid:
  /system/processes/process/state/name:
  /system/processes/process/state/start-time:
  /system/processes/process/state/cpu-utilization:
  /system/processes/process/state/memory-usage:
  /system/processes/process/state/memory-utilization:
  /components/component/state/type:
    platform_type: ["CHASSIS", "CONTROLLER_CARD", "CPU"]
  /components/component/state/oper-status:
    platform_type: ["CHASSIS", "CONTROLLER_CARD"]
  /components/component/state/memory/available:
    platform_type: ["CHASSIS", "CONTROLLER_CARD", "CPU"]
  /components/component/state/memory/utilized:
    platform_type: ["CHASSIS", "CONTROLLER_CARD", "CPU"]
  /system/utilization/resources/resource/state/name:
  /system/utilization/resources/resource/state/used-threshold-upper:
  /system/utilization/resources/resource/state/used-threshold-upper-clear:
  /components/component/integrated-circuit/utilization/resources/resource/state/name:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/utilization/resources/resource/state/used:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/utilization/resources/resource/state/free:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/utilization/resources/resource/state/max-limit:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/utilization/resources/resource/state/committed:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/utilization/resources/resource/state/high-watermark:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/utilization/resources/resource/state/used-threshold-upper:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/utilization/resources/resource/state/used-threshold-upper-clear:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/utilization/resources/resource/state/used-threshold-upper-exceeded:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /interfaces/interface/state/oper-status:
  /interfaces/interface/state/counters/in-pkts:
  /interfaces/interface/state/counters/out-pkts:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/established-transitions:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/installed:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit/state/prefix-limit-exceeded:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv6-unicast/prefix-limit/state/prefix-limit-exceeded:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
      sample: true
```

## Required DUT platform

* FFF
