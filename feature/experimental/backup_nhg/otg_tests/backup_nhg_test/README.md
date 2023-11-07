# TE-11.1: Backup NHG: Single NH

## Summary

Ensure that backup NextHopGroup entries are honoured in gRIBI for NHGs
containing a single NH.

## Procedure

*   Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2, and ATE port-3
    to DUT port-3.
*   Create a non-default `VRF-B` that contains no interfaces.
*   Connect gRIBI client to DUT with persistence `PRESERVE`, redundancy
    `SINGLE_PRIMARY`, with election ID 1.
*   Install the following gRIBI structure: (if not specifically mentioned, the
    objects are installed in the DEFAULT VRF)

*   NHG#1 --> NH#1 {next-hop: ATEPort2IP}
*   NHG#2 --> NH#2 {next-hop: ATEPort3IP}
*   192.0.2.254/32 --> NHG#1
*   NHG#100 --> NH#100 {network-instance:VRF-B}
*   NHG#101 --> [NH#101 {next-hop: 192.0.2.254}, backupNHG: NHG#100]
*   198.51.100.0/32 {DEFAULT} --> NHG#101
*   198.51.100.0/32 {VRF-B} --> NHG#2
*   Validate:
    *   AFT telemetry shows the installed NHGs and NHs.
    *   Traffic is forwarded to ATE port-2 from ATE port-1.
*   For each of the following cases, ensure that traffic switches to being
    forwarded to ATE port-3:
    *   Interface ATE port-2 is disabled.
    *   Interface DUT port-2 is disabled.
*   Remove the entry for 192.0.2.254/32.

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
