# Local BGP Test

## Summary

The local_bgp_test brings up two OpenConfig controlled devices and tests that a
BGP session can be established between them.

This test is suitable for running in a KNE environment.

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
*  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/keepalive-interval
