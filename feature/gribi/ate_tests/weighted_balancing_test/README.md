# TE-3.2: Traffic Balancing According to Weights

# Summary

Ensure that traffic splits within a `NextHopGroup` are correctly honoured.

# Procedure

*   Configure ATE port-1 connected to DUT port-1, and ATE port 2-9 connected to
    DUT port 2-9. Connect to gRIBI with persistence `PRESERVE`, make it become
    leader and flush all entries before each case.

*   Via gRIBI, install an `IPv4Entry` for 203.0.113.0/24 pointing to a
    `NextHopGroup` id 10.

*   Install next-hops corresponding to each of ATE port 2-9 from the DUT mapped
    to an IPv4 address, e.g., 192.0.2.2/30 corresponding to ATE port 2. For the
    following cases, verify (ensuring traffic with sufficient entropy - mixed
    IPv4 source and destination ports):

    *   With NHG 10 containing 1 next hop, 100% of traffic is forwarded to the
        installed next-hop.
    *   With NHG 10 containing 2 next hops with no associated weights assigned,
        50% of traffic is forwarded to each next-hop.
    *   With NHG 10 containing 8 next hops, with no associated weights assigned,
        12.5% of traffic is forwarded to each next-hop.
    *   With NHG 10 containing 2 next-hops, specify and validate the following
        ratios:

        *   Weight 1:1 - 50% per-NH.
        *   Weight 2:1 - 66% traffic to NH1, 33% to NH2.
        *   Weight 9:1 - 90% traffic to NH1, 10% to NH2.
        *   Weight 31:1 - ~96.9% traffic to NH1, ~3.1% to NH2.
        *   Weight 63:1 - ~98.4% traffic to NH1, ~1.6% to NH2.

*   Validate that weights of:

    *   <64K are supported
    *   \>64K are correctly balanced if the device supports it.

*   With NHG10 containing 8 next-hops, with a weight of 1 assigned to each,
    sequentially remove each next-hop by turning down the port at the ATE
    (invalidates nexthop), ensure that traffic is rebalanced across remaining
    NHs until only one NH remains.

# Telemetry Parameter Coverage

*   TODO:
    /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/weight

# Protocol/RPC

*   gRIBI:
    *   Modify()
        *   ModifyRequest:
            *   AFTOperation:
                *   next\_hop\_group
                    *   NextHopGroupKey: id
                    *   NextHopGroup: weight
