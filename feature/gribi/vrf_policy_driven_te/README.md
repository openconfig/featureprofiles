# TE-17.1 VRF selection policy driven TE

## Summary

Test VRF selection logic involving different decapsulation and encapsulation lookup scenarios via gRIBI.

## Topology

ATE port-1 <------> port-1 DUT
DUT port-2 <------> port-2 ATE
DUT port-3 <------> port-3 ATE
DUT port-4 <------> port-4 ATE
DUT port-5 <------> port-5 ATE
DUT port-6 <------> port-6 ATE
DUT port-7 <------> port-7 ATE
DUT port-8 <------> port-8 ATE

## Variables

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

vrf_selection_policy_c
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

vrf_selection_policy_w
```
network-instances {
    network-instance {
        name: DEFAULT
        policy-forwarding {
            policies {
                policy {
                    policy-id: "vrf_selection_policy_w"
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

## Baseline

*   Install the following gRIBI AFTs.

```
IPv4Entry {138.0.11.0/24 (ENCAP_TE_VRF_A)} -> NHG#101 (DEFAULT VRF) -> {
  {NH#101, DEFAULT VRF, weight:1},
  {NH#102, DEFAULT VRF, weight:3},
  backup_next_hop_group: 200 // in case specific vendor implementation or bugs pruned the NHs.
}
IPv4Entry {138.0.11.0/24 (ENCAP_TE_VRF_B)} -> NHG#102 (DEFAULT VRF) -> {
  {NH#101, DEFAULT VRF, weight:3},
  {NH#102, DEFAULT VRF, weight:1},
  backup_next_hop_group: 200 // in case specific vendor implementation or bugs pruned the NHs.
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

NHG#200 (Default VRF) {
  {NH#200, DEFAULT VRF, weight:1}
}
NH#200 -> {
    network_instance: "DEFAULT"
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

NHG#1000 (Default VRF) {
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
  backup_next_hop_group: 1001 // decap to DEFAULT VRF
}
IPv4Entry {192.0.2.103/32 (DEFAULT VRF)} -> NHG#13 (DEFAULT VRF) -> {
  {NH#14, DEFAULT VRF, weight:1,mac_address:magic_mac, interface-ref:dut-port-5-interface},
}
NHG#1001 (Default VRF) {
  {NH#2001, DEFAULT VRF, weight:1}
}
NH#1001 -> {
    decapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
    network_instance: "DEFAULT"
}

// 203.10.113.2 is the tunnel IP address. Note that the NHG#3 is different than NHG#1.

IPv4Entry {203.10.113.2/32 (TE_VRF_111)} -> NHG#3 (DEFAULT VRF) -> {
  {NH#4, DEFAULT VRF, weight:1,ip_address=192.0.2.104},
  backup_next_hop_group: 1002 // re-encap to 203.10.113.101
}
IPv4Entry {192.0.2.104/32 (DEFAULT VRF)} -> NHG#14 (DEFAULT VRF) -> {
  {NH#15, DEFAULT VRF, weight:1,mac_address:magic_mac, interface-ref:dut-port-6-interface},
}
NHG#1002 (DEFAULT VRF) {
  {NH#1002, DEFAULT VRF}
}
NH#1002 -> {
  decapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
  encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
  ip_in_ip {
    dst_ip: "203.0.113.101"
    src_ip: "ipv4_outer_src_222"
  }
  network_instance: "TE_VRF_222"
}
IPv4Entry {203.0.113.101/32 (TE_VRF_222)} -> NHG#4 (DEFAULT VRF) -> {
  {NH#5, DEFAULT VRF, weight:1,ip_address=192.0.2.103},
  backup_next_hop_group: 1001 // decap to DEFAULT VRF
}
IPv4Entry {192.0.2.103/32 (DEFAULT VRF)} -> NHG#15 (DEFAULT VRF) -> {
  {NH#16, DEFAULT VRF, weight:1,mac_address:magic_mac, interface-ref:dut-port-7-interface},
}

```

*   Install a BGP route resolved by ISIS in default VRF to rout traffic out of DUT port-8.

*   Install an 0/0 static route in ENCAP_VRF_A and ENCAP_VRF_B pointing to the DEFAULT VRF.

## Procedure

The DUT should be reset to the baseline after each of the following tests.

#### Test-1, match on source and protocol, no match on DSCP; flow VRF_DECAP hit -> DEFAULT

1.  Using gRIBI to install the following entries in the `DECAP_TE_VRF`:

    ```
    IPv4Entry {192.51.100.1/24 (DECAP_TE_VRF)} -> NHG#1001 (DEFAULT VRF) -> {
        {NH#1001, DEFAULT VRF, weight:1}
    }
    NH#1001 -> {
        decapsulate_header: OPENCONFIGAFTTYPESDECAPSULATIONHEADERTYPE_IPV4
    }
    ```

2.  Apply vrf selection policy `vrf_selection_policy_w` to DUT port-1.

3.  Send the following 6in4 and 4in4 flows to DUT port-1:

    ```
    * inner_src: `ipv4_inner_src`
    * inner_dst: `ipv4_inner_encap_match`
    * dscp: `dscp_encap_no_match`
    * outter_src: `ipv4_outter_src_111`
    * outter_dst: `ipv4_outter_decap_match`
    * dscp: `dscp_encap_no_match`
    * proto: `4`

    * inner_src: `ipv6_inner_src`
    * inner_dst: `ipv6_inner_encap_match`
    * dscp: `dscp_encap_no_match`
    * outter_src: `ipv4_outter_src_111`
    * outter_dst: `ipv4_outter_decap_match`
    * dscp: `dscp_encap_no_match`
    * proto: `41`
    ```

4.  Verify that the packets have their outer v4 header stripped and are forwarded out of DUT port-8 per the BGP-ISIS routes in the DEFAULT VRF.

5.  Verify that the TTL value is copied from the outer header to the inner header.

6.  Change the subnet mask from /24 and repeat the test for the masks  /32, /22, and /28 and verify again that the packets are decapped and forwarded correctly.

7.  Repeat the test with packets with a destination address that does not match the decap entry, and verify that such packets are not decapped.

#### Test-2, match on source, protocol and DSCP, VRF_DECAP hit -> VRF_ENCAP_A miss -> DEFAULT

1.  Using gRIBI to install the following entries in the `DECAP_TE_VRF`:

    ```
    IPv4Entry {192.51.100.1/24 (DECAP_TE_VRF)} -> NHG#1001 (DEFAULT VRF) -> {
        {NH#1001, DEFAULT VRF, weight:1}
    }
    NH#1001 -> {
        decapsulate_header: OPENCONFIGAFTTYPESDECAPSULATIONHEADERTYPE_IPV4
    }
    ```

2.  Apply vrf selection policy `vrf_selection_policy_w` to DUT port-1.

3.  Send the following 6in4 and 4in4 flows to DUT port-1:

    ```
    * inner_src: `ipv4_inner_src`
    * inner_dst: `ipv4_inner_encap_no_match`
    * dscp: `dscp_encap_a_1`
    * outter_src: `ipv4_outter_src_111`
    * outter_dst: `ipv4_outter_decap_match`
    * dscp: `dscp_encap_a_1`
    * proto: `4`

    * inner_src: `ipv6_inner_src`
    * inner_dst: `ipv6_inner_encap_no_match`
    * dscp: `dscp_encap_a_1`
    * outter_src: `ipv4_outter_src_111`
    * outter_dst: `ipv4_outter_decap_match`
    * dscp: `dscp_encap_a_1`
    * proto: `41`
    ```

4.  Verify that the packets have their outer v4 header stripped and are forwarded out of DUT port-8 per the BGP-ISIS routes in the DEFAULT VRF.

5.  Verify that the TTL value is copied from the outer header to the inner header.

6.  Change the subnet mask from /24 and repeat the test for the masks  /32, /22, and /28 and verify again that the packets are decapped and forwarded correctly.

#### Test-3, Mixed Prefix Decap gRIBI Entries

Support for decap actions with mixed prefixes installed through gRIBI

1.  Add the following gRIBI entries:

    ```
    IPv4Entry {192.51.129.0/22 (DECAP_TE_VRF)} -> NHG#1001 (DEFAULT VRF) -> {
        {NH#1001, DEFAULT VRF, weight:1}
    }
    IPv4Entry {192.55.200.3/32 (DECAP_TE_VRF)} -> NHG#1001 (DEFAULT VRF) -> {
        {NH#1001, DEFAULT VRF, weight:1}
    }

    NH#1001 -> {
        decapsulate_header: OPENCONFIGAFTTYPESDECAPSULATIONHEADERTYPE_IPV4
    }
    ```

2.  Apply vrf selection policy `vrf_selection_policy_w` to DUT port-1.

3.  Send the following 6in4 and 4in4 flows to DUT port-1:

    ```
    * inner_src: `ipv4_inner_src`
    * inner_dst: `ipv4_inner_encap_match`
    * dscp: `dscp_encap_no_match`
    * outter_src: `ipv4_outter_src_111`
    * outter_dst: `192.51.100.64`
    * dscp: `dscp_encap_no_match`
    * proto: `4`

    * inner_src: `ipv6_inner_src`
    * inner_dst: `ipv6_inner_encap_match`
    * dscp: `dscp_encap_no_match`
    * outter_src: `ipv4_outter_src_111`
    * outter_dst: `192.55.200.3`
    * dscp: `dscp_encap_no_match`
    * proto: `41`

    * inner_src: `ipv4_inner_src`
    * inner_dst: `ipv4_inner_encap_match`
    * dscp: `dscp_encap_no_match`
    * outter_src: `ipv4_outter_src_111`
    * outter_dst: `192.51.128.5`
    * dscp: `dscp_encap_no_match`
    * proto: `4`
    ```

4.  Verify that the packets have their outer v4 header stripped, and are forwarded according to the route in the DEFAULT VRF that matches the inner IP address.

5.  Repeat the test with packets with a destination address such as that does not match the decap route, and verify that such packets are not decapped.

#### Test-4: Tunneled traffic with no decap

Ensures that tunneled traffic is correctly forwarded when there is no match in the DECAP_VRF. The intent of this test is to ensure that the VRF selection policy correctly sends these packets to either `TE_VRF_111` or `TE_VRF_222`.

1.  Apply vrf selection policy `vrf_selection_policy_c` to DUT port-1.
2.  Send 4in4 (IP protocol 4) and 6in4 (IP protocol 41) packets to DUT port-1 where
    *   The outer v4 header has the destination address 203.0.113.1.
    *   The outer v4 header has the source address ipv4_outer_src_111.
    *   The outer v4 header has DSCP value has `dscp_encap_no_match` and `dscp_encap_match`
3.  We should expect that all egress packets (100%) are IPinIP encapped with 203.0.113.1 as the outer header, and egress on DUT port-2, port-3, port-4 and port-6 per the hierarchical weight.
4.  Send 4in4 (IP protocol 4) and 6in4 (IP protocol 41) packets to DUT port-2 where
    *   The outer v4 header has the destination address 203.0.113.100.
    *   The outer v4 header has the source address ipv4_outer_src_222.
    *   The outer v4 header has DSCP value has `dscp_encap_no_match` and `dscp_encap_match`
We should expect that the egress traffic are 100% encapped with 203.0.113.100 as the outer header, and egress on DUT port-5.

#### Test-5: match on "default term", send to default VRF

Tests support for TE disabled IPinIP IPv4 (IP protocol 4) cluster traffic arriving on WAN facing ports. Specifically, this test verifies the tunnel traffic identification using ipv4_outer_src_111 and ipv4_outer_src_222 in the VRF selection policy.

1.  Apply vrf selection policy `vrf_selection_policy_w` to DUT port-1.
2.  Send 6in4 and 4in4 packets to DUT port-1, where:
    *   The outer v4 header has the destination address 138.0.11.8.
    *   The outer v4 header has the source address that’s not ipv4_outer_src_111 or ipv4_outer_src_222. For example, we can use 198.100.200.123.
3.  We should expect that all egress packets:
    *   100% are still IPinIP (4in4) with outer v4 destination address as `138.0.11.8`.
    *   and, egressed out of DUT port-8 per the route in the DEFAULT VRF.
4.  Send v4 packet with protocol `17` (not 6in4 or 4in4), where:
    *   The outer v4 header has the destination address 138.0.11.8.
    *   50% of the packets with source address as ipv4_outer_src_111.
    *   50% of the packets with source address as ipv4_outer_src_222.
5.  We should expect that all egress packets:
    *   100% are still of protocl `17` and with outer v4 destination address as `138.0.11.8`.
    *   and, egressed out of DUT port-8 per the route in the DEFAULT VRF.
6.  Remove the matching route (e.g. stop the BGP routes) in the DEFAULT VRF and verify that the traffic are dropped.

#### Test-6, decap then encap

1.  Apply vrf selection policy `vrf_selection_policy_w` to DUT port-1.

2.  Send the following packets to DUT port-1:

    ```
    * inner_src: `ipv4_inner_src`
    * inner_dst: `ipv4_inner_encap_match`
    * dscp: `dscp_encap_a_1`
    * outter_src: `ipv4_outter_src_222`
    * outter_dst: `ipv4_outter_decap_match`
    * dscp: `dscp_encap_a_1`
    * proto: `4`
    ```

    ```
    * inner_src: `ipv6_inner_src`
    * inner_dst: `ipv6_inner_encap_match`
    * dscp: `dscp_encap_a_1`
    * outter_src: `ipv4_outter_src_111`
    * outter_dst: `ipv4_outter_decap_match`
    * dscp: `dscp_encap_a_1`
    * proto: `41`
    ```

3.  We should expect that all egress packets:

    *   are IPinIP encapped with outer source IP as `ipv4_outter_src_111` and dscp value `dscp_encap_a_1`.
    *   1/4 are with 203.0.113.1 as the outer header destination IP.
    *   3/4 are with 203.10.113.2 as the outer header destination IPs.
    *   egress on DUT port-2, port-3, port-4 and port-6 per the hierarchical weight.

4.  Send the following packets to DUT port -1

    ```
    * inner_src: `ipv4_inner_src`
    * inner_dst: `ipv4_inner_encap_match`
    * dscp: `dscp_encap_b_1`
    * outter_src: `ipv4_outter_src_111`
    * outter_dst: `ipv4_outter_decap_match`
    * dscp: `dscp_encap_b_1`
    * proto: `4`

    * inner_src: `ipv6_inner_src`
    * inner_dst: `ipv6_inner_encap_match`
    * dscp: `dscp_encap_b_1`
    * outter_src: `ipv4_outter_src_222`
    * outter_dst: `ipv4_outter_decap_match`
    * dscp: `dscp_encap_b_1`
    * proto: `41`
    ```

5.  We should expect that all egress packets:

    *   are IPinIP encapped with outer source IP as `ipv4_outter_src_111` and dscp value `dscp_encap_b_1`.
    *   3/4 are with 203.0.113.1 as the outer header destination IP.
    *   1/4 are with 203.10.113.2 as the outer header destination IPs.
    *   egress on DUT port-2, port-3, port-4 and port-6 per the hierarchical weight.


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

## Protocol/RPC Parameter Coverage

*   gRIBI:
    *   Modify
        *   ModifyRequest

## Required DUT platform

vRX