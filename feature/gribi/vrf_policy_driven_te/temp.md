# TE-17.1 VRF selection policy driven TE.

## Summary
Test VRF selection logic involving different decapsulation and encapsulation lookup scenarios via gRIBI.

## Topology

ATE port-1 <------> port-1 DUT\
DUT port-2 <------> port-2 ATE\
DUT port-3 <------> port-3 ATE\
DUT port-4 <------> port-4 ATE\
DUT port-5 <------> port-5 ATE\
DUT port-6 <------> port-6 ATE\
DUT port-7 <------> port-7 ATE\ 
DUT port-8 <------> port-8 ATE\ (exited by static route in the DEFAULT VRF)

## Test Setup

* Variables:
    // DSCP value that will be matched to VRF_ENCAP_A
    * `dscp_encap_a_1` = 10
    * `dscp_encap_a_2` = 18

    // DSCP value that will be matched to VRF_ENCAP_B
    * `dscp_encap_b_1` = 20
    * `dscp_encap_b_2` = 28

    // DSCP value that will NOT be matched to any VRF for encapsulation.
    * `dscp_encap_no_match` = 30

    // ipv4 destination IP that will produce a lookup hit in the VRF_DECAP.
    * `ipv4_outter_decap_match`  = `203.0.113.100`

    // ipv4 destination IP that will NOT produce a lookup hit in the VRF_DECAP.
    * `ipv4_outter_decap_no_match`  = `203.0.113.200`

    // prefix installed in the VRF_DECAP.
    * `ipv4_prefix_decap_entry` = `ipv4_outter_decap_match`/32

    // Prefixes installed in the VRF_ENCAP_A and VRF_ENCAP_B
    * `ipv4_prefix_inner_encap_entry_1` = `198.18.10.0/30`
    * `ipv4_prefix_inner_encap_entry_2` = `198.18.10.8/29`
    * `ipv6_prefix_inner_encap_entry_1` = `2001:DB8:2::198:18:10:0/126`
    * `ipv6_prefix_inner_encap_entry_2` = `2001:DB8:2::198:18:10:8/125`

    // IPs that will produce a lookup hit in the VRF_ENCAP_A and VRF_ENCAP_B.
    * `ipv4_inner_encap_match` = `198.18.0.1`
    * `ipv6_inner_encap_match` = `2001:DB8:2::198:18:0:1`

    // Outer IPv4 destination IP that VRF_ENCAP_A encapsulate packet to
    * `ipv4_outter_encap_a` = `203.0.113.10`

    // Outer IPv4 destination IP that VRF_ENCAP_B encapsulate packet to
    * `ipv4_outter_encap_b` = `203.0.113.20`

    // Outer IPv4 source IP options:
    * `ipv4_outter_src_111` = `198.51.100.111`
    * `ipv4_outter_src_222` = `198.51.100.222`

    // Inner IPv4 source IP options:
    * `ipv4_inner_src` = `198.18.1.1`
    * `ipv6_inner_src` = `2001:DB8:2::198:18:1:1`


* VRF_DECAP network instance, where gRIBI installs the following entries for decapsulation:
    * `ipv4_prefix_decap_entry`

* VRF_ENCAP_A network instance, where gRIBI installs the following entries to encap to {src: `ipv4_outter_src_111`, dst:`ipv4_outter_encap_a`} and with VRF_111 for forwarding lookup.
    * `ipv4_prefix_inner_encap_entry_1`
    * `ipv4_prefix_inner_encap_entry_2`
    * `ipv6_prefix_inner_encap_entry_1`
    * `ipv6_prefix_inner_encap_entry_2`

* VRF_ENCAP_B network instance, where gRIBI installs the following entries to encap to {src: `ipv4_outter_src_111`, dst:`ipv4_outter_encap_b`} and with VRF_111 for forwarding lookup.
    * `ipv4_prefix_inner_encap_entry_1`
    * `ipv4_prefix_inner_encap_entry_2`
    * `ipv6_prefix_inner_encap_entry_1`
    * `ipv6_prefix_inner_encap_entry_2`

* VRF_111 network instance, where gRIBI installs the following entries for routing:
    * `ipv4_outter_encap_a`/32 route via DUT port-2
    * `ipv4_outter_encap_b`/32 route via DUT port-2

* VRF_222 network instance, where gRIBI install the following entries for routing:
    * `ipv4_outter_encap_a`/32 route via DUT port-3
    * `ipv4_outter_encap_b`/32 route via DUT port-3

* DEFAULT network instance, where gRIBI installs the following entries for routing:
    * `0/0` route via DUT port-4

* `vrf_selection_policy_c`
```
network-instances {
    network-instance {
        name: DEFAULT
        policy-forwarding {
            policies {
                policy {
                    policy-id: "merged VRF selection policy"
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

* `vrf_selection_policy_w`
```
network-instances {
    network-instance {
        name: DEFAULT
        policy-forwarding {
            policies {
                policy {
                    policy-id: "merged VRF selection policy"
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

### Test procedure

// TE-2.1 have question here, this is related to TE-2.1 in the dcGate test plan
**Test-5**: match on source and protocol, no match on DSCP; flow VRF_DECAP hit -> DEFAULT

Traffic flow#5.1
* inner_src: `ipv4_inner_src`
* inner_dst: `ipv4_inner_encap_match`
* dscp: `dscp_encap_no_match`
* outter_src: `ipv4_outter_src_111`
* outter_dst: `ipv4_outter_decap_match`
* dscp: `dscp_encap_no_match`
* proto: `4`

Traffic flow#5.2
* inner_src: `ipv6_inner_src`
* inner_dst: `ipv6_inner_encap_match`
* dscp: `dscp_encap_no_match`
* outter_src: `ipv4_outter_src_111`
* outter_dst: `ipv4_outter_decap_match`
* dscp: `dscp_encap_no_match`
* proto: `41`

Wanted behavior: ATE receives traffic flows
* on port-4, and of the following attributes:
* dscp: `dscp_encap_no_match`


---------------------------------------------------------

// TE-2.2
apply `vrf_selectioin_policy_c`

```
IPv4Entry {138.0.11.0/24 (ENCAP_TE_VRF_A)} -> NHG#10 (DEFAULT VRF) -> {
  {NH#201, DEFAULT VRF, weight:1},
}
NH#201 -> {
  encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
  ip_in_ip {
    dst_ip: "203.0.113.1"
    src_ip: "ipv4_outer_src_111"
  }
  network_instance: "TE_VRF_111"
}

# 203.0.113.1 is the tunnel IP address.

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

```

**Test-8**: "match on only on DSCP"; flow VRF_ENCAP_A

Traffic flow#8.1
* src: `ipv4_inner_src`
* dst: `ipv4_inner_encap_match`
* dscp: `dscp_encap_a_1`
* proto: `17`

Traffic flow#8.2
* inner_src: `ipv6_inner_src`
* inner_dst: `ipv6_inner_encap_match`
* dscp: `dscp_encap_a_1`
* proto: `17`

Wanted behavior: ATE receives traffic flows
* on port-2 with the following attributes:
* src: `ipv4_outter_src_111`
* dst:`ipv4_outter_encap_a`
* dscp: `dscp_encap_a_1`


**Test-9**: "match on only on DSCP"; flow VRF_ENCAP_B

Traffic flow#9.1
* src: `ipv4_inner_src`
* dst: `ipv4_inner_encap_match`
* dscp: `dscp_encap_b_1`
* proto: `17`

Traffic flow#9.2
* inner_src: `ipv6_inner_src`
* inner_dst: `ipv6_inner_encap_match`
* dscp: `dscp_encap_b_1`
* proto: `17`

Wanted behavior: ATE receives traffic flows
* on port-2 with the following attributes:
* src: `ipv4_outter_src_111`
* dst:`ipv4_outter_encap_b`
* dscp: `dscp_encap_b_1`

// in above we should add a flow that are v4inv4 or v6inv4, but without 111 or 222 and still got encapsulated.

---------------------------------------------------------

// TE-2.3
Apply `vrf_selectioin_policy_c`


---------------------------------------------------------

**Test-1**: match on source, protocol and DSCP; VRF_DECAP hit -> VRF_ENCAP_A

Traffic flow#1.1
* inner_src: `ipv4_inner_src`
* inner_dst: `ipv4_inner_encap_match`
* dscp: `dscp_encap_a_1`
* outter_src: `ipv4_outter_src_222`
* outter_dst: `ipv4_outter_decap_match`
* dscp: `dscp_encap_a_1`
* proto: `4`

Traffic flow#1.2
* inner_src: `ipv6_inner_src`
* inner_dst: `ipv6_inner_encap_match`
* dscp: `dscp_encap_a_1`
* outter_src: `ipv4_outter_src_222`
* outter_dst: `ipv4_outter_decap_match`
* dscp: `dscp_encap_a_1`
* proto: `41`

Wanted behavior: ATE receives traffic flows:
* on port-2, and of the following attributes
* src: `ipv4_outter_src_111`
* dst:`ipv4_outter_encap_a`
* dscp: `dscp_encap_a_1`


**Test-2**: match on source, protocol and DSCP, VRF_DECAP hit -> VRF_ENCAP_B

Traffic flow#2.1
* inner_src: `ipv4_inner_src`
* inner_dst: `ipv4_inner_encap_match`
* dscp: `dscp_encap_b_1`
* outter_src: `ipv4_outter_src_222`
* outter_dst: `ipv4_outter_decap_match`
* dscp: `dscp_encap_b_1`
* proto: `4`

Traffic flow#2.2
* inner_src: `ipv6_inner_src`
* inner_dst: `ipv6_inner_encap_match`
* dscp: `dscp_encap_b_1`
* outter_src: `ipv4_outter_src_222`
* outter_dst: `ipv4_outter_decap_match`
* dscp: `dscp_encap_b_1`
* proto: `41`

Wanted behavior: ATE receives traffic flows
* on port-2, and of the following attributes
* src: `ipv4_outter_src_111`
* dst:`ipv4_outter_encap_b`
* dscp: `dscp_encap_b_1`


**Test-3**: match on source, protocol and DSCP, VRF_DECAP miss -> VRF_R

Traffic flow#3.1
* inner_src: `ipv4_inner_src`
* inner_dst: `ipv4_inner_encap_match`
* dscp: `dscp_encap_a_1`
* outter_src: `ipv4_outter_src_222`
* outter_dst: `ipv4_outter_decap_no_match`
* dscp: `dscp_encap_a_1`
* proto: `4`

Traffic flow#3.2
* inner_src: `ipv6_inner_src`
* inner_dst: `ipv6_inner_encap_match`
* dscp: `dscp_encap_a_1`
* outter_src: `ipv4_outter_src_222`
* outter_dst: `ipv4_outter_decap_no_match`
* dscp: `dscp_encap_a_1`
* proto: `41`

Wanted behavior: ATE receives traffic flows
* on port-3, and of the following attributes
* src: `ipv4_outter_src_222`
* dst: `ipv4_outter_decap_no_match`
* dscp: `dscp_encap_a_1`


**Test-4**: match on source, protocol and DSCP; flow VRF_DECAP miss -> VRF_T

Traffic flow#4.1
* inner_src: `ipv4_inner_src`
* inner_dst: `ipv4_inner_encap_match`
* dscp: `dscp_encap_a_1`
* outter_src: `ipv4_outter_src_111`
* outter_dst: `ipv4_outter_decap_no_match`
* dscp: `dscp_encap_a_1`
* proto: `4`

Traffic flow#4.2
* inner_src: `ipv6_inner_src`
* inner_dst: `ipv6_inner_encap_match`
* dscp: `dscp_encap_a_1`
* outter_src: `ipv4_outter_src_111`
* outter_dst: `ipv4_outter_decap_no_match`
* dscp: `dscp_encap_a_1`
* proto: `41`

Wanted behavior: ATE receives traffic flows:
* on port-2, and of the following attributes
* src: `ipv4_outter_src_111`
* dst: `ipv4_outter_decap_no_match`
* dscp: `dscp_encap_a_1`


**Test-6**: match on source and protocol, no match on DSCP; flow VRF_DECAP miss -> VRF_R

Traffic flow#6.1
* inner_src: `ipv4_inner_src`
* inner_dst: `ipv4_inner_encap_match`
* dscp: `dscp_encap_no_match`
* outter_src: `ipv4_outter_src_222`
* outter_dst: `dscp_encap_no_match`
* dscp: `dscp_encap_no_match`
* proto: `4`

Traffic flow#6.2
* inner_src: `ipv6_inner_src`
* inner_dst: `ipv6_inner_encap_match`
* dscp: `dscp_encap_no_match`
* outter_src: `ipv4_outter_src_222`
* outter_dst: `dscp_encap_no_match`
* dscp: `dscp_encap_no_match`
* proto: `41`

Wanted behavior: ATE receives traffic flows
* on port-3, and of the following attributes:
* dscp: `dscp_encap_no_match`

**Test-7**: match on source and protocol, no match on DSCP; flow VRF_DECAP miss -> VRF_T

Traffic flow#7.1
* inner_src: `ipv4_inner_src`
* inner_dst: `ipv4_inner_encap_match`
* dscp: `dscp_encap_no_match`
* outter_src: `ipv4_outter_src_111`
* outter_dst: `dscp_encap_no_match`
* dscp: `dscp_encap_no_match`
* proto: `4`

Traffic flow#7.2
* inner_src: `ipv6_inner_src`
* inner_dst: `ipv6_inner_encap_match`
* dscp: `dscp_encap_no_match`
* outter_src: `ipv4_outter_src_111`
* outter_dst: `dscp_encap_no_match`
* dscp: `dscp_encap_no_match`
* proto: `41`

Wanted behavior: ATE receives traffic flows
* on port-2, and of the following attributes:
* dscp: `dscp_encap_no_match`



**Test-10**: "default term"

Traffic flow#10.1
* src: `ipv4_inner_src`
* dst: `ipv4_inner_encap_match`
* dscp: `dscp_encap_no_match`
* proto: `17`

Traffic flow#10.2
* inner_src: `ipv6_inner_src`
* inner_dst: `ipv6_inner_encap_match`
* dscp: `dscp_encap_no_match`
* proto: `17`

Wanted behavior: ATE receives traffic flows
* on port-4 with the following attributes:
* dscp: `dscp_encap_no_match`