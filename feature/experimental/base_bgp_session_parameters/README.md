RT-1.1: Base BGP Session Parameters

Summary

    Base BGP session establishment, with required capabilities

Procedure

    Establish BGP session on ATE and DUT with table-based configuration to cover:
        ○ Explicitly specified Router ID
        ○ MD5 password
        ○ Explicitly specified local and remote AS number
        ○ Explicitly specified local IP address
        ○ Explicit holdtime and keepalive interval
        ○ Explicitly specified retry interval
    ● Validate session state and capabilities received on ATE.
    ● Validate session state and capabilities received on DUT using telemetry.
    ● Terminate BGP session using NOTIFICATION message and ensure that device telemetry correctly   reports the error codes.

Config Parameter Coverage

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

To do

    1.  Sending Notification from ATE is not  supported in ondatra framework. Waiting on ixia to support this in ondatra framework.

Notes

    1.	Explicit Keep alive timer in not supported. We do calculate relative keepalive timer from holdtimer value. Keepalive will be 1/3rd of negotiated hold timer.
