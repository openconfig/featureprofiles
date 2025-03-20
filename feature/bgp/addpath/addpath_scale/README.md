# RT-1.15: BGP ADDPATH SCALE

## Summary

BGP ADDPATH TEST WITH SCALE

## Testbed type

  *  [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)
  * ATE port1 - Used for traffic source
  * ATE port2, port3 - Used to advertise routes to the DUT
  * ATE port4 - Used for verification of Add-path send or receive capabilities.

## Procedure

Establish eBGP sessions between:

*   ATE port2 (AS65402) and DUT port2 (AS65401)
*   ATE port3 (AS65403) and DUT port3 (AS65401)
*   ATE port4 (AS65404) and DUT port4 (AS65401)

### RT-1.15.1: Add-Path (Initial State):

*   Enable Add-Path send and receive for the neighbor connected from DUT port-2,
    port-3, port4 to ATE Port-2, Port-3, Port-4 for address-family IPv4 and
    address-family IPv6.
*   Configure DUT port-1 with a simple IPv4, IPv6 address-family with prefixes
    200.0.0.0/24 and 1000::200.0.0.0/126 respectively.
*   Advertise 50k IPv4 & 50k IPv6 routes from ATE port-2, port-3,
    with following properties
      -IPv4 Routes to be distributed with mask lengths of /22 (10k routes), /24
        (30k routes) and /30 (10k routes)
      -IPv6 Routes to be distributed with mask lengths of /48 (10k routes), /64
        (10k routes) and /126 (30k routes)
      -Make sure that same 50k IPv4 & 50k IPv6 routes are advertised from all
        2 ATE ports (port2 and port3)
*   Configure traffic with following source and destination prefixes
      - source (IPv4/IPv6) - 200.0.0.0/24 and 1000::200.0.0.0/126 respectively.
      - destination (IPv4/IPv6) - All prefixes of ATE port2, port3

*   Verification (Telemetry):
    *   Verify that all advertised routes from ATE ports are learnt by DUT and
        each prefix has 3 next-hops of ATE port2, port3, port4 respectively.
    *   Verify that the DUT's telemetry output reflects the enabled Add-Path
        capabilities with send and receive
    *   Verify that ATE port-4 receives the routes advertised from ATE-port2,
        port3 with add-path send and receive capabilities enabled.
    *   Verify that the DUT forwards traffic only on the best path with 100%
        and other paths traffic should be 100% loss for other paths.

### RT-1.15.3: Route churn and verify Add-path telemetry

*   Do BGP route flap from the ATE ports a few times like maybe 120 seconds and
    wait for sometime for the route churn to settle down.
*   Verification: Telemetry
    *   Repeat verification steps in RT-1.15.1


### RT-1.15.3: Disabling Add-Path send for ATE port2

*   Disable Add-Path send for the neighbor connected to ATE Port2 only
    for both IPv4 & IPv6 and ATE port3 continues to advertise with add-path
    send and receive
*   Verification (Telemetry):
    *   Verify that the DUT's telemetry reflects the disabled Add-Path send
        status for routes received from port2. And there is only 1 best path
        for the routes selected by BGP on the device instead of 2 earlier.
    *   Verify that ATE port-4 receives the routes re-advertised by the DUT
        which were learnt from ATE port3 with add-path send and receive
        capabilities enabled.
    *   Verify that the DUT forwards traffic only on the best path with 100%
        and other paths traffic should be 100% loss for other paths.

### RT-1.15.4: Disabling Add-Path receive for ATE port3

*   DUT: Disable Add-Path receive for the neighbor connected to ATE Port4 only
    for both IPv4 & IPv6 and ATE port2, port3 continues to advertise with add-path
    send and receive
*   Verification (Telemetry):
    *   Verify that the DUT's telemetry reflects the disabled Add-Path receive
        status for routes received from port2, port3. And there are 2 best path
        for the routes selected by BGP.
    *   Verify that ATE port-4 Add-path receive status is disabled for all the
        routes that routes re-advertised by the DUT which were learnt from ATE 
        port2, port3 with add-path send and receive capabilities enabled
    *   Verify that the DUT forwards traffic only on the best path with 100%
        and other paths traffic should be 100% loss for other paths.

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

* MFF - A modular form factor device containing LINECARD, FABRIC and redundant CONTROLLER_CARD components
* FFF - fixed form factor

