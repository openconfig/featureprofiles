# RT-1.5: BGP Prefix Limit

## Summary

BGP Prefix Limit

## Procedure

*   Configure eBGP session between ATE port-1 and DUT port-1,with an Accept-route all import-policy/export-policy under the BGP peer-group AFI/SAFI.
*   With maximum prefix limits of unlimited, and N.
    *   Advertise prefixes of `limit - 1`, `limit`, `limit + 1`. Validate
        session state meets expected value at ATE.
    *   Ensure that DUT marks session as prefix-limit exceeded for limit+1
        prefixes.
*   Advertise prefixes to exceed configured limit, and to `limit - 1` following
    session teardown, ensure session is re-established per the restart timer
    (with ATE session marked as passive).
*   With maximum-prefix warning-only configured, ensure that the routes that
    were sent prior to the max-prefix being exceeded are retained in the routing
    table by forwarding traffic to `prefix{0..n-1}` and `prefix{n}` where n is
    the maximum prefix limit configured.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/state/peer-group-name:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/neighbor-address:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/ipv4-unicast/prefix-limit/config/max-prefixes:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/ipv4-unicast/prefix-limit/state/warning-threshold-pct:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/ipv4-unicast/prefix-limit/state/prefix-limit-exceeded:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Subscribe:
    gNMI.Set:
```

## Minimum DUT platform requirement

vRX
