# gNMI-1.27: gNMI Sample Mode Test

## Summary

Test to validate basic gNMI streaming telemetry works with `SAMPLE` mode.

## Procedure

### Test 1: Verify that correct `SAMPLE Mode` telemetry is streamed when Interface Description is updated

*   Create a new gNMI Subscription to Interface description `state` leaf in
    `SAMPLE` mode. with a 10 second interval

*   Configure Port-1 with description `DUT Port 1`.

*   Verify correct description is streamed.

*   Update Port-1 description to `DUT Port 1 - Updated`.

*   Verify correct description is streamed.

### Test 2: Verify that no invalid telemetry is streamed during state update

*   Create a new gNMI Subscription to Interface description `state` leaf in
    `SAMPLE` mode with a 10 second interval.

*   Configure Port-1 with description `DUT Port 1`.

*   Flap port 1 interface and wait for it to be UP.

*   Collect all the samples streamed and validate no invalid values were
    streamed during the flap.

### Test 3: Verify `SAMPLE Mode` telemetry is eventually consistent

*   Create a new gNMI Subscription to Default Network Instance `state` container
    in `SAMPLE` mode with a 10 second interval.

*   Configure ISIS on Port 1 in Default Network Instance.

*   Verify that ISIS telemetry is streamed within the next 5 samples.
