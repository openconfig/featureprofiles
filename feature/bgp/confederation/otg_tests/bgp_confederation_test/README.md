# RT-1.111: BGP Autonomous System Confederations (RFC 5065)

## Summary

Validate BGP Autonomous System Confederations
([RFC 5065](https://datatracker.ietf.org/doc/html/rfc5065)) on a single DUT:
confederation configuration and state telemetry, AS_PATH handling toward
external, iBGP and confederation-eBGP peers, LOCAL_PREF / MED / NEXT_HOP
handling across the Member-AS boundary, and rejection of looped and malformed
AS_PATHs without session resets. IPv4 and IPv6 unicast.

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

*   Configure IPv4/IPv6 addresses on the interfaces

    | Link   | DUT port                          | ATE port                           |
    | :----- | :-------------------------------- | :--------------------------------- |
    | Link 1 | `192.0.2.1/30`, `2001:db8::1/126` | `192.0.2.2/30`, `2001:db8::2/126`  |
    | Link 2 | `192.0.2.5/30`, `2001:db8::5/126` | `192.0.2.6/30`, `2001:db8::6/126`  |
    | Link 3 | `192.0.2.9/30`, `2001:db8::9/126` | `192.0.2.10/30`, `2001:db8::a/126` |

*   Configure BGP on the DUT (`DEFAULT` network instance, gNMI `Set` `REPLACE`,
    see [Canonical OC](#canonical-oc)): Member-AS `64501`, `router-id`
    `192.0.2.254`, confederation `identifier` `64500`, `member-as` `[64502]`;
    peer-groups `EBGP-EXT` (`peer-as` `64510`), `IBGP-MEMBER` (`64501`) and
    `CONFED-EBGP` (`64502`) with `IPV4_UNICAST` and `IPV6_UNICAST` enabled and
    an accept-all import/export policy `ALLOW`; one IPv4 and one IPv6 neighbor
    per peer-group, each enabling only its own address family
    *   /network-instances/network-instance/protocols/protocol/bgp/global/confederation/config/identifier
    *   /network-instances/network-instance/protocols/protocol/bgp/global/confederation/config/member-as
*   Establish BGP sessions (one IPv4 and one IPv6 per port) between:
    *   ATE port-1 (AS `64510`, OTG `ebgp`) and DUT port-1: external eBGP; the
        DUT uses the confederation identifier `64500` toward this peer. OTG
        has no remote-AS field, so this is verified through the AS_PATH
        received in RT-1.111.4
    *   ATE port-2 (AS `64501`, OTG `ibgp`) and DUT port-2: iBGP in the same
        Member-AS
    *   ATE port-3 (AS `64502`, OTG `ebgp`) and DUT port-3: confederation-eBGP
        with the neighboring Member-AS
*   Advertise the following route ranges from the ATE (`64503` and `64511`
    appear only inside AS_PATHs; LOCAL_PREF and MED are sent only where given)

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

*   OTG route range settings: on `ebgp` peers the ATE always prepends its own
    AS, so use `as_set_mode` `include_as_confed_seq` on port-3 (`CONFED`,
    `CONFED-TRANSIT`, `CONFED-LOOP`) and `include_as_seq` on port-1 and for
    `CONFED-MALFORMED`; add the remaining segments with `as_path.segments`
    (`as_seq [64511]` for `CONFED-TRANSIT`, `as_confed_seq [64501]` for
    `CONFED-LOOP`, `as_seq [64500]` for `EXT-LOOP`, `as_confed_seq [64510]`
    for `EXT-MALFORMED`). On the `ibgp` peer use `do_not_include_local_as`
    (`as_confed_seq [64503]` for `IBGP-CONFED`). Set
    `advanced.include_local_preference` and
    `advanced.include_multi_exit_discriminator` explicitly on every range. The
    looped routes may be sent as one or two adjacent segments; AS_PATHs sent
    by the DUT are compared segment by segment
*   Configure all nine route ranges in a single ATE configuration, start
    protocols, then withdraw `CONFED-LOOP`, `EXT-LOOP`, `EXT-MALFORMED` and
    `CONFED-MALFORMED` with OTG route control state. Negative ranges are only
    advertised/withdrawn with control state; never push a new ATE
    configuration mid-test. Wait for ARP and IPv6 ND on all three links
*   Configure 12 ATE-to-ATE flows (`512` B, `1000` pps, `30` s; IPv4 and IPv6):
    port-2 to port-3, port-3 to port-2, port-1 to port-3, port-1 to port-2,
    port-3 to port-1, port-2 to port-1. Source and destination addresses are
    inside the advertised prefixes (`203.0.113.1`, `198.51.100.1`,
    `198.51.100.65`, `2001:db8:100::1`, `2001:db8:200::1`, `2001:db8:300::1`).
    All flows run once at the end of RT-1.111.5; a flow passes with Tx > 0 and
    0% loss and is reported with the subtest whose routes it uses
*   Checks shared by the subtests
    *   DUT telemetry via gNMI `Subscribe` `ON_CHANGE`. `as-segment` `index` `0`
        is the leftmost segment, `member` is in AS_PATH order; a different
        order needs a deviation
    *   Absence check: record the sender's OTG `routes_advertised`, advertise
        the range, wait up to `60` s for the counter to increase (positive
        control; otherwise inconclusive = fail), then the prefix must stay
        absent for a `30` s hold window. The positive control proves an
        UPDATE was sent, not its encoding; base-route encoding is verified
        by the DUT `adj-rib-in-post` checks
    *   Session stability: DUT `established-transitions` and OTG
        `session_flap_count` of all six sessions unchanged from the baseline
        recorded in RT-1.111.1 (re-baselined after a reported change);
        in RT-1.111.6/7 also OTG `notifications_received`. A DUT without
        `established-transitions` needs a deviation; OTG counter only
    *   Route set check per ATE peer: no negative prefix (even an echoed one),
        no prefix of the other address family, all expected prefixes present,
        own echoed prefixes ignored, anything else fails. Expected: port-1
        `CONFED`, `IBGP`, `IBGP-CONFED`, `CONFED-TRANSIT`; port-2 `EXT`,
        `CONFED`, `CONFED-TRANSIT`; port-3 `EXT`, `IBGP`, `IBGP-CONFED`
    *   A setup failure or a session not reaching `ESTABLISHED` in RT-1.111.1
        stops the test; otherwise each failed check is reported and the
        subtest continues. On failure collect DUT `session-state`,
        `last-notification-error-code`, `loc-rib` and OTG peer metrics

### RT-1.111.1: Confederation configuration, state and session establishment

*   Verify `global/state/as` = `64501`, `confederation/state/identifier` =
    `64500`, `confederation/state/member-as` = `[64502]` (`[64501, 64502]`
    only with a deviation)
    *   /network-instances/network-instance/protocols/protocol/bgp/global/state/as
    *   /network-instances/network-instance/protocols/protocol/bgp/global/confederation/state/identifier
    *   /network-instances/network-instance/protocols/protocol/bgp/global/confederation/state/member-as
*   Wait for each of the six DUT neighbors to be `ESTABLISHED` and each of the
    six OTG peers to be `up`, then record the session stability baseline
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/established-transitions
*   For `30` s none of the eight negative prefixes may be in the DUT `loc-rib`
    (they may have been advertised briefly at protocol start); stability
    counters unchanged

### RT-1.111.2: Confederation peer to iBGP peer (AS_PATH unchanged, RFC 5065 Section 4.1 rule a)

*   On the DUT, read `CONFED` and `CONFED-TRANSIT` in `adj-rib-in-post` of
    neighbor `192.0.2.10` / `2001:db8::a` (`attr-index` to `attr-set`) and
    confirm them in `loc-rib`: `CONFED` = exactly `AS_CONFED_SEQUENCE [64502]`
    with `local-pref` `150`; `CONFED-TRANSIT` = exactly
    `AS_CONFED_SEQUENCE [64502]`, `AS_SEQ [64511]`
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/attr-index
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/attr-index
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/type
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/member
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/local-pref
*   ATE port-2 receives the same four prefixes with the same AS_PATHs (no
    `64501` or `64500` prepended) and LOCAL_PREF `150` for `CONFED` (RFC 4271
    Section 5.1.5 confederation exception)
*   Flows port-2 to port-3: 0% loss

### RT-1.111.3: iBGP peer to confederation peer (AS_CONFED_SEQUENCE prepend, rule b)

*   On the DUT, read the `attr-set` of `IBGP` and `IBGP-CONFED` from neighbor
    `192.0.2.6` / `2001:db8::6`: `IBGP` has no AS_PATH segment, `local-pref`
    `200`, `med` `50`, `next-hop` `192.0.2.6` / `2001:db8::6`; `IBGP-CONFED` =
    exactly `AS_CONFED_SEQUENCE [64503]`
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/local-pref
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/next-hop
*   ATE port-3 receives `IBGP` = exactly `AS_CONFED_SEQUENCE [64501]` (rule
    b.3) and `IBGP-CONFED` = exactly one segment
    `AS_CONFED_SEQUENCE [64501, 64503]` (rule b.1); LOCAL_PREF `200` and MED
    `50` unchanged (RFC 5065 Section 5.2); NEXT_HOP either unchanged
    (`192.0.2.6` / `2001:db8::6`, the Section 5.1 default) or the DUT port-3
    address (`192.0.2.9` / `2001:db8::9`), log which
*   Flows port-3 to port-2: 0% loss

### RT-1.111.4: Confederation routes to the external peer (segments removed, rule c)

*   ATE port-1 receives `CONFED`, `IBGP` and `IBGP-CONFED` = exactly
    `AS_SEQ [64500]` and `CONFED-TRANSIT` = exactly one segment
    `AS_SEQ [64500, 64511]`; no `as_confed_seq` / `as_confed_set` segment and
    none of `64501`, `64502`, `64503` anywhere (RFC 5065 Section 5); no
    LOCAL_PREF (RFC 4271 Section 5.1.5); next hop `192.0.2.1` / `2001:db8::1`
*   Flows port-1 to port-3 and port-1 to port-2: 0% loss

### RT-1.111.5: External route to confederation and iBGP peers

*   On the DUT, `EXT` is in `loc-rib` with exactly `AS_SEQ [64510]`
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/prefix
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/state/prefix
*   ATE port-3 receives `EXT` = exactly `AS_CONFED_SEQUENCE [64501]`,
    `AS_SEQ [64510]` (rule b.2); ATE port-2 = exactly `AS_SEQ [64510]` (rule a)
*   Route set check for all six ATE peers
*   Start all 12 flows together for `30` s, stop, and evaluate every flow

### RT-1.111.6: AS loop detection (Member-AS and confederation identifier)

Negative test, RFC 5065 Section 4: a path containing the local Member-AS in an
`AS_CONFED_*` segment or the local confederation identifier is a loop
(RFC 4271 Section 9.1.2), not a protocol error.

*   Confirm all sessions `ESTABLISHED` and the baseline current; record OTG
    `notifications_received` of all six peers and `routes_advertised` of the
    port-1 and port-3 peers
*   Advertise `CONFED-LOOP` and `EXT-LOOP` with route control state; positive
    control
*   During the `30` s hold window the four looped prefixes are absent from the
    DUT `loc-rib` and from all ATE ports; base prefixes stay in `loc-rib`;
    all sessions stay `ESTABLISHED`; stability counters and
    `notifications_received` unchanged; route set check passes
*   Withdraw `CONFED-LOOP` and `EXT-LOOP` (also when the subtest fails)

### RT-1.111.7: Malformed AS_PATH from external and confederation peers

Negative test, RFC 5065 Section 5: an `AS_CONFED_*` segment from a peer
outside the confederation (`EXT-MALFORMED`) or a confederation-peer UPDATE
whose first segment is not an `AS_CONFED_SEQUENCE` (`CONFED-MALFORMED`) is a
malformed AS_PATH (RFC 4271 Section 6.3); RFC 7606 Section 7.2 replaces the
session reset with treat-as-withdraw.

*   Confirm all sessions `ESTABLISHED` and the baseline current; record OTG
    `notifications_received` of all six peers and `routes_advertised` of the
    port-1 and port-3 peers
*   Advertise `EXT-MALFORMED` and `CONFED-MALFORMED` with route control state;
    positive control
*   During the `30` s hold window the four prefixes are absent from the
    sender's `adj-rib-in-post`, the DUT `loc-rib` and all ATE ports; base
    prefixes stay in `loc-rib`; all sessions stay `ESTABLISHED`; stability
    counters and `notifications_received` unchanged; route set check passes.
    A session reset is a failure (deviation only if agreed in review and
    tracked by a vendor bug)
*   Withdraw `EXT-MALFORMED` and `CONFED-MALFORMED` (also when the subtest
    fails)

### Cleanup

Runs at the end of the test, also after failures, in this order:

*   Stop all flows, then all protocols on the ATE
*   On the DUT remove the six neighbors, then the three peer-groups, then the
    confederation and global BGP configuration (gNMI `Set` delete)
*   Remove the routing policy `ALLOW`
*   Remove the IPv4/IPv6 addresses of `DUT:port1`, `DUT:port2` and `DUT:port3`

## Canonical OC

BGP and routing-policy configuration of the DUT. Interface addresses are in the
address table; the IPv6 neighbors (`2001:db8::2`, `2001:db8::6`, `2001:db8::a`)
are identical to the IPv4 neighbors shown except for the address and the
enabled address family.

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
