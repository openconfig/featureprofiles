# RT-3.1: Protocol Dscp Based Vrf Selection

## Summary

Configuration of protocol and DSCP based VRF selection and verification

## Procedure

Configure DUT with 1 input interface and egress VLANs via a single DUT port-2 connected to ATE port-2 corresponding to: 
*   network-instance “10”, default route to egress via VLAN 10 
*   network-instance “20”, default route to egress via VLAN 20
 
Configure DUT and validate packet forwarding for: 
*   Matching of IPv4 protocol - to network-instance 10 for all input IPv4. No IPv6 packets received in network-instance 10. 
*   Matching of IPinIP protocol - to network-instance 10 for all input IPinIP, no IPv4 or IPv6 packets received in network-instance 10. 
*   Matching of IPv4 to network-instance 10, and IPv6 to network-instance 20 -  ensure that only IPv4 packets are seen at egress VLAN 10, and IPv6 received at VLAN 20. 
*   Match IPinIP, single DSCP 46 - routed to network-instance 10 - ensure that only DSCP 46 packets are received at VLAN 10. Ensure that DSCP 0 packets are not received at VLAN 10. 
*   Match IPinIP, DSCP 42, 46 - routed to network-instance 10, ensure that DSCP 42 and 46 packets are received at VLAN 10. Ensure that DSCP 0 packets are not received at VLAN 10. 

## Config Parameter Coverage
 *  /openconfig-network-instance/network-instances/network-instance/policy-forwarding/policies/policy/config/type
 *  /openconfig-network-instance/network-instances/network-instance/policy-forwarding/policies/policy/policy-id
 *  /openconfig-network-instance/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/sequence-id
 *  /openconfig-network-instance/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/l2/config/ethertype
 *  /openconfig-network-instance/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/dscp-set
 *  /openconfig-network-instance/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/dscp-set
 *  /openconfig-network-instance/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/protocol
 *  /openconfig-network-instance/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/protocol
 *  /openconfig-network-instance/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/network-instance
 *  /openconfig-network-instance/network-instances/network-instance/policy-forwarding/interfaces/interface/interface-id
 *  /openconfig-network-instance/network-instances/network-instance/policy-forwarding/interfaces/interface/config/apply-vrf-selection-policy

## Paths

* /openconfig-network-instance/network-instances/network-instance/policy-forwarding
* /openconfig-network-instance/network-instances/network-instance/policy-forwarding/policies/policy
* /openconfig-network-instance/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule
* /openconfig-network-instance/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4
* /openconfig-network-instance/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6
* /openconfig-network-instance/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/l2
* /openconfig-network-instance/network-instances/network-instance/policy-forwarding/interfaces/interface/config/apply-vrf-selection-policy 

