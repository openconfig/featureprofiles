# gNMI-1.2: Benchmarking: Full Configuration Replace 

## Summary

Measure performance of full configuration replace.

## Procedure

Configure DUT with:
 - Maximum number of interfaces to be supported.
 - Maximum number of BGP peers to be supported.
 - Maximum number of IS-IS adjacencies to be supported.
Measure time required for Set operation to complete. 
Modify descriptions of a subset of interfaces within the system.
Measure time for Set to complete.

Notes:
This test does not measure the time to an entirely converged state, only to completion of the gNMI update.

## Config Parameter Coverage


## Telemetry Parameter Coverage


