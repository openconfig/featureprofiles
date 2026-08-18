# TRANSCEIVER-5.1: Client Optics Telemetry, Instant, Threshold, and Miscellaneous Static Info

## Summary

Validate client optics related streaming telemetry performance monitoring parameters
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
