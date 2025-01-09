# PF-1.7 Decapsulate MPLS in GRE and UDP

Create a policy-forwarding configuration using gNMI to decapsulate MPLS
in GRE and UDP packets which are sent to a loopback address and apply to
the DUT.

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

Canonical OpenConfig for policy forwarding, matching IP prefix with action
decapsulate in GRE.

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
                  "policy-id": "default decap rule",
                  "type": "PBR_POLICY"
                },
                "policy": "default decap rule",
                "rules": [
                  {
                    "config": {
                      "sequence-id": 1,
                    },
                    "ipv6": {
                      "config": {
                        "destination-address": "decap_loopback_ipv6"
                      }
                    },
                    "action": {
                      "decapsulate-mpls-in-gre": TRUE  
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
* Configure ATE port 1 with traffic flow
  * Flow should have a packet encap format : outer_decap_gre_ipv6 <- MPLS label <- inner_decap_ipv6
* Generate traffic from ATE port 1
* Validate ATE port 2 receives the innermost IPv4 traffic with correct VLAN and inner_decap_ipv6

### PF-1.7.2 - MPLS in UDP decapsulation set by gNMI

Canonical OpenConfig for policy forwarding, matching IP prefix with action
decapsulate MPLS in UDP.

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
                  "policy-id": "default decap rule",
                  "type": "PBR_POLICY"
                },
                "policy": "default decap rule",
                "rules": [
                  {
                    "config": {
                      "sequence-id": 1,
                    },
                    "ipv6": {
                      "config": {
                        "destination-address": "decap_loopback_ipv6"
                      }
                    },
                    "action": {
                      "decapsulate-mpls-in-udp": TRUE  
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
