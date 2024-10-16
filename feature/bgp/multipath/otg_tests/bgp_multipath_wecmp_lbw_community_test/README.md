# RT-1.52: BGP multipath UCMP support with Link Bandwidth Community

## Summary

Validate BGP in multipath UCMP support with link bandwidth community

## Testbed type

[TESTBED_DUT_ATE_4LINKS](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Setup

*   Connect DUT port 1, 2 and 3 to ATE port 1, 2 and 3 respectively
*   Configure IPv4/IPv6 addresses on the interfaces
*   Establish eBGP sessions between:
    *   ATE port-1 and DUT port-1
    *   ATE port-2 and DUT port-2
    *   ATE port-3 and DUT port-3
*   Enable an Accept-route all import-policy/export-policy for eBGP session
    under the neighbor AFI/SAFI - IPv6 unicast and IPv4 unicast.
*   Create an single IPv4 internal target network attached to ATE port 2 and 3
*   [TODO] Create an single IPv6 internal target network attached to ATE port 2 and 3


### Tests

*   RT-1.52.1: Verify use of unequal community type

    *   Test Configuration
        *   Configure ATE port 1, 2 and 3 on different AS, with boths AFI/SAFI
            * Advertise IPv4 and IPv6 internal target, both, form both ATE port-1 and ATE port-2 in eBGP.
            * [TODO] For ATE port 2 attach `link-bandwidth:23456:10K` extended-community
            * [TODO] For ATE port 3 attach `link-bandwidth:23456:5K` extended-community
        *   Enable multipath, set maximum-paths limit to 2, enable allow multiple
            AS, and send community type to [STANDARD, EXTENDED, LARGE]
            *   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/use-multiple-paths/config/enabled
            *   [TODO] /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/use-multiple-paths/ebgp/config/allow-multiple-as
            *   [TODO] /network-instances/network-instance/protocols/protocol/bgp/globalp/afi-safis/afi-safi/use-multiple-paths/ebgp/config/maximum-paths
            *   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/send-community-type
            *   /network-instances/network-instance/protocols/protocol/bgp/global/use-multiple-paths/ebgp/link-bandwidth-ext-community/config/enabled
        *   Advertise equal cost paths from port2 and port3 of ATE
        *   Initiate traffic from ATE port-1 to the DUT and destined to internal
            target network. 
            *   Use UDP traffic with src and dst port randomly selected from 1-65535 range for each packet. Or equivalent pattern guaranteeng high entropy of traffic.
    * Behaviour Validation
        *   Check entries in AFT for advertised prefix, it should have 2 entries.\
            [TODO] The `weight` leafs of next-hops shall be in 2:1 ratio.
            *   Find next-hop-group IDs for both internal target networks:
                *   /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry[prefix=IPv4 internal target network]/state/**next-hop-group**
                *   /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry[prefix=IPv6 internal target network]/state/**next-hop-group**
            *   using next-hop-group as key find number and weight of next-hops of both internal target network
                *   /network-instances/network-instance/afts/next-hop-groups/next-hop-group[id=next-hop-group ID]/next-hops/state/index
                *   /network-instances/network-instance/afts/next-hop-groups/next-hop-group[id=next-hop-group ID]/next-hops/state/**weight**
        *   [TODO] Check entire traffic should  be unequally forwarded between DUT
            port2 and port3 only
                *   66% via port2
                *   33% via port3
                *   with +/-5% tolerance

*   [TODO] RT-1.52.2: Verify use of equal community type

    *   Test Configuration
        Use test configuration as in RT-1.52.1 above with following modifications:
        * Advertise IPv4 and IPv6 internal target, both, form both ATE port-1 and ATE port-2 in eBGP.
                * For ATE port 2 attach `link-bandwidth:23456:10K` extended-community
                * For ATE port 3 attach `link-bandwidth:23456:10K` extended-community
    *   Behaviour Validation
        *   Check entries in AFT for advertised prefix, it should have 2 entries.\
            [TODO] The `weight` leafs of next-hops shall be in 1:1 ratio.
            *   Find next-hop-group IDs for both internal target networks:
                *   /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry[prefix=IPv4 internal target network]/state/**next-hop-group**
                *   /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry[prefix=IPv6 internal target network]/state/**next-hop-group**
            *   using next-hop-group as key find number and weight of next-hops of both internal target network
                *   /network-instances/network-instance/afts/next-hop-groups/next-hop-group[id=next-hop-group ID]/next-hops/state/index
                *   /network-instances/network-instance/afts/next-hop-groups/next-hop-group[id=next-hop-group ID]/next-hops/state/**weight**
        *   [TODO] Check entire traffic should  be unequally forwarded between DUT
            port2 and port3 only
            *   50% via port2
            *   50% via port3
            *   with +/-5% tolerance

*   [TODO] RT-1.52.3: Verify BGP multipath when some path missing link-bandwidth extended-community

    *   Test Configuration
        Use test configuration as in RT-1.52.1 above with following modifications:
        *   Configure ATE port 1, 2 and 3 on different AS, with boths AFI/SAFI
            * Advertise IPv4 and IPv6 internal target, both, form both ATE port-1 and ATE port-2 in eBGP.
            * For ATE port 2 attach `link-bandwidth:23456:10K` extended-community
            * For ATE port 3 **DO NOT** attach any link-bandwidth extended-community
    *   Behaviour Validation
        *   Check entries in AFT for advertised prefix, it should have 2 entries.\
            [TODO] The `weight` leafs of next-hops shall be in 1:1 ratio.
            *   Find next-hop-group IDs for both internal target networks:
                *   /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry[prefix=IPv4 internal target network]/state/**next-hop-group**
                *   /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry[prefix=IPv6 internal target network]/state/**next-hop-group**
            *   using next-hop-group as key find number and weight of next-hops of both internal target network
                *   /network-instances/network-instance/afts/next-hop-groups/next-hop-group[id=next-hop-group ID]/next-hops/state/index
                *   /network-instances/network-instance/afts/next-hop-groups/next-hop-group[id=next-hop-group ID]/next-hops/state/**weight**
        *   [TODO] Check entire traffic should  be unequally forwarded between DUT
            port2 and port3 only
            *   50% via port2
            *   50% via port3
            *   with +/-5% tolerance


## Config Parameter Coverage

*   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/use-multiple-paths/config/enabled
*   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/use-multiple-paths/ebgp/config/allow-multiple-as
*   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/use-multiple-paths/ebgp/config/maximum-paths
*   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/send-community-type
*   /network-instances/network-instance/protocols/protocol/bgp/global/use-multiple-paths/ebgp/link-bandwidth-ext-community/config/enabled

## Telemetry Parameter Coverage

*   /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group
*   /network-instances/network-instance/afts/next-hop-groups/next-hop-group[id=<id>]/next-hops/next-hop[index=<index>]/state/weight

## OpenConfig Path and RPC Coverage

```yaml
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Subscribe:
```
## Required DUT platform

*   FFF - Fixed Form Factor

