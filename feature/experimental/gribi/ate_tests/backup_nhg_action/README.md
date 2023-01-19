# TE-11.3: Backup NHG: Actions

## Summary

Validate gRIBI Backup NHG actions.

## Procedure

*   Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2, and ATE port-3 to DUT port-3 
*   Create a non-default VRF: VRF-1 that includes DUT port-1.
*   PrimaryTunnelDstIP: An /32 outer destination IP address for an IPinIP tunnel. 
    This test is expected to generate IPinIP traffic with this outer IP from one ATE port and received on another ATE port with the same outer IP.
*   SecondaryTunnelDstIP: An /32 outer destination IP address for another IPinIP tunnel. 
    This test is expected to generate IPinIP traffic with PrimaryTunnelDstIP as this outer IP from one ATE port and received on another ATE port with the SecondaryTunnelDstIP as the outer IP.
*   InnerDstIP: “a /32 inner destination IP address for an IPinIP tunnel”. This test is expected to generate IPinIP traffic with this inner IP from one ATE port and received on another ATE port with the outer IP decapsulated.
*   ATEPort2IP: ”testbed assigned interface IP to ATE port 2”
*   ATEPort3IP: ”testbed assigned interface IP to ATE port 3”
*   Create a static route in the default VRF for InnerDstIP, pointing to ATE port-3.
*   TEST#1 - (next-hop viability triggers decap in backup NHG)::
    *   Connect a gRIBI client to DUT with session parameters:
        persistence=PRESERVE
        Redundancy=SINGLE_PRIMARY

    *   Inject the gRIBI structure to the DUT
    *   Send IPinIP traffic to <PrimaryTunnelDstIP>, and validate that ATE port-2 receives the IPinIP traffic.
    *   Shutdown DUT port-2 interface, and validate that ATE port-3 receives the decapsulated traffic with InnerDstIP.
*   Test#2 - (new tunnel viability triggers decap in the backup NHG): 
    *   Remove gRIBI entries installed in Test#1, and bring back DUT port-2 to UP status.
    *   Inject the gRIBI structure to the DUT
    *   Send IPinIP traffic to <DstIPinIPOuterPrefix>.
        Validate that ATE port-2 receives the IPinIP traffic with  outer  IP as SecondaryTunnelDstIP.
    *   Shutdown DUT port-2 interface, and validate that ATE port-3 receives the traffic with the inner IP at ATE port-3.

## Config Parameter coverage

No new configuration covered.

## Telemetry Parameter coverage

No new telemetry covered.

## Protocol/RPC Parameter coverage

## Minimum DUT platform requirement

vRX if the vendor implementation supports FIB-ACK simulation, otherwise FFF.
