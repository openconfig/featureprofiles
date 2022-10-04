# TE-3.3: Hierarchical weight resolution

## Summary

Ensures that next-hop weights (for WCMP) are honored hierarchically in gRIBI
recursive resolution and traffic is load-shared according to these weights.

## Procedure

Configure ATE and DUT:

*   Connect ATE port-1 to DUT port-1. ATE port-2 to DUT port-2

*   Create a non-default VRF (VRF-1) that includes DUT port-1.

*   For DUT port-2 interface, create a subinterface with Index 0 and IPv4
    address 192.0.2.5, not in a VLAN.

*   For ATE port-2, create a subinterface with IPv4 address 192.0.2.6 and
    Default Gateway of 192.0.2.5, not in a VLAN.

*   Repeat for 18 more subinterfaces in a VLAN configuration:

    *   For DUT port-2, subinterfaces indices 1...18 with VLAN IDs 1...18 and
        corresponding IPv4 addresses 192.0.2.9 ... 192.0.2.73

    *   For ATE port-2, subinterfaces with VLAN IDs 1...18 and corresponding
        IPv4 addresses 192.0.2.10 ... 192.0.2.79 with default gateways of
        192.0.2.9 ... 192.0.2.78

Test case for basic hierarchical weight:

*   Establish gRIBI client connection with DUT with PERSISTENCE and install the
    following Entries:

    *   IPv4Entry 203.0.113.0/24 in VRF-1, pointing to NextHopGroup(NHG#1) in
        default VRF, with two NextHops(NH#1, NH#2) in default VRF:

        *   NH#1 with weight:1, pointing to 192.0.2.111

        *   NH#2 with weight:2, pointing to 192.0.2.222

    *   IPv4Entry 192.0.2.111/32 in default VRF, pointing to NextHopGroup(NHG#2)
        in default VRF, with two NextHops(NH#10, NH#11) in default VRF:

        *   NH#10 with weight:1, pointing to 192.0.2.10

        *   NH#11 with weight:3, pointing to 192.0.2.14

    *   IPv4Entry 192.0.2.222/32 in default VRF, pointing to NextHopGroup(NHG#3)
        in default VRF, with two NextHops(NH#100, NH#101) in default VRF:

        *   NH#100 with weight:2, pointing to 192.0.2.18

        *   NH#101 with weight:3, pointing to 192.0.2.22

*   Validate with traffic:

    *   NH10: (1/4) * (1/4) = 6.25% traffic received by ATE port-2 VLAN 1

    *   NH11: (1/4) * (3/4) = 18.75% traffic received by ATE port-2 VLAN 2

    *   NH100: (3/4) * (2/5) = 30% traffic received by ATE port-2 VLAN 3

    *   NH101: (3/4) * (3/5) = 45% traffic received by ATE port-2 VLAN 4

    *   A deviation of 0.5% is allowed for each VLAN for now, since we only test
        for 2 mins.

Test case for hierarchical weight in boundary scenarios, with maximum expected
WCMP width of 16 nexthops:

*   Flush previous gRIBI Entries for all NIs and establish a new connection with
    DUT with PERSISTENCE and install the following Entries:

    *   IPv4Entry 203.0.113.0/24 in VRF-1, pointing to NextHopGroup(NHG#1) in
        default VRF, with two NextHops(NH#1, NH#2) in default VRF:

        *   NH#1 with weight:1, pointing to 192.0.2.111

        *   NH#2 with weight:31, pointing to 192.0.2.222

    *   IPv4Entry 192.0.2.111/32 in default VRF, pointing to NextHopGroup(NHG#2)
        in default VRF, with two NextHops(NH#10, NH#11) in default VRF:

        *   NH#10 with weight:2, pointing to 192.0.2.10

        *   NH#11 with weight:3, pointing to 192.0.2.14

    *   IPv4Entry 192.0.2.222/32 in default VRF, pointing to NextHopGroup(NHG#3)
        in default VRF, with 16 NextHops(NH#100, NH#101, ..., NH#115), all with
        weight: 1, in default VRF:

        *   NH#100 with weight:1, pointing to 192.0.2.18

        *   NH#101 with weight:1, pointing to 192.0.2.22

        *   ...

        *   NH#115 with weight:1, pointing to 192.0.2.79

*   Validate with traffic:

    *   NH10: (1/32) * (2/5) = 1.25% traffic received by ATE port-2 VLAN 1

    *   NH11: (1/32) * (3/5) = 1.875% traffic received by ATE port-2 VLAN 2

    *   for each VLAN ID in 3...18:

        *   NH: (31/32) * (1/16) ~ 6.05% traffic received by ATE port-2 VLAN ID

    *   A deviation of 0.5% is allowed for each VLAN for now, since we only test
        for 2 mins.

## Config Parameter Coverage

N/A

## Telemetry Parameter Coverage

TODO:
/network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/weight

## Protocol/RPC Parameter coverage

*   gRIBI:
    *   Modify()
        *   ModifyRequest:
            *   AFTOperation:
                *   next_hop_group
                    *   NextHopGroupKey: id
                    *   NextHopGroup: weight

## Minimum DUT platform requirement

vRX
