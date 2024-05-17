# RT-2.15: IS-IS circuit-type point-to-point

## Summary

* IS-IS circuit-type point-to-point Test

## Topology

* ATE:port1 <-> DUT:port1

## Procedure

* Configure IS-IS for ATE port-1 and DUT port-1.
* Configure both DUT and ATE interfaces as ISIS type point-to-point.
    * Verify that IS-IS adjacency is coming up.
    * Verify the output of ST path displaying the interface circuit-type as broadcast in ISIS database/adj table

## Config Parameter coverage

* For prefix:

     *   /network-instances/network-instance/protocols/protocol/isis/

*   Parameters:

    *   interfaces/interface/config/circuit-type

## Telemetry Parameter coverage

*   For prefix:

    *   /network-instances/network-instance/protocols/protocol/isis/

*   Parameters:

    *   interfaces/interface/state/circuit-type
