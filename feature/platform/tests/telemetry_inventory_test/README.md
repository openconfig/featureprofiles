# gNMI-1.4: Telemetry: Inventory

## Summary

Validate gNMI telemetry paths for all Field Replaceable Units (FRUs) and
physical/logical platform components within the device chassis. The test
verifies that hardware components (Chassis, Linecard, Fabric, Fan, Fan Tray,
Controller Card/Supervisor, Power Supply, Integrated Circuit/NPU) correctly
expose operational state and telemetry data required for network discovery,
health monitoring, and inventory tracking.

## Testbed type

* `TESTBED_DUT`

## Procedure

### Setup

Ensure the Device Under Test (DUT) is powered on and accessible via gNMI.
Retrieve the list of all hardware components present in the chassis telemetry
hierarchy under `/components/component[name=*]`.

### TestID-1.4.1 - Component Presence and Attribute Validation

For each component type (Chassis, Linecard, Fabric, Fan, Fan Tray, Controller
Card/Supervisor, Integrated Circuit/NPU), validate the presence of expected
telemetry attributes:

* Verify component presence under `/components/component/state/name`.
* Validate that component operational attributes are set (such as
  `description`, `hardware-version`, `mfg-name`, `oper-status`, `part-no`,
  `serial-no`, `type`).

### TestID-1.4.2 - Temperature Sensor Telemetry

For each component of type `SENSOR` representing a temperature sensor:

* Validate `/components/component/state/temperature/instant` returns a non-nil
  temperature reading.
* Validate `/components/component/state/temperature/alarm-status` is present.
* Validate `/components/component/state/temperature/max` and
  `/components/component/state/temperature/max-time` telemetry leaves.

### TestID-1.4.3 - Power Supply Telemetry

For each non-empty component of type `POWER_SUPPLY`:

* Query `/components/component[name=<psu_name>]/power-supply/state`.
* Verify that telemetry state is present in gNMI response.
* Validate the operational telemetry leaves for power supply metrics:
  * `/components/component/power-supply/state/input-current` (Amperes)
  * `/components/component/power-supply/state/input-voltage` (Volts)
  * `/components/component/power-supply/state/output-current` (Amperes)
  * `/components/component/power-supply/state/output-voltage` (Volts)

## Config Parameter coverage

* `/components/component/controller-card/config/power-admin-state`
* `/components/component/fabric/config/power-admin-state`
* `/components/component/linecard/config/power-admin-state`

## Canonical OC

```json
{}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /components/component/state/description:
     platform_type: ["CHASSIS", "CONTROLLER_CARD", "FABRIC", "FAN", "FAN_TRAY", "LINECARD", "POWER_SUPPLY"]
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
  /components/component/state/removable:
     platform_type: ["CONTROLLER_CARD", "FABRIC", "FAN", "FAN_TRAY", "LINECARD", "POWER_SUPPLY", "TRANSCEIVER"]
  /components/component/state/serial-no:
     platform_type: ["CHASSIS", "CONTROLLER_CARD", "CPU", "FABRIC", "FAN", "FAN_TRAY", "LINECARD", "POWER_SUPPLY", "STORAGE", "TRANSCEIVER"]
  /components/component/state/type:
     platform_type: ["CHASSIS", "CONTROLLER_CARD", "CPU", "FABRIC", "FAN", "FAN_TRAY", "INTEGRATED_CIRCUIT", "LINECARD", "POWER_SUPPLY", "SENSOR", "STORAGE", "TRANSCEIVER"]
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
  /components/component/power-supply/state/input-current:
     platform_type: ["POWER_SUPPLY"]
  /components/component/power-supply/state/input-voltage:
     platform_type: ["POWER_SUPPLY"]
  /components/component/power-supply/state/output-current:
     platform_type: ["POWER_SUPPLY"]
  /components/component/power-supply/state/output-voltage:
     platform_type: ["POWER_SUPPLY"]
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
