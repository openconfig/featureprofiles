# RT-14.2: GRIBI Route Test

## Summary

Ensure Traffic is Encap/Decap to NextHop based on Gribi structure.

## Topology

ATE port-1 <------> port-1 DUT
DUT port-2 <------> port-2 ATE
DUT port-3 <------> port-3 ATE

## Variables
```
# Magic source IP addresses used in this test
  * ipv4_outer_src_111 = 198.51.100.111
  * ipv4PrefixEncapped = ipv4InnerDst = 138.0.11.8
  * ipv4PrefixNotEncapped = ipv4OuterDst222 = 198.50.100.65
```

## Baseline

### VRF Selection Policy

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
                                source-address: "ipv4_outer_src_111"
                            }
                            action {
                                network-instance: "ENCAP_TE_VRF_A"
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
                                network-instance: "TRANSIT_TE_VRF"
                            }
                        }
                    }
                }
            }
        }
    }
}
```
```
### Install the following gRIBI AFTs.

- IPv4Entry {0.0.0.0/0 (TRANSIT_TE_VRF)} -> NHG#1 (DEFAULT VRF) -> {
    {NH#1, DEFAULT VRF, weight:1},
  }
  NH#1 -> {
    decapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
    network_instance: "DEFAULT"
  }
- IPv4Entry {198.50.100.64/32 (TRANSIT_TE_VRF)} -> NHG#2 (DEFAULT VRF) -> {
    {NH#2, DEFAULT VRF, weight:1}, interface-ref:dut-port-2-interface,
  }
- IPv4Entry {198.50.100.64/32 (TE_VRF_111)} -> NHG#3 (DEFAULT VRF) -> {
    {NH#3, DEFAULT VRF, weight:1}, interface-ref:dut-port-2-interface,
  }
- IPv4Entry {0.0.0.0/0 (ENCAP_TE_VRF_A)} -> NHG#5 (DEFAULT VRF) -> {
    {NH#5, DEFAULT VRF, weight:1, interface-ref:dut-port-3-interface},
  }
- IPv4Entry {138.0.11.8/32 (ENCAP_TE_VRF_A)} -> NHG#4 (DEFAULT VRF) -> {
    {NH#4, DEFAULT VRF, weight:1},
  }
  NH#4 -> {
  encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
  ip_in_ip {
    dst_ip: "198.50.100.64"
    src_ip: "ipv4_outer_src_111"
  }
  network_instance: "TE_VRF_111"
  }
```
- Install a BGP route in default VRF to route traffic out of DUT port-3.

## Procedure

The DUT should be reset to the baseline after each of the following tests.

Test-1, Match on source prefix, flow hits ENCAP_TE_VRF_A followed by TE_VRF_111

```
  1. Send flow with source IP ipv4_outer_src_111 with destination IP ipv4PrefixEncapped.
  2. Verify v4 packet matched with tunnel prefix and encapped -> hit TE_VRF_111 
     and egress via port-2.
  3. No traffic loss in steady state

```
Test-2, Match on source prefix, flow hits ENCAP_TE_VRF_A followed by Default VRF

```
  1. Send flow with source IP ipv4_outer_src_111 with destination IP ipv4PrefixNotEncapped.
  2. Verify v4 packet not matched with tunnel prefix and egress via port-3.
  3. No traffic loss in steady state

```
Test-3, Match on source prefix and protocol, flow hits TRANSIT_TE_VRF
match with Tunnel prefix /32

```
  1. Send the following 4in4 flows to DUT port-1: 
    * inner_src: `ipv4_inner_src`
    * inner_dst: `ipv4InnerDst`
    * outter_src: `ipv4_outter_src_111`
    * outter_dst: `ipv4_outter_decap_no_match`
    * proto: `4`
  2. Verify packet matched with tunnel prefix and egress via port-2.
  3. No traffic loss in steady state

```
Test-4, Match on source prefix and protocol, flow hits TRANSIT_TE_VRF matched with 0/0 prefix, decap & sent to default vrf
```
  1. Send the following 4in4 flows to DUT port-1: 
    * inner_src: `ipv4_inner_src`
    * inner_dst: `ipv4InnerDst`
    * outter_src: `ipv4_outter_src_111`
    * outter_dst: `ipv4_outter_decap_match`
    * proto: `4`
  2. Verify packet not matched with tunnel prefix, decap and failback 
     to default vrf.
  3. No traffic loss in steady state

```
## Config Parameter Coverage

*   network-instances/network-instance/name
*   network-instances/network-instance/policy-forwarding/policies/policy/policy-id
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/sequence-id
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/protocol
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/source-address
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/source-address
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/network-instance

## Telemetry Parameter Coverage

*   network-instances/network-instance/name
*   network-instances/network-instance/policy-forwarding/policies/policy/policy-id
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/sequence-id
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/protocol
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/source-address
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/source-address
*   network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/network-instance

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
