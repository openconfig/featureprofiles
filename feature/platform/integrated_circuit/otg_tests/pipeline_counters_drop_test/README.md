# gNMI-1.29: Pipeline Counters Drops Test

## Summary

Verify that NPU (integrated-circuit) pipeline drop counters are reported
correctly via telemetry and correctly increment under conditions that induce the
respective drops.

The test covers both the vendor-neutral OpenConfig drop aggregates under
`/components/component/integrated-circuit/pipeline-counters/drop/state` and the
vendor-defined drop leaves under
`/components/component/integrated-circuit/pipeline-counters/drop/vendor`. It
verifies that:

* Every vendor publishes a non-empty, continuously streamed set of drop leaves.
* Forwarded traffic never increments drop counters.
* Packets dropped by legitimate forwarding decisions (discard routes, missing
  routes, unresolved next hops after an edge link is cut) are counted
  **exactly** by at least one vendor drop leaf, the incremented leaves are
  logged, and the drops are never classified as `adverse` or `congestion`.
* Drop accounting is exact for IPv4 and IPv6, across multiple ingress ports and
  integrated circuits, across multiple discard prefixes, and across repeated
  iterations (no clear-on-read, reset or double counting).

## Testbed type

* [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

#### Port roles

| Testbed port ID         | Role                                                |
| ----------------------- | --------------------------------------------------- |
| `dut:port1`-`ate:port1` | Ingress A                                           |
| `dut:port2`-`ate:port2` | Ingress B                                           |
| `dut:port3`-`ate:port3` | Ingress C                                           |
| `dut:port4`-`ate:port4` | Egress, and the edge link that is cut in 1.29.8     |

DUT interface names are vendor specific. The test resolves each testbed port ID
to its DUT interface name at runtime. Subtest gNMI-1.29.2 sends traffic from
`ate:port1` towards `ate:port2`.

#### Interface addressing

| Port  | DUT IPv4        | ATE IPv4        | DUT IPv6          | ATE IPv6          |
| ----- | --------------- | --------------- | ----------------- | ----------------- |
| port1 | `192.0.2.1/30`  | `192.0.2.2/30`  | `2001:db8::1/126` | `2001:db8::2/126` |
| port2 | `192.0.2.5/30`  | `192.0.2.6/30`  | `2001:db8::5/126` | `2001:db8::6/126` |
| port3 | `192.0.2.9/30`  | `192.0.2.10/30` | `2001:db8::9/126` | `2001:db8::a/126` |
| port4 | `192.0.2.13/30` | `192.0.2.14/30` | `2001:db8::d/126` | `2001:db8::e/126` |

* The ATE ports use the MAC addresses `02:00:01:01:01:01` (port1),
  `02:00:01:01:01:02` (port2), `02:00:01:01:01:03` (port3) and
  `02:00:01:01:01:04` (port4).
* Configure all 4 DUT ports with a description, type `ethernetCsmacd`,
  `enabled` = `true`, L2 MTU `1514`, and IPv4/IPv6 MTU `1500` on subinterface
  `0` (subject to the standard interface deviations, for example
  `interface_enabled` and `omit_l2_mtu`).
* Place all 4 DUT ports in the default network instance (`DEFAULT`). Static
  routes use the static protocol with identifier `STATIC` and name `DEFAULT`.
  Configure the default network instance with its name and type
  `DEFAULT_INSTANCE` before any protocol is configured.
* No configuration is needed to enable the drop counters; the test only
  subscribes to the
  `/components/component/integrated-circuit/pipeline-counters/drop/vendor` path.

#### Prefix plan

| Purpose                     | IPv4                            | IPv6                                          | Next hop                      |
| --------------------------- | ------------------------------- | --------------------------------------------- | ----------------------------- |
| Legacy discard (1.29.2)     | `203.0.113.2/32`                | n/a                                           | `DROP`                        |
| Forwarded control (1.29.5)  | `203.0.113.128/25`              | `2001:db8:200::/64`                           | `192.0.2.14` / `2001:db8::e`  |
| Single discard (1.29.6)     | `198.51.100.0/24`               | `2001:db8:100::/64`                           | `DROP`                        |
| Scale discard (1.29.7)      | `198.18.0.0/24`-`198.18.15.0/24` | `2001:db8:100:1::/64`-`2001:db8:100:10::/64` | `DROP`                        |
| Edge cut (1.29.8)           | `100.64.0.0/24`                 | `2001:db8:300::/64`                           | `192.0.2.14` / `2001:db8::e`  |
| Counter stability (1.29.9)  | `192.0.2.128/25`                | n/a                                           | `DROP`                        |

No default route (`0.0.0.0/0` or `::/0`) may be present on the DUT during the
test, so that a destination with no matching prefix is dropped as no-route.
A default entry whose next hops do not forward traffic — either having neither
an IP address nor an interface (for example an implicit default reject route),
or an entry with unset `origin-protocol` pointing to `Null0` / `discard` with
an unspecified IP (`0.0.0.0` or `::`) — drops the traffic just like having no
route and is only logged.
If a default route is present (for example a management route in the default
network instance), setup reports it as an error, together with its origin
protocol and next hops. The steps that measure no-route drops (gNMI-1.29.8
Step 5, gNMI-1.29.9.2 and gNMI-1.29.9.3) then fail without sending traffic,
because the traffic would be forwarded instead of dropped; all other subtests
still run.

#### Standard flow template

Every ATE flow in this test uses the following packet definition unless a
subtest states otherwise.

| Field          | Value                                                                  |
| -------------- | ---------------------------------------------------------------------- |
| Ethernet dst   | MAC of the DUT ingress port, read from `/interfaces/interface/ethernet/state/mac-address` |
| Ethernet src   | MAC of the ATE ingress port                                            |
| IPv4           | source = ATE ingress IPv4, TTL `64`, DSCP `0`                          |
| IPv6           | source = ATE ingress IPv6, hop limit `64`, traffic class `0`           |
| L4             | UDP, source port `49152`, destination port `49153`                     |
| Frame size     | `512` bytes (fixed)                                                    |
| Rate           | `100` packets per second per flow                                      |
| Packet count   | Fixed count, `1,000` unless a subtest states otherwise                 |
| Metrics        | Flow metrics enabled, received packets tracked on the ATE egress port  |

The low rate and fixed frame size keep every flow far below line rate, so no
packet can be dropped because of congestion.

#### Readiness gates

The test never uses fixed sleeps. Each of the following conditions is awaited
with a gNMI `Subscribe` (`gnmi.Watch`) and a timeout of 120 seconds. The timeout
applies to the condition as a whole, not to each individual port, neighbor,
component or route. Keyed state paths (neighbors, static routes, AFT entries
and interface counters) are watched for at most 10 seconds at a time and
subscribed again until the condition's deadline, because some devices send only
the initial state of a keyed path and no later updates:

* `/interfaces/interface/state/oper-status` is `UP` on all 4 DUT ports.
* For every ATE IPv4 address,
  `/interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/state/link-layer-address`
  is present on the attached DUT port.
* For every ATE IPv6 address,
  `/interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/neighbor-state`
  is `REACHABLE` or `STALE` on the attached DUT port.
* On the ATE, ARP (IPv4) and neighbor discovery (IPv6) are resolved for every
  address family used by a flow before the flow is started.
* `/components/component/state/oper-status` is `ACTIVE` for every component of
  type `INTEGRATED_CIRCUIT` that reports oper-status. Some platforms also report
  components that do not forward traffic (for example FPGAs) with type
  `INTEGRATED_CIRCUIT` and without oper-status; such components are logged and
  skipped. Components that report `DISABLED` (administratively out of service,
  for example the chips of an empty fabric card slot) are also logged and
  skipped. A component that reports any other oper-status than `ACTIVE` fails
  the check.
* After every static route change, the route is reported by
  `/network-instances/network-instance/protocols/protocol/static-routes/static/state/prefix`
  (after a configuration) and is present (or absent, after deletion) in
  `/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix`
  or
  `/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix`
  with `.../state/origin-protocol` = `STATIC`. When waiting for absence, an
  entry of another origin (a route the test did not configure) counts as
  absent.
* After every flow is started, the OTG flow transmit state is awaited until it
  reports `stopped` before any counter is read. For a forwarded flow, the OTG
  received count is then awaited until it equals the transmitted count (the
  step reports any loss). For a dropped flow, the received count is read after
  the drop counters settled.

#### Counter collection and settling

* **Drop subscription**: one gNMI `Subscribe` request with mode `STREAM`,
  subscription mode `SAMPLE` and a sample interval of 10 seconds, containing
  the following subscriptions:
  * `/components/component[name=*]/integrated-circuit/pipeline-counters/drop/vendor`
    (primary): all vendor drop leaves of every integrated circuit. All pass/fail
    drop checks use these leaves.
  * `/components/component[name=*]/integrated-circuit/pipeline-counters/drop/state`
    and
    `/components/component[name=*]/integrated-circuit/pipeline-counters/drop/lookup-block/state`
    (secondary): the OC aggregates and lookup block counters, used for logging
    and cross-checks when the DUT reports them.

  The subscription is opened by the first subtest that needs it (gNMI-1.29.4
  at the latest) and kept open until the end of the test.
* **Sample**: a sample (round) is complete when every known drop leaf has been
  updated once more or, for devices that resend only counters whose value
  changed, when 1.5 sample intervals have passed and at least one new
  notification with drop counters arrived (leaves that were not resent keep
  their last value). A sample that does not complete within 60 seconds fails
  the subtest.
* **Quiet baseline**: before each traffic step, wait for two consecutive
  samples in which no leaf of any integrated circuit changes (increases,
  decreases, appears or disappears; vendor leaves, OC aggregates and lookup
  block counters). Record that sample as the baseline. If any leaf changes,
  repeat the wait up to 3 times, then fail the subtest as "environment not
  quiescent".
* **Ingress sum**: for a vendor leaf, the sum of its deltas over the
  integrated circuits that host the ingress ports of the step, using the
  ingress map from gNMI-1.29.4. If the ingress map is not available, the sum
  is taken over all integrated circuits. A vendor leaf is identified by its
  path below `.../drop/vendor`, so the same leaf on different integrated
  circuits is summed.
* **Settled final value** for a step that expects `N` drops: await on the drop
  subscription (timeout 120 seconds) until the ingress sum of at least one
  vendor leaf has increased by at least `N` since the baseline. Then take the
  next sample and record it as the final value. Using the next sample, rather
  than the first one that reaches `N`, catches late-arriving over-counting. If
  `N` is reached but the next sample does not complete, the step fails with
  that reason.
* **Settled final value** for a step that expects zero drops: after the flow
  transmit state is `stopped`, record the second sample that follows as the
  final value.
* **Monotonicity**: across the whole test, no drop counter of the drop
  subscription (vendor leaves, OC aggregates and lookup block counters) may
  ever decrease between two samples. Decreases are recorded for every update
  and reported in gNMI-1.29.9.1 and in cleanup; the log-only checks (P3, P7,
  lookup block and control plane counters) only log a decrease.

#### Standard drop validation

Every step that expects `N` dropped packets applies the following checks, where
`delta = final - baseline`:

* **P0 (precondition)**: the ATE flow transmitted exactly `N` packets. If not,
  abort the step as a traffic generator error rather than a DUT failure.
* **P1**: `/interfaces/interface/state/counters/in-unicast-pkts` delta on the
  DUT ingress port is at least `N`, and the
  `/interfaces/interface/state/counters/in-errors` delta is `0`. The
  `in-unicast-pkts` increase is awaited for up to 120 seconds per port,
  subscribing again every 10 seconds (some devices send only the initial
  value of an interface counter). The
  `/interfaces/interface/state/counters/in-discards` delta is logged but is
  not a pass/fail condition, because some platforms also count packets
  dropped by a forwarding decision (discard route, no route) as interface
  discards.
* **P2**: the ATE egress port received `0` packets of the dropped flow, read
  after the drop counters settled.
* **P3 (log only)**: the `drop/state/packet-processing-aggregate` delta,
  summed over the integrated circuits that host the ingress ports, is logged
  and compared with `N`. A mismatch, or a missing aggregate, is logged as a
  warning and does not fail the step.
* **P4**: the ingress sum of at least one vendor-specific leaf under
  `/components/component/integrated-circuit/pipeline-counters/drop/vendor` is
  exactly `N`. The OpenConfig model recommends that such drops are
  exposed under `.../<VENDOR>/<PLATFORM>/packet-processing/state/`; the class
  of the matching leaf is logged, and drops that are counted as `adverse` or
  `congestion` are caught by P5 and P6.
* **P5**: every vendor `.../adverse/state/*` delta is `0`. If the DUT reports
  `drop/state/adverse-aggregate`, its delta must also be `0`.
* **P6**: every vendor `.../congestion/state/*` delta is `0`. If the DUT
  reports `drop/state/congestion-aggregate`, its delta must also be `0`.
* **P7 (log only)**: the sum of all vendor `.../packet-processing/state/*`
  deltas is compared with the `drop/state/packet-processing-aggregate` delta
  for each integrated circuit and logged. A mismatch, or a missing aggregate,
  is logged as a warning and does not fail the step.
* **Log**: every `<integrated circuit, leaf>` with a delta greater than `0` is
  logged with the integrated circuit name, the full leaf path and the delta.

### gNMI-1.29.1 - Packet Processing: L3 Route Lookup Failed

gNMI-1.29.1 to gNMI-1.29.3 apply only to Cisco XR. IOS-XR releases before
25.4.15 expose these leaves through the `ciscoxr_vendordrop_ft` deviation
(functional translator); IOS-XR 25.4.15 and later stream them natively. Each
leaf is matched by a case-insensitive name prefix below
`.../drop/vendor/<VENDOR>/<PLATFORM>/packet-processing/state/`:

| Subtest | Functional translator (`CiscoXR/spitfire`) | Native IOS-XR 25.4.15+ (`<module>:cisco/<asic>`) |
| --- | --- | --- |
| gNMI-1.29.1 L3 route lookup failed | `L3_ROUTE_LOOKUP_FAILED` | `l3_drop_adj_non_inject` |
| gNMI-1.29.2 L3 null adjacency | `L3_NULL_ADJ` (for example `L3_NULL_ADJ(D*)`) | `l3_null_adj` |
| gNMI-1.29.3 MPLS label miss | `MPLS_TE_MIDPOINT_LDP_LABELS_MISS` (for example `MPLS_TE_MIDPOINT_LDP_LABELS_MISS(D*)`) | `mpls_te_midpoint_ldp_labels_miss` |

The subtests run when the functional translator deviation is set or when the
drop subscription streams at least one leaf named in the table. On other
platforms they are skipped and the reason is logged.

* Verify via telemetry that the L3 route lookup failed leaf (see the table) is present and is streamed at the subscribed sample interval (at least 2 samples, no gap longer than twice the sample interval).

### gNMI-1.29.2 - Packet Processing: L3 Null Adjacency

* Configure a static route on the DUT for `203.0.113.2/32` with a single next
  hop (index `0`) set to the `DROP` local-defined next hop (the discard
  interface).
* From ATE port 1, send traffic destined to `203.0.113.2`.
* Verify via telemetry that the L3 null adjacency leaf (see the table in gNMI-1.29.1) increments.
* Delete the static route for `203.0.113.2/32` and await its removal from the
  AFT before the next subtest.

#### Canonical OC

```json
{
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
              "config": {
                "identifier": "oc-pol-types:STATIC",
                "name": "DEFAULT"
              },
              "identifier": "oc-pol-types:STATIC",
              "name": "DEFAULT",
              "static-routes": {
                "static": [
                  {
                    "config": {
                      "prefix": "203.0.113.2/32"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "config": {
                            "index": "0",
                            "next-hop": "DROP"
                          },
                          "index": "0"
                        }
                      ]
                    },
                    "prefix": "203.0.113.2/32"
                  }
                ]
              }
            }
          ]
        }
      }
    ]
  }
}
```

### gNMI-1.29.3 - Packet Processing: MPLS Label Miss

* Verify via telemetry that the MPLS label miss leaf (see the table in gNMI-1.29.1) is present and is streamed at the subscribed sample interval (at least 2 samples, no gap longer than twice the sample interval).

### gNMI-1.29.4 - Vendor Drop Counter Discovery and Baseline

The subtests above validate individual, vendor-specific drop reasons. This
subtest is vendor neutral: it verifies that every DUT, regardless of
`dut.Vendor()`, publishes a non-empty set of drop counter leaves under the
`/components/component/integrated-circuit/pipeline-counters/drop/vendor`
container. These vendor-defined leaves are the only place where an operator can
attribute packet loss inside the forwarding pipeline (for example, traffic that
is black-holed at the network edge after a link cut) to a specific drop reason.
All later subtests build on the subscription, ingress map and baseline created
here.

* Step 1 - Discover the integrated circuits.
  * The integrated circuits are discovered during test environment setup (the
    `INTEGRATED_CIRCUIT` readiness gate), by reading
    `/components/component/state` and selecting every component whose
    `/components/component/state/type` is `INTEGRATED_CIRCUIT`. If none is
    found, setup fails and no subtest runs.
  * Verify again that `/components/component/state/oper-status` is `ACTIVE`
    for each integrated circuit that reports oper-status (see "Readiness
    gates"). Log the names of all discovered integrated circuits, and of those
    that do not report oper-status.
* Step 2 - Map ingress ports to integrated circuits (best effort).
  * For `dut:port1`, `dut:port2` and `dut:port3`, read
    `/interfaces/interface/state/hardware-port` to find the `PORT` component,
    then follow `/components/component/state/parent` upwards until a component
    of type `INTEGRATED_CIRCUIT` is reached.
  * If an integrated circuit is found for every ingress port, record the map
    and use it for per-integrated-circuit checks in all later subtests.
    Otherwise log that the map is not available, and evaluate those checks
    over the sum of all integrated circuits.
* Step 3 - Open the drop subscription described in "Counter collection and
  settling" and collect all updates until the first `sync_response`.
* Step 4 - Validate the per-vendor leaf list.
  * Log `dut.Vendor()` and the complete list of vendor drop leaves received, in
    the form
    `/components/component[name=<IC_NAME>]/integrated-circuit/pipeline-counters/drop/vendor/<VENDOR>/<PLATFORM>/<CLASS>/state/<DROP_REASON>`,
    where `<CLASS>` is `adverse`, `congestion` or `packet-processing`.
  * Verify that the list contains at least one leaf for `dut.Vendor()`: the
    first path element below `.../drop/vendor` (the `<VENDOR>` container) must
    start with the `dut.Vendor()` name, compared case-insensitively (for
    example `CiscoXR` for `CISCO`, `arista` for `ARISTA`).
  * Verify that every received leaf carries a non-negative integer counter
    value (`uint_val`, a non-negative `int_val`, or a JSON integer). A leaf
    that only repeats the key of its list entry (for example
    `.../trap[name=<X>]/name`) is not a counter and is ignored.
  * Leaves that report a rate rather than a count (leaf name `rate`, or
    starting with `rate-` or ending with `-rate`, for example
    `.../trap[name=<X>]/rate-pps`) rise and fall with the traffic. Log them and
    exclude them from all counter checks (streaming, monotonicity and P4 to
    P7).
  * Log whether the OC aggregates `drop/state/packet-processing-aggregate`,
    `drop/state/adverse-aggregate` and `drop/state/congestion-aggregate` are
    present for every integrated circuit. Their absence is a warning, not a
    failure.
* Step 5 - Verify streaming. Every leaf must be received in at least two
  samples at the 10 second sample interval, with no gap longer than twice the
  sample interval, including the time since its last update (a leaf that
  stops streaming fails). On platforms that use a functional translator
  (`ciscoxr_vendordrop_ft`), every notification must also be translated
  without error.
* Step 6 - Take the first quiet baseline and start the monotonicity check.

If this subtest finds no vendor drop leaves, it fails, but the later subtests
still run so that forwarding, routing and ATE checks are reported; their P4
checks then fail as well.

This subtest does not change the DUT configuration, so it has no Canonical OC
block.

#### Pass/Fail criteria

* **Pass**: At least one `INTEGRATED_CIRCUIT` component exists, every one that
  reports oper-status is `ACTIVE`,
  the drop subscription returns at least one vendor counter leaf for
  `dut.Vendor()`, every leaf is streamed at the requested sample interval, a
  quiet baseline is obtained, and the complete leaf list is logged.
* **Fail**: The subscription returns an error, the vendor container is missing
  or empty for `dut.Vendor()`, a leaf is not a non-negative integer counter, a
  leaf is not streamed at the requested interval, the functional translator
  (if used) fails to translate a notification, or no quiet baseline can be
  obtained.

### gNMI-1.29.5 - Forwarded Traffic Does Not Increment Drop Counters

Before measuring drops, this subtest proves the opposite: traffic that the DUT
forwards successfully must not increment any drop counter. Without this check,
a counter that increments for every packet (dropped or not) could pass the
exact-count subtests that follow.

* Step 1 - Generate the DUT configuration.
  * Configure an IPv4 static route for `203.0.113.128/25` with next hop
    `192.0.2.14` (ATE port 4), and an IPv6 static route for
    `2001:db8:200::/64` with next hop `2001:db8::e` (ATE port 4).

#### Canonical OC

```json
{
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
              "config": {
                "identifier": "oc-pol-types:STATIC",
                "name": "DEFAULT"
              },
              "identifier": "oc-pol-types:STATIC",
              "name": "DEFAULT",
              "static-routes": {
                "static": [
                  {
                    "config": {
                      "prefix": "203.0.113.128/25"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "config": {
                            "index": "0",
                            "next-hop": "192.0.2.14"
                          },
                          "index": "0"
                        }
                      ]
                    },
                    "prefix": "203.0.113.128/25"
                  },
                  {
                    "config": {
                      "prefix": "2001:db8:200::/64"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "config": {
                            "index": "0",
                            "next-hop": "2001:db8::e"
                          },
                          "index": "0"
                        }
                      ]
                    },
                    "prefix": "2001:db8:200::/64"
                  }
                ]
              }
            }
          ]
        }
      }
    ]
  }
}
```

* Step 2 - Push the configuration to the DUT using gNMI `Set`.
* Step 3 - Await both routes in the static route state and in the AFT, as
  described in "Readiness gates".
* Step 4 - Take a quiet baseline.
* Step 5 - Send traffic.
  * Flow `v4-ctrl`: ATE port 1 to ATE port 4, IPv4 destination
    `203.0.113.129`, 1,000 packets.
  * Flow `v6-ctrl`: ATE port 1 to ATE port 4, IPv6 destination
    `2001:db8:200::1`, 1,000 packets.
  * Start both flows and await the `stopped` transmit state.
* Step 6 - Record the settled final value (zero-drop case).
* Step 7 - Validation.
  * ATE port 4 received exactly 1,000 packets of each flow (no loss).
  * Every vendor drop leaf delta is `0` on every integrated circuit. Every OC
    aggregate and lookup block counter that the DUT reports also has a delta of
    `0`.
* The control routes stay installed for gNMI-1.29.7 and gNMI-1.29.8.

#### Pass/Fail criteria

* **Pass**: Both flows are received without loss and no drop counter changes.
* **Fail**: Any packet of either flow is lost, or any drop counter increments.

### gNMI-1.29.6 - IPv4 and IPv6 Discard Route Exact Drop Count

This subtest verifies that the drop counters measure packet loss accurately,
not just that they exist. Discard (null) routes make the DUT drop a known
number of IPv4 and IPv6 packets inside its forwarding pipeline. Exactly 1,000
packets are sent per address family, and the counters must increment by
exactly 1,000 each time. An exact count catches both under-counting (for
example, packets lost without being counted) and over-counting (for example,
one packet counted twice). The OpenConfig model classifies drops caused by a
discard route as expected (`packet-processing`) drops, so they must never be
counted as `adverse`. IPv4 and IPv6 often use different lookup and drop paths
in the forwarding pipeline, so each address family is measured separately.

* Step 1 - Generate the DUT configuration.
  * In the default network instance, configure an IPv4 static route for
    `198.51.100.0/24` and an IPv6 static route for `2001:db8:100::/64`, each
    with a single next hop (index `0`) set to the `DROP` local-defined next
    hop.

#### Canonical OC

```json
{
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
              "config": {
                "identifier": "oc-pol-types:STATIC",
                "name": "DEFAULT"
              },
              "identifier": "oc-pol-types:STATIC",
              "name": "DEFAULT",
              "static-routes": {
                "static": [
                  {
                    "config": {
                      "prefix": "198.51.100.0/24"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "config": {
                            "index": "0",
                            "next-hop": "DROP"
                          },
                          "index": "0"
                        }
                      ]
                    },
                    "prefix": "198.51.100.0/24"
                  },
                  {
                    "config": {
                      "prefix": "2001:db8:100::/64"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "config": {
                            "index": "0",
                            "next-hop": "DROP"
                          },
                          "index": "0"
                        }
                      ]
                    },
                    "prefix": "2001:db8:100::/64"
                  }
                ]
              }
            }
          ]
        }
      }
    ]
  }
}
```

* Step 2 - Push the configuration to the DUT using gNMI `Set`.
* Step 3 - Verify the routes are programmed.
  * Await both routes in the static route state, and in the IPv4 and IPv6 AFTs
    with `origin-protocol` = `STATIC`.
  * Verify that no more-specific AFT entry exists for `198.51.100.1/32` or
    `2001:db8:100::1/128`, so the discard routes are the only match for the
    test destinations.
* Step 4 - Measure IPv4 drops.
  * Take a quiet baseline.
  * Send flow `v4-drop`: ATE port 1 to ATE port 2, IPv4 destination
    `198.51.100.1`, 1,000 packets. Await the `stopped` transmit state.
  * Record the settled final value and apply the standard drop validation (P0
    to P7) with `N` = 1,000.
  * Record which vendor leaves have an ingress sum of exactly 1,000 (the IPv4
    discard reason leaves, used by gNMI-1.29.9.2).
* Step 5 - Measure IPv6 drops.
  * Take a quiet baseline.
  * Await IPv6 neighbor discovery on the ATE, then send flow `v6-drop`: ATE
    port 1 to ATE port 2, IPv6 destination `2001:db8:100::1`, 1,000 packets.
    Await the `stopped` transmit state.
  * Record the settled final value and apply the standard drop validation (P0
    to P7) with `N` = 1,000.
  * Record which vendor leaves have an ingress sum of exactly 1,000 (the IPv6
    discard reason leaves, used by gNMI-1.29.9.3).
* Step 6 - Verify that both discard routes are still present in the AFT after
  both measurements. Traffic for one address family must not disturb the other.
* The discard routes stay installed for gNMI-1.29.7 and gNMI-1.29.9.

#### Pass/Fail criteria

* **Pass**: P0, P1, P2 and P4 to P6 hold with `N` = 1,000 for both the IPv4 and
  the IPv6 flow,
  both routes remain programmed, and all incremented leaves are logged.
* **Fail**: Any of P1, P2, P4, P5 or P6 fails for either address family, or
  either route is no longer programmed. This includes counters that increment
  by fewer packets (under-counting) or more packets (over-counting), and drops
  counted as `adverse` or `congestion`.

### gNMI-1.29.7 - Dual-Stack Multi-Port Multi-Prefix Drop Accounting

This subtest adds a small amount of scale: 32 additional discard prefixes, 3
ingress ports and 7 concurrent flows. Each ingress port sends a different
number of packets, so a drop attributed to the wrong integrated circuit or
counted twice produces a total that the test can detect. Repeating the
measurement 3 times detects counters that reset when read.

* Step 1 - Generate the DUT configuration.
  * Configure 16 IPv4 static routes `198.18.0.0/24` to `198.18.15.0/24` and 16
    IPv6 static routes `2001:db8:100:1::/64` to `2001:db8:100:10::/64`, each
    with a single next hop (index `0`) set to `DROP`.

The following Canonical OC shows 2 representative routes, `198.18.0.0/24`
and `2001:db8:100:1::/64`. Repeat the same structure, with next hop `DROP` and
index `0`, for all 16 IPv4 and 16 IPv6 prefixes listed in Step 1.

#### Canonical OC

```json
{
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
              "config": {
                "identifier": "oc-pol-types:STATIC",
                "name": "DEFAULT"
              },
              "identifier": "oc-pol-types:STATIC",
              "name": "DEFAULT",
              "static-routes": {
                "static": [
                  {
                    "config": {
                      "prefix": "198.18.0.0/24"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "config": {
                            "index": "0",
                            "next-hop": "DROP"
                          },
                          "index": "0"
                        }
                      ]
                    },
                    "prefix": "198.18.0.0/24"
                  },
                  {
                    "config": {
                      "prefix": "2001:db8:100:1::/64"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "config": {
                            "index": "0",
                            "next-hop": "DROP"
                          },
                          "index": "0"
                        }
                      ]
                    },
                    "prefix": "2001:db8:100:1::/64"
                  }
                ]
              }
            }
          ]
        }
      }
    ]
  }
}
```

* Step 2 - Push the configuration to the DUT using a single gNMI `Set`.
* Step 3 - Await all 32 new discard routes, the 2 discard routes from
  gNMI-1.29.6, and the 2 control routes from gNMI-1.29.5 in the
  static route state and in the AFT.
* Step 4 - Configure 7 flows on the ATE.

  | Flow        | Ingress    | Egress     | Destinations                                                         | Packets |
  | ----------- | ---------- | ---------- | -------------------------------------------------------------------- | ------- |
  | `v4-drop-A` | ATE port 1 | ATE port 4 | `198.18.0.1` to `198.18.15.1`, step `0.0.1.0`, 16 values             | 1,000   |
  | `v6-drop-A` | ATE port 1 | ATE port 4 | `2001:db8:100:1::1` to `2001:db8:100:10::1`, step `0:0:0:1::`, 16 values | 1,000 |
  | `v4-drop-B` | ATE port 2 | ATE port 4 | same as `v4-drop-A`                                                  | 2,000   |
  | `v6-drop-B` | ATE port 2 | ATE port 4 | same as `v6-drop-A`                                                  | 2,000   |
  | `v4-drop-C` | ATE port 3 | ATE port 4 | same as `v4-drop-A`                                                  | 3,000   |
  | `v6-drop-C` | ATE port 3 | ATE port 4 | same as `v6-drop-A`                                                  | 3,000   |
  | `v4-ctrl`   | ATE port 1 | ATE port 4 | `203.0.113.129`                                                      | 1,000   |

* Step 5 - Run 3 iterations. In each iteration:
  * Take a quiet baseline.
  * Start all 7 flows at the same time and await the `stopped` transmit state
    for each.
  * Record the settled final value for `N` = 12,000.
  * Apply the standard drop validation with the following expected values:
    * P0: each flow transmitted exactly its packet count.
    * P1: the `in-unicast-pkts` delta is at least 3,000 on `dut:port1`, 4,000
      on `dut:port2` and 6,000 on `dut:port3`.
    * P2: ATE port 4 received `0` packets of the 6 drop flows and exactly
      1,000 packets of `v4-ctrl`.
    * P3 (log only): the `packet-processing-aggregate` deltas are logged and
      compared with the expected values below.
    * P4: if the ingress map from gNMI-1.29.4 is available, at least one vendor
      leaf on the integrated circuit hosting each ingress port has a delta of
      exactly 2,000 (`dut:port1`), 4,000 (`dut:port2`) and 6,000
      (`dut:port3`), summed when ports share an integrated circuit. Otherwise,
      at least one vendor leaf, summed over all integrated circuits, has a
      delta of exactly 12,000. The IPv4 and IPv6 drops of a port are counted
      together, so a single vendor leaf must count the discard drops of both
      address families (as measured separately in gNMI-1.29.6).
    * P5, P6, P7 (log only) and logging as defined in "Standard drop
      validation".
  * Verify that each iteration produces exactly the same per-integrated-circuit
    deltas of the vendor drop leaves as the first iteration that completed.
* Step 6 (optional, high count, enabled with the test flag
  `--pipeline_hc_flow`) - Configure flow `v4-drop-hc`: ATE port 1 to
  ATE port 4, IPv4 destination `198.18.0.1`, 100,000 packets at 10,000 packets
  per second. Take a quiet baseline, run the flow, and apply the standard drop
  validation with `N` = 100,000. This catches hardware counters that are
  sampled or estimated and only look correct at low packet counts.
* The scale discard routes stay installed until the global cleanup.

#### Pass/Fail criteria

* **Pass**: In all 3 iterations, every check above holds with exactly the
  expected values, `v4-ctrl` is received without loss, and the deltas are
  identical across iterations.
* **Fail**: Any total or per-integrated-circuit value differs from the expected
  value in any iteration, `v4-ctrl` loses packets, `adverse` or `congestion`
  counters increment, or the deltas differ between iterations.

### gNMI-1.29.8 - Edge Link Cut Drop Accounting

This subtest reproduces the scenario that motivates vendor drop counters:
traffic is black-holed after the edge link towards its next hop is cut. The
packets that the DUT receives but cannot forward must be counted exactly by a
vendor drop leaf (typically a no route or no next hop reason), and never as an
`adverse` drop. When the link is restored, forwarding must recover and the drop
counters must stop incrementing.

* Step 1 - Generate the DUT configuration.
  * Configure an IPv4 static route for `100.64.0.0/24` with next hop
    `192.0.2.14` (ATE port 4), and an IPv6 static route for `2001:db8:300::/64`
    with next hop `2001:db8::e` (ATE port 4).

#### Canonical OC

```json
{
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
              "config": {
                "identifier": "oc-pol-types:STATIC",
                "name": "DEFAULT"
              },
              "identifier": "oc-pol-types:STATIC",
              "name": "DEFAULT",
              "static-routes": {
                "static": [
                  {
                    "config": {
                      "prefix": "100.64.0.0/24"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "config": {
                            "index": "0",
                            "next-hop": "192.0.2.14"
                          },
                          "index": "0"
                        }
                      ]
                    },
                    "prefix": "100.64.0.0/24"
                  },
                  {
                    "config": {
                      "prefix": "2001:db8:300::/64"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "config": {
                            "index": "0",
                            "next-hop": "2001:db8::e"
                          },
                          "index": "0"
                        }
                      ]
                    },
                    "prefix": "2001:db8:300::/64"
                  }
                ]
              }
            }
          ]
        }
      }
    ]
  }
}
```

* Step 2 - Push the configuration to the DUT using gNMI `Set`, and await both
  routes in the static route state and in the AFT.
* Step 3 - Pre-cut verification.
  * Take a quiet baseline.
  * Send flow `v4-edge` (ATE port 1 to ATE port 4, IPv4 destination
    `100.64.0.1`, 1,000 packets) and flow `v6-edge` (ATE port 1 to ATE port 4,
    IPv6 destination `2001:db8:300::1`, 1,000 packets).
  * Verify that ATE port 4 received exactly 1,000 packets of each flow and that
    no drop counter changed.
* Step 4 - Cut the edge link.
  * Set `/interfaces/interface/config/enabled` to `false` on `dut:port4`.
    Register the restoration of `enabled` = `true` as a deferred cleanup
    immediately, so that the link is restored even if the subtest fails.
  * Await `/interfaces/interface/state/oper-status` = `DOWN` on `dut:port4`.
  * Await the removal of `100.64.0.0/24` and `2001:db8:300::/64` from the AFT,
    or, on platforms that keep the entry, the removal of the next hop
    `192.0.2.14` / `2001:db8::e` from
    `/network-instances/network-instance/afts/next-hops/next-hop/state/ip-address`.
* Step 5 - Measure drops during the cut. Run IPv4 and IPv6 one after the other
  so that each count can be attributed:
  * Take a quiet baseline, send `v4-edge` (1,000 packets), record the settled
    final value and apply the standard drop validation with `N` = 1,000.
  * Take a quiet baseline, send `v6-edge` (1,000 packets), record the settled
    final value and apply the standard drop validation with `N` = 1,000.
  * In addition, if the DUT populates the lookup block counters, log the
    delta of `drop/lookup-block/state/no-route` and
    `drop/lookup-block/state/no-nexthop` on the ingress integrated circuit.
    The lookup block counters are optional and are not a pass/fail condition.
  * Control plane fallback for P4: traffic that matches no route may be punted
    to the CPU and dropped by control plane policing (CoPP) instead of being
    counted as a pipeline drop. Before each flow, record the sum of
    `/components/component/integrated-circuit/pipeline-counters/control-plane-traffic/state/dropped-aggregate`
    over all integrated circuits. If no vendor drop leaf increments at all
    during the flow, P4 passes when the delta of this sum is at least 1,000;
    log that 0 pipeline vendor drops and the control plane drop count were
    recorded. The step fails if neither the vendor drop leaves nor the control
    plane dropped-aggregate account for the 1,000 packets. A vendor drop count
    that is greater than 0 but not exactly 1,000 still fails P4. A decrease of
    the control plane dropped-aggregate is logged and its delta is treated as
    `0`.
* Step 6 - Restore the edge link.
  * Set `/interfaces/interface/config/enabled` to `true` on `dut:port4`.
  * Await `oper-status` = `UP`, the IPv4 neighbor `link-layer-address` and the
    IPv6 neighbor `neighbor-state` for ATE port 4, and the return of both routes
    to the AFT.
* Step 7 - Post-restore verification. Repeat Step 3. Both flows must be received
  without loss and no drop counter may change.

#### Pass/Fail criteria

* **Pass**: Traffic is forwarded without loss before the cut and after the
  restore, with no drop counter changes. During the cut, P0, P1, P2 and P4 to
  P6 hold with
  `N` = 1,000 for each address family, where P4 is met either by a vendor drop
  leaf or by the control plane fallback described in Step 5.
* **Fail**: Any loss before the cut or after the restore, any drop counter
  change outside the cut, or any failed check during the cut.

### gNMI-1.29.9 - Negative Tests

These subtests verify that the drop counters do not report drops that did not
happen, do not reset, and attribute drops to the correct reason.

Two negative checks are built into earlier subtests and must also pass:
forwarded traffic never increments drop counters (gNMI-1.29.5 and flow
`v4-ctrl` in gNMI-1.29.7), and expected drops are never classified as
`adverse` (check P5 in every drop step of gNMI-1.29.6 to gNMI-1.29.8).

#### gNMI-1.29.9.1 - Counters Do Not Reset

* Without sending traffic, wait for a quiet period (two consecutive samples
  with no change, as in "Quiet baseline"). Within that period, read the vendor
  drop leaves 3 times in a row: twice with a gNMI `Subscribe` in mode `ONCE`,
  and once from the `SAMPLE` subscription. Then await the next `SAMPLE` sample.
  If it is unchanged, all 3 reads must return identical values: the same leaves
  (no leaf missing or new) with the same values. If a `Subscribe ONCE` request
  fails, the subtest fails, and the remaining reads and checks still run (the
  failed request is not repeated).
* Configure an IPv4 static route for `192.0.2.128/25` with next hop `DROP`,
  await it in the AFT, delete it, and await its removal from the AFT.
* Read the vendor drop leaves again within a quiet period, in the same way.
  The values must be identical to the reads above.
* If the next `SAMPLE` sample shows that a counter changed during the reads,
  or the values changed between the reads before and after the configuration
  change (a background drop between the reads also does this), discard the
  reads and repeat the whole sequence, up to 3 attempts in total, before
  failing.
* The monotonicity check must not have recorded any decrease during the test.

#### Canonical OC

```json
{
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
              "config": {
                "identifier": "oc-pol-types:STATIC",
                "name": "DEFAULT"
              },
              "identifier": "oc-pol-types:STATIC",
              "name": "DEFAULT",
              "static-routes": {
                "static": [
                  {
                    "config": {
                      "prefix": "192.0.2.128/25"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "config": {
                            "index": "0",
                            "next-hop": "DROP"
                          },
                          "index": "0"
                        }
                      ]
                    },
                    "prefix": "192.0.2.128/25"
                  }
                ]
              }
            }
          ]
        }
      }
    ]
  }
}
```

#### gNMI-1.29.9.2 - IPv4 Drop Reason Changes With The Route

* Use the vendor leaves that had an ingress sum of exactly 1,000 in
  gNMI-1.29.6 Step 4 (the discard reason leaves). If none was recorded (P4 of
  gNMI-1.29.6 failed), the subtest fails because the drop reason change cannot
  be verified, but the steps below still run.
* Delete the static route for `198.51.100.0/24` and await its removal from the
  IPv4 AFT.
* Take a quiet baseline, send flow `v4-drop` (ATE port 1 to ATE port 2, IPv4
  destination `198.51.100.1`, 1,000 packets), and record the settled final
  value.
* Apply the standard drop validation with `N` = 1,000.
* Log the vendor leaves that incremented, and compare the leaves whose ingress
  sum is exactly 1,000 with the discard reason leaves recorded above:
  * A leaf that is in both sets counts discard and no-route drops alike. Log
    it as information; it does not fail the subtest.
  * If a leaf whose ingress sum is exactly 1,000 is not a discard reason leaf,
    verify that every discard reason leaf that is not in this set has a delta
    of `0` on every integrated circuit, so the drop is attributed to a
    different reason (no route).
* If the DUT populates the lookup block counters, log the delta of
  `drop/lookup-block/state/no-route` on the ingress integrated circuit. The
  lookup block counters are optional and are not a pass/fail condition.

#### gNMI-1.29.9.3 - IPv6 Drop Reason Changes With The Route

* Repeat gNMI-1.29.9.2 for IPv6: delete `2001:db8:100::/64`, send flow
  `v6-drop` (IPv6 destination `2001:db8:100::1`, 1,000 packets), and use the
  discard reason leaves recorded in gNMI-1.29.6 Step 5.

#### gNMI-1.29.9.4 - No Drops On Integrated Circuits Without Ingress Traffic

* This subtest runs only if the ingress map from gNMI-1.29.4 is available and
  the DUT has more than one integrated circuit. Otherwise it is skipped and the
  reason is logged.
* During every single-flow step of gNMI-1.29.6 and gNMI-1.29.8
  (traffic from ATE port 1 only), verify that every integrated circuit that
  does not host `dut:port1` reports a delta of `0` for every vendor drop leaf,
  and for `packet-processing-aggregate` if it is reported. Log the steps that
  were evaluated; the subtest fails if none was.

#### gNMI-1.29.9.5 - TTL Expiry Drops

* Send flow `v4-ttl` (ATE port 1 to ATE port 4, IPv4 destination
  `203.0.113.129`, TTL `1`, 1,000 packets) and flow `v6-ttl` (ATE port 1 to ATE
  port 4, IPv6 destination `2001:db8:200::1`, hop limit `1`, 1,000 packets), one
  after the other, each with a quiet baseline.
* P0 applies: each flow must transmit exactly 1,000 packets.
* Verify that ATE port 4 receives `0` packets of each flow.
* Verify that at least one vendor drop leaf has an ingress sum of at least
  1,000. Log the `packet-processing-aggregate` delta if it is reported.
* A platform that punts TTL expired packets to the control plane instead of
  counting them as pipeline drops must document this in a deviation
  (`pipeline_drop_ttl_expired_punted`). On such a platform, instead of the
  vendor leaf lower bound, verify that the delta of
  `/components/component/integrated-circuit/pipeline-counters/control-plane-traffic/state/queued-aggregate`
  plus
  `/components/component/integrated-circuit/pipeline-counters/control-plane-traffic/state/dropped-aggregate`,
  summed over all integrated circuits, is at least 1,000 (every punted packet
  is either enqueued to the CPU or dropped by control plane policing). The ATE
  port 4 check and P5 still apply.
* Verify P5: the `adverse` counters do not increment.

#### gNMI-1.29.9.6 - Subscription To A Non-Existent Vendor Container

* Send a gNMI `Subscribe` in mode `ONCE` for
  `/components/component/integrated-circuit/pipeline-counters/drop/vendor/NonExistentVendor`.
* Verify that the RPC either completes cleanly with no updates (with a
  `sync_response` or an `EOF` after zero updates), or returns the gRPC status
  `NOT_FOUND` or `INVALID_ARGUMENT`.
* Verify that `/components/component/state/oper-status` is `ACTIVE` for every
  integrated circuit that reports oper-status (readiness gate), and that the
  drop subscription is still streaming.

#### Pass/Fail criteria

* **Pass**: Every negative subtest that is not skipped meets its expectations.
* **Fail**: Any negative subtest does not meet its expectations.

### Cleanup

At the end of the test, regardless of the result, restore the DUT to its
original state. The cleanup steps are independent: a failure in one step is
reported and the remaining steps still run, also when setup failed part-way:

* Stop and remove all ATE flows (if any flow was configured).
* Verify that the monotonicity check recorded no decrease, and close the drop
  subscription.
* Set `/interfaces/interface/config/enabled` to `true` on any DUT port where it
  is `false`.
* Delete every static route configured by this test that is still present:
  `203.0.113.2/32`, `203.0.113.128/25`,
  `2001:db8:200::/64`, `198.51.100.0/24`, `2001:db8:100::/64`, `198.18.0.0/24`
  to `198.18.15.0/24`, `2001:db8:100:1::/64` to `2001:db8:100:10::/64`,
  `100.64.0.0/24`, `2001:db8:300::/64` and `192.0.2.128/25`. Await the removal
  of each one from the AFT.
* Verify that `/interfaces/interface/state/oper-status` is `UP` on all 4 DUT
  ports.
* Verify that `/components/component/state/oper-status` is `ACTIVE` for every
  integrated circuit that reports oper-status (skipped if the integrated
  circuits were not discovered).

## Canonical OC

The following Canonical OC shows `port1` in full. `port2`, `port3` and
`port4` use exactly the same structure, with the addresses listed in the
"Interface addressing" table.

```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "DUT port1 (ingress A)",
          "enabled": true,
          "mtu": 1514,
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
                  "enabled": true,
                  "mtu": 1500
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
                  "enabled": true,
                  "mtu": 1500
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
          "type": "oc-ni-types:DEFAULT_INSTANCE"
        },
        "name": "DEFAULT",
        "protocols": {
          "protocol": [
            {
              "config": {
                "identifier": "oc-pol-types:STATIC",
                "name": "DEFAULT"
              },
              "identifier": "oc-pol-types:STATIC",
              "name": "DEFAULT"
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
  # TODO: Replace the following non-OC vendor paths with actual OC paths once they are available in the public models.
  # /components/component/integrated-circuit/pipeline-counters/drop/vendor/CiscoXR/spitfire/packet-processing/state/L3_ROUTE_LOOKUP_FAILED:
  # /components/component/integrated-circuit/pipeline-counters/drop/vendor/CiscoXR/spitfire/packet-processing/state/L3_NULL_ADJ*:
  # /components/component/integrated-circuit/pipeline-counters/drop/vendor/CiscoXR/spitfire/packet-processing/state/MPLS_TE_MIDPOINT_LDP_LABELS_MISS*:
  # Native IOS-XR 25.4.15+: .../drop/vendor/<module>:cisco/<asic>/packet-processing/state/{l3_drop_adj_non_inject,l3_null_adj,mpls_te_midpoint_ldp_labels_miss}

  # gNMI-1.29.4 to gNMI-1.29.9: vendor-neutral drop counter coverage.
  # The leaves under the vendor container are defined by each vendor, so no
  # single leaf path can be listed. The container covered is:
  # /components/component/integrated-circuit/pipeline-counters/drop/vendor

  # Integrated circuit discovery, health and port mapping
  /components/component/state/type:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/state/oper-status:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/state/parent:
    platform_type: ["PORT"]
  /interfaces/interface/state/hardware-port:

  # OC drop aggregates and lookup block counters
  /components/component/integrated-circuit/pipeline-counters/drop/state/packet-processing-aggregate:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/pipeline-counters/drop/state/adverse-aggregate:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/pipeline-counters/drop/state/congestion-aggregate:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/pipeline-counters/drop/lookup-block/state/no-route:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/pipeline-counters/drop/lookup-block/state/no-nexthop:
    platform_type: ["INTEGRATED_CIRCUIT"]

  # Control plane traffic (gNMI-1.29.8 P4 fallback; gNMI-1.29.9.5 with the
  # pipeline_drop_ttl_expired_punted deviation)
  /components/component/integrated-circuit/pipeline-counters/control-plane-traffic/state/queued-aggregate:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/pipeline-counters/control-plane-traffic/state/dropped-aggregate:
    platform_type: ["INTEGRATED_CIRCUIT"]

  # Interface configuration
  /interfaces/interface/config/type:
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/mtu:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/mtu:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv6/config/mtu:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:

  # Default network instance and static route configuration
  /network-instances/network-instance/config/name:
  /network-instances/network-instance/config/type:
  /network-instances/network-instance/protocols/protocol/config/identifier:
  /network-instances/network-instance/protocols/protocol/config/name:
  /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/index:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop:

  # Static route and forwarding state
  /network-instances/network-instance/protocols/protocol/static-routes/static/state/prefix:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/origin-protocol:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/origin-protocol:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address:
  /network-instances/network-instance/afts/next-hops/next-hop/interface-ref/state/interface:

  # Interface state, neighbors and counters
  /interfaces/interface/state/oper-status:
  /interfaces/interface/state/counters/in-unicast-pkts:
  /interfaces/interface/state/counters/in-errors:
  /interfaces/interface/state/counters/in-discards:
  /interfaces/interface/ethernet/state/mac-address:
  /interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/state/link-layer-address:
  /interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/neighbor-state:

rpcs:
  gnmi:
    gNMI.Set:
      replace: true
      update: true
      delete: true
    gNMI.Subscribe:
      sample: true
      once: true
```

## Required DUT platform

* FFF
