# P4RT-1.2: P4RT Daemon Failure

## Summary

Ensure that data plane traffic is not interrupted by P4RT daemon failure.

## Procedure
* P4RT-1.2.1
  * Connect ATE port-1 to DUT port-1, and ATE port-2 to DUT port-2.
  * Establish a gRIBI client connection to the DUT (SINGLE_PRIMARY and PRESERVE
    mode) and make it become leader.
 *  Inject an IPv4 Entry for 203.0.113.0/24 pointed to a NHG containing a NH
        of ATE port-2. Ensure that traffic with a destination in 203.0.113.0/24
        can be forwarded between ATE port-1 and port-2. Verify through AFT
        telemetry that the route is installed.
* gNMI Subscribe ON_CHANGE to `/interfaces/interface/id`
*   Kill P4RT daemon on DUT using gNOI test command (gNOI.KillProcess).
*   Validate:
    *   `/interfaces/interface/id` does not change
    *   Traffic can continue to be forwarded between ATE port-1 and port-2.

* NOTE: P4RT is not being used to configure the data plane in this test because our
    test tables only configure the control plane traffic. Instead, this test
    configures the data plane using gRIBI. 

## OpenConfig Path and RPC Coverage
```yaml
rpcs:
  gribi:
    gRIBI.Get:
    gRIBI.Modify:
    gRIBI.Flush:
```

## Telemetry Parameter Coverage
*   /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix/
*   /interfaces/interface/state/id
*   /interfaces/interface/state/name
