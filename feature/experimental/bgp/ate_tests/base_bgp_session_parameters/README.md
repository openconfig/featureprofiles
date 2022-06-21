## RT-1.1: Base BGP Session Parameters

## Summary

    Base BGP session establishment, with required capabilities

## Procedure

    Establish BGP session on ATE and DUT with table-based configuration to cover:
        ○ Explicitly specified Router ID
        ○ MD5 password
        ○ Explicitly specified device-global same local AS number and remote AS number - iBGP
        ○ Explicitly specified device-global same local AS number and remote AS number - eBGP
        ○ Explicitly specified per-neighbor local AS number and remote AS number - eBGP
        ○ Explicitly specified local IP address
        ○ Explicit holdtime and keepalive interval
        ○ Explicitly specified retry interval
    ● Validate session state and capabilities received on ATE.
    ● Validate session state and capabilities received on DUT using telemetry.
    ● TODO: Terminate BGP session using NOTIFICATION message and ensure that device telemetry correctly   reports the error codes.

## Config Parameter Coverage

    For prefix:
        /network-instances/network-instance/protocols/protocol/bgp
    Parameters:
        config/as
        config/router-id
    For prefixes:
        /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group
        /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor
    Parameters:
        config/peer-as
        config/local-as
        config/description
        timers/config/hold-time
        timers/config/keepalive-interval
        timers/config/minimum-route-advertisement-interval

## Telemetry Parameter coverage
    For prefix:
        /network-instances/network-instance/protocols/protocol/bgp/
    Parameters:
        afi-safis/afi-safi/state/active
        afi-safis/afi-safi/state/afi-safi-name
        afi-safis/afi-safi/state/enabled
        afi-safis/afi-safi/state/messages/sent/last-notification-error-code
        afi-safis/afi-safi/state/messages/sent/last-notification-error-subcode
        afi-safis/afi-safi/state/messages/sent/last-notification-time
        state/admin-state
        state/auth-password
        state/description
        state/enabled
        state/established-transitions
        state/last-established
        state/local-as
        state/messages/received/last-notification-error-code
        state/messages/received/last-notification-error-subcode
        state/messages/received/last-notification-time
        state/messages/received/notification
        state/messages/received/NOTIFICATION
        state/messages/received/UPDATE
        state/messages/sent/NOTIFICATION
        state/messages/sent/UPDATE
        state/negotiated-hold-time
        state/neighbor-address
        state/peer-as
        state/peer-group
        state/peer-type
        state/queues/input
        state/queues/output
        state/remove-private-as
        state/session-state
        state/supported-capabilities
        timers/state/negotiated-hold-time
        transport/state/local-address
        transport/state/local-port
        transport/state/mtu-discovery
        transport/state/passive-mode
        transport/state/remote-address
        transport/state/remote-port
        state/peer-as
        transport/state/local-address
        transport/state/remote-address
        transport/state/remote-port

## Protocol/RPC Parameter coverage
    BGP
        ● OPEN
            ○ Version
            ○ My Autonomous System
            ○ BGP Identifier
            ○ Hold Time 

## Notes

   Explicit Keep alive timer in not supported. We do calculate relative keepalive timer from holdtimer value. Keepalive will be 1/3rd of negotiated hold timer.
