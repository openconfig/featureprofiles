# FP-1.1: Power admin DOWN/UP Test

## Summary

Validate ability to set power-admin-state for Fabric, Linecard and
ControllerCard.

## Procedure

*   For each of the non empty Fabric, Linecard and ControllerCard component:
    *   Verify /components/component/state/oper-status is ACTIVE.
    *   Update
        /components/component/{fabric|linecard|controller-card}/config/power-admin-state
        to POWER_DISABLED.
    *   Verify
        /components/component/{fabric|linecard|controller-card}/state/power-admin-state
        changes to POWER_DISABLED.
    *   Verify /components/component/state/oper-status is DISABLED.
    *   Update
        /components/component/{fabric|linecard|controller-card}/config/power-admin-state
        to POWER_ENABLED.
    *   Verify /components/component/state/oper-status returns to ACTIVE.

## Config Parameter coverage

*   /components/component/{fabric|linecard|controller-card}/config/power-admin-state

## Telemetry Parameter coverage

*   /components/component/state/oper-status
*   /components/component/{fabric|linecard|controller-card}/state/power-admin-state
