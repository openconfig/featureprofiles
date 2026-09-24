# RT-1.111: BGP Autonomous System Confederations (RFC 5065)

## Summary

This test validates support for BGP Autonomous System Confederations as defined
in [RFC 5065](https://datatracker.ietf.org/doc/html/rfc5065) using OpenConfig
data models. A BGP confederation subdivides a single Autonomous System
(identified by an externally visible AS Confederation Identifier) into multiple
smaller Member Autonomous Systems (Member-AS). This reduces the internal BGP
(iBGP) full-mesh requirement while the confederation is still presented as a
single Autonomous System to BGP peers outside the confederation.

The test uses a single DUT that is a member of Member-AS `64501` inside
confederation `64500`. The ATE emulates three different kinds of BGP neighbors
on three DUT ports:

*   `ATE:port1` emulates an **external** AS (`64510`) that is not a member of
    the confederation (standard eBGP).
*   `ATE:port2` emulates an **iBGP** speaker inside the DUT's own Member-AS
    (`64501`).
*   `ATE:port3` emulates a speaker in a **neighboring Member-AS** (`64502`) of
    the same confederation (confederation-eBGP).

The test covers the following requirements for both IPv4 unicast and IPv6
unicast:

1.  **Configuration and telemetry**: The OpenConfig BGP `confederation`
    `identifier` and `member-as` leaves are configurable via gNMI `Set` and are
    reflected in the corresponding `state` leaves.
2.  **AS_PATH modification rules for propagated routes (RFC 5065
    Section 4.1)**. The following rules are covered:
    *   Rule a (advertise to a peer in the same Member-AS: AS_PATH not
        modified): RT-1.111.2 and RT-1.111.5.
    *   Rule b.1 (advertise to a neighboring Member-AS, first segment is
        `AS_CONFED_SEQUENCE`: prepend Member-AS to that segment): RT-1.111.3.
    *   Rule b.2 (advertise to a neighboring Member-AS, first segment is not
        `AS_CONFED_SEQUENCE`: prepend a new `AS_CONFED_SEQUENCE`): RT-1.111.5.
    *   Rule b.3 (advertise to a neighboring Member-AS, AS_PATH empty: create
        `AS_CONFED_SEQUENCE`): RT-1.111.3.
    *   Rule c.1 (advertise outside the confederation: remove all
        `AS_CONFED_*` segments): RT-1.111.4.
    *   Rule c.2 (then, first remaining segment is `AS_SEQUENCE`: prepend the
        Confederation Identifier to it): RT-1.111.4.
    *   Rule c.4 (then, remaining AS_PATH empty: create `AS_SEQUENCE` with the
        Confederation Identifier): RT-1.111.4.
3.  **Attributes across the Member-AS boundary (RFC 5065 Sections 5.1 and
    5.2)**: LOCAL_PREF and MED received from the iBGP peer are sent unchanged
    to the neighboring Member-AS, and the NEXT_HOP is either unchanged (the
    default described in Section 5.1) or set to the DUT's own address.
4.  **No confederation information leaks to the external peer (RFC 5065
    Section 5)**: No `AS_CONFED_*` segment and no Member-AS number is sent to
    `ATE:port1`, and LOCAL_PREF is not sent to `ATE:port1` (RFC 4271
    Section 5.1.5).
5.  **AS loop detection (negative, RFC 5065 Section 4)**: The DUT rejects a
    route whose `AS_CONFED_SEQUENCE` contains its own Member-AS `64501`, and a
    route whose `AS_SEQ` contains its own Confederation Identifier `64500`.
6.  **Malformed AS_PATH handling (negative, RFC 5065 Section 5)**: The DUT
    does not accept an `AS_CONFED_SEQUENCE` from the external peer, or a
    route from the neighboring Member-AS whose first segment is not an
    `AS_CONFED_SEQUENCE`, and keeps the BGP session up.

Data plane forwarding is verified with ATE-to-ATE traffic flows in both
directions between every pair of ATE ports, for both IPv4 and IPv6.

The following are **not covered** by this test: RFC 5065 Section 4.1 rule c.3
(first remaining segment is an `AS_SET`), the rules for routes originated by
the DUT itself, the `AS_CONFED_SET` segment type, and the RFC 5065
Section 5.3 path selection rules (for example, not counting `AS_CONFED_*`
segments in AS_PATH length). Path selection is a candidate for a follow-up
test that needs two paths to the same prefix.

## Testbed type

*   [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)
    (`TESTBED_DUT_ATE_4LINKS`). Only `port1`, `port2`, and `port3` are used;
    `port4` is not used by this test.

## Procedure

### Test environment setup

#### Test Topology

```text
                          Confederation 64500
              +-----------------------------------------+
              |                                         |
+----------+  |  +-------------+          +----------+  |
| ATE      |  |  |     DUT     |  port2   | ATE      |  |
| port1    |=====| Member-AS   |==========| port2    |  |
| AS 64510 |  |  |   64501     |  iBGP    | AS 64501 |  |
+----------+  |  +-------------+          +----------+  |
 External     |        || port3                         |
 eBGP (DUT    |        || confederation-eBGP            |
 uses 64500)  |  +-------------+                        |
              |  | ATE port3   |                        |
              |  | Member-AS   |                        |
              |  |   64502     |                        |
              |  +-------------+                        |
              +-----------------------------------------+
```

*   Connect `DUT:port1` to `ATE:port1` (external eBGP, ATE AS `64510`).
*   Connect `DUT:port2` to `ATE:port2` (iBGP, ATE AS `64501`).
*   Connect `DUT:port3` to `ATE:port3` (confederation-eBGP, ATE Member-AS
    `64502`).

No DUT loopback interface is required. The DUT BGP router ID is configured as
the static value `192.0.2.254`, which is not assigned to any interface and is
not advertised in BGP.

#### BGP Autonomous System Numbers (Table 1)

All AS numbers are from the RFC 5398 documentation range.

| Device | BGP role toward DUT | Local AS used in the session | AS the peer sees for the DUT |
| :--- | :--- | :--- | :--- |
| DUT | Confederation member | Member-AS `64501`, Confederation ID `64500`, `member-as` `[64502]` | N/A |
| `ATE:port1` | External eBGP peer | `64510` | `64500` (Confederation ID) |
| `ATE:port2` | iBGP peer in the same Member-AS | `64501` | `64501` |
| `ATE:port3` | Confederation-eBGP peer in Member-AS `64502` | `64502` | `64501` (Member-AS) |

Two more AS numbers appear only inside AS_PATHs sent by the ATE: `64503`
(another Member-AS of the confederation, behind the iBGP peer) and `64511`
(an external AS behind Member-AS `64502`). The DUT has no session with either.

Per RFC 5065 Section 4, the DUT uses its Confederation Identifier (`64500`) in
the OPEN message and AS_PATH toward `ATE:port1`, because AS `64510` is not a
member of the confederation. It uses its Member-AS number (`64501`) toward
`ATE:port2` and `ATE:port3`, because both are confederation members.

#### Interface Addressing (Table 2)

IPv4 addresses are from RFC 5737 `192.0.2.0/24` and IPv6 addresses are from
RFC 3849 `2001:db8::/32`. Interface addresses are taken only from
`192.0.2.0/24` and `2001:db8:0::/64`, which do not overlap any advertised
prefix in Table 3.

| Link | Device and port | IPv4 address | IPv6 address |
| :--- | :--- | :--- | :--- |
| Link 1 | `DUT:port1` | `192.0.2.1/30` | `2001:db8::1/126` |
| Link 1 | `ATE:port1` | `192.0.2.2/30` | `2001:db8::2/126` |
| Link 2 | `DUT:port2` | `192.0.2.5/30` | `2001:db8::5/126` |
| Link 2 | `ATE:port2` | `192.0.2.6/30` | `2001:db8::6/126` |
| Link 3 | `DUT:port3` | `192.0.2.9/30` | `2001:db8::9/126` |
| Link 3 | `ATE:port3` | `192.0.2.10/30` | `2001:db8::a/126` |

#### Advertised BGP Prefixes (Table 3)

Advertised prefixes are from RFC 5737 `198.51.100.0/24` and `203.0.113.0/24`
and from RFC 3849 `2001:db8::/32` outside `2001:db8:0::/48`. The column
"AS_PATH sent by ATE" is the exact AS_PATH attribute that must be present in
the UPDATE received by the DUT. The LOCAL_PREF and MED columns give the exact
attributes the ATE sends; "not sent" means the attribute is not included in
the UPDATE.

| Name | Advertised by | IPv4 prefix | IPv6 prefix | AS_PATH sent by ATE | LOCAL_PREF | MED | Used in |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `EXT` | `ATE:port1` | `203.0.113.0/26` | `2001:db8:100::/48` | `AS_SEQ [64510]` | not sent | not sent | Base |
| `IBGP` | `ATE:port2` | `198.51.100.0/26` | `2001:db8:200::/48` | Empty | `200` | `50` | Base |
| `IBGP-CONFED` | `ATE:port2` | `198.51.100.192/26` | `2001:db8:201::/48` | `AS_CONFED_SEQUENCE [64503]` | `100` | not sent | Base |
| `CONFED` | `ATE:port3` | `198.51.100.64/26` | `2001:db8:300::/48` | `AS_CONFED_SEQUENCE [64502]` | not sent | not sent | Base |
| `CONFED-TRANSIT` | `ATE:port3` | `203.0.113.128/26` | `2001:db8:302::/48` | `AS_CONFED_SEQUENCE [64502]`, `AS_SEQ [64511]` | not sent | not sent | Base |
| `CONFED-LOOP` | `ATE:port3` | `198.51.100.128/26` | `2001:db8:301::/48` | `AS_CONFED_SEQUENCE [64502, 64501]` | not sent | not sent | RT-1.111.6 |
| `EXT-LOOP` | `ATE:port1` | `203.0.113.64/26` | `2001:db8:101::/48` | `AS_SEQ [64510, 64500]` | not sent | not sent | RT-1.111.6 |
| `EXT-MALFORMED` | `ATE:port1` | `203.0.113.192/27` | `2001:db8:102::/48` | `AS_CONFED_SEQUENCE [64510]`, `AS_SEQ [64510]` | not sent | not sent | RT-1.111.7 |
| `CONFED-MALFORMED` | `ATE:port3` | `203.0.113.224/27` | `2001:db8:303::/48` | `AS_SEQ [64502]` | not sent | not sent | RT-1.111.7 |

The ATE is not a native confederation speaker. The AS_PATH of each route range
is built explicitly with the OTG route range `as_path` configuration. Do not
use the OTG `include_as_confed_seq` mode, because how it combines the local AS
with configured segments is not defined precisely enough for this test.

OTG sends LOCAL_PREF and MED by default (`advanced.include_local_preference`
and `advanced.include_multi_exit_discriminator` both default to `true`), so
every route range sets both flags explicitly:

| Name | OTG `as_path` | OTG `advanced` |
| :--- | :--- | :--- |
| `EXT` | `as_set_mode` = `include_as_seq`, no segments | `include_local_preference` = `false`, `include_multi_exit_discriminator` = `false` |
| `IBGP` | `as_set_mode` = `do_not_include_local_as`, no segments | `include_local_preference` = `true`, `local_preference` = `200`, `include_multi_exit_discriminator` = `true`, `multi_exit_discriminator` = `50` |
| `IBGP-CONFED` | `do_not_include_local_as`; segment `as_confed_seq` `[64503]` | `include_local_preference` = `true`, `local_preference` = `100`, `include_multi_exit_discriminator` = `false` |
| `CONFED` | `do_not_include_local_as`; segment `as_confed_seq` `[64502]` | `include_local_preference` = `false`, `include_multi_exit_discriminator` = `false` |
| `CONFED-TRANSIT` | `do_not_include_local_as`; segments `as_confed_seq` `[64502]`, then `as_seq` `[64511]` | `include_local_preference` = `false`, `include_multi_exit_discriminator` = `false` |
| `CONFED-LOOP` | `do_not_include_local_as`; segment `as_confed_seq` `[64502, 64501]` | `include_local_preference` = `false`, `include_multi_exit_discriminator` = `false` |
| `EXT-LOOP` | `do_not_include_local_as`; segment `as_seq` `[64510, 64500]` | `include_local_preference` = `false`, `include_multi_exit_discriminator` = `false` |
| `EXT-MALFORMED` | `do_not_include_local_as`; segments `as_confed_seq` `[64510]`, then `as_seq` `[64510]` | `include_local_preference` = `false`, `include_multi_exit_discriminator` = `false` |
| `CONFED-MALFORMED` | `do_not_include_local_as`; segment `as_seq` `[64502]` | `include_local_preference` = `false`, `include_multi_exit_discriminator` = `false` |

The AS_PATH checks on the DUT RIB in RT-1.111.2, RT-1.111.3, and RT-1.111.5
also confirm that the ATE encoded each AS_PATH as intended.

#### ATE Traffic Flows (Table 4)

All flows use a frame size of `512` bytes, a rate of `1000` packets per second,
and a duration of `30` seconds. The source address of each flow is inside the
prefix advertised by the transmitting ATE port, and the destination address is
inside the prefix advertised by the receiving ATE port. No traffic is sent to
any DUT address.

| Flow name | Tx port | Rx port | Source IP | Destination IP | Used in |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `v4-p2-to-p3` | `ATE:port2` | `ATE:port3` | `198.51.100.1` | `198.51.100.65` | RT-1.111.2 |
| `v6-p2-to-p3` | `ATE:port2` | `ATE:port3` | `2001:db8:200::1` | `2001:db8:300::1` | RT-1.111.2 |
| `v4-p3-to-p2` | `ATE:port3` | `ATE:port2` | `198.51.100.65` | `198.51.100.1` | RT-1.111.3 |
| `v6-p3-to-p2` | `ATE:port3` | `ATE:port2` | `2001:db8:300::1` | `2001:db8:200::1` | RT-1.111.3 |
| `v4-p1-to-p3` | `ATE:port1` | `ATE:port3` | `203.0.113.1` | `198.51.100.65` | RT-1.111.4 |
| `v6-p1-to-p3` | `ATE:port1` | `ATE:port3` | `2001:db8:100::1` | `2001:db8:300::1` | RT-1.111.4 |
| `v4-p1-to-p2` | `ATE:port1` | `ATE:port2` | `203.0.113.1` | `198.51.100.1` | RT-1.111.4 |
| `v6-p1-to-p2` | `ATE:port1` | `ATE:port2` | `2001:db8:100::1` | `2001:db8:200::1` | RT-1.111.4 |
| `v4-p3-to-p1` | `ATE:port3` | `ATE:port1` | `198.51.100.65` | `203.0.113.1` | RT-1.111.5 |
| `v6-p3-to-p1` | `ATE:port3` | `ATE:port1` | `2001:db8:300::1` | `2001:db8:100::1` | RT-1.111.5 |
| `v4-p2-to-p1` | `ATE:port2` | `ATE:port1` | `198.51.100.1` | `203.0.113.1` | RT-1.111.5 |
| `v6-p2-to-p1` | `ATE:port2` | `ATE:port1` | `2001:db8:200::1` | `2001:db8:100::1` | RT-1.111.5 |

Together these flows exercise both directions between every pair of ATE ports.
The routes `IBGP-CONFED` and `CONFED-TRANSIT` are used only for control plane
checks; they use the same forwarding paths as `IBGP` and `CONFED`.

#### DUT and ATE Base Configuration

1.  Configure `DUT:port1`, `DUT:port2`, and `DUT:port3` with the IPv4 and IPv6
    addresses in Table 2 in the `DEFAULT` network instance.
2.  Create a routing policy `ALLOW` with a single statement whose
    `policy-result` is `ACCEPT_ROUTE`.
3.  Configure BGP in the `DEFAULT` network instance on the DUT:
    *   `global/config/as` = `64501` and `global/config/router-id` =
        `192.0.2.254`.
    *   `global/confederation/config/identifier` = `64500`.
    *   `global/confederation/config/member-as` = `[64502]`.
    *   Enable `IPV4_UNICAST` and `IPV6_UNICAST` globally.
4.  Configure three peer-groups, each with `IPV4_UNICAST` and `IPV6_UNICAST`
    enabled and import and export policy `ALLOW`:
    *   `EBGP-EXT` with `peer-as` = `64510`.
    *   `IBGP-MEMBER` with `peer-as` = `64501`.
    *   `CONFED-EBGP` with `peer-as` = `64502`.
5.  Configure six neighbors. Because each peer-group enables both address
    families, every neighbor explicitly enables only its own address family
    and disables the other one, so that no neighbor inherits the other
    address family from its peer-group:
    *   IPv4 neighbors: `IPV4_UNICAST` `enabled` = `true` and `IPV6_UNICAST`
        `enabled` = `false`.
    *   IPv6 neighbors: `IPV6_UNICAST` `enabled` = `true` and `IPV4_UNICAST`
        `enabled` = `false`.
    *   `192.0.2.2` and `2001:db8::2` in `EBGP-EXT` (`peer-as` `64510`).
    *   `192.0.2.6` and `2001:db8::6` in `IBGP-MEMBER` (`peer-as` `64501`).
    *   `192.0.2.10` and `2001:db8::a` in `CONFED-EBGP` (`peer-as` `64502`).
6.  Push the configuration with gNMI `Set` (`REPLACE`). The complete DUT
    configuration is shown in [Canonical OC](#canonical-oc).
7.  On the ATE, configure the interfaces in Table 2 and one IPv4 and one IPv6
    BGP peer per port toward the DUT address on the same link:
    *   `ATE:port1`: `as_type` = `ebgp`, `as_number` = `64510`.
    *   `ATE:port2`: `as_type` = `ibgp`, `as_number` = `64501`.
    *   `ATE:port3`: `as_type` = `ebgp`, `as_number` = `64502`.
    *   OTG enables both `capability.ipv4_unicast` and
        `capability.ipv6_unicast` by default. On every IPv4 peer set
        `ipv4_unicast` = `true` and `ipv6_unicast` = `false`; on every IPv6
        peer set `ipv4_unicast` = `false` and `ipv6_unicast` = `true`.
8.  Configure the ATE route ranges marked "Base" in Table 3 (`EXT`, `IBGP`,
    `IBGP-CONFED`, `CONFED`, and `CONFED-TRANSIT`) with the OTG settings
    above. The other route ranges are added in RT-1.111.6 and RT-1.111.7.
9.  Configure the flows in Table 4, but do not start them yet.
10. Push the ATE configuration and start protocols. Wait for ARP and IPv6
    neighbor discovery to complete for all three links.

NOTE: OTG does not configure a remote AS for a BGP peer, so the ATE does not
itself check which AS the DUT uses in its OPEN message. The use of
Confederation Identifier `64500` toward `ATE:port1` is verified through the
AS_PATH received by `ATE:port1` in RT-1.111.4.

All DUT telemetry in this test is read with gNMI `Subscribe` in `ON_CHANGE`
mode (for example, `gnmi.Watch` / `gnmi.Await` in Ondatra), which matches the
RPC coverage below.

#### Traffic Pass/Fail Criteria (applies to every flow check)

*   **Pass**: For each flow, the ATE Rx packet count on the receiving port
    equals the Tx packet count (0% loss), and the Tx packet count is
    non-zero.
*   **Fail**: Any loss greater than 0%, or zero packets transmitted.

#### Absence Check Procedure (applies to RT-1.111.6 and RT-1.111.7)

A negative result ("the prefix is not present") is only meaningful if the ATE
actually sent the route and the DUT had time to process it. For every
absence check:

1.  **Positive control**: Before adding the new route range, record the OTG
    `bgpv4-metrics` / `bgpv6-metrics` `routes_advertised` counter of the
    sending ATE peer. After adding it, wait until the counter increases by
    `1`. Optionally, if the DUT supports `adj-rib-in-pre`, also confirm that
    the prefix is visible there (informational only).
2.  **Hold window**: Then watch the checked location for a hold window of `30`
    seconds (for example, a `gnmi.Watch` on the DUT RIB that must time out
    without the prefix appearing, and repeated OTG `bgp-prefix` state reads).
    The prefix must stay absent for the whole window.

---

### RT-1.111.1: Confederation Configuration, State, and Session Establishment

This subtest checks that the DUT accepts the confederation configuration and
reports it in operational state, and that all three types of BGP sessions come
up.

#### Step 1: Verify Confederation State Telemetry

1.  Apply the base configuration from
    [Test environment setup](#test-environment-setup).
2.  Using gNMI `Subscribe` (`ON_CHANGE`), read:
    *   `/network-instances/network-instance/protocols/protocol/bgp/global/state/as`
    *   `/network-instances/network-instance/protocols/protocol/bgp/global/confederation/state/identifier`
    *   `/network-instances/network-instance/protocols/protocol/bgp/global/confederation/state/member-as`

#### Step 2: Verify BGP Sessions

1.  For each of the six neighbors in the base configuration, wait for
    `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state`
    to become `ESTABLISHED`.

#### RT-1.111.1 Pass/Fail Criteria

*   **Pass**:
    *   `global/state/as` equals `64501`.
    *   `global/confederation/state/identifier` equals `64500`.
    *   `global/confederation/state/member-as` equals `[64502]`.
    *   All six neighbors (`192.0.2.2`, `2001:db8::2`, `192.0.2.6`,
        `2001:db8::6`, `192.0.2.10`, `2001:db8::a`) report `session-state`
        `ESTABLISHED`.
*   **Fail**: Any of the state leaves is missing or differs from the
    configured value, or any session does not reach `ESTABLISHED`.

---

### RT-1.111.2: Confederation Peer to iBGP Peer (AS_PATH Unchanged)

RFC 5065 Section 4.1, rule a: when a speaker advertises a route to a peer in
its own Member-AS, it SHALL NOT modify the AS_PATH. The routes `CONFED` and
`CONFED-TRANSIT` are received from Member-AS `64502` on `DUT:port3` and
advertised by the DUT to the iBGP peer on `DUT:port2`.

#### Step 1: Verify DUT RIB

1.  On the DUT, find the `adj-rib-in-post` routes for `198.51.100.64/26` and
    `203.0.113.128/26` from neighbor `192.0.2.10`, and for
    `2001:db8:300::/48` and `2001:db8:302::/48` from neighbor `2001:db8::a`.
    Follow `state/attr-index` to the referenced `attr-set`.
2.  Read the AS_PATH segments of that `attr-set`:
    *   `/network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/type`
    *   `/network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/member`
3.  Confirm that the four prefixes are present in the DUT `loc-rib`.

#### Step 2: Verify Routes Received by `ATE:port2`

1.  On `ATE:port2`, read the OTG BGP prefix state (`bgp-prefix` states for the
    IPv4 and IPv6 peers) for the same four prefixes, and inspect the received
    AS_PATH segments.

#### Step 3: Verify Traffic

1.  Start flows `v4-p2-to-p3` and `v6-p2-to-p3` for 30 seconds, stop them, and
    check the counters.

#### RT-1.111.2 Pass/Fail Criteria

*   **Pass**:
    *   On the DUT, the AS_PATH of `198.51.100.64/26` and `2001:db8:300::/48`
        is exactly one segment of type `AS_CONFED_SEQUENCE` with members
        `[64502]`, and the AS_PATH of `203.0.113.128/26` and
        `2001:db8:302::/48` is `AS_CONFED_SEQUENCE [64502]` followed by
        `AS_SEQ [64511]`.
    *   `ATE:port2` receives `198.51.100.64/26` and `2001:db8:300::/48` with
        an AS_PATH of exactly one segment of type `as_confed_seq` with
        `as_numbers` `[64502]`.
    *   `ATE:port2` receives `203.0.113.128/26` and `2001:db8:302::/48` with
        an AS_PATH of exactly two segments: `as_confed_seq` `[64502]`, then
        `as_seq` `[64511]`.
    *   The DUT did not prepend `64501` or `64500` to any of these AS_PATHs.
    *   Flows `v4-p2-to-p3` and `v6-p2-to-p3` meet the
        [traffic criteria](#traffic-passfail-criteria-applies-to-every-flow-check).
*   **Fail**: A prefix is missing on the DUT or `ATE:port2`, the AS_PATH
    received by `ATE:port2` differs from the AS_PATH received by the DUT (for
    example, `64501` was prepended), or traffic loss is observed.

---

### RT-1.111.3: iBGP Peer to Confederation Peer (AS_CONFED_SEQUENCE Prepend and Attribute Preservation)

RFC 5065 Section 4.1, rule b: when a speaker advertises a route to a peer in a
neighboring Member-AS of the same confederation, it prepends its own Member-AS
number in an `AS_CONFED_SEQUENCE` segment.

*   `IBGP` has an empty AS_PATH, so the DUT must create a new
    `AS_CONFED_SEQUENCE` containing `64501` (rule b.3). Expected AS_PATH:
    `AS_CONFED_SEQUENCE [64501]`.
*   `IBGP-CONFED` already starts with `AS_CONFED_SEQUENCE [64503]`, so the DUT
    must prepend `64501` to that segment (rule b.1). Expected AS_PATH:
    `AS_CONFED_SEQUENCE [64501, 64503]` (one segment, `64501` leftmost).

This subtest also checks the path attributes that RFC 5065 allows to cross a
Member-AS boundary, using the route `IBGP`:

*   **LOCAL_PREF**: RFC 5065 Section 5.2 removes the restriction against
    sending LOCAL_PREF to a peer in a neighboring Member-AS. This test
    requires the DUT to send the received LOCAL_PREF (`200`) unchanged.
*   **MED**: RFC 5065 Section 5.2 states that it SHALL be legal to advertise
    an unchanged MED to a peer in a neighboring Member-AS. This test requires
    the DUT to send the received MED (`50`) unchanged.
*   **NEXT_HOP**: RFC 5065 Section 5.2 states that it SHALL be legal to
    advertise an unchanged NEXT_HOP to a peer in a neighboring Member-AS, and
    Section 5.1 states that by default the NEXT_HOP is unchanged (it may be
    changed by policy). Because the RFC permits but does not require an
    unchanged NEXT_HOP, this test accepts either behavior: the NEXT_HOP
    received from `ATE:port2` unchanged (`192.0.2.6` / `2001:db8::6`, the
    expected default), or the DUT's own address on Link 3 (`192.0.2.9` /
    `2001:db8::9`, next-hop-self). The test logs which behavior was
    observed. Optionally, a follow-up step could pin the NEXT_HOP with an
    export policy on `CONFED-EBGP` and require the pinned value; this is not
    part of this test.

NOTE: RFC 5065 permits, but does not mandate, sending LOCAL_PREF and MED
unchanged. The LOCAL_PREF and MED expectations above are what this test
requires of the DUT's default behavior for confederation-eBGP sessions.

#### Step 1: Verify DUT RIB

1.  On the DUT, find the `adj-rib-in-post` routes for `198.51.100.0/26` and
    `198.51.100.192/26` from neighbor `192.0.2.6`, and for
    `2001:db8:200::/48` and `2001:db8:201::/48` from neighbor `2001:db8::6`.
    Follow `state/attr-index` to the `attr-set` and read:
    *   `/network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/local-pref`
    *   `/network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med`
    *   `/network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/next-hop`
    *   `/network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/type`
    *   `/network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/member`

#### Step 2: Verify Routes Received by `ATE:port3`

1.  On `ATE:port3`, read the OTG BGP prefix state for `198.51.100.0/26`,
    `2001:db8:200::/48`, `198.51.100.192/26`, and `2001:db8:201::/48` and
    inspect the AS_PATH segments, `local_preference`,
    `multi_exit_discriminator`, and `ipv4_next_hop` / `ipv6_next_hop`.

#### Step 3: Verify Traffic

1.  Start flows `v4-p3-to-p2` and `v6-p3-to-p2` for 30 seconds, stop them, and
    check the counters.

#### RT-1.111.3 Pass/Fail Criteria

*   **Pass**:
    *   On the DUT, the `attr-set` for `198.51.100.0/26` and
        `2001:db8:200::/48` has `local-pref` `200`, `med` `50`, `next-hop`
        `192.0.2.6` (IPv4) or `2001:db8::6` (IPv6), and no AS_PATH segments.
    *   On the DUT, the `attr-set` for `198.51.100.192/26` and
        `2001:db8:201::/48` has one `AS_CONFED_SEQUENCE` segment with members
        `[64503]`.
    *   `ATE:port3` receives `198.51.100.0/26` and `2001:db8:200::/48` with an
        AS_PATH of exactly one segment of type `as_confed_seq` with
        `as_numbers` `[64501]` (rule b.3).
    *   `ATE:port3` receives `198.51.100.192/26` and `2001:db8:201::/48` with
        an AS_PATH of exactly one segment of type `as_confed_seq` with
        `as_numbers` `[64501, 64503]` in that order (rule b.1).
    *   `ATE:port3` reports `local_preference` `200` and
        `multi_exit_discriminator` `50` for `198.51.100.0/26` and
        `2001:db8:200::/48`.
    *   `ATE:port3` reports `ipv4_next_hop` for `198.51.100.0/26` equal to
        either `192.0.2.6` (unchanged) or `192.0.2.9` (next-hop-self), and
        `ipv6_next_hop` for `2001:db8:200::/48` equal to either `2001:db8::6`
        or `2001:db8::9`. The observed behavior is logged.
    *   Flows `v4-p3-to-p2` and `v6-p3-to-p2` meet the
        [traffic criteria](#traffic-passfail-criteria-applies-to-every-flow-check).
*   **Fail**: A prefix is missing on `ATE:port3`; an AS_PATH differs from the
    expected value (for example, `64501` is sent as an `AS_SEQ`, `64500`
    appears, or `64501` is placed in a new segment instead of being prepended
    to `[64503]`); LOCAL_PREF is missing or not `200`; MED is missing or not
    `50`; the NEXT_HOP is neither of the two accepted values; or traffic loss
    is observed.

---

### RT-1.111.4: Routes from Inside the Confederation to the External Peer (Confederation Segments Removed)

RFC 5065 Section 4.1, rule c: when a speaker advertises a route to a peer
outside the confederation, it MUST remove all `AS_CONFED_SEQUENCE` and
`AS_CONFED_SET` segments (rule c.1) and then prepend its Confederation
Identifier as an `AS_SEQ` (rules c.2 and c.4). RFC 5065 Section 5 also states
that a speaker MUST NOT send `AS_CONFED_SET` or `AS_CONFED_SEQUENCE` to peers
that are not members of the local confederation.

| Route | AS_PATH received by DUT | Rule | Expected AS_PATH at `ATE:port1` |
| :--- | :--- | :--- | :--- |
| `CONFED` | `AS_CONFED_SEQUENCE [64502]` | c.1, c.4 | `AS_SEQ [64500]` |
| `IBGP` | Empty | c.4 | `AS_SEQ [64500]` |
| `IBGP-CONFED` | `AS_CONFED_SEQUENCE [64503]` | c.1, c.4 | `AS_SEQ [64500]` |
| `CONFED-TRANSIT` | `AS_CONFED_SEQUENCE [64502]`, `AS_SEQ [64511]` | c.1, c.2 | `AS_SEQ [64500, 64511]` (one segment) |

RFC 4271 Section 5.1.5 states that a speaker MUST NOT include LOCAL_PREF in
UPDATE messages sent to external peers (except within a confederation), so
`ATE:port1` must not receive LOCAL_PREF even though the DUT received LOCAL_PREF
`200` for `IBGP` and `100` for `IBGP-CONFED`.

#### Step 1: Verify Routes Received by `ATE:port1`

1.  On `ATE:port1`, read the OTG BGP prefix state for `198.51.100.64/26`,
    `2001:db8:300::/48`, `198.51.100.0/26`, `2001:db8:200::/48`,
    `198.51.100.192/26`, `2001:db8:201::/48`, `203.0.113.128/26`, and
    `2001:db8:302::/48`, and inspect the AS_PATH segments,
    `local_preference`, and the next hop.

#### Step 2: Verify Traffic

1.  Start flows `v4-p1-to-p3`, `v6-p1-to-p3`, `v4-p1-to-p2`, and `v6-p1-to-p2`
    for 30 seconds, stop them, and check the counters.

#### RT-1.111.4 Pass/Fail Criteria

*   **Pass**:
    *   `ATE:port1` receives all eight prefixes with the AS_PATH given in the
        table above: exactly one segment of type `as_seq`, with `as_numbers`
        `[64500]` for `CONFED`, `IBGP`, and `IBGP-CONFED`, and `[64500, 64511]`
        for `CONFED-TRANSIT`.
    *   No `as_confed_seq` or `as_confed_set` segment is present, and none of
        `64501`, `64502`, or `64503` appears anywhere in the AS_PATH.
    *   `local_preference` is not present (unset) for any of the eight
        prefixes.
    *   The received next hop is the DUT address on Link 1: `ipv4_next_hop`
        `192.0.2.1` for IPv4 prefixes, and the global IPv6 next hop
        (`ipv6_next_hop`) `2001:db8::1` for IPv6 prefixes. A link-local IPv6
        next hop, if also sent, is not checked.
    *   The four flows meet the
        [traffic criteria](#traffic-passfail-criteria-applies-to-every-flow-check).
*   **Fail**: A prefix is missing on `ATE:port1`; the AS_PATH contains any
    confederation segment or Member-AS number; the AS_PATH differs from the
    expected value (for example, `64500` is placed in a new segment instead of
    being prepended to `[64511]`); LOCAL_PREF is received; or traffic loss is
    observed.

---

### RT-1.111.5: External Route to Confederation and iBGP Peers

The route `EXT` arrives from external AS `64510` with `AS_SEQ [64510]`.

*   Toward the confederation peer on `DUT:port3`, the first segment is not an
    `AS_CONFED_SEQUENCE`, so the DUT prepends a new `AS_CONFED_SEQUENCE`
    segment containing `64501` (RFC 5065 Section 4.1, rule b.2). The expected
    AS_PATH is `AS_CONFED_SEQUENCE [64501]`, `AS_SEQ [64510]`.
*   Toward the iBGP peer on `DUT:port2`, the AS_PATH is not modified (rule a).
    The expected AS_PATH is `AS_SEQ [64510]`.

#### Step 1: Verify DUT RIB

1.  On the DUT, confirm that `203.0.113.0/26` and `2001:db8:100::/48` are in
    the `loc-rib` and that the referenced `attr-set` has one `AS_SEQ` segment
    with members `[64510]`.

#### Step 2: Verify Routes Received by `ATE:port3` and `ATE:port2`

1.  On `ATE:port3` and on `ATE:port2`, read the OTG BGP prefix state for
    `203.0.113.0/26` and `2001:db8:100::/48` and inspect the AS_PATH
    segments.

#### Step 3: Verify Traffic

1.  Start flows `v4-p3-to-p1`, `v6-p3-to-p1`, `v4-p2-to-p1`, and `v6-p2-to-p1`
    for 30 seconds, stop them, and check the counters.

#### RT-1.111.5 Pass/Fail Criteria

*   **Pass**:
    *   On the DUT, both prefixes are in the `loc-rib` with AS_PATH
        `AS_SEQ [64510]`.
    *   `ATE:port3` receives both prefixes with an AS_PATH of exactly two
        segments, in this order: `as_confed_seq` with `as_numbers` `[64501]`,
        then `as_seq` with `as_numbers` `[64510]`.
    *   `ATE:port2` receives both prefixes with an AS_PATH of exactly one
        segment of type `as_seq` with `as_numbers` `[64510]`.
    *   The four flows meet the
        [traffic criteria](#traffic-passfail-criteria-applies-to-every-flow-check).
*   **Fail**: A prefix is missing; `ATE:port3` receives `64501` as an `as_seq`
    instead of an `as_confed_seq`, or receives `64500`; `ATE:port2` receives
    any AS other than `64510`; or traffic loss is observed.

---

### RT-1.111.6: AS Loop Detection with Member-AS and Confederation Identifier (Negative)

RFC 5065 Section 4 states that a speaker receiving an AS_PATH with an
`AS_CONFED_SEQUENCE` or `AS_CONFED_SET` that contains its own Member-AS number,
or an AS_PATH that contains its own Confederation Identifier, SHALL treat the
path as if it contained its own AS number. A path containing the local AS is
an AS loop and is not selected or propagated (RFC 4271 Section 9.1.2). This is
not a protocol error, so the BGP sessions must stay up.

#### Step 1: Advertise Looped Routes

1.  Record the OTG `routes_advertised` counters of the `ATE:port1` and
    `ATE:port3` IPv4 and IPv6 peers.
2.  On `ATE:port3`, add the route range `CONFED-LOOP` from Table 3
    (`AS_CONFED_SEQUENCE [64502, 64501]`).
3.  On `ATE:port1`, add the route range `EXT-LOOP` from Table 3
    (`AS_SEQ [64510, 64500]`).
4.  Push the updated ATE configuration and advertise the new route ranges.
5.  Wait until each of the four `routes_advertised` counters has increased by
    `1` (positive control, see
    [Absence Check Procedure](#absence-check-procedure-applies-to-rt-11116-and-rt-11117)).

#### Step 2: Verify the Looped Routes Are Rejected

1.  For the hold window of 30 seconds, check the DUT `loc-rib` for
    `198.51.100.128/26`, `2001:db8:301::/48`, `203.0.113.64/26`, and
    `2001:db8:101::/48`.
2.  During the same window, check the OTG BGP prefix state on `ATE:port1`,
    `ATE:port2`, and `ATE:port3` for the same four prefixes.
3.  Check that the six BGP sessions are still `ESTABLISHED` and that the
    base prefixes (`EXT`, `IBGP`, `IBGP-CONFED`, `CONFED`, and
    `CONFED-TRANSIT`) are still in the DUT `loc-rib`.

#### RT-1.111.6 Pass/Fail Criteria

*   **Pass**:
    *   The positive control passed (all four `routes_advertised` counters
        increased by `1`).
    *   None of `198.51.100.128/26`, `2001:db8:301::/48`, `203.0.113.64/26`,
        or `2001:db8:101::/48` appears in the DUT `loc-rib` during the hold
        window.
    *   None of these four prefixes is received by `ATE:port1`, `ATE:port2`,
        or `ATE:port3` from the DUT during the hold window.
    *   All six BGP sessions remain `ESTABLISHED`, and all base prefixes
        remain in the DUT `loc-rib`.
*   **Fail**: The positive control fails (the result is then inconclusive and
    the subtest fails), any looped prefix is installed in the DUT `loc-rib`
    or advertised to any ATE port, any session leaves `ESTABLISHED`, or a
    base prefix is lost.

After this subtest, withdraw `CONFED-LOOP` and `EXT-LOOP` from the ATE.

---

### RT-1.111.7: Malformed AS_PATH from External and Confederation Peers (Negative)

RFC 5065 Section 5 defines two errors:

*   It is an error to receive an UPDATE whose AS_PATH contains
    `AS_CONFED_SEQUENCE` or `AS_CONFED_SET` segments from a neighbor that is
    not in the same confederation. The route `EXT-MALFORMED` from
    `ATE:port1` (`AS_CONFED_SEQUENCE [64510]`, `AS_SEQ [64510]`) is such an
    UPDATE.
*   It is an error to receive an UPDATE from a confederation peer in a
    different Member-AS that does not have `AS_CONFED_SEQUENCE` as the first
    segment. The route `CONFED-MALFORMED` from `ATE:port3` (`AS_SEQ [64502]`)
    is such an UPDATE.

In both cases RFC 5065 Section 5 requires the speaker to treat the UPDATE as
having a malformed AS_PATH according to RFC 4271 Section 6.3. RFC 7606, which
updates the error handling of RFC 4271, states in Section 7.2 that an UPDATE
message with a malformed AS_PATH attribute SHALL be handled using the
"treat-as-withdraw" approach. The expected behavior is therefore that the
route is discarded (treated as withdrawn) and the BGP session stays up. If an
implementation resets the session instead, that behavior must be documented
as a deviation.

#### Step 1: Advertise Malformed Routes

1.  Record the OTG `routes_advertised` counters of the `ATE:port1` and
    `ATE:port3` IPv4 and IPv6 peers.
2.  On `ATE:port1`, add the route range `EXT-MALFORMED` from Table 3.
3.  On `ATE:port3`, add the route range `CONFED-MALFORMED` from Table 3.
4.  Push the updated ATE configuration and advertise the new route ranges.
5.  Wait until each of the four `routes_advertised` counters has increased by
    `1` (positive control).

#### Step 2: Verify the Malformed Routes Are Not Accepted

1.  For the hold window of 30 seconds, check the DUT `adj-rib-in-post` of the
    sending neighbor and the DUT `loc-rib` for `203.0.113.192/27`,
    `2001:db8:102::/48`, `203.0.113.224/27`, and `2001:db8:303::/48`.
2.  During the same window, check the OTG BGP prefix state on `ATE:port1`,
    `ATE:port2`, and `ATE:port3` for the same four prefixes.
3.  During the same window, check that `session-state` of all six neighbors
    stays `ESTABLISHED`, and that the base prefixes are still in the DUT
    `loc-rib`.

#### RT-1.111.7 Pass/Fail Criteria

*   **Pass**:
    *   The positive control passed.
    *   None of `203.0.113.192/27`, `2001:db8:102::/48`, `203.0.113.224/27`,
        or `2001:db8:303::/48` appears in the DUT `adj-rib-in-post` or
        `loc-rib` during the hold window.
    *   None of these four prefixes is received by any ATE port from the DUT.
    *   All six BGP sessions remain `ESTABLISHED` (treat-as-withdraw), and
        all base prefixes remain in the DUT `loc-rib`.
*   **Fail**: The positive control fails; any malformed prefix is accepted
    into `adj-rib-in-post` or `loc-rib`, or advertised to any ATE port; or a
    base prefix is lost. A session reset (the session leaves `ESTABLISHED`)
    is a failure unless it is covered by a documented deviation.

After this subtest, withdraw `EXT-MALFORMED` and `CONFED-MALFORMED` from the
ATE.

---

### Cleanup

1.  Stop all traffic flows and BGP protocols on the ATE.
2.  On the DUT, remove the BGP configuration added by this test (six
    neighbors, the three peer-groups, the confederation `identifier` and
    `member-as` configuration, and the global BGP configuration) using gNMI
    `Set`.
3.  On the DUT, remove the routing policy `ALLOW` and the IPv4 and IPv6
    addresses on `DUT:port1`, `DUT:port2`, and `DUT:port3`, restoring the DUT
    to its baseline configuration.

## Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "DUT port1 to ATE port1 (external eBGP)",
          "enabled": true,
          "name": "eth1",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "eth1",
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
          "description": "DUT port2 to ATE port2 (iBGP)",
          "enabled": true,
          "name": "eth2",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "eth2",
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
          "description": "DUT port3 to ATE port3 (confederation eBGP)",
          "enabled": true,
          "name": "eth3",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "eth3",
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
                  "confederation": {
                    "config": {
                      "identifier": 64500,
                      "member-as": [
                        64502
                      ]
                    }
                  },
                  "config": {
                    "as": 64501,
                    "router-id": "192.0.2.254"
                  }
                },
                "neighbors": {
                  "neighbor": [
                    {
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
                              "enabled": false
                            }
                          }
                        ]
                      },
                      "config": {
                        "enabled": true,
                        "neighbor-address": "192.0.2.2",
                        "peer-as": 64510,
                        "peer-group": "EBGP-EXT"
                      },
                      "neighbor-address": "192.0.2.2"
                    },
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                              "enabled": false
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
                        "enabled": true,
                        "neighbor-address": "2001:db8::2",
                        "peer-as": 64510,
                        "peer-group": "EBGP-EXT"
                      },
                      "neighbor-address": "2001:db8::2"
                    },
                    {
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
                              "enabled": false
                            }
                          }
                        ]
                      },
                      "config": {
                        "enabled": true,
                        "neighbor-address": "192.0.2.6",
                        "peer-as": 64501,
                        "peer-group": "IBGP-MEMBER"
                      },
                      "neighbor-address": "192.0.2.6"
                    },
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                              "enabled": false
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
                        "enabled": true,
                        "neighbor-address": "2001:db8::6",
                        "peer-as": 64501,
                        "peer-group": "IBGP-MEMBER"
                      },
                      "neighbor-address": "2001:db8::6"
                    },
                    {
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
                              "enabled": false
                            }
                          }
                        ]
                      },
                      "config": {
                        "enabled": true,
                        "neighbor-address": "192.0.2.10",
                        "peer-as": 64502,
                        "peer-group": "CONFED-EBGP"
                      },
                      "neighbor-address": "192.0.2.10"
                    },
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                              "enabled": false
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
                        "enabled": true,
                        "neighbor-address": "2001:db8::a",
                        "peer-as": 64502,
                        "peer-group": "CONFED-EBGP"
                      },
                      "neighbor-address": "2001:db8::a"
                    }
                  ]
                },
                "peer-groups": {
                  "peer-group": [
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ALLOW"
                                ],
                                "import-policy": [
                                  "ALLOW"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                              "enabled": true
                            }
                          },
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ALLOW"
                                ],
                                "import-policy": [
                                  "ALLOW"
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
                        "peer-as": 64510,
                        "peer-group-name": "EBGP-EXT"
                      },
                      "peer-group-name": "EBGP-EXT"
                    },
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ALLOW"
                                ],
                                "import-policy": [
                                  "ALLOW"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                              "enabled": true
                            }
                          },
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ALLOW"
                                ],
                                "import-policy": [
                                  "ALLOW"
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
                        "peer-as": 64501,
                        "peer-group-name": "IBGP-MEMBER"
                      },
                      "peer-group-name": "IBGP-MEMBER"
                    },
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ALLOW"
                                ],
                                "import-policy": [
                                  "ALLOW"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                              "enabled": true
                            }
                          },
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ALLOW"
                                ],
                                "import-policy": [
                                  "ALLOW"
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
                        "peer-as": 64502,
                        "peer-group-name": "CONFED-EBGP"
                      },
                      "peer-group-name": "CONFED-EBGP"
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
            "name": "ALLOW"
          },
          "name": "ALLOW",
          "statements": {
            "statement": [
              {
                "actions": {
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                },
                "config": {
                  "name": "accept-all"
                },
                "name": "accept-all"
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
  ## Config Parameter Coverage
  /network-instances/network-instance/protocols/protocol/bgp/global/config/as:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/global/confederation/config/identifier:
  /network-instances/network-instance/protocols/protocol/bgp/global/confederation/config/member-as:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/neighbor-address:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-group:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-as:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-group-name:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/export-policy:
  /routing-policy/policy-definitions/policy-definition/config/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/config/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result:

  ## Telemetry Parameter Coverage
  /network-instances/network-instance/protocols/protocol/bgp/global/state/as:
  /network-instances/network-instance/protocols/protocol/bgp/global/confederation/state/identifier:
  /network-instances/network-instance/protocols/protocol/bgp/global/confederation/state/member-as:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/prefix:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/attr-index:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/state/prefix:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/state/attr-index:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/prefix:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/attr-index:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/prefix:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/attr-index:
  /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/local-pref:
  /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med:
  /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/next-hop:
  /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/index:
  /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/type:
  /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/member:

rpcs:
  gnmi:
    gNMI.Set:
      replace: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

*   vRX
