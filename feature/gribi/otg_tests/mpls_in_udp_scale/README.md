# TE-18.3 MPLS in UDP Encapsulation with QoS Scheduler Scale Test

Building on TE-18.1 and TE-18.2, add scaling parameters

## Topology

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

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
  * 1,000 vlans
  * Inner IP address space should be reused for each network-instance.
  * gRIBI client update rate `flow_r` = 1 update per second
  * Each gRIBI update include ip entries in batches of `flow_q` = 200
  * DUT packet forwarding updated within 1 second after adding entries

* Scheduler (policer) scale
  * 1,000 policer rates
  * 20,000 token buckets / scheduler instantiations
  * Update schedulers at 1 per `sched_r` = 60 seconds
  * Update schdulers in a batch of `sched_q` = 1,000
  * Scheduler changes should take effect within `sched_r` / 2 time

#### Scale profile A - many vlans

* 20 ip destinations * 1,000 vlans = 20,000 'flows'
* Each ingress vlan has 10 policers = 10,000 'token buckets'
* The 20 ip destinations are split evenly between the 10 policers
* Each policer is assigned rate limits matching one of 800 different possible limits between 1Gbps to 400Gbps in 0.5Gbps increments

#### Scale profile B - many destinations, few vlans

* 200 ip destinations * 100 vlans = 20,000 'flows'
* Each ingress vlan has 4 policers = 4,000 'token buckets'
* The 200 ip destinations are split evenly between the 4 policers
* Each policer is assigned rate limits matching one of 800 different possible limits between 1Gbps to 400Gbps in 0.5Gbps increments

#### Procedure - Flow Scale

* For each scale profile, create the following subsets TE-18.1.5.n
  * Configure ATE flows to send 100 pps per flow and wait for ARP
  * Send traffic for q flows (destination IP prefixes) for 2 seconds
  * At traffic start time, gRIBI client to send `flow_q` aft entries and their
    related NHG and NH at rate `flow_r`
  * Validate RIB_AND_FIB_ACK with FIB_PROGRAMMED is received from DUT within
    1 second
  * Measure packet loss.  Target packet loss <= 50%.
  * Repeat adding 200 flows until 20,000 flows have been added
  * Once reaching 20,000 flows, perform 1 iteration of modifying the first
    `flow_q` flows to use different NH,NHG

#### Procedure - Policer + Flow Scale

* For each scale profile, create the following subsets TE-18.1.6.n
  * Program all 20,000 flows
  * Every `sched_r` interval use gnmi.Set to replace `sched_q` scheduler policies
  * Verify packet loss changes for all flows within `sched_r` / 2 time

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
