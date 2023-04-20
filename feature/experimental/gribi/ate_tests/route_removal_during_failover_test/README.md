# TE-13.2: gRIBI route DELETE during Failover 

## Summary

Validate gRIBI route flush during SSO

## Procedure

*   Connect ATE port-1 to DUT port-1, and ATE port-2 to DUT port-2.

*   Create 64 L3 sub-interfaces under DUT port-2 and corresponding 64 L3 sub-interfaces on ATE port-2.

*   Establish `gRIBI client` connection with DUT negotiating `FIB_ACK` as the requested ack_type.

*   Install 64 L3 sub-interfaces IP to NextHopGroup(NHGID: `1`) pointing to ATE port-2 as the nexthop.

*   Inject `1000` IPv4Entries(IPBlock1: `198.18.196.1/22`) in default VRF with NHGID: `1`.

*   Validate that the entries are installed as FIB_PROGRAMMED using getRPC.

*   Send traffic from ATE port-1 to prefixes in IPBlock1 and ensure traffic flows 100% and reaches ATE port-2.

*   Start flushing  IPv4Entries((IPBlock1: `198.18.196.1/22`) in default VRF with NHGID: `1`. Concurrently, trigger a supervisor switchover using gNOI `SwitchControlProcessor`  while IPBlock1 entries are only partially installed.

*   Following reconnection of the `gRIBI client` to a new master supervisor, validate if partially deleted entries of IPBlock1  are not present in the FIB using a get RPC.

*   Check for coredumps in the DUT and validate that none are present post failover.

*   Re-inject IPBlock1 in default VRF with NHGID: `1`.

*   Send traffic from ATE port-1 to prefixes in IPBlock1 and ensure traffic flows 100% and reaches ATE port-2.

## Protocol/RPC Parameter coverage

*   gNOI:
    *   System
        *   SwitchControlProcessor

## Config parameter coverage

## Telemery parameter coverage

*   CHASSIS:

    *   /components/component[name=<chassis>]/state/last-reboot-time
    *   /components/component[name=<chassis>]/state/last-reboot-reason

*   CONTROLLER_CARD:

    *   /components/component[name=<supervisor>]/state/redundant-role
    *   /components/component[name=<supervisor>]/state/last-switchover-time
    *   /components/component[name=<supervisor>]/state/last-switchover-reason/trigger
    *   /components/component[name=<supervisor>]/state/last-switchover-reason/details
    *   /components/component[name=<supervisor>]/state/last-reboot-time
    *   /components/component[name=<supervisor>]/state/last-reboot-reason
