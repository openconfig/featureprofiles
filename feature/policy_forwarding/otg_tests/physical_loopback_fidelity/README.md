# PF-1.27: Physical Loopback Dataplane Fidelity

## Summary

Verify the ASIC dataplane fidelity for traffic physically re-entering the device via loopback. This test confirms that the ASIC correctly handles packets in its ingress pipeline after they have traversed an external physical link. It specifically validates the handling of control plane packets (TTL=1) requiring Policy-Based Routing (PBR) redirection and standard data plane probes following a routing lookup.

## Testbed type

* dut.testbed: Single DUT with physical fiber loop connected between a Primary port and a Secondary port.

## Procedure

### Test environment setup

* Configure subinterfaces on both loopback ports with a specific VLAN ID (e.g., VLAN 100) and identical link-local subnets.
* Configure independent VRFs on each of them (vrf-primary and vrf-secondary) and bind the subinterfaces to them.
* Configure the DUT to decapsulate incoming ATE traffic (e.g., MPLSoUDPv6) on primary interface and egress the native IP payload to the loopback port.
* Apply policy-forwarding configuration on the secondary subinterface to redirect specific traffic (TTL=1) to an encapsulation next-hop group.

### PF-1.27.1 - Ingress Redirection for Looped TTL=1 Packets

Verify that the ASIC ingress pipeline correctly identifies packets re-entering from the loop with TTL=1 and redirects them via PBR to the Encapsulation Next-Hop Group (NHG).

#### Step 1 - Generate DUT configuration

Configure the Traffic Policy (PBR) to match the TTL=1 traffic on the receiving interface.

#### Canonical OC

```json
{
  "network-instances": {
    "network-instance": [
      {
        "name": "vrf-secondary",
        "policy-forwarding": {
          "interfaces": {
            "interface": [
              {
                "config": {
                  "apply-forwarding-policy": "LOOPBACK_REDIRECTION",
                  "interface-id": "Port-Channel7.100"
                },
                "interface-id": "Port-Channel7.100"
              }
            ]
          },
          "policies": {
            "policy": [
              {
                "config": { "policy-id": "LOOPBACK_REDIRECTION" },
                "policy-id": "LOOPBACK_REDIRECTION",
                "rules": {
                  "rule": [
                    {
                      "config": { "sequence-id": 1 },
                      "ipv4": {
                        "config": {
                          "hop-limit": 1,
                          "protocol": 6
                        }
                      },
                      "action": {
                        "config": {
                          "network-instance": "DEFAULT",
                          "next-hop-group": "ENCAPSULATION_NHG",
                          "set-ttl": 1
                        }
                      },
                      "sequence-id": 1
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

* Step 2 - Push configuration to DUT.
* Step 3 - Inject a BGP Hello packet (TTL=1) from ATE to DUT. DUT decaps & does label lookup to egress from the primary-side router & re-enter to the secondary-side peer IP across the loop.
* Step 4 - Validation: The secondary-side vrf interface receives the packet, matches on the PBR for TTL=1 rule & redirects to the Encapsulation NHG with TTL=1. 0 packet loss observed.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/next-hop-group:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/hop-limit:
  /network-instances/network-instance/policy-forwarding/interfaces/interface/config/apply-forwarding-policy:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/set-ttl:
  /network-instances/network-instance/next-hop-groups/next-hop-group/state/id:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

* FFF (Fixed Form Factor) or MFF (Modular Form Factor) with support for multiple VRFs.
