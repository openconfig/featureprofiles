# TE-2.2: gRIBI IPv4 Entry With Aggregate Ports

## Summary

Validate IPv4 support in gRIBI using an Aggregate Port as a static route Next
Hop.

## Procedure

*   Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2, and ATE port-3
    to DUT port-3.
*   Configure ATE and DUT ports 2-3 to be part of a Static LAG.
*   Establish gRIBI client connection with DUT, negotiating `RIB_AND_FIB_ACK` as
    the requested `ack_type` and persistence mode `PRESERVE`. make it become
    leader. Flush all entries after each case.
*   Using gRIBI Modify RPC install the following IPv4Entry sets, and validate
    the specified behaviours:

    *   Single IPv4Entry -> NHG -> NH with MAC Override.

        *   Configure a static ARP entry for the LAG interface pointing the
            synthetic IP 192.0.2.22 to the neighbor's MAC address
            02:00:00:00:00:01.
        *   Configure a static route matching 192.0.2.22/32 to the interface ref
            of the LAG.
        *   The gRIBI NH entry uses 192.0.2.22 to select the LAG as the egress
            interface.

    *   Install 198.51.100.0/24 to NextHopGroup containing one NextHop which is
        a static route to the ATE LAG port containing ports 2-3, and override
        the destination MAC to a specified value.

    *   Forward packets between ATE port-1 and ATE LAG (destined to
        198.51.100.0/24 i.e. packets with destination IP starting 198.51.100.1
        up to 198.51.100.255) and determine that packets are forwarded
        successfully.

    *   Disable ATE port-2 and forward packets between ATE port-1 and ATE LAG
        (destined to 198.51.100.0/24 ) and determine that packets are forwarded
        successfully.

    *   Disable ATE port-2 and port-3 and forward packets between ATE port-1 and
        ATE LAG (destined to 198.51.100.0/24 ) and determine that packets are
        lost 100%.

    *   Re-enable both ATE port-2 and port-3 and forward packets between ATE
        port-1 and ATE LAG (destined to 198.51.100.0/24 ) and determine that
        packets are forwarded successfully again.

## Config Parameter coverage

N/A

## Telemetry Parameter coverage

N/A

## Protocol/RPC Parameter coverage

*   gRIBI
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
        *   ModifyResponse:
            *   AFTResult:
                *   id
                *   status
