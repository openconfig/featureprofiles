# TE-16.2: encapsulation FRR scenarios

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
IPv4Entry {138.0.11.0/24 (ENCAP_TE_VRF_A)} -> NHG#101 (DEFAULT VRF) -> {
  {NH#101, DEFAULT VRF, weight:1},
  {NH#102, DEFAULT VRF, weight:3},
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
    dst_ip: "203.10.113.2"
    src_ip: "ipv4_outer_src_111"
  }
  network_instance: "TE_VRF_111"
}


IPv4Entry {203.0.113.1/32 (TE_VRF_111)} -> NHG#1 (DEFAULT VRF) -> {
  {NH#1, DEFAULT VRF, weight:1,ip_address=192.0.2.101},
  {NH#2, DEFAULT VRF, weight:3,ip_address=192.0.2.102},
  backup_next_hop_group: 1000 // re-encap to 203.0.113.100
}
IPv4Entry {192.0.2.101/32 (DEFAULT VRF)} -> NHG#11 (DEFAULT VRF) -> {
  {NH#11, DEFAULT VRF, weight:1,mac_address:magic_mac, interface-ref:dut-port-2-interface},
  {NH#12, DEFAULT VRF, weight:3,mac_address:magic_mac, interface-ref:dut-port-3-interface},
}
IPv4Entry {192.0.2.102/32 (DEFAUlT VRF)} -> NHG#12 (DEFAULT VRF) -> {
  {NH#13, DEFAULT VRF, weight:2,mac_address:magic_mac, interface-ref:dut-port-4-interface},
}

NHG#1000 (DEFAULT VRF) {
  {NH#1000, DEFAULT VRF}
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

// 203.10.113.2 is the tunnel IP address. Note that the NHG#3 is different than NHG#1.

IPv4Entry {203.10.113.2/32 (TE_VRF_111)} -> NHG#3 (DEFAULT VRF) -> {
  {NH#4, DEFAULT VRF, weight:1,ip_address=192.0.2.104},
  backup_next_hop_group: 1001 // re-encap to 203.0.113.101
}
IPv4Entry {192.0.2.104/32 (DEFAULT VRF)} -> NHG#14 (DEFAULT VRF) -> {
  {NH#15, DEFAULT VRF, weight:1,mac_address:magic_mac, interface-ref:dut-port-6-interface},
}
NHG#1001 (DEFAULT VRF) {
  {NH#1001, DEFAULT VRF}
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
IPv4Entry {203.0.113.101/32 (TE_VRF_222)} -> NHG#4 (DEFAULT VRF) -> {
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

*   all ports are up and baseline is reset as above.
*   Send packets to DUT port-1. The outer v4 header has the destination
    addresses 138.0.11.8.
*   Validate that traffic is encapsulated to 203.0.113.1 and 203.0.113.2, and is
    distributed per the hierarchical weights.

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
2.  Validate that traffic is encapsulated to 203.0.113.100 and 203.0.113.101 per
    the weights.

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

1.  In `ENCAP_TE_VRF_A`, Add an 0/0 static route pointing to the DEFAULT VRF.
2.  Send traffic with destination address 20.0.0.1, which should produce no
    match in `ENCAP_TE_VRF_A`.
3.  Validate that the traffic is routed per the BGP-ISIS routes (in the DEFAULT
    VR) out of DUT port-8.

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
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
  gribi:
    gRIBI.Get:
    gRIBI.Modify:
    gRIBI.Flush:
```

## Required DUT platform

-   vRX
