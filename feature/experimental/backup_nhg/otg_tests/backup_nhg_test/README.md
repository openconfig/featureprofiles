# TE-11.1: Backup NHG: Single NH

## Summary

Ensure that backup NextHopGroup entries are honoured in gRIBI for NHGs
containing a single NH.

## Procedure

*   Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2, and ATE port-3
    to DUT port-3.
*   TODO: Create a non-default VRF, VRF-3, which includes DUT port-3.
*   TODO: Create a static default route in the VRF-3, pointing to ATE port-3.
*   Connect gRIBI client to DUT with persistence `PRESERVE`, redundancy
    `SINGLE_PRIMARY`, with election ID 1.
*   Install an IPv4Entry in the default VRF pointing to ATE port-2 for prefix
    198.51.100.0/24, with a backup nexthop group pointing to ATE port-3 (TODO:
    change ATE port-3 to VRF-3).
*   Validate:
    *   AFT telemetry shows next-hop-group of DUT port-2 being selected for
        198.51.100.0/24.
    *   Traffic is forwarded to ATE port-2 from ATE port-1.
*   For each of the following cases, ensure that traffic switches to being
    forwarded to ATE port-3:
    *   Interface ATE port-2 is disabled.
    *   Interface DUT port-2 is disabled.
*   Remove all previously installed IPv4Entry routes. Create an entry for
    198.51.100.0/24 with a next-hop of 192.0.2.254/32. Inject a second entry
    with 192.0.2.254/32 resolved to ATE port-2. Specify a backup NHG pointing to
    ATE port-3 (TODO: change ATE port-3 to VRF-3) for the 198.51.100.0/24
    entry’s NHG.
    *   Remove the entry for 192.0.2.254/32, and ensure that traffic is
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
