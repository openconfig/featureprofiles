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

    *   /components/component/[name=%s]/state/mfg-name
    *   /components/component/[name=%s]/transceiver/state/form-factor
    *   /components/component/[name=%s]/state/serial-no
    *   /components/component/[name=%s]/state/part-no
    *   /components/component/[name=%s]/state/firmware-version

    *   Using /interfaces/interface[name=%s]/state get the list of Interfaces and
        validate following leafs are set:

        *   /interfaces/interface[name=%s]/state/physical-channel
        *   /interfaces/interface[name=%s]/state/transceiver

    *   Verify that the list of transceivers received are the same. This is only a
        consistency check that the vendor implemented the model correctly.

        *   /components/component[name=%s]/state
        *   /interfaces/interface[name=%s]/state/transceiver

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
    *   Verify /interfaces/interface[name=%s]/state/oper-status is DOWN.
    *   Enable the DUT transceiver (power on module 3.3V supply)
    *   Verify /interfaces/interface[name=%s]/state/oper-status is UP.
    *   Repeat Step1 and Step2.

* Step 4: Verify the telemetry is updated after the interface is flapped.
    *   Disable/shutdown the interface of the DUT.
    *   Verify the optics input power and output power are updated to below the corresponding low alarm threshold.
    *   Verify /interfaces/interface[name=%s]/state/oper-status is DOWN
    *   Re-enable the interface of the DUT
    *   Verify /interfaces/interface[name=%s]/state/oper-status is UP.
    *   Repeat Step1 and Step2.

## Config Parameter coverage

*   /interfaces/interface/config/enabled
*   /components/component/transceiver/state/enabled (transceiver 3.3V power supply on/off)

## Telemetry Parameter coverage

*   /components/component/transceiver/physical-channels/channel/state/input-power/instant
*   /components/component/transceiver/physical-channels/channel/state/output-power/instant
*   /components/component/transceiver/physical-channels/channel/state/laser-bias-current/instant
*   /components/component/state/temperature/instant
*   /components/component[name=%s]/state/mfg-name
*   /components/component[name=%s]/transceiver/state/form-factor
*   /components/component/state/serial-no
*   /components/component[name=%s]/state/part-no
*   /components/component/state/firmware-version
*   /components/component/transceiver/thresholds/threshold/state/output-power-lower
*   /components/component/transceiver/thresholds/threshold/state/output-power-upper
*   /components/component/transceiver/thresholds/threshold/state/input-power-lower
*   /components/component/transceiver/thresholds/threshold/state/input-power-upper
*   /components/component/transceiver/thresholds/threshold/state/module-temperature-lower
*   /components/component/transceiver/thresholds/threshold/state/module-temperature-upper
*   /components/component/transceiver/thresholds/threshold/state/laser-bias-current-lower
*   /components/component/transceiver/thresholds/threshold/state/laser-bias-current-upper
*   /components/component/transceiver/thresholds/threshold/state/severity