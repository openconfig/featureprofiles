# RT-1.34: BGP route-distance configuration

## Summary

BGP default-route-distance, external-route-distance and internal-route-distance (administrative distance) configuration.

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed

## Procedure

### Applying configuration

For each section of configuration below, prepare a gnmi.SetBatch  with all the configuration items appended to one SetBatch. Then apply the configuration to the DUT in one gnmi.Set using the `replace` option

#### Initial Setup:

*   Connect DUT port-1, 2 and 3 to ATE port-1, 2 and 3
*   Configure IPv4/IPv6 addresses on the ports
*   Create an IPv4 network i.e. ```ipv4-network-1 = 192.168.10.0/24``` attached to ATE port-1 and port-2
*   Create an IPv6 network i.e. ```ipv6-network-1 = 2024:db8:64:64::/64``` attached to ATE port-1 and port-2
*   Configure IPv4 and IPv6 IS-IS between DUT Port-1 and ATE Port-1
    *   /network-instances/network-instance/protocols/protocol/isis/global/config
    *   Advertise ```ipv4-network-1 = 192.168.10.0/24``` and ```ipv6-network-1 = 2024:db8:64:64::/64``` from ATE to DUT over the IPv4 and IPv6 IS-IS session on port-1

### RT-1.34.1 [TODO:https://github.com/openconfig/featureprofiles/issues/3050]
#### Validate traffic with modified eBGP Route-Distance of 5
*   Configure IPv4 and IPv6 eBGP between DUT Port-2 and ATE Port-2
    *   /network-instances/network-instance/protocols/protocol/bgp/global/config
    *   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/
    *   Advertise ```ipv4-network-1 = 192.168.10.0/24``` and ```ipv6-network-1 = 2024:db8:64:64::/64``` from ATE to DUT over the IPv4 and IPv6 eBGP session on port-2
*   Configure Route-Distance of eBGP session on port-2 to 5
    *   /network-instances/network-instance/protocols/protocol/bgp/global/default-route-distance/config/external-route-distance 
*   Validate using gNMI Subscribe with mode 'ONCE' that the correct Route-Distance value of 5 is reported:
    *   /network-instances/network-instance/protocols/protocol/bgp/global/default-route-distance/state/external-route-distance
*   Generate traffic from ATE port-3 towards ```ipv4-network-1 = 192.168.10.0/24``` and ```ipv6-network-1 = 2024:db8:64:64::/64```
*   Verify that the traffic is received on port-2 of the ATE

### RT-1.34.2 [TODO:https://github.com/openconfig/featureprofiles/issues/3050]
#### Validate traffic with modified eBGP Route-Distance of 250
*   Configure Route-Distance of eBGP session on port-2 to 250
    *   /network-instances/network-instance/protocols/protocol/bgp/global/default-route-distance/config/external-route-distance 
*   Validate using gNMI Subscribe with mode 'ONCE' that the correct Route-Distance value of 250 is reported:
    *   /network-instances/network-instance/protocols/protocol/bgp/global/default-route-distance/state/external-route-distance
*   Generate traffic from ATE port-3 towards ```ipv4-network-1 = 192.168.10.0/24``` and ```ipv6-network-1 = 2024:db8:64:64::/64```
*   Verify that the traffic is received on port-1 of the ATE

### RT-1.34.3 [TODO:https://github.com/openconfig/featureprofiles/issues/3050]
#### Validate traffic with modified iBGP Route-Distance of 5
*   Replace IPv4 and IPv6 eBGP with IPv4 and IPv6 iBGP between DUT Port-2 and ATE Port-2
    *   /network-instances/network-instance/protocols/protocol/bgp/global/config
    *   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/
    *   Advertise ```ipv4-network-1 = 192.168.10.0/24``` and ```ipv6-network-1 = 2024:db8:64:64::/64``` from ATE to DUT over the IPv4 and IPv6 iBGP session on port-2
*   Configure Route-Distance of iBGP session on port-2 to 5
    *   /network-instances/network-instance/protocols/protocol/bgp/global/default-route-distance/config/internal-route-distance
*   Validate using gNMI Subscribe with mode 'ONCE' that the correct Route-Distance value of 5 is reported:
    *   /network-instances/network-instance/protocols/protocol/bgp/global/default-route-distance/state/internal-route-distance
*   Generate traffic from ATE port-3 towards ```ipv4-network-1 = 192.168.10.0/24``` and ```ipv6-network-1 = 2024:db8:64:64::/64```
*   Validate that the traffic is received on port-2 of the ATE

### RT-1.34.4 [TODO:https://github.com/openconfig/featureprofiles/issues/3050]
#### Validate traffic with modified iBGP Route-Distance of 250
*   Configure Route-Distance of iBGP session on port-2 to 250
    *   /network-instances/network-instance/protocols/protocol/bgp/global/default-route-distance/config/internal-route-distance
*   Validate using gNMI Subscribe with mode 'ONCE' that the correct Route-Distance value of 250 is reported:
    *   /network-instances/network-instance/protocols/protocol/bgp/global/default-route-distance/state/internal-route-distance
*   Generate traffic from ATE port-3 towards ```ipv4-network-1 = 192.168.10.0/24``` and ```ipv6-network-1 = 2024:db8:64:64::/64```
*   Validate that the traffic is received on port-1 of the ATE

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml
paths:
  ## Config paths
  ### Route-Distance
  /network-instances/network-instance/protocols/protocol/bgp/global/default-route-distance/config/external-route-distance:
  /network-instances/network-instance/protocols/protocol/bgp/global/default-route-distance/config/internal-route-distance:

  ## State paths
  ### Route-Distance
  /network-instances/network-instance/protocols/protocol/bgp/global/default-route-distance/state/internal-route-distance:
  /network-instances/network-instance/protocols/protocol/bgp/global/default-route-distance/state/external-route-distance:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```
