# RT-5.4: Aggregate Forwarding Viable

## Summary

Ensure that forwarding-viable transition does not result in packet loss.
Ensure that setting forwarding-viable=false impact **only** transmitting traffic on given port.
Ensure that port set with forwarding-viable=false can receive all type of traffic and forward it normally (same as with forwarding-viable=true)

## Procedure

*   Connect ATE port-1 to DUT port-1, and ATE ports 2 through 9 to DUT ports 2-9. Configure ATE and DUT ports 2-9 to be part of a LAG.

*   For both Static LAG and LACP:
[TODO: https://github.com/openconfig/featureprofiles/issues/1553;]
    *   Run traffic bidirectionally between ATA port-1 and ATE port2-9;
        *   ensure all ports has bidirectional traffic.
        *   ensure that traffic is load-balanced across all port2-9
        *   ensure ther is no packet losses in steady state (no congestion).
    *   Make the forwarding-viable transitions from TRUE --> FALSE on a single port within the LAG on the DUT (one of ports 2-9).
        *   ensure that above DUT's port is not sending any traffic to ATE (hence corresponding ATE port do not recive any traffic)
        *   ensure that DUT load-balance traffic across remaing ports of LAG (7 out of port2-9)
        *   ensure that above DUT's port is receiving  traffic from coresponding ATE port
        *   ensure that there is no packet losses in steady state. (some very few packet could be lost right during transition)
        *   ensure that DUT recives ~equal traffic on all ports of LAG (all port2-9)
    *   Make the forwarding-viable transitions from FALSE --> TRUE on a single port within the LAG on the DUT (one of ports 2-9).
        *   ensure that above DUT's port is sending traffic to ATE (hence corresponding ATE port do recive traffic)
        *   ensure that DUT load-balance traffic across all ports of LAG (all port2-9)
        *   ensure that above DUT's port is receiving  traffic from coresponding ATE port
        *   ensure that there is no packet losses in steady state. (some very few packet could be lost right during transition)
        *   ensure that DUT recives load-balanced traffic across all ports of LAG (all port2-9)

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
