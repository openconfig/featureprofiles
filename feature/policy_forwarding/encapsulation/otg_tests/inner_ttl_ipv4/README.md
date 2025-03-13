# PF-1.11 Rewrite the ingress innner packet TTL

Create a policy-forwarding configuration using gNMI with action set-ip-ttl.

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

### TE-1.11.1 Rewrite the ingress innner packet TTL = 1, if the incoming TTL = 1.
**[TODO]** Test code needs to be implemented.

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
                      "set-ip-ttl": 1  #TODO: Add set-ip-ttl [https://github.com/openconfig/public/pull/1263/files]
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

  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/destination-address:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/hop-limit:

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