# TE-4.2: Leader Failover

## Summary

Validate gRIBI route persistence.

## Procedure

*   Connect ATE port-1 to DUT port-1, and ATE port-2 to DUT port-2.

*   Establish gRIBI connection to device (referred to as gRIBI-A), setting
    SINGLE_PRIMARY, and persistence to DELETE. Make it become leader.

    *   Inject an IPv4Entry for 203.0.113.0/24 pointed to a NHG containing a NH
        of ATE port-2. Ensure that traffic with a destination in 203.0.113.0/24
        can be forwarded between ATE port-1 and port-2. Validate AFT entry is
        installed through telemetry.

    *   Disconnect the gRIBI-A client, and ensure that traffic can no longer be
        forwarded for a destination in 203.0.113.0/24 between ATE port-1 and
        port-2. Validate AFT entry is no longer installed through telemetry.

*   Establish gRIBI connection to device (referred to as gRIBI-A), setting
    SINGLE_PRIMARY and persistence to PRESERVE. Make it become leader.

    *   Inject an IPv4Entry for 203.0.113.0/24 pointed to a NHG containing a NH
        of ATE port-2. Ensure that traffic with a destination in 203.0.113.0/24
        can be forwarded between ATE port-1 and port-2. Validate AFT entry is
        installed through telemetry.

    *   Disconnect gRIBI-A client and ensure that traffic for a destination in
        203.0.113.0/24 can still be forwarded between ATE port-1 and port-2.
        Validate that the entry continues to be installed in the AFT.

    *   Reconnect gRIBI-A using the same parameters, and delete the IPv4Entry
        for 203.0.113.0/24 created previously. Ensure that traffic can no longer
        be forwarded to a destination in 203.0.113.0/24, and that AFT telemetry
        indicates the entry is no longer installed.

## Protocol/RPC Parameter Coverage

*   gRIBI
    *   ModifyRequest
        *   SessionParameters:
            *   redundancy
            *   persistence

## Telemetry Parameter Coverage

*   AFT
    *   /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix/
