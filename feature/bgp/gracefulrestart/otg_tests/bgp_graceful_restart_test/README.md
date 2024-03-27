# RT-1.4: BGP Graceful Restart

## Summary

BGP Graceful Restart

## Procedure

*   Establish BGP sessions between:
    *   ATE port-1 and DUT port-1
    *   ATE port-2 and DUT port-2
*   Validate received capabilities at DUT and ATE reflect support for graceful
    restart.
*   For IPv4 and IPv6 routes:
    *   (Receiving speaker) Advertise prefixes between the ATE ports, through
        the DUT. Trigger session restart from ATE port-2.
        *   Ensure that prefixes are propagated to ATE port-2 during the
            restart.
        *   Ensure that traffic can be forwarded between ATE port-1 and ATE
            port-2 during graceful restart time.
        *   Ensure that prefixes are withdrawn, and traffic cannot be forwarded
            between ATE port-1 and port-2 after the stale routes time expires.
    *   (Restarting speaker) Advertise prefixes between the ATE ports, through
        the DUT. Trigger DUT session restart by stopping the BGP process and determine
        that packets are:
        *   Forwarded between ATE port-1 and DUT port-1 for the duration of the
            specified stale routes time.
        *   Dropped after the stale routes timer has expired.
        *   Forwarded again between ATE port-1 and DUT port-1 after the session
            is re-established.

## Config Parameter Coverage

For prefixes:

*   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor

Parameters:

*   graceful-restart/config/enabled
*   graceful-restart/config/restart-time
*   graceful-restart/config/stale-routes-time
*   graceful-restart/config/helper-only

## Telemetry Parameter Coverage

*   afi-safis/afi-safi/afi-safi-name
*   afi-safis/afi-safi/graceful-restart/state/advertised
*   afi-safis/afi-safi/graceful-restart/state/peer-restart-time
*   afi-safis/afi-safi/graceful-restart/state/received
