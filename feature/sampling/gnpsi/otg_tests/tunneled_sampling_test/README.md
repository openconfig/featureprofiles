# gNPSI-2: gNPSI Packet Sampling for Tunneled Traffic

## Summary

Validate that the gRPC Network Packet Sampling Interface (`gNPSI`) service
accurately samples tunneled packets and streams valid sFlow v5 records over
`gNPSI.Subscribe` that preserve both the outer tunnel headers and the inner
encapsulated packet headers within the configured sample size (`256` bytes).
The DUT samples packets at ingress on `port1` and at egress on `port2`. The
test covers:

*   IP-in-IP: `IPv4-in-IPv4` and `IPv6-in-IPv4` encapsulated by the DUT through
    a `gRIBI` encapsulation next hop, and `IPv6-in-IPv6` routed by the DUT on
    its outer header (transit).
*   GRE (IPv4 and IPv6 payload), MPLSoGRE and MPLSoUDP tunnels built by the ATE
    and routed by the DUT on the outer IPv4 header (transit).
*   MPLS label-switched packets forwarded by a static LSP label swap.
*   Negative isolation: no samples from interfaces where sampling is disabled,
    and no sample that reports traffic dropped by the DUT as forwarded.
*   `IPv4-in-IPv4` and `IPv6-in-IPv4` packets that a policy-forwarding VRF
    selection rule steers to a decapsulation network instance, where a `gRIBI`
    decapsulation entry removes the outer header before the inner packet is
    forwarded.

In backbone and core deployments, transit traffic and traffic that the router
encapsulates or decapsulates crosses IP-in-IP, GRE, UDP and MPLS tunnels
programmed through `gRIBI`, static MPLS LSPs and OpenConfig policy-forwarding.
Packet sampling collectors rely on `gNPSI` sFlow records to inspect both the
outer tunnel endpoints and the inner 5-tuple flow headers for traffic
engineering and telemetry accuracy. A sampler that under-samples packets taking
a tunnel-specific forwarding path, such as decapsulation after policy-based VRF
selection, silently corrupts traffic estimates. For every stream and sampler
the test therefore verifies header completeness, interface `ifindex`,
sampling-rate and frame-length metadata, the sample count against the
configured rate within a 4-sigma bound, and the sFlow sequence-number,
`sample_pool` and `drops` accounting. Where the platform emits sFlow tunnel or
MPLS extended flow records, their content is validated as well.

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

*   All `4` DUT-ATE links run at the same speed of at least `10GE`. No LAG or
    aggregate interface is used.
*   DUT `port1`-`port4` are routed interfaces (subinterface `0`) in the
    `DEFAULT` network instance. The `DECAP_TE_VRF` network instance created in
    gNPSI-2.5 has no interfaces; it is only the decapsulation lookup table.
*   Streams are sent one at a time, except in gNPSI-2.4, where a reference
    stream runs concurrently with the stream under test. The per-stream rate
    cap defined in the test parameters keeps the offered load on every link at
    or below `50%` of the link rate (`0.85 Gb/s` on `port1` in gNPSI-2.4 at
    `N = 10,000`).

## Procedure

### Test environment setup

#### Test parameters

*   Sampling rate `N = 10,000` (`1:10,000`) for both the `port1` ingress
    sampler and the `port2` egress sampler. A platform that cannot sample at
    `1:10,000` uses its smallest supported rate above `10,000`; the test logs
    the value used, and every quantity below scales with `N`.
*   Every stream sends `P = 400 * N` packets (`4,000,000` at `N = 10,000`) of
    fixed `512`-byte frames (including the FCS) at `R = min(10 * N, R_max)`
    packets per second for `T = P / R` seconds, with
    `R_max = 0.5 * L / (k * (532 + 20) * 8)`. `L` is the link speed in bit/s,
    `532` bytes is the largest frame on any link (after DUT encapsulation),
    `20` bytes are the preamble and inter-frame gap, and `k` is the number of
    streams that share a link at the same time (`2` in gNPSI-2.4, otherwise
    `1`). At `N = 10,000`, `R = 100,000` pps and `T = 40` seconds on any link
    of at least `10GE`. A larger `N` that reaches the cap keeps `P` and the
    expected sample count unchanged and stretches `T` instead; for example,
    `N = 100,000` on `10GE` in gNPSI-2.4 gives `R = 566,123` pps and
    `T = 70.7` seconds.
*   Expected sample count: `E = D / N` per sampler and stream, where `D` is the
    DUT counter delta measured for the stream (`in-pkts` of the ingress port
    for the ingress sampler, `out-pkts` of `port2` for the egress sampler). A
    sampler passes when it delivers between `0.8 * E` and `1.2 * E` samples
    for the stream (`320`-`480` when `D = 4,000,000`). For `E = 400` the
    binomial standard deviation is `20`, so the band is `+/-4` sigma: the
    probability that a compliant DUT fails one check is about `6 x 10^-5`, and
    about `2 x 10^-3` over the `31` count checks of this test.
*   Payload signature: in every stream, the first `128` bytes after the
    innermost UDP header are `0x00` and the next `16` bytes are the ASCII
    string `GNPSI2-SIGNATURE`. In every frame of this test, as sent by the ATE
    and as forwarded by the DUT, the signature occupies bytes between frame
    offsets `170` and `245`, so it is captured only if the DUT honors
    `sample-size = 256`; the OpenConfig default of `128` bytes cannot capture
    it.
*   Every stream's Ethernet header is ATE port MAC -> DUT ingress port MAC with
    no VLAN tag. ATE MAC addresses: `port1` `02:00:01:01:01:01`, `port2`
    `02:00:02:01:01:01`, `port3` `02:00:03:01:01:01` and `port4`
    `02:00:04:01:01:01`.
*   Unless a stream definition says otherwise, every IPv4 header sent by the
    ATE has DSCP `32`, ECN `0` and TTL `64`, and every IPv6 header has traffic
    class `0x80` (DSCP `32`), flow label `0` and hop limit `64`.

#### DUT and ATE configuration

*   Configure `4` physical interfaces connecting the DUT and ATE with
    dual-stack IPv4 (`/30`) and IPv6 (`/126`) addressing in the `DEFAULT`
    network instance:
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
*   Configure OpenConfig packet sampling (`/sampling/sflow`) on the DUT:
    *   Enable global sFlow (`/sampling/sflow/config/enabled = true`) with
        `sample-size = 256` bytes, agent identifier `192.0.2.1`
        (`agent-id-ipv4`) and `2001:db8:1::1` (`agent-id-ipv6`), and global
        `ingress-sampling-rate = N`.
    *   Enable ingress packet sampling on `port1` (`enabled = true`,
        `ingress-sampling-rate = N`) to sample packets entering the DUT on
        `port1`.
    *   Enable egress packet sampling on `port2` (`enabled = true`,
        `egress-sampling-rate = N`) to sample packets as transmitted on
        `port2`, after any DUT encapsulation, decapsulation or label swap. On
        platforms with deviation `sflow_egress_sampling_rate_unsupported`, the
        per-interface egress rate cannot be set through OpenConfig: the global
        `ingress-sampling-rate = N` provides the rate and egress sampling is
        enabled on `port2` through the deviation's alternate method, so the
        effective egress rate is `N`. On these platforms the `port2`
        `state/egress-sampling-rate` check is skipped, and the effective egress
        rate is verified from the `sampling_rate` field (`N`) of every `port2`
        egress sample.
    *   Keep sFlow disabled (`enabled = false`) on `port3` and `port4` for
        negative isolation checks; the global `ingress-sampling-rate` must not
        enable sampling on these interfaces.
    *   No UDP sFlow collector is required: `gNPSI.Subscribe` is the only
        export path validated. If the shared sFlow helper used by gNPSI-1 adds
        a collector entry, the datagrams sent to it are not validated, and the
        entry is removed with the rest of `/sampling/sflow` in Cleanup.
*   Enable the `gNPSI` gRPC service on the port that the testbed binding uses
    for `gNPSI` (the `introspect.GNPSI` dialer port). The service identity is
    `openconfig-gnpsi-types:GNPSI` (base `GRPC_SERVICE`):
    *   If `/system/grpc-servers` already has a server on that port whose
        `services` include `openconfig-gnpsi-types:GNPSI` (for example, one
        provisioned with the binding), use it unchanged.
    *   Otherwise create `/system/grpc-servers/grpc-server[name=gnpsi]` with
        `services = [openconfig-gnpsi-types:GNPSI]`, `enable = true` and
        `port` = the binding port (`50051` in the Canonical OC is an example),
        and remove it in Cleanup. Never create a second server on a port that
        an existing server already uses.
    *   On platforms with deviation `gnpsi_oc_unsupported`, which cannot
        enable `gNPSI` through `/system/grpc-servers` (for example, because the
        `services` leaf does not accept `openconfig-gnpsi-types:GNPSI`), enable
        the same service through the deviation's alternate method and skip the
        grpc-server state check; the first `Sample` received on the stream
        still verifies the service.
    *   TODO: OpenConfig has no leaf yet that binds the `gNPSI` server to the
        sFlow packet-sampling source. Until one is published, the binding is
        implementation-defined (implicit, or vendor configuration applied with
        the service).
*   Apply the configuration in the Canonical OC below with `gNMI.Set`
    `REPLACE` scoped to `/interfaces/interface[name=portN]` (each of
    `port1`-`port4`) and to `/sampling/sflow`, a `REPLACE` of
    `/system/grpc-servers/grpc-server[name=gnpsi]` only when the test creates
    that server, and an `UPDATE` of
    `/network-instances/network-instance[name=DEFAULT]/config`. Apply it once,
    before the `gRIBI` and `gNPSI` sessions are opened. Never use a root-level
    `REPLACE`: it would remove the gRPC servers and any deviation-applied
    configuration and tear down both sessions in the middle of the test.
*   Verify `/interfaces/interface/state/oper-status = UP` for DUT
    `port1`-`port4` with `gNMI.Subscribe` (`ON_CHANGE`, `gnmi.Watch` and
    `Await`). Then start OTG protocols and verify ARP and IPv6 Neighbor
    Discovery (ND) resolution on all `4` links (`otgutils.WaitForARP` for IPv4
    and IPv6) before any traffic is started.
*   Query `/interfaces/interface/state/ifindex` and
    `/interfaces/interface/ethernet/state/mac-address` for DUT `port1`-`port4`
    in one `gNMI.Get` (batched with `OCBatch`). The `ifindex` values identify
    samplers and interfaces in sFlow records; the MAC addresses are the
    expected Ethernet addresses in sampled headers and the destination MAC of
    the streams sent to each port.
*   Verify with `gNMI.Subscribe` (`ON_CHANGE`), without static sleeps, that
    `/sampling/sflow/state` reports `enabled = true`, `sample-size = 256`,
    `agent-id-ipv4 = 192.0.2.1`, `agent-id-ipv6 = 2001:db8:1::1` and
    `ingress-sampling-rate = N`; that `port1` reports `enabled = true` and
    `ingress-sampling-rate = N`; that `port2` reports `enabled = true` and
    `egress-sampling-rate = N` (rate check skipped under
    `sflow_egress_sampling_rate_unsupported`); and that `port3` and `port4`
    report `enabled = false`. On platforms without `gnpsi_oc_unsupported`,
    also verify that the state of the gRPC server used for `gNPSI` (existing
    or created) reports `enable = true` and `services` containing
    `openconfig-gnpsi-types:GNPSI`.
*   Establish a `gRIBI` leader session (persistence `PRESERVE`, redundancy
    `SINGLE_PRIMARY`, ack type `RIB_AND_FIB_ACK`, election ID `1`) and run
    `gRIBI.Flush` on `DEFAULT` before programming any entry.
*   Connect a gRPC client and open one `gNPSI.Subscribe` stream
    (`gnpsi.Sample`) that stays open for the whole test. Per `gnpsi.proto`,
    `sflow_metadata` is present only in the first `Sample` of a stream, which
    normally also carries the first datagram: the first `Sample` received must
    report `sflow_metadata.version = V5`. Do not wait for a metadata-only
    message before starting traffic; collect the streamed sFlow v5 `packet`
    payloads and their `timestamp`s.

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
        "ingress-sampling-rate": 10000,
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
  },
  "system": {
    "grpc-servers": {
      "grpc-server": [
        {
          "config": {
            "enable": true,
            "name": "gnpsi",
            "port": 50051,
            "services": [
              "openconfig-gnpsi-types:GNPSI"
            ]
          },
          "name": "gnpsi"
        }
      ]
    }
  }
}
```

#### sFlow decoding rules

These rules apply to every `gNPSI.Subscribe` message in every subtest:

*   Each `Sample.packet` is one sFlow v5 datagram with `version = 5`. Its agent
    address must be `192.0.2.1` when the address type is IPv4, or
    `2001:db8:1::1` when the address type is IPv6, and all datagrams on the
    stream must use the same agent address.
*   Decode both compact `flow_sample` (enterprise `0`, format `1`) and
    `expanded_flow_sample` (enterprise `0`, format `3`) records, since
    platforms whose `ifindex` values exceed `24` bits must use the expanded
    form. The `source_id` type must be `0` (ifIndex); compare only its index
    (`24` bits in compact records, `32` bits in expanded records). In compact
    records the top `2` bits of `input` and `output` are the format and the
    low `30` bits are the value; expanded records carry format and value in
    separate fields.
*   Assign each sample to a sampler by its `source_id` index and direction,
    not by `output` alone: a sample is an ingress sample of the interface whose
    `ifindex` equals the `source_id` index when its `input` value equals that
    index, and an egress sample of that interface when its `output` value
    equals that index. The analyzed samplers are the `port1` ingress sampler
    and the `port2` egress sampler (an `output` match alone would also select
    `port1` ingress samples of forwarded streams). Any sample whose
    `source_id` index is the `port3` or `port4` `ifindex` fails the test.
*   Because the global `ingress-sampling-rate` also applies to `port2`, the DUT
    may emit `port2` ingress samples (and, on some platforms, `port1` egress
    samples) that share a `source_id` with an analyzed sampler. These samples
    may contain only DUT or ATE control frames: they are logged and excluded
    from the count, header, `sample_pool` and `drops` checks, and a test-stream
    packet in any of them fails the test.
*   Ingress samples report `input` = `port1` `ifindex` (format `0`), and
    ingress samples of forwarded streams report `output` = `port2` `ifindex`
    (format `0`). Egress samples report `output` = `port2` `ifindex` (format
    `0`) and `input` = `port1` `ifindex` or `0`; sFlow v5 allows `0`
    ("unknown") when the agent does not know the input interface of an egress
    sample.
*   `sampling_rate` equals `N` for both samplers.
*   Every flow sample carries exactly one raw packet header record
    (`sampled_header`, enterprise `0`, format `1`) with `header_protocol = 1`
    (Ethernet). Every sample attributed to a test stream reports
    `header_length = 256` and the `frame_length` defined for the stream and
    sampler; sFlow v5 `frame_length` includes the 4-byte FCS, so frames as
    sent by the ATE report `512`. Samples of control frames, which are shorter
    than `256` bytes, are not held to these values. The raw packet header is
    the primary check for every sample attributed to a test stream.
*   "As sent" means that the raw header, from its first byte through the end
    of the payload signature, equals the frame transmitted by the ATE: MAC
    addresses, every header field and the signature. Bytes after the signature
    (ATE payload fill and instrumentation) are not compared.
*   In `port2` egress samples, the Ethernet header is DUT `port2` MAC ->
    `02:00:02:01:01:01`, every IPv4 header checksum must be valid, and the
    DSCP and ECN bits of every IP header are logged, not compared, because
    egress CoS remarking is platform-defined (as in TE-16.1). The
    Identification and flags of an outer IPv4 header added by the DUT are not
    compared. All other fields must match the values stated for the stream.
*   The decoder walks flow records by their XDR length and skips, without
    discarding the rest of the datagram, any record that it does not validate,
    including enterprise-specific records. The stock gopacket
    `SFlowDatagram.DecodeFromBytes` (used by gNPSI-1) fails the whole datagram
    when it contains record type `1006`-`1012` or an unknown record type, so
    the test must use a decoder that tolerates them.
*   Samples received in a stream window are attributed to the stream whose
    header they match, except DUT or ATE control frames (ARP, LLDP, LACP,
    ICMPv6, or any frame with a multicast or broadcast destination MAC), which
    are logged and excluded. Every other sample must match one stream of the
    window and pass that stream's header checks, so a truncated or corrupted
    header fails the test instead of silently lowering the count.

#### sFlow extended flow records

The raw packet header remains the primary check. sFlow v5 also defines
extended flow records that describe encapsulation and MPLS processing. These
records are required only where the platform emits them: the presence or
absence of each record type is logged per stream, and absence is not a
failure. When one of the records below is present, it must match:

| Record (enterprise `0`) | Typical placement | Required content when present |
| --- | --- | --- |
| `1023` `extended_ipv4_tunnel_egress` | `port1` ingress samples of DUT-encapsulated streams (gNPSI-2.1) | `src_ip = 198.51.100.1`, `dst_ip = 203.0.113.10`, `protocol = 4` (`Flow-v4-in-v4`) or `41` (`Flow-v6-in-v4`) |
| `1028` `extended_decapsulate_ingress` | `port2` egress samples of DUT-encapsulated streams (gNPSI-2.1) | `inner_header_offset = 34` (Ethernet `14` + outer IPv4 `20`) |
| `1027` `extended_decapsulate_egress` | `port1` ingress samples of DUT-decapsulated streams (gNPSI-2.5) | `inner_header_offset = 34` |
| `1024` `extended_ipv4_tunnel_ingress` | `port2` egress samples of DUT-decapsulated streams (gNPSI-2.5) | `src_ip = 198.51.100.3`, `dst_ip = 198.51.100.100`, `protocol = 4` (`Flow-Decap-v4-in-v4`) or `41` (`Flow-Decap-v6-in-v4`) |
| `1006` `extended_mpls` | samples of gNPSI-2.3 streams | `nexthop = 192.0.2.6`; `in_stack` labels `[1001, 2001]`; `out_stack` labels `[1002, 2001]` |
| `1008` `extended_mpls_tunnel` | samples of gNPSI-2.3 streams | logged only; LSP naming is implementation-defined |

*   An IPv4 tunnel record (`1023` or `1024`) on any sample must describe the
    outer IPv4 header that the DUT adds (gNPSI-2.1) or removes (gNPSI-2.5) for
    that stream. A decapsulation record (`1027` or `1028`) is valid only on a
    sample whose raw header contains the outer header, and its
    `inner_header_offset` must point at the inner IP header (`34`).
*   IPv6 tunnel records (`1025`, `1026`) must not appear, because the DUT adds
    or removes no IPv6 outer header. No record in the range `1023`-`1028` may
    appear on samples of streams that the DUT neither encapsulates nor
    decapsulates (`Flow-v6-in-v6` and the streams of gNPSI-2.2, gNPSI-2.3 and
    gNPSI-2.4).
*   An `extended_mpls` label-stack entry is compared as a label value; an
    entry larger than `1048575` is treated as a full label-stack entry, whose
    label is `entry >> 12`.
*   A `port2` egress sample of a DUT-encapsulated stream must carry the outer
    header in its raw packet header. A raw header that contains only the inner
    packet fails, even with a matching `1023` record: sFlow v5 defines the
    sampled header as the packet observed at the data source, and on an egress
    data source that packet includes the outer header the DUT added.

#### Per-stream measurement procedure

Every traffic step uses this procedure. Streams run one at a time, except
where gNPSI-2.4 runs a reference stream concurrently with the stream under
test:

1.  Read `/interfaces/interface/state/counters/in-pkts` of the ingress
    port(s) and `/interfaces/interface/state/counters/out-pkts` of `port2` and
    `port3` in one `gNMI.Get` (`OCBatch`).
2.  Start the OTG flow(s) (`P` packets each) and wait with `gnmi.Watch` until
    the OTG flow transmit state reports that every flow has stopped.
3.  Wait with `gnmi.Watch` (`gNMI.Subscribe`, `SAMPLE`) until the `in-pkts`
    delta of each ingress port reaches the OTG transmitted frame count of the
    flows entering it, and the `out-pkts` delta of each egress port reaches
    the OTG received frame count of the flows leaving it.
4.  Keep reading the `gNPSI.Subscribe` stream until no sample attributed to
    the window's streams has arrived for `10` seconds; the test fails if such
    samples are still arriving `60` seconds after traffic stopped.
5.  Read the counters again. `D_in` is the `in-pkts` delta of the ingress port
    and `D_out` is the `out-pkts` delta of `port2`.
6.  The stream window runs from the start of step 2 to the end of step 4.
    For each sampler, evaluate the samples received in the window:
    *   Loss: the OTG received frame count of each forwarded flow equals its
        transmitted frame count (`P`); `D_in` is at least the OTG transmitted
        frame count of the flows entering the ingress port, and `D_out` is at
        least the OTG received frame count of the flows leaving `port2`.
    *   Count: the number of samples attributed to the stream is within
        `[0.8 * E, 1.2 * E]`, where `E = D_in / N` for the ingress sampler and
        `E = D_out / N` for the egress sampler.
    *   No lost samples: across the window, the flow-sample `sequence_number`
        of the sampler's samples (control frames included) increases by
        exactly `1` (modulo `2^32`) from one sample to the next. A gap is
        allowed only where every missing number belongs to an
        opposite-direction sample of the same `source_id` received in the
        window (platforms that share one sequence space per `source_id`). The
        datagram `sequence_number` of each (`agent_address`, `sub_agent_id`)
        increases by exactly `1` (modulo `2^32`) from one datagram to the next,
        and the `drops` value of the sampler's samples does not change.
    *   Sample pool: the `sample_pool` difference (modulo `2^32`) between the
        first and the last sample of the sampler in the window is within
        `[0.8 * D, 1.2 * D]`, with `D = D_in` for the ingress sampler and
        `D = D_out` for the egress sampler.
    *   Headers and metadata: every attributed sample passes the decoding
        rules and the stream's raw header and `frame_length` checks.

### gNPSI-2.1 - IP-in-IP Tunneled Traffic Sampling (IPv4 and IPv6)

*   Step 1 - Program IP-in-IP encapsulation and transit tunnel routes:
    *   The Canonical OC of the test environment setup is already applied and
        verified; it is not pushed again.
    *   Via `gRIBI.Modify` in `DEFAULT`, program:
        *   Underlay `NextHop` `1` (`ip_address = 192.0.2.6`, ATE `port2`) ->
            `NextHopGroup` `10` -> outer tunnel destination `IPv4Entry`
            `203.0.113.0/24`.
        *   Underlay `NextHop` `2` (`ip_address = 2001:db8:2::2`, ATE `port2`)
            -> `NextHopGroup` `20` -> outer tunnel destination `IPv6Entry`
            `2001:db8:200::/64`.
        *   IP-in-IP encapsulating `NextHop` `101` (`encapsulate_header =
            IPV4`, `ip_in_ip = {src_ip: 198.51.100.1, dst_ip: 203.0.113.10}`,
            `network_instance = DEFAULT`) -> `NextHopGroup` `100`, referenced
            by inner prefixes `IPv4Entry` `100.64.1.0/24` (`IPv4-in-IPv4`) and
            `IPv6Entry` `2001:db8:10::/64` (`IPv6-in-IPv4`).
    *   Every operation must return `AFTResult` status `FIB_PROGRAMMED`.
        Verify the entries with `gRIBI.Get` (`network_instance = DEFAULT`,
        `aft = ALL`): every programmed entry is present with `fib_status =
        PROGRAMMED`. Verify the AFT with `gNMI.Subscribe` (`ON_CHANGE`): each
        prefix resolves through its `next-hop-group` to a next hop with the
        programmed `ip-address`, or with `encapsulate-header = IPV4` and the
        programmed `ip-in-ip` `src-ip` and `dst-ip`.
*   Step 2 - Generate IP-in-IP traffic: send each stream below from ATE
    `port1` to ATE `port2` with the per-stream measurement procedure.

| Stream | Packet sent by ATE `port1` (after the Ethernet header) | Inner UDP |
| --- | --- | --- |
| `Flow-v4-in-v4` | IPv4 `192.0.2.2` -> `100.64.1.10` (encapsulated by the DUT) | `5001` -> `6001` |
| `Flow-v6-in-v4` | IPv6 `2001:db8:1::2` -> `2001:db8:10::10` (encapsulated by the DUT) | `5011` -> `6011` |
| `Flow-v6-in-v6` | outer IPv6 `2001:db8:100::1` -> `2001:db8:200::10` (next header `41`); inner IPv6 `2001:db8:11::1` -> `2001:db8:11::10` | `5021` -> `6021` |

*   Step 3 - Validate the `gNPSI` sFlow samples of each stream with the
    decoding rules and the per-stream measurement procedure (`0%` packet loss
    at ATE `port2`, count, no lost samples and sample pool for both samplers),
    and check the raw packet header of every sample:

| Stream | `port1` ingress raw header | `port2` egress raw header | `frame_length` (`port1` / `port2`) |
| --- | --- | --- | --- |
| `Flow-v4-in-v4` | as sent: unencapsulated IPv4 + UDP + signature | outer IPv4 `198.51.100.1` -> `203.0.113.10`, protocol `4`, TTL `63`, total length `514`; inner IPv4, UDP and signature as sent except inner TTL `63` | `512` / `532` |
| `Flow-v6-in-v4` | as sent: unencapsulated IPv6 + UDP + signature | outer IPv4 `198.51.100.1` -> `203.0.113.10`, protocol `41`, TTL `63`, total length `514`; inner IPv6, UDP and signature as sent except inner hop limit `63` | `512` / `532` |
| `Flow-v6-in-v6` | as sent: outer IPv6 + inner IPv6 + UDP + signature | outer IPv6 as sent except hop limit `63`; inner IPv6 (hop limit `64`), UDP and signature as sent | `512` / `512` |

*   The inner TTL and hop limit of `63` in `port2` egress samples reflect the
    DUT routing the inner packet before encapsulation; the outer TTL of `63` is
    the decremented inner TTL copied into the outer header, the encapsulation
    behavior that TE-16.1 requires for `gRIBI` IP-in-IP encapsulation. The
    `port2` EtherType is `0x0800` for the DUT-encapsulated streams and
    `0x86DD` for `Flow-v6-in-v6`.
*   The `port1` ingress samples of the DUT-encapsulated streams are taken
    before encapsulation and therefore contain only the inner packet; the
    `port2` egress samples must contain both the outer IPv4 header and the
    intact inner header. Validate any extended flow records as described in
    the test environment setup.

### gNPSI-2.2 - GRE, MPLSoGRE and MPLSoUDP Transit Tunnel Sampling

The ATE builds every tunnel in this subtest. The DUT routes each stream on its
outer IPv4 destination through `IPv4Entry` `203.0.113.0/24` (programmed in
gNPSI-2.1) and performs no GRE, UDP or MPLS processing; only the Ethernet
header and the outer IPv4 TTL and checksum change.

*   Step 1 - Transmit GRE, MPLSoGRE and MPLSoUDP streams from ATE `port1` to
    ATE `port2`, one at a time, with the per-stream measurement procedure.
    Every stream uses outer IPv4 source `198.51.100.2`:

| Stream | Outer IPv4 destination, protocol | Tunnel header | MPLS label stack entry | Inner packet | Inner UDP |
| --- | --- | --- | --- | --- | --- |
| `Flow-GRE-v4` | `203.0.113.20`, `47` | GRE, `4` bytes: `C = 0`, `K = 0`, `S = 0`, version `0`, protocol type `0x0800` | none | IPv4 `100.64.2.1` -> `100.64.2.10` | `5002` -> `6002` |
| `Flow-GRE-v6` | `203.0.113.20`, `47` | GRE, `8` bytes: `C = 1` (checksum and reserved1 fields present), `K = 0`, `S = 0`, version `0`, protocol type `0x86DD` | none | IPv6 `2001:db8:20::1` -> `2001:db8:20::10` | `5012` -> `6012` |
| `Flow-MPLSoGRE-v4` | `203.0.113.40`, `47` | GRE, `4` bytes: `C = 0`, `K = 0`, `S = 0`, version `0`, protocol type `0x8847` | label `1043`, `TC = 4`, `S = 1`, `TTL = 64` | IPv4 `100.64.6.1` -> `100.64.6.10` | `5006` -> `6006` |
| `Flow-MPLSoGRE-v6` | `203.0.113.40`, `47` | GRE, `4` bytes: `C = 0`, `K = 0`, `S = 0`, version `0`, protocol type `0x8847` | label `1043`, `TC = 4`, `S = 1`, `TTL = 64` | IPv6 `2001:db8:60::1` -> `2001:db8:60::10` | `5016` -> `6016` |
| `Flow-MPLSoUDP-v4` | `203.0.113.30`, `17` | UDP `49152` -> `6635` | label `1042`, `TC = 4`, `S = 1`, `TTL = 64` | IPv4 `100.64.3.1` -> `100.64.3.10` | `5003` -> `6003` |
| `Flow-MPLSoUDP-v6` | `203.0.113.30`, `17` | UDP `49152` -> `6635` | label `1042`, `TC = 4`, `S = 1`, `TTL = 64` | IPv6 `2001:db8:30::1` -> `2001:db8:30::10` | `5013` -> `6013` |

*   Step 2 - Validate `gNPSI` header integrity:
    *   `0%` packet loss at ATE `port2`, and the count, no-lost-samples and
        sample-pool checks of the per-stream measurement procedure for both
        samplers.
    *   `port1` ingress samples: raw header as sent. `port2` egress samples:
        raw header as sent except the Ethernet header, the outer IPv4 TTL
        (`63`) and the outer IPv4 checksum. `frame_length` is `512` for both
        samplers.
    *   In every sample, the outer IPv4 header, the `4`-byte or `8`-byte GRE
        header or the `8`-byte UDP header, the `4`-byte MPLS label stack entry,
        the inner IPv4 or IPv6 header, the inner UDP header and the signature
        are all present and decode at the offsets implied by the stream
        definition, without byte-offset misalignment, within the `256`-byte
        raw header.

### gNPSI-2.3 - MPLS Label-Switched Packet Sampling

#### Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "name": "port1"
        },
        "name": "port1",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0
            }
          ]
        }
      },
      {
        "config": {
          "name": "port2"
        },
        "name": "port2",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0
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
        "mpls": {
          "global": {
            "interface-attributes": {
              "interface": [
                {
                  "config": {
                    "interface-id": "port1",
                    "mpls-enabled": true
                  },
                  "interface-id": "port1",
                  "interface-ref": {
                    "config": {
                      "interface": "port1",
                      "subinterface": 0
                    }
                  }
                },
                {
                  "config": {
                    "interface-id": "port2",
                    "mpls-enabled": true
                  },
                  "interface-id": "port2",
                  "interface-ref": {
                    "config": {
                      "interface": "port2",
                      "subinterface": 0
                    }
                  }
                }
              ]
            }
          },
          "lsps": {
            "static-lsps": {
              "static-lsp": [
                {
                  "config": {
                    "name": "gnpsi-2-transit-1001"
                  },
                  "name": "gnpsi-2-transit-1001",
                  "transit": {
                    "config": {
                      "incoming-label": 1001
                    },
                    "lsp-next-hops": {
                      "lsp-next-hop": [
                        {
                          "config": {
                            "index": 1,
                            "ip-address": "192.0.2.6",
                            "push-label": 1002
                          },
                          "index": 1
                        }
                      ]
                    }
                  }
                }
              ]
            }
          }
        },
        "name": "DEFAULT"
      }
    ]
  }
}
```

*   Step 1 - Configure MPLS forwarding with a static transit LSP:
    *   Enable MPLS on `port1` and `port2` and configure static LSP
        `gnpsi-2-transit-1001`: transit `incoming-label = 1001` and next hop
        `1` (`ip-address = 192.0.2.6`, ATE `port2`) with `push-label = 1002`.
        The DUT swaps top label `1001` to `1002` and leaves the rest of the
        stack untouched. Under
        `/network-instances/network-instance[name=DEFAULT]/mpls`, apply with
        `gNMI.Set` `REPLACE` scoped to the static LSP entry
        (`static-lsp[name=gnpsi-2-transit-1001]`) and to the `port1` and
        `port2` entries of `global/interface-attributes/interface`.
    *   On platforms with deviation `mpls_unsupported` (MPLS enablement),
        `static_mpls_unsupported` (static MPLS) or
        `static_mpls_lsp_oc_unsupported` (static LSP parameters), this
        configuration cannot be expressed in OpenConfig: configure the same
        MPLS enablement on `port1` and `port2` and the same label swap (`1001`
        -> `1002` via `192.0.2.6`) through the platform's alternate
        configuration method. gNPSI-2.3 remains mandatory on these platforms,
        and the streams and the expected samples do not change.
    *   Only a platform that cannot forward MPLS label-switched traffic with
        any configuration method skips gNPSI-2.3, and the test logs the
        reason.
    *   The label swap is not programmed through `gRIBI` `LabelEntry`
        operations, because `gRIBI` MPLS AFT programming is not available on
        every platform. (A `gRIBI` server reports a rejected operation in-band
        as `AFTResult` status `FAILED`, not as a gRPC `UNIMPLEMENTED` status, so
        a runtime status is not a usable selector.)
    *   On platforms without any of these three deviations, verify with
        `gNMI.Subscribe` (`ON_CHANGE`): `mpls-enabled = true` on `port1` and
        `port2`, transit `incoming-label = 1001`, and next hop `1` with
        `ip-address = 192.0.2.6` and `push-label = 1002`.
*   Step 2 - Send each MPLS stream from ATE `port1` to ATE `port2`, one at a
    time, with the per-stream measurement procedure. The Ethernet destination
    is DUT `port1` MAC and the EtherType is `0x8847`:

| Stream | Label stack sent (top first) | Inner packet | Inner UDP |
| --- | --- | --- | --- |
| `Flow-MPLS-v4` | `1001` (`TC = 4`, `S = 0`, `TTL = 64`), `2001` (`TC = 4`, `S = 1`, `TTL = 64`) | IPv4 `100.64.4.1` -> `100.64.4.10` | `5004` -> `6004` |
| `Flow-MPLS-v6` | `1001` (`TC = 4`, `S = 0`, `TTL = 64`), `2001` (`TC = 4`, `S = 1`, `TTL = 64`) | IPv6 `2001:db8:40::1` -> `2001:db8:40::10` | `5014` -> `6014` |

*   Step 3 - Validate the MPLS stack and inner payload in `gNPSI` samples:
    *   `0%` packet loss at ATE `port2`, and the count, no-lost-samples and
        sample-pool checks for both samplers.
    *   `port1` ingress samples: raw header as sent, with label stack `1001`
        (`TC = 4`, `S = 0`, `TTL = 64`), `2001` (`TC = 4`, `S = 1`,
        `TTL = 64`); `output` = `port2` `ifindex`.
    *   `port2` egress samples: EtherType `0x8847` and label stack `1002`
        (`TC = 4`, `S = 0`, `TTL = 63`), `2001` (`TC = 4`, `S = 1`,
        `TTL = 64`): the top label is swapped and its TTL decremented, the TC
        and `S` bits are unchanged, and the bottom label is untouched. The
        inner IPv4 or IPv6 header (TTL or hop limit `64`), the inner UDP header
        and the signature are as sent.
    *   `frame_length` is `512` for both samplers.
*   Step 4 - Remove the static LSP and the MPLS interface enablement (OC or
    deviation method); register the removal with `t.Cleanup()` when they are
    created.

### gNPSI-2.4 - Negative Validation: Unsampled Ports and Dropped Traffic

Each step sends the reference stream `Flow-Ref` on the sampled path
concurrently with the stream under test, so the absence of samples is measured
while the `gNPSI` export path is shown to be working. Every stream below is
IPv4-in-IPv4 (outer protocol `4`), routed by the DUT on its outer header:

| Stream | ATE ingress -> expected egress | Outer IPv4 | Inner IPv4 | Inner UDP |
| --- | --- | --- | --- | --- |
| `Flow-Ref` | `port1` -> `port2` | `198.51.100.5` -> `203.0.113.50` | `100.64.7.1` -> `100.64.7.10` | `5997` -> `6997` |
| `Flow-Unsampled` | `port4` -> `port3` | `198.51.100.4` -> `203.0.113.130` | `100.64.9.1` -> `100.64.9.10` | `5999` -> `6999` |
| `Flow-Dropped` | `port1` -> dropped by the DUT | `198.51.100.5` -> `198.51.100.130` | `100.64.8.1` -> `100.64.8.10` | `5998` -> `6998` |

`Flow-Ref` samples use the `port1` / `port2` raw header checks of gNPSI-2.2
(`port2`: outer TTL `63`); `frame_length` is `512` for every sample in this
subtest.

#### Canonical OC

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
              "config": {
                "identifier": "openconfig-policy-types:STATIC",
                "name": "DEFAULT"
              },
              "identifier": "openconfig-policy-types:STATIC",
              "name": "DEFAULT",
              "static-routes": {
                "static": [
                  {
                    "config": {
                      "prefix": "198.51.100.128/25"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "config": {
                            "index": "0",
                            "next-hop": "openconfig-local-routing:DROP"
                          },
                          "index": "0"
                        }
                      ]
                    },
                    "prefix": "198.51.100.128/25"
                  }
                ]
              }
            }
          ]
        }
      }
    ]
  }
}
```

*   Step 1 - Unsampled ingress-to-egress path (`port4` -> `port3`):
    *   Via `gRIBI.Modify` in `DEFAULT`, program `NextHop` `3` (`ip_address =
        192.0.2.10`, ATE `port3`, where sFlow is `enabled = false`) ->
        `NextHopGroup` `30` -> outer tunnel destination `IPv4Entry`
        `203.0.113.128/25`. Confirm `AFTResult` status `FIB_PROGRAMMED` and
        verify the entry with `gRIBI.Get` and the AFT as in gNPSI-2.1.
    *   Send `Flow-Ref` and `Flow-Unsampled` concurrently (`P` packets each)
        with the per-stream measurement procedure.
    *   Verify:
        *   `100%` of `Flow-Unsampled` packets arrive at ATE `port3` and
            `100%` of `Flow-Ref` packets arrive at ATE `port2`; the `port4`
            `in-pkts` delta is at least `P`.
        *   `Flow-Ref` passes the count, no-lost-samples, sample-pool and
            header checks for both samplers, with `D_in` and `D_out` from the
            `port1` and `port2` counters.
        *   **`0` sFlow samples** match `Flow-Unsampled` (inner UDP `5999` ->
            `6999`); `0` samples have a `source_id` index equal to the `port3`
            or `port4` `ifindex`; and `0` samples report `input` = `port4`
            `ifindex` or `output` = `port3` `ifindex`.
*   Step 2 - Dropped traffic on a sampled ingress port:
    *   Platforms with deviation `static_route_with_drop_nh` (static route with
        a `DROP` next hop unsupported) skip Step 2 with a logged reason.
    *   Configure the OpenConfig static route `198.51.100.128/25` with next
        hop `DROP` in `DEFAULT` (Canonical OC above), with `gNMI.Set`
        `REPLACE` scoped to the `static[prefix=198.51.100.128/25]` entry of
        the `STATIC` protocol instance (`name = DEFAULT`) of network instance
        `DEFAULT`. Verify `state/prefix` with `gNMI.Subscribe` (`ON_CHANGE`),
        except on platforms with deviation
        `missing_static_route_drop_next_hop_telemetry`.
    *   Send `Flow-Ref` and `Flow-Dropped` concurrently from ATE `port1` (`P`
        packets each) with the per-stream measurement procedure.
    *   Verify, scoped to the samples attributed to each stream:
        *   OTG: `Flow-Ref` received at ATE `port2` equals `P`, and
            `Flow-Dropped` received equals `0`. DUT: the `port1` `in-pkts`
            delta is at least `2 * P`, the `port2` `out-pkts` delta is between
            `P` and `1.01 * P`, and the `port3` `out-pkts` delta is below
            `0.01 * P`.
        *   Egress sampler: the `Flow-Ref` count is within
            `[0.8, 1.2] * D_out / N`, and `0` samples are attributed to
            `Flow-Dropped`.
        *   Ingress sampler: the `Flow-Ref` count is within
            `[0.8, 1.2] * P / N`, using the OTG transmitted frame count of
            `Flow-Ref` because `D_in` also counts the dropped packets. Dropping
            `Flow-Dropped` must not disturb the sampling of forwarded traffic
            on the same port.
        *   Every ingress sample attributed to `Flow-Dropped` reports `output`
            in the sFlow v5 discard format (format `1`: `0x40000000 | reason`
            in compact records, output format field `1` in expanded records;
            the reason code is logged) or `0` (unknown); none reports the
            `port2`, `port3` or `port4` `ifindex`.
        *   sFlow v5 defines how a sampled discard is reported but not whether
            packets discarded by the forwarding lookup are sampling candidates,
            so the DUT's own `sample_pool` accounting selects the expected
            number of `Flow-Dropped` ingress samples. With `pool_delta` the
            `sample_pool` increase of the `port1` ingress sampler over the
            window (which replaces the general sample-pool check for `port1`
            in this step):

| `pool_delta` | Meaning | Required `Flow-Dropped` ingress samples |
| --- | --- | --- |
| within `[0.8, 1.2] * D_in` | discarded packets are sampling candidates | within `[0.8, 1.2] * P / N` |
| within `[0.8, 1.2] * (D_in - P)` | discarded packets are not sampling candidates | `0` |
| any other value | accounting inconsistent with the samples delivered | the test fails |

### gNPSI-2.5 - Sampling of Tunneled Traffic Decapsulated after VRF Selection

`IPv4-in-IPv4` and `IPv6-in-IPv4` packets enter sampled `port1`, are steered by
a policy-forwarding VRF selection rule to `DECAP_TE_VRF`, where a `gRIBI`
decapsulation entry removes the outer IPv4 header, and the inner packet is
forwarded in `DEFAULT` out of `port2`. The `port1` ingress sampler must sample
these packets at the configured rate, with the same criterion as every other
stream (no under-sampling of decapsulated traffic), and report them as
received, with the outer and inner headers; the `port2` egress sampler reports
only the inner packet.

#### Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "name": "port1"
        },
        "name": "port1",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0
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
          "name": "DECAP_TE_VRF",
          "type": "openconfig-network-instance-types:L3VRF"
        },
        "name": "DECAP_TE_VRF"
      },
      {
        "config": {
          "name": "DEFAULT",
          "type": "openconfig-network-instance-types:DEFAULT_INSTANCE"
        },
        "name": "DEFAULT",
        "policy-forwarding": {
          "interfaces": {
            "interface": [
              {
                "config": {
                  "apply-vrf-selection-policy": "vrf_selection_policy_decap",
                  "interface-id": "port1"
                },
                "interface-id": "port1",
                "interface-ref": {
                  "config": {
                    "interface": "port1",
                    "subinterface": 0
                  }
                }
              }
            ]
          },
          "policies": {
            "policy": [
              {
                "config": {
                  "policy-id": "vrf_selection_policy_decap",
                  "type": "VRF_SELECTION_POLICY"
                },
                "policy-id": "vrf_selection_policy_decap",
                "rules": {
                  "rule": [
                    {
                      "action": {
                        "config": {
                          "decap-fallback-network-instance": "DEFAULT",
                          "decap-network-instance": "DECAP_TE_VRF",
                          "post-decap-network-instance": "DEFAULT"
                        }
                      },
                      "config": {
                        "sequence-id": 1
                      },
                      "ipv4": {
                        "config": {
                          "protocol": 4,
                          "source-address": "198.51.100.3/32"
                        }
                      },
                      "sequence-id": 1
                    },
                    {
                      "action": {
                        "config": {
                          "decap-fallback-network-instance": "DEFAULT",
                          "decap-network-instance": "DECAP_TE_VRF",
                          "post-decap-network-instance": "DEFAULT"
                        }
                      },
                      "config": {
                        "sequence-id": 2
                      },
                      "ipv4": {
                        "config": {
                          "protocol": 41,
                          "source-address": "198.51.100.3/32"
                        }
                      },
                      "sequence-id": 2
                    },
                    {
                      "action": {
                        "config": {
                          "network-instance": "DEFAULT"
                        }
                      },
                      "config": {
                        "sequence-id": 3
                      },
                      "sequence-id": 3
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
```

*   Step 1 - Configure decapsulation after VRF selection:
    *   Create network instance `DECAP_TE_VRF` (`L3VRF`, no interfaces).
    *   In `DEFAULT`, configure VRF selection policy
        `vrf_selection_policy_decap` (`type = VRF_SELECTION_POLICY`):
        *   Rule `1`: IPv4 `protocol = 4` and `source-address =
            198.51.100.3/32` -> `decap-network-instance = DECAP_TE_VRF`,
            `post-decap-network-instance = DEFAULT`,
            `decap-fallback-network-instance = DEFAULT`.
        *   Rule `2`: the same match and action with IPv4 `protocol = 41`.
        *   Rule `3`: no match conditions -> `network-instance = DEFAULT`.
    *   Apply the policy to `port1` (`apply-vrf-selection-policy`, interface
        `port1`, subinterface `0`).
    *   Apply with `gNMI.Set` `REPLACE` scoped to
        `/network-instances/network-instance[name=DECAP_TE_VRF]` and to
        `/network-instances/network-instance[name=DEFAULT]/policy-forwarding`.
        On platforms that need an alternate method to configure or bind a VRF
        selection policy (existing policy-forwarding deviations), apply the
        equivalent configuration through that method; the forwarding behavior
        and the pass criteria do not change.
*   Step 2 - Program decapsulation and inner routes via `gRIBI.Modify`:
    *   In `DEFAULT`, decapsulating `NextHop` `201` (`decapsulate_header =
        IPV4`, no `network_instance`; the post-decapsulation lookup network
        instance, `DEFAULT`, comes from the policy) -> `NextHopGroup` `200`.
    *   In `DECAP_TE_VRF`, tunnel endpoint `IPv4Entry` `198.51.100.100/32` ->
        `NextHopGroup` `200` (`next_hop_group_network_instance = DEFAULT`).
    *   Inner routes in `DEFAULT`: `IPv4Entry` `100.64.5.0/24` ->
        `NextHopGroup` `10` and `IPv6Entry` `2001:db8:50::/64` ->
        `NextHopGroup` `20` (both ATE `port2`).
    *   Every operation must return `AFTResult` status `FIB_PROGRAMMED`.
        Verify the entries with `gRIBI.Get` on `DECAP_TE_VRF` and `DEFAULT`,
        and verify the AFT with `gNMI.Subscribe` (`ON_CHANGE`):
        `198.51.100.100/32` in `DECAP_TE_VRF` resolves to a next hop with
        `decapsulate-header = IPV4`.
*   Step 3 - Send each stream below from ATE `port1` to ATE `port2`, one at a
    time, with the per-stream measurement procedure. The outer and inner
    headers both use TTL or hop limit `64` and DSCP `32`, so the forwarded
    inner TTL is `63` whether the DUT copies the TTL from the outer header or
    keeps the inner value:

| Stream | Outer IPv4 sent by ATE `port1` | Inner packet | Inner UDP |
| --- | --- | --- | --- |
| `Flow-Decap-v4-in-v4` | `198.51.100.3` -> `198.51.100.100`, protocol `4` | IPv4 `100.64.15.1` -> `100.64.5.10` | `5005` -> `6005` |
| `Flow-Decap-v6-in-v4` | `198.51.100.3` -> `198.51.100.100`, protocol `41` | IPv6 `2001:db8:15::1` -> `2001:db8:50::10` | `5015` -> `6015` |

*   Step 4 - Validate:
    *   `0%` packet loss at ATE `port2`; the OTG flow counts the decapsulated
        inner packets received at ATE `port2`.
    *   `port1` ingress sampler: the count, no-lost-samples and sample-pool
        checks with `D_in`, so a DUT that under-samples packets decapsulated
        after VRF selection fails. Raw header as sent, captured before
        decapsulation: Ethernet + outer IPv4 (`198.51.100.3` ->
        `198.51.100.100`, protocol `4` or `41`, TTL `64`) + inner header + UDP +
        signature; `frame_length = 512`; `output` = `port2` `ifindex`.
    *   `port2` egress sampler: the count, no-lost-samples and sample-pool
        checks with `D_out`. The raw header contains only the inner packet:
        Ethernet (EtherType `0x0800` for `Flow-Decap-v4-in-v4`, `0x86DD` for
        `Flow-Decap-v6-in-v4`) + inner IPv4 or IPv6 header as sent except TTL
        or hop limit `63` + UDP + signature, with no outer IPv4 header;
        `frame_length = 492`.
*   Step 5 - Remove the policy binding and the policy, flush the
    `DECAP_TE_VRF` entries with `gRIBI.Flush`, and delete the `DECAP_TE_VRF`
    network instance, in that order; register each removal with
    `t.Cleanup()` when the item is created.

### Cleanup

Register every item with `t.Cleanup()` when it is created, so that the DUT is
restored to its pre-test state even if a step fails:

*   Stop all OTG traffic streams and protocols, and close the
    `gNPSI.Subscribe` client stream.
*   Run `gRIBI.Flush` on `DEFAULT` (and on `DECAP_TE_VRF` if it still exists)
    and close the `gRIBI` session.
*   Remove the gNPSI-2.5 VRF selection policy binding, the policy and the
    `DECAP_TE_VRF` network instance.
*   Remove the gNPSI-2.4 static `DROP` route.
*   Remove the gNPSI-2.3 static LSP and MPLS interface enablement (OC or
    deviation method).
*   Restore `/sampling/sflow` to its pre-test configuration (including any
    collector added by the shared sFlow helper) and remove the
    deviation-applied `port2` egress sampling configuration.
*   Remove the `gnpsi` gRPC server if the test created it, or revert the
    deviation-applied `gNPSI` configuration; a pre-existing gRPC server that
    the test only used is left unchanged.
*   Restore the `port1`-`port4` interface configuration to its pre-test state.

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
  /network-instances/network-instance/mpls/global/interface-attributes/interface/config/interface-id:
  /network-instances/network-instance/mpls/global/interface-attributes/interface/config/mpls-enabled:
  /network-instances/network-instance/mpls/global/interface-attributes/interface/interface-ref/config/interface:
  /network-instances/network-instance/mpls/global/interface-attributes/interface/interface-ref/config/subinterface:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/config/name:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/transit/config/incoming-label:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/transit/lsp-next-hops/lsp-next-hop/config/index:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/transit/lsp-next-hops/lsp-next-hop/config/ip-address:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/transit/lsp-next-hops/lsp-next-hop/config/push-label:
  /network-instances/network-instance/policy-forwarding/interfaces/interface/config/apply-vrf-selection-policy:
  /network-instances/network-instance/policy-forwarding/interfaces/interface/config/interface-id:
  /network-instances/network-instance/policy-forwarding/interfaces/interface/interface-ref/config/interface:
  /network-instances/network-instance/policy-forwarding/interfaces/interface/interface-ref/config/subinterface:
  /network-instances/network-instance/policy-forwarding/policies/policy/config/policy-id:
  /network-instances/network-instance/policy-forwarding/policies/policy/config/type:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decap-fallback-network-instance:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decap-network-instance:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/network-instance:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/post-decap-network-instance:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/config/sequence-id:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/protocol:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/source-address:
  /network-instances/network-instance/protocols/protocol/config/identifier:
  /network-instances/network-instance/protocols/protocol/config/name:
  /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/index:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop:
  /sampling/sflow/config/agent-id-ipv4:
  /sampling/sflow/config/agent-id-ipv6:
  /sampling/sflow/config/enabled:
  /sampling/sflow/config/ingress-sampling-rate:
  /sampling/sflow/config/sample-size:
  /sampling/sflow/interfaces/interface/config/egress-sampling-rate:
  /sampling/sflow/interfaces/interface/config/enabled:
  /sampling/sflow/interfaces/interface/config/ingress-sampling-rate:
  /sampling/sflow/interfaces/interface/config/name:
  /system/grpc-servers/grpc-server/config/enable:
  /system/grpc-servers/grpc-server/config/name:
  /system/grpc-servers/grpc-server/config/port:
  /system/grpc-servers/grpc-server/config/services:
  # TODO: no OpenConfig leaf binds the gNPSI server to the sFlow sampling
  # source yet; add it here once it is published in openconfig/public.

  ## Telemetry Parameter Coverage
  /interfaces/interface/ethernet/state/mac-address:
  /interfaces/interface/state/counters/in-pkts:
  /interfaces/interface/state/counters/out-pkts:
  /interfaces/interface/state/ifindex:
  /interfaces/interface/state/oper-status:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hops/next-hop/ip-in-ip/state/dst-ip:
  /network-instances/network-instance/afts/next-hops/next-hop/ip-in-ip/state/src-ip:
  /network-instances/network-instance/afts/next-hops/next-hop/state/decapsulate-header:
  /network-instances/network-instance/afts/next-hops/next-hop/state/encapsulate-header:
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address:
  /network-instances/network-instance/mpls/global/interface-attributes/interface/state/mpls-enabled:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/transit/lsp-next-hops/lsp-next-hop/state/ip-address:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/transit/lsp-next-hops/lsp-next-hop/state/push-label:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/transit/state/incoming-label:
  /network-instances/network-instance/protocols/protocol/static-routes/static/state/prefix:
  /sampling/sflow/interfaces/interface/state/egress-sampling-rate:
  /sampling/sflow/interfaces/interface/state/enabled:
  /sampling/sflow/interfaces/interface/state/ingress-sampling-rate:
  /sampling/sflow/interfaces/interface/state/name:
  /sampling/sflow/state/agent-id-ipv4:
  /sampling/sflow/state/agent-id-ipv6:
  /sampling/sflow/state/enabled:
  /sampling/sflow/state/ingress-sampling-rate:
  /sampling/sflow/state/sample-size:
  /system/grpc-servers/grpc-server/state/enable:
  /system/grpc-servers/grpc-server/state/services:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
      replace: true
      update: true
      delete: true
    gNMI.Subscribe:
      on_change: true
      sample: true
  gnpsi:
    gNPSI.Subscribe:
  gribi:
    gRIBI.Flush:
    gRIBI.Get:
    gRIBI.Modify:
```

## Required DUT platform

* FFF
