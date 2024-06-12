# RT-7.6: BGP Link Bandwidth Community - Aggregation

## Summary

This test verifies Link-bandwidth (LBW) extended community aggregation feature by DUT.

## Testbed type

*  [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

### Test environment setup

    ```
                                          |         |
    [ 64x eBGP ] --- [ ATE Port 1 ] ----  |   DUT   | ---- [ ATE Port 2 ]
                                          |         |
    ```

### RT-7.6.1: Verify LBW aggregation to eBGP peer

* Configure DUT with routed ports on DUT.
* Configure 64x eBGP peers on ATE Port 1 interface in peering group UPSTREAM.
* Configure a single eBGP peer on ATE Port 2 interface in peering group DOWNSTREAM.
* Enable BGP LBW receive for peering group UPSTREAM.
* Enable BGP LBW send for peering group DOWNSTREAM. Enable Link Bandwidth aggregation feature.

**TODO:** Link Bandwidth aggregation feature is not currently modeled in OC.

2. Advertise the same test prefix from ATE from all UPSTREAM peers with LBW community:
  * 32 peers - 10Mbps
  * 16 peers - 20Mbps
  * 8 peers - 40Mbps
  * 8 peers - 80Mbps

3. Verify that DUT advertises the test route to DOWNSTREAM eBGP peer with aggregated bandwidth community of 1600Mbps.

### RT-7.6.2: Verify LBW changes.
Using RT-7.6.1 set up conduct following changes:

1) Disable 32 peers advertising 10Mpbs bandwidth community.
2) Verify that DUT advertises the test route to Upstream peer with aggregated bandwidth community of 800Mbps.
3) Re-enable 32 peers advertising 10Mpbs bandwidth community.
4) Verify that DUT advertises the test route to Upstream peer with aggregated bandwidth community of 1600Mbps.

### RT-7.6.3: Verify LBW aggregation to iBGP peer

* Configure 64x eBGP peers on ATE Port 1 interface in peering group UPSTREAM.
* Configure a single iBGP peer on ATE Port 2 interface in peering group DOWNSTREAM.
* Enable BGP LBW receive for peering group UPSTREAM.
* Enable BGP LBW send for peering group DOWNSTREAM. Enable Link Bandwidth aggregation feature.

**TODO:** Link Bandwidth aggregation feature is not currently modeled in OC.

2. Advertise the same test prefix from ATE from all UPSTREAM peer with LBW community:
  * 32 peers - 10Mbps
  * 16 peers - 20Mbps
  * 8 peers - 40Mbps
  * 8 peers - 80Mbps

3. Verify that DUT advertises the test route to DOWNSTREAM iBGP peer with aggregated bandwidth community of 1600Mbps.

## OpenConfig Path and RPC Coverage

```yaml
paths:
    # tunnel interfaces
    /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/use-multiple-paths/ebgp/link-bandwidth-ext-community/config/enabled:
    /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/use-multiple-paths/ibgp/link-bandwidth-ext-community/config/enabled:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

* FFF