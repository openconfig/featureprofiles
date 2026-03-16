# gNMI-1.13: Optics Telemetry, Instant, threshold, and miscellaneous static info

## Summary

Validate optics related streaming telemetry performance monitoring parameters
like input power, output power, bias current and so on.

## Setup
Optics test requirements alongside the platform funtional tests require 2
optics DUT samples each optics part number, connect both two optical ethernet interfaces
to Automatic Test Equipment (ATE). 


## Procedure

*   Connect at least one optical ethernet interface to ATE.
*   Step 1: Using components/component/[name=%s]/state get the list of transceivers and validate
    following leafs are set:

    *   /components/component/state/mfg-name
    *   /components/component/transceiver/state/form-factor
    *   /components/component/state/serial-no
    *   /components/component/state/part-no
    *   /components/component/state/firmware-version

    *   Using /interfaces/interface/state get the list of Interfaces and
        validate following leafs are set:

        *   /interfaces/interface/state/physical-channel
        *   /interfaces/interface/state/transceiver

    *   Verify that the list of transceivers received are the same. This is only a
        consistency check that the vendor implemented the model correctly.

        *   /components/component/state
        *   /interfaces/interface/state/transceiver

*   Step 2: Get list of components of type TRANSCEIVER. Verify the instant value is
    between the corresponding lower and upper thresholds for both
    [severity]=WARNING and [severity]=CRITICAL. In case of multiple physical
    channels or lanes relevant PMs like TX and RX power should be reported for
    all the lanes. 
    *   Module case temperature
        *   /components/component/transceiver/thresholds/threshold/state/module-temperature-lower
        *   /components/component/transceiver/thresholds/threshold/state/module-temperature-upper
        *   /components/component/transceiver/thresholds/threshold/state/severity
        *   /components/component/state/temperature/instant
    *   Tx output power
        *   /components/component/transceiver/thresholds/threshold/state/output-power-lower
        *   /components/component/transceiver/thresholds/threshold/state/output-power-upper
        *   /components/component/transceiver/thresholds/threshold/state/severity
        *   /components/component/transceiver/physical-channels/channel/state/output-power/instant
    *   Rx input power
        *   /components/component/transceiver/thresholds/threshold/state/input-power-lower
        *   /components/component/transceiver/thresholds/threshold/state/input-power-upper
        *   /components/component/transceiver/thresholds/threshold/state/severity
        *   /components/component/transceiver/physical-channels/channel/state/input-power/instant
    *   Laser bias-current
        *   /components/component/transceiver/thresholds/threshold/state/laser-bias-current-lower
        *   /components/component/transceiver/thresholds/threshold/state/laser-bias-current-upper
        *   /components/component/transceiver/thresholds/threshold/state/severity
        *   /components/component/transceiver/physical-channels/channel/state/laser-bias-current/instant

* Step 3: 
    *   Verify the telemetry is updated after the optics power cycle.
    *   Disable the DUT transceiver (power off module 3.3V supply).
    *   Verify /interfaces/interface/state/oper-status is DOWN.
    *   Enable the DUT transceiver (power on module 3.3V supply)
    *   Verify /interfaces/interface/state/oper-status is UP.
    *   Repeat Step1 and Step2.

* Step 4: Verify the telemetry is updated after the interface is flapped.
    *   Disable/shutdown the interface of the DUT.
    *   Verify the optics input power and output power are updated to below the corresponding low alarm threshold.
    *   Verify /interfaces/interface/state/oper-status is DOWN
    *   Re-enable the interface of the DUT
    *   Verify /interfaces/interface/state/oper-status is UP.
    *   Repeat Step1 and Step2.

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
  /components/component/transceiver/physical-channels/channel/state/input-power/instant:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/physical-channels/channel/state/laser-bias-current/instant:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/physical-channels/channel/state/output-power/instant:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/state/form-factor:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/state/vendor:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/state/vendor-part:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/state/vendor-rev:
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

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```
