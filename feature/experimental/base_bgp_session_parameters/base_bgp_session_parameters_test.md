# Local BGP Test

## Summary

The base_bgp_session_parameters_test brings up DUT - ATE 1-port devices and tests that a
BGP session can be established between them.

## WBB testcase ID 
   RT-1.1: Base BGP Session Parameters

## Topology

    DUT et-0/0/1 ---------------------- 1/1 ATE 

## Procedure
    1. Establish and Verify BGP sessions between DUT-ATE. Also Validate capabilites received.
    
    2. Test disconnect between bgp Peers using terminate on ATE and verify error code on the DUT.  (Pending on ixia support)
    
    3. Test BGP session with various test parameters.
        1. I-BGP session.
        2. E-BGP session.
        3. Explicit AS number under neighbor information.
        4. Explicit Router id configured under BGP.
        5. MD5 password authentication.
        6. Hold time configuration.
        7. Connect retry interval configuration.

## Parameter Coverage
*  /network-instances/network-instance/protocols/protocol/bgp/global/config/as
*  /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id
*  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/auth-password
*  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/local-as
*  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/neighbor-address
*  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as
*  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/neighbor-address
*  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/messages/received/last-notification-error-code
*  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state
*  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/supported-capabilities
*  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/hold-time
*  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/connect-retry"

## To do

    1.  Sending Notification from ATE is not  supported in ondatra framework. Waiting on ixia to support this in ondatra framework.

## Notes

    1.	Explicit Keep alive timer in not supported. We do calculate relative keepalive timer from holdtimer value. Keepalive will be 1/3rd of negotiated hold timer.
