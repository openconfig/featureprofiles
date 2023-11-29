# TE-11.4: Gribi Programming with reload

## Summary

Ensure that gribi programming is honored following chassis reload while returning appropriate error message if the system is not ready and making new connection requests within the timeout value set. 

## Topology

*   Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2 and ATE port-3 to
        DUT port-3
*   Create 2 non-default VRF:
    *   `VRF-A` that includes DUT port-1.
    *   `VRF-B` includes no interface.
*   `OuterDstIP_1`, `OuterSrcIP_1`, `OuterDstIP_2`, `OuterSrcIP_2`: IPinIP outer
    IP addresses.
*   `InnerDstIP_1`, `InnerSrcIP_1`: IPinIP inner IP addresses.
*   `VIP_1`: IP addresses used for recursive resolution.
*   `ATEPort1IP`: testbed assigned interface IP to ATE port 1
*   `ATEPort2IP`: testbed assigned interface IP to ATE port 2
*   `ATEPort3IP`: testbed assigned interface IP to ATE port 3
*   All the NHG and NH objects injected by gRIBI are in the DEFAULT VRF.

## Setups

Different test scenarios requires different setups.

*   Setup#1

    *   Make sure DUT port-1, port-2 and port-3 are up.
    *   Make sure there is a static route in the default VRF for `InnerDstIP_1`,
        pointing to ATE port-3.
    *   Connect a gRIBI client to DUT with session parameters
        `{persistence=PRESERVE Redundancy=SINGLE_PRIMARY}`
    *   gRIBI Flush the DUT.
    *   Inject the following gRIBI structure to the DUT:

        ```text
            NHG#101 --> NH#101 {decap, network-instance:DEFAULT}
            NHG#100 --> [NH#100 {decap-encap, src:`OuterSrcIP_2`, dst:`OuterDstIP_2`, network-instance: VRF-B}, backupNHG: NHG#101]
            OuterDstIP_1/32 {VRF-A} --> NHG#100

            VIP_1/32 {DEFAULT VRF} --> NHG#1 --> NH#1 {next-hop: ATEPort2IP}
            NHG#2 --> [NH#2 {next-hop: VIP_1}]
            OuterDstIP_2/32 {VRF-B} --> NHG#2
        ```

*   Setup#2

    *   Make sure DUT port-1, port-2 and port-3 are up.
    *   Connect a gRIBI client to DUT with session parameters
        `{persistence=PRESERVE Redundancy=SINGLE_PRIMARY}`
    *   gRIBI Flush the DUT.
    *   Inject the following gRIBI structure to the DUT:

        ```text
            NHG#101 --> [NH#101 {decap-encap, src:`OuterSrcIP_2`, dst:`OuterDstIP_2`, network-instance: VRF-B}]
            VIP_1/32 {DEFAULT VRF} --> NHG#1 --> NH#1 {next-hop: ATEPort2IP}
            NHG#100 --> [NH#100 {next-hop: VIP_1}, backupNHG: NHG#101] 
            OuterDstIP_1/32 {VRF-A} --> NHG#100
            
            VIP_2/32 {DEFAULT VRF} --> NHG#2 --> NH#2 {next-hop: ATEPort3IP}
            NHG#102 --> [NH#102 {next-hop: VIP_2}]
            OuterDstIP_2/32 {VRF-B} --> NHG#102
        ```

## Procedure

*   TEST#1 

    1.  Deploy Setup#1 as above.

    2.  Send IPinIP traffic to `OuterDstIP_1` with inner IP as `InnerDstIP_1` and validate that ATE port-2 receives the traffic after DECAP-ENCAP over the primary path with outer destination IP as `OuterDstIP_2` and outer source IP as `OuterSrcIP_2`.
    
    3.  Perform entire device reload.

    4.  Redeploy Setup#1 as above. If the system is not ready, gRIBI returns UNAVAILABLE till it is ready, and indeed reattempts will be made every 30 seconds with max timeout of 3 minutes as per the client behavior.
        
        T0 -> System Up.
        T1 -> EMSD process bring up (gRPC port up).
        T2 -> gRIBI registers Service.
        T3 -> Waiting for gRIBI XR verticals to be up.
        T4 -> gRIBI accepts Programming (Afer gRIBI XR verticals are in sync), UNAVAILABLE is returned between T2-T4 if not ready.

    5.  Shutdown DUT port-2 interface.
    
    6.  Validate that ATE port-3 receives the decapsulated traffic with `InnerDstIP_1` over backup path.

*   Test#2

    1.  Deploy Setup#2 as above.

    2.  Send IPinIP traffic to `OuterDstIP_1` with inner IP as `InnerDstIP_1` and validate that ATE port-2 receives the traffic over the primary path.
    
    3.  Perform entire device reload.

    4.  Redeploy Setup#1 as above. If the system is not ready, gRIBI returns UNAVAILABLE till it is ready, and indeed reattempts will be made every 30 seconds with max timeout of 3 minutes as per the client behavior.
        
        T0 -> System Up.
        T1 -> EMSD process bring up (gRPC port up).
        T2 -> gRIBI registers Service.
        T3 -> Waiting for gRIBI XR verticals to be up.
        T4 -> gRIBI accepts Programming (Afer gRIBI XR verticals are in sync), UNAVAILABLE is returned between T2-T4 if not ready.

    5.  Shutdown DUT port-2 interface.
    
    6.  Validate that ATE port-3 receives the traffic over the backup DECAP-ENCAP path with outer destination IP as `OuterDstIP_2` and outer source IP as `OuterSrcIP_2`.

## Config Parameter coverage

*   No new configuration covered.

## Telemetry Parameter coverage

*   No new telemetry covered.

## Protocol/RPC Parameter coverage

## Minimum DUT platform requirement

vRX
