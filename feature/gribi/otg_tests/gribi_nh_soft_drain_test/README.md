# TE-3.11: gRIBI Next-Hop Weight Adjustment and Soft Drain

## Summary

Validate hitless traffic soft-draining and undraining of an egress path using
dynamic `gRIBI.Modify` next-hop weight adjustments and `NextHopGroup`
membership updates while keeping the physical egress interface operationally up
and preserving active eBGP and IS-IS control-plane sessions without flaps.

In production traffic-engineering workflows, controllers gracefully drain an
egress link prior to maintenance by shifting WCMP weights away from the target
next-hop and removing it from active `NextHopGroup`s before bringing the link
down, then gradually restoring weight upon re-entry. This test validates
dual-stack (`1,000` IPv4 and `1,000` IPv6 routes) WCMP traffic redistribution,
`0%` packet loss across drain and undrain transitions, link-down/bring-up
isolation while drained, and zero collateral impact on unrelated `NextHopGroup`s
or control-plane adjacencies.

Relationship to existing tests: TE-3.10 covers WCMP weight changes and next-hop
addition/removal, TE-3.2 covers the accuracy of weight ratios (including
`63:1`), and TE-3.7 covers a lossless `NextHopGroup` update that moves traffic
to other next hops. This test reuses the TE-3.10 port addressing, next-hop IDs
and scale, and adds the checks those tests do not make: the drained interface
stays up and its eBGP and IS-IS sessions do not flap during a gRIBI soft drain
(progressive weight reduction, then removal of the next hop from the
`NextHopGroup`) and during the staged undrain, and the drained link can be
taken down and brought back up without traffic loss or premature use of the
drained path. Draining with a `weight = 0` member is covered by the optional
subtest TE-3.11.5 on platforms that support it.

## Testbed type

* [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

```text
+------------+                   +------------+
|    ATE     |  port1 (Ingress)  |    DUT     |
|            |-------------------|            |
|            |  port2 (NH1 Drn)  |            |
|            |-------------------|            |
|            |  port3 (NH2 Act)  |            |
|            |-------------------|            |
|            |  port4 (NH3 Bgd)  |            |
|            |-------------------|            |
+------------+                   +------------+
```

## Procedure

### Test environment setup

*   Configure `4` physical interfaces connecting the DUT and ATE with dual-stack
    IPv4 (`/30`) and IPv6 (`/126`) point-to-point addressing in the `DEFAULT`
    network instance:
    *   **Port 1 (Traffic Ingress)**: Connect DUT `port1` (`192.0.2.1/30`,
        `2001:db8:1::1/126`) to ATE `port1` (`192.0.2.2/30`,
        `2001:db8:1::2/126`).
    *   **Port 2 (Primary Egress 1 — Target Soft-Drain Path `NH1`)**: Connect
        DUT `port2` (`192.0.2.5/30`, `2001:db8:2::1/126`) to ATE `port2`
        (`192.0.2.6/30`, `2001:db8:2::2/126`).
    *   **Port 3 (Primary Egress 2 — Remaining Active Path `NH2`)**: Connect
        DUT `port3` (`192.0.2.9/30`, `2001:db8:3::1/126`) to ATE `port3`
        (`192.0.2.10/30`, `2001:db8:3::2/126`).
    *   **Port 4 (Isolated Background Egress `NH3`)**: Connect DUT `port4`
        (`192.0.2.13/30`, `2001:db8:4::1/126`) to ATE `port4` (`192.0.2.14/30`,
        `2001:db8:4::2/126`).
*   Configure control-plane routing adjacencies on Egress Links `port2` and
    `port3` in `DEFAULT` so session stability can be continuously monitored
    during gRIBI data-plane soft-drain operations:
    *   **eBGP**: Configure DUT local AS `64500` (`hold-time: 9`,
        `keepalive-interval: 3`) peering with ATE `port2` (`192.0.2.6` and
        `2001:db8:2::2`, remote AS `64501`) and ATE `port3` (`192.0.2.10` and
        `2001:db8:3::2`, remote AS `64502`). `IPV4_UNICAST` is enabled on the
        IPv4 neighbors and `IPV6_UNICAST` on the IPv6 neighbors, and both
        AFI/SAFIs are also enabled under `/bgp/global/afi-safis`.
    *   **IS-IS**: Enable Level-2 IS-IS (`WIDE_METRIC`, `POINT_TO_POINT`,
        Area `49.0001`, DUT System ID `1920.0000.2001`) on DUT `port2` and
        `port3` paired with emulated IS-IS routers on ATE `port2`
        (`1920.0000.2002`) and ATE `port3` (`1920.0000.2003`).
    *   The ATE eBGP peers and emulated IS-IS routers advertise no route
        ranges, so no BGP- or IS-IS-learned route can overlap the
        gRIBI-programmed prefixes.
*   Apply the Canonical OC below with `gNMI.Set` (`REPLACE`) on each DUT
    `/interfaces/interface[name=portN]` subtree, on the `DEFAULT`
    network-instance `config` container, and on its `BGP` and `ISIS`
    `protocols/protocol` subtrees, before the OTG protocols are started and
    the control-plane baseline is recorded. No later step changes the DUT
    configuration until Cleanup.
*   Start OTG protocols and explicitly verify ARP and IPv6 Neighbor Discovery
    (ND) resolution across all `4` links before starting any traffic flows.
*   Verify via `gNMI.Subscribe` (`ON_CHANGE`, timeout `120 s`) that all `4`
    eBGP neighbor sessions (`192.0.2.6`, `2001:db8:2::2`, `192.0.2.10`,
    `2001:db8:3::2`) are `ESTABLISHED` and both IS-IS adjacencies on `port2`
    and `port3` are `UP`. Then record control-plane baseline `B0`: each
    neighbor's `state/established-transitions` and `state/last-established`,
    and each IS-IS adjacency's `state/up-timestamp`.
*   Establish a `gRIBI` client session to the DUT with session parameters
    `persistence = PRESERVE`, `redundancy = SINGLE_PRIMARY`
    (`election_id = {low: 1, high: 0}`), and `ack_type = RIB_AND_FIB_ACK`
    (negotiated once for the session; it applies to every `AFTOperation` of
    this test). Elect the client as leader and issue `gRIBI.Flush` on `DEFAULT`
    to guarantee a clean initial AFT state.

#### Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
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
                    "as": 64500,
                    "router-id": "192.0.2.1"
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
                        "neighbor-address": "192.0.2.6",
                        "peer-as": 64501
                      },
                      "neighbor-address": "192.0.2.6",
                      "timers": {
                        "config": {
                          "hold-time": 9,
                          "keepalive-interval": 3
                        }
                      }
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
                        "neighbor-address": "2001:db8:2::2",
                        "peer-as": 64501
                      },
                      "neighbor-address": "2001:db8:2::2",
                      "timers": {
                        "config": {
                          "hold-time": 9,
                          "keepalive-interval": 3
                        }
                      }
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
                        "peer-as": 64502
                      },
                      "neighbor-address": "192.0.2.10",
                      "timers": {
                        "config": {
                          "hold-time": 9,
                          "keepalive-interval": 3
                        }
                      }
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
                        "neighbor-address": "2001:db8:3::2",
                        "peer-as": 64502
                      },
                      "neighbor-address": "2001:db8:3::2",
                      "timers": {
                        "config": {
                          "hold-time": 9,
                          "keepalive-interval": 3
                        }
                      }
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
            },
            {
              "config": {
                "identifier": "openconfig-policy-types:ISIS",
                "name": "ISIS"
              },
              "identifier": "openconfig-policy-types:ISIS",
              "isis": {
                "global": {
                  "afi-safi": {
                    "af": [
                      {
                        "afi-name": "openconfig-isis-types:IPV4",
                        "config": {
                          "afi-name": "openconfig-isis-types:IPV4",
                          "enabled": true,
                          "safi-name": "openconfig-isis-types:UNICAST"
                        },
                        "safi-name": "openconfig-isis-types:UNICAST"
                      },
                      {
                        "afi-name": "openconfig-isis-types:IPV6",
                        "config": {
                          "afi-name": "openconfig-isis-types:IPV6",
                          "enabled": true,
                          "safi-name": "openconfig-isis-types:UNICAST"
                        },
                        "safi-name": "openconfig-isis-types:UNICAST"
                      }
                    ]
                  },
                  "config": {
                    "level-capability": "LEVEL_2",
                    "net": [
                      "49.0001.1920.0000.2001.00"
                    ]
                  }
                },
                "interfaces": {
                  "interface": [
                    {
                      "config": {
                        "circuit-type": "POINT_TO_POINT",
                        "enabled": true,
                        "interface-id": "port2.0"
                      },
                      "interface-id": "port2.0"
                    },
                    {
                      "config": {
                        "circuit-type": "POINT_TO_POINT",
                        "enabled": true,
                        "interface-id": "port3.0"
                      },
                      "interface-id": "port3.0"
                    }
                  ]
                },
                "levels": {
                  "level": [
                    {
                      "config": {
                        "level-number": 2,
                        "metric-style": "WIDE_METRIC"
                      },
                      "level-number": 2
                    }
                  ]
                }
              },
              "name": "ISIS"
            }
          ]
        }
      }
    ]
  }
}
```

#### WCMP states

| State | `NHG 20` (IPv4) | `NHG 21` (IPv6) | `port2` share (window) | `port3` share (window) |
|---|---|---|---|---|
| `1:1` | `{NH 101: 1, NH 102: 1}` | `{NH 201: 1, NH 202: 1}` | `50%` (`[48%, 52%]`) | `50%` (`[48%, 52%]`) |
| `1:63` | `{NH 101: 1, NH 102: 63}` | `{NH 201: 1, NH 202: 63}` | `1/64` = `1.5625%` (`[0.8125%, 2.3125%]`) | `63/64` = `98.4375%` (`[97.6875%, 99.1875%]`) |
| drained | `{NH 102: 1}` | `{NH 202: 1}` | `0` packets (drain check) | `100%` |
| zero-weight (TE-3.11.5 only) | `{NH 101: 0, NH 102: 1}` | `{NH 201: 0, NH 202: 1}` | `0` packets (drain check) | `100%` |

*   Members are written as `{NextHop: weight}`. The windows are `±2%` (`1:1`)
    and `±0.75%` (`1:63`) around the expected shares. With `10,000` equal-rate
    5-tuples per family, the binomial standard deviation of the `port2` share
    is `0.50` percentage points at `1:1` and `0.124` percentage points at
    `1:63`, so the tolerances are `4σ` and `6σ`, and a `0%` `port2` share
    cannot pass the `1:63` window.
*   The `1:63` state requires the DUT to program a `NextHopGroup` with a total
    weight of `64` without reducing the weight resolution (the same requirement
    as the `63:1` case of TE-3.2). A DUT that rounds `1:63`, for example to
    `1:31` (`3.125%`), fails the `1:63` window.
*   In gNMI AFT telemetry, the next-hop-group of a state must have exactly the
    listed members, identified by next-hop `ip-address` (`NH 101` =
    `192.0.2.6`, `NH 102` = `192.0.2.10`, `NH 201` = `2001:db8:2::2`,
    `NH 202` = `2001:db8:3::2`), with the listed weights. For the drained state
    only the membership is compared. For the zero-weight state the group must
    contain `NH 102` / `NH 202` with weight `1` and no `NH 101` / `NH 201`
    member with a weight greater than `0`.

#### Traffic flows

Every flow is sent from ATE `port1` with `512`-byte frames at a constant rate.
The flows are configured once during setup; later steps only start and stop
selected flows (OTG flow-transmit control by flow name).

| Flow | Rate | Packet headers | OTG Rx ports |
|---|---|---|---|
| `Flow-WCMP-v4` | `20,000 pps` | IPv4 src `192.0.2.2`, dst `100.64.10.1` incrementing by `1` for `1,000` values (last `100.64.13.232`); UDP src port `49152` incrementing by `1` for `10,000` values (last `59151`); UDP dst port `60000` | ATE `port2`, ATE `port3` |
| `Flow-WCMP-v6` | `20,000 pps` | IPv6 src `2001:db8:1::2`, dst `2001:db8:10::1` incrementing by `1` for `1,000` values (last `2001:db8:10::3e8`); UDP ports as in `Flow-WCMP-v4` | ATE `port2`, ATE `port3` |
| `Flow-WCMP-v4-P3` | `20,000 pps` | Same as `Flow-WCMP-v4` | ATE `port3` only |
| `Flow-WCMP-v6-P3` | `20,000 pps` | Same as `Flow-WCMP-v6` | ATE `port3` only |
| `Flow-BG-v4` | `2,000 pps` | IPv4 src `192.0.2.2`, dst `198.51.100.1`; UDP src port `49152`, dst port `60000` | ATE `port4` |
| `Flow-BG-v6` | `2,000 pps` | IPv6 src `2001:db8:1::2`, dst `2001:db8:99::1`; UDP src port `49152`, dst port `60000` | ATE `port4` |

*   The destination address and the UDP source port of a primary flow
    increment together on every packet, so each primary flow repeats a cycle of
    exactly `10,000` distinct 5-tuples (each of the `1,000` destinations with
    `10` source ports), all sent at the same rate.
*   OTG counts a flow's frames only on its Rx ports, so a packet that egresses
    a DUT port whose ATE port is not an Rx port of its flow is reported as
    lost. The `-P3` flows are transmitted instead of `Flow-WCMP-v4` and
    `Flow-WCMP-v6` only in drain checks, where any primary-flow packet that
    egresses DUT `port2` is therefore counted as lost.

#### Measurement procedures

The subtests use the following procedures. Every pass/fail criterion is
evaluated after traffic has been stopped, so frames in flight cannot be
mistaken for loss, and only flow-tracked OTG counters and DUT transit-packet
counters are used, so the eBGP, IS-IS, ARP and ND packets exchanged on `port2`
and `port3` cannot affect any criterion.

*   **Loss check**: after stopping traffic and once every flow reports
    `transmit = stopped`, wait via `gnmi.Watch` (timeout `10 s`) until each
    transmitted flow's per-run OTG `frames_rx` equals its per-run `frames_tx`
    (per-run = delta from the value read before the run was started). A
    timeout fails the check with `frames_tx - frames_rx` lost packets.
*   **Loss budget**: every gRIBI change in this test re-weights, removes or
    re-adds `NH 101` / `NH 201` in a `NextHopGroup` whose other member
    (`NH 102` / `NH 202`) stays valid and installed, so the DUT can apply it
    make-before-break. The test therefore uses the strictest budget allowed by
    the requirement ("`0%` packet loss or under an allowable threshold"): `0`
    lost packets per flow in every run. A DUT that drops packets while applying
    a `NextHopGroup` `REPLACE` does not provide a hitless soft drain and fails.
*   **DUT forwarding counters**: the `ipv4` and `ipv6`
    `state/counters/out-forwarded-pkts` of `subinterface[index=0]` of DUT
    `port2` and `port3`. They count only packets forwarded by the DUT, not
    packets it originates (eBGP, ND) or non-IP frames (IS-IS), and the
    background flows egress `port4`, so on `port2` and `port3` they count only
    primary-flow packets. They are read only while traffic is stopped: once
    before the first run, and at the end of every run after its loss check.
    The end-of-run snapshot is taken as soon as, for each family, the `port2`
    plus `port3` deltas since the previous snapshot reach the per-run
    `frames_rx` of that family's `Flow-WCMP` (or `-P3`) flow (`gnmi.Watch` on a
    `SAMPLE` subscription, timeout `120 s`). No transit traffic is forwarded
    between runs, so every delta belongs to exactly one run regardless of the
    DUT's counter update interval. On platforms that do not report subinterface
    IPv4/IPv6 packet counters, no DUT counter snapshots are taken.
*   **AFT state check** (state `S`): via `gNMI.Subscribe` (`ON_CHANGE`) on the
    `DEFAULT` AFTs, wait (timeout `30 s`) until the next-hop-group referenced
    by `state/next-hop-group` of `100.64.10.1/32`, `100.64.13.232/32`,
    `2001:db8:10::1/128` and `2001:db8:10::3e8/128` matches `S` in the WCMP
    states table. Members are read from
    `next-hop-groups/next-hop-group/next-hops/next-hop/state/index` and
    `state/weight`, and each index is resolved to
    `next-hops/next-hop/state/ip-address`. AFT next-hop-group IDs and next-hop
    indexes are assigned by the DUT and are not compared with gRIBI IDs.
*   **Transition run** (gRIBI change `X`, new state `S`):
    1.  Start `Flow-WCMP-v4`, `Flow-WCMP-v6`, `Flow-BG-v4` and `Flow-BG-v6`.
    2.  After `10 s`, send `X` in one `ModifyRequest` and await
        `FIB_PROGRAMMED` for each of its operations (timeout `30 s`; `FAILED`
        or `FIB_FAILED` fails the step). Log the time from sending the request
        to the last `FIB_PROGRAMMED` (informational, not a pass/fail
        criterion).
    3.  Run the AFT state check for `S`.
    4.  Transmit for another `10 s`, stop traffic and run the loss check on all
        four flows: `0` lost packets across the change.
    5.  Take the end-of-run DUT counter snapshot.
*   **Split check** (state `S`):
    1.  Start `Flow-WCMP-v4`, `Flow-WCMP-v6`, `Flow-BG-v4` and `Flow-BG-v6`,
        transmit for `60 s`, stop traffic and run the loss check on all four
        flows: `0` lost packets.
    2.  Take the end-of-run DUT counter snapshot. For each family, the `port2`
        share (`port2` delta / (`port2` delta + `port3` delta), deltas from the
        previous snapshot) and the `port3` share (its complement) must be
        within the windows of `S` in the WCMP states table.
    3.  On platforms that do not report subinterface IPv4/IPv6 packet counters,
        perform step 1 twice, transmitting only `Flow-WCMP-v4` (plus the
        background flows) and then only `Flow-WCMP-v6` (plus the background
        flows), and compute the shares of step 2 from the per-run OTG port
        metrics `frames_rx` of ATE `port2` and ATE `port3`. These also count the
        control frames that the DUT sends on `port2` and `port3` (eBGP
        keepalives every `3 s`, IS-IS hellos, ARP/ND), which stay below `300`
        per port per `60 s` run and change the computed shares by less than
        `0.05` percentage points.
*   **Drain check** (run length `D`, `60 s` unless stated otherwise):
    1.  Start `Flow-WCMP-v4-P3`, `Flow-WCMP-v6-P3`, `Flow-BG-v4` and
        `Flow-BG-v6`, transmit for `D`, stop traffic and run the loss check on
        all four flows: `0` lost packets. Because the `-P3` flows are counted
        only on ATE `port3`, this proves that every primary-flow packet
        egressed DUT `port3` and none egressed `port2`.
    2.  Take the end-of-run DUT counter snapshot: the `port2` `ipv4` and `ipv6`
        deltas from the previous snapshot must be `0`. On platforms that do not
        report subinterface IPv4/IPv6 packet counters, step 1 alone is the
        drain criterion.

### TE-3.11.1 - Baseline Dual-Stack 1:1 WCMP Forwarding and Isolation

*   **Step 1 — Program Initial gRIBI AFT Hierarchy**:
    *   The Canonical OC was applied in Test environment setup, before baseline
        `B0` was recorded, and is not re-applied here.
    *   Via `gRIBI.Modify` `ADD` operations in `DEFAULT`, install in this order
        (next hops, then next-hop groups, then prefixes):
        *   IPv4 `NextHop`s: `NH 101` -> `192.0.2.6` (ATE `port2`), `NH 102` ->
            `192.0.2.10` (ATE `port3`), and `NH 103` -> `192.0.2.14` (ATE
            `port4`).
        *   IPv6 `NextHop`s: `NH 201` -> `2001:db8:2::2` (ATE `port2`),
            `NH 202` -> `2001:db8:3::2` (ATE `port3`), and `NH 203` ->
            `2001:db8:4::2` (ATE `port4`).
        *   Primary WCMP `NextHopGroup` `20` (IPv4): `{NH 101: weight 1, NH 102:
            weight 1}`.
        *   Primary WCMP `NextHopGroup` `21` (IPv6): `{NH 201: weight 1, NH 202:
            weight 1}`.
        *   Isolated Background `NextHopGroup` `30` (IPv4): `{NH 103: weight
            1}`.
        *   Isolated Background `NextHopGroup` `31` (IPv6): `{NH 203: weight
            1}`.
        *   `1,000` IPv4 `/32` prefixes (`100.64.10.1/32` through
            `100.64.13.232/32`) -> `NextHopGroup` `20`.
        *   `1,000` IPv6 `/128` prefixes (`2001:db8:10::1/128` through
            `2001:db8:10::3e8/128`) -> `NextHopGroup` `21`.
        *   Background IPv4 prefix `198.51.100.0/24` -> `NextHopGroup` `30` and
            Background IPv6 prefix `2001:db8:99::/64` -> `NextHopGroup` `31`.
    *   Verify that every operation is acknowledged with `AFTResult.status =
        FIB_PROGRAMMED` and none with `FAILED` or `FIB_FAILED` (timeout `120 s`
        after the last operation is sent). On platforms that do not send
        `FIB_PROGRAMMED` for a `NextHop` or `NextHopGroup` until a forwarding
        entry references it, the `NextHop` and `NextHopGroup` acknowledgements
        are awaited only after the operations of the referencing
        `NextHopGroup`s and prefixes have been acknowledged.
*   **Step 2 — Verify Programmed AFT State**:
    *   `gRIBI.Get` (`network_instance = DEFAULT`) returns exactly: for
        `aft = IPV4`, the `1,000` `/32` entries referencing `NHG 20` and
        `198.51.100.0/24` referencing `NHG 30`; for `aft = IPV6`, the `1,000`
        `/128` entries referencing `NHG 21` and `2001:db8:99::/64` referencing
        `NHG 31`; for `aft = NEXTHOP_GROUP`, `NHG 20`, `21`, `30` and `31` with
        the members and weights of Step 1; for `aft = NEXTHOP`, `NH 101`-`103`
        and `NH 201`-`203` with the IP addresses of Step 1.
    *   Via `gNMI.Subscribe` (`ON_CHANGE`) on the `DEFAULT` AFTs, verify that
        all `1,000` IPv4 and `1,000` IPv6 primary prefixes are present
        (`state/prefix`) and that their `state/next-hop-group` matches state
        `1:1` of the WCMP states table, and that `198.51.100.0/24` and
        `2001:db8:99::/64` reference a next-hop-group whose single member has
        `ip-address` `192.0.2.14` and `2001:db8:4::2` respectively.
*   **Step 3 — Baseline Traffic Distribution Verification**:
    *   Run the split check for state `1:1`: for each family, the `port2` and
        `port3` shares are within `[48%, 52%]` (`50%` ± `2%`), with `0` lost
        packets on all four flows.
    *   Because `Flow-BG-v4` and `Flow-BG-v6` are counted only on ATE `port4`
        and the primary flows only on ATE `port2` and `port3`, the zero-loss
        result also shows that `100%` of the background traffic arrives on ATE
        `port4` and no WCMP traffic egresses `port4`.

### TE-3.11.2 - Hitless Progressive Weight Soft-Drain and NextHop Removal

*   **Step 1 — Progressive Positive-Weight Soft-Drain (`1:1` -> `1:63`)**:
    *   Perform a transition run whose gRIBI change is a `REPLACE` of
        `NextHopGroup` `20` (`{NH 101: weight 1, NH 102: weight 63}`) and of
        `NextHopGroup` `21` (`{NH 201: weight 1, NH 202: weight 63}`), new
        state `1:63`: `FIB_PROGRAMMED` for both operations, AFT `weight` leaves
        `1` and `63`, and `0` lost packets on all four flows.
    *   Run the split check for state `1:63`: for each family, the `port2`
        share is within `[0.8125%, 2.3125%]` (`1.5625%` ± `0.75%`) and the
        `port3` share within `[97.6875%, 99.1875%]`, with `0` lost packets.
*   **Step 2 — Complete Soft-Drain by NextHop Removal (`1:63` -> drained)**:
    *   Perform a transition run whose gRIBI change is a `REPLACE` of
        `NextHopGroup` `20` with `{NH 102: weight 1}` and of `NextHopGroup`
        `21` with `{NH 202: weight 1}`, new state drained: `FIB_PROGRAMMED` for
        both operations and `0` lost packets on all four flows. This removes
        `NH 101` and `NH 201` from active WCMP forwarding while keeping them
        installed in the gRIBI `NextHop` table (no operation is sent for them).
    *   Verify via `gRIBI.Get` that `aft = NEXTHOP_GROUP` returns `NHG 20` =
        `{NH 102: weight 1}` and `NHG 21` = `{NH 202: weight 1}`, and that
        `aft = NEXTHOP` still returns `NH 101` and `NH 201`.
    *   `NH 101` and `NH 201` are now unreferenced: no acknowledgement is
        expected for them, and their FIB status and their presence in gNMI AFT
        next-hop telemetry are not asserted while they are drained.
    *   Removing the next hop from the `NextHopGroup` is the mandatory drain
        method. Draining with a `weight = 0` member is not part of this step;
        it is covered by the optional subtest TE-3.11.5 after the undrain.
*   **Step 3 — Data-Plane Drain & Control-Plane Session Non-Disruption Check**:
    *   Run the drain check (`60 s`): `0` lost packets on `Flow-WCMP-v4-P3`,
        `Flow-WCMP-v6-P3`, `Flow-BG-v4` and `Flow-BG-v6`, so `100%` of both
        primary streams egress DUT `port3` and none egress `port2`; on
        platforms that report subinterface counters, the `port2` `ipv4` and
        `ipv6` `out-forwarded-pkts` deltas are `0`.
    *   Verify via `gNMI.Get` / `gNMI.Subscribe` that:
        *   DUT `port2` remains physically and operationally up
            (`/interfaces/interface[name=port2]/state/oper-status = UP`,
            `admin-status = UP`).
        *   All `4` eBGP sessions, including IPv4 (`192.0.2.6`) and IPv6
            (`2001:db8:2::2`) on `port2`, are `ESTABLISHED` with
            `state/established-transitions` and `state/last-established` equal
            to baseline `B0` (zero increments since setup).
        *   The IS-IS Level-2 adjacencies on `port2` and `port3` are `UP` with
            `state/up-timestamp` equal to baseline `B0`.
        *   Background traffic on `port4` (`Flow-BG-v4`, `Flow-BG-v6`) had `0`
            lost packets in every run of TE-3.11.1 and TE-3.11.2.

### TE-3.11.3 - Drained Link Down and Link Bring-Up Maintenance Isolation

*   While `NH 101` and `NH 201` remain drained out of `NextHopGroup`s `20` and
    `21`, perform one drain check whose run spans the following sequence (run
    length `D` ends `30 s` after the last step below completes):
    *   After `10 s` of traffic, set ATE `port2` link state to `DOWN` via OTG
        `SetControlState`, immediately registering a `defer` cleanup callback
        to guarantee ATE `port2` is restored to `UP` even if a subsequent
        assertion fails.
    *   Await `/interfaces/interface[name=port2]/state/oper-status = DOWN`
        (timeout `30 s`), the `port2` eBGP sessions (`192.0.2.6`,
        `2001:db8:2::2`) leaving `ESTABLISHED`, and the `port2` IS-IS
        adjacency no longer being reported as `UP` (timeout `30 s`), via
        `gNMI.Subscribe` (`ON_CHANGE`).
    *   While `port2` is down, verify that the `port3` eBGP sessions
        (`192.0.2.10`, `2001:db8:3::2`) are `ESTABLISHED` with
        `established-transitions` and `last-established` equal to `B0`, and
        that the `port3` IS-IS adjacency is `UP` with `up-timestamp` equal to
        `B0`.
    *   Restore ATE `port2` link state to `UP` via OTG `SetControlState`, then
        await DUT `port2` `oper-status = UP` (timeout `30 s`), OTG ARP and ND
        resolution on ATE `port2`, eBGP `ESTABLISHED` (`192.0.2.6`,
        `2001:db8:2::2`) and IS-IS adjacency `UP` on `port2` (timeout `120 s`)
        via `gNMI.Subscribe` (`ON_CHANGE`).
*   Verify the drain check criteria for this run: `0` lost packets on all four
    flows, so no primary-flow packet egressed `port2` while it went down, came
    back up and re-established its routing sessions before the explicit gRIBI
    undrain, and nothing was lost on `port3` or `port4`; on platforms that
    report subinterface counters, the `port2` `out-forwarded-pkts` deltas are
    `0`.
*   Verify via `gRIBI.Get` that `NHG 20` = `{NH 102: weight 1}` and `NHG 21` =
    `{NH 202: weight 1}`, and that `NH 101` (`192.0.2.6`) and `NH 201`
    (`2001:db8:2::2`) are still installed (the link flap does not remove gRIBI
    entries).
*   Verify that the `port3` eBGP sessions and IS-IS adjacency still match `B0`
    (zero increments, unchanged timestamps).
*   Record control-plane baseline `B1` (the leaves of `B0`, for all `4`
    neighbors and both adjacencies) after the `port2` sessions are back up.
    TE-3.11.4 and TE-3.11.5 compare against `B1`, because the intentional
    `port2` flap legitimately changed the `port2` values of `B0`.

### TE-3.11.4 - Hitless Undrain (Gradual Re-Entry to 50/50 WCMP Balance)

*   **Step 1 — Cautious Re-Entry (`1:63` Weight)**:
    *   Perform a transition run whose gRIBI change is a `REPLACE` of
        `NextHopGroup` `20` with `{NH 101: weight 1, NH 102: weight 63}` and of
        `NextHopGroup` `21` with `{NH 201: weight 1, NH 202: weight 63}`, new
        state `1:63`: `FIB_PROGRAMMED` for both operations and `0` lost packets
        on all four flows.
    *   Run the split check for state `1:63`: for each family, the `port2`
        share is within `[0.8125%, 2.3125%]` and the `port3` share within
        `[97.6875%, 99.1875%]`, with `0` lost packets.
*   **Step 2 — Full Restoration (`1:1` Equal Balance)**:
    *   Perform a transition run whose gRIBI change is a `REPLACE` restoring
        `NextHopGroup` `20` (`{NH 101: weight 1, NH 102: weight 1}`) and
        `NextHopGroup` `21` (`{NH 201: weight 1, NH 202: weight 1}`), new state
        `1:1`: `FIB_PROGRAMMED` for both operations and `0` lost packets on all
        four flows.
    *   Run the split check for state `1:1`: for each family, the `port2` and
        `port3` shares are within `[48%, 52%]`, with `0` lost packets.
*   **Step 3 — Control-Plane and Isolation Verification**:
    *   All eBGP and IS-IS adjacencies on `port2` and `port3` remain
        `ESTABLISHED` / `UP`, with `state/established-transitions` and
        `state/last-established` (BGP) and `adjacency/state/up-timestamp`
        (IS-IS) equal to baseline `B1`.
    *   `gRIBI.Get` (`aft = NEXTHOP_GROUP`) returns `NHG 30` =
        `{NH 103: weight 1}` and `NHG 31` = `{NH 203: weight 1}`, the AFT
        entries of `198.51.100.0/24` and `2001:db8:99::/64` still reference a
        next-hop-group whose single member is `192.0.2.14` and `2001:db8:4::2`
        respectively, and `Flow-BG-v4` and `Flow-BG-v6` had `0` lost packets in
        every run.

### TE-3.11.5 - Zero-Weight NextHop Soft Drain (Optional)

This subtest is optional. It runs only on platforms that support an explicit
`weight = 0` member in a gRIBI `NextHopGroup`; on other platforms it is skipped
and makes no assertion (TODO: gate it in the test code with a new deviation,
e.g. `gribi_nhg_zero_weight_unsupported`). Removing the next hop from the
`NextHopGroup` (TE-3.11.2) remains the mandatory drain method, and no pass
criterion of TE-3.11.1 to TE-3.11.4 depends on this subtest. It runs after
TE-3.11.4, starting from state `1:1`, so it cannot affect the drained-state
checks of TE-3.11.2 and TE-3.11.3. The `NextHopGroup`s it modifies are
referenced by `1,000` prefixes each, so their `FIB_PROGRAMMED` acknowledgements
do not depend on how a platform acknowledges unreferenced entries.

*   **Step 1 — Zero-Weight Drain**: perform a transition run whose gRIBI change
    is a `REPLACE` of `NextHopGroup` `20` with `{NH 101: weight 0, NH 102:
    weight 1}` and of `NextHopGroup` `21` with `{NH 201: weight 0, NH 202:
    weight 1}`, new state zero-weight: `FIB_PROGRAMMED` for both operations and
    `0` lost packets on all four flows. Verify via `gRIBI.Get`
    (`aft = NEXTHOP_GROUP`) that `NHG 20` and `NHG 21` contain `NH 101` and
    `NH 201` with `weight 0`.
*   **Step 2 — Zero-Weight Drain Check**: run the drain check (`60 s`): `0` lost
    packets on all four flows, so no primary-flow packet egresses `port2`; on
    platforms that report subinterface counters, the `port2` `ipv4` and `ipv6`
    `out-forwarded-pkts` deltas are `0`.
*   **Step 3 — Restore**: perform a transition run whose gRIBI change is a
    `REPLACE` of `NextHopGroup` `20` with `{NH 101: weight 1, NH 102: weight
    1}` and of `NextHopGroup` `21` with `{NH 201: weight 1, NH 202: weight 1}`,
    new state `1:1` (`0` lost packets), then run the split check for state
    `1:1`.
*   The eBGP and IS-IS values remain equal to baseline `B1`.

### Cleanup

*   Stop all OTG traffic streams and ensure any modified OTG link states (`ATE
    port2`) are restored to `UP` via `defer` / `t.Cleanup()`.
*   Issue `gRIBI.Flush` on the `DEFAULT` network instance and revert all DUT
    interface, eBGP, and IS-IS configurations via `gNMI.Set` registered in
    `t.Cleanup()`, leaving the DUT in its baseline pre-test state.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  ## Config Parameter Coverage
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/name:
  /interfaces/interface/config/type:
  /interfaces/interface/subinterfaces/subinterface/config/index:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled:
  /network-instances/network-instance/config/name:
  /network-instances/network-instance/config/type:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/afi-safi-name:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/as:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/config/afi-safi-name:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/neighbor-address:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/hold-time:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/keepalive-interval:
  /network-instances/network-instance/protocols/protocol/config/identifier:
  /network-instances/network-instance/protocols/protocol/config/name:
  /network-instances/network-instance/protocols/protocol/isis/global/afi-safi/af/config/afi-name:
  /network-instances/network-instance/protocols/protocol/isis/global/afi-safi/af/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/global/afi-safi/af/config/safi-name:
  /network-instances/network-instance/protocols/protocol/isis/global/config/level-capability:
  /network-instances/network-instance/protocols/protocol/isis/global/config/net:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/config/circuit-type:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/config/interface-id:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/config/level-number:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/config/metric-style:

  ## Telemetry Parameter Coverage
  /interfaces/interface/state/admin-status:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-forwarded-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-forwarded-pkts:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/weight:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/established-transitions:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/last-established:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/adjacency-state:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/up-timestamp:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
      replace: true
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
