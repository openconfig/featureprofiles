# TE-3.8: gRIBI Tunnel Recursion over Multi-Level LPM Underlays

## Summary

This test validates tunnel encapsulation (MPLS-over-GRE and MPLS-over-UDP)
resolving over multi-level Longest Prefix Match (LPM) gRIBI underlay routing
hierarchies. Specifically, it tests tunnel resolution across 3 levels of LPM
underlay recursion depth (1-hop direct, 2-hop recursive, and 3-hop recursive),
executed across two variants: with FEC-hierarchical enabled versus disabled on
the tunnel NextHopGroup entry.

## Testbed type

[TESTBED_DUT_ATE_2LINKS](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Topology

```text
+-------------------------------------------------------------+
|                         Test Setup                          |
+-------------------------------------------------------------+
|  ___________ DUT ___________     ___________ ATE ___________  |
| |                         |     |                         | |
| |  Port 1 (Ingress) <---------> | Port 1 (Ingress)        | |
| |                         |     |                         | |
| |  Port 2 (Egress)  <---------> | Port 2 (Egress)         | |
| |_________________________|     |_________________________| |
|                                                             |
+-------------------------------------------------------------+
```

## Baseline Setup

*   Connect DUT port-1 to ATE port-1 (Ingress port).
*   Connect DUT port-2 to ATE port-2 (Egress port).
*   Configure MTU 9216 on all interfaces.
*   Assign IP addresses:
    *   DUT port-1: `198.51.100.1/30` / `2001:db8:1::1/126`
    *   ATE port-1: `198.51.100.2/30` / `2001:db8:1::2/126`
    *   DUT port-2: `198.51.100.5/30` / `2001:db8:2::1/126`
    *   ATE port-2: `198.51.100.6/30` / `2001:db8:2::2/126`
*   Configure magic MAC `02:00:00:00:00:01` on ATE port-2.
*   Establish gRIBI client connection with DUT, negotiate `RIB_AND_FIB_ACK` as
    the requested `ack_type`, and persistence mode `PRESERVE`. Make client leader.

## Procedure

The underlay consists of an ordered layer chain:
*   **Layer 1 (Anchor)**: `198.51.100.101/32` $\to$ Connected NextHop (`198.51.100.6` on `dut:port2`)
*   **Layer 2 (Hop 1)**: `198.51.100.102/32` $\to$ NextHop IP `198.51.100.101`
*   **Layer 3 (Hop 2)**: `198.51.100.103/32` $\to$ NextHop IP `198.51.100.102`

The overlay route `192.0.2.1/32` points to the tunnel entry, whose destination is set to the top-of-stack underlay IP for each tested recursion depth.

### TE-3.8.1: Level-1 LPM Underlay (1-Hop Direct) with FEC-Hierarchical Disabled

1.  **Install Layer 1 Underlay Route via gRIBI:**
    *   `NextHop#10`: `ip-address: 198.51.100.6`, `interface-ref: port2`.
    *   `NextHopGroup#10`: contains `NextHop#10` (weight: 1).
    *   `IPv4Entry: 198.51.100.101/32` -> `NextHopGroup#10`.
    *   Verify `FIB_ACK` received.

2.  **Configure Tunnel Entry with FEC-Hierarchical Disabled:**
    *   Tunnel destination: `198.51.100.101` (Layer 1).
    *   `NextHop#100`: `encapsulate_header: MPLS`, `mpls_label_stack: [100000]`, `ip_address: 198.51.100.101`.
    *   `NextHopGroup#100`: contains `NextHop#100` (FEC-hierarchical: disabled).
    *   Overlay route: `IPv4Entry: 192.0.2.1/32` -> `NextHopGroup#100`.

3.  **Send Traffic & Verify Encapsulation:**
    *   Send IPv4 traffic from ATE port-1 destined to `192.0.2.1`.
    *   Validate 100% of packets arrive at ATE port-2 with outer GRE encapsulation, outer destination `198.51.100.101`, outer source `198.51.100.5`, and MPLS label `100000`.
    *   Verify 0% packet loss.

### TE-3.8.2: Level-1 LPM Underlay (1-Hop Direct) with FEC-Hierarchical Enabled

1.  **Install Layer 1 Underlay Route via gRIBI:**
    *   `IPv4Entry: 198.51.100.101/32` -> `NextHopGroup#10` (`198.51.100.6`, `port2`).

2.  **Configure Tunnel Entry with FEC-Hierarchical Enabled:**
    *   Tunnel destination: `198.51.100.101` (Layer 1).
    *   `NextHopGroup#100`: contains `NextHop#100` (FEC-hierarchical: enabled).
    *   Overlay route: `IPv4Entry: 192.0.2.1/32` -> `NextHopGroup#100`.

3.  **Send Traffic & Verify:**
    *   Send traffic destined to `192.0.2.1`.
    *   Validate 100% of packets arrive at ATE port-2 encapsulated with label `100000` with 0% loss.

### TE-3.8.3: Level-2 LPM Underlay (2-Hop Recursive) with FEC-Hierarchical Disabled

1.  **Install Layers 1 & 2 Underlay Routes via gRIBI:**
    *   Layer 1 (Anchor): `NextHop#10` (`198.51.100.6`, `port2`) -> `NextHopGroup#10` -> `IPv4Entry: 198.51.100.101/32`.
    *   Layer 2 (Hop 1): `NextHop#20` (`198.51.100.101`) -> `NextHopGroup#20` -> `IPv4Entry: 198.51.100.102/32`.

2.  **Configure Tunnel Entry with FEC-Hierarchical Disabled:**
    *   Tunnel destination: `198.51.100.102` (Layer 2).
    *   `NextHopGroup#100`: (FEC-hierarchical: disabled) -> `IPv4Entry: 192.0.2.1/32`.

3.  **Send Traffic & Verify:**
    *   Send IPv4 traffic destined to `192.0.2.1`.
    *   Validate packets resolve via flat FEC and egress encapsulated out port-2.

### TE-3.8.4: Level-2 LPM Underlay (2-Hop Recursive) with FEC-Hierarchical Enabled

1.  **Install Layers 1 & 2 Underlay Routes:**
    *   Layer 1 (Anchor): `IPv4Entry: 198.51.100.101/32` -> `NextHopGroup#10` (`198.51.100.6`, `port2`).
    *   Layer 2 (Hop 1): `IPv4Entry: 198.51.100.102/32` -> `NextHopGroup#20` (`198.51.100.101`).

2.  **Configure Tunnel Entry with FEC-Hierarchical Enabled:**
    *   Tunnel destination: `198.51.100.102` (Layer 2).
    *   `NextHopGroup#100`: (FEC-hierarchical: enabled) -> `IPv4Entry: 192.0.2.1/32`.

3.  **Send Traffic & Evaluate:**
    *   Send IPv4 traffic destined to `192.0.2.1`.
    *   **Standard Pass Criteria**: DUT resolves 2-level hierarchical FEC in hardware FIB; 100% of packets egress encapsulated with label `100000` on port-2.
    *   **Limitation State (Arista Bug 578368)**: If the platform rejects or drops 2-hop hierarchical FEC tunnel resolution, record deviation status.

### TE-3.8.5: Level-3 LPM Underlay (3-Hop Recursive) with FEC-Hierarchical Disabled

1.  **Install Layers 1, 2 & 3 Underlay Routes via gRIBI:**
    *   Layer 1 (Anchor): `NextHop#10` (`198.51.100.6`, `port2`) -> `NextHopGroup#10` -> `IPv4Entry: 198.51.100.101/32`.
    *   Layer 2 (Hop 1): `NextHop#20` (`198.51.100.101`) -> `NextHopGroup#20` -> `IPv4Entry: 198.51.100.102/32`.
    *   Layer 3 (Hop 2): `NextHop#30` (`198.51.100.102`) -> `NextHopGroup#30` -> `IPv4Entry: 198.51.100.103/32`.

2.  **Configure Tunnel Entry with FEC-Hierarchical Disabled:**
    *   Tunnel destination: `198.51.100.103` (Layer 3).
    *   `NextHopGroup#100`: (FEC-hierarchical: disabled) -> `IPv4Entry: 192.0.2.1/32`.

3.  **Send Traffic & Verify:**
    *   Send traffic destined to `192.0.2.1`.
    *   Validate 3-level LPM resolution under flat FEC.

### TE-3.8.6: Level-3 LPM Underlay (3-Hop Recursive) with FEC-Hierarchical Enabled

1.  **Install Layers 1, 2 & 3 Underlay Routes via gRIBI:**
    *   Underlay chain: `198.51.100.103` -> `198.51.100.102` -> `198.51.100.101` -> `[198.51.100.6, port2]`.

2.  **Configure Tunnel Entry with FEC-Hierarchical Enabled:**
    *   Tunnel destination: `198.51.100.103` (Layer 3).
    *   `NextHopGroup#100`: (FEC-hierarchical: enabled) -> `IPv4Entry: 192.0.2.1/32`.

3.  **Send Traffic & Verify:**
    *   Send traffic destined to `192.0.2.1`.
    *   Validate hardware FIB resolution behavior and traffic forwarding across 3 hierarchical levels.

## Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "name": "Ethernet1/1",
        "config": {
          "name": "Ethernet1/1",
          "enabled": true,
          "type": "iana-if-type:ethernetCsmacd"
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
                      "ip": "198.51.100.1",
                      "config": {
                        "ip": "198.51.100.1",
                        "prefix-length": 30
                      }
                    }
                  ]
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "ip": "2001:db8:1::1",
                      "config": {
                        "ip": "2001:db8:1::1",
                        "prefix-length": 126
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
        "name": "Ethernet1/2",
        "config": {
          "name": "Ethernet1/2",
          "enabled": true,
          "type": "iana-if-type:ethernetCsmacd"
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
                      "ip": "198.51.100.5",
                      "config": {
                        "ip": "198.51.100.5",
                        "prefix-length": 30
                      }
                    }
                  ]
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "ip": "2001:db8:2::1",
                      "config": {
                        "ip": "2001:db8:2::1",
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
  "network-instances": {
    "network-instance": [
      {
        "name": "DEFAULT",
        "config": {
          "name": "DEFAULT",
          "type": "openconfig-network-instance-types:DEFAULT_INSTANCE"
        },
        "interfaces": {
          "interface": [
            {
              "id": "Ethernet1/1.0",
              "config": {
                "id": "Ethernet1/1.0",
                "interface": "Ethernet1/1",
                "subinterface": 0
              }
            },
            {
              "id": "Ethernet1/2.0",
              "config": {
                "id": "Ethernet1/2.0",
                "interface": "Ethernet1/2",
                "subinterface": 0
              }
            }
          ]
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  ## Configuration coverage
  /interfaces/interface/config/name:
  /interfaces/interface/config/type:
  /interfaces/interface/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/config/index:
  /interfaces/interface/subinterfaces/subinterface/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /network-instances/network-instance/config/name:
  /network-instances/network-instance/config/type:
  /network-instances/network-instance/interfaces/interface/config/id:
  /network-instances/network-instance/interfaces/interface/config/interface:
  /network-instances/network-instance/interfaces/interface/config/subinterface:

  ## Telemetry & State paths
  /interfaces/interface/state/oper-status:
  /interfaces/interface/subinterfaces/subinterface/state/oper-status:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group-network-instance:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address:
  /network-instances/network-instance/afts/next-hops/next-hop/interface-ref/state/interface:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/state/type:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/mpls/state/mpls-label-stack:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
  gribi:
    gRIBI.Modify:
    gRIBI.Flush:
    gRIBI.Get:
```

## Required DUT platform

FFF
