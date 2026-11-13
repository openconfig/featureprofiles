# FP-1.1: Power admin DOWN/UP Test

## Summary

Validate the ability to toggle the power-admin-state for Fabric, Linecard, and ControllerCard components. Including negative scenario for attempting to power down both controller cards.


## FP-1.1.1: Powering down Linecard and Fabric Card.

1. **Test Setup**:
    *  Verify `/components/component/{linecard|fabric}/state/oper-status` is `ACTIVE` for all installed Linecards and Fabric Cards.
2. **Test Logic**:
    *  Select one linecard and one fabric card.
    *  Update `/components/component/{linecard|fabric}/config/power-admin-state` to `POWER_DISABLED` for the selected linecard and fabric card.
    *  Verify `/components/component/{linecard|fabric}/state/power-admin-state` changes to `POWER_DISABLED` for the selected linecard and fabric card.
    *  Verify `/components/component/{linecard|fabric}/state/oper-status` is `DISABLED` for the selected linecard and fabric card.
    *  Update `/components/component/{linecard|fabric}/config/power-admin-state` to `POWER_ENABLED` for the selected linecard and fabric card.
    *  Verify `/components/component/{linecard|fabric}/state/oper-status` returns to `ACTIVE` for the selected linecard and fabric card.

## FP-1.1.2: Attempting to power down both controller cards

1.  **Test Setup**:
    *   Connect to the DUT via gNMI and retrieve the list of all available controller cards by checking components of type `CONTROLLER_CARD`.
    *   Ensure there are at least two controller cards available on the DUT to perform this test.
    *   Verify that the `/components/component/state/oper-status` is `ACTIVE` for both the controller cards.
    *   Verify that the `/components/component/state/switchover-ready` is `true` for both the controller cards.
2.  **Test Logic**:
    *   Find out the `PRIMARY` Controller Card using `/components/component/state/redundant-role`.
    *   Using gNMI Set, attempt to update `/components/component/controller-card/config/power-admin-state` to `POWER_DISABLED` for the `PRIMARY` controller card.
    *   Wait until the switchover happens and the `SECONDARY` becomes `PRIMARY`.
    *   Verify that the `/components/component/state/oper-status` of the disabled Controller Card is now `DISABLED`.
    *   Using gNMI Set, attempt to update `/components/component/controller-card/config/power-admin-state` to `POWER_DISABLED` for the newly elected `PRIMARY` controller card.
    *   The device should either:
        *   Reject the configuration to `POWER_DISABLED` the controller card OR
        *   Automatically enable the previously `POWER_DISABLED` Controller Card and then `POWER_DISABLED` the requested Controller Card.
    *   In any case the device should not become unresponsive, unreachable or lose connectivity.
    *   Restore the `/components/component/controller-card/config/power-admin-state` for the disabled controller card back to `POWER_ENABLED` via gNMI Set.
3.  **Verification**:
    *   Verify the operational status and redundant roles of the controller cards post-recovery.

### Canonical OC

```json
{
  "openconfig-platform:components": {
    "component": [
      {
        "name": "RE0",
        "openconfig-platform-controller-card:controller-card": {
          "config": {
            "power-admin-state": "POWER_DISABLED"
          }
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths and RPC intended to be covered by this test.

```yaml
paths:
  /components/component/controller-card/config/power-admin-state:
    platform_type: [CONTROLLER_CARD]
  /components/component/controller-card/state/power-admin-state:
    platform_type: [CONTROLLER_CARD]
  /components/component/fabric/config/power-admin-state:
    platform_type: [FABRIC]
  /components/component/fabric/state/power-admin-state:
    platform_type: [FABRIC]
  /components/component/linecard/config/power-admin-state:
    platform_type: [LINECARD]
  /components/component/linecard/state/power-admin-state:
    platform_type: [LINECARD]
  /components/component/state/oper-status:
    platform_type: [CONTROLLER_CARD, FABRIC, LINECARD]
  /components/component/state/redundant-role:
    platform_type: [CONTROLLER_CARD]
  /components/component/state/switchover-ready:
    platform_type: [CONTROLLER_CARD]
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```
## Minimum DUT platform requirement
*   MFF