# LB-1.1: wECMP Hashing Polarization Detection

## Summary

Validate that the DUT's hashing implementation does not produce
polarization when traffic traverses multiple levels of weighted
forwarding paths. The test sends traffic through a two-level recursive
gRIBI hierarchy, captures packets on a target port, then replays only
those captured flows after perturbing the hash configuration. If the
hash is non-linear, the replayed flows scatter across all ports; if
linear, they remain correlated and the test fails.

## Topology

```
                  +-------+
   ATE:port1 --->| port1 |
                 |       |
                 |  DUT  |---> port2 \  LAG 1 (capture + assert on port2)
                 |       |---> port3 /
                 |       |
                 |       |---> port4 \  LAG 2 (mixed dump bucket)
                 +-------+---> port5 /
```

*   **Ingress**: ATE port1 -> DUT port1 (L3 interface). Port1 is not a
    polarization measurement point.
*   **LAG 1** (ports 2, 3): target path. **port2** is the only port used
    for capture and the polarization assertion. **port3** is the other
    member of the same LAG and is expected to carry a similar share, but
    is not asserted independently.
*   **LAG 2** (ports 4, 5): secondary path used as a mixed dump bucket.
    Three recursive next-hops land on the same bundle, so port4 and port5
    cannot be used to detect polarization.

With default weights the forwarding tree is:

```
ATE port1
   |
   v
198.51.100.0/24
   |
   v
NHG 101  (8:1)
   |
   |-- 8/9 (~88.9%) --> NH 1011 --> 203.0.113.1 --> NHG 2010 (8:1)
   |                        |
   |                        |-- 8/9 of 88.9% = ~79.0% --> NH 1501 --> LAG1
   |                        |                                    |
   |                        |                          +---------+---------+
   |                        |                          |                   |
   |                        |                        port2              port3
   |                        |                       ~39.5%             ~39.5%
   |                        |                    (capture+assert)     (same path,
   |                        |                                         not asserted)
   |                        |
   |                        `-- 1/9 of 88.9% = ~9.9% --> NH 1601 ----+
   |                                                                  |
   `-- 1/9 (~11.1%) --> NH 1012 --> 203.0.113.2 --> NHG 3000 (1:1)    |
                                |                                     |
                                |-- 1/2 of 11.1% = ~5.6% --> NH 1602 -+
                                |                                     |
                                `-- 1/2 of 11.1% = ~5.6% --> NH 1603 -+
                                                                      |
                                                                      v
                                                             LAG2 (mixed bucket)
                                                         three recursive NHs
                                                         land on the same bundle
                                                         (~9.9% + ~5.6% + ~5.6%)
                                                                      |
                                                            +---------+---------+
                                                            |                   |
                                                          port4               port5
                                                         ~10.5%              ~10.5%
```

## Procedure

### Setup

1.  Configure DUT port1 as a routed L3 interface.
2.  Configure two static LAGs on the DUT:
    *   LAG 1 with ports 2 and 3.
    *   LAG 2 with ports 4 and 5.
3.  Program gRIBI entries to create a two-level recursive weighted
    forwarding hierarchy for `198.51.100.0/24`:

    ```
    198.51.100.0/24 --> NHG 101 (weight 8:1)
    |
    +--[8/9]--> NH 1011 --> 203.0.113.1 --> NHG 2010 (weight 8:1)
    |            +--[8/9]--> NH 1501 --> LAG1 (port2, port3)
    |            +--[1/9]--> NH 1601 --> LAG2 (port4, port5)
    |
    +--[1/9]--> NH 1012 --> 203.0.113.2 --> NHG 3000 (weight 1:1)
                 +--[1/2]--> NH 1602 --> LAG2 (port4, port5)
                 +--[1/2]--> NH 1603 --> LAG2 (port4, port5)
    ```

    All NHG weights are defined as named constants in the test for easy
    tuning. The expected per-port distribution is computed automatically
    from these weights. With the default weights, port2 receives ~39.5%
    of traffic.

4.  Configure two static LAGs on the ATE that mirror the DUT bundles
    (ports 2+3 and ports 4+5). Each gRIBI next-hop is an ATE device on
    one of those LAGs: NH 1501 on LAG 1, and NHs 1601, 1602, and 1603 on
    LAG 2. The ATE and the DUT resolve every adjacency with ARP; no static
    neighbors are configured. Because both sides are LAGs, ARP works
    whichever member carries the request or reply.
5.  Generate a large set of unique IPv4/UDP flow tuples (varying source IP and
    UDP ports, fixed destination IP within `198.51.100.0/24`). All addresses are
    confined to reserved ranges only — RFC 5737 documentation blocks
    (`192.0.2.0/24`, `198.51.100.0/24`, `203.0.113.0/24`) and the RFC 2544
    benchmarking block (`198.18.0.0/15`) — so no test traffic can leak onto or
    spoof a real public network.

### Test: Iterative Replay

Run a **Baseline** round followed by a set of **Replay** rounds. Each
round:

1.  **Perturb hash**: Apply a vendor-specific configuration change to
    alter how the DUT makes path selections.

    | Vendor  | Mechanism                                       |
    |---------|-------------------------------------------------|
    | Cisco   | Loopback0 IPv4 address change (OC, gNMI Replace) |

    On Cisco the perturbation is pure OpenConfig: each round replaces
    `/interfaces/interface[name=Loopback0]` with a new `/32` address
    (`10.10.10.10`, `10.11.11.11`, ...). IOS-XR feeds the loopback address
    into the ECMP hash, so every round uses a different hash input. See
    the Canonical OC below for the Baseline round.

    Other vendors should add their implementation to `perturbHashConfig`
    in the test file.

2.  **Send traffic in batches**: Inject the current flow list through
    ATE port1 in a configurable batch count of packets. Each batch: push config,
    start capture on port2, send traffic, stop and download capture,
    accumulate captured packets and per-port Rx counters. Each batch also
    asserts the flow is delivered without loss (Rx ≈ Tx).
3.  **Assert**: Two independent checks, in order:
    *   **Loss** (`lossTolerancePct`, 0.2%): frames received across
        ports2-5 must account for nearly all packets sent this round.
        Drops on ports3-5 cannot be masked by the port2-only distribution
        check.
    *   **Imbalance / polarization** (`tolerancePct`, 2%): port2 received
        the expected share of total packets (derived from NHG weights).
4.  **Feed back**: The packets captured on port2 across all batches
    become the sole input for the next round. The packet count shrinks
    each round (~39.5% retained per round with default weights).

### Why Replay Detects Polarization

After the Baseline, we know exactly which flows hashed to port2. If we
replay only those flows after perturbing the hash and the hash is
non-linear, the flows scatter across all ports — with the default 8:1
weights, only ~39.5% should land on port2 again. If the hash is
linear, perturbing it shifts all flows by the same constant; flows
that grouped together stay grouped, and nearly 100% of the replayed
flows land on port2 again.

### Why Batch Packets

The Ixia hardware capture buffer holds a limited number of packets per port. Sending
more packets per batch than this the limit (different in different hardware) causes captured packets to be
overwritten. The configurable batch size ensures every packet arriving on
port2 is captured and available for replay in the next round. 

### Pass/Fail Criteria

*   **Pass**: In every round, loss stays within 0.2% (frames received
    across ports2-5 ≈ packets sent) and port2 receives the expected share
    of injected packets (within 2% imbalance of total).
*   **Fail**: The DUT drops forwarded traffic on any of ports2-5, or port2
    receives significantly more or less than expected, indicating flows remained
    correlated (polarized) despite the hash perturbation.

## Canonical OC

DUT configuration pushed by the test, shown for the Baseline round.
Interface names are placeholders. The test uses the testbed port names and
the next free aggregate IDs. `Loopback0` carries the hash perturbation, and
its `/32` address changes every round. The `neighbors` entries are `state`
only: the test configures no static ARP and waits for the DUT to learn each
gRIBI next-hop with ARP.

```json
{
  "interfaces": {
    "interface": [
      {
        "name": "port1",
        "config": {
          "name": "port1",
          "description": "DUT Port 1",
          "type": "ethernetCsmacd"
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "config": {
                "index": 0
              },
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "ip": "192.0.2.1",
                      "config": {
                        "ip": "192.0.2.1",
                        "prefix-length": 30
                      }
                    }
                  ]
                }
              }
            }
          ]
        }
      },
      {
        "name": "port2",
        "config": {
          "name": "port2",
          "type": "ethernetCsmacd",
          "enabled": true
        },
        "ethernet": {
          "config": {
            "aggregate-id": "lag1"
          }
        }
      },
      {
        "name": "port3",
        "config": {
          "name": "port3",
          "type": "ethernetCsmacd",
          "enabled": true
        },
        "ethernet": {
          "config": {
            "aggregate-id": "lag1"
          }
        }
      },
      {
        "name": "port4",
        "config": {
          "name": "port4",
          "type": "ethernetCsmacd",
          "enabled": true
        },
        "ethernet": {
          "config": {
            "aggregate-id": "lag2"
          }
        }
      },
      {
        "name": "port5",
        "config": {
          "name": "port5",
          "type": "ethernetCsmacd",
          "enabled": true
        },
        "ethernet": {
          "config": {
            "aggregate-id": "lag2"
          }
        }
      },
      {
        "name": "lag1",
        "config": {
          "name": "lag1",
          "type": "ieee8023adLag",
          "enabled": true
        },
        "aggregation": {
          "config": {
            "lag-type": "STATIC"
          }
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "config": {
                "index": 0
              },
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "ip": "198.19.1.1",
                      "config": {
                        "ip": "198.19.1.1",
                        "prefix-length": 24
                      }
                    }
                  ]
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "ip": "198.19.1.23",
                      "state": {
                        "ip": "198.19.1.23",
                        "link-layer-address": "02:00:23:00:15:01"
                      }
                    }
                  ]
                }
              }
            }
          ]
        }
      },
      {
        "name": "lag2",
        "config": {
          "name": "lag2",
          "type": "ieee8023adLag",
          "enabled": true
        },
        "aggregation": {
          "config": {
            "lag-type": "STATIC"
          }
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "config": {
                "index": 0
              },
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "ip": "198.19.2.1",
                      "config": {
                        "ip": "198.19.2.1",
                        "prefix-length": 24
                      }
                    }
                  ]
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "ip": "198.19.2.24",
                      "state": {
                        "ip": "198.19.2.24",
                        "link-layer-address": "02:00:45:00:16:01"
                      }
                    },
                    {
                      "ip": "198.19.2.2",
                      "state": {
                        "ip": "198.19.2.2",
                        "link-layer-address": "02:00:45:00:16:02"
                      }
                    },
                    {
                      "ip": "198.19.2.3",
                      "state": {
                        "ip": "198.19.2.3",
                        "link-layer-address": "02:00:45:00:16:03"
                      }
                    }
                  ]
                }
              }
            }
          ]
        }
      },
      {
        "name": "Loopback0",
        "config": {
          "name": "Loopback0",
          "type": "softwareLoopback",
          "enabled": true
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "config": {
                "index": 0,
                "enabled": true
              },
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "ip": "10.10.10.10",
                      "config": {
                        "ip": "10.10.10.10",
                        "prefix-length": 32
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

## Config Parameter Coverage

*   Interfaces: name, description, type, enabled
*   Subinterfaces: index, enabled, IPv4 address and prefix-length
*   Static LAG: `aggregation/config/lag-type` and member
    `ethernet/config/aggregate-id`
*   Hash perturbation (Cisco): `Loopback0` subinterface IPv4 address,
    replaced every round

## Telemetry Parameter Coverage

N/A

## Protocol/RPC Parameter Coverage

*   gRIBI
    *   Modify: NextHopEntry, NextHopGroupEntry (with weights),
        IPv4Entry
    *   Flush

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /interfaces/interface/config/name:
  /interfaces/interface/config/description:
  /interfaces/interface/config/type:
  /interfaces/interface/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/config/index:
  /interfaces/interface/subinterfaces/subinterface/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/state/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/state/link-layer-address:
  /interfaces/interface/ethernet/config/aggregate-id:
  /interfaces/interface/aggregation/config/lag-type:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
  gribi:
    gRIBI.Modify:
    gRIBI.Flush:
```

## TODO

*   Add IP-in-IP encap flow variant
*   Add IP-in-IP decap flow variant
*   Add IP-in-IP transit flow variant
*   Optional: extra TGEN ports or a two-DUT topology so polarization can
    be measured on every LAG member instead of port2 only

## Minimum DUT Platform Requirement

N/A
