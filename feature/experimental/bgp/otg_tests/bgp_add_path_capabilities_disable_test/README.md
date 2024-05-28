# RT-1.53: BGP Add Path send/receive disable 

## Summary

BGP Add Path send/receive disable

## Procedure

*   Establish BGP connections between:.
    *   DUT Port1 (AS 65501) ---eBGP 1--- ATE Port1 (AS 65501)
    *   DUT Port2 (AS 65501) ---eBGP 2--- ATE Port2 (AS 65502)
*   Configure both DUT and ATE with add-path send and receive capabilities enabled for both IPv4 and IPv6.
*   Validate Initial Add-Path Capability: 
    *   Verify the telemetry path output to confirm the Add-Path capabilities for DUT.
*   Disable Add-Path Send: On DUT, disable the Add-Path send capability for the neighbor on ATE Port-2 for IPv4 and IPv6.
*   Verify the telemetry path output to confirm the Add-Path send capability for DUT is disabled for both ipv4 and ipv6 for the neighbor on ATE Port-2.
*   Disable Add-Path Receive: On DUT, disable the Add-Path receive capability for the neighbor on ATE Port-1.
*   Verify the telemetry path output to confirm the Add-Path receive capability for DUT is disabled for both ipv4 and ipv6 for the neighbor on ATE Port-1.

## OpenConfig Path and RPC Coverage

This example yaml defines the OC paths intended to be covered by this test.  OC paths used for test environment setup are not required to be listed here.

```yaml
paths:
  ## Config paths
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/config/receive
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/config/send
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/config/receive
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/config/send

  ## State paths
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/state/receive
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/state/send
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/state/receive
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/state/send
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/supported-capabilities

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

* MFF - A modular form factor device containing LINECARDs, FABRIC and redundant CONTROLLER_CARD components
* FFF - fixed form factor
