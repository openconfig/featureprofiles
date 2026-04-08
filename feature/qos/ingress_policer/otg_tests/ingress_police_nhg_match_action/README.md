# DP-2.2: QoS Policer with Match-Action on Next-Hop Group

## Summary

This feature profile defines a test for applying QoS policing by matching traffic based on its Next-Hop Group (NHG) using Policy-Forwarding `MATCH_ACTION`. Traffic matching a specific NHG is subjected to a policer defined in the QoS model, rather than relying on legacy QoS classifier matching.

This approach aligns with modern hardware architectures where policing is tightly coupled with forwarding decisions.

## Topology

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Test setup

Use TE-18.1 test environment setup.

## Procedure

### DP-2.2.1 Configure Qos Policer Groups

* Generate configuration for Policer Groups under `/qos/policer-groups/policer-group`.
* Define rates (CIR, PIR) and burst sizes (BC, BE) if applicable.
* Define actions for conform, exceed, and violate.

### DP-2.2.2 Configure Policy Forwarding with Match-Action

* Generate a policy forwarding policy of type `MATCH_ACTION`.
* Define a rule that matches on the Next-Hop Group using the field `next-hop-group`.
  * It will match on the NHG **name** if it is a static NHG.
  * It will match on the NHG **ID** if it is a gRIBI NHG.
* Set the action to point to the `policer-group` defined in step 1.
* Apply the policy to the ingress interface.

### DP-2.2.3 push gRIBI AFT encapsulation rules

* Create a gRIBI client and install AFT entries for target flows.
* Ensure the `next-hop-group` matches the condition in the policy forwarding rule.

### DP-2.2.4 Test flow policing

* Send traffic exceeding the CIR and verify packet drops or marking as per the policer configuration.
* Send traffic within the CIR and verify full delivery.

## Test Cases

### Test Case 1: Policing with gRIBI NHG

* **Setup**: Program a Next-Hop Group via gRIBI with a specific numeric ID.
* **Configuration**: 
  * Configure a Policer Group with desired rates (e.g., 1-rate 2-color).
  * Configure a Policy Forwarding rule pointing to that Policer Group.
  * Set the condition in `next-hop-group` to match the gRIBI NHG ID.
* **Verification**: Send traffic matching the gRIBI route and verify that policing is applied (e.g., drops exceeding CIR).

### Test Case 2: Policing with Static NHG

* **Setup**: Configure a Static Next-Hop Group via gNMI with a specific name.
* **Configuration**: 
  * Configure a Policer Group with desired rates.
  * Configure a Policy Forwarding rule pointing to that Policer Group.
  * Set the condition in `next-hop-group` to match the Static NHG Name.
* **Verification**: Send traffic matching the static route and verify that policing is applied.

## Canonical OC

The following JSON illustrates the proposed schema usage for this feature profile.

```json
{
  "qos": {
    "policer-groups": {
      "policer-group": [
        {
          "config": {
            "name": "policer_A"
          },
          "name": "policer_A",
          "one-rate-two-color": {
            "config": {
              "cir": "1000000000",
              "bc": 1000000,
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
  },
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "DEFAULT"
        },
        "name": "DEFAULT",
        "policy-forwarding": {
          "policies": {
            "policy": [
              {
                "config": {
                  "policy-id": "policer-match-action-policy",
                  "type": "MATCH_ACTION"
                },
                "policy-id": "policer-match-action-policy",
                "rules": {
                  "rule": [
                    {
                      "action": {
                        "config": {
                          "policer-group": "policer_A"
                        }
                      },
                      "config": {
                        "sequence-id": 10
                      },
                      "conditions": {
                        "config": {
                          "next-hop-group": "nhg_A"
                          // Note: matches on NHG name for static NHG, or NHG ID for gRIBI NHG
                        }
                      },
                      "sequence-id": 10
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

Here is an example for a two-rate three-color policer.

```json
{
  "qos": {
    "policer-groups": {
      "policer-group": [
        {
          "config": {
            "name": "policer_B"
          },
          "name": "policer_B",
          "two-rate-three-color": {
            "config": {
              "cir": "1000000000",
              "bc": 1000000,
              "pir": "2000000000",
              "be": 2000000,
              "queuing-behavior": "POLICE"
            },
            "conform-action": {
              "config": {
                "set-dscp": 10
              }
            },
            "exceed-action": {
              "config": {
                "set-dscp": 20
              }
            },
            "violate-action": {
              "config": {
                "drop": true
              }
            }
          }
        }
      ]
    }
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  # Policer Group config
  /qos/policer-groups/policer-group/config/name:
  /qos/policer-groups/policer-group/one-rate-two-color/config/cir:
  /qos/policer-groups/policer-group/one-rate-two-color/config/bc:
  /qos/policer-groups/policer-group/one-rate-two-color/config/queuing-behavior:
  /qos/policer-groups/policer-group/one-rate-two-color/exceed-action/config/drop:

  /qos/policer-groups/policer-group/two-rate-three-color/config/cir:
  /qos/policer-groups/policer-group/two-rate-three-color/config/bc:
  /qos/policer-groups/policer-group/two-rate-three-color/config/pir:
  /qos/policer-groups/policer-group/two-rate-three-color/config/be:
  /qos/policer-groups/policer-group/two-rate-three-color/conform-action/config/set-dscp:
  /qos/policer-groups/policer-group/two-rate-three-color/conform-action/config/set-dot1p:
  /qos/policer-groups/policer-group/two-rate-three-color/conform-action/config/set-mpls-tc:
  /qos/policer-groups/policer-group/two-rate-three-color/exceed-action/config/set-dscp:
  /qos/policer-groups/policer-group/two-rate-three-color/exceed-action/config/drop:
  /qos/policer-groups/policer-group/two-rate-three-color/violate-action/config/drop:

  # Policer Group Counters (Proposed)
  /qos/policer-groups/policer-group/state/matched-packets:
  /qos/policer-groups/policer-group/state/matched-octets:
  /qos/policer-groups/policer-group/two-rate-three-color/conform-action/state/matched-packets:
  /qos/policer-groups/policer-group/two-rate-three-color/exceed-action/state/matched-packets:
  /qos/policer-groups/policer-group/two-rate-three-color/violate-action/state/matched-packets:

  # Policy Forwarding Match-Action config
  /network-instances/network-instance/policy-forwarding/policies/policy/config/type:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/config/sequence-id:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/conditions/config/next-hop-group:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/policer-group:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
  gribi:
    gRIBI.Flush:
    gRIBI.Modify:
```

## Required DUT platform

* FFF
