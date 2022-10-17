# Not Know 

## Summary 

Validate static routing for a DUT.

## Procedure

*   Configure a DUT that has 4 interfaces to an ATE device configured with IPv4 and IPv6 addresses at both ends.
*   Configure ATE port-1 connected to DUT port-1, and ATE port 2-4 connected to DUT port 2-4 with the relevant IPv4 addresses.
*   Using the OpenConfig static routing configuration validate simple static routes for IPv4:
    *   Add a static route for an IPv4 prefix 10.0.0.0/24 that routes packets from ATE port 1 to the next-hop address on each ATE port 2-4:
        *   Validate that packets are received at each ATE port 2-4 when sourced from ATE port 1.

## Config Paramter Coverage 

*   /interfaces/interface/config/name
*   /interfaces/interface/config/description
*   /interfaces/interface/config/enabled
*   /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length
*   /network-instances/network-instance/protocols/protocol/config/name
*   /network-instances/network-instance/protocols/protocol/static-routes/static/prefix
*   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop


## Telemetry Parameter Coverage

*   /interfaces/interface/state/counters/out-octets
*   /interfaces/interface/state/counters/out-pkts
*   /interfaces/interface/state/counters/in-pkts

## Protocol/RPC Parameter Coverage

None

## Minimum DUT Platform Requirement

vRX


