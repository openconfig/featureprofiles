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
on three DUT ports, so that every AS_PATH modification rule of RFC 5065
Section 4.1 can be observed on one device:

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
2.  **Confederation peer to iBGP peer**: A route received from Member-AS
    `64502` is advertised to the iBGP peer with the AS_PATH unchanged
    (RFC 5065 Section 4.1, rule a).
3.  **iBGP peer to confederation peer**: A route received from the iBGP peer is
    advertised to Member-AS `64502` with the DUT's Member-AS `64501` in an
    `AS_CONFED_SEQUENCE` segment (RFC 5065 Section 4.1, rule b), and
    LOCAL_PREF, MED, and NEXT_HOP are carried across the Member-AS boundary
    (RFC 5065 Sections 5.1 and 5.2).
4.  **Confederation boundary toward an external peer**: Routes learned inside
    the confederation are advertised to the external peer with all
    `AS_CONFED_SEQUENCE` / `AS_CONFED_SET` segments removed and the
    Confederation Identifier `64500` prepended as an `AS_SEQ` (RFC 5065
    Section 4.1, rule c, and Section 5). Member-AS numbers `64501` and `64502`
    are never visible to the external peer.
5.  **External peer into the confederation**: A route from external AS `64510`
    is advertised to the confederation peer with `AS_CONFED_SEQUENCE [64501]`
    followed by `AS_SEQ [64510]`, and to the iBGP peer with `AS_SEQ [64510]`.
6.  **AS loop detection (negative)**: The DUT rejects a route whose
    `AS_CONFED_SEQUENCE` contains its own Member-AS `64501`, and a route whose
    `AS_SEQ` contains its own Confederation Identifier `64500` (RFC 5065
    Section 4).

Data plane forwarding is verified with ATE-to-ATE traffic flows in both
directions between every pair of ATE ports, for both IPv4 and IPv6.

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
the UPDATE received by the DUT.

| Name | Advertised by | IPv4 prefix | IPv6 prefix | AS_PATH sent by ATE | Other attributes |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `EXT` | `ATE:port1` | `203.0.113.0/26` | `2001:db8:100::/48` | `AS_SEQ [64510]` | None |
| `IBGP` | `ATE:port2` | `198.51.100.0/26` | `2001:db8:200::/48` | Empty | LOCAL_PREF `200`, MED `50` |
| `CONFED` | `ATE:port3` | `198.51.100.64/26` | `2001:db8:300::/48` | `AS_CONFED_SEQUENCE [64502]` | None |
| `CONFED-LOOP` | `ATE:port3` | `198.51.100.128/26` | `2001:db8:301::/48` | `AS_CONFED_SEQUENCE [64502, 64501]` | None (RT-1.111.6 only) |
| `EXT-LOOP` | `ATE:port1` | `203.0.113.64/26` | `2001:db8:101::/48` | `AS_SEQ [64510, 64500]` | None (RT-1.111.6 only) |

The ATE is not a native confederation speaker. The `AS_CONFED_SEQUENCE`
segments are crafted using the OTG route range `as_path` configuration:

*   `EXT`: `as_set_mode` = `include_as_seq` (the ATE prepends its local AS
    `64510` as an `AS_SEQ`), no extra segments.
*   `IBGP`: `as_set_mode` = `do_not_include_local_as`, no segments (empty
    AS_PATH, as sent by an originator to a peer in the same Member-AS per
    RFC 5065 Section 4.1). Set `advanced.local_preference` = `200` and
    `advanced.multi_exit_discriminator` = `50`.
*   `CONFED`: `as_set_mode` = `do_not_include_local_as` with one segment of
    type `as_confed_seq` and `as_numbers` = `[64502]`.
*   `CONFED-LOOP`: `as_set_mode` = `do_not_include_local_as` with one segment
    of type `as_confed_seq` and `as_numbers` = `[64502, 64501]`.
*   `EXT-LOOP`: `as_set_mode` = `do_not_include_local_as` with one segment of
    type `as_seq` and `as_numbers` = `[64510, 64500]`.

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
5.  Configure six neighbors. Each IPv4 neighbor has only `IPV4_UNICAST`
    enabled, and each IPv6 neighbor has only `IPV6_UNICAST` enabled:
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
8.  Configure the ATE route ranges `EXT`, `IBGP`, and `CONFED` from Table 3.
    Do not configure `CONFED-LOOP` and `EXT-LOOP` yet; they are added in
    RT-1.111.6.
9.  Configure the flows in Table 4, but do not start them yet.
10. Push the ATE configuration and start protocols. Wait for ARP and IPv6
    neighbor discovery to complete for all three links.

NOTE: OTG does not configure a remote AS for a BGP peer, so the ATE does not
itself check which AS the DUT uses in its OPEN message. The use of
Confederation Identifier `64500` toward `ATE:port1` is verified through the
AS_PATH received by `ATE:port1` in RT-1.111.4.

#### Traffic Pass/Fail Criteria (applies to every flow check)

*   **Pass**: For each flow, the ATE Rx packet count on the receiving port
    equals the Tx packet count (0% loss), and the Tx packet count is
    non-zero.
*   **Fail**: Any loss greater than 0%, or zero packets transmitted.

---

### RT-1.111.1: Confederation Configuration, State, and Session Establishment

This subtest checks that the DUT accepts the confederation configuration and
reports it in operational state, and that all three types of BGP sessions come
up.

#### Step 1: Verify Confederation State Telemetry

1.  Apply the base configuration from
    [Test environment setup](#test-environment-setup).
2.  Using gNMI `Subscribe` (`ONCE` or `SAMPLE`), read:
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
its own Member-AS, it SHALL NOT modify the AS_PATH. The route `CONFED` is
received from Member-AS `64502` on `DUT:port3` and advertised by the DUT to
the iBGP peer on `DUT:port2`.

#### Step 1: Verify DUT RIB

1.  On the DUT, find the `adj-rib-in-post` route for `198.51.100.64/26` from
    neighbor `192.0.2.10` and for `2001:db8:300::/48` from neighbor
    `2001:db8::a`, and follow `state/attr-index` to the referenced `attr-set`.
2.  Read the AS_PATH segments of that `attr-set`:
    *   `/network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/type`
    *   `/network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/member`
3.  Confirm that both prefixes are present in the DUT `loc-rib`.

#### Step 2: Verify Route Received by `ATE:port2`

1.  On `ATE:port2`, read the OTG BGP prefix state (`bgp-prefix` states for the
    IPv4 and IPv6 peers) for `198.51.100.64/26` and `2001:db8:300::/48`, and
    inspect the received AS_PATH segments.

#### Step 3: Verify Traffic

1.  Start flows `v4-p2-to-p3` and `v6-p2-to-p3` for 30 seconds, stop them, and
    check the counters.

#### RT-1.111.2 Pass/Fail Criteria

*   **Pass**:
    *   On the DUT, the AS_PATH for both prefixes is exactly one segment of
        type `AS_CONFED_SEQUENCE` with members `[64502]`.
    *   `ATE:port2` receives both prefixes with an AS_PATH of exactly one
        segment of type `as_confed_seq` with `as_numbers` `[64502]`. The DUT
        did not prepend `64501`, and no `as_seq` segment is present.
    *   Flows `v4-p2-to-p3` and `v6-p2-to-p3` meet the
        [traffic criteria](#traffic-passfail-criteria-applies-to-every-flow-check).
*   **Fail**: A prefix is missing on the DUT or `ATE:port2`, the AS_PATH
    received by `ATE:port2` differs from `AS_CONFED_SEQUENCE [64502]` (for
    example, `64501` was prepended), or traffic loss is observed.

---

### RT-1.111.3: iBGP Peer to Confederation Peer (AS_CONFED_SEQUENCE Prepend and Attribute Preservation)

RFC 5065 Section 4.1, rule b: when a speaker advertises a route to a peer in a
neighboring Member-AS of the same confederation, it prepends its own Member-AS
number in an `AS_CONFED_SEQUENCE` segment. The route `IBGP` has an empty
AS_PATH, so the DUT must create a new `AS_CONFED_SEQUENCE` containing `64501`
(rule b.3).

This subtest also checks the path attributes that RFC 5065 allows to cross a
Member-AS boundary:

*   **LOCAL_PREF**: RFC 5065 Section 5.2 removes the restriction against
    sending LOCAL_PREF to a peer in a neighboring Member-AS. This test
    requires the DUT to send the received LOCAL_PREF (`200`) unchanged.
*   **MED**: RFC 5065 Section 5.2 states that it SHALL be legal to advertise
    an unchanged MED to a peer in a neighboring Member-AS. This test requires
    the DUT to send the received MED (`50`) unchanged.
*   **NEXT_HOP**: RFC 5065 Section 5.2 states that it SHALL be legal to
    advertise an unchanged NEXT_HOP to a peer in a neighboring Member-AS, and
    Section 5.1 states that by default the NEXT_HOP is unchanged (it may be
    changed by policy). No next-hop policy is configured in this test, so the
    DUT is expected to send the NEXT_HOP it received from `ATE:port2`
    (`192.0.2.6` for IPv4, `2001:db8::6` for IPv6).

NOTE: RFC 5065 permits, but does not mandate, sending these attributes
unchanged. The expectations above are what this test requires of the DUT's
default behavior for confederation-eBGP sessions.

#### Step 1: Verify DUT RIB

1.  On the DUT, find the `adj-rib-in-post` route for `198.51.100.0/26` from
    neighbor `192.0.2.6` and for `2001:db8:200::/48` from neighbor
    `2001:db8::6`. Follow `state/attr-index` to the `attr-set` and read:
    *   `/network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/local-pref`
    *   `/network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med`
    *   `/network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/next-hop`

#### Step 2: Verify Route Received by `ATE:port3`

1.  On `ATE:port3`, read the OTG BGP prefix state for `198.51.100.0/26` and
    `2001:db8:200::/48` and inspect the AS_PATH segments, `local_preference`,
    `multi_exit_discriminator`, and `ipv4_next_hop` / `ipv6_next_hop`.

#### Step 3: Verify Traffic

1.  Start flows `v4-p3-to-p2` and `v6-p3-to-p2` for 30 seconds, stop them, and
    check the counters.

#### RT-1.111.3 Pass/Fail Criteria

*   **Pass**:
    *   On the DUT, the `attr-set` for both prefixes has `local-pref` `200`
        and `med` `50`, and `next-hop` `192.0.2.6` (IPv4) or `2001:db8::6`
        (IPv6).
    *   `ATE:port3` receives both prefixes with an AS_PATH of exactly one
        segment of type `as_confed_seq` with `as_numbers` `[64501]`.
    *   `ATE:port3` reports `local_preference` `200` and
        `multi_exit_discriminator` `50` for both prefixes.
    *   `ATE:port3` reports `ipv4_next_hop` `192.0.2.6` for `198.51.100.0/26`
        and `ipv6_next_hop` `2001:db8::6` for `2001:db8:200::/48`.
    *   Flows `v4-p3-to-p2` and `v6-p3-to-p2` meet the
        [traffic criteria](#traffic-passfail-criteria-applies-to-every-flow-check).
*   **Fail**: A prefix is missing on `ATE:port3`; the AS_PATH is not
    `AS_CONFED_SEQUENCE [64501]` (for example, it is an `AS_SEQ`, or `64500`
    appears); LOCAL_PREF is missing or not `200`; MED is missing or not `50`;
    the NEXT_HOP differs from the value received from `ATE:port2`; or traffic
    loss is observed.

---

### RT-1.111.4: Confederation and iBGP Routes to External Peer (Confederation Segments Removed)

RFC 5065 Section 4.1, rule c: when a speaker advertises a route to a peer
outside the confederation, it MUST remove all `AS_CONFED_SEQUENCE` and
`AS_CONFED_SET` segments and then prepend its Confederation Identifier as an
`AS_SEQ`. RFC 5065 Section 5 also states that a speaker MUST NOT send
`AS_CONFED_SET` or `AS_CONFED_SEQUENCE` to peers that are not members of the
local confederation.

*   `CONFED` arrives with `AS_CONFED_SEQUENCE [64502]`. After removal the
    AS_PATH is empty, so the DUT sends `AS_SEQ [64500]` (rule c.4).
*   `IBGP` arrives with an empty AS_PATH, so the DUT sends `AS_SEQ [64500]`
    (rule c.4).

#### Step 1: Verify Routes Received by `ATE:port1`

1.  On `ATE:port1`, read the OTG BGP prefix state for `198.51.100.64/26`,
    `2001:db8:300::/48`, `198.51.100.0/26`, and `2001:db8:200::/48`, and
    inspect the AS_PATH segments and the next hop.

#### Step 2: Verify Traffic

1.  Start flows `v4-p1-to-p3`, `v6-p1-to-p3`, `v4-p1-to-p2`, and `v6-p1-to-p2`
    for 30 seconds, stop them, and check the counters.

#### RT-1.111.4 Pass/Fail Criteria

*   **Pass**:
    *   `ATE:port1` receives all four prefixes, each with an AS_PATH of
        exactly one segment of type `as_seq` with `as_numbers` `[64500]`.
    *   No `as_confed_seq` or `as_confed_set` segment is present, and neither
        `64501` nor `64502` appears anywhere in the AS_PATH.
    *   The received next hop is the DUT address on Link 1 (`192.0.2.1` for
        IPv4 prefixes, `2001:db8::1` for IPv6 prefixes), as for any eBGP
        advertisement from the DUT.
    *   The four flows meet the
        [traffic criteria](#traffic-passfail-criteria-applies-to-every-flow-check).
*   **Fail**: A prefix is missing on `ATE:port1`; the AS_PATH contains any
    confederation segment, `64501`, or `64502`; the AS_PATH is not
    `AS_SEQ [64500]`; or traffic loss is observed.

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

1.  On `ATE:port3`, add the route range `CONFED-LOOP` from Table 3
    (`AS_CONFED_SEQUENCE [64502, 64501]`).
2.  On `ATE:port1`, add the route range `EXT-LOOP` from Table 3
    (`AS_SEQ [64510, 64500]`).
3.  Push the updated ATE configuration and advertise the new route ranges.

#### Step 2: Verify the Looped Routes Are Rejected

1.  On the DUT, check the `loc-rib` for `198.51.100.128/26`,
    `2001:db8:301::/48`, `203.0.113.64/26`, and `2001:db8:101::/48`.
2.  On `ATE:port1`, `ATE:port2`, and `ATE:port3`, check the OTG BGP prefix
    state for the same four prefixes.
3.  Check that the six BGP sessions are still `ESTABLISHED` and that the
    valid prefixes `EXT`, `IBGP`, and `CONFED` are still in the DUT
    `loc-rib`.

#### RT-1.111.6 Pass/Fail Criteria

*   **Pass**:
    *   None of `198.51.100.128/26`, `2001:db8:301::/48`, `203.0.113.64/26`,
        or `2001:db8:101::/48` is present in the DUT `loc-rib`.
    *   None of these four prefixes is received by `ATE:port1`, `ATE:port2`,
        or `ATE:port3` from the DUT.
    *   All six BGP sessions remain `ESTABLISHED`, and the `EXT`, `IBGP`, and
        `CONFED` prefixes remain in the DUT `loc-rib`.
*   **Fail**: Any looped prefix is installed in the DUT `loc-rib` or
    advertised to any ATE port, any session leaves `ESTABLISHED`, or a valid
    prefix is lost.

After this subtest, withdraw `CONFED-LOOP` and `EXT-LOOP` from the ATE.

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
  /network-instances/network-instance/protocols/protocol/bgp/global/confederation/config/identifier:
  /network-instances/network-instance/protocols/protocol/bgp/global/confederation/config/member-as:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/neighbor-address:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-group:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-as:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-group-name:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/export-policy:

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
