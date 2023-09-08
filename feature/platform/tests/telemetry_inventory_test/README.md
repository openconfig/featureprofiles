# gNMI-1.4: Telemetry: Inventory

## Summary

Validate Telemetry for each FRU within chassis.

## Procedure

For each of the following component types (linecard, chassis, fan, controller
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
*   /components/component/state/firmware-version
*   /components/component/state/hardware-version
*   /components/component/state/id
*   /components/component/state/mfg-date
*   /components/component/state/mfg-name
*   /components/component/state/name
*   /components/component/state/oper-status
*   /components/component/state/parent
*   /components/component/state/part-no
*   /components/component/state/serial-no
*   /components/component/state/software-version
*   /components/component/state/type
*   /components/component/state/temperature/alarm-status
*   /components/component/state/temperature/instant
*   /components/component/state/temperature/max
*   /components/component/state/temperature/max-time
*   /components/component/integrated-circuit/backplane-facing-capacity/state/available-pct
*   /components/component/integrated-circuit/backplane-facing-capacity/state/consumed-capacity
*   /components/component/integrated-circuit/backplane-facing-capacity/state/total
*   /components/component/integrated-circuit/backplane-facing-capacity/state/total-operational-capacity
