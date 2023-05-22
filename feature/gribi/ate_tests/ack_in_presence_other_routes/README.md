# TE-3.6: ACK in the Presence of Other Routes

## Summary

Ensure that ACKs are received in the presence of other routes.

## Procedure

*   Connect DUT port-1 to ATE port-1, DUT port-2 to ATE port-2, DUT port-3 to
    ATE port-3. Assign IPv4 addresses to all ports.

*   Configure static routes on the DUT for 203.0.113.0/24 pointing to ATE
    port-2. Ensure that the static route is installed in the DUT.

*   Connect gRIBI client to DUT specifying persistence mode `PRESERVE`,
    `SINGLE_PRIMARY` client redundancy in the SessionParameters request, and
    make it become leader. Ensure that no error is reported from the gRIBI
    server.

*   Add an `IPv4Entry` for same prefix `203.0.113.0/24` pointing to ATE port-3
    via `gRIBI-A`, ensure that the entry is active through AFT telemetry and
    correct ACK is received – InstalledInRIB.

*   Send traffic from ATE port-1 to prefix `203.0.113.0/24`, and ensure traffic
    flows 100% using the static route configured at ATE port-2.

## Protocol/RPC Parameter coverage

*   gRIBI:
    *   Modify()
    *   ModifyRequest:
        *   AFTOperation:
        *   id
        *   network_instance
        *   op
        *   Ipv4
            *   Ipv4EntryKey: prefix
            *   Ipv4Entry: next_hop_group
        *   next_hop_group
            *   NextHopGroupKey: id
            *   NextHopGroup: next_hop
        *   next_hop
            *   NextHopKey: id
            *   NextHop:
            *   ip_address
            *   interface_ref
    *   ModifyResponse:
    *   AFTResult:
        *   id
        *   status

## Config parameter coverage

*   /network-instance/name/
*   /network-instance/config/type
*   /network-instance/name/protocols/protocol/identifier/
*   /network-instance/name/protocols/protocol/name/
*   /network-instance/name/protocols/protocol/identifier/static-routes/static/prefix
*   /network-instance/name/protocols/protocol/identifier/static-routes/static/prefix/config/prefix
*   /network-instance/name/protocols/protocol/identifier/static-routes/static/next-hops/next-hop/index
*   /network-instance/name/protocols/protocol/identifier/static-routes/static/next-hops/next-hop/config/index
*   /network-instance/name/protocols/protocol/identifier/static-routes/static/next-hops/next-hop/config/next-hop

## Telemery parameter coverage

*   /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix/
