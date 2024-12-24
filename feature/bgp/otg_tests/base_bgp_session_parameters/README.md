# RT-1.1: Base BGP Session Parameters

## Summary

BGP session establishment between DUT - ATE and verifiying different session parameters.

## Topology

    DUT port-1 -------- ATE port-1

## Procedure

Test the abnormal termination of session using notification message:

*   Establish BGP session between DUT (AS 65540) and ATE (AS 65550). The DUT/ATE
    peers should be configured with MD5 authentication using the same password.

    *   Ensure session state should be `ESTABLISHED`.
    *   Verify BGP capabilities: route refresh, ASN32 and MPBGP.
    
*   Verify BGP session disconnect by sending notification message from ATE.

    *   Send `CEASE` notification from ATE.
    *   Ensure that DUT telemetry correctly reports the error code.

Test md5 password authentication on session establishment:

*   Configure matching passwords on DUT and ATE. Verify that BGP adjacency succeeds.

*   Configure mismatched passwords on DUT and ATE. Verify that BGP adjacency fails.


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
    *   Explicit holdtime interval and keepalive interval.
    *   Explicit connect retry interval.

## OpenConfig Path and RPC Coverage
```yaml
paths:
  ## Config Parameter Coverage
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/hold-time:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/keepalive-interval:

  ## Telemetry Parameter Coverage
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/last-established:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/messages/received/NOTIFICATION:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/state/negotiated-hold-time:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/supported-capabilities:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
```
