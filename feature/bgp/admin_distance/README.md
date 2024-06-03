# RT-1.34: BGP AD/route-preference setting

## Summary

iBGP and eBGP Administartive Distance (AD, route preference) setting

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

## Procedure

### Applying configuration

For each section of configuration below, prepare a gnmi.SetBatch  with all the configuration items appended to one SetBatch.  Then apply the configuration to the DUT in one gnmi.Set using the `replace` option


#### Initial Setup:

*   Connect DUT port-1, 2 to ATE port-1, 2
*   Configure IPv4/IPv6 addresses on the ports
*   Create an IPv4 network i.e. ```ipv4-network-1 = 192.168.10.0/24``` attached to ATE port-1
*   Create an IPv6 network i.e. ```ipv6-network-1 = 2024:db8:64:64::/64``` attached to ATE port-1
*   Create an IPv4 network i.e. ```ipv4-network-2 = 192.168.20.0/24``` attached to ATE port-2
*   Create an IPv6 network i.e. ```ipv6-network-2 = 2024:db8:128:128::/64``` attached to ATE port-2
*   Configure IPv4 and IPv6 eBGP between DUT Port-1 and ATE Port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/global/config
    *   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/
    *   Advertise ```ipv4-network-1 = 192.168.10.0/24``` and ```ipv6-network-1 = 2024:db8:64:64::/64``` from ATE to DUT over the IPv4 and IPv6 eBGP session on port-1
*   Configure IPv4 and IPv6 IS-IS between DUT Port-1 and ATE Port-1
    *   /network-instances/network-instance/protocols/protocol/isis/global/config
    *   Advertise ```ipv4-network-1 = 192.168.10.0/24``` and ```ipv6-network-1 = 2024:db8:64:64::/64``` from ATE to DUT over the IPv4 and IPv6 IS-IS session on port-1
*   Configure IPv4 and IPv6 iBGP between DUT Port-2 and ATE Port-2
    *   /network-instances/network-instance/protocols/protocol/bgp/global/config
    *   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/
    *   Advertise ```ipv4-network-2 = 192.168.20.0/24``` and ```ipv6-network-2 = 2024:db8:128:128::/64``` from ATE to DUT over the IPv4 and IPv6 eBGP session on port-2
*   Configure IPv4 and IPv6 IS-IS between DUT Port-2 and ATE Port-2
    *   /network-instances/network-instance/protocols/protocol/isis/global/config
    *   Advertise ```ipv4-network-2 = 192.168.20.0/24``` and ```ipv6-network-2 = 2024:db8:128:128::/64``` from ATE to DUT over the IPv4 and IPv6 IS-IS session on port-2


### RT-1.34.1 [TODO:https://github.com/openconfig/featureprofiles/issues/3050]
#### Validate prefixes received with default Administrative Distance
*   Ensure that ```ipv4-network-1 = 192.168.10.0/24``` is received through both the eBGP session and the IS-IS session on port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv4-internal-reachability/prefixes/prefix
*   Validate that ```ipv4-network-1 = 192.168.10.0/24``` is a valid route through the eBGP session on port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/valid-route
*   Ensure that ```ipv6-network-1 = 2024:db8:64:64::/64``` is received through both the eBGP session and the IS-IS session on port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-reachability/prefixes/prefix
*   Validate that ```ipv6-network-1 = 2024:db8:64:64::/64``` is a valid route through the eBGP session on port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/state/valid-route

*   Ensure that ```ipv4-network-2 = 192.168.20.0/24``` is received through both the iBGP session and the IS-IS session on port-2
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv4-internal-reachability/prefixes/prefix
*   Validate that ```ipv4-network-2 = 192.168.20.0/24``` is NOT a valid route through the iBGP session on port-2 and invalid route reason is ```BGP_NOT_SELECTED_BESTPATH```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/valid-route
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/invalid-reason
*   Ensure that ```ipv6-network-2 = 2024:db8:128:128::/64``` is received through both the iBGP session and the IS-IS session on port-2
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-reachability/prefixes/prefix
*   Validate that ```ipv6-network-2 = 2024:db8:128:128::/64``` is NOT a valid route through the iBGP session on port-2 and invalid route reason is ```BGP_NOT_SELECTED_BESTPATH```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/state/valid-route
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/state/invalid-reason

### RT-1.34.2 [TODO:https://github.com/openconfig/featureprofiles/issues/3050]
#### Validate prefixes received with modified Administrative Distance
*   Configure Administrative Distance or Preference of eBGP session on port-1 to 220
    *   /network-instances/network-instance/protocols/protocol/bgp/global/default-route-distance/config/external-route-distance 
*   Ensure that ```ipv4-network-1 = 192.168.10.0/24``` is received through both the eBGP session and the IS-IS session on port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv4-internal-reachability/prefixes/prefix
*   Validate that ```ipv4-network-1 = 192.168.10.0/24``` is NOT a valid route through the eBGP session on port-1 and invalid route reason is ```BGP_NOT_SELECTED_BESTPATH```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/valid-route
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/invalid-reason
*   Ensure that ```ipv6-network-1 = 2024:db8:64:64::/64``` is received through both the eBGP session and the IS-IS session on port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-reachability/prefixes/prefix
*   Validate that ```ipv6-network-1 = 2024:db8:64:64::/64``` is NOT a valid route through the eBGP session on port-1 and invalid route reason is ```BGP_NOT_SELECTED_BESTPATH```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/state/valid-route
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/state/invalid-reason

*   Configure Administrative Distance or Preference of iBGP session on port-2 to 40
    *   /network-instances/network-instance/protocols/protocol/bgp/global/default-route-distance/config/internal-route-distance
*   Ensure that ```ipv4-network-2 = 192.168.20.0/24``` is received through both the iBGP session and the IS-IS session on port-2
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv4-internal-reachability/prefixes/prefix
*   Validate that ```ipv4-network-2 = 192.168.20.0/24``` is a valid route through the iBGP session on port-2
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/valid-route
*   Ensure that ```ipv6-network-2 = 2024:db8:128:128::/64``` is received through both the iBGP session and the IS-IS session on port-2
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix
    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-reachability/prefixes/prefix
*   Validate that ```ipv6-network-2 = 2024:db8:128:128::/64``` is a valid route through the iBGP session on port-2
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/state/valid-route

## Config Parameter Coverage

* /network-instances/network-instance/protocols/protocol/bgp/global/config
* /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/
* /network-instances/network-instance/protocols/protocol/isis/global/config
* /network-instances/network-instance/protocols/protocol/bgp/global/default-route-distance/config/external-route-distance
* /network-instances/network-instance/protocols/protocol/bgp/global/default-route-distance/config/internal-route-distance

## Telemetry Parameter Coverage

* /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix
* /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix
* /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/valid-route
* /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/state/valid-route
* /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/invalid-reason
* /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/state/invalid-reason
* /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv4-internal-reachability/prefixes/prefix
* /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-reachability/prefixes/prefix

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml
paths:
  ## Config paths
  ### Administrative Distance or Preference
  /network-instances/network-instance/protocols/protocol/bgp/global/default-route-distance/config/external-route-distance:
  /network-instances/network-instance/protocols/protocol/bgp/global/default-route-distance/config/internal-route-distance:

  ## State paths
  ### BGP Prefix state
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/valid-route
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/state/valid-route
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/invalid-reason
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/state/invalid-reason

  ### IS-IS Prefix state
  /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv4-internal-reachability/prefixes/prefix
  /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-reachability/prefixes/prefix

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```
