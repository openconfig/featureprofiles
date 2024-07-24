# RT-7.6: BGP Link Bandwidth Community - Cumulative

## Summary

This test verifies Link-bandwidth (LBW) extended community cumulative feature by DUT.

## Testbed type

*  [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

### Test environment setup

    ```
                                          |         |
    [ 64x eBGP ] --- [ ATE Port 1 ] ----  |   DUT   | ---- [ ATE Port 2 ]
                                          |         |
    ```

#### Configuration

* Configure DUT with 2 routed ports.
* Configure 64x eBGP peers on ATE Port 1 interface
* Configure 64x eBGP peers on DUT Port 1 interface in peering group UPSTREAM.
* Configure a single eBGP peer on ATE Port 2 interface.
* Configure a single eBGP peer on DUT Port 2 interface in peering group DOWNSTREAM.


### RT-7.6.1: Verify LBW cumulative to eBGP peer

* Enable BGP LBW receive for peering group UPSTREAM.
* Enable BGP LBW send for peering group DOWNSTREAM. 
* Enable Link Bandwidth Cumulative feature on DOWNSTREAM.

**TODO:** [Cumulative Link Bandwidth feature](https://datatracker.ietf.org/doc/draft-ietf-bess-ebgp-dmz/) is not currently modeled in OC. Related PR: https://github.com/openconfig/public/pull/1131


2. Advertise the same test prefix from ATE from all UPSTREAM peers with LBW community:
  * 32 peers - 10Mbps
  * 16 peers - 20Mbps
  * 8 peers - 40Mbps
  * 8 peers - 80Mbps

3. Verify that DUT advertises the test route to DOWNSTREAM eBGP peer with cumulative bandwidth community of 1600Mbps.

### RT-7.6.2: Verify LBW changes.
Using RT-7.6.1 set up conduct following changes:

1) Disable 32 peers advertising 10Mpbs bandwidth community.
2) Verify that DUT advertises the test route to Upstream peer with cumulative bandwidth community of 1280Mbps.
3) Re-enable 32 peers advertising 10Mpbs bandwidth community.
4) Verify that DUT advertises the test route to Upstream peer with cumulative bandwidth community of 1600Mbps.

### RT-7.6.3: Verify LBW cumulative to iBGP peer

1. Reconfigure 64x peers in peering group UPSTREAM to iBGP
2. Reconfigure a single peer in peering group DOWNSTREAM to iBGP
3. Repeat test RT-7.6.1 for iBGP.

## OpenConfig Path and RPC Coverage

```yaml
paths:
    /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/use-multiple-paths/ebgp/link-bandwidth-ext-community/config/enabled:
    /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/use-multiple-paths/ibgp/link-bandwidth-ext-community/config/enabled:
    # TODO: Add Cumulative LBW path.

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