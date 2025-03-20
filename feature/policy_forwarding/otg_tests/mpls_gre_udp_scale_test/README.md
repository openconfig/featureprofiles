# PF-1.12 MPLS in GRE and MPLS in UDP Scale Test

Building on TE-18.1 and PF-1.2, add scaling parameters

## Topology

* 2 ports as the 'input port set'
* 4 ports as "uplink facing"
* VRF configurations
  * input vlans are distributed evenly across the 'input port set'

## Test setup

TODO: Complete test environment setup steps

inner_ipv6_dst_A = "2001:aa:bb::1/128"
inner_ipv6_dst_B = "2001:aa:bb::2/128"
inner_ipv6_default = "::/0"

ipv4_inner_dst_A = "10.5.1.1/32"
ipv4_inner_dst_B = "10.5.1.2/32"
ipv4_inner_default = "0.0.0.0/0"

outer_ipv6_src =      "2001:f:a:1::0"
outer_ipv6_dst_A =    "2001:f:c:e::1"
outer_ipv6_dst_B =    "2001:f:c:e::2"
outer_ipv6_dst_def =  "2001:1:1:1::0"
outer_dst_udp_port =  "5555"
outer_dscp =          "26"
outer_ip-ttl =        "64"

## Procedure

### PF-1.12.1 Scale tests for MPLS in GRE and UDP

#### Scale targets

* Scale
  * 1000 VRF
  * 100k longest exact match route entries
  * Inner IP address space should be reused for each network-instance.

#### Scale profile A - 1000 VRF with different policy forwarding NHG

* 1000 sub interfaces.
* 1000 VRF have 1000 policy forwarding NHG with 8 NH
* 1 default route in each VRF pointing to policy forwarding NHG.
* Create 20K gRIBI NHG with 1 NH per NHG with different MPLS label.
* 100K longest exact match route entries with 5 pointing to 1 gRIBI NHG.

#### Scale profile B - 1000 VRF share same policy forwarding NHG
* 1000 sub interfaces.
* 1000 VRF have 1 policy forwarding NHG with 8 NH.
* 1 default route in each VRF pointing to 1 policy forwarding NHG.
* Create 20K gRIBI NHG with 1 NH per NHG with same MPLS label.
* 100K longest exact match route entries with 5 pointint to 1 gRIBI NHG.


#### Procedure - VRF Scale

* For each scale profile, create the following subsets PF-1.12.1
  * Generate traffic stream for policy forwarding NHG and gRIBI NHG
  * Observe that MPLS over GRE and MPLS over UDP encapsulation are working properly.


#### OpenConfig Path and RPC Coverage

```yaml
paths:
 
  # afts state paths set via gRIBI
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

  # Policy forwarding paths
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/network-instance:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/config/sequence-id:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encap-headers/encap-header/mpls/config/mpls-label-stack:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encap-headers/encap-header/gre/config/destination-ip:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encap-headers/encap-header/gre/config/dscp:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encap-headers/encap-header/gre/config/id:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encap-headers/encap-header/gre/config/ip-ttl:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encap-headers/encap-header/gre/config/source-ip:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
  gribi:
    gRIBI.Modify:
    gRIBI.Flush:
```

## Required DUT platform

* FFF