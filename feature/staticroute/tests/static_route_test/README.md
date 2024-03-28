# RT-1.25: Management network-instance default static route

## Summary

Validate static route functionality in Management network-instance (VRF).

## Procedure


*  Configure DUT with Management VRF and ATE1, ATE2 interfaces configured within this VRF
*  Configure IPv4 and IPv6 default routes within Management VRF pointing to ATE2 interface
*  Generate IPv4 and IPv6 traffic from ATE1 to any destination.
*  Verify that traffic is received at ATE2 interface

## Config Parameter coverage

*   /network-instances/network-instance/config/name
*   /network-instances/network-instance/config/description
*   /network-instances/network-instance/config/type

*   /network-instances/network-instance/interfaces/interface/config/id


*   /network-instances/network-instance/protocols/protocol/static-routes/static
*   /network-instances/network-instance/protocols/protocol/static-routes/static/prefix
*   /network-instances/network-instance/protocols/protocol/static-routes/static/config
*   /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix
*   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop
*   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/index
*   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config


## Telemetry Parameter coverage
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/state

