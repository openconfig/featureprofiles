# PF-1.11 Policy forwarding rule to match IP and rewrite TTL

This test uses policy-forwarding to set the IP TTL.  It contains 2 scenarios as subtests:
1. Apply this policy alone on an ingress interface.
2. Apply this policy in combination with a gRIBI programmed next-hop which performs encapsulation and sets the TTL on the outer, encapsulation packet.

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
outer_dst_udp_port =  "6635"
outer_dscp =          "26"
outer_ip-ttl =        "64"

## Procedure

### TE-1.11.1 Rewrite the packet TTL = 1 if matching a specified destination IP.
**[TODO]** Test code needs to be implemented.

Canonical OpenConfig for policy forwarding, matching IP prefix and TTL = 1 with action
set packet TTL = 1.

```json
{
  "openconfig-network-instance": {
    "network-instances": [
      {
        "afts": {
          "policy-forwarding": {
            "policies": [
              {
                "config": {
                  "policy-id": "customer1_prefix4_retain ttl",
                  "type": "PBR_POLICY"
                },
                "policy": "retain ttl",
                "rules": [
                  {
                    "config": {
                      "sequence-id": 1,
                    },
                    "ipv4": {
                      "config": {
                        "destination-address": "ipv4_inner_dst_B"
                        "hop-limit": 1
                      }
                    },
                    "action": {
                      "next-hop-group": "cloud_v4_nhg",
                      "set-ttl": 1
                     }
                  }
                ]
              },
              {
                "config": {
                  "policy-id": "customer1_prefix6_retain ttl",
                  "type": "PBR_POLICY"
                },
                "policy": "retain ttl",
                "rules": [
                  {
                    "config": {
                      "sequence-id": 1,
                    },
                    "ipv6": {
                      "config": {
                        "destination-address": "inner_ipv6_dst_B"
                        "hop-limit": 1
                      }
                    },
                    "action": {
                      "next-hop-group": "cloud_v6_nhg",
                      "set-hop-limit": 1
                     }
                  }
                ]
              }
            ]  
          }
        },
       "next-hops": {
                        "next-hop": [
                            {
                                "index": 1,
                                "config": {
                                    "index": 1,
                                    "next-hop": "nh_ip_addr_1",
                                    "encap-headers": {
                                        "encap-header": [
                                            {
                                                "index": 1,
                                                "type": "GRE",
                                                "config": {
                                                    "dst-ip": "outer_ipv4_dst_def",
                                                    "src-ip": "outer_ipv4_src1",
                                                    "dscp": "outer_dscp",
                                                    "ip-ttl": "outer_ip-ttl"
                                                }
                                            },
                                            {
                                                "index": 2,
                                                "type": "MPLS",
                                                "config": {
                                                    "index": 2,
                                                    "mpls-label-stack": [
                                                        100
                                                    ]
                                                }
                                            }
                                        ]
                                    }
                                }
                            },
                            {
                                "index": 2,
                                "config": {
                                    "index": 2,
                                    "next-hop": "nh_ip_addr_2",
                                    "encap-headers": {
                                        "encap-header": [
                                            {
                                                "index": 1,
                                                "type": "GRE",
                                                "config": {
                                                    "dst-ip": "outer_ipv4_dst_def",
                                                    "src-ip": "outer_ipv4_src2",
                                                    "dscp": "outer_dscp",
                                                    "ip-ttl": "outer_ip-ttl"
                                                }
                                            },
                                            {
                                                "index": 2,
                                                "type": "MPLS",
                                                "config": {
                                                    "index": 2,
                                                    "mpls-label-stack": [
                                                        100
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
            }
       }
    ]
  }
}
```
* Push the gNMI the policy forwarding configuration
* Push the configuration to DUT using gnmi.Set with REPLACE option
* Send traffic from ATE port 1 to DUT port 1 with TTL as 1.
* Using OTG, validate ATE port 2 receives packets
  * Validate packet ttl as 1.

### TE-1.11.2 Rewrite the ingress innner packet TTL = 1, perform encap and set the outer TTL.
**[TODO]** Test code needs to be implemented.

Building on TE-1.11.1, gRIBI client should send this proto message to the DUT to create AFT
entries.

```proto
#
# aft entries used for network instance "NI_A"
IPv6Entry {2001:DB8:2::2/128 (NI_A)} -> NHG#100 (DEFAULT VRF)
IPv4Entry {203.0.113.2/32 (NI_A)} -> NHG#100 (DEFAULT VRF) -> {
  {NH#101, DEFAULT VRF}
}

# this nexthop specifies a MPLS in UDP encapsulation
NH#101 -> {
  encap_-_headers {
    encap_header {
      index: 1
      mpls {
        pushed_mpls_label_stack: [101,]
      }
    }
    encap_header {
      index: 2
      udp_v6 {
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
  encap_headers {
    encap_header {
      index: 1
      mpls {
        pushed_mpls_label_stack: [201,]
      }
    }
    encap_header {
      index: 2
      udp_v6 {
        src_ip: "outer_ipv6_src"
        dst_ip: "outer_ipv6_dst_B"
        dst_udp_port: "outer_dst_udp_port"
        ip_ttl: "outer_ip-ttl"
        dscp: "outer_dscp"
      }
    }
  }
  next_hop_group_id: "nhg_B"  
  # network_instance: "DEFAULT"  TODO: requires new OC path /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/network-instance
}
```

* Send traffic from ATE port 1 to DUT port 1 with inner packet TTL as 1.
* Using OTG, validate ATE port 2 receives MPLS-IN-UDP packets
  * Validate destination IPs are outer_ipv6_dst_A and outer_ipv6_dst_B
  * Validate MPLS label is set
  * Validate inner packet ttl as 1.
  * Validate outer packet ttl to be "outer_ip-ttl"

## OpenConfig Path and RPC Coverage

```yaml
paths:

  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/destination-address:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/hop-limit:
  # afts state paths set via gRIBI
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/next-hop-group-id:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/network-instance:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/state/type:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/state/mpls/pushed-mpls-label-stack:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/state/udp/src-ip:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/state/udp/dst-ip:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/state/udp/dst-udp-port:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/state/udp/ip-ttl:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/state/udp/dscp:

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
