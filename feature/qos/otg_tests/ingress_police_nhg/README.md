# TE-18.2 QoS scheduler with 1 rate 2 color policer, classifying on next-hop group

## Summary

Use the gRIBI applied ip entries from TE-18.1 gRIBI. Configure an ingress scheduler
to police traffic using a 1 rate, 2 color policer. Configure a classifier to match
traffic on a next-hop-group.  Apply the configuration to a VLAN on an aggregate
interface.  Send traffic to validate the policer.

## Topology

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Test setup

Use TE-18.1 test environment setup.

## Procedure

### TE-18.2.1 Generate and push configuration

* Generate config for 2 scheduler polices with an input rate limit.  
* Generate config for 2 classifiers which match on next-hop-group.
* Generate config for 2 input policies which map the scheduler and classifers
  together.
* Generate config to apply classifer and scheduler to DUT subinterface with vlan.
* Use gnmi.Replace to push the config to the DUT.

```yaml
---
openconfig-qos:
  policer-policies:
    - policer-policy: "limit_2Gb"
      config:
        name: "limit_2Gb"
      policers:
        - policer: 0
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

    - policer-policy: "limit_1Gb"
      config:
        name: "limit_1Gb"
      policers:
        - policer: 0
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
                    name: "nhg_A"     # new OC path needed, string related to /afts/next-hop-groups/next-hop-group/state/next-hop-group-id
          actions:
            config:
              policer-policy: "limit_group_A_2Gb"  # new OC path needed
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
                    name: "nhg_B"     # new OC path needed, string related to /afts/next-hop-groups/next-hop-group/state/next-hop-group-id
          actions:
            config:
              policer-policy: "limit_group_B_1Gb"  # new OC path needed

  interfaces:                  # this is repeated per subinterface (vlan)
    - interface: "PortChannel1.100"
        config:
          interface-id: "PortChannel1.100"
        input:
          classifers:
            - classifier:
              config:
                name: "dest_A"
                type: "IPV4"
          policer-policy:     # New OC subtree /qos/interfaces/interface/policer-policy
            config:
              name: "limit_2G"
    - interface: "PortChannel1.200"
        config:
          interface-id: "PortChannel1.200"
        input:
          classifers:
            - classifier:
              config:
                name: "dest_B"
                type: "IPV4"
          policer-policy:    # New OC subtree /qos/interfaces/interface/policer-policy
            config:
              name: "limit_1G"
```

### TE-18.2.2 push gRIBI aft encap rules with next-hop-group-id

Create a gRIBI client and send this proto message to the DUT to create AFT
entries.  Note the next-hop-groups here include a `next_hop_group_id` field
which matches the
`/qos/classifiers/classifier/condition/next-hop-group/config/name` leaf.

* [TODO: OC AFT Encap PR in progress](https://github.com/openconfig/public/pull/1153)
* [TODO: gRIBI v1 protobuf defintions](https://github.com/openconfig/gribi/blob/master/v1/proto/README.md)

```proto
network_instances: {
  network_instance: {
    afts {
      #
      # entries used for "group_A"
      ipv6_unicast {
        ipv6_entry {
          prefix: "inner_ipv6_dst_A"   # this is an IPv6 entry for the origin/inner packet
          next_hop_group: 100
        }
      }
      ipv4_unicast {
        ipv4_entry {
          prefix: "ipv4_inner_dst_A"   # this is an IPv4 entry for the origin/inner packet
          next_hop_group: 100
        }
      }
      next_hop_groups {
        next_hop_group {
          id: 100
          next_hop_group_id: "nhg_A"  # new OC path /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/next-hop-group-id
          next_hops {                 # reference to a next-hop
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
          encap-headers {
            encap-header {
              index: 1
              pushed_mpls_label_stack: [100,]
            }
          }
          encap-headers {
            encap-header {
              index: 2
              src_ip: "outer_ipv6_src"
              dst_ip: "outer_ipv6_dst_A"
              dst_udp_port: "outer_dst_udp_port"
              ip_ttl: "outer_ip-ttl"
              dscp: "outer_dscp"
            }
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
          id: 200
          next_hop_group_id: "nhg_B"  # new OC path /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/next-hop-group-id
          next_hops {                 # reference to a next-hop
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
          encap-headers {
            encap-header {
              index: 1
              type : OPENCONFIG_AFT_TYPES:MPLS
              mpls {
                pushed_mpls_label_stack: [200,]
              }
            }
          }
          encap-headers {
            encap-header {
              index: 2
              type: OPENCONFIG_AFT_TYPES:UDP
              udp {
                src_ip: "outer_ipv6_src"
                dst_ip: "outer_ipv6_dst_B"
                dst_udp_port: "outer_dst_udp_port"
                ip_ttl: "outer_ip-ttl"
                dscp: "outer_dscp"
              }
            }
          }
        }
      }
    }
  }
}
```


### TE-18.2.3 Test traffic

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

  # qos policer config - TODO: a new OC subtree (/qos/policer-policies, essentially copying/moving policer action from schedulers)
  # /qos/policer-policies/policer-policy/config/name:
  # /qos/policer-policies/policer-policy/config/policers/policer/config/sequence:
  # /qos/policer-policies/policer-policy/config/policers/policer/one-rate-two-color/config/cir:
  # /qos/policer-policies/policer-policy/config/policers/policer/one-rate-two-color/config/bc:
  # /qos/policer-policies/policer-policy/config/policers/policer/one-rate-two-color/config/cir:
  # /qos/policer-policies/policer-policy/config/policers/policer/one-rate-two-color/exceed-action/config/drop:

  # qos interface scheduler counters
  /qos/interfaces/interface/input/scheduler-policy/schedulers/scheduler/state/conforming-pkts:
  /qos/interfaces/interface/input/scheduler-policy/schedulers/scheduler/state/conforming-octets:
  /qos/interfaces/interface/input/scheduler-policy/schedulers/scheduler/state/exceeding-pkts:
  /qos/interfaces/interface/input/scheduler-policy/schedulers/scheduler/state/exceeding-octets:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

* FFF

