# RT-1.111: BGP Autonomous System Confederations (RFC 5065)

## Summary

Validate BGP Autonomous System Confederations
([RFC 5065](https://datatracker.ietf.org/doc/html/rfc5065)) on a single DUT
that is Member-AS `64501` of confederation `64500`. The ATE emulates the three
kinds of BGP neighbors a confederation member can have: an external eBGP peer,
an iBGP peer in the same Member-AS, and a confederation-eBGP peer in a
neighboring Member-AS. The test covers, for IPv4 and IPv6 unicast:

*   Confederation configuration and state telemetry
*   AS_PATH handling toward each kind of peer (RFC 5065 Section 4.1, rules a,
    b and c)
*   LOCAL_PREF, MED and NEXT_HOP handling across the Member-AS boundary
    (RFC 5065 Sections 5.1 and 5.2, RFC 4271 Section 5.1.5)
*   Rejection of AS_PATHs that contain the DUT's own Member-AS or
    Confederation Identifier (loop detection) and of malformed confederation
    AS_PATHs (RFC 5065 Section 5, RFC 7606 Section 7.2), without session
    resets
*   ATE-to-ATE traffic over the learned routes

Not covered (candidates for follow-up tests): `AS_SET` / `AS_CONFED_SET`
segments, routes originated by the DUT, NEXT_HOP pinning by export policy,
path selection rules of RFC 5065 Section 5.3, and `NO_EXPORT` /
`NO_EXPORT_SUBCONFED` communities at the confederation boundary.

## Testbed type

*   [TESTBED_DUT_ATE_4LINKS](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)
    (`port4` is not used)

## Procedure

### Test environment setup

*   Connect DUT port 1, 2 and 3 to ATE port 1, 2 and 3 respectively

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

*   Configure IPv4/IPv6 addresses on the interfaces (RFC 5737 / RFC 3849
    ranges; they do not overlap any advertised prefix)

    | Link   | DUT port                           | ATE port                            |
    | :----- | :--------------------------------- | :---------------------------------- |
    | Link 1 | `192.0.2.1/30`, `2001:db8::1/126`  | `192.0.2.2/30`, `2001:db8::2/126`   |
    | Link 2 | `192.0.2.5/30`, `2001:db8::5/126`  | `192.0.2.6/30`, `2001:db8::6/126`   |
    | Link 3 | `192.0.2.9/30`, `2001:db8::9/126`  | `192.0.2.10/30`, `2001:db8::a/126`  |

*   Configure BGP on the DUT in the `DEFAULT` network instance
    *   `global/config/as` = `64501` (Member-AS), `router-id` = `192.0.2.254`
        (static value, not assigned to any interface)
    *   `global/confederation/config/identifier` = `64500`
    *   `global/confederation/config/member-as` = `[64502]`
    *   Routing policy `ALLOW` with one statement, `policy-result` =
        `ACCEPT_ROUTE`
    *   Peer-groups `EBGP-EXT` (`peer-as` `64510`), `IBGP-MEMBER` (`peer-as`
        `64501`) and `CONFED-EBGP` (`peer-as` `64502`), each with
        `IPV4_UNICAST` and `IPV6_UNICAST` enabled and `ALLOW` as import and
        export policy
    *   One IPv4 and one IPv6 neighbor per peer-group (ATE addresses in the
        table above). Each neighbor enables only its own address family and
        disables the other one
    *   Push with gNMI `Set` `REPLACE` (see [Canonical OC](#canonical-oc))
*   Establish BGP sessions between:
    *   ATE port-1 (AS `64510`, OTG `ebgp`) and DUT port-1: external eBGP. The
        DUT uses the Confederation Identifier `64500` toward this peer
    *   ATE port-2 (AS `64501`, OTG `ibgp`) and DUT port-2: iBGP in the same
        Member-AS
    *   ATE port-3 (AS `64502`, OTG `ebgp`) and DUT port-3: confederation-eBGP
        with the neighboring Member-AS
    *   One IPv4 and one IPv6 session per port; each OTG peer enables only its
        own address-family capability
    *   OTG does not configure or check the remote AS of a peer. The use of
        `64500` in the DUT OPEN message toward ATE port-1 is verified only
        through the AS_PATH received by ATE port-1 in RT-1.111.4
*   Advertise the following route ranges from the ATE. "AS_PATH sent by ATE" is
    the AS_PATH the DUT must receive. `64503` (another Member-AS) and `64511`
    (an external AS behind Member-AS `64502`) appear only inside AS_PATHs

    | Name               | ATE port | IPv4 prefix        | IPv6 prefix          | AS_PATH sent by ATE                         | LOCAL_PREF | MED      | Used in    |
    | :----------------- | :------- | :----------------- | :------------------- | :------------------------------------------ | :--------- | :------- | :--------- |
    | `EXT`              | port-1   | `203.0.113.0/26`   | `2001:db8:100::/48`  | `AS_SEQ [64510]`                            | not sent   | not sent | Base       |
    | `IBGP`             | port-2   | `198.51.100.0/26`  | `2001:db8:200::/48`  | empty                                       | `200`      | `50`     | Base       |
    | `IBGP-CONFED`      | port-2   | `198.51.100.192/26`| `2001:db8:201::/48`  | `AS_CONFED_SEQUENCE [64503]`                | `100`      | not sent | Base       |
    | `CONFED`           | port-3   | `198.51.100.64/26` | `2001:db8:300::/48`  | `AS_CONFED_SEQUENCE [64502]`                | `150`      | not sent | Base       |
    | `CONFED-TRANSIT`   | port-3   | `203.0.113.128/26` | `2001:db8:302::/48`  | `AS_CONFED_SEQUENCE [64502]`, `AS_SEQ [64511]` | not sent | not sent | Base       |
    | `CONFED-LOOP`      | port-3   | `198.51.100.128/26`| `2001:db8:301::/48`  | `AS_CONFED_SEQUENCE [64502, 64501]`         | not sent   | not sent | RT-1.111.6 |
    | `EXT-LOOP`         | port-1   | `203.0.113.64/26`  | `2001:db8:101::/48`  | `AS_SEQ [64510, 64500]`                     | not sent   | not sent | RT-1.111.6 |
    | `EXT-MALFORMED`    | port-1   | `203.0.113.192/27` | `2001:db8:102::/48`  | `AS_SEQ [64510]`, `AS_CONFED_SEQUENCE [64510]` | not sent | not sent | RT-1.111.7 |
    | `CONFED-MALFORMED` | port-3   | `203.0.113.224/27` | `2001:db8:303::/48`  | `AS_SEQ [64502]`                            | not sent   | not sent | RT-1.111.7 |

*   OTG settings for the route ranges
    *   The ATE is not a native confederation speaker. The OTG model requires
        `do_not_include_local_as` for iBGP sessions and any other
        `as_set_mode` for eBGP sessions, so on the `ebgp` peers the ATE always
        prepends its own AS and the `as_set_mode` selects how it is
        prepended: `include_as_confed_seq` prepends `64502` as an
        `AS_CONFED_SEQUENCE` (`CONFED`, `CONFED-TRANSIT`, `CONFED-LOOP`);
        `include_as_seq` prepends the local AS as an `AS_SEQ` (`EXT`,
        `EXT-LOOP`, `EXT-MALFORMED`, `CONFED-MALFORMED`). Additional segments
        are configured with `as_path.segments`: `as_seq [64511]` for
        `CONFED-TRANSIT`, `as_confed_seq [64501]` for `CONFED-LOOP`,
        `as_seq [64500]` for `EXT-LOOP`, `as_confed_seq [64510]` for
        `EXT-MALFORMED`
    *   On the OTG `ibgp` peer use `do_not_include_local_as`: no segments for
        `IBGP`, `as_confed_seq [64503]` for `IBGP-CONFED`
    *   Set `advanced.include_local_preference` and
        `advanced.include_multi_exit_discriminator` explicitly on every route
        range (OTG sends both by default); `local_preference` and
        `multi_exit_discriminator` as in the table
    *   The ATE may send `EXT-LOOP` as one `AS_SEQ [64510, 64500]` or as two
        adjacent `AS_SEQ` segments, and `CONFED-LOOP` as one or two adjacent
        `AS_CONFED_SEQUENCE` segments; both forms contain the looped AS and
        are rejected the same way. AS_PATHs sent by the DUT are compared
        segment by segment: `AS_CONFED_SEQUENCE [64501, 64503]` (RT-1.111.3)
        and `AS_SEQ [64500, 64511]` (RT-1.111.4) must each be one segment,
        because RFC 5065 Section 4.1 rules b.1 and c.2 prepend the AS into the
        existing first segment
*   Configure all nine route ranges in the single base ATE configuration and
    start protocols. Immediately afterwards withdraw `CONFED-LOOP`, `EXT-LOOP`,
    `EXT-MALFORMED` and `CONFED-MALFORMED` with OTG route control state
    (`gosnappi.StateProtocolRouteState.WITHDRAW`). Negative route ranges are
    only advertised and withdrawn with control state; a new ATE configuration
    is never pushed mid-test because `SetConfig` restarts the protocols
*   Wait for ARP and IPv6 neighbor discovery to resolve on all three links
    before RT-1.111.1
*   Configure ATE-to-ATE flows (frame size `512` bytes, `1000` pps, `30` s).
    Source and destination addresses are inside the advertised prefixes; no
    traffic is sent to a DUT address

    | Flows (IPv4 and IPv6)       | Source address                     | Destination address                | Reported with |
    | :-------------------------- | :--------------------------------- | :--------------------------------- | :------------ |
    | ATE port-2 to ATE port-3    | `198.51.100.1`, `2001:db8:200::1`  | `198.51.100.65`, `2001:db8:300::1` | RT-1.111.2 |
    | ATE port-3 to ATE port-2    | `198.51.100.65`, `2001:db8:300::1` | `198.51.100.1`, `2001:db8:200::1`  | RT-1.111.3 |
    | ATE port-1 to ATE port-3    | `203.0.113.1`, `2001:db8:100::1`   | `198.51.100.65`, `2001:db8:300::1` | RT-1.111.4 |
    | ATE port-1 to ATE port-2    | `203.0.113.1`, `2001:db8:100::1`   | `198.51.100.1`, `2001:db8:200::1`  | RT-1.111.4 |
    | ATE port-3 to ATE port-1    | `198.51.100.65`, `2001:db8:300::1` | `203.0.113.1`, `2001:db8:100::1`   | RT-1.111.5 |
    | ATE port-2 to ATE port-1    | `198.51.100.1`, `2001:db8:200::1`  | `203.0.113.1`, `2001:db8:100::1`   | RT-1.111.5 |

*   Common procedures used by the subtests
    *   Traffic check: pass if Tx packets > 0 and Rx packets equal Tx packets
        (0% loss) for every flow. All 12 flows run once, together, at the end
        of RT-1.111.5; each flow is reported with the subtest in the table
    *   DUT telemetry is read with gNMI `Subscribe` `ON_CHANGE` (`gnmi.Watch`
        / `gnmi.Await`); no static sleeps
    *   AS_PATH telemetry: `as-segment` `index` `0` is the leftmost segment;
        the `member` list is in AS_PATH order (most recently prepended AS
        first). An implementation that reports a different order or indexing
        must document this as a deviation
    *   Absence check (negative routes): record the OTG `routes_advertised`
        counter of the sending peer, advertise the route range, and wait up
        to `60` s for the counter to increase (positive control; if it does
        not increase the result is inconclusive and the subtest fails). Then
        the prefix must stay absent from the checked location for a hold
        window of `30` s. The positive control proves that an UPDATE was
        sent, not its AS_PATH encoding; the encoding of the base routes is
        verified by the DUT `adj-rib-in-post` checks in RT-1.111.2,
        RT-1.111.3 and RT-1.111.5
    *   Session stability: DUT
        `neighbors/neighbor/state/established-transitions` (all six
        neighbors) and OTG `session_flap_count` (all six peers) must be
        unchanged from the baseline recorded in RT-1.111.1; in RT-1.111.6 and
        RT-1.111.7 the OTG `notifications_received` counter must also be
        unchanged. If a subtest reports a change, the next subtest waits for
        `ESTABLISHED` again and records a new baseline. A DUT that does not
        report `established-transitions` needs a deviation; only the OTG
        counter is checked then
    *   Route set check (each of the six ATE peers): no negative prefix (even
        one the same peer advertised itself) and no prefix of the other
        address family is received; every expected
        prefix is received; prefixes the same peer advertised itself (echo)
        are ignored; any other prefix is a failure. Expected sets: port-1
        `CONFED`, `IBGP`, `IBGP-CONFED`, `CONFED-TRANSIT`; port-2 `EXT`,
        `CONFED`, `CONFED-TRANSIT`; port-3 `EXT`, `IBGP`, `IBGP-CONFED`
    *   A failed DUT/ATE setup or a session that does not reach `ESTABLISHED`
        in RT-1.111.1 stops the test. Otherwise each failed check is reported
        with expected and observed values and the remaining checks still run.
        On failure, collect DUT `session-state`,
        `last-notification-error-code`, `loc-rib` prefixes and OTG peer
        metrics

### RT-1.111.1: Confederation configuration, state and session establishment

*   Read the confederation state with gNMI `Subscribe` `ON_CHANGE`
    *   /network-instances/network-instance/protocols/protocol/bgp/global/state/as
    *   /network-instances/network-instance/protocols/protocol/bgp/global/confederation/state/identifier
    *   /network-instances/network-instance/protocols/protocol/bgp/global/confederation/state/member-as
*   Wait for each of the six DUT neighbors separately to reach `ESTABLISHED`,
    and for each of the six OTG peers to reach `up`
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state
*   Record the session stability baseline (`established-transitions`,
    `session_flap_count`)
*   For a `30` s hold window check that none of the eight negative prefixes is
    in the DUT `loc-rib` (they may have been advertised briefly at protocol
    start), then compare the stability counters with the baseline
*   Pass: `as` = `64501`, `identifier` = `64500`, `member-as` = `[64502]`
    (`[64501, 64502]` only with a documented deviation); all sessions
    `ESTABLISHED` / `up`; no negative prefix in `loc-rib`; stability counters
    unchanged

### RT-1.111.2: Confederation peer to iBGP peer (AS_PATH unchanged)

RFC 5065 Section 4.1 rule a: no AS_PATH change toward a peer in the same
Member-AS. LOCAL_PREF from a neighboring Member-AS is accepted and passed on
(RFC 4271 Section 5.1.5 confederation exception).

*   On the DUT, find `CONFED` and `CONFED-TRANSIT` in `adj-rib-in-post` of
    neighbor `192.0.2.10` / `2001:db8::a`, follow `attr-index` to the
    `attr-set` and read the AS_PATH and `local-pref`; confirm the prefixes are
    in `loc-rib`
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/attr-index
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/attr-index
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/type
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/member
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/local-pref
*   On ATE port-2, read the OTG `bgp-prefix` state of the same four prefixes
*   Pass:
    *   DUT and ATE port-2: `CONFED` has exactly `AS_CONFED_SEQUENCE [64502]`;
        `CONFED-TRANSIT` has exactly `AS_CONFED_SEQUENCE [64502]`,
        `AS_SEQ [64511]`; neither `64501` nor `64500` was prepended
    *   DUT `attr-set` and ATE port-2 report LOCAL_PREF `150` for `CONFED`
    *   Flows ATE port-2 to ATE port-3 pass the traffic check (RT-1.111.5)

### RT-1.111.3: iBGP peer to confederation peer (AS_CONFED_SEQUENCE prepend)

RFC 5065 Section 4.1 rule b: toward a neighboring Member-AS the DUT prepends
its Member-AS in an `AS_CONFED_SEQUENCE` (new segment for an empty AS_PATH,
rule b.3; prepended to an existing leading `AS_CONFED_SEQUENCE`, rule b.1).
Section 5.2: LOCAL_PREF and MED may be sent unchanged across the Member-AS
boundary; this test requires the DUT default to do so. Section 5.1: NEXT_HOP is
unchanged by default but may be changed by policy, so both unchanged and
next-hop-self are accepted and the observed value is logged.

*   On the DUT, read the `attr-set` of `IBGP` and `IBGP-CONFED` from neighbor
    `192.0.2.6` / `2001:db8::6` (`adj-rib-in-post`): AS_PATH, `local-pref`,
    `med`, `next-hop`
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/local-pref
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/next-hop
*   On ATE port-3, read the OTG `bgp-prefix` state of the same four prefixes:
    AS_PATH, `local_preference`, `multi_exit_discriminator`, `ipv4_next_hop` /
    `ipv6_next_hop`
*   Pass:
    *   DUT `attr-set` of `IBGP`: no AS_PATH segment, `local-pref` `200`,
        `med` `50`, `next-hop` `192.0.2.6` / `2001:db8::6`; `IBGP-CONFED`:
        exactly `AS_CONFED_SEQUENCE [64503]`
    *   ATE port-3 receives `IBGP` with exactly `AS_CONFED_SEQUENCE [64501]`
        and `IBGP-CONFED` with exactly `AS_CONFED_SEQUENCE [64501, 64503]`
        (one segment, `64501` first)
    *   ATE port-3 reports LOCAL_PREF `200` and MED `50` for `IBGP`
    *   ATE port-3 reports NEXT_HOP `192.0.2.6` or `192.0.2.9` (IPv4) and
        `2001:db8::6` or `2001:db8::9` (IPv6) for `IBGP`; the observed value is
        logged
    *   Flows ATE port-3 to ATE port-2 pass the traffic check (RT-1.111.5)

### RT-1.111.4: Confederation routes to the external peer (segments removed)

RFC 5065 Section 4.1 rule c and Section 5: toward a peer outside the
confederation the DUT removes all `AS_CONFED_*` segments and prepends the
Confederation Identifier as `AS_SEQ`; no `AS_CONFED_*` segment or Member-AS
number may reach the external peer. RFC 4271 Section 5.1.5: LOCAL_PREF is not
sent to external peers.

*   On ATE port-1, read the OTG `bgp-prefix` state of `CONFED`, `IBGP`,
    `IBGP-CONFED` and `CONFED-TRANSIT`: AS_PATH, `local_preference`, next hop
*   Pass:
    *   `CONFED`, `IBGP` and `IBGP-CONFED` are received with exactly
        `AS_SEQ [64500]`; `CONFED-TRANSIT` with exactly `AS_SEQ [64500, 64511]`
    *   No `as_confed_seq` / `as_confed_set` segment and none of `64501`,
        `64502`, `64503` anywhere in the AS_PATH
    *   LOCAL_PREF is not present for any of these prefixes
    *   Next hop is `192.0.2.1` (IPv4) / global `2001:db8::1` (IPv6)
    *   Flows ATE port-1 to ATE port-3 and ATE port-1 to ATE port-2 pass the
        traffic check (RT-1.111.5)

### RT-1.111.5: External route to confederation and iBGP peers

RFC 5065 Section 4.1 rule b.2: toward the neighboring Member-AS the DUT adds a
new `AS_CONFED_SEQUENCE [64501]` in front of `AS_SEQ [64510]`; rule a: toward
the iBGP peer the AS_PATH is unchanged.

*   On the DUT, confirm `EXT` is in `loc-rib` with exactly `AS_SEQ [64510]`
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/prefix
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/state/prefix
*   On ATE port-3 and ATE port-2, read the OTG `bgp-prefix` state of `EXT`
*   Run the route set check for all six ATE peers
*   Start all 12 flows together, run them for `30` s, stop them and evaluate
    every flow against the traffic check; report each flow with the subtest in
    the flow table
*   Pass:
    *   ATE port-3 receives `EXT` with exactly `AS_CONFED_SEQUENCE [64501]`,
        `AS_SEQ [64510]`; ATE port-2 with exactly `AS_SEQ [64510]`
    *   Route set check passes for all six ATE peers
    *   All 12 flows pass the traffic check

### RT-1.111.6: AS loop detection (Member-AS and Confederation Identifier)

Negative test. RFC 5065 Section 4: an `AS_CONFED_*` segment containing the
local Member-AS, or an AS_PATH containing the local Confederation Identifier,
is treated as if it contained the local AS (loop, RFC 4271 Section 9.1.2). This
is not a protocol error; the sessions must stay up.

*   Confirm all sessions are `ESTABLISHED` and the stability baseline is
    current; record OTG `notifications_received` of all six ATE peers and
    `routes_advertised` of the ATE port-1 and port-3 peers
*   Advertise `CONFED-LOOP` (ATE port-3) and `EXT-LOOP` (ATE port-1) with OTG
    route control state (`ADVERTISE`) and wait for the positive control
*   During the `30` s hold window check the DUT `loc-rib` and the OTG
    `bgp-prefix` state of ATE port-1, port-2 and port-3 for the four looped
    prefixes, and check that the base prefixes stay in `loc-rib`
*   Compare `established-transitions`, `session_flap_count` and
    `notifications_received` with the recorded values; run the route set check
*   Withdraw `CONFED-LOOP` and `EXT-LOOP` (also when the subtest fails)
*   Pass: positive control passed; no looped prefix in `loc-rib` or received by
    any ATE port; base prefixes still present; all sessions `ESTABLISHED`;
    stability and notification counters unchanged; route set check passes

### RT-1.111.7: Malformed AS_PATH from external and confederation peers

Negative test. RFC 5065 Section 5: an `AS_CONFED_*` segment from a peer
outside the confederation (`EXT-MALFORMED`), or an UPDATE from a neighboring
Member-AS whose first segment is not an `AS_CONFED_SEQUENCE`
(`CONFED-MALFORMED`), must be treated as having a malformed AS_PATH
(RFC 4271 Section 6.3). RFC 7606 Section 7.2 replaces the Section 6.3 session
reset for a malformed AS_PATH with treat-as-withdraw, so the route is discarded
and the session stays up. A session reset is a failure and is acceptable only
as a deviation agreed in review and tracked by a vendor bug.

*   Confirm all sessions are `ESTABLISHED` and the stability baseline is
    current; record OTG `notifications_received` of all six ATE peers and
    `routes_advertised` of the ATE port-1 and port-3 peers
*   Advertise `EXT-MALFORMED` (ATE port-1) and `CONFED-MALFORMED` (ATE port-3)
    with OTG route control state (`ADVERTISE`) and wait for the positive
    control
*   During the `30` s hold window check the sending neighbor's
    `adj-rib-in-post`, the DUT `loc-rib` and the OTG `bgp-prefix` state of all
    ATE ports for the four malformed prefixes; check that all six sessions stay
    `ESTABLISHED` and the base prefixes stay in `loc-rib`
*   Compare `established-transitions`, `session_flap_count` and
    `notifications_received` with the recorded values; run the route set check
*   Withdraw `EXT-MALFORMED` and `CONFED-MALFORMED` (also when the subtest
    fails)
*   Pass: positive control passed; no malformed prefix in `adj-rib-in-post`,
    `loc-rib` or received by any ATE port; base prefixes still present; all
    sessions `ESTABLISHED`; stability and notification counters unchanged
    (no NOTIFICATION sent by the DUT); route set check passes

### Cleanup

Runs at the end of the test, also when subtests failed, in this order:

*   Stop all traffic flows, then all protocols on the ATE
*   On the DUT, remove with gNMI `Set`: the six neighbors, then the three
    peer-groups, then the confederation and global BGP configuration
*   Remove the routing policy `ALLOW`
*   Remove the IPv4 and IPv6 addresses of `DUT:port1`, `DUT:port2` and
    `DUT:port3`, restoring the baseline configuration

## Canonical OC

BGP configuration of the DUT (interface addresses omitted, see the address
table above).

```json
{
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
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/established-transitions:
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
      delete: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

*   vRX
