# TE-13.1: gRIBI route ADD during Failover 

## Summary

Validate gRIBI route persistence during SSO

## Procedure

*   Connect ATE port-1 to DUT port-1, and ATE port-2 to DUT port-2.

*   Create 64 L3 sub-interfaces under DUT port-2 and corresponding 64 L3 sub-interfaces on ATE port-2.

*   Establish `gRIBI client` connection with DUT negotiating `FIB_ACK` as the requested ack_type.

*   Install 64 L3 sub-interfaces IP to NextHopGroup(NHGID: `1`) pointing to ATE port-2 as the nexthop.

*   Inject `1000` IPv4Entries(`IPBlock1: 198.18.196.1/22`) in default VRF with NHGID: `1`.

*   Validate that the entries are installed as FIB_PROGRAMMED using Get RPC.

*   Send traffic from ATE port-1 to prefixes in IPBlock1, ensure traffic flows 100% and reaches ATE port-2, stop the traffic.

*   Start injecting another 1000 IPv4Entries(`IPBlock2: 198.18.100.1/22`) in default VRF with NHGID: #1. 

*   Check for gRIBI core dumps in the DUT and validate that none are present.

*   Concurrently, trigger a supervisor switchover using gNOI `SwitchControlProcessor` while `IPBlock2` entries are only partially installed.

*   Check for gRIBI core dumps in the DUT and validate that none are present post failover

*   Following reconnection of the gRIBI client to a new master supervisor, validate if partially ACKed entries of `IPBlock2` are present as FIB_PROGRAMMED using a Get RPC.

*   Re-inject `IPBlock2` in default VRF with NHGID: #1.

*   Send traffic from ATE port-1 to prefixes in `IPBlock1`, ensure traffic flows 100% and reaches ATE port-2.

*   Send traffic from ATE port-1 to prefixes in `IPBlock2` and ensure traffic flows 100% and reaches ATE port-2. 

## Protocol/RPC Parameter coverage

*   gNOI:
    *   System
        *   SwitchControlProcessor

## Config parameter coverage

## Telemery parameter coverage
