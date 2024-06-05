# TE-16.1: basic encapsulation tests

## Summary

Test basic encapsulation behaviors.

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

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
  gribi:
    gRIBI.Modify:
    gRIBI.Flush:    
```
