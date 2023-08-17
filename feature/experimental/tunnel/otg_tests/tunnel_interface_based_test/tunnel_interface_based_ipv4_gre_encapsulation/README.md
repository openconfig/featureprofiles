# TUN-1.3: Interface based IPv4 GRE Encapsulation

## Summary

Validate Interface based Ipv4 GRE Tunnel Config.

## Procedure

Validate the GRE configuration
Tunnel endpoint configuration options
Tunnel source
Tunnel source should be able to configure with unnumbered interface address
Tunnel Destination
Filter match and action to divert traffic for GRE Encap/Decap
Filter is able to match the source and destination IP ranges or prefix lists
Filter is able to divert traffic to IPv4 and IPv6 Next HOP(NH) or GRE Encap/Decap instruction/action
Filter is able to count the packet
Filter application to specific interface address family
Apply filter on IPv4 and IPv6 AF on DUT-PORT1
Validate the applied config did not report any errors
Validate system for:
Health-1.1
With traffic running end to end, no feature related error or drop counters incrementing, discussion with vendors required to highlight additional fields to monitor based on implementation and architecture

## Config Parameter coverage

Prefix:
wbb://software/interfaces/tunnel/
Parameters:
gre/
gre/decap-group/
gre/dest/
gre/dest/address/
gre/dest/address/ipv4/
gre/dest/address/ipv6/
gre/dest/nexthop-group/
gre/source/
gre/source/address/
gre/source/address/ipv4/
gre/source/address/ipv6/
gre/source/interface/

Prefix:
wbb://software/forwarding/policy/acl/rule/
Parameters:
action/
action/count/
action/nexthop/
Prefix:
wbb://software/forwarding/policy/pbr/action/encap/ip-gre/
Prefix:
wbb://software/routing/nexthop-group/
wbb://software/routing/nexthop-group/gre/

## Telemetry Parameter coverage

Prefix:
wbb://software/interfaces/tunnel/
wbb://software/interfaces/tunnel/gre/
wbb://software/forwarding/policy/acl/rule/

ST for Tunnel counters
ST for packet capture on DUT and ATE to read:
Packet metadata
IP Source
IP Destination
IP Protocol number
state/counters/in-pkts
state/counters/in-octets
state/counters/in-error-pkts
state/counters/in-forwarded-pkts
state/counters/in-forwarded-octets
state/counters/in-discarded-pkts
state/counters/out-pkts
state/counters/out-octets
state/counters/out-error-pkts
state/counters/out-forwarded-pkts
state/counters/out-forwarded-octets
state/counters/out-discarded-pkts
Fragmentation and assembly counters
Filter counters
Output to display the traffic is spread across the different tunnel subnet ranges/NH groups/Interfaces