# DP-2.2: QoS scheduler with 1 rate 2 color policer, classifying on next-hop group

## Summary

Use the gRIBI applied IP entries from DP-2.1 gRIBI. Configure an ingress scheduler
to police traffic using a 1 rate, 2 color policer. Configure a classifier to match
traffic on a next-hop-group.  Apply the configuration to a VLAN on an aggregate
interface.  Send traffic to validate the policer.

## Topology

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Test setup

Use TE-18.1 test environment setup.

## Procedure

### DP-2.2.1 Generate and push policer configuration

* Generate config for 2 classifiers which match on next-hop-group.
* Generate config for 2 forwarding-groups mapped to "dummy" input queues
  * Note that the DUT is not required to have an input queue, the dummy queue
    satisfies the OC schema which requires defining nodes mapping
    classfier->forwarding-group->queue->scheduler
* Generate config for 2 scheduler-policies to police traffic
* Generate config to apply classifer and scheduler to DUT subinterface.
* Use gnmi.Replace to push the config to the DUT.

### DP-2.2.2 push gRIBI AFT encapsulation rules with next-hop-group-id

Create a gRIBI client and send this proto message to the DUT to create AFT
entries.  Note the next-hop-groups here include a `next_hop_group_id` field
which matches the
`/qos/classifiers/classifier/condition/next-hop-group/config/name` leaf.

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
  next_hop_group_id: "nhg_A"  # TODO: new OC path /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/next-hop-group-id
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
  next_hop_group_id: "nhg_B"  # TODO: new OC path /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/next-hop-group-id
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

### DP-2.2.4 IPv6 flow label validiation

  * Send 100 packets for flow A and flow B.  (Use an OTG fixed packet count flow)
  * When the outer packet is IPv6, the flow-label should be inspected on the ATE.
    * If the inner packet is IPv4, the outer IPv6 flow label should be computed based on the IPv4 5 tuple src,dst address and ports, plus protocol.
    * If the inner packet is IPv6, the inner flow label should be copied to the outer packet.
    * To validate the flow label, use the ATE to verify that the packets for 
      * flow A all have the same flow label
      * flow B have the same flow label
      * flow A and B labels do not match

# Canonical OC
TODO: The following OC is a placeholder and should not be implemented.  It is only included here as a another reference for the configuration items.  The Canonical OC will be redesigned to follow policy-forwarding `MATCH_ACTION` after https://github.com/openconfig/public/pull/1417/ is merged to the OpenConfig data models.

```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "Input Interface",
          "name": "port1"
        },
        "name": "port1"
      }
    ]
  },
  "qos": {
    "classifiers": {
      "classifier": [
        {
          "config": {
            "name": "match-dscp-to-police",
            "type": "IPV4"
          },
          "name": "match-dscp-to-police",
          "terms": {
            "term": [
              {
                "actions": {
                  "config": {
                    "target-group": "fg-policer"
                  }
                },
                "conditions": {
                  "ipv4": {
                    "config": {
                      "dscp": 34
                    }
                  }
                },
                "config": {
                  "id": "term1"
                },
                "id": "term1"
              }
            ]
          }
        }
      ]
    },
    "forwarding-groups": {
      "forwarding-group": [
        {
          "config": {
            "name": "fg-policer",
            "output-queue": "q-dummy"
          },
          "name": "fg-policer"
        }
      ]
    },
    "interfaces": {
      "interface": [
        {
          "config": {
            "interface-id": "port1"
          },
          "input": {
            "classifiers": {
              "classifier": [
                {
                  "config": {
                    "name": "match-dscp-to-police",
                    "type": "IPV4"
                  },
                  "type": "IPV4"
                }
              ]
            },
            "queues": {
              "queue": [
                {
                  "config": {
                    "name": "q-dummy"
                  },
                  "name": "q-dummy"
                }
              ]
            },
            "scheduler-policy": {
              "config": {
                "name": "policer"
              }
            }
          },
          "interface-id": "port1"
        }
      ]
    },
    "queues": {
      "queue": [
        {
          "config": {
            "name": "q-dummy"
          },
          "name": "q-dummy"
        }
      ]
    },
    "scheduler-policies": {
      "scheduler-policy": [
        {
          "config": {
            "name": "policer"
          },
          "name": "policer",
          "schedulers": {
            "scheduler": [
              {
                "config": {
                  "sequence": 1,
                  "type": "ONE_RATE_TWO_COLOR"
                },
                "inputs": {
                  "input": [
                    {
                      "config": {
                        "id": "in-policer",
                        "input-type": "QUEUE",
                        "queue": "q-dummy"
                      },
                      "id": "in-policer"
                    }
                  ]
                },
                "one-rate-two-color": {
                  "config": {
                    "bc": 1000000,
                    "cir": "1000000000",
                    "queuing-behavior": "POLICE"
                  },
                  "exceed-action": {
                    "config": {
                      "drop": true
                    }
                  }
                },
                "sequence": 1
              }
            ]
          }
        }
      ]
    }
  }
}
```

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
