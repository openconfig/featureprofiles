# RT-5.6: Interface Loopback mode

## Summary

Ensure Interface mode can be set to loopback mode and can be added as part of static LAG.

## Procedure

### TestCase-1:

*   Configure DUT port-1 to ATE port-1
*   Admin down ATE port-1 down
*   Verify DUT port-1 is down
*   Configure “loopback mode” set to “FACILITY”
*   Add port-1 as part of Static LAG (lacp mode static(on))
*   Validate that port-1 operational status is “UP”
*   Validate that  LAG port status is “UP”

### TestCase-2: (TBD)

*   Configure DUT port-1 to ATE port-1
*   Configure “loopback mode” set to ”TERMINAL”
*   Add port-1 as part of Static LAG (lacp mode static(on))
*   Validate that port-1 operational status is “UP”
*   Validate that  LAG port status is “UP”
*   send traffic from ATE port-1
*   Verify RX/TX on ATE port-1 should match(ATE port-1 packets sent to DUT port-1 in TERMINAL mode directs back to the ATE port-1)

## Config Parameter Coverage

*   /interfaces/interface/config/loopback-mode
*   /interfaces/interface/ethernet/config/port-speed
*   /interfaces/interface/ethernet/config/duplex-mode
*   /interfaces/interface/ethernet/config/aggregate-id
*   /interfaces/interface/aggregation/config/lag-type
*   /interfaces/interface/aggregation/config/min-links
*   /lacp/config/system-priority
*   /lacp/interfaces/interface/config/name
*   /lacp/interfaces/interface/config/interval
*   /lacp/interfaces/interface/config/lacp-mode
*   /lacp/interfaces/interface/config/system-id-mac
*   /lacp/interfaces/interface/config/system-priority

## Telemetry Parameter Coverage

*   /interfaces/interface/state/loopback-mode
*   /lacp/interfaces/interface/members/member/state/counters/lacp-in-pkts
*   /lacp/interfaces/interface/members/member/state/counters/lacp-out-pkts
*   /lacp/interfaces/interface/members/member/state/counters/lacp-rx-errors
*   /lacp/interfaces/interface/members/member/state/oper-key
*   /lacp/interfaces/interface/members/member/state/partner-id
*   /lacp/interfaces/interface/members/member/state/system-id
*   /lacp/interfaces/interface/members/member/state/port-num

## Protocol/RPC Parameter Coverage

None

## Minimum DUT Platform Requirement

vRX
