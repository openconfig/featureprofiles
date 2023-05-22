# TE-14.1: gRIBI Scaling

## Summary

Validate gRIBI scaling requirements.

## Procedure

*   Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2.
*   Create 64 L3 sub-interfaces under DUT port-2 and corresponding 64 L3
    sub-interfaces on ATE port-2
*   Establish gRIBI client connection with DUT negotiating FIBACK as the
    requested ack_type and make it become leader.
*   Using gRIBI Modify RPC install the following IPv4Entry sets, and validate
    the specified behaviours:
    *   <Default VRF> IPv4Entries -> NHG -> Multiple NH.
        *   Inject IPv4Entries(IPBlockDefaultVRF: 198.18.196.1/22) in default
            VRF
        *   Install 64 L3 sub-interfaces IP to NextHopGroup containing one
            NextHop specified to ATE port-2.
        *   Validate that the entries are installed as FIB_PROGRAMMED
    *   <VRF1> IPv4Entries -> Multiple NHG -> Multiple NH.
        *   Inject IPv4Entries(IPBlock1: "198.18.0.1/18") in VRF1.
        *   Install 1000 IPs from IPBlockDefaultVRF to 10 NextHopGroups
            containing 100 NextHops each
        *   Validate that the entries are installed as FIB_PROGRAMMED
    *   <VRF2> IPv4Entries -> Multiple NHG -> Multiple NH.
        *   Inject IPv4Entries(IPBlock2: "198.18.64.1/18") in VRF2.
        *   Install *repeat* 17.5K NH from 1K /32 from IPBlockDefaultVRF to 35
            NextHopGroups containing 45 NextHops each
        *   Validate that the entries are installed as FIB_PROGRAMMED
    *   <VRF3> IPv4Entries -> Multiple NHG -> Multiple NH.
        *   Inject IPv4Entries(IPBlock3: "198.18.128.1/18") in VRF3.
        *   Install IPiniP decap-then-encap to 500 first /32 from <IPBlockVRF1>
            to 500 NextHopGroups containing 1 NextHop each
        *   Validate that the entries are installed as FIB_PROGRAMMED
*   TODO: Add flows destinating to IPBlocks and ensure ATEPort2 receives it with
    no loss
