# TE-16.1: basic encapsulation tests

## Summary

Test basic encapsulation and decapsulation behaviors, including DSCP, TTL, and
Explicit Congestion Notification (ECN) bit management during IP-in-IP
encapsulation, congestion propagation during decapsulation, and simultaneous
gNMI `SAMPLE` streaming telemetry and counter accuracy for gRIBI IP-in-IP
encapsulation tunnel next-hop counters and subinterface/interface IPv4 and IPv6
egress/ingress counters.

## Topology

ATE port-1 <------> port-1 DUT
DUT port-2 <------> port-2 ATE
DUT port-3 <------> port-3 ATE
DUT port-4 <------> port-4 ATE
DUT port-5 <------> port-5 ATE

## Baseline setup

* Apply the following vrf selection policy to DUT port-1

```
# DSCP value that will be matched to ENCAP_TE_VRF_A
* dscp_encap_a_1 = 10
* dscp_encap_a_2 = 18

# DSCP value that will be matched to ENCAP_TE_VRF_B
* dscp_encap_b_1 = 20
* dscp_encap_b_2 = 28

# DSCP value that will NOT be matched to any VRF for encapsulation.
* dscp_encap_no_match = 30

# Magic source IP addresses used in VRF selection policy
* ipv4_outer_src_111 = 198.51.100.111
* ipv4_outer_src_222 = 198.51.100.222

# Magic destination MAC address
* magic_mac = 02:00:00:00:00:01`
```

```
network-instances {
    network-instance {
        name: DEFAULT
        policy-forwarding {
            policies {
                policy {
                    policy-id: "vrf_selection_policy_c"
                    rules {
                        rule {
                            sequence-id: 1
                            ipv4 {
                                protocol: 4
                                dscp-set: [dscp_encap_a_1, dscp_encap_a_2]
                                source-address: "ipv4_outer_src_222"
                            }
                            action {
                                decap-network-instance: "DECAP_TE_VRF"
                                post-network-instance: "ENCAP_TE_VRF_A"
                                decap-fallback-network-instance: "TE_VRF_222"
                            }
                        }
                        rule {
                            sequence-id: 2
                            ipv4 {
                                protocol: 41
                                dscp-set: [dscp_encap_a_1, dscp_encap_a_2]
                                source-address: "ipv4_outer_src_222"
                            }
                            action {
                                decap-network-instance: "DECAP_TE_VRF"
                                post-network-instance: "ENCAP_TE_VRF_A"
                                decap-fallback-network-instance: "TE_VRF_222"
                            }
                        }
                        rule {
                            sequence-id: 3
                            ipv4 {
                                protocol: 4
                                dscp-set: [dscp_encap_a_1, dscp_encap_a_2]
                                source-address: "ipv4_outer_src_111"
                            }
                            action {
                                decap-network-instance: "DECAP_TE_VRF"
                                post-network-instance: "ENCAP_TE_VRF_A"
                                decap-fallback-network-instance: "TE_VRF_111"
                            }
                        }
                        rule {
                            sequence-id: 4
                            ipv4 {
                                protocol: 41
                                dscp-set: [dscp_encap_a_1, dscp_encap_a_2]
                                source-address: "ipv4_outer_src_111"
                            }
                            action {
                                decap-network-instance: "DECAP_TE_VRF"
                                post-network-instance: "ENCAP_TE_VRF_A"
                                decap-fallback-network-instance: "TE_VRF_111"
                            }
                        }
                        rule {
                            sequence-id: 5
                            ipv4 {
                                protocol: 4
                                dscp-set: [dscp_encap_b_1, dscp_encap_b_2]
                                source-address: "ipv4_outer_src_222"
                            }
                            action {
                                decap-network-instance: "DECAP_TE_VRF"
                                post-network-instance: "ENCAP_TE_VRF_B"
                                decap-fallback-network-instance: "TE_VRF_222"
                            }
                        }
                        rule {
                            sequence-id: 6
                            ipv4 {
                                protocol: 41
                                dscp-set: [dscp_encap_b_1, dscp_encap_b_2]
                                source-address: "ipv4_outer_src_222"
                            }
                            action {
                                decap-network-instance: "DECAP_TE_VRF"
                                post-network-instance: "ENCAP_TE_VRF_B"
                                decap-fallback-network-instance: "TE_VRF_222"
                            }
                        }
                        rule {
                            sequence-id: 7
                            ipv4 {
                                protocol: 4
                                dscp-set: [dscp_encap_b_1, dscp_encap_b_2]
                                source-address: "ipv4_outer_src_111"
                            }
                            action {
                                decap-network-instance: "DECAP_TE_VRF"
                                post-network-instance: "ENCAP_TE_VRF_B"
                                decap-fallback-network-instance: "TE_VRF_111"
                            }
                        }
                        rule {
                            sequence-id: 8
                            ipv4 {
                                protocol: 41
                                dscp-set: [dscp_encap_b_1, dscp_encap_b_2]
                                source-address: "ipv4_outer_src_111"
                            }
                            action {
                                decap-network-instance: "DECAP_TE_VRF"
                                post-network-instance: "ENCAP_TE_VRF_B"
                                decap-fallback-network-instance: "TE_VRF_111"
                            }
                        }
                        rule {
                            sequence-id: 9
                            ipv4 {
                                protocol: 4
                                source-address: "ipv4_outer_src_222"
                            }
                            action {
                                decap-network-instance: "DECAP_TE_VRF"
                                post-network-instance: "DEFAULT"
                                decap-fallback-network-instance: "TE_VRF_222"
                            }
                        }
                        rule {
                            sequence-id: 10
                            ipv4 {
                                protocol: 41
                                source-address: "ipv4_outer_src_222"
                            }
                            action {
                                decap-network-instance: "DECAP_TE_VRF"
                                post-network-instance: "DEFAULT"
                                decap-fallback-network-instance: "TE_VRF_222"
                            }
                        }
                        rule {
                            sequence-id: 11
                            ipv4 {
                                protocol: 4
                                source-address: "ipv4_outer_src_111"
                            }
                            action {
                                decap-network-instance: "DECAP_TE_VRF"
                                post-network-instance: "DEFAULT"
                                decap-fallback-network-instance: "TE_VRF_111"
                            }
                        }
                        rule {
                            sequence-id: 12
                            ipv4 {
                                protocol: 41
                                source-address: "ipv4_outer_src_111"
                            }
                            action {
                                decap-network-instance: "DECAP_TE_VRF"
                                post-network-instance: "DEFAULT"
                                decap-fallback-network-instance: "TE_VRF_111"
                            }
                        }
                        rule {
                            sequence-id: 13
                            ipv4 {
                                dscp-set: [dscp_encap_a_1, dscp_encap_a_2]
                            }
                            action {
                                network-instance: "ENCAP_TE_VRF_A"
                            }
                        }
                        rule {
                            sequence-id: 14
                            ipv6 {
                                dscp-set: [dscp_encap_a_1, dscp_encap_a_2]
                            }
                            action {
                                network-instance: "ENCAP_TE_VRF_A"
                            }
                        }
                        rule {
                            sequence-id: 15
                            ipv4 {
                                dscp-set: [dscp_encap_b_1, dscp_encap_b_2]
                            }
                            action {
                                network-instance: "ENCAP_TE_VRF_B"
                            }
                        }
                        rule {
                            sequence-id: 16
                            ipv6 {
                                dscp-set: [dscp_encap_b_1, dscp_encap_b_2]
                            }
                            action {
                                network-instance: "ENCAP_TE_VRF_B"
                            }
                        }
                        rule {
                            sequence-id: 17
                            action {
                                network-instance: "DEFAULT"
                            }
                        }
                    }
                }
            }
        }
    }
}
```

*   Using gRIBI, install the following gRIBI AFTs, and validate the specified
    behavior.

```
IPv6Entry {2015:aa8::/32 (ENCAP_TE_VRF_A)} -> NHG#10 (DEFAULT VRF)
IPv4Entry {138.0.11.0/24 (ENCAP_TE_VRF_A)} -> NHG#10 (DEFAULT VRF) -> {
  {NH#201, DEFAULT VRF, weight:1},
  {NH#202, DEFAULT VRF, weight:3},
}
NH#201 -> {
  encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
  ip_in_ip {
    dst_ip: "203.0.113.1"
    src_ip: "ipv4_outer_src_111"
  }
  network_instance: "TE_VRF_111"
}
NH#202 -> {
  encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
  ip_in_ip {
    dst_ip: "203.10.113.2"
    src_ip: "ipv4_outer_src_111"
  }
  network_instance: "TE_VRF_111"
}

// 203.0.113.1 is the tunnel IP address.

IPv4Entry {203.0.113.1/32 (TE_VRF_111)} -> NHG#1 (DEFAULT VRF) -> {
  {NH#1, DEFAULT VRF, weight:1,ip_address=192.0.2.111},
  {NH#2, DEFAULT VRF, weight:3,ip_address=192.0.2.222},
}
IPv4Entry {192.0.2.111/32 (DEFAULT VRF)} -> NHG#2 (DEFAULT VRF) -> {
  {NH#10, DEFAULT VRF, weight:1,mac_address:magic_mac, interface-ref:dut-port-2-interface},
  {NH#11, DEFAULT VRF, weight:3,mac_address:magic_mac, interface-ref:dut-port-3-interface},
}
IPv4Entry {192.0.2.222/32 (DEFAUlT VRF)} -> NHG#3 (DEFAULT VRF) -> {
  {NH#100, DEFAULT VRF, weight:2,mac_address:magic_mac, interface-ref:dut-port-4-interface},
  {NH#101, DEFAULT VRF, weight:3,mac_address:magic_mac, interface-ref:dut-port-5-interface},
}

// 203.10.113.2 is the tunnel IP address. Note that the NHG#1 is shared by both tunnels.

IPv4Entry {203.10.113.2/32 (TE_VRF_111)} -> NHG#1 (DEFAULT VRF) -> <omitted for brevity>

// Decapsulation AFTs in DECAP_TE_VRF:
// When packets match decap rules in vrf_selection_policy_c, the outer IP header is removed
// and inner packet lookup occurs in DECAP_TE_VRF.

IPv4Entry {198.18.11.0/24 (DECAP_TE_VRF)} -> NHG#1000 (DEFAULT VRF) -> {
  {NH#1001, DEFAULT VRF}
}
NH#1001 -> {
  decapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
  network_instance: "DEFAULT"
}

IPv6Entry {2001:db8:2016::/64 (DECAP_TE_VRF)} -> NHG#1000 (DEFAULT VRF) -> {
  {NH#1001, DEFAULT VRF}
}

// Fallback default routing entries in ENCAP_TE_VRF_A to DEFAULT VRF:
IPv4Entry {0.0.0.0/0 (ENCAP_TE_VRF_A)} -> NHG#2000 (DEFAULT VRF) -> {
  {NH#2001, DEFAULT VRF, ip_address: 192.0.2.100}
}
IPv4Entry {192.0.2.100/32 (DEFAULT VRF)} -> NHG#2002 (DEFAULT VRF) -> {
  {NH#2003, DEFAULT VRF, mac_address: magic_mac, interface-ref: dut-port-2-interface}
}

IPv6Entry {::/0 (ENCAP_TE_VRF_A)} -> NHG#3000 (DEFAULT VRF) -> {
  {NH#3001, DEFAULT VRF, ip_address: 2001:db8::100}
}
IPv6Entry {2001:db8::100/128 (DEFAULT VRF)} -> NHG#3002 (DEFAULT VRF) -> {
  {NH#3003, DEFAULT VRF, mac_address: magic_mac, interface-ref: dut-port-2-interface}
}
```

## Procedure

#### Test-1, IPv4 traffic WCMP Encap

Send packets to DUT port-1. The outer v4 header has the destination addresses
138.0.11.8. Validate that:

*   All egress packets (100%) are IPinIP (4in4) encapped.
*   Packets are encapped to the tunnel IPs in the specified ratio. Specifically,
    25% of the egress packets should have the destination address 203.0.113.1,
    and 75% of the egress packets should have the destination address
    203.10.113.2.
*   The encapped/tunneled packets should be distributed hierarchically per the
    weight.
*   The DSCP value is copied from the inner header to the outer header.
*   The TTL value is copied from the inner header to the outer header.

#### Test-2, IPv6 traffic WCMP Encap

Send packets to DUT port-1. The outer v6 header has the destination addresses
2015:aa8::1. Validate that:

*   All egress packets (100%) are 6in4 encapped.
*   Packets are encapped to the tunnel IPs in the specified ratio. Specifically,
    25% of the egress packets should have the destination address 203.0.113.1,
    and 75% of the egress packets should have the destination address
    203.10.113.2.
*   The encapped/tunneled packets should be distributed hierarchically per the
    weight.
*   The DSCP value is copied from the inner header to the outer header.
*   The TTL value is copied from the inner header to the outer header.

#### Test-3, IPinIP Traffic Encap

Tests support for encap of IPinIP IPv4 (IP protocol 4) traffic. Specifically, in
this test we’ll focus on tunnel traffic identification using
`ipv4_outer_src_111``and`ipv4_outer_src_222``.

1.  Send 4in4 (IP protocol 4) and 6in4 (IP protocol 41) packets to DUT port-1.
    *   The outer v4 header has the destination address 138.0.11.8.
    *   The outer v4 header has the source address that’s not
        `ipv4_outer_src_111``or`ipv4_outer_src_222``. For example, we can use
        198.100.200.123.
    *   The outer v4 header should have DSCP value `dscp_encap_a_1`.
2.  Validate that:
    *   All egress packets (100%) are IPinIP (4in4) encapped.
    *   Packets are encapped to the tunnel IPs in the specified ratio.
        Specifically, 25% of the egress packets should have the destination
        address 203.0.113.1, and 75% of the egress packets should have the
        destination address 203.10.113.2.
    *   The encapped/tunneled packets should be distributed hierarchically per
        the weight.
    *   The DSCP value is copied from the inner header to the outer header.
    *   The TTL value is copied from the inner header to the outer header.

#### Test-4, ECN Encap Copy

Validate that the 2-bit ECN codepoints are correctly copied and preserved from
the inner IP header to the outer tunnel IP header during encapsulation across
all defined ECN states:

*   ECN codepoints tested:
    *   `00`: Not-ECT (Not ECN-Capable Transport)
    *   `01`: ECT(1) (ECN-Capable Transport 1)
    *   `10`: ECT(0) (ECN-Capable Transport 0)
    *   `11`: CE (Congestion Experienced)
*   Traffic types:
    *   Native IPv4 traffic to `138.0.11.8` (inner IPv4, encapsulated as 4in4).
    *   Native IPv6 traffic to `2015:aa8::1` (inner IPv6, encapsulated as 6in4).
*   Validate that:
    *   100% of egress packets are encapsulated with an outer IPv4 tunnel header.
    *   For each flow, the 2-bit ECN field of the outer IPv4 header (`TOS & 0x03`)
        matches the 2-bit ECN field of the inner packet header (`inner
        TOS & 0x03` for IPv4 or `inner TrafficClass & 0x03` for IPv6). For
        platforms adhering to RFC 3168 Section 9.1.1 (such as Juniper PTX),
        an arriving inner `ECT(1)` packet is encapsulated as outer `ECT(0)`.
    *   DSCP and TTL copy behavior remains compliant with Test-1 and Test-2.
    *   Zero packet loss across all flows.

#### Test-5, ECN Decap Congestion Propagation

Validate that the DUT decapsulates tunneled traffic and propagates congestion
notifications from the outer tunnel header to the inner IP header delivered to
the end receiver:

1.  Send pre-encapsulated IP-in-IP (IP protocol 4) and 6in4 (IP protocol 41)
    packets to DUT port-1:
    *   Outer IPv4 header source address matching decap rule in
        `vrf_selection_policy_c` (`ipv4_outer_src_111 = 198.51.100.111`).
    *   Outer destination IP matching tunnel endpoint prefix (`198.18.11.8`).
    *   Inner packet destination addressed to receiver in the DEFAULT VRF.
2.  Test combinations of outer and inner ECN codepoints:
    *   **Congestion Marking Propagation**:
        *   Outer = `11` (CE), Inner = `10` (ECT(0)) -> Expected Decapped Inner =
            `11` (CE).
        *   Outer = `11` (CE), Inner = `01` (ECT(1)) -> Expected Decapped Inner =
            `11` (CE).
    *   **Normal / Non-Congested Transport**:
        *   Outer = `10` (ECT(0)), Inner = `10` (ECT(0)) -> Expected Decapped Inner
            = `10` (ECT(0)).
        *   Outer = `01` (ECT(1)), Inner = `01` (ECT(1)) -> Expected Decapped Inner
            = `01` (ECT(1)).
        *   Outer = `00` (Not-ECT), Inner = `00` (Not-ECT) -> Expected Decapped Inner
            = `00` (Not-ECT).
3.  Validate that:
    *   The DUT decapsulates the packets (removes outer IPv4 tunnel header).
    *   The decapsulated inner packet is forwarded out egress port-2 according
        to the DEFAULT VRF route.
    *   When the outer tunnel header indicates congestion (`CE = 11`) on an
        ECN-capable inner packet (`ECT(0)` or `ECT(1)`), the inner packet has
        its ECN field updated to `CE (11)` upon egress.
    *   When the outer header does not indicate congestion, the inner packet ECN
        bits remain unchanged.
    *   Decap fallback to DEFAULT VRF functions correctly when no explicit
        matching route exists in `ENCAP_TE_VRF_A`.
    *   Zero packet loss across all valid flows.
    *   Note: Test-5 specifically tests tunnel decapsulation, TTL preservation,
        and ECN congestion propagation. Default platform egress Class of
        Service (CoS) drop-precedence remarking on physical interfaces (e.g. Junos
        mapping AF11 to CS1) is permitted and logged.

### ECN Standards & Platform Support

Two specifications govern Explicit Congestion Notification (ECN) handling in IP tunnels:
*   **RFC 3168 (Section 9.1.1)**: Specifies that the encapsulator sets the outgoing outer ECN field to `ECT(0)` whenever the arriving inner header is either `ECT(0)` or `ECT(1)`.
*   **RFC 6040 (Section 4.1)**: Specifies copying `ECT(1)` to the outer header in Normal Mode.

Platform compliance in this test suite:
*   **Encapsulation**:
    *   **Juniper (PTX)**: Implements **RFC 3168** encapsulation (inner `ECT(1)` is encapsulated as outer `ECT(0)`).
    *   **Arista, Cisco, Nokia**: Implement **RFC 6040** encapsulation (inner `ECT(1)` is mirrored as outer `ECT(1)`).
*   **Decapsulation**:
    *   All tested platforms (including Juniper) propagate outer tunnel congestion (`CE = 11`) to the decapsulated inner packet per RFC 6040 decapsulation rules.

#### Test-6, Simultaneous IP-in-IP Encapsulation Tunnel and Subinterface Egress Streaming Telemetry Counters Validation

Validate that enabling and streaming both gRIBI IP-in-IP encapsulation tunnel
next-hop counters and subinterface/interface IPv4 and IPv6 egress and ingress
counters simultaneously operates without feature conflict, counter suppression,
or stale (`0`) counter updates during 4in4 and 6in4 encapsulation:

1.  **Baseline Setup and Simultaneous gNMI `SAMPLE` Streaming Subscriptions**:
    *   Ensure the baseline VRF selection policy (`vrf_selection_policy_c`) on
        DUT `port-1` and the gRIBI IP-in-IP encapsulation AFT hierarchy
        (`NH#201` with `dst_ip = 203.0.113.1`, weight `1` and `NH#202` with
        `dst_ip = 203.10.113.2`, weight `3` in `NHG#10`, resolving recursively
        out `port-2`, `port-3`, `port-4`, and `port-5`) are programmed and
        `FIB_PROGRAMMED`.
    *   Establish concurrent gNMI `Subscribe` sessions in `STREAM` mode with
        `SubscriptionMode: SAMPLE` (at `sample_interval = 10s`, or `5s`–`10s`
        per platform capability) across both IP-in-IP encapsulation tunnel
        counters and interface/subinterface counters simultaneously:
        *   **gRIBI IP-in-IP Encapsulation Tunnel Next-Hop & AFT Entry Counter Paths**:
            *   `/network-instances/network-instance[name=DEFAULT]/afts/next-hops/next-hop[index=201]/state/counters/packets-forwarded`
            *   `/network-instances/network-instance[name=DEFAULT]/afts/next-hops/next-hop[index=201]/state/counters/octets-forwarded`
            *   `/network-instances/network-instance[name=DEFAULT]/afts/next-hops/next-hop[index=202]/state/counters/packets-forwarded`
            *   `/network-instances/network-instance[name=DEFAULT]/afts/next-hops/next-hop[index=202]/state/counters/octets-forwarded`
            *   `/network-instances/network-instance[name=ENCAP_TE_VRF_A]/afts/ipv4-unicast/ipv4-entry[prefix=138.0.11.0/24]/state/counters/packets-forwarded`
            *   `/network-instances/network-instance[name=ENCAP_TE_VRF_A]/afts/ipv4-unicast/ipv4-entry[prefix=138.0.11.0/24]/state/counters/octets-forwarded`
            *   `/network-instances/network-instance[name=ENCAP_TE_VRF_A]/afts/ipv6-unicast/ipv6-entry[prefix=2015:aa8::/64]/state/counters/packets-forwarded`
            *   `/network-instances/network-instance[name=ENCAP_TE_VRF_A]/afts/ipv6-unicast/ipv6-entry[prefix=2015:aa8::/64]/state/counters/octets-forwarded`
        *   **Subinterface IPv4 and IPv6 Egress and Ingress Counter Paths** (on
            ingress `port-1` `subinterface 0` and egress `port-2`, `port-3`,
            `port-4`, `port-5` `subinterface 0`):
            *   `/interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-pkts`
            *   `/interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-forwarded-octets`
            *   `/interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-pkts`
            *   `/interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-forwarded-octets`
            *   `/interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-pkts`
            *   `/interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-forwarded-octets`
            *   `/interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-pkts`
            *   `/interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-forwarded-octets`
        *   **Physical Interface Egress and Ingress Counter Paths** (on ingress
            `port-1` and egress `port-2`, `port-3`, `port-4`, `port-5`):
            *   `/interfaces/interface/state/counters/out-pkts`
            *   `/interfaces/interface/state/counters/out-octets`
            *   `/interfaces/interface/state/counters/in-pkts`
            *   `/interfaces/interface/state/counters/in-octets`
    *   Record initial baseline counter readings (`C_before`) from the
        initial streaming sample updates (and via `gNMI.Get`) prior to starting
        traffic.
2.  **Sustained Encapsulated Traffic Transmission (4in4 and 6in4)**:
    *   From ATE `port-1`, transmit continuous dual-stack traffic matching
        `vrf_selection_policy_c` (`dscp = dscp_encap_a_1 = 10` ->
        `ENCAP_TE_VRF_A`) for at least `30` seconds (sufficient to collect at
        least `3` to `10` periodic `SAMPLE` updates):
        *   **IPv4 Flow (4in4 Encap)**: Source IPv4 `192.0.2.2`, destination
            IPv4 `138.0.11.8`, DSCP `10`, frame size `512` bytes, rate
            `1,000 pps`.
        *   **IPv6 Flow (6in4 Encap)**: Source IPv6 `2001:db8::192:0:2:2`,
            destination IPv6 `2015:aa8::1`, DSCP `10`, frame size `512` bytes,
            rate `1,000 pps`.
3.  **Periodic Streaming Telemetry & Simultaneous Feature Coexistence Verification**:
    *   While traffic is actively flowing, verify that the DUT emits periodic
        `SAMPLE` telemetry updates at the configured interval for **both** the
        gRIBI IP-in-IP encapsulation tunnel next-hops (`NH#201` and `NH#202`)
        **and** the egress subinterfaces (`port-2` through `port-5`
        `subinterface 0` `ipv4/state/counters/out-pkts` and
        `out-forwarded-octets`) and ingress subinterface (`port-1`
        `subinterface 0` `ipv4/state/counters/in-pkts`, `in-forwarded-octets`
        and `ipv6/state/counters/in-pkts`, `in-forwarded-octets`).
    *   Verify that enabling IP-in-IP encapsulation tunnel counters does **not**
        cause subinterface IPv4/IPv6 egress counters (`out-pkts`,
        `out-forwarded-octets`) to freeze or report stale (`0` rate/delta)
        values, and conversely that enabling subinterface counters does not
        suppress IP-in-IP tunnel counter updates.
    *   Verify that consecutive streaming sample updates (`S_1, S_2, ..., S_k`)
        for active tunnel next-hops (`NH#201`, `NH#202`) and active egress
        subinterfaces (`port-2` through `port-5`) are strictly monotonically
        increasing (`S_1 < S_2 < ... < S_k`) during traffic transmission.
4.  **Post-Traffic Counter Delta & Hierarchical Distribution Accuracy**:
    *   Stop traffic on ATE `port-1`, wait for at least one full
        `sample_interval` after traffic cessation to receive final settled
        streaming counter values (`C_after`), and validate:
        *   **IP-in-IP Encap Tunnel Next-Hop Counters**:
            *   The combined packet delta across `NH#201` and `NH#202`
                (`delta_NH201_pkts + delta_NH202_pkts`) equals the total
                encapsulated packets transmitted across the 4in4 and 6in4 flows
                (`N_tx_v4 + N_tx_v6`, within `+/- 1%` tolerance for
                control-plane packets).
            *   Individual tunnel next-hop packet and octet deltas reflect the
                configured `1:3` WCMP weights on `NHG#10`: `delta_NH201` is
                approximately `25%` (`+/- 5%`) and `delta_NH202` is
                approximately `75%` (`+/- 5%`) of total encapsulated traffic.
        *   **Subinterface and Physical Interface Egress Counters**:
            *   Because both 4in4 and 6in4 encapsulated packets egress the DUT
                with an outer IPv4 tunnel header, the sum of egress subinterface
                IPv4 packet deltas (`ipv4/state/counters/out-pkts`) and physical
                interface egress packet deltas (`state/counters/out-pkts`)
                across DUT `port-2`, `port-3`, `port-4`, and `port-5` equals the
                total encapsulated packets received on ATE `port-2` through
                `port-5` (`N_rx_total = N_tx_v4 + N_tx_v6`).
            *   The sum of egress subinterface IPv4 forwarded octet deltas
                (`ipv4/state/counters/out-forwarded-octets`) across `port-2`
                through `port-5` matches the total L3 outer IPv4 encapsulated
                payload octets forwarded across `port-2` through `port-5`.
            *   Per-port egress subinterface IPv4 counter deltas on `port-2`,
                `port-3`, `port-4`, and `port-5` match the expected
                hierarchical WCMP distribution (`6.25%`, `18.75%`, `30%`, and
                `45%` with `+/- 2%` tolerance).
            *   For native IPv6 egress validation (when native IPv6 traffic is
                forwarded out `port-2` via the `::/0` fallback route in
                `ENCAP_TE_VRF_A` -> `NHG#3000` -> `NHG#3002`),
                `/interfaces/interface[name=port-2]/subinterfaces/subinterface[index=0]/ipv6/state/counters/out-pkts`
                and `out-forwarded-octets` increment monotonically via `SAMPLE`
                streaming updates and match the native IPv6 packets and octets
                received on ATE `port-2`.
        *   **Ingress Subinterface Counters & Post-Traffic Stability**:
            *   Ingress subinterface counters on DUT `port-1`
                (`ipv4/state/counters/in-pkts`, `in-forwarded-octets` and
                `ipv6/state/counters/in-pkts`, `in-forwarded-octets`) equal the
                respective native IPv4 (`N_tx_v4`) and IPv6 (`N_tx_v6`) packets
                and octets sent by ATE `port-1`.
            *   Subsequent streaming sample updates after traffic stops remain
                stable with no stale backlog or unexpected counter drift.

## Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "dutPort1",
          "enabled": true,
          "name": "Ethernet1",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "ethernet": {
          "config": {
            "mac-address": "02:01:00:00:00:01"
          }
        },
        "name": "Ethernet1",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "enabled": true,
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
                        "ip": "2001:db8::192:0:2:1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8::192:0:2:1"
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
          "description": "dutPort2",
          "enabled": true,
          "name": "Ethernet2",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "Ethernet2",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "enabled": true,
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
                        "ip": "2001:db8::192:0:2:5",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8::192:0:2:5"
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
          "description": "dutPort3",
          "enabled": true,
          "name": "Ethernet3",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "Ethernet3",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "enabled": true,
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
                        "ip": "2001:db8::192:0:2:9",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8::192:0:2:9"
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
          "description": "dutPort4",
          "enabled": true,
          "name": "Ethernet4",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "Ethernet4",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "enabled": true,
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
                        "ip": "2001:db8::192:0:2:d",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8::192:0:2:d"
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
          "description": "dutPort5",
          "enabled": true,
          "name": "Ethernet5",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "Ethernet5",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "enabled": true,
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.17",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.17"
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
                        "ip": "2001:db8::192:0:2:11",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8::192:0:2:11"
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
        "policy-forwarding": {
          "interfaces": {
            "interface": [
              {
                "config": {
                  "apply-vrf-selection-policy": "vrf_selection_policy_c",
                  "interface-id": "Ethernet1"
                },
                "interface-id": "Ethernet1",
                "interface-ref": {
                  "config": {
                    "interface": "Ethernet1",
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
                  "policy-id": "vrf_selection_policy_c",
                  "type": "VRF_SELECTION_POLICY"
                },
                "policy-id": "vrf_selection_policy_c",
                "rules": {
                  "rule": [
                    {
                      "action": {
                        "config": {
                          "decap-fallback-network-instance": "TE_VRF_111",
                          "decap-network-instance": "DECAP_TE_VRF",
                          "post-decap-network-instance": "ENCAP_TE_VRF_A"
                        }
                      },
                      "config": {
                        "sequence-id": 3
                      },
                      "ipv4": {
                        "config": {
                          "dscp-set": [
                            10,
                            18
                          ],
                          "protocol": 4,
                          "source-address": "198.51.100.111/32"
                        }
                      },
                      "sequence-id": 3
                    },
                    {
                      "action": {
                        "config": {
                          "network-instance": "ENCAP_TE_VRF_A"
                        }
                      },
                      "config": {
                        "sequence-id": 13
                      },
                      "ipv4": {
                        "config": {
                          "dscp-set": [
                            10,
                            18
                          ]
                        }
                      },
                      "sequence-id": 13
                    },
                    {
                      "action": {
                        "config": {
                          "network-instance": "ENCAP_TE_VRF_A"
                        }
                      },
                      "config": {
                        "sequence-id": 14
                      },
                      "ipv6": {
                        "config": {
                          "dscp-set": [
                            10,
                            18
                          ]
                        }
                      },
                      "sequence-id": 14
                    },
                    {
                      "action": {
                        "config": {
                          "network-instance": "DEFAULT"
                        }
                      },
                      "config": {
                        "sequence-id": 17
                      },
                      "sequence-id": 17
                    }
                  ]
                }
              }
            ]
          }
        }
      },
      {
        "config": {
          "name": "DECAP_TE_VRF",
          "type": "openconfig-network-instance-types:L3VRF"
        },
        "name": "DECAP_TE_VRF"
      },
      {
        "config": {
          "name": "TE_VRF_111",
          "type": "openconfig-network-instance-types:L3VRF"
        },
        "name": "TE_VRF_111"
      },
      {
        "config": {
          "name": "TE_VRF_222",
          "type": "openconfig-network-instance-types:L3VRF"
        },
        "name": "TE_VRF_222"
      },
      {
        "config": {
          "name": "ENCAP_TE_VRF_A",
          "type": "openconfig-network-instance-types:L3VRF"
        },
        "name": "ENCAP_TE_VRF_A"
      },
      {
        "config": {
          "name": "ENCAP_TE_VRF_B",
          "type": "openconfig-network-instance-types:L3VRF"
        },
        "name": "ENCAP_TE_VRF_B"
      }
    ]
  }
}
```

## Config Parameter Coverage

*   network-instances/network-instance/name
*   network-instances/network-instance/policy-forwarding/policies/policy/policy-id
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/sequence-id
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/protocol
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/dscp-set
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/source-address
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/protocol
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/dscp-set
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/source-address
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/decap-network-instance
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/post-network-instance
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/decap-fallback-network-instance

## Telemetry Parameter Coverage

*   network-instances/network-instance/name
*   network-instances/network-instance/policy-forwarding/policies/policy/policy-id
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/sequence-id
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/protocol
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/dscp-set
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/source-address
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/protocol
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/dscp-set
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/source-address
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/decap-network-instance
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/post-network-instance
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/decap-fallback-network-instance
*   network-instances/network-instance/afts/next-hops/next-hop/state/encapsulate-header
*   network-instances/network-instance/afts/next-hops/next-hop/ip-in-ip/state/src-ip
*   network-instances/network-instance/afts/next-hops/next-hop/ip-in-ip/state/dst-ip
*   network-instances/network-instance/afts/next-hops/next-hop/state/counters/packets-forwarded
*   network-instances/network-instance/afts/next-hops/next-hop/state/counters/octets-forwarded
*   network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/counters/packets-forwarded
*   network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/counters/octets-forwarded
*   network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/counters/packets-forwarded
*   network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/counters/octets-forwarded
*   interfaces/interface/state/counters/in-pkts
*   interfaces/interface/state/counters/in-octets
*   interfaces/interface/state/counters/out-pkts
*   interfaces/interface/state/counters/out-octets
*   interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-pkts
*   interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-forwarded-octets
*   interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-pkts
*   interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-forwarded-octets
*   interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-pkts
*   interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-forwarded-octets
*   interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-pkts
*   interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-forwarded-octets

## Protocol/RPC Parameter Coverage

*   gRIBI:
    *   Modify
        *   ModifyRequest
    *   Flush
    *   Get
*   gNMI:
    *   Set
    *   Get
    *   Subscribe (STREAM mode: SAMPLE and ON_CHANGE)

## Required DUT platform

vRX

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml
paths:
  ## Config paths
  /network-instances/network-instance/policy-forwarding/policies/policy/config/policy-id:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/config/sequence-id:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/protocol:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/dscp-set:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/source-address:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/protocol:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/dscp-set:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/source-address:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decap-network-instance:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/post-decap-network-instance:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decap-fallback-network-instance:

  ## State paths
  /interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/state/link-layer-address:
  /interfaces/interface/state/counters/in-pkts:
  /interfaces/interface/state/counters/in-octets:
  /interfaces/interface/state/counters/out-pkts:
  /interfaces/interface/state/counters/out-octets:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-forwarded-octets:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-forwarded-octets:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-forwarded-octets:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-forwarded-octets:
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/encapsulate-header:
  /network-instances/network-instance/afts/next-hops/next-hop/state/decapsulate-header:
  /network-instances/network-instance/afts/next-hops/next-hop/ip-in-ip/state/src-ip:
  /network-instances/network-instance/afts/next-hops/next-hop/ip-in-ip/state/dst-ip:
  /network-instances/network-instance/afts/next-hops/next-hop/state/counters/packets-forwarded:
  /network-instances/network-instance/afts/next-hops/next-hop/state/counters/octets-forwarded:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/counters/packets-forwarded:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/counters/octets-forwarded:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/counters/packets-forwarded:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/counters/octets-forwarded:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Get:
    gNMI.Subscribe:
      on_change: true
      sample: true
  gribi:
    gRIBI.Modify:
    gRIBI.Flush:
    gRIBI.Get:
```
