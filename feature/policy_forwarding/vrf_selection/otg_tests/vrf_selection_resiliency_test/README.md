# RT-3.4: VRF Selection Policy Hardware Programming with Linecard and Supervisor Resiliency

## Summary

Validate that a scaled VRF Selection Policy is programmed to hardware successfully on ingress ports and that these hardware programming entries persist and forward traffic hitlessly during a Supervisor Switchover, and reprogram successfully following a Linecard OIR (Online Insertion and Removal).

## Testbed type

* `TESTBED_DUT_ATE_4LINKS`

## Procedure

### Test environment setup

*   Connect OTG/ATE Ports 1, 2, 3, and 4 to DUT Ports 1, 2, 3, and 4 respectively.
*   **DUT Port-1 (Ingress):** Base interface assigned entirely to the `DEFAULT` network-instance (`/network-instances/network-instance[name=DEFAULT]/config/name`). The VRF Selection Policy will be applied here.
*   **DUT Port-2 (Egress 1):** Configure 11 sub-interfaces (`/interfaces/interface/subinterfaces/subinterface/config/index`). Assign 10 sub-interfaces to `VRF-V4-1` through `VRF-V4-10`. Assign 1 sub-interface to the `DEFAULT` network-instance.
*   **DUT Port-3 (Egress 2):** Configure 10 sub-interfaces. Assign 5 sub-interfaces to `VRF-V4-11` through `VRF-V4-15`. Assign 5 sub-interfaces to `VRF-V6-1` through `VRF-V6-5`.
*   **DUT Port-4 (Egress 3):** Configure 10 sub-interfaces. Assign 10 sub-interfaces to `VRF-V6-6` through `VRF-V6-15`.
*   Configure the DUT with 30 distinct static routes in their respective 30 VRFs (`/network-instances/network-instance/protocols/protocol/static-routes/static[prefix=<network>]/config/prefix`), pointing out to specific OTG-emulated networks attached to ATE Ports 2, 3, and 4. Configure a default static route in the `DEFAULT` network-instance pointing out the DUT Port-2 `DEFAULT` sub-interface.
*   Configure the OTG to advertise 30 unique networks (15 IPv4 and 15 IPv6 blocks) across the ATE egress ports corresponding to the static routes.

### RT-3.4.1 - Scaled VRF Selection Policy Hardware Programming

*   **Step 1 - Generate DUT configuration:** Configure a massive VRF Selection Policy (`HA_VRF_SELECTION`) on DUT Ingress Port-1 to steer specific traffic flows into the 30 VRFs.
    *   **Rules 1-15:** Match IPinIP (Protocol 4) with unique IPv4 source subnets and map to `VRF-V4-1` through `VRF-V4-15`.
    *   **Rules 16-30:** Match IPv6inIP (Protocol 41) with unique IPv6 source subnets and map to `VRF-V6-1` through `VRF-V6-15`.
    *   **Rule 31 (Ghost VRF):** Match a specific IPinIP flow and map to `VRF-GHOST`. After applying, ensure `VRF-GHOST` is deleted from the system.
    *   **Rule 100 (Catch-All):** Match any IPinIP traffic and map to `VRF-V4-15`.

#### Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "name": "Port-1"
        },
        "name": "Port-1",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0
            }
          ]
        }
      }
    ]
  },
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "VRF-10"
        },
        "name": "VRF-10"
      },
      {
        "name": "DEFAULT",
        "policy-forwarding": {
          "interfaces": {
            "interface": [
              {
                "config": {
                  "apply-vrf-selection-policy": "PBF-VRF-10",
                  "interface-id": "Port-1"
                },
                "interface-id": "Port-1",
                "interface-ref": {
                  "config": {
                    "interface": "Port-1",
                    "subinterface": 0
                  }
                }
              }
            ]
          },
          "policies": {
            "policy": [
              {
                "config": {
                  "policy-id": "PBF-VRF-10",
                  "type": "openconfig-policy-forwarding:VRF_SELECTION_POLICY"
                },
                "policy-id": "PBF-VRF-10",
                "rules": {
                  "rule": [
                    {
                      "action": {
                        "config": {
                          "network-instance": "VRF-10"
                        }
                      },
                      "config": {
                        "sequence-id": 10
                      },
                      "ipv4": {
                        "config": {
                          "protocol": 4
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

*   **Step 2 - Push configuration:** Push to DUT using `gnmi.Set` with `REPLACE` option.

*   **Step 3 - Send Traffic:** Generate continuous OTG traffic from ATE Port-1:
    *   **30 Positive Streams:** Crafted to explicitly match Rules 1-30.
    *   **30 Negative Streams:** Standard UDP streams destined to the 30 networks but lacking the IPinIP/IPv6inIP outer headers (should match no rules).
    *   **Ghost Stream:** Traffic matching Rule 31.
    *   **Shadow Stream:** Traffic matching Rule 100 but *not* matching Rules 1-15.

*   **Step 4 - Validation (Baseline):** 
    *   Verify the 30 Positive Streams arrive perfectly isolated at their respective VLANs on ATE Ports 2, 3, and 4 with 0% loss.
    *   Verify the 30 Negative Streams fall back to the `DEFAULT` VRF and successfully egress the `DEFAULT` sub-interface on DUT Port-2.
    *   Verify the Ghost Stream (Rule 31) is entirely dropped.
    *   Verify the Shadow Stream correctly maps to `VRF-V4-15`.

### RT-3.4.2 - VRF Selection Policy Resilience Post Supervisor Switchover

*   **Step 1 - Trigger Switchover:** While all 60+ continuous streams are running, use the gNOI RPC `gnoi.system.System.SwitchControlProcessor` to trigger a Route Processor (Supervisor) switchover. Wait for the new supervisor to become `ACTIVE`.
*   **Step 2 - Validation:** Verify **0% traffic loss** across all running streams (Positive, Negative, and Shadow). This proves the forwarding ASICs flawlessly maintained their VRF selection policy state and FIB pointers while the control plane transitioned. Verify Ghost Stream remains dropped.

### RT-3.4.3 - VRF Selection Policy Resilience Post Linecard OIR

*   **Step 1 - Perform Linecard Soft OIR:** While all 60+ continuous streams are still running, identify the Linecard component hosting DUT Port-1 using `/components/component[name=<LC_NAME>]`.
    *   Use `gnmi.Set` to change `/components/component/linecard/config/power-admin-state` to `POWER_DISABLED`.
    *   Wait for the linecard operational status `/components/component[name=<LC_NAME>]/state/oper-status` to transition to `DISABLED`. Traffic will drop.
    *   Use `gnmi.Set` to change `/components/component/linecard/config/power-admin-state` back to `POWER_ENABLED`.
    *   Wait for the linecard operational status `/components/component[name=<LC_NAME>]/state/oper-status` to transition back to `ACTIVE` and all ports to come up.
*   **Step 2 - Validation:** Once the linecard is `ACTIVE`, verify that all 60+ streams autonomously recover. Verify that traffic correctly maps back to the 30 specific VRFs and the `DEFAULT` VRF without cross-talk, leakage, or TCAM shadowing (Rule 100 overriding Rules 1-15). Verify Ghost Stream remains dropped.

### RT-3.4.4 - Policy Deletion

*   **Step 1 - Generate DUT configuration:** Remove the VRF Selection policy from the ingress interface (`apply-vrf-selection-policy`).
*   **Step 2 - Push configuration:** Push to DUT using `gnmi.Set` with `REPLACE` (or `DELETE` the specific path).
*   **Step 3 - Validation:** Verify that all previously matching Positive Streams are no longer steered to their specific VRFs and correctly revert to using the `DEFAULT` VRF routing table (egressing DUT Port-2's default sub-interface).

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /components/component/linecard/config/power-admin-state:
    platform_type: ["LINECARD"]
  /components/component/state/oper-status:
    platform_type: ["LINECARD"]
  /components/component/linecard/state/power-admin-state:
    platform_type: ["LINECARD"]
  /components/component/state/last-switchover-time:
    platform_type: ["CONTROLLER_CARD"]
  /components/component/state/last-switchover-reason/trigger:
    platform_type: ["CONTROLLER_CARD"]
  /components/component/state/last-switchover-reason/details:
    platform_type: ["CONTROLLER_CARD"]
  /network-instances/network-instance/policy-forwarding/interfaces/interface/config/apply-vrf-selection-policy:
  /network-instances/network-instance/policy-forwarding/interfaces/interface/interface-ref/config/interface:
  /network-instances/network-instance/policy-forwarding/interfaces/interface/interface-ref/config/subinterface:
  /network-instances/network-instance/policy-forwarding/policies/policy/config/policy-id:
  /network-instances/network-instance/policy-forwarding/policies/policy/config/type:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/network-instance:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/config/sequence-id:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/protocol:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/source-address:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/protocol:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/source-address:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
      replace: true
  gnoi:
    system.System.SwitchControlProcessor:
```

## Required DUT platform

* MFF
