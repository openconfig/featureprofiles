# RT-1.11: Static route to BGP redistribution

## Summary

- Static routes selected for redistribution base on combination of: prefix-set, set-tag
- MED set to value of metric of static route (metric propagation)
- AS-Path set to contain one AS with value provided in configuration
- Community list set to defined community set
- BGP protocol next-hop set to value provided in configuration

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed

## Procedure

#### Initial Setup:

*   Connect DUT port-1 and port-2 to ATE port-1 and port-2 respectively

*   Configure IPv4 and IPv6 addresses on DUT and ATE ports as shown below
    *   DUT port-1 IPv4 address ```dp1-v4 = 192.168.1.1/30```
    *   ATE port-1 IPv4 address ```ap1-v4 = 192.168.1.2/30```

    *   DUT port-2 IPv4 address ```dp2-v4 = 192.168.1.5/30```
    *   ATE port-2 IPv4 address ```ap2-v4 = 192.168.1.6/30```

    *   DUT port-1 IPv6 address ```dp1-v6 = 2001:DB8::1/126```
    *   ATE port-1 IPv6 address ```ap1-v6 = 2001:DB8::2/126```

    *   DUT port-2 IPv6 address ```dp2-v6 = 2001:DB8::4/126```
    *   ATE port-2 IPv6 address ```ap2-v6 = 2001:DB8::5/126```

*   Create an IPv4 network i.e. ```ipv4-network = 192.168.10.0/24``` attached to ATE port-2

*   Create an IPv6 network i.e. ```ipv6-network = 2024:db8:128:128::/64``` attached to ATE port-2

*   Configure IPv4 and IPv6 eBGP session between ATE port-1 and DUT port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/global/config
    *   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/

*   On the DUT advertise networks of ```dp2-v4``` i.e. ```192.168.1.4/30``` and ```dp2-v6``` i.e. ```2001:DB8::0/126``` through the BGP session between DUT port-1 and ATE port-1
    *   Do not configure BGP between DUT port-2 and ATE port-2
    *   Do not advertise ```ipv4-network 192.168.10.0/24``` or ```ipv6-network 2024:db8:128:128::/64```

*   Configure an IPv4 static route ```ipv4-route``` on DUT destined to the ```ipv4-network``` i.e. ```192.168.10.0/24``` with the next hop set to the IPv4 address of ATE port-2 ```ap2-v4``` i.e. ```192.168.1.6/30```
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop
    *   Set the metric of the ```ipv4-route``` to 104
        *   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/metric
    *   Set a tag on the ```ipv4-route``` to 40
        *   /network-instances/network-instance/protocols/protocol/static-routes/static/config/set-tag

*   Configure an IPv6 static route on DUT destined to the ```ipv6-network``` i.e. ```2024:db8:128:128::/64``` with the next hop set to the IPv6 address of ATE port-2 ```ap2-v6``` i.e. ```2001:DB8::5/126```
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix
    *   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop
    *   Set the metric of the ```ipv6-route``` to 106
        *   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/metric
    *   Set a tag on the ```ipv6-route``` to 60
        *   /network-instances/network-instance/protocols/protocol/static-routes/static/config/set-tag

