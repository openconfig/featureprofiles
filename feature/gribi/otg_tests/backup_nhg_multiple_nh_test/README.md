# TE-11.2: Backup NHG: Multiple NH

## Summary

Ensure that backup NHGs are honoured with NextHopGroup entries containing >1 NH.

## Procedure
*   Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2, ATE port-3 to DUT port-3, and ATE port-4 to DUT port-4.
*   Connect a gRIBI client to the DUT and inject an IPv4Entry for 1.0.0.0/8 pointing to a NextHopGroup containing:
    *   Two primary next-hops:
        *   2: to ATE port-2
        *   3: to ATE port-3
    *   A backup NHG containing a single next-hop:
        *   4: to ATE port-4
*   Ensure that traffic forwarded to a destination in 1.0.0.0/8 is received at ATE port-2 and port-3. Validate that AFT telemetry covers this case.
*   Disable ATE port-2. Ensure that traffic for a destination in 1.0.0.0/8 is received at ATE port-3.
*   Disable ATE port-3. Ensure that traffic for a destination in 1.0.0.0/8 is received at ATE port-4.

## Config Parameter coverage

*   No new configuration covered.

## Telemetry Parameter coverage

*   No new telemetry covered.

## Protocol/RPC Parameter coverage

*   gRIBI:
    *   Modify
        *   ModifyRequest
            *   NextHopGroup
                *   backup_nexthop_group

## Minimum DUT platform requirement

vRX
