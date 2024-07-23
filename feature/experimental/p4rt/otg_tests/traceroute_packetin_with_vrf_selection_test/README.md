# P4RT-5.3: Traceroute: PacketIn With VRF Selection

## Summary

Test FRR behaviors with VRF selection scenarios.

## Topology

ATE port-1 <------> port-1 DUT
DUT port-2 <------> port-2 ATE
DUT port-3 <------> port-3 ATE
DUT port-4 <------> port-4 ATE
DUT port-5 <------> port-5 ATE
DUT port-6 <------> port-6 ATE
DUT port-7 <------> port-7 ATE
DUT port-8 <------> port-8 ATE

## Baseline setup

*   Setup equivalent to [TE-17.1 vrf_policy_driven_te](https://github.com/openconfig/featureprofiles/blob/main/feature/experimental/gribi/otg_tests/vrf_policy_driven_te/README.md), including GRibi programming.

*   Install a BGP route resolved by ISIS in default VRF to route traffic out of DUT port-8 for 203.0.113.0.

*   Enable the P4RT server on the device.

*   Connect a P4RT client and configure the forwarding pipeline. Install P4RT table entries required for traceroute.
	These are located in [p4rt_utils.go] (https://github.com/openconfig/featureprofiles/blob/main/internal/p4rtutils/p4rtutils.go)
	p4rtutils.ACLWbbIngressTableEntryGet(packetIO.GetTableEntry(delete, isIPv4))


## Procedure

At the start of each of the following scenarios, ensure:

*   All ports are up and baseline is reset as above.

Unless otherwise specified, all the tests below should use traffic with
`dscp_encap_a_1` referenced int he VRF selection policy.

### Test-1

Tests that traceroute with TTL=1 for a packet that would match the VRF
selection policy for encap has target_egress_port set based on the
encap VRF.

*   Send packets to DUT port-1 with outer packet header TTL=1. The outer v4 header has the destination
    addresses 138.0.11.8. verify that packets with TTL=1 are received by the client.

*   Verify that the punted packets have both ingress_port and target_egress_port metadata set.

The distribution of packets should have target_egress_port set with port2 1.56% of
the time, port3 4.68%, port4 18.75% and port6 75%.

### Test-2

Tests that traceroute with TTL=1 for a packet that would match the VRF
selection policy for default has target_egress_port set based on the
default VRF.

*   Send packets to DUT port-1 with packet TTL=1. The v4 header has the destination
    address 203.0.113.0.  Verify that packets with TTL=1 are received by the client.
*   Verify that the packets have both ingress_port and target_egress_port metadata set.
target_egress_port should be dut port 8.


### Test-3

Tests that a packet punted due to TTL=1 for a packet that would
otherwise hit a transit VRF has target_egress_port set based on that
transit VRF.

*   Send 4in4 (IP protocol 4) and 6in4 (IP protocol 41) packets to DUT port-1 where
    *   The outer v4 header has the destination address 203.0.113.1.
    *   The outer v4 header has the source address ipv4_outer_src_111.
    *   The outer v4 header has DSCP value has `dscp_encap_no_match` and `dscp_encap_match`
	*   The outer v4 header has TTL=1
*  Verify that the punted packets have both ingress_port and
   target_egress_port metadata set.  target_egress_port should be set
   to on DUT port-2, port-3, and port-4 per the heirarchical weight.

### Test-4 (TBD)

Tests that traceroute respects transit FRR.

### Test-5 (TBD)

Tests that traceroute respects transit FRR when the backup is also unviable.

### Test-6

Tests that traceroute respects decap rules.

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
    * outer_src: `ipv4_outer_src_111`
    * outer_dst: `ipv4_outer_decap_match`
    * dscp: `dscp_encap_no_match`
    * proto: `4`
	* outer TTL: `1`

    * inner_src: `ipv6_inner_src`
    * inner_dst: `ipv6_inner_encap_match`
    * dscp: `dscp_encap_no_match`
    * outer_src: `ipv4_outer_src_111`
    * outer_dst: `ipv4_outer_decap_match`
    * dscp: `dscp_encap_no_match`
    * proto: `41`
	* outer TTL: `1`
    ```

4.  Verify that all punted packets:
    *   Have ingress_port and target_egress_port metadata set
    *   target_egress_port is set to DUT port-8 per the hierarchical weight.

6.  Change the subnet mask from /24 and repeat the test for the masks  /32, /22, and /28 and verify again that the packets are punted correctly.


### Test-7 (TBD)

Encap failure cases (TBD on confirmation)

### Test-8 (TBD)

Tests that traceroute for a packet with a route lookup miss has an unset target_egress_port.

### Test-9, decap the encap

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

3.  Send the following packets to DUT port-1:

    ```
    * inner_src: `ipv4_inner_src`
    * inner_dst: `ipv4_inner_encap_match`
    * dscp: `dscp_encap_a_1`
    * outer_src: `ipv4_outer_src_222`
    * outer_dst: `ipv4_outer_decap_match`
    * dscp: `dscp_encap_a_1`
    * proto: `4`
    * outer TTL: '1'
    ```

    ```
    * inner_src: `ipv6_inner_src`
    * inner_dst: `ipv6_inner_encap_match`
    * dscp: `dscp_encap_a_1`
    * outer_src: `ipv4_outer_src_111`
    * outer_dst: `ipv4_outer_decap_match`
    * dscp: `dscp_encap_a_1`
    * proto: `41`
	* outer TTL: `1`
    ```

4.  We should expect that all punted packets:
    *   Have ingress_port and target_egress_port metadata set
    *   target_egress_port is set to DUT port-2, port-3, port-4 and port-6 per the hierarchical weight.

5.  Send the following packets to DUT port -1

    ```
    * inner_src: `ipv4_inner_src`
    * inner_dst: `ipv4_inner_encap_match`
    * dscp: `dscp_encap_b_1`
    * outer_src: `ipv4_outer_src_111`
    * outer_dst: `ipv4_outer_decap_match`
    * dscp: `dscp_encap_b_1`
    * proto: `4`

    * inner_src: `ipv6_inner_src`
    * inner_dst: `ipv6_inner_encap_match`
    * dscp: `dscp_encap_b_1`
    * outer_src: `ipv4_outer_src_222`
    * outer_dst: `ipv4_outer_decap_match`
    * dscp: `dscp_encap_b_1`
    * proto: `41`
    ```

6.  Verify that all punted packets:
    *   Have ingress_port and target_egress_port metadata set
    *   target_egress_port is set to DUT port-2, port-3, port-4 and port-6 per the hierarchical weight.

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
