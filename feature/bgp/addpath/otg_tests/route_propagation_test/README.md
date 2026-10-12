# RT-1.3: BGP Route Propagation

## Summary

BGP Route Propagation. Validates that the DUT propagates IPv4 and IPv6
prefixes between eBGP peers, and that the BGP ADD-Path (RFC 7911) `send` and
`receive` capabilities can be enabled and disabled per neighbor and per
peer-group through OpenConfig, with the configured value reflected in state,
in capability negotiation, and in the number of paths the DUT accepts and
advertises.

ADD-Path allows a BGP speaker to advertise multiple paths for the same prefix
over a single session by tagging each path with a path identifier. Without the
`receive` capability a speaker treats every new advertisement of a prefix from
a neighbor as an implicit withdraw of the previous one and therefore holds only
a single path.

In production the `receive` capability is used on route-reflector-to-client
sessions where the client is a receiver only and the route reflector
advertises up to four paths per prefix; the client re-advertises only its
single best path. This test exercises the same per-AFI/SAFI knobs on eBGP
sessions: ATE port-1 plays the multi-path sender, the DUT plays the receiver,
and ATE port-2 observes what the DUT re-advertises.

Out of scope for this test: installation of the received paths as ECMP
forwarding entries (covered by the BGP multipath tests), sender-side
`send-max` enforcement and multi-peer scale (covered by RT-1.15).

## Testbed type

*  [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Topology

```text
                        +-----------------------------------+
+---------------------+ | DUT  (AS 65537)                   |
| ATE                 | |                                   |
|  port-1 (AS 65536) <---> port-1  192.0.2.1 / 2001:db8::1  |
|  multi-path sender  | |          peer-group BGP-PEER-GROUP1
|                     | |                                   |
|  port-2 (AS 65538) <---> port-2  192.0.2.5 / 2001:db8::5  |
|  observer           | |          peer-group BGP-PEER-GROUP2
+---------------------+ +-----------------------------------+
```

*   DUT port-1: `192.0.2.1/30`, `2001:db8::1/126`.
    ATE port-1: `192.0.2.2/30`, `2001:db8::2/126`.
*   DUT port-2: `192.0.2.5/30`, `2001:db8::5/126`.
    ATE port-2: `192.0.2.6/30`, `2001:db8::6/126`.

## Procedure

### Test environment setup

Establish eBGP sessions between:

*   ATE port-1 (AS 65536) and DUT port-1 (DUT AS 65537)
*   ATE port-2 (AS 65538) and DUT port-2

*   DUT port-1 neighbors (`192.0.2.2` / `2001:db8::2`) are members of
    peer-group `BGP-PEER-GROUP1`; DUT port-2 neighbors (`192.0.2.6` /
    `2001:db8::6`) are members of peer-group `BGP-PEER-GROUP2`.
*   Apply an accept-all import and export policy on both peer-groups.
*   ATE port-1 advertises 4 IPv4 prefixes (`198.51.100.0/32` to
    `198.51.100.3/32`) and 4 IPv6 prefixes (`2001:db8:1::1/128` to
    `2001:db8:1::4/128`).
*   For the ADD-Path subtests (RT-1.3.3 onwards) ATE port-1 advertises **each
    prefix four times** with path identifiers `1`, `2`, `3`, `4`. All four
    paths use the ATE port-1 address as next-hop (`192.0.2.2` /
    `2001:db8::2`) and differ only in MED (`100`, `200`, `300`, `400`
    respectively), so every path is valid and resolvable on the DUT and a
    single best path (MED 100) is deterministic.
*   Both ATE peers announce the ADD-Path capability with send and receive for
    IPv4-unicast and IPv6-unicast.
*   Wait for all four BGP sessions to reach ESTABLISHED on the DUT
    (`/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state`)
    and on the ATE before each verification step.
*   Because ADD-Path is negotiated in the BGP OPEN message, every change to
    `add-paths` configuration is followed by restarting the BGP protocols on
    the affected ATE port (stop and start the OTG BGP peers) so the capability
    is renegotiated. No device CLI is used to clear sessions.
*   Path counts are verified primarily on ATE port-2 as the number of distinct
    path identifiers received per prefix. The DUT advertises to ATE port-2
    over eBGP, so next-hop is rewritten and MED is not propagated; only the
    path identifier distinguishes the paths there. Where the DUT exposes
    `adj-rib-in-pre`, the count is additionally verified on the DUT.

### RT-1.3.1: MRAI: [TODO: https://github.com/openconfig/featureprofiles/issues/3035]
*   DUT: Configure the Minimum Route Advertisement Interval (MRAI) for desired behavior.
*   ATE Port 2: Verify received routes adhere to the MRAI timing.

### RT-1.3.2: RFC5549
*   DUT: Enable RFC5549 support:
    *   Update the BGP peer group configuration to enable extended next hop encoding using  `/network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/ipv4-unicast/config/extended-next-hop-encoding`
*   ATE Port 1: Advertise IPv4 routes with IPv6 next-hops.
*   ATE Port 2: Validate correct acceptance and propagation of routes with IPv6 next-hops.

### RT-1.3.3: Add-Path receive enabled on neighbor, send enabled towards observer

Intended state: ATE port-1 neighbors `receive = true` · ATE port-2 neighbors
`send = true` · `BGP-PEER-GROUP1` add-paths unset.

*   Step 1: On the DUT, for the ATE port-1 neighbors, set
    `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/config/receive`
    to `true` for IPV4_UNICAST and IPV6_UNICAST. For the ATE port-2 neighbors,
    set
    `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/config/send`
    to `true` for both AFI/SAFIs.
*   Step 2: Restart the BGP protocols on ATE port-1 and ATE port-2.
*   Step 3: ATE port-1 advertises every prefix with path identifiers 1 to 4.
*   Step 4: Validation.
    *   `.../neighbors/neighbor/afi-safis/afi-safi/add-paths/state/receive`
        for the ATE port-1 neighbors is `true`.
    *   `.../neighbors/neighbor/afi-safis/afi-safi/add-paths/state/send` for
        the ATE port-2 neighbors is `true`.
    *   `.../neighbors/neighbor/state/supported-capabilities` for the ATE
        port-1 neighbors contains `ADD_PATHS`.
    *   Optional, where supported:
        `.../bgp/rib/afi-safis/afi-safi/ipv4-unicast/neighbors/neighbor/adj-rib-in-pre/routes/route`
        for the ATE port-1 neighbor holds four entries per prefix, one per
        path identifier (likewise for ipv6-unicast).
    *   ATE port-2 receives every IPv4 and IPv6 prefix **four times**, with
        four distinct path identifiers.
*   Pass: all of the above. Fail: state or capability not reflected, or fewer
    than four distinct path identifiers per prefix observed on ATE port-2.

#### Canonical OC

```json
{
  "network-instances": {
    "network-instance": [
      {
        "name": "DEFAULT",
        "config": { "name": "DEFAULT" },
        "protocols": {
          "protocol": [
            {
              "identifier": "BGP",
              "name": "BGP",
              "config": { "identifier": "BGP", "name": "BGP" },
              "bgp": {
                "neighbors": {
                  "neighbor": [
                    {
                      "neighbor-address": "192.0.2.2",
                      "config": { "neighbor-address": "192.0.2.2", "peer-as": 65536 },
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "IPV4_UNICAST",
                            "config": { "afi-safi-name": "IPV4_UNICAST", "enabled": true },
                            "add-paths": { "config": { "receive": true } }
                          },
                          {
                            "afi-safi-name": "IPV6_UNICAST",
                            "config": { "afi-safi-name": "IPV6_UNICAST", "enabled": true },
                            "add-paths": { "config": { "receive": true } }
                          }
                        ]
                      }
                    },
                    {
                      "neighbor-address": "192.0.2.6",
                      "config": { "neighbor-address": "192.0.2.6", "peer-as": 65538 },
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "IPV4_UNICAST",
                            "config": { "afi-safi-name": "IPV4_UNICAST", "enabled": true },
                            "add-paths": { "config": { "send": true } }
                          },
                          {
                            "afi-safi-name": "IPV6_UNICAST",
                            "config": { "afi-safi-name": "IPV6_UNICAST", "enabled": true },
                            "add-paths": { "config": { "send": true } }
                          }
                        ]
                      }
                    }
                  ]
                }
              }
            }
          ]
        }
      }
    ]
  }
}
```

### RT-1.3.4: Add-Path send disabled towards observer

Intended state: ATE port-1 neighbors `receive = true` · ATE port-2 neighbors
`send = false` · `BGP-PEER-GROUP1` add-paths unset.

*   Step 1: On the DUT, for the ATE port-2 neighbors, set
    `add-paths/config/send` to `false` for both AFI/SAFIs. Ensure
    `add-paths/config/receive` is `true` on the ATE port-1 neighbors.
*   Step 2: Restart the BGP protocols on ATE port-2.
*   Step 3: ATE port-1 re-advertises every prefix with path identifiers 1 to 4.
*   Step 4: Validation.
    *   `.../neighbors/neighbor/afi-safis/afi-safi/add-paths/state/send` for
        the ATE port-2 neighbors is `false`.
    *   Optional, where supported: DUT Adj-RIB-In for the ATE port-1 neighbors
        still holds four entries per prefix.
    *   ATE port-2 receives every prefix **once** (best path only).
*   Pass: send state `false` and one path per prefix on ATE port-2. Fail:
    more than one path observed on ATE port-2.

#### Canonical OC

```json
{
  "network-instances": {
    "network-instance": [
      {
        "name": "DEFAULT",
        "config": { "name": "DEFAULT" },
        "protocols": {
          "protocol": [
            {
              "identifier": "BGP",
              "name": "BGP",
              "config": { "identifier": "BGP", "name": "BGP" },
              "bgp": {
                "neighbors": {
                  "neighbor": [
                    {
                      "neighbor-address": "192.0.2.6",
                      "config": { "neighbor-address": "192.0.2.6", "peer-as": 65538 },
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "IPV4_UNICAST",
                            "config": { "afi-safi-name": "IPV4_UNICAST", "enabled": true },
                            "add-paths": { "config": { "send": false } }
                          },
                          {
                            "afi-safi-name": "IPV6_UNICAST",
                            "config": { "afi-safi-name": "IPV6_UNICAST", "enabled": true },
                            "add-paths": { "config": { "send": false } }
                          }
                        ]
                      }
                    }
                  ]
                }
              }
            }
          ]
        }
      }
    ]
  }
}
```

### RT-1.3.5: Add-Path receive disabled on neighbor

Intended state: ATE port-1 neighbors `receive = false` · ATE port-2 neighbors
`send = true` · `BGP-PEER-GROUP1` add-paths unset.

*   Step 1: On the DUT, for the ATE port-1 neighbors, set
    `add-paths/config/receive` to `false` for both AFI/SAFIs. Set
    `add-paths/config/send` to `true` on the ATE port-2 neighbors.
*   Step 2: Restart the BGP protocols on ATE port-1 and ATE port-2.
*   Step 3: ATE port-1 re-advertises every prefix with path identifiers 1 to 4.
*   Step 4: Validation.
    *   `.../neighbors/neighbor/afi-safis/afi-safi/add-paths/state/receive`
        for the ATE port-1 neighbors is `false`.
    *   Optional, where supported: DUT Adj-RIB-In for the ATE port-1 neighbors
        holds **one** entry per prefix (each later advertisement replaces the
        previous one).
    *   ATE port-2 receives every prefix **once**, even though send is enabled
        on that neighbor, because the DUT holds a single path.
    *   Informational only: `.../neighbors/neighbor/state/supported-capabilities`
        for the ATE port-1 neighbors is expected not to list `ADD_PATHS`.
        This leaf carries no direction or AFI/SAFI and some implementations
        report peer-advertised capabilities, so it is logged, not asserted.
*   Pass: receive state `false` and one path per prefix on ATE port-2. Fail:
    receive state not `false` or more than one path observed.

#### Canonical OC

```json
{
  "network-instances": {
    "network-instance": [
      {
        "name": "DEFAULT",
        "config": { "name": "DEFAULT" },
        "protocols": {
          "protocol": [
            {
              "identifier": "BGP",
              "name": "BGP",
              "config": { "identifier": "BGP", "name": "BGP" },
              "bgp": {
                "neighbors": {
                  "neighbor": [
                    {
                      "neighbor-address": "192.0.2.2",
                      "config": { "neighbor-address": "192.0.2.2", "peer-as": 65536 },
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "IPV4_UNICAST",
                            "config": { "afi-safi-name": "IPV4_UNICAST", "enabled": true },
                            "add-paths": { "config": { "receive": false } }
                          },
                          {
                            "afi-safi-name": "IPV6_UNICAST",
                            "config": { "afi-safi-name": "IPV6_UNICAST", "enabled": true },
                            "add-paths": { "config": { "receive": false } }
                          }
                        ]
                      }
                    }
                  ]
                }
              }
            }
          ]
        }
      }
    ]
  }
}
```

### RT-1.3.6: Add-Path receive configured on peer-group, neighbor override

Intended state (inheritance): ATE port-1 neighbors add-paths **unset** ·
`BGP-PEER-GROUP1` `receive = true` · ATE port-2 neighbors `send = true`.
Intended state (override): ATE port-1 neighbors `receive = false` ·
`BGP-PEER-GROUP1` `receive = true` · ATE port-2 neighbors `send = true`.

*   Step 1: On the DUT, remove the `add-paths` container from the ATE port-1
    neighbors using a gNMI Delete on
    `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths`
    for both AFI/SAFIs, so the neighbor has no explicit value and inherits
    from its peer-group. On peer-group `BGP-PEER-GROUP1` set
    `/network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/config/receive`
    to `true` for IPV4_UNICAST and IPV6_UNICAST.
*   Step 2: Restart the BGP protocols on ATE port-1.
*   Step 3: ATE port-1 re-advertises every prefix with path identifiers 1 to 4.
*   Step 4: Validation (inheritance).
    *   `.../peer-groups/peer-group/afi-safis/afi-safi/add-paths/state/receive`
        for `BGP-PEER-GROUP1` is `true`.
    *   `.../neighbors/neighbor/afi-safis/afi-safi/add-paths/state/receive`
        for the ATE port-1 neighbors is `true` (inherited).
    *   ATE port-2 receives every prefix four times, with four distinct path
        identifiers.
*   Step 5: On the DUT set `add-paths/config/receive` to `false` on the ATE
    port-1 **neighbors** while leaving the peer-group value `true`. Restart
    the BGP protocols on ATE port-1 and re-advertise.
*   Step 6: Validation (override).
    *   Peer-group `add-paths/state/receive` remains `true`.
    *   Neighbor `add-paths/state/receive` is `false` (neighbor overrides
        peer-group).
    *   ATE port-2 receives every prefix once.
*   Pass: both inheritance and override behave as above. Fail: neighbor state
    does not follow the peer-group when unset, or does not override it when
    set.

#### Canonical OC

```json
{
  "network-instances": {
    "network-instance": [
      {
        "name": "DEFAULT",
        "config": { "name": "DEFAULT" },
        "protocols": {
          "protocol": [
            {
              "identifier": "BGP",
              "name": "BGP",
              "config": { "identifier": "BGP", "name": "BGP" },
              "bgp": {
                "peer-groups": {
                  "peer-group": [
                    {
                      "peer-group-name": "BGP-PEER-GROUP1",
                      "config": { "peer-group-name": "BGP-PEER-GROUP1" },
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "IPV4_UNICAST",
                            "config": { "afi-safi-name": "IPV4_UNICAST", "enabled": true },
                            "add-paths": { "config": { "receive": true } }
                          },
                          {
                            "afi-safi-name": "IPV6_UNICAST",
                            "config": { "afi-safi-name": "IPV6_UNICAST", "enabled": true },
                            "add-paths": { "config": { "receive": true } }
                          }
                        ]
                      }
                    }
                  ]
                }
              }
            }
          ]
        }
      }
    ]
  }
}
```

### Cleanup

*   Remove the `add-paths` container from all DUT neighbors and from
    `BGP-PEER-GROUP1` using gNMI Delete.
*   Withdraw the ATE port-1 advertisements and stop ATE protocols.
*   Restore the DUT BGP configuration used by RT-1.3.1 and RT-1.3.2.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
  ## Config paths
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/config/receive:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/config/send:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/config/send-max:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/ipv4-unicast/config/extended-next-hop-encoding:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/config/receive:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/config/send:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/config/send-max:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/config/minimum-advertisement-interval:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/ipv4-unicast/config/extended-next-hop-encoding:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/config/receive:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/config/send:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/config/send-max:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/minimum-advertisement-interval:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/config/extended-next-hop-encoding:

  ## State paths
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/state/receive:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/state/send:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/state/send-max:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/ipv4-unicast/state/extended-next-hop-encoding:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/state/receive:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/state/send:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/state/send-max:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/state/minimum-advertisement-interval:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/ipv4-unicast/state/extended-next-hop-encoding:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/state/receive:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/state/send:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/state/send-max:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/state/minimum-advertisement-interval:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/state/extended-next-hop-encoding:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/supported-capabilities:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/neighbors/neighbor/adj-rib-in-pre/routes/route/state/path-id:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/neighbors/neighbor/adj-rib-in-pre/routes/route/state/path-id:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

* MFF - A modular form factor device containing LINECARDs, FABRIC and redundant CONTROLLER_CARD components
* FFF - fixed form factor
