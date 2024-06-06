# gNMI-1.4: Telemetry: Inventory

## Summary

Validate Telemetry for each FRU within chassis.

## Procedure

For each of the following component types (linecard, chassis, fan, fan_tray, controller
card, power supply, disk, flash, NPU, transceiver, fabric card), validate:

*   Presence of component within gNMI telemetry.
*   Set of telemetry paths required for network discovery (to be specified for
    each case).
*   TODO: Removal of telemetry when the component is removed or rebooted within
    the chassis, if applicable.

## Config Parameter coverage

*   TODO: /components/component/linecard/config

## Telemetry Parameter coverage

*   /components/component[name=<heatsink-temperature-sensor>]/state/temperature/instant
*   /components/component/storage
*   TODO: /components/component/software-module
*   TODO: /components/component/software-module/state/module-type
*   /components/component/state/description
    *  patform_type: ["CHASSIS", "CONTROLLER_CARD", "FABRIC", "FAN", "FAN_TRAY", "LINECARD", "POWER_SUPPLY"]
*   /components/component/state/firmware-version
    *  patform_type: ["TRANSCEIVER"]
*   /components/component/state/hardware-version
    *  patform_type: ["CHASSIS", "CONTROLLER_CARD", "FABRIC", "LINECARD", "POWER_SUPPLY", "TRANSCEIVER"]
*   /components/component/state/id
    *  patform_type: ["CONTROLLER_CARD", "FABRIC", "FAN", "FAN_TRAY", "INTEGRATED_CIRCUIT", "LINECARD", "POWER_SUPPLY", "SENSOR"]
*   /components/component/state/mfg-date
    *  patform_type: []
*   /components/component/state/mfg-name
    *  patform_type: ["CHASSIS", "CONTROLLER_CARD", "FABRIC", "LINECARD", "POWER_SUPPLY", "TRANSCEIVER"]
*   /components/component/state/model-name
    *  patform_type: ["CHASSIS"]
*   /components/component/state/name
    *  patform_type: ["CHASSIS", "CONTROLLER_CARD", "CPU", "FABRIC", "FAN", "FAN_TRAY", "INTEGRATED_CIRCUIT", "LINECARD", "POWER_SUPPLY", "SENSOR", "STORAGE", "TRANSCEIVER"]
*   /components/component/state/oper-status
    *  patform_type: ["CHASSIS", "CONTROLLER_CARD", "CPU", "FABRIC", "FAN", "FAN_TRAY", "INTEGRATED_CIRCUIT", "LINECARD", "POWER_SUPPLY", "STORAGE", "TRANSCEIVER"]
*   /components/component/state/parent
    *  patform_type: ["CONTROLLER_CARD", "FABRIC", "FAN_TRAY", "LINECARD", "POWER_SUPPLY"]
*   /components/component/state/part-no
    *  patform_type: ["CHASSIS", "CONTROLLER_CARD", "CPU", "FABRIC", "FAN", "FAN_TRAY", "LINECARD", "POWER_SUPPLY", "STORAGE", "TRANSCEIVER"]
*   /components/component/state/serial-no
    *  patform_type: ["CHASSIS", "CONTROLLER_CARD", "CPU", "FABRIC", "FAN", "FAN_TRAY", "LINECARD", "POWER_SUPPLY", "STORAGE", "TRANSCEIVER"]
*   /components/component/state/software-version
    *  patform_type: []
*   /components/component/state/type
    *  patform_type: ["CHASSIS", "CONTROLLER_CARD", "CPU", "FABRIC", "FAN", "FAN_TRAY", "INTEGRATED_CIRCUIT", "LINECARD", "POWER_SUPPLY", "SENSOR", "STORAGE", "TRANSCEIVER"]
*   /components/component/state/temperature/alarm-status
    *  patform_type: ["SENSOR"]
*   /components/component/state/temperature/instant
    *  patform_type: ["SENSOR"]
*   /components/component/state/temperature/max
    *  patform_type: ["SENSOR"]
*   /components/component/state/temperature/max-time
    *  patform_type: ["SENSOR"]
*   /components/component/integrated-circuit/backplane-facing-capacity/state/available-pct
    *  patform_type: ["INTEGRATED_CIRCUIT"]
*   /components/component/integrated-circuit/backplane-facing-capacity/state/consumed-capacity
    *  patform_type: ["INTEGRATED_CIRCUIT"]
*   /components/component/integrated-circuit/backplane-facing-capacity/state/total
    *  patform_type: ["INTEGRATED_CIRCUIT"]
*   /components/component/integrated-circuit/backplane-facing-capacity/state/total-operational-capacity
    *  patform_type: ["INTEGRATED_CIRCUIT"]
