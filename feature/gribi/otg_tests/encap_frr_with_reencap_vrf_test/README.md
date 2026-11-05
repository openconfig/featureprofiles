# TE-16.3: encapsulation FRR scenarios

## Summary

Test FRR behaviors with encapsulation scenarios.

## Topology

-   ATE port-1 <------> port-1 DUT
-   DUT port-2 <------> port-2 ATE
-   DUT port-3 <------> port-3 ATE
-   DUT port-4 <------> port-4 ATE
-   DUT port-5 <------> port-5 ATE
-   DUT port-6 <------> port-6 ATE
-   DUT port-7 <------> port-7 ATE
-   DUT port-8 <------> port-8 ATE

## Baseline setup

*   Apply the following vrf selection policy to DUT port-1

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
IPv4Entry {0.0.0.0/0 (ENCAP_TE_VRF_A)} -> NHG#1003 (DEFAULT VRF) -> {
  {NH#1003, DEFAULT VRF},
} // fallback route to redirect to DEFAULT VRF
NHG#1003 (DEFAULT VRF) {
  {NH#1003, DEFAULT VRF}
}
NH#1003 -> {
  network_instance: "DEFAULT"
}

IPv4Entry {138.0.11.0/24 (ENCAP_TE_VRF_A)} -> NHG#101 (DEFAULT VRF) -> {
  {NH#101, DEFAULT VRF, weight:1},
  {NH#102, DEFAULT VRF, weight:3},
  backup_next_hop_group: 2001 // fallback to DEFAULT VRF
}

NHG#2001 (DEFAULT VRF) {
  {NH#2001, DEFAULT VRF}
}
NH#2001 -> {
  network_instance: "DEFAULT"
}

NH#101 -> {
  encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
  ip_in_ip {
    dst_ip: "203.0.113.1"
    src_ip: "ipv4_outer_src_111"
  }
  network_instance: "TE_VRF_111"
}
NH#102 -> {
  encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
  ip_in_ip {
    dst_ip: "203.0.113.2"
    src_ip: "ipv4_outer_src_111"
  }
  network_instance: "TE_VRF_111"
}


IPv4Entry {203.0.113.1/32 (TE_VRF_111)} -> NHG#1 (DEFAULT VRF) -> {
  {NH#1, DEFAULT VRF, weight:1,ip_address=192.0.2.101},
  {NH#2, DEFAULT VRF, weight:3,ip_address=192.0.2.102},
  backup_next_hop_group: 3000 // Go to REPAIR VRF
}
IPv4Entry {192.0.2.101/32 (DEFAULT VRF)} -> NHG#11 (DEFAULT VRF) -> {
  {NH#11, DEFAULT VRF, weight:1,mac_address:magic_mac, interface-ref:dut-port-2-interface},
  {NH#12, DEFAULT VRF, weight:3,mac_address:magic_mac, interface-ref:dut-port-3-interface},
}
IPv4Entry {192.0.2.102/32 (DEFAUlT VRF)} -> NHG#12 (DEFAULT VRF) -> {
  {NH#13, DEFAULT VRF, weight:2,mac_address:magic_mac, interface-ref:dut-port-4-interface},
}

NHG#3000 (DEFAULT VRF) {
  {NH#3000, DEFAULT VRF}
}
NH#3000 -> {
  network_instance: "REPAIR"
}

IPv4Entry {203.0.113.1/32 (REPAIR)} -> NHG#1000 (DEFAULT VRF) -> {
  {NH#1000, DEFAULT VRF}
  backup_next_hop_group: 2000 // decap and fallback to DEFAULT VRF
}
NH#1000 -> {
  decapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
  encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
  ip_in_ip {
    dst_ip: "203.0.113.100"
    src_ip: "ipv4_outer_src_222"
  }
  network_instance: "TE_VRF_222"
}

IPv4Entry {203.0.113.100/32 (TE_VRF_222)} -> NHG#2 (DEFAULT VRF) -> {
  {NH#3, DEFAULT VRF, weight:1,ip_address=192.0.2.103},
  backup_next_hop_group: 2000 // decap and fallback to DEFAULT VRF
}
IPv4Entry {192.0.2.103/32 (DEFAULT VRF)} -> NHG#13 (DEFAULT VRF) -> {
  {NH#14, DEFAULT VRF, weight:1,mac_address:magic_mac, interface-ref:dut-port-5-interface},
}

// 203.0.113.2 is the tunnel IP address. Note that the NHG#3 is different than NHG#1.

IPv4Entry {203.0.113.2/32 (TE_VRF_111)} -> NHG#3 (DEFAULT VRF) -> {
  {NH#4, DEFAULT VRF, weight:1,ip_address=192.0.2.104},
  backup_next_hop_group: 3000 // Go to REPAIR VRF
}

IPv4Entry {192.0.2.104/32 (DEFAULT VRF)} -> NHG#14 (DEFAULT VRF) -> {
  {NH#15, DEFAULT VRF, weight:1,mac_address:magic_mac, interface-ref:dut-port-6-interface},
}

IPv4Entry {203.0.113.2/32 (REPAIR)} -> NHG#1001 (DEFAULT VRF) -> {
  {NH#1001, DEFAULT VRF}
  backup_next_hop_group: 2000 // decap and fallback to DEFAULT VRF
}
NHG#1001 (DEFAULT VRF) {
  {NH#1001, DEFAULT VRF},
  {NH#1003, DEFAULT VRF}
}
NH#1001 -> {
  decapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
  encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
  ip_in_ip {
    dst_ip: "203.0.113.101"
    src_ip: "ipv4_outer_src_222"
  }
  network_instance: "TE_VRF_222"
}
NH#1003 -> {
  decapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
  encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
  ip_in_ip {
    dst_ip: "203.0.113.102"
    src_ip: "ipv4_outer_src_222"
  }
  network_instance: "TE_VRF_222"
}

IPv4Entry {203.0.113.101/32 (TE_VRF_222)} -> NHG#4 (DEFAULT VRF) -> {
  {NH#5, DEFAULT VRF, weight:1,ip_address=192.0.2.105},
  backup_next_hop_group: 2000 // decap and fallback to DEFAULT VRF
}
IPv4Entry {203.0.113.102/32 (TE_VRF_222)} -> NHG#4 (DEFAULT VRF) -> {
  {NH#5, DEFAULT VRF, weight:1,ip_address=192.0.2.105},
  backup_next_hop_group: 2000 // decap and fallback to DEFAULT VRF
}
IPv4Entry {192.0.2.105/32 (DEFAULT VRF)} -> NHG#15 (DEFAULT VRF) -> {
  {NH#16, DEFAULT VRF, weight:1,mac_address:magic_mac, interface-ref:dut-port-7-interface},
}

NHG#2000 (DEFAULT VRF) {
  {NH#2000, DEFAULT VRF}
}
NH#2000 -> {
  decapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
  network_instance: "DEFAULT"
}
```

*   Install a BGP route resolved by ISIS in default VRF to route 138.0.11.8
    traffic out of DUT port-8.

## Procedure

At the start of each of the following scenarios, ensure:

*   All ports are up and baseline is reset as above.
*   Send packets to DUT port-1. The outer v4 header has the destination
    addresses 138.0.11.8.
*   Validate that traffic is encapsulated to 203.0.113.1 and 203.0.113.2, and is
    distributed per the hierarchical weights.

Unless otherwise specified, all the tests below should use traffic with
`dscp_encap_a_1`.

#### Baseline AFT telemetry validation

Before running the scenarios below, install the baseline gRIBI AFTs and
validate via gNMI that:

1.  The IPv4Entry `138.0.11.0/24` in `ENCAP_TE_VRF_A` is present and points
    to a next-hop-group.
2.  Every next-hop in that next-hop-group (NH#101 and NH#102) reports
    `next-hop/state/encapsulate-header` as `IPV4`.
3.  Every next-hop in that next-hop-group reports
    `next-hop/state/origin-protocol` as `GRIBI`.

#### Test-1, primary encap unviable but backup encap viable for single tunnel

Tests that if the primary NHG for an encap tunnel is unviable, then the traffic
for that tunnel is re-encaped into its specified backup tunnel.

1.  Shutdown DUT port-2, port-3, and port-4.
2.  Validate that corresponding traffic that was encapped to 203.0.113.1 should
    now be encapped with 203.0.113.100.

#### Test-2, primary and backup encap unviable for single tunnel

Tests that if the primary NHGs of both the encap tunnel and its backup tunnel
are unviable, then the traffic for that tunnel is not encapped. Instead, that
fraction of traffic should be forwarded according to the BGP/IS-IS routes in the
DEFAULT VRF.

1.  Shutdown DUT port-2, port-3, port-4 and port-5.
2.  Validate that corresponding traffic (25% of the total traffic) that was
    encapped to 203.0.113.1 are no longer encapped, and forwarded per BGP-ISIS
    routes (in the default VRF) out of DUT port-8.

#### Test-3, primary encap unviable with backup to routing for single tunnel

Tests that if the primary NHGs of both the encap tunnel is unviable, and its
backup specifies fallback to routing, then the traffic for that tunnel is not
encapped. Instead, that fraction of traffic should be forwarded according to the
BGP/IS-IS routes in the DEFAULT VRF.

1.  Update `NHG#1000` to the following:

```
NHG#1000 (Default VRF) {
        {NH#1000, DEFAULT VRF}
}
NH#1000 -> {
  decapsulate_header: OPENCONFIGAFTTYPESDECAPSULATIONHEADERTYPE_IPV4
  network_instance: "DEFAULT"
}
```

1.  Validate that all traffic is distributed per the hierarchical weights.
2.  Shutdown DUT port-2, port-3, and port-4.
3.  Validate that corresponding traffic (25% of the total traffic) that was
    encapped to 203.0.113.1 are no longer encapped, and forwarded per BGP-ISIS
    routes (in the default VRF) out of DUT port-8.

#### Test-4, primary encap unviable but backup encap viable for all tunnels

Tests that if the primary NHG for all encap tunnels are unviable, then the
traffic is re-encaped into the specified backup tunnels. This test ensures that
the device does not withdraw this IPv4Entry and sends this traffic to routing.

1.  Shutdown DUT port-2, port-3, port-4 and port-6.
2.  Validate that traffic is encapsulated to 203.0.113.100, 203.0.113.101, and
    203.0.113.102 per the weights, with 203.0.113.101 and 203.0.113.102 both
    valid for the TE_VRF_222 backup path.

#### Test-5, primary and backup encap unviable for all tunnels

Tests that if the primary NHGs of both the encap tunnel and its backup tunnel
are unviable for all tunnels in the encap NHG, then the traffic for that cluster
prefix is not encapped. Instead, that traffic should be forwarded according to
the BGP/IS-IS routes in the DEFAULT VRF. This stresses the double failure
handling, and ensures that the fallback to DEFAULT is activated through the
backup NHGs of the tunnels instead of withdrawing the IPv4Entry.

1.  Shutdown DUT port-2, port-3, port-4, port-5, port-6 and port-7.
2.  Validate that all traffic is no longer encapsulated, and is all egressing
    out of DUT port-8 per the BGP-ISIS routes in the default VRF.

#### Test-6, primary encap unviable with backup to routing for all tunnels

Tests that if the primary NHGs of both the encap tunnel is unviable, and its
backup specifies fallback to routing, for all tunnels in the encap NHG, then the
traffic for that cluster prefix is not encapped. Instead, that traffic should be
forwarded according to the BGP/IS-IS routes in the DEFAULT VRF. This stresses
the double failure handling, and ensures that the fallback to DEFAULT is
activated through the backup NHGs of the tunnels instead of withdrawing the
IPv4Entry.

1.  Update `NHG#1000` and `NHG#1001` to the following:

```
NHG#1000 (Default VRF) { {NH#1000, DEFAULT VRF} }

NH#1000 -> {
  decapsulate_header: OPENCONFIGAFTTYPESDECAPSULATIONHEADERTYPE_IPV4
  network_instance: "DEFAULT"
}

NHG#1001 (Default VRF) { {NH#1001, DEFAULT VRF} }

NH#1001 -> {
  decapsulate_header: OPENCONFIGAFTTYPESDECAPSULATIONHEADERTYPE_IPV4
  network_instance: "DEFAULT"
}
```

1.  Validate that all traffic is distributed per the hierarchical weights.
2.  Shutdown DUT port-2, port-3, and port-4, and port-6.
3.  Validate that all traffic is no longer encapsulated, and is all egressing
    out of DUT port-8 per the BGP-ISIS routes in the default VRF.

#### Test-7, no match in encap VRF

Test that if there is no lookup match in the encap VRF, then the traffic should
be routed to the DEFAULT VRF for further lookup.

1.  In `ENCAP_TE_VRF_A`, Add an 0/0 static route pointing to the DEFAULT VRF
    using gNMI.
2.  Send traffic with destination address 20.0.0.1, which should produce no
    match in `ENCAP_TE_VRF_A`.
3.  Validate that the traffic is routed per the BGP-ISIS routes (in the DEFAULT
    VR) out of DUT port-8.

#### Test-8, no match in TE_VRF_222

Tests that if the re-encaps point to tunnels that do not exist, then the traffic
should be routed to the `DEFAULT` VRF for further lookup.

1.  Update `NHG#1000` and `NHG#1001` to the following:

```
NHG#1000 (Default VRF) { {NH#1000, DEFAULT VRF} }

NH#1000 -> {
  decapsulate_header: OPENCONFIGAFTTYPESDECAPSULATIONHEADERTYPE_IPV4
  encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
  ip_in_ip {
    dst_ip: "203.100.113.100" // This route does not exist in TE_VRF_222
    src_ip: "ipv4_outer_src_222"
  }
  network_instance: "TE_VRF_222"
}

NHG#1001 (Default VRF) { {NH#1001, DEFAULT VRF} }

NH#1001 -> {
  decapsulate_header: OPENCONFIGAFTTYPESDECAPSULATIONHEADERTYPE_IPV4
  encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
  ip_in_ip {
    dst_ip: "203.100.113.101" // This route does not exist in TE_VRF_222
    src_ip: "ipv4_outer_src_222"
  }
  network_instance: "TE_VRF_222"
}
```

1.  Validate that all traffic is distributed per the hierarchical weights.
2.  Shutdown DUT port-2, port-3, and port-4, and port-6.
3.  Validate that all traffic is no longer encapsulated, and is all egressing
    out of DUT port-8 per the BGP-ISIS routes in the default VRF.

#### Test-9, no match in TE_VRF_111

Tests that if the primary encaps point to tunnels that do not exist, then the
traffic should be routed to the `DEFAULT` VRF for further lookup.

1.  Validate that all traffic is distributed per the hierarchical weights.
2.  Update NHG#101 to the following:

```
NHG#101 (DEFAULT VRF) -> {
  {NH#101, DEFAULT VRF, weight:1},
  {NH#102, DEFAULT VRF, weight:3},
  backup_next_hop_group: 2001 // fallback to DEFAULT VRF
}

NH#101 -> {
  encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
  ip_in_ip {
    dst_ip: "203.100.113.1" // This route does not exist in TE_VRF_111
    src_ip: "ipv4_outer_src_111"
  }
  network_instance: "TE_VRF_111"
}
NH#102 -> {
  encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
  ip_in_ip {
    dst_ip: "203.100.113.2" // This route does not exist in TE_VRF_111
    src_ip: "ipv4_outer_src_111"
  }
  network_instance: "TE_VRF_111"
}
```

1.  Validate that all traffic is no longer encapsulated, and is all egressing
    out of DUT port-8 per the BGP-ISIS routes in the default VRF.

#### Test-10, multi-VRF (ENCAP_TE_VRF_A and ENCAP_TE_VRF_B) dual-stack encap path convergence into REPAIR VRF on primary tunnel edge cut

Tests that when an edge cut occurs on the primary tunnel egress interfaces while
dual-stack (IPv4 and IPv6) traffic is actively flowing across multiple
encapsulation VRFs (`ENCAP_TE_VRF_A` representing low-priority encapsulation
flows matched by `dscp_encap_a_1 = 10` / `dscp_encap_a_2 = 18`, and
`ENCAP_TE_VRF_B` representing low-latency encapsulation flows matched by
`dscp_encap_b_1 = 20` / `dscp_encap_b_2 = 28`), all affected flows immediately
converge onto the `REPAIR` VRF backup tunnels in `TE_VRF_222` without sustained
traffic blackholing or ingress/egress ACL/VOQ packet drops.

1.  Ensure all 8 DUT ports (`port-1` through `port-8`) are operationally `UP`
    (`/interfaces/interface/state/oper-status` = `UP`) and the baseline
    interfaces (`192.0.2.1/30` and `2001:db8::192:0:2:1/126` on DUT `port-1`
    through `192.0.2.29/30` and `2001:db8::192:0:2:1d/126` on DUT `port-8`),
    `vrf_selection_policy_c` on DUT `port-1`, and baseline gRIBI AFT entries are
    configured.
2.  Configure dual-stack BGP (ASN `65501`, DUT loopback `203.0.113.11/32` and
    `2001:db8::203:0:113:1/128`, ATE `port-8` loopback `203.0.113.10/32` and
    `2001:db8::203:0:113:10/128`) resolved by IS-IS in the `DEFAULT` VRF on DUT
    `port-8` so that both IPv4 destination `138.0.11.8/32` and IPv6 destination
    `2001:db8::138:0:11:8/128` are reachable out of DUT `port-8` in the
    `DEFAULT` VRF.
3.  Using gRIBI (`gRIBI.Modify`), in addition to the baseline `ENCAP_TE_VRF_A`,
    `TE_VRF_111`, `REPAIR`, `TE_VRF_222`, and `DEFAULT` VRF entries, install the
    IPv6 entries in `ENCAP_TE_VRF_A` and both IPv4 and IPv6 entries in
    `ENCAP_TE_VRF_B` with inverted tunnel weights (`3:1`) to distinguish
    `ENCAP_TE_VRF_B` forwarding from `ENCAP_TE_VRF_A` (`1:3`):

```
IPv6Entry {::/0 (ENCAP_TE_VRF_A)} -> NHG#1003 (DEFAULT VRF) -> {
  {NH#1003, DEFAULT VRF},
} // fallback route to redirect to DEFAULT VRF

IPv6Entry {2001:db8::138:0:11:0/126 (ENCAP_TE_VRF_A)} -> NHG#101 (DEFAULT VRF) -> {
  {NH#101, DEFAULT VRF, weight:1},
  {NH#102, DEFAULT VRF, weight:3},
  backup_next_hop_group: 2001 // fallback to DEFAULT VRF
}

IPv4Entry {0.0.0.0/0 (ENCAP_TE_VRF_B)} -> NHG#1003 (DEFAULT VRF) -> {
  {NH#1003, DEFAULT VRF},
} // fallback route to redirect to DEFAULT VRF

IPv6Entry {::/0 (ENCAP_TE_VRF_B)} -> NHG#1003 (DEFAULT VRF) -> {
  {NH#1003, DEFAULT VRF},
} // fallback route to redirect to DEFAULT VRF

IPv4Entry {138.0.11.0/24 (ENCAP_TE_VRF_B)} -> NHG#102 (DEFAULT VRF) -> {
  {NH#101, DEFAULT VRF, weight:3},
  {NH#102, DEFAULT VRF, weight:1},
  backup_next_hop_group: 2001 // fallback to DEFAULT VRF
}

IPv6Entry {2001:db8::138:0:11:0/126 (ENCAP_TE_VRF_B)} -> NHG#102 (DEFAULT VRF) -> {
  {NH#101, DEFAULT VRF, weight:3},
  {NH#102, DEFAULT VRF, weight:1},
  backup_next_hop_group: 2001 // fallback to DEFAULT VRF
}
```

4.  Validate via gNMI AFT state telemetry that all programmed IPv4 and IPv6
    entries in `ENCAP_TE_VRF_A` and `ENCAP_TE_VRF_B` are installed in the FIB
    (`/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group`
    and
    `/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group`),
    and that `NHG#1` and `NHG#3` report
    `/network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/backup-next-hop-group`
    as `3000` (`REPAIR` VRF).
5.  Start continuous traffic flows on ATE `port-1` into DUT `port-1`:
    *   **Flow 1 (IPv4 `ENCAP_TE_VRF_A` - LowPriority)**: Unencapsulated IPv4
        packets with source `198.51.100.1`–`198.51.100.250`, destination
        `138.0.11.8`, and DSCP `10` (`dscp_encap_a_1`) and `18`
        (`dscp_encap_a_2`).
    *   **Flow 2 (IPv6 `ENCAP_TE_VRF_A` - LowPriority)**: Unencapsulated IPv6
        packets with source `2001:db8::198:51:100:1`–`2001:db8::198:51:100:fa`,
        destination `2001:db8::138:0:11:8`, and DSCP `10` (`dscp_encap_a_1`) and
        `18` (`dscp_encap_a_2`).
    *   **Flow 3 (IPv4 `ENCAP_TE_VRF_B` - LowLatency)**: Unencapsulated IPv4
        packets with source `198.51.100.1`–`198.51.100.250`, destination
        `138.0.11.8`, and DSCP `20` (`dscp_encap_b_1`) and `28`
        (`dscp_encap_b_2`).
    *   **Flow 4 (IPv6 `ENCAP_TE_VRF_B` - LowLatency)**: Unencapsulated IPv6
        packets with source `2001:db8::198:51:100:1`–`2001:db8::198:51:100:fa`,
        destination `2001:db8::138:0:11:8`, and DSCP `20` (`dscp_encap_b_1`) and
        `28` (`dscp_encap_b_2`).
6.  Validate in steady state that all 4 flows are encapsulated with outer IPv4
    source `198.51.100.111` (`ipv4_outer_src_111`) and outer IPv4 destinations
    `203.0.113.1` and `203.0.113.2` (4in4 with IP protocol `4` and 6in4 with IP
    protocol `41`), and egress on DUT `port-2`, `port-3`, `port-4`, and `port-6`
    according to their respective hierarchical weights (`1:3` between
    `203.0.113.1` and `203.0.113.2` for `ENCAP_TE_VRF_A`, and `3:1` for
    `ENCAP_TE_VRF_B`) with 0% packet loss.
7.  While all 4 flows are actively transmitting, shutdown DUT `port-2`, `port-3`,
    and `port-4` (`/interfaces/interface/config/enabled` set to `false`) to
    simulate an edge cut on primary tunnel `203.0.113.1/32` (`NHG#1`) in
    `TE_VRF_111`.
8.  Verify via gNMI (`/interfaces/interface/state/oper-status`) that DUT
    `port-2`, `port-3`, and `port-4` transition to `DOWN`, and validate that:
    *   The fraction of traffic in both `ENCAP_TE_VRF_A` (25% of Flows 1 and 2)
        and `ENCAP_TE_VRF_B` (75% of Flows 3 and 4) targeting tunnel
        `203.0.113.1` immediately converges via `NHG#3000` into the `REPAIR` VRF
        (`NHG#1000`), where the primary outer IPv4 header is decapsulated and
        re-encapsulated with outer source `198.51.100.222` (`ipv4_outer_src_222`)
        and outer destination `203.0.113.100`, egressing out of DUT `port-5`.
    *   Total packet loss across each of the 4 flows during the link cut and
        convergence into the `REPAIR` VRF is within standard FNT tolerance
        (`<= 2%`), with zero sustained traffic drop after convergence (no
        prolonged multi-minute blackhole) and no continuous drop increments on
        `/interfaces/interface/state/counters/in-discards`,
        `/interfaces/interface/state/counters/out-discards`, or
        `/qos/interfaces/interface/output/queues/queue/state/dropped-pkts`.
9.  While all 4 flows continue transmitting, shutdown DUT `port-6`
    (`/interfaces/interface/config/enabled` set to `false`) so that all primary
    tunnels in `TE_VRF_111` (`203.0.113.1/32` and `203.0.113.2/32`) are
    unviable.
10. Validate that 100% of all 4 flows (`ENCAP_TE_VRF_A` and `ENCAP_TE_VRF_B`,
    IPv4 and IPv6) converge onto the `REPAIR` VRF backup tunnels (`203.0.113.100`
    egressing on DUT `port-5`, and `203.0.113.101`/`203.0.113.102` egressing on
    DUT `port-7`) per the hierarchical weights (`25%` on `port-5` and `75%` on
    `port-7` for `ENCAP_TE_VRF_A`; `75%` on `port-5` and `25%` on `port-7` for
    `ENCAP_TE_VRF_B`), with `<= 2%` packet loss and zero sustained drops after
    convergence.
11. Re-enable DUT `port-2`, `port-3`, `port-4`, and `port-6`
    (`/interfaces/interface/config/enabled` set to `true`), verify
    `/interfaces/interface/state/oper-status` is `UP`, and confirm all 4 flows
    restore to the primary `TE_VRF_111` tunnels on DUT `port-2`, `port-3`,
    `port-4`, and `port-6` with 0% steady-state packet loss.
12. **Cleanup**: Stop all OTG traffic flows, ensure all DUT interfaces
    (`port-1` through `port-8`) are enabled
    (`/interfaces/interface/config/enabled` set to `true`) and operationally
    `UP`, revert the gRIBI entries added during `Test-10` via `gRIBI.Flush`, and
    restore the baseline gRIBI AFT entries and policy-forwarding configuration
    (`vrf_selection_policy_c` on DUT `port-1`) so the DUT is returned to its
    baseline operational state.

##### Canonical OC
```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "DUT port-1",
          "enabled": true,
          "name": "port1",
          "type": "ethernetCsmacd"
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
      }
    ]
  },
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "DECAP_TE_VRF",
          "type": "L3VRF"
        },
        "name": "DECAP_TE_VRF"
      },
      {
        "config": {
          "name": "ENCAP_TE_VRF_A",
          "type": "L3VRF"
        },
        "name": "ENCAP_TE_VRF_A"
      },
      {
        "config": {
          "name": "ENCAP_TE_VRF_B",
          "type": "L3VRF"
        },
        "name": "ENCAP_TE_VRF_B"
      },
      {
        "config": {
          "name": "REPAIR",
          "type": "L3VRF"
        },
        "name": "REPAIR"
      },
      {
        "config": {
          "name": "TE_VRF_111",
          "type": "L3VRF"
        },
        "name": "TE_VRF_111"
      },
      {
        "config": {
          "name": "TE_VRF_222",
          "type": "L3VRF"
        },
        "name": "TE_VRF_222"
      },
      {
        "config": {
          "name": "DEFAULT",
          "type": "DEFAULT_INSTANCE"
        },
        "name": "DEFAULT",
        "policy-forwarding": {
          "interfaces": {
            "interface": [
              {
                "config": {
                  "apply-vrf-selection-policy": "vrf_selection_policy_c",
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
                          "decap-fallback-network-instance": "TE_VRF_111",
                          "decap-network-instance": "DECAP_TE_VRF",
                          "post-decap-network-instance": "ENCAP_TE_VRF_A"
                        }
                      },
                      "config": {
                        "sequence-id": 4
                      },
                      "ipv4": {
                        "config": {
                          "dscp-set": [
                            10,
                            18
                          ],
                          "protocol": 41,
                          "source-address": "198.51.100.111/32"
                        }
                      },
                      "sequence-id": 4
                    },
                    {
                      "action": {
                        "config": {
                          "decap-fallback-network-instance": "TE_VRF_111",
                          "decap-network-instance": "DECAP_TE_VRF",
                          "post-decap-network-instance": "ENCAP_TE_VRF_B"
                        }
                      },
                      "config": {
                        "sequence-id": 7
                      },
                      "ipv4": {
                        "config": {
                          "dscp-set": [
                            20,
                            28
                          ],
                          "protocol": 4,
                          "source-address": "198.51.100.111/32"
                        }
                      },
                      "sequence-id": 7
                    },
                    {
                      "action": {
                        "config": {
                          "decap-fallback-network-instance": "TE_VRF_111",
                          "decap-network-instance": "DECAP_TE_VRF",
                          "post-decap-network-instance": "ENCAP_TE_VRF_B"
                        }
                      },
                      "config": {
                        "sequence-id": 8
                      },
                      "ipv4": {
                        "config": {
                          "dscp-set": [
                            20,
                            28
                          ],
                          "protocol": 41,
                          "source-address": "198.51.100.111/32"
                        }
                      },
                      "sequence-id": 8
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
                          "network-instance": "ENCAP_TE_VRF_B"
                        }
                      },
                      "config": {
                        "sequence-id": 15
                      },
                      "ipv4": {
                        "config": {
                          "dscp-set": [
                            20,
                            28
                          ]
                        }
                      },
                      "sequence-id": 15
                    },
                    {
                      "action": {
                        "config": {
                          "network-instance": "ENCAP_TE_VRF_B"
                        }
                      },
                      "config": {
                        "sequence-id": 16
                      },
                      "ipv6": {
                        "config": {
                          "dscp-set": [
                            20,
                            28
                          ]
                        }
                      },
                      "sequence-id": 16
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

#### Test-11, transit IP-in-IP encapsulated (4in4 and 6in4) traffic convergence and re-encapsulation via REPAIR VRF on primary tunnel edge cut

Tests that transit IP-in-IP encapsulated packets (both 4in4 with IP protocol `4`
and 6in4 with IP protocol `41`) arriving on DUT `port-1` with outer source IP
`198.51.100.111` (`ipv4_outer_src_111`), which miss `DECAP_TE_VRF` and fall back
to `TE_VRF_111` via `decap-fallback-network-instance` in
`vrf_selection_policy_c`, immediately converge onto the `REPAIR` VRF and are
re-encapsulated into `TE_VRF_222` when the primary transit tunnel egress ports
are shut down, without experiencing sustained traffic loss or ACL/VOQ drops.

1.  Reset the DUT and ATE to the baseline setup with all 8 ports (`port-1`
    through `port-8`) operationally `UP`
    (`/interfaces/interface/state/oper-status` = `UP`) and baseline gRIBI AFT
    entries installed in `ENCAP_TE_VRF_A`, `ENCAP_TE_VRF_B`, `TE_VRF_111`,
    `REPAIR`, `TE_VRF_222`, and `DEFAULT`.
2.  Verify via gNMI (`gNMI.Get`) on
    `/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/state/decap-fallback-network-instance`,
    `/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/state/source-address`,
    and
    `/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/state/dscp-set`
    that `vrf_selection_policy_c` rules 3, 4, 7, 8, 11, and 12 match outer IPv4
    source `198.51.100.111/32` with IP protocols `4` and `41` and direct packets
    missing `DECAP_TE_VRF` to `decap-fallback-network-instance: "TE_VRF_111"`.
3.  Start continuous pre-encapsulated IP-in-IP transit traffic flows on ATE
    `port-1` into DUT `port-1`:
    *   **Transit Flow 1 (4in4 LowPriority - DSCP 10)**: Outer IPv4 header
        (`src: 198.51.100.111`, `dst: 203.0.113.1` and `203.0.113.2`,
        `protocol: 4`, `dscp: 10`), Inner IPv4 header
        (`src: 198.51.100.1`–`198.51.100.250`, `dst: 138.0.11.8`, `dscp: 10`).
    *   **Transit Flow 2 (6in4 LowPriority - DSCP 10)**: Outer IPv4 header
        (`src: 198.51.100.111`, `dst: 203.0.113.1` and `203.0.113.2`,
        `protocol: 41`, `dscp: 10`), Inner IPv6 header
        (`src: 2001:db8::198:51:100:1`–`2001:db8::198:51:100:fa`,
        `dst: 2001:db8::138:0:11:8`, `dscp: 10`).
    *   **Transit Flow 3 (4in4 LowLatency - DSCP 20)**: Outer IPv4 header
        (`src: 198.51.100.111`, `dst: 203.0.113.1` and `203.0.113.2`,
        `protocol: 4`, `dscp: 20`), Inner IPv4 header
        (`src: 198.51.100.1`–`198.51.100.250`, `dst: 138.0.11.8`, `dscp: 20`).
    *   **Transit Flow 4 (6in4 LowLatency - DSCP 20)**: Outer IPv4 header
        (`src: 198.51.100.111`, `dst: 203.0.113.1` and `203.0.113.2`,
        `protocol: 41`, `dscp: 20`), Inner IPv6 header
        (`src: 2001:db8::198:51:100:1`–`2001:db8::198:51:100:fa`,
        `dst: 2001:db8::138:0:11:8`, `dscp: 20`).
4.  Validate in steady state that all 4 transit flows match `TE_VRF_111` via
    `decap-fallback-network-instance` and egress on DUT `port-2`, `port-3`, and
    `port-4` (for outer destination `203.0.113.1`) and DUT `port-6` (for outer
    destination `203.0.113.2`) per the programmed next-hop weights with outer
    source `198.51.100.111` preserved and 0% packet loss.
5.  While all 4 transit flows are actively transmitting, shutdown DUT `port-2`,
    `port-3`, `port-4`, and `port-6` (`/interfaces/interface/config/enabled` set
    to `false`) to simulate a simultaneous transit link cut across the primary
    `TE_VRF_111` tunnels.
6.  Verify via gNMI (`/interfaces/interface/state/oper-status`) that DUT
    `port-2`, `port-3`, `port-4`, and `port-6` transition to `DOWN`, and
    validate that:
    *   All 4in4 and 6in4 transit flows (across both DSCP `10` and DSCP `20`)
        immediately fail over via `NHG#3000` into the `REPAIR` VRF, where
        `NHG#1000` (for `203.0.113.1/32`) and `NHG#1001` (for `203.0.113.2/32`)
        decapsulate the incoming `TE_VRF_111` outer IPv4 header
        (`/network-instances/network-instance/afts/next-hops/next-hop/state/decapsulate-header` = `IPV4`)
        and re-encapsulate the packets
        (`/network-instances/network-instance/afts/next-hops/next-hop/state/encapsulate-header` = `IPV4`)
        with outer source IP `198.51.100.222` (`ipv4_outer_src_222`) and outer
        destination IP `203.0.113.100` (egressing DUT `port-5`) or
        `203.0.113.101`/`203.0.113.102` (egressing DUT `port-7`) in
        `TE_VRF_222`.
    *   The inner IPv4 (`138.0.11.8`) and IPv6 (`2001:db8::138:0:11:8`) payloads
        and their DSCP markings (`10` and `20`) are preserved intact.
    *   End-to-end packet loss across each transit flow is `<= 2%`, with zero
        sustained traffic blackhole (no ~2-minute traffic drop) and no
        continuous discard increments on
        `/interfaces/interface/state/counters/in-discards`,
        `/interfaces/interface/state/counters/out-discards`, or
        `/qos/interfaces/interface/output/queues/queue/state/dropped-pkts`.
7.  While the re-encapsulated transit flows are actively egressing DUT `port-5`
    and `port-7`, shutdown DUT `port-5` and `port-7`
    (`/interfaces/interface/config/enabled` set to `false`) so that the backup
    tunnels in `TE_VRF_222` also become unviable and trigger
    `backup_next_hop_group: 2000` (`NH#2000` decapsulate to `DEFAULT` VRF).
8.  Validate that 100% of the 4in4 and 6in4 flows have their outer IPv4 header
    stripped and are forwarded as native IPv4 (`138.0.11.8`) and IPv6
    (`2001:db8::138:0:11:8`) packets out of DUT `port-8` via the BGP/IS-IS
    routes in the `DEFAULT` VRF with `<= 2%` transition loss.
9.  Re-enable DUT `port-2`, `port-3`, `port-4`, `port-5`, `port-6`, and `port-7`
    (`/interfaces/interface/config/enabled` set to `true`), verify
    `/interfaces/interface/state/oper-status` is `UP` on all ports, and confirm
    all transit flows revert to the primary `TE_VRF_111` tunnels on DUT
    `port-2`, `port-3`, `port-4`, and `port-6` with 0% steady-state loss.
10. **Cleanup**: Stop all OTG transit traffic flows, ensure all DUT interfaces
    (`port-1` through `port-8`) are enabled
    (`/interfaces/interface/config/enabled` set to `true`) and operationally
    `UP`, revert the gRIBI entries modified during `Test-11` via `gRIBI.Flush`,
    and restore the baseline gRIBI AFT entries and policy-forwarding
    configuration (`vrf_selection_policy_c` on DUT `port-1`) so the DUT is
    returned to its baseline operational state.

  #### Canonical OC
```json
{}
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

## OpenConfig Path and RPC Coverage
```yaml
paths:
  /interfaces/interface/config/enabled:
  /interfaces/interface/state/counters/in-discards:
  /interfaces/interface/state/counters/out-discards:
  /interfaces/interface/state/oper-status:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/backup-next-hop-group:
  /network-instances/network-instance/afts/next-hops/next-hop/state/decapsulate-header:
  /network-instances/network-instance/afts/next-hops/next-hop/state/encapsulate-header:
  /network-instances/network-instance/afts/next-hops/next-hop/state/origin-protocol:
  /network-instances/network-instance/policy-forwarding/interfaces/interface/config/apply-vrf-selection-policy:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decap-fallback-network-instance:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decap-network-instance:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/network-instance:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/post-decap-network-instance:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/state/decap-fallback-network-instance:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/state/dscp-set:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/state/source-address:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/state/dscp-set:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/adjacency-state:
  /qos/interfaces/interface/output/queues/queue/state/dropped-pkts:
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
  gribi:
    gRIBI.Flush:
    gRIBI.Get:
    gRIBI.Modify:
```

## Required DUT platform

-   vRX

## Canonical OC
```json
{}
```   
