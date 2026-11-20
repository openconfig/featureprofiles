# AFT-4.1: AFT MPLS Label Entry Counters

## Summary

Verify that the Device Under Test (DUT) accurately increments and reports
OpenConfig Abstract Forwarding Table (AFT) MPLS label-entry telemetry counters
(`packets-forwarded` and `octets-forwarded` under
`/network-instances/network-instance/afts/mpls/label-entry/state/counters/`)
when decapsulating and forwarding MPLS-in-UDP (MPLSoGUE) traffic across two
forwarding modes:

1.  **Static MPLS LSP Pop-and-Forward (Direct Next-Hop Forwarding)**: The
    incoming MPLS label is popped and the inner IPv4 or IPv6 packet is forwarded
    directly to a configured next-hop IP address in a target network instance
    without performing an inner destination IP route lookup.
2.  **Static MPLS LSP Pop-and-Lookup (Network-Instance FIB Lookup)**: The
    incoming MPLS label is popped and the inner IPv4 or IPv6 packet's
    destination address is looked up in a target network instance's FIB/RIB and
    forwarded out the corresponding egress interface.

## Testbed type

*   [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

### Test environment setup

Connect the Automated Test Equipment (ATE) and DUT using two physical links as
defined in `atedut_2.testbed`:

```
  +-------------------+              dut:port1 +-----------+ dut:port2              +-------------------+
  |                   | ---------------------> |           | ---------------------> |                   |
  |     ATE Port 1    |   MPLSoGUE Ingress     |    DUT    |   Decapsulated IP      |     ATE Port 2    |
  | (192.0.2.2/30)    |   (UDP Dst Port 6635)  |           |   Egress               | (192.0.2.6/30)    |
  | (2001:db8::2/126) |                        |           |                        | (2001:db8::6/126) |
  +-------------------+                        +-----------+                        +-------------------+
```

<!-- mdformat off(reason: GFM table compatibility for VSCode/external renderers) -->

| Interface | Connected Peer | IPv4 Address | IPv6 Address | Role |
| :--- | :--- | :--- | :--- | :--- |
| `dut:port1` | `ate:port1` | `192.0.2.1/30` | `2001:db8::1/126` | MPLSoGUE Ingress |
| `ate:port1` | `dut:port1` | `192.0.2.2/30` | `2001:db8::2/126` | Traffic Source |
| `dut:port2` | `ate:port2` | `192.0.2.5/30` | `2001:db8::5/126` | Decapsulated IP Egress |
| `ate:port2` | `dut:port2` | `192.0.2.6/30` | `2001:db8::6/126` | Traffic Receiver |

<!-- mdformat on -->

#### Common Baseline Configuration

1.  Configure `dut:port1` and `dut:port2` with the IPv4 and IPv6 addresses
    listed in the topology table above and assign both interfaces to the
    `DEFAULT` network instance.
2.  Configure `ate:port1` and `ate:port2` with their corresponding IPv4 and IPv6
    addresses and start ATE protocols.
3.  Verify that IPv4 ARP and IPv6 Neighbor Discovery (ND) are resolved on both
    links before starting any traffic flows.
4.  Configure an MPLSoGUE decapsulation policy in the `DEFAULT` network instance
    on the DUT:
    *   **Outer Encapsulation**: MPLS-in-UDP (GUE) using UDP destination port
        `6635`.
    *   **Outer IPv4 Decapsulation Prefix**: `203.0.113.0/28` (packets arriving
        with outer IPv4 destination address `203.0.113.1` and UDP destination
        port `6635` have their outer IPv4 and UDP headers stripped and the inner
        packet processed by the MPLS forwarding plane).

---

### AFT-4.1.1: Static MPLS LSP Pop-and-Forward (Direct Next-Hop Forwarding)

#### Objective

Verify that AFT MPLS label-entry counters (`packets-forwarded` and
`octets-forwarded`) increment accurately for static MPLS LSP pop-and-forward
entries that pop the MPLS label and forward the decapsulated IPv4 or IPv6 packet
directly to a configured next-hop IP address in the `DEFAULT` network instance.

#### Configuration

1.  Ensure `dut:port2` (`192.0.2.5/30`, `2001:db8::5/126`) is assigned to the
    `DEFAULT` network instance and ARP/ND is resolved with `ate:port2`
    (`192.0.2.6`, `2001:db8::6`).
2.  Configure two static MPLS LSP pop-and-forward egress entries in the
    `DEFAULT` network instance on the DUT:
    *   **Static Pop-and-Forward Entry 1 (IPv4 Payload)**:
        *   **Incoming MPLS Label**: `100101`
        *   **Label Action**: Pop (`IMPLICIT_NULL`)
        *   **Payload Type**: IPv4
        *   **Next-Hop IP Address**: `192.0.2.6` (`ate:port2` IPv4 address in
            `DEFAULT`)
    *   **Static Pop-and-Forward Entry 2 (IPv6 Payload)**:
        *   **Incoming MPLS Label**: `100102`
        *   **Label Action**: Pop (`IMPLICIT_NULL`)
        *   **Payload Type**: IPv6
        *   **Next-Hop IP Address**: `2001:db8::6` (`ate:port2` IPv6 address in
            `DEFAULT`)

#### Canonical OC

```json
{
  "openconfig-network-instance:network-instances": {
    "network-instance": [
      {
        "name": "DEFAULT",
        "config": {
          "name": "DEFAULT",
          "type": "openconfig-network-instance-types:DEFAULT_INSTANCE"
        },
        "mpls": {
          "lsps": {
            "static-lsps": {
              "static-lsp": [
                {
                  "name": "static-lsp-pop-forward-ipv4",
                  "config": {
                    "name": "static-lsp-pop-forward-ipv4"
                  },
                  "egress": {
                    "config": {
                      "incoming-label": 100101,
                      "next-hop": "192.0.2.6",
                      "push-label": "IMPLICIT_NULL"
                    }
                  }
                },
                {
                  "name": "static-lsp-pop-forward-ipv6",
                  "config": {
                    "name": "static-lsp-pop-forward-ipv6"
                  },
                  "egress": {
                    "config": {
                      "incoming-label": 100102,
                      "next-hop": "2001:db8::6",
                      "push-label": "IMPLICIT_NULL"
                    }
                  }
                }
              ]
            }
          }
        },
        "policy-forwarding": {
          "policies": {
            "policy": [
              {
                "policy-id": "decap-mpls-in-udp",
                "config": {
                  "policy-id": "decap-mpls-in-udp"
                },
                "rules": {
                  "rule": [
                    {
                      "sequence-id": 10,
                      "config": {
                        "sequence-id": 10
                      },
                      "ipv4": {
                        "config": {
                          "destination-address": "203.0.113.0/28",
                          "protocol": 17
                        }
                      },
                      "transport": {
                        "config": {
                          "destination-port": 6635
                        }
                      },
                      "action": {
                        "config": {
                          "decapsulate-mpls-in-udp": true
                        }
                      }
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

#### Traffic Generation & Procedure

1.  Subscribe via `gNMI.Subscribe` to the AFT MPLS label-entry counter paths in
    the `DEFAULT` network instance for labels `100101` and `100102` and record
    their baseline counter values (`initial_pkts` and `initial_octets`,
    defaulting to `0` if not yet populated prior to traffic):
    *   `/network-instances/network-instance[name=DEFAULT]/afts/mpls/label-entry[label=100101]/state/counters/packets-forwarded`
    *   `/network-instances/network-instance[name=DEFAULT]/afts/mpls/label-entry[label=100101]/state/counters/octets-forwarded`
    *   `/network-instances/network-instance[name=DEFAULT]/afts/mpls/label-entry[label=100102]/state/counters/packets-forwarded`
    *   `/network-instances/network-instance[name=DEFAULT]/afts/mpls/label-entry[label=100102]/state/counters/octets-forwarded`
2.  Configure and transmit **Flow 1 (`pop-forward-ipv4-flow`)** from `ate:port1`
    to `dut:port1`:
    *   **Packet Count**: `1000` packets
    *   **Frame Rate**: `500` pps
    *   **Frame Size**: `256` bytes (fixed)
    *   **Outer IPv4 Header**: Source `198.51.100.1`, Destination `203.0.113.1`,
        TTL `64`, DSCP `0`
    *   **Outer UDP Header**: Source Port `10000`, Destination Port `6635`
    *   **MPLS Header**: Label `100101`, S-bit `1` (Bottom-of-Stack), TTL `64`
    *   **Inner IPv4 Header**: Source `198.18.0.1`, Destination `198.18.1.1`,
        TTL `64`
3.  Configure and transmit **Flow 2 (`pop-forward-ipv6-flow`)** from `ate:port1`
    to `dut:port1`:
    *   **Packet Count**: `1000` packets
    *   **Frame Rate**: `500` pps
    *   **Frame Size**: `256` bytes (fixed)
    *   **Outer IPv4 Header**: Source `198.51.100.1`, Destination `203.0.113.1`,
        TTL `64`, DSCP `0`
    *   **Outer UDP Header**: Source Port `10001`, Destination Port `6635`
    *   **MPLS Header**: Label `100102`, S-bit `1` (Bottom-of-Stack), TTL `64`
    *   **Inner IPv6 Header**: Source `2001:db8:1::1`, Destination
        `2001:db8:2::1`, Hop Limit `64`

#### Verification & Pass/Fail Criteria

*   **Data-Plane Forwarding Verification**:
    *   `ate:port2` must receive all `1000` decapsulated IPv4 packets from Flow
        1 and all `1000` decapsulated IPv6 packets from Flow 2 with `0` packet
        loss.
*   **AFT MPLS Telemetry Counter Verification (`gNMI.Subscribe`)**:
    *   For `label-entry[label=100101]` under `network-instance[name=DEFAULT]`:
        *   `packets-forwarded` delta (`final_pkts - initial_pkts`) must equal
            `1000`.
        *   `octets-forwarded` delta (`final_octets - initial_octets`) must be
            `>= 1000 * inner_ipv4_packet_size`.
    *   For `label-entry[label=100102]` under `network-instance[name=DEFAULT]`:
        *   `packets-forwarded` delta (`final_pkts - initial_pkts`) must equal
            `1000`.
        *   `octets-forwarded` delta (`final_octets - initial_octets`) must be
            `>= 1000 * inner_ipv6_packet_size`.

---

### AFT-4.1.2: Static MPLS LSP Pop-and-Lookup (Network-Instance FIB Lookup)

#### Objective

Verify that AFT MPLS label-entry counters (`packets-forwarded` and
`octets-forwarded`) increment accurately for a static MPLS LSP pop-and-lookup
entry that pops the MPLS label and directs the inner packet to a non-default
L3VRF (`VRF-1`), where the inner IPv4 or IPv6 destination address is looked up
in `VRF-1`'s FIB/RIB and forwarded out the matched egress interface.

#### Configuration

1.  Create a non-default Layer 3 VRF network instance named `VRF-1` (`type:
    L3VRF`) on the DUT.
2.  Reassign `dut:port2` from the `DEFAULT` network instance to `VRF-1`,
    configure `dut:port2` with IPv4 address `192.0.2.5/30` and IPv6 address
    `2001:db8::5/126`, and verify ARP/ND resolution with `ate:port2`
    (`192.0.2.6`, `2001:db8::6`).
3.  Configure static routes inside `VRF-1` for the inner traffic destination
    prefixes pointing to `ate:port2`:
    *   **IPv4 Static Route in `VRF-1`**: Prefix `198.18.1.0/24` -> Next-Hop
        `192.0.2.6`
    *   **IPv6 Static Route in `VRF-1`**: Prefix `2001:db8:2::/64` -> Next-Hop
        `2001:db8::6`
4.  Configure a static MPLS LSP pop-and-lookup entry on the DUT:
    *   **Incoming MPLS Label**: `100201`
    *   **Label Action**: Pop and perform FIB/RIB route lookup on the inner IP
        packet's destination address inside target network instance `VRF-1`.

#### Canonical OC

```json
{
  "openconfig-interfaces:interfaces": {
    "interface": [
      {
        "name": "port2",
        "config": {
          "name": "port2",
          "type": "iana-if-type:ethernetCsmacd",
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
              "openconfig-if-ip:ipv4": {
                "config": {
                  "enabled": true
                },
                "addresses": {
                  "address": [
                    {
                      "ip": "192.0.2.5",
                      "config": {
                        "ip": "192.0.2.5",
                        "prefix-length": 30
                      }
                    }
                  ]
                }
              },
              "openconfig-if-ip:ipv6": {
                "config": {
                  "enabled": true
                },
                "addresses": {
                  "address": [
                    {
                      "ip": "2001:db8::5",
                      "config": {
                        "ip": "2001:db8::5",
                        "prefix-length": 126
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
  },
  "openconfig-network-instance:network-instances": {
    "network-instance": [
      {
        "name": "VRF-1",
        "config": {
          "name": "VRF-1",
          "type": "openconfig-network-instance-types:L3VRF"
        },
        "interfaces": {
          "interface": [
            {
              "id": "port2.0",
              "config": {
                "id": "port2.0",
                "interface": "port2",
                "subinterface": 0
              }
            }
          ]
        },
        "protocols": {
          "protocol": [
            {
              "identifier": "openconfig-policy-types:STATIC",
              "name": "STATIC",
              "config": {
                "identifier": "openconfig-policy-types:STATIC",
                "name": "STATIC"
              },
              "static-routes": {
                "static": [
                  {
                    "prefix": "198.18.1.0/24",
                    "config": {
                      "prefix": "198.18.1.0/24"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "index": "0",
                          "config": {
                            "index": "0",
                            "next-hop": "192.0.2.6"
                          }
                        }
                      ]
                    }
                  },
                  {
                    "prefix": "2001:db8:2::/64",
                    "config": {
                      "prefix": "2001:db8:2::/64"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "index": "0",
                          "config": {
                            "index": "0",
                            "next-hop": "2001:db8::6"
                          }
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
    ]
  }
}
```

#### Traffic Generation & Procedure

1.  Subscribe via `gNMI.Subscribe` to the AFT MPLS label-entry counter paths in
    the `DEFAULT` network instance for label `100201` and record the baseline
    counter values (`initial_pkts` and `initial_octets`, defaulting to `0` if
    not yet populated prior to traffic):
    *   `/network-instances/network-instance[name=DEFAULT]/afts/mpls/label-entry[label=100201]/state/counters/packets-forwarded`
    *   `/network-instances/network-instance[name=DEFAULT]/afts/mpls/label-entry[label=100201]/state/counters/octets-forwarded`
2.  Configure and transmit **Flow 1 (`pop-lookup-ipv4-flow`)** from `ate:port1`
    to `dut:port1`:
    *   **Packet Count**: `1000` packets
    *   **Frame Rate**: `500` pps
    *   **Frame Size**: `256` bytes (fixed)
    *   **Outer IPv4 Header**: Source `198.51.100.1`, Destination `203.0.113.1`,
        TTL `64`, DSCP `0`
    *   **Outer UDP Header**: Source Port `10002`, Destination Port `6635`
    *   **MPLS Header**: Label `100201`, S-bit `1` (Bottom-of-Stack), TTL `64`
    *   **Inner IPv4 Header**: Source `198.18.0.1`, Destination `198.18.1.1`
        (matches `198.18.1.0/24` route in `VRF-1`), TTL `64`
3.  Verify the counter increment after Flow 1, or record the intermediate
    counter state, then configure and transmit **Flow 2
    (`pop-lookup-ipv6-flow`)** from `ate:port1` to `dut:port1`:
    *   **Packet Count**: `1000` packets
    *   **Frame Rate**: `500` pps
    *   **Frame Size**: `256` bytes (fixed)
    *   **Outer IPv4 Header**: Source `198.51.100.1`, Destination `203.0.113.1`,
        TTL `64`, DSCP `0`
    *   **Outer UDP Header**: Source Port `10003`, Destination Port `6635`
    *   **MPLS Header**: Label `100201`, S-bit `1` (Bottom-of-Stack), TTL `64`
    *   **Inner IPv6 Header**: Source `2001:db8:1::1`, Destination
        `2001:db8:2::1` (matches `2001:db8:2::/64` route in `VRF-1`), Hop Limit
        `64`

#### Verification & Pass/Fail Criteria

*   **Data-Plane Forwarding Verification**:
    *   After outer MPLSoGUE decapsulation and popping MPLS label `100201`, the
        DUT must look up the inner packet's destination IP (`198.18.1.1` for
        Flow 1 and `2001:db8:2::1` for Flow 2) in `VRF-1`'s FIB/RIB and forward
        the decapsulated IP packets out `dut:port2` to `ate:port2`.
    *   `ate:port2` must receive all `1000` decapsulated IPv4 packets from Flow
        1 and all `1000` decapsulated IPv6 packets from Flow 2 with `0` packet
        loss.
*   **AFT MPLS Telemetry Counter Verification (`gNMI.Subscribe`)**:
    *   For `label-entry[label=100201]` under `network-instance[name=DEFAULT]`:
        *   After Flow 1 (`1000` IPv4 packets), `packets-forwarded` delta must
            equal `1000` and `octets-forwarded` delta must be
            `>= 1000 * inner_ipv4_packet_size`.
        *   After Flow 2 (`1000` IPv6 packets), the cumulative
            `packets-forwarded` delta across both flows must equal `2000` (an
            additional `1000` packets for Flow 2) and cumulative
            `octets-forwarded` delta must be
            `>= 1000 * inner_ipv4_packet_size + 1000 * inner_ipv6_packet_size`.

---

### Cleanup

Upon completion of the subtests, remove all static MPLS LSP pop-and-forward and
pop-and-lookup entries, remove the MPLSoGUE policy-forwarding decapsulation
configuration, reassign `dut:port2` back to the `DEFAULT` network instance, and
delete the non-default `VRF-1` network instance so that the DUT is restored to
its baseline operational state.

## OpenConfig Path and RPC Coverage

The below YAML defines the OpenConfig paths intended to be covered by this test.
OpenConfig paths used strictly for test environment setup are not listed here.

```yaml
paths:
  # AFT MPLS label-entry telemetry counters
  /network-instances/network-instance/afts/mpls/label-entry/state/counters/packets-forwarded:
  /network-instances/network-instance/afts/mpls/label-entry/state/counters/octets-forwarded:

rpcs:
  gnmi:
    gNMI.Subscribe:
```

## Required DUT platform

*   FFF
