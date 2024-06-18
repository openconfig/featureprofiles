# P4RT-5.1: Traceroute: PacketIn

## Summary

Test FRR behaviors with encapsulation scenarios.

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

*   Install a BGP route resolved by ISIS in default VRF to route traffic out of DUT port-8 for 1.2.3.4.

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

Tests that traceroute with TTL=1 matched the VRF selection policy for encap.

*   Send packets to DUT port-1 with outer packet header TTL=1. The outer v4 header has the destination
    addresses 138.0.11.8. verify that packets with TTL=1 are received by the client.

*   Verify that the punted packets have both ingress_port and target_egress_port metadata set.
The distribution of packets should have target_egress_port set with port 2 10% of the time, port 3 30%, port 4 60%.

### Test-2

Tests that traceroute with TTL=1 matched the VRF selection policy for default.

*   Send packets to DUT port-1 with outer packet TTL=1. The outer v4 header has the destination
    address 1.2.3.4.  Verify that packets with TTL=1 are received by the client.
*   Verify that the packets have both ingress_port and target_egress_port metadata set.
target_egress_port should be dut port 8.


### Test-3 (TDB from here)

Tests that traceroute for a packet that should hit a transit VRF does so.

### Test-4

Tests that traceroute respects transit FRR.

### Test-5

Tests that traceroute respects transit FRR when the backup is also unviable.

### Test-6

Tests that traceroute respects decap.

### Test-7

Encap failure cases (TBD on confirmation)

### Test-8

Tests that traceroute for a packet with no route reports a miss.

### Test-9

Decap-And-Reencap cases (TBD)

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


