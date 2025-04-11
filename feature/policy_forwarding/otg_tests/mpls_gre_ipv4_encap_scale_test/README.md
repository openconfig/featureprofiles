# PF-1.15 - MPLSoGRE IPV4 encapsulation of IPV4/IPV6 payload scale test

## Summary

This test verifies scaling of MPLSoGRE encapsulation of IP traffic using policy-forwarding configuration. Traffic on ingress to the DUT is encapsulated and forwarded towards the egress with an IPV4 tunnel header, GRE, MPLS label and the incoming IPV4/IPV6 payload.

## Testbed type

* [`featureprofiles/topologies/atedut_8.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_8.testbed)

## Procedure

### Test environment setup

```text
DUT has an ingress and 2 egress aggregate interfaces.

                         |         | --eBGP-- | ATE Ports 3,4 |
    [ ATE Ports 1,2 ]----|   DUT   |          |               |
                         |         | --eBGP-- | ATE Port 5,6  |
```

Test uses aggregate 802.3ad bundled interfaces (Aggregate Interfaces).

* Ingress Port: Traffic is generated from Aggregate1 (ATE Ports 1,2).

* Egress Ports: Aggregate2 (ATE Ports 3,4) and Aggregate3 (ATE Ports 5,6) are used as the destination ports for encapsulated traffic.

## PF-1.15.1: Generate DUT Configuration
Please generate config using PF-1.14.1

## PF-1.15.2: Verify IPV4/IPV6 traffic scale

Generate IPV4 and IPV6 traffic on ATE Ports 1,2 to random destination addresses including addresses configured on the device
Increase the number of VLANs on the device and scale traffic across all the new VLANs on Aggregate1 (ATE Ports 1,2)

Verify:
* All traffic received on Aggregate1 other than local traffic gets forwarded as MPLSoGRE-encapsulated packets
* IPV4 unicast are preserved during encapsulation.
* No packet loss when forwarding with counters incrementing corresponding to traffic.
* Traffic equally load-balanced across 16 GRE destinations and distributed equally across 2 egress ports.
* Verify that device can achieve the maximum interface scale on the device
* Verify that entire static label range is usable and functional by sending traffic across the entire label range

## OpenConfig Path and RPC Coverage
TODO: Finalize and update the below paths after the review and testing on any vendor device.

```yaml
paths:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-pkts:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-octets:
  /network-instances/network-instance/afts/policy-forwarding/policy-forwarding-entry/state/counters/packets-forwarded:
  /network-instances/network-instance/afts/policy-forwarding/policy-forwarding-entry/state/counters/octets-forwarded:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/sequence-id:
  /network-instances/network-instance/policy-forwarding/interfaces/interface/config/apply-forwarding-policy:
  /network-instances/network-instance/policy-forwarding/interfaces/interface/config/interface-id:
  /network-instances/network-instance/policy-forwarding/policies/policy/config/policy-id:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/destination-address:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/icmpv4/config/type:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/icmpv6/config/type:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/icmpv4/config/code:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/icmpv6/config/code:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/hop-limit:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/hop-limit:
  /network-instances/network-instance/static/next-hop-groups/next-hop-group/config/name:
 
  #TODO: Add new OC for GRE encap headers
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/config/index:
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/config/next-hop:
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/config/index:
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/type:          
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/dst-ip:
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/src-ip:
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/dscp:
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/ip-ttl:
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/index:
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/mpls-label-stack:

  #TODO: Add new OC for policy forwarding actions
  #/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/next-hop-group:   
  #/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/set-ttl:   
  #/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/set-hop-limit:  
  #/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/packet-type:    
  #/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/count:     

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

FFF