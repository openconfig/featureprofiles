# RT-1.26: Basic static route support

## Summary

-   Static route ECMP must be supported
-   Static route metric must be supported
-   Static route Administrative Distance / Preference must be supported
-   `set-tag` attribute must be supported for static routes
-   Disabling recursive nexthop resolution must be supported

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed

## Procedure

#### Initial Setup:

*   Connect DUT port-1, port-2 and port-3 to ATE port-1, port-2 and port-3
    respectively
*   Configure IPv4/IPv6 addresses on DUT and ATE the interfaces
*   Configure one IPv4 destination i.e. `ipv4-network = 192.168.10.0/24`
    connected to ATE port 1 and 2
*   Configure one IPv6 destination i.e. `ipv6-network = 2024:db8:128:128::/64`
    connected to ATE port 1 and 2

### RT-1.26.1 [TODO: https://github.com/openconfig/featureprofiles/issues/2489]

#### Test to validate static route ECMP

*   Configure IPv4 static routes:
    *   Configure one IPv4 static route i.e. ipv4-route-a on the DUT for
        destination `ipv4-network 192.168.10.0/24` with the next hop set to the
        IPv4 address of ATE port-1
    *   Configure another IPv4 static route i.e. ipv4-route-b on the DUT for
        destination `ipv4-network 192.168.10.0/24` with the next hop set to the
        IPv4 address of ATE port-2
*   Validate both the routes i.e. ipv4-route-[a|b] are configured and reported
    correctly
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/prefix
*   Configure IPv6 static routes:
    *   Configure one IPv6 static route i.e. ipv6-route-a on the DUT for
        destination `ipv6-network 2024:db8:128:128::/64` with the next hop set
        to the IPv6 address of ATE port-1
    *   Configure another IPv6 static route i.e. ipv6-route-b on the DUT for
        destination `ipv6-network 2024:db8:128:128::/64` with the next hop set
        to the IPv6 address of ATE port-2
*   Validate both the routes i.e. ipv6-route-[a|b] sre configured and reported
    correctly
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/prefix
*   Initiate traffic from ATE port-3 towards destination `ipv4-network
    192.168.10.0/24` and `ipv6-network 2024:db8:128:128::/64`
*   Validate that traffic is received from DUT on both port-1 and port-2 and
    ECMP works

### RT-1.26.2 [TODO: https://github.com/openconfig/featureprofiles/issues/2489]

#### Test to validate static route metric

*   Configure metric of ipv4-route-b and ipv6-route-b to 1000
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/metric
*   Validate that the metric is set correctly
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/state/metric
*   Initiate traffic from ATE port-3 towards destination `ipv4-network
    192.168.10.0/24` and `ipv6-network 2024:db8:128:128::/64`
*   Validate that traffic is received from DUT on port-1 and not on port-2

### RT-1.26.3 [TODO: https://github.com/openconfig/featureprofiles/issues/2489]

#### Test to validate static route prefrence

*   Configure preference of ipv4-route-a and ipv6-route-a to 200
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/preference
*   Validate that the preference is set correctly
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/state/preference
*   Initiate traffic from ATE port-3 towards destination `ipv4-network
    192.168.10.0/24` and `ipv6-network 2024:db8:128:128::/64`
*   Validate that traffic is now received from DUT on port-2 and not on port-1

### RT-1.26.4 [TODO: https://github.com/openconfig/featureprofiles/issues/2489]

#### Test to validate static route tag

*   Configure a tag of value 10 on ipv4-route-a and ipv6-route-a
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/config/set-tag
*   Validate the tag is set
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/state/set-tag

### RT-1.26.5 [TODO: https://github.com/openconfig/featureprofiles/issues/2489]

#### Test to validate IPv6 static route with IPv4 next-hop

*   Remove metric of 1000 from ipv4-route-b and ipv6-route-b
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/metric
*   Remove preference of 200 from ipv4-route-a and ipv6-route-a
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/preference
*   Change the IPv6 next-hop of the ipv6-route-a with the next hop set to the
    IPv4 address of ATE port-1
*   Change the IPv6 next-hop of the ipv6-route-b with the next hop set to the
    IPv4 address of ATE port-2
*   Validate both the routes i.e. ipv6-route-[a|b] are configured and the IPv4
    next-hop is reported correctly
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/prefix
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop
*   Initiate traffic from ATE port-3 towards destination `ipv6-network
    2024:db8:128:128::/64`
*   Validate that traffic is received from DUT on both port-1 and port-2 and
    ECMP works

### RT-1.26.6 [TODO: https://github.com/openconfig/featureprofiles/issues/2489]

#### Test to validate IPv4 static route with IPv6 next-hop

*   Change the IPv4 next-hop of the ipv4-route-a with the next hop set to the
    IPv6 address of ATE port-1
*   Change the IPv4 next-hop of the ipv4-route-b with the next hop set to the
    IPv6 address of ATE port-2
*   Validate both the routes i.e. ipv4-route-[a|b] are configured and the IPv6
    next-hop is reported correctly
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/prefix
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop
*   Initiate traffic from ATE port-3 towards destination `ipv4-network
    192.168.10.0/24`
*   Validate that traffic is received from DUT on both port-1 and port-2 and
    ECMP works

### RT-1.26.7 [TODO: https://github.com/openconfig/featureprofiles/issues/2489]

#### Test to validate disabling of recursive next-hop resolution

*   Remove the ipv4-route-[a|b] and ipv6-route-[a|b] static routes
*   Configure ipv4 and ipv6 ISIS between ATE port-1 <-> DUT port-1 and ATE
    port-2 <-> DUT port2
    *   /network-instances/network-instance/protocols/protocol/isis/global/afi-safi
*   Configure one IPv4 /32 host route i.e. `ipv4-loopback = 192.168.100.100/32`
    connected to ATE and advertised to DUT through both the IPv4 ISIS
    adjacencies
*   Configure one IPv6 /128 host route i.e. `ipv6-loopback =
    2023:ab1:64:64::1/128` connected to ATE and advertised to DUT through both
    the IPv6 ISIS adjacencies
*   Configure one IPv4 static route i.e. ipv4-route on the DUT for destination
    `ipv4-network 192.168.10.0/24` with the next hop of `ipv4-loopback
    192.168.100.100/32`
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop
*   Configure one IPv6 static route i.e. ipv6-route on the DUT for destination
    `ipv6-network 2024:db8:128:128::/64` with the next hop of `ipv6-loopback =
    2023:ab1:64:64::1/128`
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop
*   Initiate traffic from ATE port-3 towards destination `ipv4-network
    192.168.10.0/24` and `ipv6-network 2024:db8:128:128::/64`
*   Validate that traffic is received from DUT (doesnt matter which port)
*   Disable static route next-hop recursive lookup (set to false)
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/recurse
*   Validate static route next-hop recursive lookup is disabled
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/state/recurse
*   Initiate traffic from ATE port-3 towards destination `ipv4-network
    192.168.10.0/24` and `ipv6-network 2024:db8:128:128::/64`
*   Validate that traffic is NOT received from DUT

## Config parameter coverage

*   /interfaces/interface/config/enabled
*   /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled
*   /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled
*   /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix
*   /network-instances/network-instance/protocols/protocol/static-routes/static/config/set-tag
*   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop
*   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/metric
*   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/preference
*   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/recurse

## Telemetry parameter coverage

*   /network-instances/network-instance/protocols/protocol/static-routes/static/state/prefix
*   /network-instances/network-instance/protocols/protocol/static-routes/static/state/set-tag
*   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/state/next-hop
*   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/state/metric
*   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/state/preference
*   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/state/recurse

## Protocol/RPC Parameter Coverage

*   gNMI
    *   Get
    *   Set

## Required DUT platform

*   FFF

