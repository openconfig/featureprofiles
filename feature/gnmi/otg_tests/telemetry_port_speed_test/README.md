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

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

TODO(OCPATHS): Config paths TBD

```yaml
paths:
  ## Config Paths ##
  # TBD

  ## State Paths ##
  /interfaces/interface/state/oper-status:
  /interfaces/interface/ethernet/state/port-speed:
  /interfaces/interface/aggregation/state/lag-speed:

rpcs:
  gnmi:
    gNMI.Subscribe:
```


## Minimum DUT platform requirement

vRX
