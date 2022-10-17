# TE-8.1: DUT Daemon Failure


## Summary

Ensure that gRIBI entries are persisted over daemon failure.


## Procedure

*   Connect ATE port-1 to DUT port-1, and ATE port-2 to DUT port-2.

*   Establish a gRIBI client connection to the DUT (SINGLE_PRIMARY and PRESERVE mode).

    *   Inject an IPv4Entry for 203.0.113.0/24 pointed to a NHG containing a NH
        of ATE port-2. Ensure that traffic with a destination in 203.0.113.0/24
        can be forwarded between ATE port-1 and port-2. Verify through AFT
        telemetry that the route is installed.

*   Kill gRIBI daemon on DUT using gNOI test command (gNOI KillProcessRequest).

*   Validate:

    *   Traffic can continue to be forwarded between ATE port-1 and port-2.

    *   Through AFT telemetry that the route entries remain present.

    *   Following daemon restart, the gRIBI client connection can be re-established.

    *   Issuing a gRIBI Get RPC results in 203.0.113.0/24 being returned.


## Protocol/RPC Parameter Coverage

*   gRIBI
    *   ModifyRequest
    *   GetRequest


## Telemetry Parameter Coverage

*   AFT
    *   /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix/
