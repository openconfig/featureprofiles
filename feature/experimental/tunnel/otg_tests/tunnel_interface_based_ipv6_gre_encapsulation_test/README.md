# TUN-1.4: Interface based IPv6 GRE Encapsulation

## Summary

Validate Interface based Ipv6 GRE Tunnel Config.

## Procedure
Validate the GRE configuration
GRE Tunnel interfaces configuration options
Tunnel source
Tunnel source should be able to configure with unnumbered interface address
Tunnel Destination
Directly on Tunnel interface or Tunnel Group
Configure such 32 tunnel interfaces
Filter match and action
Filter is able to match the source and destination IP ranges or prefix lists
Filter is able to divert traffic to IPv6 Next HOP
Filter is able to count the packet
Filter diverting traffic to IPv6 NH
Configure static route with IPv6 NH pointing to Tunnel interfaces
Filter application to specific interface
Apply filter on IPv6 AF on DUT-PORT1
Send 1000 IPv6 flows from tester on ATE-PORT1 connected to DUT-PORT1  where GRE Encap filter is applied with the traffic profile documented:
IPv6 flows are made of 5 tuples:
Source IP address
Source port
Destination IP address
Destination port
Payload protocol

Enable the packet capture on ATE ingress port to verify the GRE header of uncapped traffic.
Verify the filter counters for packet being diverted or sent for encapsulation.
Verify the tunnel interfaces counters to confirm the traffic encapsulation.
After encapsulation, traffic should be load balanced/hash to all available L3 ECMP or LAG or combination of both features.
Validate the applied config did not report any errors.
Validate system for:
Health-1.1
No feature related error or drop counters incrementing, discussion with vendors required to highlight additional fields to monitor based on implementation and architecture

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