TE-18.4 - gRIBI Flush operation with Live traffic

## Summary
This test verifies when gRIBI flush operation is issued all the gRIBI rules are removed leaving the static routes intact.
Ongoing traffic flows using gRIBI rules will switch to static route.

## Topology

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Test setup

Refer TE-18.1 for test setup.

## Procedure

### TE-18.4.1 Perform gRIBI flush operation to verify traffic switches to backup prefix match rule.

#### gRIBI RPC content

The gRIBI client should send TE-18.1 proto message to the DUT to create AFT
entries.

* Configure static-routes as mentioned at PF-1.14 with next-hops using encap-headers as a backup
  prefix match rule for MPLS in GRE encap.
* Push the configuration to DUT using gnmi.Set with REPLACE option
* Configure ATE port 1 with traffic flow which matches AFT next hop route
* Generate traffic from ATE port 1 to ATE port 2
* Send traffic from ATE port 1 to DUT port 1
* Using OTG, validate ATE port 2 receives MPLS-IN-UDP packets
  * Validate destination IPs are outer_ipv6_dst_A and outer_ipv6_dst_B
  * Validate MPLS label is set
  * Perform gRIBI flush operation.
  * Verify AFT entries are removed on DUT.
  * Validate ATE port 2 receives GRE traffic with correct inner and outer IPs

## OpenConfig Path and RPC Coverage

```yaml
paths:

# afts state paths set via gRIBI
  # Paths added for TE-18.4.1 Match and Encapsulate using gRIBI aft modify
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/state/type:
  
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/mpls/state/mpls-label-stack:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/udp-v4/state/src-ip:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/udp-v4/state/dst-ip:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/udp-v4/state/dst-udp-port:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/udp-v4/state/ip-ttl:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/udp-v4/state/dscp:

  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/udp-v6/state/src-ip:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/udp-v6/state/dst-ip:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/udp-v6/state/dst-udp-port:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/udp-v6/state/ip-ttl:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/udp-v6/state/dscp:

  # Paths added for prefix match rule for back up static route
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/network-instance:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/config/sequence-id:
  #/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encap-headers/encap-header/gre/config/destination-ip:
  #/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encap-headers/encap-header/gre/config/id:
  #/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encap-headers/encap-header/gre/config/source-ip:
  #/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encap-headers/encap-header/mpls/config/mpls-label-stack:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
  gribi:
    gRIBI.Modify:
      afts:next-hops:next-hop:encap-headers:encap-header:udp_v6:
      afts:next-hops:next-hop:encap-headers:encap-header:mpls:
    gRIBI.Flush:
```

## Required DUT platform

* FFF

