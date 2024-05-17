# RT-2.15: IS-IS circuit-type point-to-point

## Summary

* Base IS-IS functionality and adjacency establishment.
* Ensure that IS-IS adjacency is not coming up with two different circuit-type.

## Topology

* ATE:port1 <-> DUT:port1

## Procedure
* Configure IS-IS for ATE port-1 and DUT port-1.
* Configure DUT interface as ISIS type point-to-point.
* Ensure that IS-IS adjacency is not coming up on the interface.
* Verify the output of ST path displaying the interface circuit-type as point-to-point in ISIS database/adj table.
* Undo the ISIS circuit-type as point-to-point under the DUT interface
    * Verify that IS-IS adjacency for IPv4 and IPV6 address families are coming up
    * Verify the output of ST path displaying the interface circuit-type as broadcast in ISIS database/adj table
* Configure both DUT and ATE interfaces as ISIS type point-to-point
    * Redo the check as the previous step

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
