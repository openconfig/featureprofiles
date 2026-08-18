# TRANSCEIVER-20.1: Client Optics DOM Telemetry, Instant, Threshold, and Miscellaneous Static Info

## Summary

Validate client optics digital optical monitoring (DOM) streaming telemetry performance monitoring parameters
like input power, output power, bias current, supply voltage, module temperature, and so on.

## Setup

Optics test requirements alongside the platform functional tests require optics DUT samples connected across optical ethernet interfaces.

## Procedure

*   Step 1: Using `/components/component[name=%s]/state`, get the list of transceivers and validate the following leaves are set:
    *   `/components/component/state/mfg-name`
    *   `/components/component/transceiver/state/form-factor`
    *   `/components/component/state/serial-no`
    *   `/components/component/state/part-no`
    *   `/components/component/state/firmware-version`
    *   `/interfaces/interface/state/physical-channel`
    *   `/interfaces/interface/state/transceiver`

*   Step 2: Get list of components of type `TRANSCEIVER`. Verify the instant value is between the corresponding lower and upper thresholds for both `[severity]=WARNING` and `[severity]=CRITICAL`:
    *   Module case temperature
    *   Tx output power
    *   Rx input power
    *   Laser bias-current
    *   Supply voltage

*   Step 3: Flap the interface and verify telemetry updates:
    *   Disable/shutdown interface: verify oper-status DOWN and output power drops.
    *   Re-enable interface: verify oper-status UP and output power returns to normal.

*   Step 4: Verify transceiver config enabled on/off:
    *   Set `/components/component/transceiver/config/enabled` to `false`: verify oper-status is not UP and transceiver disabled.
    *   Set `/components/component/transceiver/config/enabled` to `true`: verify oper-status UP, optics power and laser normal.

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
