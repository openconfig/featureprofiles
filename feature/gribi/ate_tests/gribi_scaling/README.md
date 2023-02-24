# TE-14.1: gRIBI Scaling

## Summary

Test gRIBI scaling.

## Topology

*   Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2.
*   Create <DefaultVRFIPv4NHCount> number of L3 sub-interfaces under DUT port-2
    and corresponding L3 sub-interfaces on ATE port-2, sequentially starting
    from sub-interface unit 1.
*   Create 3 non default L3 VRFs:
    *   VRF-A contains DUT port-1.
    *   VRF-B contains no interface.
    *   VRF-C contains no interface.
*   Establish a gRIBI connection with DUT with `PRESERVE` mode `RIB_AND_FIB_ACK`
    mode.

## Procedure

1.  Install the following gRIBI entries into the DUT, and expect
    `FIB_PROGRAMMED` for all of them.

    Environment constants:

    *   StaticMAC = 00:1A:11:00:00:01
    *   IPBlockDefaultVRF = 198.18.64.1/18
    *   IPBlockNonDefaultVRF = 198.18.0.1/18

    Input variables to the tests:

    *   DefaultVRFIPv4Count
    *   DefaultVRFIPv4NHSize
    *   DefaultVRFIPv4NHGWeightSum
    *   DefaultVRFIPv4NHCount
    *   NonDefaultVRFIPv4Count
    *   NonDefaultVRFIPv4NHGCount
    *   NonDefaultVRFIPv4NHSize
    *   NonDefaultVRFIPv4NHGWeightSum
    *   DecapEncapCount

    (Unless specifically mentioned, all gRIBI objects are installed in the
    DEFAULT VRF)

    ```text
    NHG {ID #1} --> NH {ID #1, network-instance: VRF-C}

    NHG {ID #2} --> NH {ID #2, decap, network-instance: DEFAULT}

    <DefaultVRFIPv4Count> number of IPv4Entry (/32 from <IPBlockDefaultVRF>), each reference an unique NHGs. Each of the NHG contains <DefaultVRFIPv4NHSize> number of NHs, the NH sum weight is <DefaultVRFIPv4NHGWeightSum> in each NHG. The number of unique NHs are <DefaultVRFIPv4NHCount>. Each of the NH reference one unique sub-interface of DUT port-2, and does MAC address override to <StaticMAC>. Round-robing allocation of NHs to NHGs is ok.

    <NonDefaultVRFIPv4Count> number of IPv4Entry in VRF-A (/32 from <IPBlockNonDefaultVRF>) referencing to <NonDefaultVRFIPv4NHGCount> number of unique NHG. Round-robing allocation of NHGs to the IPs is ok. Each NHG contains <NonDefaultVRFIPv4NHSize> NHs, NH sum weight is <NonDefaultVRFIPv4NHGWeightSum>. Each NHG reference NHG #1 as the backup NHG. Here totally it's <NonDefaultVRFIPv4NHGCount x NonDefaultVRFIPv4NHSize> number of unique NHs. Each NH contains one next-hop IP (1:1 mapping to the <DefaultVRFIPv4Count> IPv4Entry above).

    <NonDefaultVRFIPv4Count> number of same IPv4Entry in VRF-B (as in VRF-A) referencing to <NonDefaultVRFIPv4NHGCount> number of unique NHG. Round-robing allocation of NHGs to the IPs is ok. Each NHG contains <NonDefaultVRFIPv4NHSize> NHs, NH sum weight is <NonDefaultVRFIPv4NHGWeightSum>. Each NHG reference NHG #2 as the backup NHG. Here the referenced NHs are not unique. They are the same as above that are used by IPs in VRF-A.

    <NonDefaultVRFIPv4Count> number of same IPv4Entry in VRF-B (as in VRF-A) referencing to <DecapEncapCount> number of unique NHG. Each of the NHG contains 1 unique NHs. Each of the NH does decap-and-encap to different destination addresses in <IPBlockNonDefaultVRF>, and points to VRF-B. Each NHG reference NHG#2 as the backup NHG.
    ```

2.  Send flows to the <NonDefaultVRFIPv4Count> destinations. Error out if any
    loss.
