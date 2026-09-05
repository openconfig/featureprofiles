# gNMI-1.4: Telemetry: Inventory

## Summary

Validate Telemetry for each FRU within chassis.

## Procedure

For each of the following component types (linecard, chassis, fan, fan_tray, controller
card, power supply, disk, flash, NPU, transceiver, fabric card), validate:

*   Presence of component within gNMI telemetry.
*   Set of telemetry paths required for network discovery (to be specified for
    each case).
*   Validate component health status:
    *   `/components/component/state/equipment-failure` must be `false` (or default to false if not supported).
    *   `/components/component/state/equipment-mismatch` must be `false` (or default to false if not supported).
*   Validate component power metrics (for `LINECARD`, `FABRIC`, `CONTROLLER_CARD`, `POWER_SUPPLY`):
    *   `/components/component/state/allocated-power` must be populated with a valid non-negative number.
    *   `/components/component/state/used-power` must be populated and greater than `0` for active components.
*   TODO: Removal of telemetry when the component is removed or rebooted within
    the chassis, if applicable.

## Canonical OC

```json
{
  "components": {
    "component": [
      {
        "name": "Linecard1",
        "config": {
          "name": "Linecard1"
        },
        "linecard": {
          "config": {
            "power-admin-state": "oc-platform-types:POWER_ENABLED"
          }
        },
        "state": {
          "name": "Linecard1",
          "type": "oc-platform-types:LINECARD",
          "oper-status": "oc-platform-types:ACTIVE",
          "equipment-failure": false,
          "allocated-power": 350,
          "used-power": 280
        }
      }
    ]
  }
}
```

## Config Parameter coverage

*   TODO: /components/component/linecard/config

## OpenConfig Path and RPC Coverage

TODO:
   /components/component/storage
   /components/component/software-module
   /components/component/software-module/state/module-type
   /components/component/state/mfg-date
   /components/component/state/software-version

```yaml
paths:

    /components/component/state/allocated-power:
       platform_type: ["CONTROLLER_CARD", "FABRIC", "LINECARD", "POWER_SUPPLY"]
    /components/component/state/description:
       platform_type: ["CHASSIS", "CONTROLLER_CARD", "FABRIC", "FAN", "FAN_TRAY", "LINECARD", "POWER_SUPPLY"]
    /components/component/state/equipment-failure:
       platform_type: ["CHASSIS", "CONTROLLER_CARD", "FABRIC", "FAN", "FAN_TRAY", "LINECARD", "POWER_SUPPLY", "TRANSCEIVER"]
    /components/component/state/equipment-mismatch:
       platform_type: ["CHASSIS", "CONTROLLER_CARD", "FABRIC", "FAN", "FAN_TRAY", "LINECARD", "POWER_SUPPLY", "TRANSCEIVER"]
    /components/component/state/firmware-version:
       platform_type: ["TRANSCEIVER"]
    /components/component/state/hardware-version:
       platform_type: ["CHASSIS", "CONTROLLER_CARD", "FABRIC", "LINECARD", "POWER_SUPPLY", "TRANSCEIVER"]
    /components/component/state/id:
       platform_type: ["CONTROLLER_CARD", "FABRIC", "FAN", "FAN_TRAY", "INTEGRATED_CIRCUIT", "LINECARD", "POWER_SUPPLY", "SENSOR"]
    /components/component/state/install-component:
       platform_type: ["FABRIC", "FAN", "FAN_TRAY", "FRU", "CONTROLLER_CARD", "LINECARD", "POWER_SUPPLY", "TRANSCEIVER"]
    /components/component/state/install-position:
       platform_type: ["FABRIC", "FAN", "FAN_TRAY", "FRU", "CONTROLLER_CARD", "LINECARD", "POWER_SUPPLY", "TRANSCEIVER"]
    /components/component/state/location:
       platform_type: ["FABRIC", "FAN", "FAN_TRAY", "FRU", "CONTROLLER_CARD", "LINECARD", "POWER_SUPPLY", "TRANSCEIVER"]
    /components/component/state/mfg-name:
       platform_type: ["CHASSIS", "CONTROLLER_CARD", "FABRIC", "LINECARD", "POWER_SUPPLY", "TRANSCEIVER"]
    /components/component/state/model-name:
       platform_type: ["CHASSIS"]
    /components/component/state/name:
       platform_type: ["CHASSIS", "CONTROLLER_CARD", "CPU", "FABRIC", "FAN", "FAN_TRAY", "INTEGRATED_CIRCUIT", "LINECARD", "POWER_SUPPLY", "SENSOR", "STORAGE", "TRANSCEIVER"]
    /components/component/state/oper-status:
       platform_type: ["CHASSIS", "CONTROLLER_CARD", "CPU", "FABRIC", "FAN", "FAN_TRAY", "INTEGRATED_CIRCUIT", "LINECARD", "POWER_SUPPLY", "STORAGE", "TRANSCEIVER"]
    /components/component/state/parent:
       platform_type: ["CONTROLLER_CARD", "FABRIC", "FAN", "FAN_TRAY", "LINECARD", "POWER_SUPPLY"]
    /components/component/state/part-no:
       platform_type: ["CHASSIS", "CONTROLLER_CARD", "CPU", "FABRIC", "FAN", "FAN_TRAY", "LINECARD", "POWER_SUPPLY", "STORAGE", "TRANSCEIVER"]
    /components/component/state/serial-no:
       platform_type: ["CHASSIS", "CONTROLLER_CARD", "CPU", "FABRIC", "FAN", "FAN_TRAY", "LINECARD", "POWER_SUPPLY", "STORAGE", "TRANSCEIVER"]
    /components/component/state/type:
       platform_type: ["CHASSIS", "CONTROLLER_CARD", "CPU", "FABRIC", "FAN", "FAN_TRAY", "INTEGRATED_CIRCUIT", "LINECARD", "POWER_SUPPLY", "SENSOR", "STORAGE", "TRANSCEIVER"]
    /components/component/state/used-power:
       platform_type: ["CONTROLLER_CARD", "FABRIC", "LINECARD", "POWER_SUPPLY"]
    /components/component/state/temperature/alarm-status:
       platform_type: ["SENSOR"]
    /components/component/state/temperature/instant:
       platform_type: ["SENSOR"]
    /components/component/state/temperature/max:
       platform_type: ["SENSOR"]
    /components/component/state/temperature/max-time:
       platform_type: ["SENSOR"]
    /components/component/subcomponents/subcomponent/name:
       platform_type: ["CHASSIS", "CONTROLLER_CARD", "CPU", "FABRIC", "FAN", "FAN_TRAY", "INTEGRATED_CIRCUIT", "LINECARD", "POWER_SUPPLY", "SENSOR", "STORAGE", "TRANSCEIVER"]
    /components/component/subcomponents/subcomponent/state/name:
       platform_type: ["CHASSIS", "CONTROLLER_CARD", "CPU", "FABRIC", "FAN", "FAN_TRAY", "INTEGRATED_CIRCUIT", "LINECARD", "POWER_SUPPLY", "SENSOR", "STORAGE", "TRANSCEIVER"]
    /components/component/integrated-circuit/backplane-facing-capacity/state/available-pct:
       platform_type: ["INTEGRATED_CIRCUIT"]
    /components/component/integrated-circuit/backplane-facing-capacity/state/consumed-capacity:
       platform_type: ["INTEGRATED_CIRCUIT"]
    /components/component/integrated-circuit/backplane-facing-capacity/state/total:
       platform_type: ["INTEGRATED_CIRCUIT"]
    /components/component/integrated-circuit/backplane-facing-capacity/state/total-operational-capacity:
       platform_type: ["INTEGRATED_CIRCUIT"]
    /components/component/controller-card/config/power-admin-state:
       platform_type: ["CONTROLLER_CARD"]
    /components/component/fabric/config/power-admin-state:
       platform_type: ["FABRIC"]
    /components/component/linecard/config/power-admin-state:
       platform_type: ["LINECARD"]

rpcs:
  gnmi:
    gNMI.Get:
```
