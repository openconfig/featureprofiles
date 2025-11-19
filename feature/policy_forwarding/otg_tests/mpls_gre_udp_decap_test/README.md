# PF-1.7: Decapsulate MPLS in GRE and UDP

Create a policy-forwarding configuration using gNMI to decapsulate MPLS
in GRE and UDP packets which are sent to a IP from a decap pool or loopback address and apply to
the DUT.
Create a policy-forwarding configuration using gNMI to decapsulate MPLS
in GRE and UDP packets which are sent to an IP from a decap pool or 
loopback address and apply to the DUT.  Configure a static MPLS LSP
which maps the decapsulated MPLS packets to an egress LSP (causing
the label to be popped) and forwards the packets to a next-hop in a
non-default network-instance.

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

### PF-1.7.1 - MPLS in GRE decapsulation set by gNMI

## Canonical OC

```json
{
  "network-instances": {
    "network-instance": [
      {
        "name": "DEFAULT",
        "policy-forwarding": {
          "policies": {
            "policy": [
              {
                "config": {
                  "policy-id": "decap MPLS in GRE"
                },
                "rules": {
                  "rule": [
                    {
                      "config": {
                        "sequence-id": 1
                      },
                      "ipv4": {
                        "config": {
                          "destination-address": "169.254.125.155/28"
                        }
                      },
                      "action": {
                        "config": {
                          "decapsulate-gre": true
                        }
                      }
                    }
                  ]
                }
              }
            ]
          }
        },
        "mpls": {
          "global": {
            "interface-attributes": {
              "interface": [
                {
                  "config": {
                    "interface-id": "Aggregate2",
                    "mpls-enabled": false
                  },
                  "interface-id": "Aggregate2"
                }
              ]
            }
          },
          "lsps": {
            "static-lsps": {
              "static-lsp": [
                {
                  "config": {
                    "name": "Customer IPV4 in:40571 out:pop"
                  },
                  "egress": {
                    "config": {
                      "incoming-label": 40571,
                      "next-hop": "169.254.1.138"
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
* Configure ATE port 1 with traffic flow
  * Flow1 should have a packet encap format : outer_decap_gre_ipv4 <- MPLS label <- inner_decap_ipv4
  * Flow2 should have a packet encap format : outer_decap_gre_ipv4 <- MPLS label <- inner_decap_ipv6
* Configure MPLS Static route to point to a next hop IP that is resolved towards ATE port 2
* Generate traffic from ATE port 1
* Validate ATE port 2 receives both Flow1 and Flow2 innermost IPv4 and IPv6 traffic with correct VLAN and based on the MPLS static route

### PF-1.7.2 - MPLS in UDP decapsulation set by gNMI

## Canonical OC

```json
{
  "network-instances": {
    "network-instance": [
      {
        "name": "default",
        "policy-forwarding": {
          "policies": {
            "policy": [
              {
                "config": {
                  "policy-id": "decap MPLS in UDP"
                },
                "rules": {
                  "rule": [
                    {
                      "config": {
                        "sequence-id": 1
                      },
                      "ipv4": {
                        "config": {
                          "destination-address": "169.254.126.155/28"
                        }
                      },
                      "action": {
                        "config": {
                          "decapsulate-mpls-in-udp": true
                        }
                      }
                    }
                  ]
                }
              }
            ]
          }
        },
        "mpls": {
          "global": {
            "interface-attributes": {
              "interface": [
                {
                  "config": {
                    "interface-id": "Aggregate4",
                    "mpls-enabled": false
                  },
                  "interface-id": "Aggregate4"
                }
              ]
            }
          },
          "lsps": {
            "static-lsps": {
              "static-lsp": [
                {
                  "config": {
                    "name": "Customer IPV4 in:40571 out:pop"
                  },
                  "egress": {
                    "config": {
                      "incoming-label": 40571,
                      "next-hop": "169.254.1.138"
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
* Configure ATE port 1 with traffic flow
  * Flow should have a packet encap format : outer_decap_udp_ipv6 <- MPLS label <- inner_decap_ipv6
* Generate traffic from ATE port 1
* Validate ATE port 2 receives the innermost IPv4 traffic with correct VLAN and inner_decap_ipv6

## OpenConfig Path and RPC Coverage

```yaml
paths:

  # Paths added for PF-1.7.1 - MPLS in GRE decapsulation set by gNMI
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/destination-address:
  # TODO: /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decapsulate-mpls-in-gre:

  # Paths added for PF-1.7.2 - MPLS in UDP decapsulation set by gNMI
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decapsulate-mpls-in-udp:

  #TODO: Add OC for next-network-instance see https://github.com/openconfig/public/pull/1395
  # set the network-instance to be used for the egress LSP next-hop
  # TODO: /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/config/nh-network-instance

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
