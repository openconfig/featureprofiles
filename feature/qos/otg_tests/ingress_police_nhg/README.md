# DP-2.2 QoS scheduler with 1 rate 2 color policer, classifying on next-hop group

## Summary

Use the gRIBI applied IP entries from DP-2.1 gRIBI. Configure an ingress scheduler
to police traffic using a 1 rate, 2 color policer. Configure a classifier to match
traffic on a next-hop-group.  Apply the configuration to a VLAN on an aggregate
interface.  Send traffic to validate the policer.

## Topology

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Test setup

Use DP-2.1 test environment setup.

## Procedure

### DP-2.2.1 Generate and push policer configuration

* Generate config for 2 classifiers which match on next-hop-group.
* Generate config for 2 forwarding-groups mapped to "dummy" input queues
  * Note that the DUT is not required to have an input queue, the dummy queue
    satisfies the OC schema which requires defining nodes mapping
    classfier->forwarding-group->queue->scheduler
* Generate config for 2 scheduler-policies to police traffic
* Generate config to apply classifer and scheduler to DUT subinterface.  (TODO: include interface config details with 802.1Q tags)
* Use gnmi.Replace to push the config to the DUT.

```json
{
  #
  # A classifer is created to match packets belonging to certain
  # next-hop-groups and map them either forwarding-group input_dest_A or
  # input_dest_B
  "openconfig-qos": {
    "classifers": [
      {
        "classifer": "“dest_A”",
        "config": {
          "name": "“dest_A”"
        },
        "terms": [
          {
            "term": null,
            "config": {
              "id": "match_1_dest_A1"
            },
            "conditions": {
              "next-hop-group": {
                "config": {
                  # new OC path needed, string related to /afts/next-hop-groups/next-hop-group/state/next-hop-group-id
                  "name": "nhg_A1"
                }
              }
            },
            "actions": {
              "config": {
                "target-group": "input_dest_A"
              }
            }
          },
          {
            "term": null,
            "config": {
              "id": "match_1_dest_A2"
            },
            "conditions": {
              "next-hop-group": {
                "config": {
                  "name": "nhg_A2"
                }
              }
            },
            "actions": {
              "config": {
                "target-group": "input_dest_A"
              }
            }
          }
        ]
      },
      {
        "classifer": "“dest_B”",
        "config": {
          "name": "“dest_B”"
        },
        "terms": [
          {
            "term": null,
            "config": {
              "id": "match_1_dest_B1"
            },
            "conditions": {
              "next-hop-group": {
                "config": {
                  "name": "nhg_B1"
                }
              }
            },
            "actions": {
              "config": {
                "target-group": "input_dest_B"
              }
            }
          },
          {
            "term": null,
            "config": {
              "id": "match_1_dest_B2"
            },
            "conditions": {
              "next-hop-group": {
                "config": {
                  "name": "nhg_B2"
                }
              }
            },
            "actions": {
              "config": {
                "target-group": "input_dest_B"
              }
            }
          }
        ]
      }
    ],
    #
    # Forwarding groups are created named input_dest_A and input_dest_B.
    # These are mapped to 'fake' queues 
    "forwarding-groups": [
      {
        "forwarding-group": "input_dest_A",
        "config": {
          "name": "input_dest_A",
          "output-queue": "dummy_input_queue_A"
        }
      },
      {
        "forwarding-group": "input_dest_B",
        "config": {
          "name": "input_dest_B",
          "output-queue": "dummy_input_queue_B"
        }
      }
    ],
    "queues": [
      {
        "queue": null,
        "config": {
          "name": "dummy_input_queue_A"
        }
      },
      {
        "queue": null,
        "config": {
          "name": "dummy_input_queue_B"
        }
      }
    ],
    #
    # Two scheduler policies are created, limit_1Gb and limit_2Gb
    # and are associated with the dummy queue they are servicing.
    "scheduler-policies": [
      {
        "scheduler-policy": null,
        "config": {
          "name": "limit_1Gb"
        },
        "schedulers": [
          {
            "scheduler": null,
            "config": {
              "sequence": 1,
              "type": "ONE_RATE_TWO_COLOR"
            },
            "inputs": [
              {
                "input": "my input policer 1Gb",
                "config": {
                  "id": "my input policer 1Gb",
                  "input-type": "QUEUE",
                  "queue": "dummy_input_queue_A"
                }
              }
            ],
            "one-rate-two-color": {
              "config": {
                "cir": 1000000000,
                "bc": 100000,
                "queuing-behavior": "POLICE"
              },
              "exceed-action": {
                "config": {
                  "drop": true
                }
              }
            }
          }
        ]
      },
      {
        "scheduler-policy": null,
        "config": {
          "name": "limit_2Gb"
        },
        "schedulers": [
          {
            "scheduler": null,
            "config": {
              "sequence": 1,
              "type": "ONE_RATE_TWO_COLOR"
            },
            "inputs": [
              {
                "input": "my input policer 2Gb",
                "config": {
                  "id": "my input policer 2Gb",
                  "input-type": "QUEUE",
                  "queue": "dummy_input_queue_B"
                }
              }
            ],
            "one-rate-two-color": {
              "config": {
                "cir": 2000000000,
                "bc": 100000,
                "queuing-behavior": "POLICE"
              },
              "exceed-action": {
                "config": {
                  "drop": true
                }
              }
            }
          }
        ]
      }
    ],
    #
    # Interfaces input are mapped to the desired classifier and scheduler.
    "interfaces": [
      {
        "interface": null,
        "config": {
          "interface-id": "PortChannel1.100"
        },
        "input": {
          "classifiers": [
            {
              "classifier": null,
              "config": {
                "name": "dest_A",
                "type": "IPV4"
              }
            }
          ],
          "scheduler-policy": {
            "config": {
              "name": "limit_group_A_1Gb"
            }
          }
        }
      },
      {
        "interface": null,
        "config": {
          "interface-id": "PortChannel1.200"
        },
        "input": {
          "classifiers": [
            {
              "classifier": null,
              "config": {
                "name": "dest_B",
                "type": "IPV4"
              }
            }
          ],
          "scheduler-policy": {
            "config": {
              "name": "limit_group_B_1Gb"
            }
          }
        }
      }
    ]
  }
}
```

### DP-2.2.2 push gRIBI AFT encapsulation rules with next-hop-group-id

Create a gRIBI client and send this proto message to the DUT to create AFT
entries.  Note the next-hop-groups here include a `next_hop_group_id` field
which matches the
`/qos/classifiers/classifier/condition/next-hop-group/config/name` leaf.

* [TODO: OC AFT Encap PR in progress](https://github.com/openconfig/public/pull/1153)
* [TODO: gRIBI v1 protobuf defintions](https://github.com/openconfig/gribi/blob/master/v1/proto/README.md)

```proto
#
# aft entries used for network instance "NI_A"
IPv6Entry {2001:DB8:2::2/128 (NI_A)} -> NHG#100 (DEFAULT VRF)
IPv4Entry {203.0.113.2/32 (NI_A)} -> NHG#100 (DEFAULT VRF) -> {
  {NH#101, DEFAULT VRF}
}

# this nexthop specifies a MPLS in UDP encapsulation
NH#101 -> {
  encap-headers {
    encap-header {
      index: 1
      mpls {
        pushed_mpls_label_stack: [101,]
      }
    }
    encap-header {
      index: 2
      udpv6 {
        src_ip: "outer_ipv6_src"
        dst_ip: "outer_ipv6_dst_A"
        dst_udp_port: "outer_dst_udp_port"
        ip_ttl: "outer_ip-ttl"
        dscp: "outer_dscp"
      }
    }
  }
  next_hop_group_id: "nhg_A"  # new OC path /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/
  network_instance: "DEFAULT"
}

#
# entries used for network-instance "NI_B"
IPv6Entry {2001:DB8:2::2/128 (NI_B)} -> NHG#200 (DEFAULT VRF)
IPv4Entry {203.0.113.2/32 (NI_B)} -> NHG#200 (DEFAULT VRF) -> {
  {NH#201, DEFAULT VRF}
}

NH#201 -> {
  encap-headers {
    encap-header {
      index: 1
      mpls {
        pushed_mpls_label_stack: [201,]
      }
    }
    encap-header {
      index: 2
      udpv6 {
        src_ip: "outer_ipv6_src"
        dst_ip: "outer_ipv6_dst_B"
        dst_udp_port: "outer_dst_udp_port"
        ip_ttl: "outer_ip-ttl"
        dscp: "outer_dscp"
      }
    }
  }
  next_hop_group_id: "nhg_B"  # new OC path /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/
  network_instance: "DEFAULT"
}
```

### DP-2.2.3 Test flow policing

* Send traffic
  * Send flow A traffic from ATE port 1 to DUT for dest_A at 0.7Gbps (note cir is 1Gbps).
  * Send flow B traffic from ATE port 1 to DUT for to dest_B at 1.5Gbps (note cir is 2Gbps).
  * Validate packets are received by ATE port 2.
    * Validate DUT qos interface scheduler counters count packets as conforming-pkts and conforming-octets
    * Validate at OTG that 0 packets are lost on flow A and flow B
  * Increase traffic on flow to dest_A to 2Gbps
    * Validate that flow dest_A experiences ~50% packet loss (+/- 1%)
  * Stop traffic

### DP-2.2.3 IPv6 flow label validiation

  * Send 100 packets for flow A and flow B.  (Use an OTG fixed packet count flow)
  * When the outer packet is IPv6, the flow-label should be inspected on the ATE.
    * If the inner packet is IPv4, the outer IPv6 flow label should be computed based on the IPv4 5 tuple src,dst address and ports, plus protocol.
    * If the inner packet is IPv6, the inner flow label should be copied to the outer packet.
    * To validate the flow label, use the ATE to verify that the packets for 
      * flow A all have the same flow label
      * flow B have the same flow label
      * flow A and B labels do not match

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

  # qos forwarding-groups config
  /qos/forwarding-groups/forwarding-group/config/name:
  /qos/forwarding-groups/forwarding-group/config/output-queue:

  # qos queue config
  /qos/queues/queue/config/name:

  # qos interfaces config
  /qos/interfaces/interface/config/interface-id:
  /qos/interfaces/interface/input/classifiers/classifier/config/name:
  /qos/interfaces/interface/input/classifiers/classifier/config/type:
  /qos/interfaces/interface/input/scheduler-policy/config/name:

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
