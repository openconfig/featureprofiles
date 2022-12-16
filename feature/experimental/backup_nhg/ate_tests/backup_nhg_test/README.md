# TE-11.1: Backup NHG: Single NH

## Summary

Ensure that backup NextHopGroup entries are honoured in gRIBI for NHGs
containing a single NH.

## Procedure

*   Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2, and ATE port-3
    to DUT port-3.
*   Create a non-default VRF, VRF-1. Assign DUT port-1 to VRF-1.
*   All GRIBI NHGs and NHs will be installed to the default VRF.
*   Connect gRIBI client to DUT with persistence `PRESERVE`, redundancy
    `SINGLE_PRIMARY`, with election ID 1.

To validate GRIBI backup next hop group behavior:

*   Install a GRIBI IPv4Entry in vrf VRF-1 for the prefix 198.51.100.0/24
    pointing to ATE port-2, with the backup nexthop group pointing to the
    default VRF.
*   Install a GRIBI IPv4Entry in the default VRF for the prefix 198.51.100.0/24
    pointing to ATE port-3.
*   Validate:
    *   AFT telemetry shows next-hop-group of DUT port-2 being selected for
        198.51.100.0/24.
    *   Traffic is forwarded to ATE port-2 from ATE port-1.
*   For each of the following cases, ensure that traffic switches to using ATE
    port-3:
    *   Interface ATE port-2 is disabled.
    *   Interface DUT port-2 is disabled.

To validate hierarchical backup next hop group behavior:

*   Remove all previously installed GRIBI entries.
*   Install a GRIBI IPv4Entry for the prefix 198.51.100.0/24 in vrf VRF-1
    destined to 192.0.2.254 in the default VRF, with the backup nexthop group
    pointing to the default VRF.
*   Install a GRIBI IPv4Entry for the prefix 192.0.2.254/32 in the default VRF
    destined to ATE port-2.
*   Install a GRIBI IPv4Entry for the prefix 198.51.100.0/24 in the default VRF
    destined to ATE port-3.
*   Validate that traffic is forwarded to ATE port-2 for destinations in
    198.51.100.0/24.
*   After removing the IPv4Entry for 192.0.2.254/32, validate that traffic is
    forwarded to ATE port-3 for destinations in 198.51.100.0/24.

## Config Parameter coverage

No new configuration covered.

## Telemetry Parameter coverage

No new telemetry covered.

## Protocol/RPC Parameter coverage

*   gRIBI
    *   Modify
        *   ModifyRequest
            *   NextHopGroup
                *   backup_nexthop_group

## Minimum DUT platform requirement

vRX if the vendor implementation supports FIB-ACK simulation, otherwise FFF.
