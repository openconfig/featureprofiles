# TE-4.2: Master Failover

## Summary

Validate gRIBI route persistence.

## Procedure

*   Connect ATE port-1 to DUT port-1, and ATE port-2 to DUT port-2.

*   Establish gRIBI connection to device (referred to as gRIBI-A), setting SINGLE_PRIMARY, and persistence to DELETE.

    *   Inject an IPv4Entryfor 1.0.0.0/8 pointed to a NHG containing a NH of ATE port-2. Ensure that traffic with a destination in 1.0.0.0/8 can be forwarded between ATE port-1 and port-2. Validate AFT entry is installed through telemetry.
    
    *   Disconnect the gRIBI-A client, and ensure that traffic can no longer be forwarded for a destination in 1.0.0.0/8 between ATE port-1 and port-2. Validate AFT entry is no longer installed through telemetry.
    
*   Establish gRIBI connection to device (referred to as gRIBI-A), setting SINGLE_PRIMARY and persistence to PRESERVE).

    *   Inject an IPv4Entry for 1.0.0.0/8 pointed to a NHG containing a NH of ATE port-2. Ensure that traffic with a destination in 1.0.0.0/8 can be forwarded between ATE port-1 and port-2. Validate AFT entry is installed through telemetry.
    
    *   Disconnect gRIBI-A client and ensure that traffic for a destination in 1.0.0.0/8 can still be forwarded between ATE port-1 and port-2. Validate that the entry continues to be installed in the AFT.
    
    *   Reconnect gRIBI-A using the same parameters, and delete the IPv4Entry for 1.0.0.0/8 created previously. Ensure that traffic can no longer be forwarded to a destination in 1.0.0.0/8, and that AFT telemetry indicates the entry is no longer installed.

## Protocol/RPC Parameter Coverage
*   gRIBI
    *   ModifyRequest
        *   SessionParameters:
            *   redundancy
            *   persistence


## Telemetry Parameter Coverage
*   AFT
    *   /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix/