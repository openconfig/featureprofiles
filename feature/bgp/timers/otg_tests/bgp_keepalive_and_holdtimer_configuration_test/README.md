# RT-1.10: BGP Keepalive and HoldTimer Configuration Test

## Summary

BGP Keepalive and HoldTimer Configuration Test

## Procedure

*   Establish eBGP sessions as follows between ATE and DUT
    * The DUT has eBGP peering with ATE port 1 and ATE port 2.
    * Enable an Accept-route all import-policy/export-policy under the neighbor AFI/SAFI.
    * The first pair is called the "source" pair, and the second the "destination" pair

*  Validate BGP session state on DUT using telemetry.
*  Modify BGP timer values on iBGP peers to 10/30 and on the eBGP peering to 5/15.
*  Verify that the sessions are established after soft reset.


## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

```yaml
paths:
  ## Config Paths ##
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/keepalive-interval:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/hold-time:

  ## State Paths ##
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/state/keepalive-interval:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/state/hold-time:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Subscribe:
      on_change: true
    gNMI.Set:
```
   
## Minimum DUT platform requirement

vRX
