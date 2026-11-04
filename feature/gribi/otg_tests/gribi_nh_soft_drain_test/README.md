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
        `2001:db8:3::2`, remote AS `64502`) with `IPV4_UNICAST` and
        `IPV6_UNICAST` AFI/SAFIs enabled.
    *   **IS-IS**: Enable Level-2 IS-IS (`WIDE_METRIC`, `POINT_TO_POINT`,
        Area `49.0001`, DUT System ID `1920.0000.2001`) on DUT `port2` and
        `port3` paired with emulated IS-IS routers on ATE `port2`
        (`1920.0000.2002`) and ATE `port3` (`1920.0000.2003`).
*   Start OTG protocols and explicitly verify ARP and IPv6 Neighbor Discovery
    (ND) resolution across all `4` links before starting any traffic flows.
*   Verify via `gNMI.Subscribe` (`ON_CHANGE`) that all `4` eBGP neighbor
    sessions (`192.0.2.6`, `2001:db8:2::2`, `192.0.2.10`, `2001:db8:3::2`) are
    `ESTABLISHED` (recording baseline `established-transitions` counts) and
    both IS-IS adjacencies on `port2` and `port3` are `UP`.
*   Establish a `gRIBI` client session to the DUT with `persistence = PRESERVE`,
    `redundancy = SINGLE_PRIMARY` (`election_id = {low: 1, high: 0}`), and
    `ack_type = RIB_AND_FIB_ACK`. Elect the client as leader and issue
    `gRIBI.Flush` on `DEFAULT` to guarantee a clean initial AFT state.

### TE-3.11.1 - Baseline Dual-Stack 1:1 WCMP Forwarding and Isolation

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

*   **Step 1 — Program Initial gRIBI AFT Hierarchy**:
    *   Push the Canonical OC configuration via `gNMI.Set` (`REPLACE`).
    *   Via `gRIBI.Modify` (`ADD` with `RIB_AND_FIB_ACK`) in `DEFAULT`, install:
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
    *   Verify `AFTResult.status = FIB_PROGRAMMED` for every operation and
        confirm via `gNMI.Subscribe` (`ON_CHANGE`) on
        `/network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/weight`
        and `gRIBI.Get` that weights `(1, 1)` are active in hardware.
*   **Step 2 — Baseline Traffic Distribution Verification**:
    *   Start continuous high-entropy UDP traffic from ATE `port1`:
        *   **Primary IPv4 Stream (`Flow-WCMP-v4`)**: `10,000` 5-tuple UDP flows
            (`512`-byte frames, `20,000 pps` aggregate) across all `1,000`
            destinations in `100.64.10.1`–`100.64.13.232`.
        *   **Primary IPv6 Stream (`Flow-WCMP-v6`)**: `10,000` 5-tuple UDP flows
            (`512`-byte frames, `20,000 pps` aggregate) across all `1,000`
            destinations in `2001:db8:10::1`–`2001:db8:10::3e8`.
        *   **Background Streams (`Flow-BG-v4`, `Flow-BG-v6`)**: `2,000 pps`
            each destined to `198.51.100.1` and `2001:db8:99::1`.
    *   Verify `Flow-WCMP-v4` and `Flow-WCMP-v6` balance `50% / 50%` (`±2%`
        tolerance) across ATE `port2` and ATE `port3` with `0%` packet loss.
    *   Verify `100%` of `Flow-BG-v4` and `Flow-BG-v6` arrives on ATE `port4`
        with `0%` packet loss.

### TE-3.11.2 - Hitless Progressive Weight Soft-Drain and NextHop Removal

*   **Step 1 — Progressive Positive-Weight Soft-Drain (`1:1` -> `1:63`)**:
    *   While all `4` traffic streams remain continuously running at line/test
        rate, issue a `gRIBI.Modify` (`REPLACE` with `RIB_AND_FIB_ACK`) on
        `NextHopGroup` `20` (`{NH 101: weight 1, NH 102: weight 63}`) and
        `NextHopGroup` `21` (`{NH 201: weight 1, NH 202: weight 63}`).
    *   Await `FIB_PROGRAMMED` acknowledgement and confirm via `gNMI.Subscribe`
        (`ON_CHANGE`) that the AFT `weight` leaves update to `1` and `63`
        without static sleeps.
    *   Clear or snapshot OTG port/flow rate counters after `FIB_PROGRAMMED` and
        verify traffic shifts to `1.56%` (`1/64`) on ATE `port2` and `98.44%`
        (`63/64`) on ATE `port3` within `±0.75%` tolerance (`[0.81%, 2.31%]` on
        `port2` and `[97.69%, 99.19%]` on `port3`), with **`0%` cumulative
        packet loss** across all primary and background streams.
*   **Step 2 — Complete Soft-Drain (`0%` Traffic on `port2`)**:
    *   Complete the soft-drain of `port2` (`NH 101` / `NH 201`) while traffic
        continues uninterrupted:
        *   Issue `gRIBI.Modify` (`REPLACE` with `RIB_AND_FIB_ACK`) updating
            `NextHopGroup` `20` to `{NH 102: weight 1}` and `NextHopGroup` `21`
            to `{NH 202: weight 1}`, cleanly removing `NH 101` and `NH 201` from
            active WCMP forwarding while keeping `NH 101` and `NH 201`
            installed in the `NextHop` AFT table.
        *   *(Optional Zero-Weight Capability Check)*: On platforms that
            additionally support `weight = 0` on `NextHopGroup` members, verify
            `{NH 101: weight 0, NH 102: weight 1}` drains `port2` to `0`
            data-plane packets; on platforms that enforce positive weights
            (`weight >= 1`) per `NextHop` member, verify that a `weight = 0`
            update on a scratch `NextHopGroup` returns a clean gRIBI failure
            result without disrupting the active `{NH 102: weight 1}` /
            `{NH 202: weight 1}` forwarding state.
    *   Await `FIB_PROGRAMMED` acknowledgement and verify via `gNMI.Subscribe`
        (`ON_CHANGE`) and `gRIBI.Get` (`aft = NEXTHOP_GROUP`) that `NHG 20` and
        `NHG 21` forward exclusively via `NH 102` and `NH 202`.
*   **Step 3 — Data-Plane Drain & Control-Plane Session Non-Disruption Check**:
    *   Verify **`0` data-plane flow packets** from `Flow-WCMP-v4` and
        `Flow-WCMP-v6` egress DUT `port2`, while **`100%`** of both primary
        streams egress DUT `port3` with **`0%` cumulative packet loss** (`Tx ==
        Rx`) across the entire `1:1 -> 1:63 -> drained` transition.
    *   Verify via `gNMI.Get` / `gNMI.Subscribe` that:
        *   DUT `port2` remains physically and operationally up
            (`/interfaces/interface[name=port2]/state/oper-status = UP`,
            `admin-status = UP`).
        *   The eBGP IPv4 (`192.0.2.6`) and IPv6 (`2001:db8:2::2`) sessions on
            `port2` (and `port3`) remain `ESTABLISHED` with **zero**
            increments in `state/established-transitions`.
        *   The IS-IS Level-2 adjacency on `port2` (and `port3`) remains `UP`
            with zero adjacency state transitions.
        *   Background traffic on `port4` (`Flow-BG-v4`, `Flow-BG-v6`) sustains
            `0%` packet loss.

### TE-3.11.3 - Drained Link Down and Link Bring-Up Maintenance Isolation

*   While `NH 101` and `NH 201` remain drained out of `NextHopGroup`s `20` and
    `21` and primary + background traffic streams continue transmitting:
    *   Set ATE `port2` link state to `DOWN` via OTG `SetControlState`,
        immediately registering a `defer` cleanup callback to guarantee ATE
        `port2` is restored to `UP` even if a subsequent assertion fails.
    *   Await `/interfaces/interface[name=port2]/state/oper-status = DOWN` via
        `gNMI.Subscribe` (`ON_CHANGE`).
    *   Verify `Flow-WCMP-v4`, `Flow-WCMP-v6` (on `port3`), and `Flow-BG-v4`,
        `Flow-BG-v6` (on `port4`) experience **`0` lost packets (`0%` loss)**
        when the drained link goes down.
    *   Restore ATE `port2` link state to `UP` via OTG `SetControlState`, await
        DUT `port2` `oper-status = UP`, ARP/ND re-resolution, eBGP
        `ESTABLISHED` (`192.0.2.6`, `2001:db8:2::2`), and IS-IS adjacency `UP`
        on `port2` via `gNMI.Subscribe` (`ON_CHANGE`).
    *   Verify that bringing `port2` back `UP` and re-establishing routing
        sessions causes **`0` data-plane packets** to prematurely hash onto
        `port2` prior to explicit gRIBI undrain, and **`0` lost packets** on
        `port3` and `port4`.

### TE-3.11.4 - Hitless Undrain (Gradual Re-Entry to 50/50 WCMP Balance)

*   **Step 1 — Cautious Re-Entry (`1:63` Weight)**:
    *   With traffic still running, issue `gRIBI.Modify` (`REPLACE` with
        `RIB_AND_FIB_ACK`) updating `NextHopGroup` `20` to `{NH 101: weight 1,
        NH 102: weight 63}` and `NextHopGroup` `21` to `{NH 201: weight 1,
        NH 202: weight 63}`.
    *   Await `FIB_PROGRAMMED` and `gNMI` AFT `weight` telemetry convergence,
        then verify `1.56%` (`1/64`) on ATE `port2` and `98.44%` (`63/64`) on
        ATE `port3` within `±0.75%` tolerance (`[0.81%, 2.31%]` on `port2`) with
        `0%` packet loss.
*   **Step 2 — Full Restoration (`1:1` Equal Balance)**:
    *   Issue `gRIBI.Modify` (`REPLACE` with `RIB_AND_FIB_ACK`) restoring
        `NextHopGroup` `20` (`{NH 101: weight 1, NH 102: weight 1}`) and
        `NextHopGroup` `21` (`{NH 201: weight 1, NH 202: weight 1}`).
    *   Await `FIB_PROGRAMMED` and `gNMI` AFT `weight` telemetry convergence,
        stop traffic after steady-state sampling, and verify:
        *   `Flow-WCMP-v4` and `Flow-WCMP-v6` return to a `50% / 50%` (`±2%`
            tolerance) distribution across ATE `port2` and ATE `port3`.
        *   Total end-to-end packet loss across `Flow-WCMP-v4`, `Flow-WCMP-v6`,
            `Flow-BG-v4`, and `Flow-BG-v6` is **`0%`**.
        *   All eBGP and IS-IS adjacencies on `port2` and `port3` remain healthy
            (`ESTABLISHED` / `UP`) with zero flaps during the undrain sequence.

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
  /interfaces/interface/state/counters/in-pkts:
  /interfaces/interface/state/counters/out-pkts:
  /interfaces/interface/state/oper-status:
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
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/adjacency-state:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
      on_change: true
  gribi:
    gRIBI.Flush:
    gRIBI.Get:
    gRIBI.Modify:
```

## Required DUT platform

* FFF
