# TE-11.2: Backup NHG: Multiple NH

## Summary

Ensure that backup NHGs are honoured with NextHopGroup entries containing >1 NH.

## Procedure

*   Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2, ATE port-3 to
    DUT port-3, and ATE port-4 to DUT port-4.
*   Create a L3 routing instance (VRF-A), and assign DUT port-1 to VRF-A.
*   Create a L3 routing instance (VRF-B) that includes no interface.
*   Connect a gRIBI client to the DUT, make it become leader and inject the
    following:
    *   An IPv4Entry in VRF-A, pointing to a NextHopGroup (in DEFAULT VRF)
        containing:
        *   Two primary next-hops:
            *   IP of ATE port-2
            *   IP of ATE port-3
        *   A backup NHG containing a single next-hop pointing to VRF-B.
    *   The same IPv4Entry but in VRF-B, pointing to a NextHopGroup (in DEFAULT
        VRF) containing a primary next-hop to the IP of ATE port-4.
*   Ensure that traffic forwarded to the destination is received at ATE port-2
    and port-3. Validate that AFT telemetry covers this case.
*   Disable ATE port-2. Ensure that traffic for the destination is received at
    ATE port-3.
*   Disable ATE port-3. Ensure that traffic for the destination is received at
    ATE port-4.

[TODO]: Repeat the above tests with one additional scenario with the following changes, and it should not change the expected test result.

*   Add an empty decap VRF, `DECAP_TE_VRF`.
*   Add 4 empty encap VRFs, `ENCAP_TE_VRF_A`, `ENCAP_TE_VRF_B`, `ENCAP_TE_VRF_C` and `ENCAP_TE_VRF_D`.
*   Replace the existing VRF selection policy with `vrf_selection_policy_w` as in <https://github.com/openconfig/featureprofiles/pull/2217>

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
