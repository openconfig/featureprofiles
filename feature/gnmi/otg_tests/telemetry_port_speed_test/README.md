# gNMI-1.5: Telemetry: Port Speed Test

## Summary

Validate port speed telemetry used by controller infrastructure.

## Procedure

*   For each port speed to be supported by the device:
    *   Connect single port to ATE, validate that the port speed reported in
        telemetry is the expected port speed.
    *   Turn port down at ATE, validate that operational status of the port is
        reported as down.
*   For each port speed to be supported by the device:
    *   Connect N ports between ATE and DUT configured as part of a LACP bundle.
        Validate /interfaces/interface/aggregation/state/lag-speed is reported
        as N*port speed.
    *   Disable each port at ATE and determine that the effective speed is
        reduced by the expected amount.
    *   Turn ports sequentially up at the ATE, and determine that the effective
        speed is increased as expected.

## Config Parameter Coverage

TBD

## Telemetry Parameter Coverage

/interfaces/interface/state/oper-status
/interfaces/interface/ethernet/state/port-speed
/interfaces/interface/aggregation/state/lag-speed

## Protocol/RPC Parameter Coverage

No new protocol coverage.

## Minimum DUT platform requirement

vRX
