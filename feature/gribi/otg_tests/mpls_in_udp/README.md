# TE-18.1: gRIBI MPLS in UDP Encapsulation

Create AFT entries using gRIBI to match on next hop group in a
network-instance and encapsulate the matching packets in MPLS in UDP with outer header as IPv6 Header.

The MPLS in UDP encapsulation is expected to follow
[rfc7510](https://datatracker.ietf.org/doc/html/rfc7510#section-3),
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
outer_dst_udp_port =  "6635"
outer_dscp =          "26"
outer_ip-ttl =        "64"

## Procedure

### TE-18.1.1 Match and Encapsulate using gRIBI aft modify

#### gRIBI RPC content

The gRIBI client should send this proto message to the DUT to create AFT
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

* Send traffic from ATE port 1 to DUT port 1
* Using OTG, validate ATE port 2 receives MPLS-IN-UDP packets
  * Validate destination IPs are outer_ipv6_dst_A and outer_ipv6_dst_B
  * Validate MPLS label is set

### TE-18.1.2 Validate prefix match rule for MPLS in GRE encap using default route

Canonical OpenConfig for policy forwarding, matching IP prefix with action
encapsulate in GRE.

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
                  "policy-id": "default encap rule",
                  "type": "PBR_POLICY"
                },
                "policy": "default encap rule",
                "rules": [
                  {
                    "action": {
                      "encapsulate-headers": [
                        {
                          "encapsulate-header": null,
                          "gre": {
                            "config": {
                              "destination-ip": "outer_ipv6_dst_def",
                              "dscp": "outer_dscp",
                              "id": "default_dst_1",
                              "ip-ttl": "outer_ip-ttl",
                              "source-ip": "outer_ipv6_src"
                            }
                          },
                          "mpls": {
                            "mpls-label-stack": [
                              100
                            ]
                          }
                        }
                      ],
                      "config": {
                        "network-instance": "DEFAULT"
                      }
                    },
                    "config": {
                      "sequence-id": 1,
                    },
                    "ipv6": {
                      "config": {
                        "destination-address": "inner_ipv6_default"
                      }
                    },
                    "rule": 1
                  }
                ]
              }
            ]
          }
        },
        "network-instance": "group_A"
      }
    ]
  }
}
```

* Generate the policy forwarding configuration
* Push the configuration to DUT using gnmi.Set with REPLACE option
* Configure ATE port 1 with traffic flow which does not match any AFT next hop route
* Generate traffic from ATE port 1 to ATE port 2
* Validate ATE port 2 receives GRE traffic with correct inner and outer IPs


### TE-18.1.5 Rewrite the ingress innner packet TTL = 1, if the incoming TTL = 1.
Canonical OpenConfig for policy forwarding, matching IP prefix and TTL = 1 with action
set inner packet TTL = 1.

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
                  "policy-id": "retain ttl",
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
                        "destination-address": "router_ip"
                        "hop-limit": 1
                      }
                    },
                    "action": {
                      "set-ip-ttl": 1  #TODO: Add set-ip-ttl [OC path]
                     }
                  }
                ]
              }
            ]  
          }
        }
      }
    ]
  }
}
```
* Push the gNMI the policy forwarding configuration
* Push the configuration to DUT using gnmi.Set with REPLACE option
* Send traffic from ATE port 1 to DUT port 1 with inner packet TTL as 1.
* Using OTG, validate ATE port 2 receives MPLS-IN-GRE packets
  * Validate destination IPs are outer_ipv6_dst_A and outer_ipv6_dst_B
  * Validate MPLS label is set
  * Validate inner packet ttl as 1.
  * Validate outer packet ttl to be "outer_ip-ttl"
## OpenConfig Path and RPC Coverage

```yaml
paths:

# afts state paths set via gRIBI
  # TODO: need new OC for user defined next-hop-group/state/id, needed for policy-forwarding rules pointing to a NHG
  # /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/next-hop-group-id:

  # TODO: new OC path for aft NHG pointing to a different network-instance
  # /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/network-instance:

  # Paths added for TE-18.1.1 Match and Encapsulate using gRIBI aft modify
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

  # Paths added for TE-18.1.2 Validate prefix match rule for MPLS in GRE encap using default route
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/network-instance:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/config/sequence-id:
  #/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encap-headers/encap-header/mpls/config/mpls-label-stack:
  #/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encap-headers/encap-header/gre/config/destination-ip:
  #/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encap-headers/encap-header/gre/config/dscp:
  #/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encap-headers/encap-header/gre/config/id:
  #/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encap-headers/encap-header/gre/config/ip-ttl:
  #/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encap-headers/encap-header/gre/config/source-ip:


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
