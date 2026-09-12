# DP-2.2: Policer-group with 1 rate 2 color policer, classifying on next-hop group

## Summary

Use the gRIBI applied IP entries from DP-2.1 gRIBI. Configure a policer-group
to police traffic using a 1 rate, 2 color policer. Configure a policy-forwarding rule to match
traffic on a next-hop-group.  Apply the configuration to a VLAN on an aggregate
interface.  Send traffic to validate the policer.

## Topology

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Test setup

Use TE-18.1 test environment setup.

## Procedure

### DP-2.2.1 Generate and push policer configuration

* Generate config for 2 policer-groups in the `/qos` tree to define the rate and burst.
* Generate config for 2 policy-forwarding rules (MATCH_ACTION_POLICY) which match on next-hop-group.
* Apply the `set-policer` action in the policy-forwarding rules to reference the policer-groups.
* Apply the policy-forwarding policy to the DUT subinterface.
* Use gnmi.Replace to push the config to the DUT.

### DP-2.2.2 push gRIBI AFT encapsulation rules with next-hop-group-id

Create a gRIBI client and send this proto message to the DUT to create AFT
entries.  Note the next-hop-groups here include a `next_hop_group_id` field
which matches the
`/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/config/next-hop-group-name` leaf.

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
    * Validate DUT qos policer-group counters count packets as conforming-pkts and conforming-octets
    * Validate at OTG that 0 packets are lost on flow A and flow B
  * Increase traffic on flow to dest_A to 2Gbps
    * Validate that flow dest_A experiences ~50% packet loss (+/- 1%)
    * Validate DUT qos policer-group counters count packets as exceeding-pkts and exceeding-octets
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
TODO: The following OC relies on the pending `go/oc-policer-group` schema (introducing shared policer actions and QoS buckets) which is not yet merged to the OpenConfig data models.

```json
{
  "openconfig-qos:qos": {
    "policer-groups": {
      "policer-group": [
        {
          "name": "group-policer-A",
          "config": {
            "name": "group-policer-A"
          },
          "one-rate-two-color": {
            "config": {
              "cir": 1000000000,
              "bc": 268435456
            },
            "exceed-action": {
              "config": {
                "drop": true
              }
            }
          }
        },
        {
          "name": "group-policer-B",
          "config": {
            "name": "group-policer-B"
          },
          "one-rate-two-color": {
            "config": {
              "cir": 2000000000,
              "bc": 268435456
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
  },
  "openconfig-network-instance:network-instances": {
    "network-instance": [
      {
        "name": "DEFAULT",
        "policy-forwarding": {
          "policies": {
            "policy": [
              {
                "policy-id": "pbr_cloud_id_1",
                "config": {
                  "policy-id": "pbr_cloud_id_1",
                  "type": "MATCH_ACTION_POLICY"
                },
                "rules": {
                  "rule": [
                    {
                      "sequence-id": 10,
                      "config": {
                        "sequence-id": 10,
                        "address-family": "openconfig-types:IPV4",
                        "next-hop-group-name": "nhg_A"
                      },
                      "action": {
                        "config": {
                          "policer-group": "group-policer-A"
                        }
                      }
                    },
                    {
                      "sequence-id": 20,
                      "config": {
                        "sequence-id": 20,
                        "address-family": "openconfig-types:IPV4",
                        "next-hop-group-name": "nhg_B"
                      },
                      "action": {
                        "config": {
                          "policer-group": "group-policer-B"
                        }
                      }
                    }
                  ]
                }
              }
            ]
          }
        }
      }
    ]
  }
}
```

#### OpenConfig Path and RPC Coverage

```yaml
paths:
  # qos policer-group config
  /qos/policer-groups/policer-group/config/name:
  /qos/policer-groups/policer-group/one-rate-two-color/config/cir:
  /qos/policer-groups/policer-group/one-rate-two-color/config/bc:
  /qos/policer-groups/policer-group/one-rate-two-color/exceed-action/config/drop:

  # qos policer-group state counters
  /qos/policer-groups/policer-group/state/conforming-pkts:
  /qos/policer-groups/policer-group/state/conforming-octets:
  /qos/policer-groups/policer-group/state/exceeding-pkts:
  /qos/policer-groups/policer-group/state/exceeding-octets:

  # policy-forwarding match & action config
  /network-instances/network-instance/policy-forwarding/policies/policy/config/type:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/config/next-hop-group-name:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/policer-group:

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
