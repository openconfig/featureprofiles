# RT-1.111: BGP Autonomous System Confederations (RFC 5065)

## Summary

BGP Autonomous System Confederations
([RFC 5065](https://datatracker.ietf.org/doc/html/rfc5065)): confederation
configuration and state, AS_PATH handling toward external, iBGP and
confederation peers, LOCAL_PREF / MED / NEXT_HOP handling inside the
confederation, and AS loop and malformed AS_PATH detection. IPv4 and IPv6
unicast.

## Testbed type

*   [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)
    (`port4` is not used)

## Procedure

### Test environment setup

The DUT is Member-AS `64501` of confederation `64500`:

```text
                          Confederation 64500
              +-----------------------------------------+
+----------+  |  +-------------+          +----------+  |
| ATE      |  |  |     DUT     |  port2   | ATE      |  |
| port1    |=====| Member-AS   |==========| port2    |  |
| AS 64510 |  |  |   64501     |  iBGP    | AS 64501 |  |
+----------+  |  +-------------+          +----------+  |
 External     |        || port3                         |
 eBGP (DUT    |        || confederation-eBGP            |
 uses 64500)  |  +-------------+                        |
              |  | ATE port3   |                        |
              |  | AS 64502    |                        |
              |  +-------------+                        |
              +-----------------------------------------+
```

*   Configure IPv4/IPv6 addresses on the interfaces:

    | Link   | DUT port                          | ATE port                           |
    | :----- | :-------------------------------- | :--------------------------------- |
    | Link 1 | `192.0.2.1/30`, `2001:db8::1/126` | `192.0.2.2/30`, `2001:db8::2/126`  |
    | Link 2 | `192.0.2.5/30`, `2001:db8::5/126` | `192.0.2.6/30`, `2001:db8::6/126`  |
    | Link 3 | `192.0.2.9/30`, `2001:db8::9/126` | `192.0.2.10/30`, `2001:db8::a/126` |

*   DUT: Configure BGP in the `DEFAULT` network instance (see
    [Canonical OC](#canonical-oc)): AS `64501`, `router-id` `192.0.2.254`,
    confederation `identifier` `64500` and `member-as` `[64502]`; peer-groups
    `EBGP-EXT` (`peer-as` `64510`), `IBGP-MEMBER` (`peer-as` `64501`) and
    `CONFED-EBGP` (`peer-as` `64502`) with IPv4 and IPv6 unicast enabled and an
    accept-all import/export policy `ALLOW`.
*   Establish BGP sessions (one IPv4 and one IPv6 per link) between:
    *   ATE port-1 (AS `64510`) and DUT port-1: external eBGP; the DUT uses the
        confederation identifier `64500` toward this peer.
    *   ATE port-2 (AS `64501`) and DUT port-2: iBGP within the Member-AS.
    *   ATE port-3 (AS `64502`) and DUT port-3: confederation-eBGP with the
        neighboring Member-AS.
*   ATE: Advertise the following routes. `Base` routes are advertised from the
    start; the negative routes are advertised only in the subtest that uses
    them and withdrawn afterwards. `64503` and `64511` appear only inside
    AS_PATHs.

    | Name               | ATE port | IPv4 prefix         | IPv6 prefix         | AS_PATH sent by ATE                            | LOCAL_PREF | MED  | Used in    |
    | :----------------- | :------- | :------------------ | :------------------ | :--------------------------------------------- | :--------- | :--- | :--------- |
    | `EXT`              | port-1   | `203.0.113.0/26`    | `2001:db8:100::/48` | `AS_SEQ [64510]`                               |            |      | Base       |
    | `IBGP`             | port-2   | `198.51.100.0/26`   | `2001:db8:200::/48` | empty                                          | `200`      | `50` | Base       |
    | `IBGP-CONFED`      | port-2   | `198.51.100.192/26` | `2001:db8:201::/48` | `AS_CONFED_SEQUENCE [64503]`                   | `100`      |      | Base       |
    | `CONFED`           | port-3   | `198.51.100.64/26`  | `2001:db8:300::/48` | `AS_CONFED_SEQUENCE [64502]`                   | `150`      |      | Base       |
    | `CONFED-TRANSIT`   | port-3   | `203.0.113.128/26`  | `2001:db8:302::/48` | `AS_CONFED_SEQUENCE [64502]`, `AS_SEQ [64511]` |            |      | Base       |
    | `CONFED-LOOP`      | port-3   | `198.51.100.128/26` | `2001:db8:301::/48` | `AS_CONFED_SEQUENCE [64502, 64501]`            |            |      | RT-1.111.6 |
    | `EXT-LOOP`         | port-1   | `203.0.113.64/26`   | `2001:db8:101::/48` | `AS_SEQ [64510, 64500]`                        |            |      | RT-1.111.6 |
    | `EXT-MALFORMED`    | port-1   | `203.0.113.192/27`  | `2001:db8:102::/48` | `AS_SEQ [64510]`, `AS_CONFED_SEQUENCE [64510]` |            |      | RT-1.111.7 |
    | `CONFED-MALFORMED` | port-3   | `203.0.113.224/27`  | `2001:db8:303::/48` | `AS_SEQ [64502]`                               |            |      | RT-1.111.7 |

*   Traffic: ATE-to-ATE IPv4 and IPv6 flows between the advertised prefixes
    (`512` B, `100` pps, `30` s); a flow passes with at most 1% loss.
*   AS_PATH verification compares segment types and members after merging
    adjacent segments of the same type (`AS_SEQ` or `AS_CONFED_SEQUENCE`);
    sets are never merged.
*   Absence check (negative routes): after the ATE advertises a route, confirm
    that it was actually sent (positive control: the ATE `routes_advertised`
    counter increases), then the prefix must stay absent for a `30` s hold
    window.

### RT-1.111.1: Confederation configuration, state and session establishment

*   DUT: Push the configuration above with gNMI `Set` `REPLACE`.
*   Verification (Telemetry): `global/state/as` = `64501`,
    `confederation/state/identifier` = `64500`,
    `confederation/state/member-as` = `[64502]` (`[64501, 64502]` only with a
    deviation).
*   Verification (Telemetry): all six DUT neighbors are `ESTABLISHED` and all
    six ATE peers are up. Record `established-transitions` and the ATE session
    flap counters as the baseline for the following subtests.
*   Verification (Telemetry): none of the negative prefixes is present in the
    DUT `loc-rib`.

### RT-1.111.2: Confederation peer to iBGP peer (AS_PATH unchanged, RFC 5065 Section 4.1 rule a)

*   ATE Port 3: Advertise `CONFED` and `CONFED-TRANSIT`.
*   Verification (Telemetry): DUT `adj-rib-in-post` of the port-3 neighbor and
    `loc-rib` show `CONFED` = `AS_CONFED_SEQUENCE [64502]` and
    `CONFED-TRANSIT` = `AS_CONFED_SEQUENCE [64502]`, `AS_SEQ [64511]`;
    `CONFED` keeps LOCAL_PREF `150` (RFC 4271 Section 5.1.5 confederation
    exception).
*   ATE Port 2: Receives both routes with the same AS_PATHs; no `64501` or
    `64500` prepended.
*   Traffic: flows port-2 to port-3 pass.

### RT-1.111.3: iBGP peer to confederation peer (AS_CONFED_SEQUENCE prepend, rule b)

*   ATE Port 2: Advertise `IBGP` (LOCAL_PREF `200`, MED `50`) and
    `IBGP-CONFED` (`AS_CONFED_SEQUENCE [64503]`).
*   Verification (Telemetry): the DUT `attr-set` of `IBGP` has no AS_PATH
    segment, `local-pref` `200`, `med` `50` and `next-hop` `192.0.2.6` /
    `2001:db8::6`; `IBGP-CONFED` = `AS_CONFED_SEQUENCE [64503]`;
    `adj-rib-out-post` toward port-3 keeps `local-pref` `200` (RFC 5065
    Section 5.2).
*   ATE Port 3: Receives `IBGP` = `AS_CONFED_SEQUENCE [64501]` (rule b.3) and
    `IBGP-CONFED` = `AS_CONFED_SEQUENCE [64501, 64503]` (rule b.1); MED `50`
    unchanged; NEXT_HOP either unchanged (`192.0.2.6` / `2001:db8::6`) or the
    DUT port-3 address (RFC 5065 Section 5.1).
*   Traffic: flows port-3 to port-2 pass.

### RT-1.111.4: Confederation routes to the external peer (segments removed, rule c)

*   ATE Port 1: Receives `CONFED`, `IBGP` and `IBGP-CONFED` = `AS_SEQ [64500]`
    and `CONFED-TRANSIT` = `AS_SEQ [64500, 64511]`.
*   Verification: no `AS_CONFED_SEQUENCE` / `AS_CONFED_SET` segment and none of
    `64501`, `64502`, `64503` in any received AS_PATH (RFC 5065 Section 5); no
    LOCAL_PREF (RFC 4271 Section 5.1.5); next hop `192.0.2.1` / `2001:db8::1`.
*   Traffic: flows port-1 to port-3 and port-1 to port-2 pass.

### RT-1.111.5: External route to confederation and iBGP peers

*   ATE Port 1: Advertise `EXT`.
*   Verification (Telemetry): `EXT` is in the DUT `loc-rib` with
    `AS_SEQ [64510]`.
*   ATE Port 3: Receives `EXT` = `AS_CONFED_SEQUENCE [64501]`, `AS_SEQ [64510]`
    (rule b.2).
*   ATE Port 2: Receives `EXT` = `AS_SEQ [64510]` (rule a).
*   Verification: every ATE peer holds exactly its expected routes (port-1:
    `CONFED`, `IBGP`, `IBGP-CONFED`, `CONFED-TRANSIT`; port-2: `EXT`, `CONFED`,
    `CONFED-TRANSIT`; port-3: `EXT`, `IBGP`, `IBGP-CONFED`), no negative route
    and no route of the other address family.
*   Traffic: flows port-3 to port-1 and port-2 to port-1 pass.

### RT-1.111.6: AS loop detection (Member-AS and confederation identifier)

Negative test (RFC 5065 Section 4, RFC 4271 Section 9.1.2): an AS_PATH that
contains the local Member-AS in an `AS_CONFED_*` segment, or the local
confederation identifier, is a loop and must be dropped without a session
reset.

*   ATE Port 3: Advertise `CONFED-LOOP` (`AS_CONFED_SEQUENCE [64502, 64501]`).
*   ATE Port 1: Advertise `EXT-LOOP` (`AS_SEQ [64510, 64500]`).
*   Verification (Telemetry): for `30` s the looped prefixes are absent from
    the DUT `loc-rib` and from all ATE ports; the `Base` routes stay in
    `loc-rib`; all sessions stay `ESTABLISHED` with `established-transitions`,
    ATE session flaps and ATE notifications received unchanged.
*   ATE: Withdraw `CONFED-LOOP` and `EXT-LOOP`.

### RT-1.111.7: Malformed AS_PATH from external and confederation peers

Negative test (RFC 5065 Section 5, RFC 4271 Section 6.3, RFC 7606 Section
7.2): an `AS_CONFED_*` segment from a peer outside the confederation, or a
confederation-peer UPDATE whose first segment is not an `AS_CONFED_SEQUENCE`,
is a malformed AS_PATH and must be treated as withdraw without a session reset.

*   ATE Port 1: Advertise `EXT-MALFORMED` (`AS_SEQ [64510]`,
    `AS_CONFED_SEQUENCE [64510]`).
*   ATE Port 3: Advertise `CONFED-MALFORMED` (`AS_SEQ [64502]`).
*   Verification (Telemetry): for `30` s the malformed prefixes are absent from
    the sender's `adj-rib-in-post`, the DUT `loc-rib` and all ATE ports; the
    `Base` routes stay in `loc-rib`; all sessions stay `ESTABLISHED` with the
    stability counters and ATE notifications received unchanged. A session
    reset is a failure.
*   ATE: Withdraw `EXT-MALFORMED` and `CONFED-MALFORMED`.

### Cleanup

Runs at the end of the test, also after failures:

*   ATE: Stop all flows and protocols.
*   DUT: Remove the BGP neighbors, peer-groups, confederation and global BGP
    configuration, the routing policy `ALLOW` and the interface addresses, so
    the DUT is back in its baseline state.

## Canonical OC

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
  }
}
```

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
  ## Config paths
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

  ## State paths
  /network-instances/network-instance/protocols/protocol/bgp/global/state/as:
  /network-instances/network-instance/protocols/protocol/bgp/global/confederation/state/identifier:
  /network-instances/network-instance/protocols/protocol/bgp/global/confederation/state/member-as:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/established-transitions:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/messages/sent/last-notification-error-code:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/messages/received/last-notification-error-code:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/prefix:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/attr-index:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/state/prefix:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/state/attr-index:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/prefix:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/attr-index:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/prefix:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/attr-index:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/neighbors/neighbor/adj-rib-in-pre/routes/route/state/attr-index:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/neighbors/neighbor/adj-rib-in-pre/routes/route/state/attr-index:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/neighbors/neighbor/adj-rib-out-post/routes/route/state/attr-index:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/neighbors/neighbor/adj-rib-out-post/routes/route/state/attr-index:
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

*   vRX - virtual router device
