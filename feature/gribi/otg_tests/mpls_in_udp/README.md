# TE-18.1 MPLS in UDP Encapsulation with QoS scheduler

Create AFT entries using gRIBI to match destination IP address in a
network-instance.  Encapsulate the matching packets in MPLS in UDP. Configure
a qos scheduler to rate limit / police traffic based on matching the
destination IP address of a packet input to the DUT.

The MPLS in UDP encapsulation is expected to follow [rfc7510](https://datatracker.ietf.org/doc/html/rfc7510#section-3),
but relaxing the requirement for a well-known destination UDP port.  gRIBI is
expected to be able to set the destination UDP port.

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

### TE-18.1.1 Match and Encapsulate using gRIBI aft modify

#### gRIBI RPC content

The gRIBI client should send this proto message to the DUT to create AFT
entries.  See [OC PR in progress](https://github.com/openconfig/public/pull/1153)
for the new OC AFT model nodes needed for this.  The
[gRIBI v1 protobuf defintions](https://github.com/openconfig/gribi/blob/master/v1/proto/README.md)
will be generated from the afts tree.

TODO: Update gRIBI protobuf.

```proto
network_instances: {
  network_instance: {
    afts {
      #
      # entries used for "group_A"
      ipv6_unicast {
        ipv6_entry {
          prefix: "inner_ipv6_dst_A"   # this is an IPv6 entry for the origin/inner packet.
          next_hop_group: 100
        }
      }
      ipv4_unicast {
        ipv4_entry {
          prefix: "ipv4_inner_dst_A"   # this is an IPv4 entry for the origin/inner packet.
          next_hop_group: 100
        }
      }
      next_hop_groups {
        next_hop_group {
          next_hop_group_name: "nhg_A"  # OC model says it's system assigned for telemetry, we want to let gRIBI assign this.
          id: 100
          next_hops {            # reference to a next-hop
            next_hop: {
              index: 100
            }
          }
        }
      }
      next_hops {
        next_hop {
          index: 100
          network_instance: "group_A"
          encapsulate_header: OPENCONFIG_AFT_TYPES:MPLS_IN_UDPV6
          mpls_in_udp {
            src_ip: "outer_ipv6_src"
            dst_ip: "outer_ipv6_dst_A"
            pushed_mpls_label_stack: [100,]
            dst_udp_port: "outer_dst_udp_port"
            ip_ttl: "outer_ip-ttl"
            dscp: "outer_dscp"
          }
        }
      }
      #
      # entries used for "group_B"
      ipv6_unicast {
        ipv6_entry {
          prefix: "inner_ipv6_dst_B"
          next_hop_group: 200
        }
      }
      ipv4_unicast {
        ipv4_entry {
          prefix: "ipv4_inner_dst_B"
          next_hop_group: 200
        }
      }
      next_hop_groups {
        next_hop_group {
          next_hop_group_name: "nhg_A"  # OC model says it's system assigned for telemetry, we want to let gRIBI assign this.
          id: 200
          next_hops {            # reference to a next-hop
            next_hop: {
              index: 200
            }
          }
        }
      }
      next_hops {
        next_hop {
          index: 200
          network_instance: "group_B"
          encapsulate_header: OPENCONFIG_AFT_TYPES:MPLS_IN_UDPV6
          mpls_in_udp {
            src_ip: "outer_ipv6_src"
            dst_ip: "outer_ipv6_dst_B"
            pushed_mpls_label_stack: [200,]
            dst_udp_port: "outer_dst_udp_port"
            ip_ttl: "outer_ip-ttl"
            dscp: "outer_dscp"
          }
        }
      }
    }
  }
}
```

* Send traffic from ATE port 1 to DUT port 1
* Validate afts next hop counters
* Using OTG, validate ATE port 2 receives MPLS-IN-UDP packets
  * Validate destination IPs are outer_ipv6_dst_A and outer_ipv6_dst_B
  * Validate MPLS label is set

### TE-18.1.2 Validate prefix match rule for MPLS in UDP encap using default route

Canonical OpenConfig for policy forwarding, matching IP prefix with action
encapsulate in GRE.

```yaml
openconfig-network-instance:
  network-instances:
    - network-instance: "group_A"
      afts:
        policy-forwarding:
          policies:
            policy: "default encap rule"
              config: 
                policy-id: "default encap rule"
                type: PBR_POLICY
              rules:
                rule: 1
                  config:
                    sequence-id: 1
                  ipv6:
                    config:
                      destination-address: "inner_ipv6_default"
                  action:
                    # TODO: add to OC model/PR in progress
                    encapsulate-mpls-in-gre:
                      targets:
                        target: "default_dst_1"
                          config:
                            id: "default_dst_1"
                            network-instance: "DEFAULT"
                            source-ip: "outer_ipv6_src"
                            destination-ip: "outer_ipv6_dst_def"
                            ip-ttl: outer_ip-ttl
                            dscp: outer_dscp
```

* Generate the policy forwarding configuration
* Push the configuration to DUT using gnmi.Set with REPLACE option
* Configure ATE port 1 with traffic flow which does not match any AFT next hop route
* Generate traffic from ATE port 1 to ATE port 2
* Validate ATE port 2 receives GRE traffic with correct inner and outer IPs

### TE-18.1.3 Policer attached to interface via gNMI

* Generate config for 2 scheduler polices with an input rate limit.  Apply
  to DUT port 1.
* Generate config for 2 classifiers which match on next-hop-group.
* Generate config for 2 input policies which map the scheduler and classifers
  together.
* Generate config for applying the input policies to a vlan.
* Use gnmi.Replace to push the config to the DUT.

```yaml
---
openconfig-qos:
  scheduler-policies:
    - scheduler-policy: "limit_2Gb"
      config:
        name: "limit_2Gb"
      schedulers:
        - scheduler: 0
          config:
              type: ONE_RATE_TWO_COLOR
              sequence: 0
          one-rate-two-color:
            config:
              cir: 2000000000           # 2Gbit/sec
              bc: 100000                 # 100 kilobytes
              queuing-behavior: POLICE
            exceed-action:
              config:
                drop: TRUE

    - scheduler-policy: "limit_1Gb"
      config:
        name: "limit_1Gb"
      schedulers:
        - scheduler: 0
          config:
              type: ONE_RATE_TWO_COLOR
              sequence: 0
          one-rate-two-color:
            config:
              cir: 1000000000           # 1Gbit/sec
              bc: 100000                # 100 kilobytes
              queuing-behavior: POLICE
            exceed-action:
              config:
                drop: TRUE
  classifers:
    - classifer: “dest_A”
      config:
        name: “dest_A”
      terms:
        - term:   # repeated for address in destination A
          config:
            id: "match_1_dest_A"
          conditions:
            next-hop-group:
                config:
                    name: "nhg_A"     # new OC path needed, string related to /afts/next-hop-groups/next-hop-group/state/next-hop-group-name
    - classifer: “dest_B”
      config:
        name: “dest_B”
      terms:
        - term:
          config:
            id: "match_1_dest_B"
          conditions:
            next-hop-group:
                config:
                    name: "nhg_B"     # new OC path needed, string related to /afts/next-hop-groups/next-hop-group/state/next-hop-group-name

  input-policies:       # new OC subtree input-policies (/qos/input-policies)
    - input-policy: "limit_group_A_2Gb"
      config:
        name: "limit_group_A_2Gb"
        classifer: "dest_A"
        scheduler-policy: "limit_2Gb"
    - input-policy: "limit_dest_group_B_1Gb"
      config:
        name: "limit_dest_group_B_1Gb"
        classifer: "dest_B"
        scheduler-policy: "limit_1Gb"

  interfaces:                  # this is repeated per subinterface (vlan)
    - interface: "PortChannel1"
      interface-ref:
        config:
          subinterface: 100
    input:
      config:
        policies:  [            # new OC leaf-list (/qos/interfaces/interface/input/config/policies)
          limit_dest_group_A_2Gb
        ]
  interfaces:                  # this is repeated per subinterface (vlan)
    - interface: "PortChannel1"
      interface-ref:
        config:
          subinterface: 200
    input:
      config:
        policies:  [            # new OC leaf-list (/qos/interfaces/interface/input/config/policies)
          limit_dest_group_B_1Gb
        ]

```

* Send traffic
  * Send traffic from ATE port 1 to DUT for dest_A and is conforming to cir.
  * Send traffic from ATE port 1 to DUT for to dest_B and is conforming to
    cir.
  * Validate packets are received by ATE port 2.
    * Validate qos interface scheduler counters
    * Validate afts next hop counters
  * Validate outer packet ipv6 flow label assignment
    * When the outer packet is IPv6, the flow-label should be inspected on the ATE.
      * If the inner packet is IPv4, the outer IPv6 flow label should be computed based on the IPv4 5 tuple src,dst address and ports, plus protocol
      * If the inner packet is IPv6, the inner flow label should be copied to the outer packet.
  * Increase traffic on flow to dest_B to 2Gbps
    * Validate that flow dest_B experiences ~50% packet loss (+/- 1%)

### TE-18.1.4 - Decapsulation set by gRIBI

This gRIBI content is used to perform MPLS in UDP decapsulation.

```proto
network_instances {
  network_instance {
    afts {
      ipv6_unicast {
        ipv6_entry {
          prefix: "outer_loopback_ipv6"   # IPv6 match rule for the device loopback expected to receive MPLS in UDP packets
          next-hop-group: 999
        }
      }
      ipv4_unicast {
        ipv4_entry {
          prefix: "outer_loopback_ipv4"
          next-hop-group: 999
        }
      }
      next_hop_groups {
        next_hop_group {
          next_hop_group_name: "Decap"  # OC model says it's system assigned for telemetry, should we let gRIBI assign this?
          id: 999
          next_hops {
            next_hop: {
              index: 990
            }
          }
        }
      }
      next_hops {
        next_hop {
          index: 990
          decapsulate_header: OPENCONFIG_AFT_TYPES:MPLS_IN_UDPV6      # TODO: Add OC path for this
          # The device should decapsulate the UDP packet and use the MPLS header
          # to determine the destination network-instance, then forward the packet
          # based on the inner IP packet, matching an appropriate AFT entry.
        }
      }
    }
  }
}
```

### TE-18.1.5 Scale

TODO: Move to separate README

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
