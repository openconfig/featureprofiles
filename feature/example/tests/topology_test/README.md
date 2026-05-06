# example-0.1: Topology Test

## Summary

Tests that ports an be successfully configured in a basic topology.

## Procedure

* Configure the ports on the DUT.
* Check that each port on the DUT has /config telemetry.
* Check that each port on the DUT has /state telemetry.
* Configure the ports on the ATE.
* Start control plane protocols on the ATE.
* Check that the link state on each DUT port is UP.
* Check that the link state on each ATE port is UP.

## Config Parameter coverage

No configuration coverage.

## Telemetry Parameter coverage

* /interfaces/interface/config
* /interfaces/interface/state
* /interfaces/interface/state/oper-status

## Protocol/RPC Parameter coverage

N/A

## Minimum DUT platform requirement

N/A
