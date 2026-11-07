# TE-1.23: gRIBI Tunnel Programming over Active BGP

## Summary

This test validates that gRIBI-injected IP-in-IP tunnel routes for specific IPv4
and IPv6 prefixes override active eBGP-learned routes for those exact same
prefixes in the forwarding plane. The gRIBI specification (section 4.1.8) leaves
the gRIBI route preference device-local but requires it to be configurable, so
the test first establishes, and then proves, that the DUT prefers gRIBI entries
over eBGP routes. It verifies that the network operating system prioritizes SDN
controller traffic-engineering intent (`gRIBI` `ADD`) over dynamic routing
protocols (`BGP`), encapsulates matching traffic onto the programmed ECMP tunnel
path without disturbing unrelated BGP-only prefixes, keeps the override isolated
from BGP churn of the overridden prefixes, accurately reports route precedence
in both `gRIBI.Get` and gNMI Abstract Forwarding Table (AFT) telemetry, reverts
to the underlying active BGP path when the gRIBI routes are removed (`DELETE`),
and remains stable throughout. For both the `ADD` and the `DELETE` transition,
the batch must be acknowledged `FIB_PROGRAMMED` within `10 s`, each affected
flow may lose at most `1 s` of traffic (`5,000` frames at `5,000 fps`), and the
BGP-only flows must lose no traffic.

TE-1.23 complements TE-1.21 (gRIBI tunnel fallback to native routing, where
existing gRIBI entries are switched to a decapsulate-and-fallback next hop) and
TE-1.22 (strict FIB precedence when a gRIBI next hop fails, using more-specific
gRIBI prefixes). It covers what neither does: exact-prefix gRIBI-over-eBGP
precedence with an explicit route-preference precondition, encapsulated ECMP
tunnels, a BGP-only control group, BGP churn of the overridden prefixes, gRIBI
`DELETE` reversion, and `gRIBI.Get` verification.

## Testbed type

* [`TESTBED_DUT_ATE_4LINKS`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

```text
                         +----------------+      eBGP (ASN 65537)
  [ ATE port1 ] -------- |                | ---- [ ATE port2 ] (Native BGP Path)
  (Traffic Src)          |      DUT       |
                         |    (65536)     | ---- [ ATE port3 ] (gRIBI Tunnel Path 1)
                         |   loopback0    |      eBGP (ASN 65538)
                         | 198.51.100.1/32| ---- [ ATE port4 ] (gRIBI Tunnel Path 2)
                         +----------------+      eBGP (ASN 65539)
```

* Throughout this test, `DEFAULT` denotes the device's default network instance
  (type `DEFAULT_INSTANCE`), whatever name the platform uses for it. The same
  name is used in OC paths, in the gRIBI `network_instance` fields (including
  those of `NH#10` and `NH#11`) and in the `gRIBI.Get` and `gRIBI.Flush`
  requests.
* Connect 4 interfaces between the DUT and ATE and configure dual-stack IPv4 and
  IPv6 addresses. All DUT interfaces, including `loopback0`, are in the
  `DEFAULT` network instance. All four links run at the same speed of at least
  `10 Gbps`; the total offered test load (`20,000 fps` of `512`-byte frames,
  about `85 Mbps` including preamble and inter-frame gap) is below `1%` of the
  link rate, so no congestion loss is expected on any port. Each ATE port uses a
  MAC address whose last octet is unique, which the traffic measurement relies
  on.
  * **Link 1 (Traffic Ingress)**: Connect DUT `port1` to ATE `port1`.
    * DUT `port1`: IPv4 `192.0.2.1/30`, IPv6 `2001:db8:1::1/126`
    * ATE `port1`: IPv4 `192.0.2.2/30`, IPv6 `2001:db8:1::2/126`, MAC
      `02:00:01:01:01:01`
  * **Link 2 (Native BGP Path)**: Connect DUT `port2` to ATE `port2`.
    * DUT `port2`: IPv4 `192.0.2.5/30`, IPv6 `2001:db8:2::1/126`
    * ATE `port2`: IPv4 `192.0.2.6/30`, IPv6 `2001:db8:2::2/126`, MAC
      `02:00:02:01:01:02`
  * **Link 3 (Primary Tunnel Egress Path)**: Connect DUT `port3` to ATE `port3`.
    * DUT `port3`: IPv4 `192.0.2.9/30`, IPv6 `2001:db8:3::1/126`
    * ATE `port3`: IPv4 `192.0.2.10/30`, IPv6 `2001:db8:3::2/126`, MAC
      `02:00:03:01:01:03`
  * **Link 4 (Secondary Tunnel Egress Path)**: Connect DUT `port4` to ATE
    `port4`.
    * DUT `port4`: IPv4 `192.0.2.13/30`, IPv6 `2001:db8:4::1/126`
    * ATE `port4`: IPv4 `192.0.2.14/30`, IPv6 `2001:db8:4::2/126`, MAC
      `02:00:04:01:01:04`
  * **Loopback (Tunnel Source)**: Configure DUT `loopback0` with IPv4
    `198.51.100.1/32`. It is the outer source address of the gRIBI tunnel next
    hops and does not overlap the BGP-advertised tunnel endpoints
    `198.51.100.10/32` and `198.51.100.20/32`, the Group A/B prefixes (within
    `100.64.0.0/14`) or the link subnets (within `192.0.2.0/28`).
* Apply the DUT configuration shown in [Canonical OC](#canonical-oc) with
  `gNMI.Set` and verify all 4 DUT ports and `loopback0` report `UP` using
  `gNMI.Subscribe` (`ON_CHANGE`) on `/interfaces/interface/state/oper-status`.
* Configure eBGP on the DUT (part of the same Canonical OC configuration:
  `ASN 65536`, router-id `192.0.2.1`) with the permissive routing policy
  `ACCEPT_ALL` (`policy-result = ACCEPT_ROUTE`) as import and export policy for
  IPv4-Unicast (IPv4 neighbors) and IPv6-Unicast (IPv6 neighbors):
  * Peer with ATE `port2` (`ASN 65537`, `192.0.2.6` / `2001:db8:2::2`).
  * Peer with ATE `port3` (`ASN 65538`, `192.0.2.10` / `2001:db8:3::2`).
  * Peer with ATE `port4` (`ASN 65539`, `192.0.2.14` / `2001:db8:4::2`).
* **Route-preference precondition**: the DUT must prefer a gRIBI entry over an
  eBGP route for the same prefix. Because the gRIBI route preference is
  device-local (gRIBI specification section 4.1.8), the relationship is
  established explicitly (a lower value is more preferred):
  * Set the eBGP administrative distance to `200` at
    `/network-instances/network-instance/protocols/protocol/bgp/global/default-route-distance/config/external-route-distance`
    and verify that the corresponding `state` leaf reports `200`. On platforms
    that do not support this OC leaf, configure the same eBGP distance with the
    platform's native configuration and skip the `state` check.
  * OpenConfig defines no gRIBI route-preference leaf. On platforms whose
    default gRIBI route preference is not more preferred than `200`, configure
    the gRIBI route preference natively so that it is.
  * The precondition is verified, not assumed: TE-1.23.2 fails if any Group A
    `IPv4Entry` or `IPv6Entry` is acknowledged only with `RIB_PROGRAMMED` (gRIBI
    specification section 4.1.3.2: installed in the RIB but not the preferred
    route), if any Group A AFT entry does not report `origin-protocol = GRIBI`,
    or if any Group A frame egresses ATE `port2` in the post-`ADD` steady state.
* **ECMP hashing precondition**: the DUT's ECMP hash for IPv4 and IPv6 traffic
  must include the L4 (UDP) source and destination ports in addition to the IP
  addresses and protocol (5-tuple hashing, the default on most platforms). On
  platforms where this is not the default, enable L4-port hashing with the
  platform's native configuration before TE-1.23.2. The `±5%` ECMP split
  criteria in TE-1.23.2 and TE-1.23.3 rely on this entropy.
* Start OTG protocols and explicitly resolve IPv4 ARP and IPv6 Neighbor
  Discovery (ND) across all 4 ports before starting any traffic flows.
* Verify all six IPv4 and IPv6 eBGP sessions reach `ESTABLISHED` via
  `gNMI.Subscribe` (`ON_CHANGE`) on
  `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state`.
* Advertise the following prefixes via eBGP. Each group is a separately named
  OTG route range so that it can be withdrawn and re-advertised without
  resetting the BGP session:
  * **Group A (Override Target Prefixes — advertised by ATE `port2`)**:
    * `500` IPv4 `/24` prefixes: `100.64.0.0/24` through `100.65.243.0/24`
      (step `0.0.1.0`).
    * `500` IPv6 `/64` prefixes: `2001:db8:1000::/64` through
      `2001:db8:11f3::/64` (third hextet incremented by 1 per prefix).
  * **Group B (BGP-Only Control Prefixes — advertised by ATE `port2`)**:
    * `500` IPv4 `/24` prefixes: `100.66.0.0/24` through `100.67.243.0/24`
      (step `0.0.1.0`).
    * `500` IPv6 `/64` prefixes: `2001:db8:2000::/64` through
      `2001:db8:21f3::/64` (third hextet incremented by 1 per prefix).
  * **Tunnel Endpoint Underlay Prefixes**:
    * Advertise tunnel endpoint `198.51.100.10/32` from ATE `port3` (`ASN
      65538`).
    * Advertise tunnel endpoint `198.51.100.20/32` from ATE `port4` (`ASN
      65539`).
* Establish a single `gRIBI` `Modify` stream to the DUT that is used for the
  whole test, with session parameters `redundancy = SINGLE_PRIMARY`,
  `persistence = PRESERVE` and `ack_type = RIB_AND_FIB_ACK`. Elect the client as
  leader: send a `ModifyRequest` containing only
  `election_id = {high: 0, low: 1}` to learn the highest election ID known to
  the DUT from `ModifyResponse.election_id` (gRIBI specification section
  4.1.2.1), then send that value incremented by 1 and use it for all subsequent
  operations. Execute `gRIBI.Flush` on the `DEFAULT` network instance with this
  election ID.
* **Traffic profile** (used by every subtest): from ATE `port1`, define exactly
  4 continuous OTG flows. Each flow sends `512`-byte frames at `5,000 fps`
  (`20,000 fps` in total) with inner IPv4 `TTL = 64` / IPv6 `Hop Limit = 64`
  and `DSCP = 10`, carries a UDP header whose source port increments from
  `49152` over `10,000` values (`49152`–`59151`) with fixed destination port
  `50000`, and has receive ports `port2`, `port3` and `port4`:
  * `Flow-GroupA-v4`: source `192.0.2.2`, destinations `100.64.0.1` through
    `100.65.243.1` (`500` addresses, step `0.0.1.0`).
  * `Flow-GroupA-v6`: source `2001:db8:1::2`, destinations `2001:db8:1000::1`
    through `2001:db8:11f3::1` (`500` addresses, third hextet incremented by 1).
  * `Flow-GroupB-v4`: source `192.0.2.2`, destinations `100.66.0.1` through
    `100.67.243.1` (`500` addresses, step `0.0.1.0`).
  * `Flow-GroupB-v6`: source `2001:db8:1::2`, destinations `2001:db8:2000::1`
    through `2001:db8:21f3::1` (`500` addresses, third hextet incremented by 1).
  * The destination address and the UDP source port advance together on every
    frame, so each flow carries `lcm(500, 10,000) = 10,000` distinct 5-tuples
    (ECMP hash keys) with an equal number of frames each.
* **Measurement definitions** (all OTG counters are read after traffic has been
  stopped and the transmit state of every flow reports stopped):
  * `tx` and `rx`: per-flow transmitted and received frames from OTG flow
    metrics, where `rx` is summed over `port2`, `port3` and `port4`; the number
    of lost frames of a flow is `tx - rx`.
  * `rx(portN)`: per-flow received frames attributed to ATE `portN` with OTG
    egress tracking: a metric tag on the last octet of the received Ethernet
    destination MAC address (bit offset `40`, length `8`), which is `02`, `03`
    or `04` for ATE `port2`, `port3` or `port4`. Control-plane frames (BGP, ARP,
    ND) are not flow frames and are therefore never counted.
  * Steady-state window: start all 4 flows, run them for `75 s`, then stop.
  * Transition run: start all 4 flows, wait `10 s`, perform the gRIBI action,
    wait until the AFT convergence condition of the subtest is met, wait another
    `10 s`, then stop.
  * `T_prog`: time from sending the first `ModifyRequest` of a batch to
    receiving the last `FIB_PROGRAMMED` `AFTResult` of that batch.
  * Data-plane convergence time of a flow: its lost frames divided by its rate
    (`5,000 fps`).
* **Stability monitoring** (evaluated in TE-1.23.5): when setup is complete,
  open a `gNMI.Subscribe` (`ON_CHANGE`) subscription on `session-state` of all
  six BGP neighbors that stays open until the end of TE-1.23.4, and record the
  baseline snapshot `S0` with `gNMI.Get`: `established-transitions` of all six
  neighbors, `/system/state/boot-time`, `/components/component/state/boot-time`
  of every `CHASSIS`, `CONTROLLER_CARD` and `LINECARD` component that reports
  it, and `name`, `pid` and `start-time` of the monitored control-plane daemons
  under `/system/processes/process`. The monitored daemons are the long-running
  processes that implement BGP/routing, the gRIBI server, the gNMI/gRPC server
  and the FIB/forwarding agent. They are identified by a per-platform list of
  process names supplied by the test implementation; every listed name must
  match at least one process in `S0`. Other processes (shells, CLI or SSH
  sessions, helpers, periodic jobs) are not monitored.

### TE-1.23.1 - Baseline Native BGP Forwarding Validation

* **Step 1 - Verify BGP AFT Installation**:
  * Using `gNMI.Subscribe` (`ON_CHANGE`), confirm all `1,000` IPv4 prefixes
    (Groups A and B) and `1,000` IPv6 prefixes (Groups A and B) are installed in
    the `DEFAULT` AFT with `origin-protocol` set to `BGP`.
  * Confirm the tunnel endpoints `198.51.100.10/32` and `198.51.100.20/32` are
    present in the `DEFAULT` AFT with `origin-protocol = BGP` before any gRIBI
    programming.
  * Confirm via `gNMI.Subscribe` that the ATE `port2` neighbors (`192.0.2.6` for
    IPv4-Unicast, `2001:db8:2::2` for IPv6-Unicast) each report
    `prefixes/received = 1,000` and `prefixes/installed = 1,000`.
  * Confirm via `gRIBI.Get` (`aft = ALL`) on `DEFAULT` that zero gRIBI entries
    are currently present.
* **Step 2 - Run Baseline Traffic**:
  * Run one steady-state window with all 4 flows. While it runs, subscribe with
    `gNMI.Subscribe` (`SAMPLE`, `sample_interval = 10 s`) to `in-pkts` of DUT
    `port1` and `out-pkts` of DUT `port2`.
* **Step 3 - Pass/Fail Criteria**:
  * For each of the 4 flows: `0` lost frames, `rx(port2) = rx` (`100%` on the
    native BGP path) and `rx(port3) = rx(port4) = 0`.
  * Consecutive `SAMPLE` notifications of each counter arrive every `10 s`
    (each gap between `8 s` and `12 s`).
  * For each counter, the rate computed between the first notification at least
    `5 s` after traffic start and the last notification before traffic stop
    (counter delta divided by notification timestamp delta) is `20,000 pps`
    within `±5%`. With the `75 s` window and `8`–`12 s` notification gaps, the
    first of these notifications arrives before `17 s` and the last after
    `63 s`, so the timestamp delta exceeds `46 s`; the tolerance absorbs a
    counter refresh granularity of up to `2 s` at each end (`2 s / 46 s` is
    about `4.3%`).

### TE-1.23.2 - gRIBI Tunnel Injection (ADD) Overriding Active BGP Routes

* **Step 1 - Inject gRIBI Encapsulated Tunnel Routes for Group A**:
  * Start a transition run. After the initial `10 s`, send the following `ADD`
    operations for network instance `DEFAULT` on the session's `Modify` stream,
    in the order NH, NHG, IPv4/IPv6 entries, without waiting for
    acknowledgements between objects. gRIBI IP-in-IP next hops carry only the
    outer source and destination addresses (`ip_in_ip` has no TTL or DSCP
    field), so no outer TTL or DSCP is programmed.

    ```text
    NH#10 -> {
      encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
      ip_in_ip {
        src_ip: "198.51.100.1"     // DUT loopback0
        dst_ip: "198.51.100.10"    // resolves via eBGP to ATE port3
      }
      network_instance: "DEFAULT"  // post-encapsulation lookup
    }
    NH#11 -> {
      encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
      ip_in_ip {
        src_ip: "198.51.100.1"     // DUT loopback0
        dst_ip: "198.51.100.20"    // resolves via eBGP to ATE port4
      }
      network_instance: "DEFAULT"  // post-encapsulation lookup
    }
    NHG#100 -> {
      {NH#10, weight: 1},
      {NH#11, weight: 1},
    }
    IPv4Entry {100.64.0.0/24 .. 100.65.243.0/24 (DEFAULT)} -> NHG#100      // 500 entries
    IPv6Entry {2001:db8:1000::/64 .. 2001:db8:11f3::/64 (DEFAULT)} -> NHG#100  // 500 entries
    ```

* **Step 2 - Acknowledgement, Precedence and Telemetry Validation**:
  * Await the results once, after the complete NH, NHG and IPv4/IPv6 sequence
    has been sent (some platforms defer the NH/NHG acknowledgement until the
    objects are referenced by an entry). Every one of the `1,003` operations
    must return `FIB_PROGRAMMED` and `T_prog` must be `<= 10 s`. Any `FAILED` or
    `FIB_FAILED` result fails the test. A Group A entry that returns
    `RIB_PROGRAMMED` without `FIB_PROGRAMMED` within `10 s` fails the test as a
    route-preference precondition violation.
  * Verify via `gNMI.Subscribe` (`ON_CHANGE`) on
    `/network-instances/network-instance/afts/` that, within `30 s` of the last
    `FIB_PROGRAMMED` result (the AFT convergence condition of this subtest):
    * All `500` Group A IPv4 and `500` Group A IPv6 entries report
      `origin-protocol = GRIBI` and a `next-hop-group` that identifies an AFT
      next-hop-group (`next-hop-groups/next-hop-group/state/id`) reporting
      `state/programmed-id = 100`. The AFT `state/id` is device-assigned and is
      not required to equal the gRIBI ID.
    * That next-hop-group has exactly 2 members, each with `weight = 1`, whose
      AFT next hops report `state/programmed-index` `10` and `11`. The next hop
      with `programmed-index` `10` reports `ip-in-ip/state/src-ip` =
      `198.51.100.1` and `ip-in-ip/state/dst-ip` = `198.51.100.10`; the next hop
      with `programmed-index` `11` reports `src-ip` = `198.51.100.1` and
      `dst-ip` = `198.51.100.20`. Both report `state/encapsulate-header = IPV4`;
      on platforms that do not report `encapsulate-header` in AFT telemetry,
      this leaf is not checked (the `ip-in-ip` leaves and the packet capture
      still verify the encapsulation).
    * All `500` Group B IPv4 and `500` Group B IPv6 entries and both tunnel
      endpoint `/32` entries continue to report `origin-protocol = BGP`.
  * Verify via `gNMI.Subscribe` that, within `30 s` of the last `FIB_PROGRAMMED`
    result, the ATE `port2` neighbors still report `prefixes/received = 1,000`
    per AFI-SAFI (the BGP routes are not withdrawn) and now report
    `prefixes/installed = 500` (openconfig-bgp counts only routes selected for
    forwarding after administrative distance is applied; the Group A BGP paths
    are no longer selected).
  * Issue `gRIBI.Get` (`aft = ALL`) on `DEFAULT` and verify it returns exactly
    `1,003` entries (`500` `IPv4Entry`, `500` `IPv6Entry`, `NextHopGroup 100`,
    `NextHops 10, 11`), each with `rib_status = PROGRAMMED` and
    `fib_status = PROGRAMMED`, and with attributes identical to those programmed
    (every entry points to `NextHopGroup 100`; `NextHopGroup 100` has members
    `10` and `11` with `weight = 1`; `NextHops 10, 11` have
    `encapsulate_header = IPV4`, the `ip_in_ip` `src_ip`/`dst_ip` shown above
    and `network_instance = DEFAULT`).
* **Step 3 - Transition Pass/Fail Criteria** (the transition run ends `10 s`
  after the AFT convergence condition is met):
  * `Flow-GroupA-v4` and `Flow-GroupA-v6` each lose at most `5,000` frames
    (data-plane convergence time `<= 1 s`).
  * `Flow-GroupB-v4` and `Flow-GroupB-v6` each lose `0` frames and have
    `rx(port2) = rx`.
* **Step 4 - Steady-State Data-Plane and Encapsulation Validation**:
  * Start packet capture on ATE `port3` and `port4`, run one steady-state
    window, and stop the capture after the first `5 s` of traffic.
  * `Flow-GroupA-v4` and `Flow-GroupA-v6`, each: `0` lost frames,
    `rx(port2) = 0`, `rx(port3) + rx(port4) = rx`, and a `port3` share
    `rx(port3) / rx` between `45%` and `55%` (`50% / 50%` split, `±5%`).
    Rationale: with an ideal hash, each of the `10,000` distinct 5-tuples of a
    flow is mapped independently to one of the two equal-weight next hops, so
    the share is binomial with `σ = 0.5 / sqrt(10,000) = 0.5%`; `±5%` is `10σ`
    (false-failure probability below `1e-20`) and leaves margin for non-ideal
    hash functions. With the `500` destination addresses as the only entropy,
    `σ` would be `2.24%` and `±5%` only `2.2σ` (about `2.5%` false-failure
    probability per flow), which is why the UDP source port entropy is
    required.
  * `Flow-GroupB-v4` and `Flow-GroupB-v6`, each: `0` lost frames and
    `rx(port2) = rx`.
  * Every captured IP-in-IP frame (outer IPv4 protocol `4` or `41`) on ATE
    `port3` has outer source `198.51.100.1` and outer destination
    `198.51.100.10`, and every such frame on ATE `port4` has outer source
    `198.51.100.1` and outer destination `198.51.100.20`. The outer protocol is
    `4` for an inner IPv4 packet and `41` for an inner IPv6 packet, the inner
    destination address lies in the Group A range of its address family, and
    the inner `DSCP = 10`. Each port's capture contains at least `1,000` such
    frames and no unencapsulated IPv4 or IPv6 frame destined to a Group A
    address.
  * Outer TTL and outer DSCP are derived by the platform's encapsulation model
    (they cannot be programmed through a gRIBI IP-in-IP next hop) and are not
    asserted by this test; TTL and DSCP propagation across encapsulation is
    covered by TE-16.1.

### TE-1.23.3 - BGP Underlay Churn Under Active gRIBI Override

* **Step 1 - Withdraw Overridden BGP Routes on Link 2**:
  * Start all 4 flows. After `10 s`, withdraw the Group A IPv4 and Group A IPv6
    route ranges on ATE `port2` (OTG route withdraw; the BGP sessions stay up).
  * Confirm via `gNMI.Subscribe` that, within `30 s`, `prefixes/received` of the
    ATE `port2` neighbors decreases from `1,000` to `500` per AFI-SAFI while
    `prefixes/installed` stays `500`, and that all `1,000` Group A AFT entries
    still report `origin-protocol = GRIBI` with a next-hop-group whose
    `programmed-id = 100`.
* **Step 2 - Re-advertise Overridden BGP Routes on Link 2**:
  * Re-advertise the Group A IPv4 and Group A IPv6 route ranges on ATE `port2`.
  * Confirm that, within `30 s`, `prefixes/received` returns to `1,000` per
    AFI-SAFI while `prefixes/installed` stays `500` (the re-learned BGP paths do
    not displace the gRIBI entries), and that the Group A AFT entries are
    unchanged (`origin-protocol = GRIBI`, `programmed-id = 100`).
  * Wait `10 s`, then stop traffic.
* **Step 3 - Pass/Fail Criteria** (over the whole run, which spans the BGP
  withdrawal and re-advertisement):
  * `Flow-GroupA-v4` and `Flow-GroupA-v6`, each: `0` lost frames,
    `rx(port2) = 0`, and a `port3` share `rx(port3) / rx` between `45%` and
    `55%`, proving complete FIB isolation of the active gRIBI override.
  * `Flow-GroupB-v4` and `Flow-GroupB-v6`, each: `0` lost frames and
    `rx(port2) = rx`.

### TE-1.23.4 - gRIBI Tunnel Deletion (DELETE) and Fallback to Active BGP

* **Step 1 - Delete gRIBI Routes for Group A**:
  * Start a transition run. After the initial `10 s`, send `DELETE` operations
    on the same `Modify` stream for all `500` Group A `IPv4Entry` and `500`
    Group A `IPv6Entry` routes. Only after all `1,000` have returned
    `FIB_PROGRAMMED`, delete `NextHopGroup 100`; only after it has returned
    `FIB_PROGRAMMED`, delete `NextHops 10, 11`, so that no object is deleted
    while it is still referenced.
* **Step 2 - Telemetry and Forwarding Reversion Validation**:
  * Every `DELETE` operation returns `FIB_PROGRAMMED`, and `T_prog` of the
    `1,000` IP-entry deletions is `<= 10 s`. Any `FAILED` or `FIB_FAILED` result
    fails the test.
  * Verify via `gRIBI.Get` (`aft = ALL`) that zero gRIBI entries remain in
    `DEFAULT`.
  * Verify via `gNMI.Subscribe` (`ON_CHANGE`) on AFT state paths that, within
    `30 s` of the last IP-entry `FIB_PROGRAMMED` result (the AFT convergence
    condition of this subtest), all `500` Group A IPv4 and `500` Group A IPv6
    prefixes are present in the AFT with `origin-protocol = BGP`, and that all
    Group B and tunnel endpoint entries still report `origin-protocol = BGP`.
  * Verify via `gNMI.Subscribe` that, within `30 s` of the last IP-entry
    `FIB_PROGRAMMED` result, the ATE `port2` neighbors again report
    `prefixes/received = 1,000` and `prefixes/installed = 1,000` per AFI-SAFI.
* **Step 3 - Transition Pass/Fail Criteria** (the transition run ends `10 s`
  after the AFT convergence condition is met):
  * `Flow-GroupA-v4` and `Flow-GroupA-v6` each lose at most `5,000` frames
    (data-plane convergence time `<= 1 s`).
  * `Flow-GroupB-v4` and `Flow-GroupB-v6` each lose `0` frames and have
    `rx(port2) = rx`.
* **Step 4 - Steady-State Pass/Fail Criteria**:
  * Run one steady-state window. For each of the 4 flows: `0` lost frames,
    `rx(port2) = rx` and `rx(port3) = rx(port4) = 0` (Group A traffic is back on
    the native BGP path).

### TE-1.23.5 - Switch Stability Across TE-1.23.1 to TE-1.23.4

* **Step 1 - Collect**: at the end of TE-1.23.4, take snapshot `S1` with the
  same content as `S0`.
* **Step 2 - Pass/Fail Criteria**:
  * The `session-state` subscription recorded no value other than `ESTABLISHED`
    for any of the six BGP neighbors, and the `established-transitions` of every
    neighbor in `S1` equals its value in `S0` (neither the TE-1.23.3 route
    withdrawals nor the gRIBI operations reset a session).
  * The gRIBI `Modify` stream opened during setup is still connected (no RPC
    error and no end of stream), and no `AFTResult` with status `FAILED` or
    `FIB_FAILED` was received during the test.
  * `/system/state/boot-time` and `/components/component/state/boot-time` of
    every `CHASSIS`, `CONTROLLER_CARD` and `LINECARD` component present in both
    snapshots are unchanged (no reboot).
  * No monitored control-plane daemon restarted: every process in `S0` whose
    `name` is in the per-platform monitored list is present in `S1` with the
    same `pid` and the same `start-time`.

### Cleanup

* Stop all OTG traffic flows, packet captures and protocol sessions.
* Execute `gRIBI.Flush` on network instance `DEFAULT` with the session's
  election ID to ensure zero residual gRIBI AFT state, then close the gRIBI
  session.
* Revert the DUT interface (including `loopback0`), routing-policy and BGP
  configurations (including the eBGP route distance), and any native gRIBI
  route-preference configuration applied for the precondition, to the baseline
  pre-test state.

## Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "Traffic Ingress Port 1",
          "enabled": true,
          "name": "port1",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port1",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
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
                        "ip": "2001:db8:1::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8:1::1"
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
          "description": "Native BGP Egress Port 2",
          "enabled": true,
          "name": "port2",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port2",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
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
                        "ip": "2001:db8:2::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8:2::1"
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
          "description": "Primary Tunnel Egress Port 3",
          "enabled": true,
          "name": "port3",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port3",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
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
                        "ip": "2001:db8:3::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8:3::1"
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
          "description": "Secondary Tunnel Egress Port 4",
          "enabled": true,
          "name": "port4",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port4",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
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
                        "ip": "2001:db8:4::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8:4::1"
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
          "description": "gRIBI Tunnel Source Loopback",
          "enabled": true,
          "name": "loopback0",
          "type": "iana-if-type:softwareLoopback"
        },
        "name": "loopback0",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "198.51.100.1",
                        "prefix-length": 32
                      },
                      "ip": "198.51.100.1"
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
          "name": "DEFAULT",
          "type": "openconfig-network-instance-types:DEFAULT_INSTANCE"
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
                    "as": 65536,
                    "router-id": "192.0.2.1"
                  },
                  "default-route-distance": {
                    "config": {
                      "external-route-distance": 200
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
                                  "ACCEPT_ALL"
                                ],
                                "import-policy": [
                                  "ACCEPT_ALL"
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
                        "peer-as": 65537
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
                                  "ACCEPT_ALL"
                                ],
                                "import-policy": [
                                  "ACCEPT_ALL"
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
                        "neighbor-address": "2001:db8:2::2",
                        "peer-as": 65537
                      },
                      "neighbor-address": "2001:db8:2::2"
                    },
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ACCEPT_ALL"
                                ],
                                "import-policy": [
                                  "ACCEPT_ALL"
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
                        "peer-as": 65538
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
                                  "ACCEPT_ALL"
                                ],
                                "import-policy": [
                                  "ACCEPT_ALL"
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
                        "neighbor-address": "2001:db8:3::2",
                        "peer-as": 65538
                      },
                      "neighbor-address": "2001:db8:3::2"
                    },
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ACCEPT_ALL"
                                ],
                                "import-policy": [
                                  "ACCEPT_ALL"
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
                        "neighbor-address": "192.0.2.14",
                        "peer-as": 65539
                      },
                      "neighbor-address": "192.0.2.14"
                    },
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ACCEPT_ALL"
                                ],
                                "import-policy": [
                                  "ACCEPT_ALL"
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
                        "neighbor-address": "2001:db8:4::2",
                        "peer-as": 65539
                      },
                      "neighbor-address": "2001:db8:4::2"
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
            "name": "ACCEPT_ALL"
          },
          "name": "ACCEPT_ALL",
          "statements": {
            "statement": [
              {
                "actions": {
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                },
                "config": {
                  "name": "accept_all_stmt"
                },
                "name": "accept_all_stmt"
              }
            ]
          }
        }
      ]
    }
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /components/component/state/boot-time:
    platform_type: ["CHASSIS", "CONTROLLER_CARD", "LINECARD"]
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/name:
  /interfaces/interface/config/type:
  /interfaces/interface/state/counters/in-pkts:
  /interfaces/interface/state/counters/out-pkts:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/origin-protocol:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/origin-protocol:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/weight:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/programmed-id:
  /network-instances/network-instance/afts/next-hops/next-hop/ip-in-ip/state/dst-ip:
  /network-instances/network-instance/afts/next-hops/next-hop/ip-in-ip/state/src-ip:
  /network-instances/network-instance/afts/next-hops/next-hop/state/encapsulate-header:
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/programmed-index:
  /network-instances/network-instance/config/name:
  /network-instances/network-instance/config/type:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/as:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id:
  /network-instances/network-instance/protocols/protocol/bgp/global/default-route-distance/config/external-route-distance:
  /network-instances/network-instance/protocols/protocol/bgp/global/default-route-distance/state/external-route-distance:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/installed:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/neighbor-address:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/established-transitions:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/config/identifier:
  /network-instances/network-instance/protocols/protocol/config/name:
  /routing-policy/policy-definitions/policy-definition/config/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result:
  /routing-policy/policy-definitions/policy-definition/statements/statement/config/name:
  /system/processes/process/state/name:
  /system/processes/process/state/pid:
  /system/processes/process/state/start-time:
  /system/state/boot-time:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
      on_change: true
      sample: true
  gribi:
    gRIBI.Flush:
    gRIBI.Get:
    gRIBI.Modify:
```

## Required DUT platform

* FFF
