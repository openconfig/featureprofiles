# RT-2.16: IS-IS hello-interval and multiplier Configuration Test

## Summary

IS-IS hello-interval and multiplier Configuration Test

## Procedure

* Topology: ATE-port1<—> DUT–port1
    * Configure IS-IS for ATE port-1 and DUT port-1. 
    * Establish basic adjacency
* Baseline Configuration on the DUT:
    * Set the hello-interval to a standard value (e.g., 10 seconds).
    * Set the hello-multiplier to its default (typically 3) 
    * Check that the streaming telemetry values are reported correctly by the DUT.
* Adjusting Hello-Interval configuration on the DUT:
    * Change the hello-interval to a different value in the DUT.
    * Verify that IS-IS adjacency is coming up in the DUT.
    * Verify that the updated Hello-Interval time is reflected in isis adjacency output in the ATE.
    * Verify that the correct streaming telemetry values are reported correctly by the DUT.
* Adjusting Hello-Multiplier configuration on the DUT:
    * Change the hello-multiplier to a different value in the DUT.
    * Verify that IS-IS adjacency is coming up in the DUT.
    * Verify that the updated Hello-Multiplier is reflected in isis adjacency output in the ATE.
    * Verify that the correct streaming telemetry values are reported correctly by the DUT.

## Config Parameter Coverage

* For prefix: /network-instances/network-instance/protocols/protocol/isis/

* Parameters:

    * interfaces/interface/levels/level/timers/config/hello-interval
    * interfaces/interface/levels/level/timers/config/hello-multiplier

## Telemetry Parameter Coverage

* For prefix: 

    * /network-instances/network-instance/protocols/protocol/isis/

* Parameters:

    * interfaces/interface/levels/level/timers/state/hello-interval
    * interfaces/interface/levels/level/timers/state/hello-multiplier
    * interfaces/interface/levels/level/adjacencies/adjacency/state/adjacency-state

## Protocol/RPC Parameter Coverage

* IS-IS
