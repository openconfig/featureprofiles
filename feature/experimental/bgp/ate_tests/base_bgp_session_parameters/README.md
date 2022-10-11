# RT-1.1: Base BGP Session Parameters

## Summary

BGP session establishment between DUT - ATE and verifiying different session parameters.

## Topology

    DUT port-1 -------- ATE port-1

## Procedure

Test the abnormal termination of session using notification message:

*   Establish BGP session between DUT (AS 65540) and ATE (AS 65550).

    *   Ensure session state should be `ESTABLISHED`.
    *   Verify BGP capabilities: route refresh, ASN32 and MPBGP.

*   Verify BGP session disconnect by sending notification message from ATE.

    *   Send `CEASE` notification from ATE.
    *   Ensure that DUT telemetry correctly reports the error code.

Test the normal session establishment and termination:

*   Establish BGP session for the following cases:

    *   eBGP using DUT AS 65540 and ATE AS 65550.
        *   Specifies global AS 65540 on the DUT.
        *   Specifies global AS 65536 and neighbor AS 65540 on the DUT.
            Verify that ATE sees peer AS 65540.
    *   iBGP using DUT AS 65536 and ATE AS 65536.
        *   Specifies global AS 65536 on the DUT.
        *   Specifies both global and neighbor AS 65536 on the DUT.

    And include the following session parameters for all cases:

    *   Explicitly specified Router ID.
    *   Enable MD5 authentication on DUT and ATE.
    *   Explicit holdtime interval and keepalive interval.
    *   Explicit connect retry interval.

## Config Parameter coverage

*   For prefix:

    *   /network-instances/network-instance/protocols/protocol/bgp/global

*   For Parameters:

    *   config/as
    *   config/router-id
    *   config/peer-as
    *   config/local-as
    *   config/description
    *   timers/config/hold-time
    *   timers/config/keepalive-interval
    *   timers/config/minimum-route-advertisement-interval

*   For prefixes:    

    *   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor
    
## Telemetry Parameter coverage   
    
*   For prefix:
    
    *   /network-instances/network-instance/protocols/protocol/bgp/

*   For Parameters:

    *   state/last-established
    *   state/messages/received/NOTIFICATION
    *   state/negotiated-hold-time
    *   state/supported-capabilities

## Protocol/RPC Parameter coverage

*   BGP
    
    *   OPEN
    
        *   Version
        *   My Autonomous System
        *   BGP Identifier
        *   Hold Time
