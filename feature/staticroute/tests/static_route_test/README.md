# RT-1.25: Management network-instance default static route

## Summary

Validate static route functionality in Management network-instance (VRF).

## Procedure


*  Configure DUT with Management VRF and ATE1, ATE2 interfaces configured within this VRF
*  Configure IPv4 and IPv6 default routes within Management VRF pointing to ATE2 interface
*  Generate IPv4 and IPv6 traffic from ATE1 to any destination.
*  Verify that traffic is received at ATE2 interface

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

TODO(OCPATH): Specify leaves for non-leaf paths that have been commented out.

```yaml
paths:
  ## Config Paths ##
  /network-instances/network-instance/config/name:
  /network-instances/network-instance/config/description:
  /network-instances/network-instance/config/type:
  /network-instances/network-instance/interfaces/interface/config/id:
  #/network-instances/network-instance/protocols/protocol/static-routes/static:
  /network-instances/network-instance/protocols/protocol/static-routes/static/prefix:
  #/network-instances/network-instance/protocols/protocol/static-routes/static/config:
  /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix:
  #/network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/index:
  #/network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/index:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop:

  ## State Paths ##
  #/network-instances/network-instance/protocols/protocol/static-routes/static/state:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
```

