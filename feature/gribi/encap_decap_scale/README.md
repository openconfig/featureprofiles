# TE-14.2 encap and decap scale

NOTE: this test depends on https://github.com/openconfig/featureprofiles/pull/1950 

## Summary

Introduce encapsulation and decapsulation scale test on top of TE-14.1

## Topology

Use the same topology as TE-14.1

## Baseline

1.  Build the same scale setup as TE-14.1.
2.  Apply `vrf_selection_policy_w` to DUT port-1.

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

## Procedure

1.  via gRIBI installs the following AFT entries:
    *   Add 4 VRFs for encapsulations: `ENCAP_TE_VRF_A`, `ENCAP_TE_VRF_B`, `ENCAP_TE_VRF_C` and `ENCAP_TE_VRF_D`.
    *   Inject 5000 IPv4Entry-ies and 5000 IPv6Entry-ies to each of the 4 encap VRFs.
    *   The entries in the encap VRFs should point to NextHopGroups in the `DEFAULT` VRF. Inject 200 such NextHopGroups in the DEFAULT VRF.
    *   Each NextHopGroup should have 8 NextHops where each NextHop points to a tunnel in the `TE_VRF_111`. In addition, the weights specified in the NextHopGroup should be co-prime and the sum of the weights should be 16.
    *   Add 1 VRF for decapsulation, `DECAP_TE_VRF`.
    *   Inject `48` entries in the DECAP_TE_VRF where the entries have a mix of prefix lengths /22, /24, /26, and /28.
2.  Send the following packets to DUT-1

    ```
    * inner_src: `ipv4_inner_src`
    * inner_dst: `ipv4_inner_encap_match`
    * dscp: `dscp_encap_a`
    * outter_src: `ipv4_outter_src_222`
    * outter_dst: `ipv4_outter_decap_match`
    * dscp: `dscp_encap_a`
    * proto: `4`

    * inner_src: `ipv6_inner_src`
    * inner_dst: `ipv6_inner_encap_match`
    * dscp: `dscp_encap_a`
    * outter_src: `ipv4_outter_src_111`
    * outter_dst: `ipv4_outter_decap_match`
    * dscp: `dscp_encap_a`
    * proto: `41`

    * inner_src: `ipv4_inner_src`
    * inner_dst: `ipv4_inner_encap_match`
    * dscp: `dscp_encap_b`
    * outter_src: `ipv4_outter_src_222`
    * outter_dst: `ipv4_outter_decap_match`
    * dscp: `dscp_encap_b`
    * proto: `4`

    * inner_src: `ipv6_inner_src`
    * inner_dst: `ipv6_inner_encap_match`
    * dscp: `dscp_encap_b`
    * outter_src: `ipv4_outter_src_111`
    * outter_dst: `ipv4_outter_decap_match`
    * dscp: `dscp_encap_b`
    * proto: `41`

    * inner_src: `ipv4_inner_src`
    * inner_dst: `ipv4_inner_encap_match`
    * dscp: `dscp_encap_c`
    * outter_src: `ipv4_outter_src_222`
    * outter_dst: `ipv4_outter_decap_match`
    * dscp: `dscp_encap_c`
    * proto: `4`

    * inner_src: `ipv6_inner_src`
    * inner_dst: `ipv6_inner_encap_match`
    * dscp: `dscp_encap_c`
    * outter_src: `ipv4_outter_src_111`
    * outter_dst: `ipv4_outter_decap_match`
    * dscp: `dscp_encap_c`
    * proto: `41`

    * inner_src: `ipv4_inner_src`
    * inner_dst: `ipv4_inner_encap_match`
    * dscp: `dscp_encap_d`
    * outter_src: `ipv4_outter_src_222`
    * outter_dst: `ipv4_outter_decap_match`
    * dscp: `dscp_encap_d`
    * proto: `4`

    * inner_src: `ipv6_inner_src`
    * inner_dst: `ipv6_inner_encap_match`
    * dscp: `dscp_encap_d`
    * outter_src: `ipv4_outter_src_111`
    * outter_dst: `ipv4_outter_decap_match`
    * dscp: `dscp_encap_d`
    * proto: `41`    
    ```

3.  Send traffic to DUT-1, covering all the installed v4 and v6 entries in the decap and encap VRFs. Validate that all traffic are all decapped per the DECAP VRFs and then encapsulated per the ENCAP VRFs and received as encapsulated packet by ATE.
4.  Flush the `DECAP_TE_VRF`, install 5000 entries with fixed prefix length of /32, and repeat the same traffic validation.

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