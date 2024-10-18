# TE-11.31: Backup NHG: Actions with PBF

## Summary

Validate gRIBI Backup NHG actions with PBF.

## Topology

*   Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2, ATE port-3 to
    DUT port-3, and ATE port-4 to DUT port-4.
*   Create a 3 non-default VRF:
    *   `VRF-A` that includes DUT port-1.
    *   `VRF-B` that includes no interface.
    *   `VRF-C` that includes no interface.
*   `OuterDstIP_1`, `OuterSrcIP_1`, `OuterDstIP_2`, `OuterSrcIP_2`: IPinIP outer
    IP addresses.
*   `InnerDstIP_1`, `InnerSrcIP_1`: IPinIP inner IP addresses.
*   `VIP_1`, `VIP_2`: IP addresses used for recursive resolution.
*   `ATEPort2IP`: testbed assigned interface IP to ATE port 2
*   `ATEPort3IP`: testbed assigned interface IP to ATE port 3
*   All the NHG and NH objects injected by gRIBI are in the DEFAULT VRF.
*   Add an empty decap VRF, `DECAP_TE_VRF`.
*   Add 4 empty encap VRFs, `ENCAP_TE_VRF_A`, `ENCAP_TE_VRF_B`, `ENCAP_TE_VRF_C`
    and `ENCAP_TE_VRF_D`.
*   Replace the existing VRF selection policy with `vrf_selection_policy_w` as
    in <https://github.com/openconfig/featureprofiles/pull/2217>

## Setups

Different test scenarios requires different setups.

*   Setup#1

    *   Make sure DUT port-2, port-3 and port-4 are up.
    *   Make sure there is a static route in the default VRF for `InnerDstIP_1`,
        pointing to ATE port-4.
    *   Connect a gRIBI client to DUT with session parameters
        `{persistence=PRESERVE Redundancy=SINGLE_PRIMARY}`
    *   gRIBI Flush the DUT.
    *   Inject the following gRIBI structure to the DUT:

        ```text
        VIP_1/32 {DEFAULT VRF} --> NHG#1 --> NH#1 {next-hop: ATEPort2IP}
        NHG#100 --> NH#100 {decap, network-instance:DEFAULT}
        NHG#101 --> [NH#101 {next-hop: VIP1}, backupNHG: NHG#100]
        OuterDstIP_1/32 {VRF-A} --> NHG#101
        ```

*   Setup#2

    *   Make sure DUT port-2, port-3 and port-4 are up.
    *   Make sure there is a static route in the default VRF for `InnerDstIP_1`,
        pointing to ATE port-4.
    *   Connect a gRIBI client to DUT with session parameters
        `{persistence=PRESERVE Redundancy=SINGLE_PRIMARY}`
    *   gRIBI Flush the DUT.
    *   Inject the following gRIBI structure to the DUT:

        ```text
        VIP_1/32 {DEFAULT VRF} --> NHG#1 --> NH#1 {next-hop: ATEPort2IP}
        VIP_2/32 {DEFAULT VRF} --> NHG#2 --> NH#2 {next-hop: ATEPort3IP}

        NHG#100 --> NH#100 {network-instance:VRF-B}
        NHG#101 --> [NH#101 {next-hop: VIP1}, backupNHG: NHG#100]
        OuterDstIP_1/32 {VRF-A} --> NHG#101

        NHG#103 --> NH#103 {decap, network-instance:DEFAULT}
        NHG#102 --> [NH#102 {decap-encap, src:`OuterSrcIP_2`, dst:`OuterDstIP_2`, network-instance: VRF-C}, backupNHG: NHG#103]
        OuterDstIP_1/32 {VRF-B} --> NHG#102

        NHG#104 --> [NH#104 {next-hop: VIP-2}, backupNHG: NHG#103]
        OuterDstIP_2/32 {VRF-C} --> NHG#104
        ```

## Procedure

*   TEST#1 - (next-hop viability triggers decap in backup NHG):

    1.  Deploy Setup#1 as above.

    2.  Send IPinIP traffic to `OuterDstIP_1` with inner IP as `InnerDstIP_1`,
        and validate that ATE port-2 receives the IPinIP traffic.

    *   Shutdown DUT port-2 interface, and validate that ATE port-4 receives the
        decapsulated traffic with `InnerDstIP_1`.

*   Test#2 - (tunnel viability triggers decap and encap in the backup NHG):

    *   Deploy Setup#2 as above.

    *   Send IPinIP traffic to `OuterDstIP_1`. Validate that ATE port-2 receives
        the IPinIP traffic with outer IP as `OuterDstIP_1`.

    *   Shutdown DUT port-2 interface, and validate that ATE port-3 receives the
        IPinIP traffic with outer destination IP as `OuterDstIP_2`, and outer
        source IP as `OuterSrcIP_2`

    *   Shutdown DUT port-3 interface, and validate that ATE port-4 receives the
        traffic with decapsulated traffic with destination IP as `InnerDstIP_1`
        at ATE port-4.

*   Test#3 - (tunnel viability triggers decap):

    *   Deploy Setup#2 as above and inject below gRIBI structure to the DUT:
        
        ```text
        NHG#102 --> [NH#104 {decap-encap, src:OuterSrcIP_2, dst:OuterDstIP_FAILURE, network-instance: VRF-C}, backupNHG: NHG#103]
        ```

    *   Send IPinIP traffic to `OuterDstIP_1`. Validate that ATE port-2 receives
        the IPinIP traffic with outer IP as `OuterDstIP_1`.

    *   Shutdown DUT port-2 interface, and validate that ATE port-4 receives the
        traffic with decapsulated traffic with destination IP as `InnerDstIP_1`
        at ATE port-4.

*   Test#4 - (resolution failure on new tunnels triggers decap in the backup NHG):

    *   Deploy Setup#2 as above.

    *   Remove `OuterDstIP_2/32` from `VRF-C`.

    *   Send IPinIP traffic to `OuterDstIP_1`. Validate that ATE port-2 receives
        the IPinIP traffic with outer IP as `OuterDstIP_1`.

    *   Shutdown DUT port-2 interface, and validate that ATE port-4 receives the
        traffic with decapsulated traffic with destination IP as `InnerDstIP_1`
        at ATE port-4.

## OpenConfig Path and RPC Coverage
```yaml
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
  gribi:
    gRIBI.Get:
    gRIBI.Modify:
    gRIBI.Flush:
```

## Minimum DUT platform requirement

vRX if the vendor implementation supports FIB-ACK simulation, otherwise FFF.

