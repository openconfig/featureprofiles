# RT-5.3: Aggregate Balancing

## Summary

Load balancing across members of a LACP-controlled LAG

## Procedure

*   Connect ATE port-1 to DUT port-1, and ATE ports 2 through 9 to DUT ports
    2-9. Configure ATE and DUT ports 2-9 to be part of a LACP-controlled LAG.
*   Send flows between ATE port-1 targeted towards the LAG consisting of ATE
    ports 2-9. Ensure that traffic is balanced across the LAG members with:
    *   TODO (TCP header): IPv4 packets with varying TCP source ports.
    *   TODO (TCP header): IPv6 packets with varying TCP source ports.
    *   IPv6 packets with varying flow labels.
    *   TODO (TCP header): IPinIP containing IPv4 payload with varying TCP
        source ports.
    *   TODO (TCP header): IPinIP containing IPv6 payload with varying TCP
        source port.
    *   TODO: IPinIP containing IPv6 payload with varying flow labels.

## Config Parameter coverage

*   /interfaces/interface/ethernet/config/aggregate-id
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
