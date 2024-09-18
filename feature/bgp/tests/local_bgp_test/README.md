# RT-1.7: Local BGP Test

## Summary

The local\_bgp\_test brings up two OpenConfig controlled devices and tests that for an eBGP session 

* Established between them.
* Disconnected between them.
* Verify BGP neighbor parameters

Enable an Accept-route all import-policy/export-policy for eBGP session under the neighbor AFI/SAFI.


This test is suitable for running in a KNE environment.

## OpenConfig Path and RPC Coverage

```yaml
paths:
    ## Parameter Coverage

   /network-instances/network-instance/protocols/protocol/bgp/global/config/as:
   /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id:
   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/auth-password:
   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/local-as:
   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/neighbor-address:
   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as:
   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/neighbor-address:
   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/messages/received/last-notification-error-code:
   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/supported-capabilities:
   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/hold-time:
   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/keepalive-interval:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
```

