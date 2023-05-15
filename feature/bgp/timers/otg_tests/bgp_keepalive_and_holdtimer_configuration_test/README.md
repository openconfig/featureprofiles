# RT-1.10: BGP Keepalive and HoldTimer Configuration Test

## Summary

BGP Keepalive and HoldTimer Configuration Test

## Procedure

*   Establish BGP sessions as follows between ATE and DUT
    * The DUT has eBGP peering with ATE port 1 and ATE port 2.
    * The first pair is called the "source" pair, and the second the "destination" pair

*  Validate BGP session state on DUT using telemetry.
*  Modify BGP timer values on iBGP peers to 10/30 and on the eBGP peering to 5/15.
*  Verify that the sessions are established after soft reset.


## Config Parameter coverage

*  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/keepalive-interval
*  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/hold-time

## Telemetry Parameter coverage

*  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/state/keepalive-interval
*  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/state/hold-time

## Protocol/RPC Parameter coverage

N/A

## Minimum DUT platform requirement

vRX