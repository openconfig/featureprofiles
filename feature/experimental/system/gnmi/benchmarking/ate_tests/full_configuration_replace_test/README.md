# gNMI-1.2: Benchmarking: Full Configuration Replace 

## Summary

Measure performance of full configuration replace.

## Procedure

Configure DUT with:
 - The number of interfaces needed for the benchmarking test.
 - One BGP peer per interface.
 - One ISIS adjacency per interface.
Measure time required for Set operation to complete. 
Modify descriptions of a subset of interfaces within the system.
Measure time for Set to complete.

Notes:
This test does not measure the time to an entirely converged state, only to completion of the gNMI update.

## Config Parameter Coverage


## Telemetry Parameter Coverage


