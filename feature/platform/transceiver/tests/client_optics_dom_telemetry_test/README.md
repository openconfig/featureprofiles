# TRANSCEIVER-20.1: Client Optics DOM Telemetry, Instant, Threshold, and Miscellaneous Static Info

## Summary

Validate client optics digital optical monitoring (DOM) streaming telemetry performance monitoring parameters
like input power, output power, bias current, supply voltage, module temperature, and so on.

## Setup

Optics test requirements alongside the platform functional tests require optics DUT samples connected across optical ethernet interfaces.

## Procedure

*   Step 1: Ensure interfaces and transceivers are enabled, wait for link UP, and validate inventory and telemetry:
    *   Validate static inventory metadata: `/components/component/state/mfg-name`, `/components/component/transceiver/state/form-factor`, `/components/component/state/serial-no`, `/components/component/state/part-no`, `/components/component/state/firmware-version`, `/components/component/state/hardware-version`, `/components/component/state/description`, `/components/component/state/mfg-date`, `/components/component/transceiver/physical-channels/channel/state/index`, `/interfaces/interface/state/physical-channel`, `/interfaces/interface/state/transceiver`.
    *   Validate dynamic telemetry instant values are within the normal operating range bounded by WARNING and CRITICAL lower/upper thresholds (i.e., no threshold alarms are expected to be triggered during normal operation): module temperature, Tx output power, Rx input power, laser bias-current, and supply voltage.
        *   Nested-threshold rules apply: `critical_lower <= warning_lower <= instant <= warning_upper <= critical_upper`.
        *   Some hardware platforms or pluggables may report equal values for warning and critical thresholds (e.g., `warning_upper == critical_upper` or `warning_lower == critical_lower`), which is valid and supported.

*   Step 2: Verify interface enable/disable lifecycle:
    *   Disable/shutdown interface: verify oper-status DOWN and output power drops.
    *   Re-enable interface: wait for link UP and verify telemetry parameters return to normal within thresholds.

*   Step 3: Verify transceiver config enabled on/off lifecycle:
    *   Set `/components/component/transceiver/config/enabled` to `false`: verify oper-status is not UP and transceiver disabled.
    *   Set `/components/component/transceiver/config/enabled` to `true`: wait for link UP and verify all telemetry parameters are valid and within thresholds.

## Canonical OC

```json
{}
```

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
  # Config Parameter coverage
  /components/component/transceiver/config/enabled:
    platform_type: ["TRANSCEIVER"]
  /interfaces/interface/config/enabled:

  # Telemetry Parameter coverage
  /components/component/transceiver/state/enabled:
    platform_type: ["TRANSCEIVER"]
  /components/component/state/firmware-version:
    platform_type: ["TRANSCEIVER"]
  /components/component/state/mfg-name:
    platform_type: ["TRANSCEIVER"]
  /components/component/state/part-no:
    platform_type: ["TRANSCEIVER"]
  /components/component/state/serial-no:
    platform_type: ["TRANSCEIVER"]
  /components/component/state/temperature/instant:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/physical-channels/channel/state/index:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/physical-channels/channel/state/input-power/instant:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/physical-channels/channel/state/laser-bias-current/instant:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/physical-channels/channel/state/output-power/instant:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/state/supply-voltage/instant:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/state/form-factor:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/thresholds/threshold/state/input-power-lower:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/thresholds/threshold/state/input-power-upper:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/thresholds/threshold/state/laser-bias-current-lower:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/thresholds/threshold/state/laser-bias-current-upper:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/thresholds/threshold/state/module-temperature-lower:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/thresholds/threshold/state/module-temperature-upper:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/thresholds/threshold/state/output-power-lower:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/thresholds/threshold/state/output-power-upper:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/thresholds/threshold/state/severity:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/thresholds/threshold/state/supply-voltage-lower:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/thresholds/threshold/state/supply-voltage-upper:
    platform_type: ["TRANSCEIVER"]
  /interfaces/interface/state/oper-status:
  /interfaces/interface/state/physical-channel:
  /interfaces/interface/state/transceiver:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```
