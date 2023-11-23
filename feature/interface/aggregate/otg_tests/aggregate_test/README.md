# RT-5.2: Aggregate Interfaces

## Summary

Validate link operational status of Static LAG and LACP.

## Procedure

*   Connect ATE port-1 to DUT port-1, and ATE ports 2 through 9 to DUT ports
    2-9. Configure ATE and DUT ports 2-9 to be part of a LAG.
*   For both static LAG and LACP:
    *   Ensure that LAG is successfully negotiated, verifying port status for
        each of DUT ports 2-9 reflects expected LAG state via ATE and DUT
        telemetry.
    *   Ensure that configuring a minimum links setting for the LAG the entire
        interface is marked:
        *   Down when min-1 links are up
        *   Up when min links are up
        *   Up when >min links are up.
    *   TODO: Verify the above by sending flows between ATE port-1 targeted
        towards the LAG.

## Config Parameter Coverage

*   /interfaces/interface/ethernet/config/port-speed
*   /interfaces/interface/ethernet/config/duplex-mode
*   /interfaces/interface/ethernet/config/aggregate-id
*   /interfaces/interface/aggregation/config/lag-type
*   /interfaces/interface/aggregation/config/min-links
*   TODO: /lacp/config/system-priority
*   /lacp/interfaces/interface/config/name
*   TODO: /lacp/interfaces/interface/config/interval
*   /lacp/interfaces/interface/config/lacp-mode
*   TODO: /lacp/interfaces/interface/config/system-id-mac
*   TODO: /lacp/interfaces/interface/config/system-priority

## Telemetry Parameter Coverage

*   TODO: /lacp/interfaces/interface/members/member/state/counters/lacp-in-pkts
*   TODO: /lacp/interfaces/interface/members/member/state/counters/lacp-out-pkts
*   TODO:
    /lacp/interfaces/interface/members/member/state/counters/lacp-rx-errors
*   /lacp/interfaces/interface/name
*   /lacp/interfaces/interface/state/name
*   /lacp/interfaces/interface/members/member/interface
*   /lacp/interfaces/interface/members/member/state/interface
*   /lacp/interfaces/interface/members/member/state/oper-key
*   /lacp/interfaces/interface/members/member/state/partner-key
*   /lacp/interfaces/interface/members/member/state/partner-id
*   /lacp/interfaces/interface/members/member/state/system-id
*   /lacp/interfaces/interface/members/member/state/port-num
*   /interfaces/interface/ethernet/state/aggregate-id
