# RT-5.4: Aggregate Forwarding Viable

## Summary

Ensure that forwarding-viable transition does not result in packet loss.

## Procedure

*   Connect ATE port-1 to DUT port-1, and ATE ports 2 through 9 to DUT ports 2-9. Configure ATE and DUT ports 2-9 to be part of a LAG.

*   For both Static LAG and LACP:
    *   Make the following forwarding-viable transitions on a port within the LAG on the DUT.
        *   Transition from forwarding-viable=true to forwarding-viable=false.
        *   Transition from forwarding-viable=false to forwarding-viable=true.
*   For each transition above, ensure that traffic is load-balanced across the remaining interfaces without packet loss

## Config Parameter coverage

*   /interfaces/interface/ethernet/config/aggregate-id
*   /interfaces/interface/ethernet/config/forwarding-viable (from hercules) [ph1]
*   /interfaces/interface/aggregation/config/lag-type
*   /lacp/config/system-priority
*   /lacp/interfaces/interface/config/name
*   /lacp/interfaces/interface/config/interval
*   /lacp/interfaces/interface/config/lacp-mode
*   /lacp/interfaces/interface/config/system-id-mac
*   /lacp/interfaces/interface/config/system-priority

## Telemetry Parameter coverage

None

## Protocol/RPC Parameter coverage

None

## Minimum DUT platform requirement

vRX
