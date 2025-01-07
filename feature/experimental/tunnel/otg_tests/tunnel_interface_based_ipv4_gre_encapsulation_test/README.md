# TUN-1.3: Interface based IPv4 GRE Encapsulation

## Summary

Validate Interface based Ipv4 GRE Tunnel Config.

## Procedure

- Validate the GRE configuration
  -  GRE Tunnel interfaces configuration options
     -  Tunnel source
     -  Tunnel source should be able to configure with unnumbered interface address
     -  Tunnel Destination
     -  Directly on Tunnel interface or Tunnel Group
- Configure such 2000 tunnel interfaces
- Configure static route with IPv4 NH pointing to Tunnel interfaces
- Send 1000 IPv4 flows from tester on ATE-PORT1 connected to DUT-PORT1
- Enable the packet capture on ATE ingress port to verify the GRE header of encapped traffic
- Verify the tunnel interfaces counters to confirm the traffic encapsulation
- After encapsulation, traffic should be load balanced/hash to all available L3 ECMP or LAG or combination of both features
- Validate system for:
  - TODO: Health-1.1
  - TODO: No feature related error or drop counters incrementing, discussion with vendors required to highlight additional fields to monitor based on implementation and architecture

## Config Parameter coverage

- Prefix: wbb://software/interfaces/tunnel/
- Parameters:
  - gre/
  - gre/decap-group/
  - gre/dest/
  - gre/dest/address/
  - gre/dest/address/ipv4/
  - gre/dest/nexthop-group/
  - gre/source/
  - gre/source/address/
  - gre/source/address/ipv4/
  - gre/source/interface/
  - Prefix:
  - wbb://software/routing/nexthop-group/
  - wbb://software/routing/nexthop-group/gre/

- Prefix:
- wbb://software/routing/static/
- Parameters:
  - ipv4/
  - ipv4/admin-dist/
  - ipv4/nexthop/
  - ipv4/nexthop/null/

## Telemetry Parameter coverage

- Prefix:
- wbb://software/interfaces/tunnel/
- wbb://software/interfaces/tunnel/gre/
- Needs to define
  - ST for Tunnel counters
  - ST for packet capture on DUT and ATE to read:
  - Packet metadata
  - IP Source
  - IP Destination
  - IP Protocol number
  - state/counters/in-pkts
  - state/counters/in-octets
  - state/counters/in-error-pkts
  - state/counters/in-forwarded-pkts
  - state/counters/in-forwarded-octets
  - state/counters/in-discarded-pkts
  - state/counters/out-pkts
  - state/counters/out-octets
  - state/counters/out-error-pkts
  - state/counters/out-forwarded-pkts
  - state/counters/out-forwarded-octets
  - state/counters/out-discarded-pkt
  - Fragmentation and assembly counters Filter counters Output to display the traffic is spread across the different tunnel subnet ranges/NH groups/Interfaces