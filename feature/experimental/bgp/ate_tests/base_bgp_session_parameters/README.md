# RT-1.1: Base BGP Session Parameters

## Summary

BGP session establishment between DUT - ATE and verifiying different session parameters.

## Topology

*   DUT port-1 ------- ATE port-1    

## Procedure

*   Establish eBGP session between DUT and ATE 

    *   Ensure session state should be Established.
    *   Verify BGP capabilities like Route refresh, ASN32 and MPBGP.

*   Verify BGP session disconnect by sending notification message from ATE

    *   Send Cease notification from ATE. 
    *   Ensure that DUT telemetry correctly reports the error code.

*   Establish BGP session and verify different session parameters. 
    BGP session should be established when different session parameters
    are provided and session parameters are applied correspondingly.

    *   Explicitly specified Router ID under bgp global level.
    *   Enable MD5 authentication on DUT and ATE.
    *   iBGP - Explicit same global AS is configured on DUT and ATE.
    *   eBGP - Explicit different global AS is configured on DUT and ATE.
    *   Explicit AS is configured on DUT under neighbor level.
    *   TODO: Explicit holdtime interval and keepalive interval.
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