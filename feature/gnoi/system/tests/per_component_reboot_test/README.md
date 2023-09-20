# gNOI-3.2: Per-Component Reboot

## Summary

Validate gNOI RPC can reboot specific components.

## Procedure

*   Issue gnoi.system Reboot to chassis with no populated delay and
    subcomponents set to:
    *   A field-removable linecard in the system
    *   A control-processor (supervisor)
    *   A field-removable fabric component in the system
*   TODO: For each component verify that the component has rebooted and the
    uptime has been reset.

## Config Parameter Coverage

N/A

## Telemetry Parameter Coverage

*   /components/component/state/description
*   /components/component/state/removable
*   /components/component/state/name
*   /components/component/state/oper-status
*   /interfaces/interface/state/name
*   /interfaces/interface/state/oper-status

## Protocol/RPC Parameter Coverage

*   gNOI
    *   System
        *   Reboot
