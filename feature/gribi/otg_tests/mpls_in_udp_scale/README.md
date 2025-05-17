# TE-18.3 MPLS in UDP Encapsulation with QoS Scheduler Scale Test

Building on TE-18.1 and TE-18.2, add scaling parameters

## Topology

* 32 ports as the 'input port set'
* 4 ports as "uplink facing"
* VLAN configurations
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

### TE-18.3.1 Scale

#### Scale targets
* Flow scale
  * 20,000 IPv4/IPv6 destinations
  * 1,024 vlans
  * 1024 network instances
  * gRIBI client update rate `flow_r` = 100 updates (Modify requests) per second, 60 operations per update/
  * DUT packet forwarding updated within 1 second after adding entries (after FIB_PROGRAMMED response)

#### Scale profile A (one VRF) - 1 Network instance, 32 VLANs in the network instance, 20k gRIBI NHG, 20k gRIBI exact match prefixes, 1 NH per NHG, same MPLS label across 20k prefixes
* 1 network-instance (VRF) with 32 VLANs
* Create 20K gRIBI NHG with 1 NH per NHG with the same MPLS label. 
* Each NH has a unique dst IP to encapsulate with.
* Create 1 unique gRIBI exact match prefix which points to 1 NHG and repeat this 20k times, resulting in 20K total prefixes, pointing to 20k NHG.
* 1 MPLS label across all NHs.

#### Scale profile B (lots of VRFs)- 1000 Network instances, 1 VLAN per Network instance, 20k gRIBI exact match prefixes, 1000 MPLS labels, 20 NHGs sharing one MPLS label but each with a different outer encap IP.
* 1000 network instances (VRFs)
* 1 VLAN per VRF.
* Create 20K gRIBI NHG with 1 NH per NHG, 20 NHG per VRF. 1 MPLS label per VRF.
* Create 1 unique gRIBI exact match prefix which points to 1 NHG.
* Every NH has a unique src/dst IP

#### Scale profile C (Multiple NHs per NHG)- Scale Profile A, but 8 NH per NHG
* 1 network-instance (VRF)
* Create 2.5K gRIBI NHG. Each NHG has 8 NH per NHG with the same MPLS label.
* Each NH has a unique encap src/dst IP, so 8 IPs per NHG, and 20K IPs in total.
* Create 1 unique gRIBI exact match prefix which points to 1 NHG and repeat this 2.5k times, resulting in 2.5K total prefixes, pointing to 2.5k NHG.
* 1 MPLS label across all NHs.
* Expectation on NH unviability, is that one of the other 7 NHs within the same NHG will be chosen (without gRIBI client intervention). If all NHs become unviable, NHG action becomes a DROP. 
* If an unviable NH becomes viable again, it is re-included into the h/w ECMP set without gRIBI client intervention. 

#### Scale profile D - Scale Profile A + gRIBI RPC scaling tests
* Use Scale Profile A
* gRIBI client sends 60 AFT ops in a gRIBI Modify call (50% ADD and 50% DEL operations) at a rate of 100 qps. 
* gRIBI control plane server should be able to handle this update rate.
* Ensure traffic is not dropped due to control plane load.

#### Procedure - VRF Scale

* For each scale profile, create the following subsets TE-18.3.1.n
  * Generate traffic stream for policy forwarding NHG and gRIBI NHG
  * Observe that MPLS over GRE and MPLS over UDP encapsulation are working properly.

#### OpenConfig Path and RPC Coverage

```yaml
paths:
  # qos scheduler config
  /qos/scheduler-policies/scheduler-policy/config/name:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/one-rate-two-color/config/cir:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/one-rate-two-color/config/bc:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/one-rate-two-color/config/queuing-behavior:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/one-rate-two-color/exceed-action/config/drop:

  # qos classifier config
  /qos/classifiers/classifier/config/name:
  /qos/classifiers/classifier/terms/term/config/id:
  #/qos/classifiers/classifier/terms/term/conditions/next-hop-group/config/name: # TODO: new OC leaf to be added

  # qos input-policies config - TODO: a new OC subtree (/qos/input-policies)
  # /qos/input-policies/input-policy/config/name:
  # /qos/input-policies/input-policy/config/classifier:
  # /qos/input-policies/input-policy/config/scheduler-policy:

  # qos interface config
  #/qos/interfaces/interface/subinterface/input/config/policies:   # TODO:  new OC leaf-list (/qos/interfaces/interface/input/config/policies)

  # qos interface scheduler counters
  /qos/interfaces/interface/input/scheduler-policy/schedulers/scheduler/state/conforming-pkts:
  /qos/interfaces/interface/input/scheduler-policy/schedulers/scheduler/state/conforming-octets:
  /qos/interfaces/interface/input/scheduler-policy/schedulers/scheduler/state/exceeding-pkts:
  /qos/interfaces/interface/input/scheduler-policy/schedulers/scheduler/state/exceeding-octets:

  # afts next-hop counters
  /network-instances/network-instance/afts/next-hops/next-hop/state/counters/packets-forwarded:
  /network-instances/network-instance/afts/next-hops/next-hop/state/counters/octets-forwarded:

  # afts state paths set via gRIBI
  # TODO: https://github.com/openconfig/public/pull/1153
  #/network-instances/network-instance/afts/next-hops/next-hop/mpls-in-udp/state/src-ip:
  #/network-instances/network-instance/afts/next-hops/next-hop/mpls-in-udp/state/dst-ip:
  #/network-instances/network-instance/afts/next-hops/next-hop/mpls-in-udp/state/ip-ttl:
  #/network-instances/network-instance/afts/next-hops/next-hop/mpls-in-udp/state/dst-udp-port:
  #/network-instances/network-instance/afts/next-hops/next-hop/mpls-in-udp/state/dscp:

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
