# gNOI-3.2: Per-Component Reboot

## Summary

Validate gNOI RPC can reboot specific components.

## Procedure

*   Issue gnoi.system Reboot to chassis with no populated delay and
    subcomponents set to:
    *   A field-removable linecard in the system
    *   A control-processor (supervisor)
    *   A field-removable fabric component in the system
*   After linecard reboot completes and interfaces return to operational `UP`
    state, send traffic between ATE Port 1 (`192.0.2.2/30`) and ATE Port 2
    (`192.0.2.6/30`) through the DUT and verify that the integrated-circuit
    adverse-aggregate drop counter
    (`/components/component/integrated-circuit/pipeline-counters/drop/state/adverse-aggregate`)
    does not increment during traffic forwarding.
*   TODO: For each component verify that the component has rebooted and the
    uptime has been reset.

## Canonical OC

```json
{
  "openconfig-platform:components": {
    "component": [
      {
        "config": {
          "name": "card1"
        },
        "name": "card1"
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

TODO(OCPATH): Add component names to component paths.

```yaml
paths:
    ## Config paths: N/A

    ## State paths
    ### FIXME: Add component description
    #/components/component/state/description:
    /components/component/state/name:
      platform_type: ["CONTROLLER_CARD", "FABRIC", "LINECARD"]
    /components/component/state/oper-status:
      platform_type: ["FABRIC"]
    /components/component/state/removable:
      platform_type: ["FABRIC", "LINECARD"]
    /components/component/integrated-circuit/pipeline-counters/drop/state/adverse-aggregate:
      platform_type: ["INTEGRATED_CIRCUIT"]
    /interfaces/interface/state/name:
    /interfaces/interface/state/oper-status:

rpcs:
  gnoi:
    system.System.Reboot:
    system.System.RebootStatus:
```
