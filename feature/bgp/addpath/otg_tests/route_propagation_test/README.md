# RT-1.3: BGP Route Propagation

## Summary

BGP Route Propagation

## Testbed type

*  [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

Establish eBGP sessions between:

*   ATE port-1 and DUT port-1
*   ATE port-2 and DUT port-2


### RT-1.3.1: MRAI: [TODO: https://github.com/openconfig/featureprofiles/issues/3035]
*   DUT: Configure the Minimum Route Advertisement Interval (MRAI) for desired behavior.
*   ATE Port 2: Verify received routes adhere to the MRAI timing.

### RT-1.3.2: RFC5549
*   DUT: Enable RFC5549 support:
    *   Update the BGP peer group configuration to enable extended next hop encoding using  `/network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/ipv4-unicast/config/extended-next-hop-encoding`  
*   ATE Port 1: Advertise IPv4 routes with IPv6 next-hops.
*   ATE Port 2: Validate correct acceptance and propagation of routes with IPv6 next-hops.

### RT-1.3.3: Add-Path (Initial State): [TODO: https://github.com/openconfig/featureprofiles/issues/3037]
*   ATE Port 1: Advertise multiple routes with distinct path IDs for the same prefix.
*   ATE Port 2: Confirm that all advertised routes are accepted and propagated by the DUT due to the initially enabled Add-Path.
*   Verification (Telemetry): Verify that the DUT's telemetry output reflects the enabled Add-Path capabilities.

### RT-1.3.4: Disabling Add-Path Send: [TODO: https://github.com/openconfig/featureprofiles/issues/3037]
*   DUT: Disable Add-Path send for the neighbor connected to ATE Port 2 for both IPv4 and IPv6.
*   Verification (Telemetry): Confirm that the DUT's telemetry reflects the disabled Add-Path send status.
*   ATE Port 1: Readvertise multiple paths.
*   ATE Port 2: Verify that only a single best path is received by ATE Port 2 due to disabled Add-Path send on the DUT.

### RT-1.3.5: Disabling Add-Path Receive: [TODO: https://github.com/openconfig/featureprofiles/issues/3037]
*   DUT: Disable Add-Path receive for the neighbor connected to ATE Port 1 for both IPv4 and IPv6.
*   Verification (Telemetry): Confirm the disabled Add-Path receive status in telemetry.
*   ATE Port 1: Advertise BGP routes to the DUT via Port 1.
*   ATE Port 2: Verify that the DUT has received and propagated only one single path.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
  ## Config paths
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/config/receive:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/config/send:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/config/send-max:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/ipv4-unicast/config/extended-next-hop-encoding:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/config/receive:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/config/send:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/config/send-max:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/config/minimum-advertisement-interval:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/ipv4-unicast/config/extended-next-hop-encoding:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/config/receive:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/config/send:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/config/send-max:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/minimum-advertisement-interval:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/config/extended-next-hop-encoding:

  ## State paths
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/state/receive:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/state/send:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/state/send-max:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/ipv4-unicast/state/extended-next-hop-encoding:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/state/receive:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/state/send:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/state/send-max:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/state/minimum-advertisement-interval:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/ipv4-unicast/state/extended-next-hop-encoding:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/state/receive:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/state/send:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/state/send-max:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/state/minimum-advertisement-interval:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/state/extended-next-hop-encoding:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/supported-capabilities:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

* MFF - A modular form factor device containing LINECARDs, FABRIC and redundant CONTROLLER_CARD components
* FFF - fixed form factor
