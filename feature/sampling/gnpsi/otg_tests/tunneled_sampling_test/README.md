# gNPSI-2: gNPSI Packet Sampling for Tunneled Traffic

## Summary

Validate that the gRPC Network Packet Sampling Interface (`gNPSI`) service
accurately samples tunneled packets across IP-in-IP (`IPv4-in-IPv4`,
`IPv6-in-IPv4`, `IPv6-in-IPv6`), GRE (`MPLSoGRE` / `GRE`), `MPLSoUDP`, and MPLS
encapsulations, streaming valid sFlow v5 records over `gNPSI.Subscribe` that
preserve both outer tunnel headers and inner encapsulated packet headers within
the configured sample size (`256` bytes).

In backbone and core deployments, transit and locally encapsulated traffic
traverses IP-in-IP, GRE, UDP, or MPLS tunnels programmed via `gRIBI` or
OpenConfig policy-forwarding. Packet sampling collectors rely on `gNPSI` sFlow
records to inspect both outer tunnel endpoints and inner 5-tuple flow headers
for traffic engineering and telemetry accuracy. This test verifies header
completeness, interface `ifindex` metadata, statistical sampling rate accuracy,
and negative isolation on unsampled ports and dropped flows.

## Testbed type

* [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

```text
+------------+                     +------------+
|    ATE     |  port1 (Smp Ing)    |    DUT     |
|            |---------------------|            |
|            |  port2 (Smp Egr)    |            |
|            |---------------------|            |
|            |  port3 (Unsmp Egr)  |            |
|            |---------------------|            |
|            |  port4 (Unsmp Ing)  |            |
|            |---------------------|            |
+------------+                     +------------+
```

## Procedure

### Test environment setup

*   Configure `4` physical interfaces connecting the DUT and ATE with dual-stack
    IPv4 (`/30`) and IPv6 (`/126`) addressing in the `DEFAULT` network instance:
    *   **Port 1 (Sampled Ingress)**: Connect DUT `port1` (`192.0.2.1/30`,
        `2001:db8:1::1/126`) to ATE `port1` (`192.0.2.2/30`,
        `2001:db8:1::2/126`).
    *   **Port 2 (Sampled Tunnel Egress)**: Connect DUT `port2` (`192.0.2.5/30`,
        `2001:db8:2::1/126`) to ATE `port2` (`192.0.2.6/30`,
        `2001:db8:2::2/126`).
    *   **Port 3 (Unsampled Egress Control)**: Connect DUT `port3`
        (`192.0.2.9/30`, `2001:db8:3::1/126`) to ATE `port3` (`192.0.2.10/30`,
        `2001:db8:3::2/126`).
    *   **Port 4 (Unsampled Ingress Control)**: Connect DUT `port4`
        (`192.0.2.13/30`, `2001:db8:4::1/126`) to ATE `port4` (`192.0.2.14/30`,
        `2001:db8:4::2/126`).
*   Start OTG protocols and explicitly verify ARP and IPv6 Neighbor Discovery
    (ND) resolution across all `4` links before starting traffic.
*   Query `/interfaces/interface/state/ifindex` via `gNMI.Get` (batched with
    `OCBatch`) for DUT `port1`, `port2`, `port3`, and `port4` to establish
    expected ingress and egress `ifindex` identifiers in sFlow records.
*   Configure OpenConfig packet sampling (`/sampling/sflow`) on the DUT via
    `gNMI.Set`:
    *   Enable global sFlow (`/sampling/sflow/config/enabled = true`) with
        `/sampling/sflow/config/sample-size = 256` bytes and agent identifier
        `192.0.2.1` (`2001:db8:1::1`).
    *   Enable ingress packet sampling on `port1` (`enabled = true`,
        `ingress-sampling-rate = 10000`, or platform minimum supported hardware
        rate `R` while scaling transmitted packets to `100 * R`) to sample
        packets entering the DUT on `port1`.
    *   Enable egress packet sampling on `port2` (`enabled = true`,
        `egress-sampling-rate = 10000`, or global `/sampling/sflow/config`
        sampling rate where per-interface egress rate configuration is
        unsupported) to sample post-encapsulation tunnel packets egressing
        `port2`.
    *   Keep sFlow disabled (`enabled = false`) on `port3` and `port4` for
        negative isolation checks.
*   Verify via `gNMI.Subscribe` (`ON_CHANGE`) that `/sampling/sflow/state` and
    per-interface sampling states reflect the configured values without static
    sleeps.
*   Establish a `gRIBI` leader session (`PRESERVE`, `SINGLE_PRIMARY`,
    `RIB_AND_FIB_ACK`) and flush `DEFAULT` prior to programming tunnel routes.
*   Enable the `gNPSI` gRPC service with the `sFlow` packet-sampling source on
    the DUT (pending public OpenConfig YANG standardization in
    `openconfig/public#1385`), connect a gRPC client, and open an active
    `gNPSI.Subscribe` stream (`gnpsi.Sample`), verifying the initial stream
    message reports `sflow_metadata.version = V5` before collecting streamed
    sFlow v5 `packet` payloads and `timestamp`s.

### gNPSI-2.1 - IP-in-IP Tunneled Traffic Sampling (IPv4 and IPv6)

#### Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "Sampled Ingress Port 1",
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
          "description": "Sampled Tunnel Egress Port 2",
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
          "description": "Unsampled Egress Port 3",
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
          "description": "Unsampled Ingress Port 4",
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
        "name": "DEFAULT"
      }
    ]
  },
  "sampling": {
    "sflow": {
      "config": {
        "agent-id-ipv4": "192.0.2.1",
        "agent-id-ipv6": "2001:db8:1::1",
        "enabled": true,
        "sample-size": 256
      },
      "interfaces": {
        "interface": [
          {
            "config": {
              "enabled": true,
              "ingress-sampling-rate": 10000,
              "name": "port1"
            },
            "name": "port1"
          },
          {
            "config": {
              "egress-sampling-rate": 10000,
              "enabled": true,
              "name": "port2"
            },
            "name": "port2"
          },
          {
            "config": {
              "enabled": false,
              "name": "port3"
            },
            "name": "port3"
          },
          {
            "config": {
              "enabled": false,
              "name": "port4"
            },
            "name": "port4"
          }
        ]
      }
    }
  }
}
```

*   **Step 1 — Program IP-in-IP Encapsulation and Transit Tunnel Routes**:
    *   Push the Canonical OC configuration via `gNMI.Set` (`REPLACE`).
    *   Via `gRIBI.Modify` (`RIB_AND_FIB_ACK`) in `DEFAULT`, program:
        *   Underlay `NextHop` `1` (`ip_address = "192.0.2.6"`, ATE `port2`) ->
            `NextHopGroup` `10` -> outer tunnel destination `IPv4Entry`
            `203.0.113.0/24`.
        *   Underlay `NextHop` `2` (`ip_address = "2001:db8:2::2"`, ATE `port2`)
            -> `NextHopGroup` `20` -> outer tunnel destination `IPv6Entry`
            `2001:db8:200::/64`.
        *   IPv4 IP-in-IP Encapsulating `NextHop` `101` (`encapsulate_header =
            IPV4`, `ip_in_ip = {src_ip: "198.51.100.1", dst_ip:
            "203.0.113.10"}`) -> `NextHopGroup` `100`, referenced by inner
            prefixes `100.64.1.0/24` (`IPv4-in-IPv4`) and `2001:db8:10::/64`
            (`IPv6-in-IPv4`).
        *   Note: `Flow-v6-in-v6` enters `port1` pre-encapsulated with outer
            destination `2001:db8:200::10` and inner destination
            `2001:db8:11::/64`, forwarding via outer `IPv6Entry`
            `2001:db8:200::/64` -> `NextHopGroup` `20`.
    *   Confirm `FIB_PROGRAMMED` acknowledgements and verify programmed AFT
        entries via both `gRIBI.Get` (`network_instance: {name: "DEFAULT"}`,
        `aft: ALL`) and `gNMI.Subscribe` (`ON_CHANGE`).
*   **Step 2 — Generate IP-in-IP Traffic and Validate `gNPSI` sFlow Samples**:
    *   Transmit `1,000,000` packets (`512`-byte frame size, `100,000 pps` for
        `10` seconds; or `100 * R` packets at platform sampling rate `1:R`) per
        flow profile from ATE `port1` (`Flow-v4-in-v4`, `Flow-v6-in-v4`, and
        pre-encapsulated transit `Flow-v6-in-v6` to `2001:db8:200::10`),
        embedding a deterministic inner UDP source/dest port (`5001` -> `6001`)
        and payload signature.
    *   Verify `0%` packet loss at ATE `port2` across all `3` streams.
    *   Inspect the sFlow v5 datagrams received on the `gNPSI.Subscribe` client:
        *   Verify sFlow datagram version is `5` and `agent_address` matches
            `192.0.2.1` (or `2001:db8:1::1`).
        *   Partition received `FlowSample` records by sFlow `source_id`
            (`port1` ingress vs. `port2` egress `ifindex`) and verify each
            active sampler independently produces within `±20%` of the expected
            `100` samples (`80`–`120` samples per active sampler per
            `1,000,000`-packet stream at `1:10,000`).
        *   Decode each `raw_packet_header` (`header_protocol = 1` Ethernet)
            within the `256`-byte capture according to sampler direction and
            encapsulation point:
            *   **DUT-Encapsulated Flows (`Flow-v4-in-v4`, `Flow-v6-in-v4`)**:
                Verify `port1` ingress samples (captured prior to DUT tunnel
                encapsulation) contain the valid unencapsulated inner Ethernet +
                IPv4 (`100.64.1.x`) / IPv6 (`2001:db8:10::x`) + UDP (`5001` ->
                `6001`) header and payload signature, whereas `port2` egress
                samples (captured post encapsulation) contain **both** the outer
                IPv4 tunnel header (`src = 198.51.100.1`, `dst = 203.0.113.10`,
                protocol `4` for IPv4 or `41` for IPv6) and the intact inner
                IPv4/IPv6 + UDP (`5001` -> `6001`) headers and payload
                signature.
            *   **Pre-Encapsulated Transit Flow (`Flow-v6-in-v6`)**: Verify
                **both** `port1` ingress and `port2` egress samples contain
                **both** the outer IPv6 tunnel header (`dst = 2001:db8:200::10`,
                `next_header = 41`) and the intact inner IPv6
                (`2001:db8:11::x`, preserved inner DSCP/hop-limit) + UDP
                (`5001` -> `6001`) headers and payload signature.
        *   Verify sFlow record metadata reports `sampling_rate = 10000` (or
            `R`), `input` interface matching `port1` `ifindex`, and `output`
            interface matching `port2` `ifindex`.

### gNPSI-2.2 - GRE and MPLSoGRE / MPLSoUDP Tunneled Traffic Sampling

*   **Step 1 — Transmit GRE and MPLSoUDP Encapsulated Flows**:
    *   Using the active outer tunnel routes (`203.0.113.0/24` -> ATE `port2`
        `192.0.2.6` and `2001:db8:200::/64` -> ATE `port2` `2001:db8:2::2`) —
        and policy-forwarding encapsulation rules where supported by the DUT —
        transmit `1,000,000` packets (`512`-byte frames, `100,000 pps`) from ATE
        `port1` to ATE `port2` for each encapsulation profile:
        *   **GRE (`Flow-GRE-v4`, `Flow-GRE-v6`)**: Outer IPv4 (`198.51.100.2`
            -> `203.0.113.20`, IP protocol `47`), GRE header (`protocol_type =
            0x0800` for inner IPv4 `100.64.2.10` and `0x86DD` for inner IPv6
            `2001:db8:20::10`), inner UDP (`5002` -> `6002`), and payload
            signature.
        *   **MPLSoGRE / MPLSoUDP (`Flow-MPLSoUDP`)**: Outer IPv4
            (`198.51.100.2` -> `203.0.113.30`, UDP destination port `6635`),
            MPLS label `1042` (`S = 1`, `TC = 4`, `TTL = 64`), inner IPv4
            (`100.64.3.10`) / inner IPv6 (`2001:db8:30::10`), inner UDP (`5003`
            -> `6003`), and payload signature.
*   **Step 2 — Validate `gNPSI` Header Integrity**:
    *   Verify `0%` packet loss at ATE `port2`.
    *   Decode the `gNPSI.Subscribe` sFlow v5 `raw_packet_header` samples for
        each profile and verify:
        *   Outer IPv4 header + `4`-byte/`8`-byte GRE header (or `8`-byte UDP
            `6635` + `4`-byte MPLS shim `1042`) + inner IPv4/IPv6 header + inner
            UDP header (`5002`/`5003` -> `6002`/`6003`) are completely present
            and parseable without byte-offset misalignment within `256` bytes.
        *   Partitioned by sFlow `source_id` (`port1` ingress vs. `port2` egress
            `ifindex`), each active sampler produces `100 ± 20%` (`80`–`120`)
            samples per stream and reports accurate `port1` / `port2` `ifindex`
            values.

### gNPSI-2.3 - MPLS Label-Switched Packet Sampling

*   **Step 1 — Configure MPLS Forwarding / Transit Path**:
    *   On platforms supporting gRIBI `MPLS` (`LabelEntry`) or OpenConfig static
        MPLS LSPs, program an MPLS label-swap/push entry (incoming/pushed
        transport label `1001` -> egress `port2` `192.0.2.6` with service label
        `2001`, `S = 1`). On platforms that return `UNIMPLEMENTED` for gRIBI
        `MPLS` AFTs, configure static MPLS forwarding if supported or transmit
        MPLS-over-IP/Ethernet frames sampled on `port1` ingress so `gNPSI` MPLS
        header parsing is validated cleanly.
*   **Step 2 — Validate MPLS Stack and Inner Payload in `gNPSI` Samples**:
    *   Transmit `1,000,000` MPLS-encapsulated IPv4 and IPv6 packets (`512`
        bytes, Ethernet `0x8847`, two-label stack `[1001, 2001]` with `S = 1` on
        `2001`, inner IP `100.64.4.10` / `2001:db8:40::10`, inner UDP `5004` ->
        `6004`).
    *   Verify `gNPSI.Subscribe` sFlow v5 samples (partitioned by `source_id` on
        `port1` ingress and, where forwarded to `port2`, `port2` egress) retain
        the `0x8847` Ethernet frame, full MPLS label stack (exact label IDs, TC
        bits, `S` bit, TTL), and inner IPv4/IPv6 + UDP headers within the
        `256`-byte sample window at `100 ± 20%` (`80`–`120`) samples per active
        sampler.

### gNPSI-2.4 - Negative Validation: Unsampled Ports and Dropped Traffic

*   **Step 1 — Unsampled Ingress-to-Egress Path (`port4` -> `port3`)**:
    *   Via `gRIBI.Modify` (`RIB_AND_FIB_ACK`), program outer tunnel route
        `203.0.113.128/25` -> `NextHop` `3` (`192.0.2.10`, ATE `port3`, where
        sFlow is `enabled = false`).
    *   Transmit `1,000,000` IP-in-IP packets (`Flow-Unsampled`, inner UDP
        `5999` -> `6999`) from unsampled ATE `port4` destined to
        `203.0.113.130`.
    *   Verify `100%` of `Flow-Unsampled` packets arrive at ATE `port3`, while
        **`0` sFlow samples** matching `Flow-Unsampled` (`port4`/`port3`
        `ifindex` or UDP `5999` -> `6999`) are received on `gNPSI.Subscribe`.
*   **Step 2 — Unresolved / Dropped Tunnel Destination**:
    *   Configure a static or gRIBI route for `198.51.100.128/25` with a drop /
        unresolved next-hop so packets are discarded prior to egressing `port2`.
    *   Transmit `1,000,000` IP-in-IP packets (`Flow-Dropped`, inner UDP `5998`
        -> `6998`) from unsampled ATE `port4` destined to `198.51.100.130`.
    *   Verify `0` packets egress ATE `port2` and **`0` egress sFlow samples**
        referencing `port2` as `output` interface are emitted over
        `gNPSI.Subscribe`.

### Cleanup

*   Stop all OTG traffic streams and close the `gNPSI.Subscribe` client stream.
*   Execute `gRIBI.Flush` on `DEFAULT` and remove `/sampling/sflow` and test
    interface/routing configurations via `gNMI.Set` registered in `t.Cleanup()`,
    restoring the DUT to its pre-test state.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  ## Config Parameter Coverage
  /interfaces/interface/config/description:
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
  /sampling/sflow/config/agent-id-ipv4:
  /sampling/sflow/config/agent-id-ipv6:
  /sampling/sflow/config/enabled:
  /sampling/sflow/config/sample-size:
  /sampling/sflow/interfaces/interface/config/egress-sampling-rate:
  /sampling/sflow/interfaces/interface/config/enabled:
  /sampling/sflow/interfaces/interface/config/ingress-sampling-rate:
  /sampling/sflow/interfaces/interface/config/name:

  ## Telemetry Parameter Coverage
  /interfaces/interface/state/counters/in-pkts:
  /interfaces/interface/state/counters/out-pkts:
  /interfaces/interface/state/ifindex:
  /interfaces/interface/state/oper-status:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hops/next-hop/ip-in-ip/state/dst-ip:
  /network-instances/network-instance/afts/next-hops/next-hop/ip-in-ip/state/src-ip:
  /network-instances/network-instance/afts/next-hops/next-hop/state/encapsulate-header:
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
  /sampling/sflow/interfaces/interface/state/egress-sampling-rate:
  /sampling/sflow/interfaces/interface/state/enabled:
  /sampling/sflow/interfaces/interface/state/ingress-sampling-rate:
  /sampling/sflow/interfaces/interface/state/name:
  /sampling/sflow/state/agent-id-ipv4:
  /sampling/sflow/state/agent-id-ipv6:
  /sampling/sflow/state/enabled:
  /sampling/sflow/state/sample-size:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
      on_change: true
  gnpsi:
    gNPSI.Subscribe:
  gribi:
    gRIBI.Flush:
    gRIBI.Get:
    gRIBI.Modify:
```

## Required DUT platform

* FFF
