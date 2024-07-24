# TE-3.3: Hierarchical weight resolution

## Summary

Ensures that next-hop weights (for WCMP) are honored hierarchically in gRIBI
recursive resolution and traffic is load-shared according to these weights.

## Procedure

Configure ATE and DUT:

*   Connect ATE port-1 to DUT port-1. ATE port-2 to DUT port-2.

*   Create a non-default VRF (VRF-1) that contains no interfaces.

*   On DUT port-2 and ATE port-2 create 18 L3 sub-interfaces each with a /30
    subnet as below:

    *   On DUT port-2, create subinterfaces with indices 1 to 18 mapped to VLAN
        IDs 1 to 18 and corressponding IPv4 addresses 192.0.2.5, 192.0.2.9, ...,
        192.0.2.73 respectively.

    *   On ATE port-2, create subinterfaces with indices 1 to 18 mapped to VLAN
        IDs 1 to 18 and corresponding IPv4 addresses 192.0.2.6, 192.0.2.10, ...,
        192.0.2.74 and default gateways as 192.0.2.5, 192.0.2.9, ..., 192.0.2.73
        respectively.

*   On DUT port-1 and ATE port-1 create a single L3 interface.

*   On DUT, create a policy-based forwarding rule to redirect all traffic
    received from DUT port-1 into VRF-1 (based on src. IP match criteria).

Test case for basic hierarchical weight:

*   Establish gRIBI client connection with DUT with PERSISTENCE, make it become
    leader and install the following Entries:

    *   IPv4Entry 203.0.113.0/32 in VRF-1, pointing to NextHopGroup(NHG#1) in
        default VRF, with two NextHops(NH#1, NH#2) in default VRF:

        *   NH#1 with weight:1, pointing to 192.0.2.111

        *   NH#2 with weight:3, pointing to 192.0.2.222

    *   IPv4Entry 192.0.2.111/32 in default VRF, pointing to NextHopGroup(NHG#2)
        in default VRF, with two NextHops(NH#10, NH#11) in default VRF:

        *   NH#10 with weight:1, pointing to 192.0.2.10

        *   NH#11 with weight:3, pointing to 192.0.2.14

    *   IPv4Entry 192.0.2.222/32 in default VRF, pointing to NextHopGroup(NHG#3)
        in default VRF, with two NextHops(NH#100, NH#101) in default VRF:

        *   NH#100 with weight:3, pointing to 192.0.2.18

        *   NH#101 with weight:5, pointing to 192.0.2.22

*   Validate with traffic:

    *   NH10: (1/4) * (1/4) = 6.25% traffic received by ATE port-2 VLAN 1

    *   NH11: (1/4) * (3/4) = 18.75% traffic received by ATE port-2 VLAN 2

    *   NH100: (3/4) * (3/8) = 28.12% traffic received by ATE port-2 VLAN 3

    *   NH101: (3/4) * (5/8) = 46.87% traffic received by ATE port-2 VLAN 4

    *   A tolerance of 0.2% is allowed for each VLAN for now, since we only test
        for 2 mins.

Test case for hierarchical weight in boundary scenarios, with maximum expected
WCMP width of 16 nexthops:

*   Flush previous gRIBI Entries for all NIs and establish a new connection with
    DUT with PERSISTENCE and install the following Entries:

    *   IPv4Entry 203.0.113.0/32 in VRF-1, pointing to NextHopGroup(NHG#1) in
        default VRF, with two NextHops(NH#1, NH#2) in default VRF:

        *   NH#1 with weight:1, pointing to 192.0.2.111

        *   NH#2 with weight:31, pointing to 192.0.2.222

    *   IPv4Entry 192.0.2.111/32 in default VRF, pointing to NextHopGroup(NHG#2)
        in default VRF, with two NextHops(NH#10, NH#11) in default VRF:

        *   NH#10 with weight:3, pointing to 192.0.2.10

        *   NH#11 with weight:5, pointing to 192.0.2.14

    *   IPv4Entry 192.0.2.222/32 in default VRF, pointing to NextHopGroup(NHG#3)
        in default VRF, with 16 NextHops(NH#100, NH#101, ..., NH#115), all with
        weight: 16 except NHG#100 is of weight 1, in default VRF:

        *   NH#100 with weight:1, pointing to 192.0.2.18

        *   NH#101 with weight:16, pointing to 192.0.2.22

        *   ...

        *   NH#115 with weight:16, pointing to 192.0.2.79

*   Validate with traffic:

    *   NH10: (1/32) * (3/8) ~ 1.171% traffic received by ATE port-2 VLAN 1

    *   NH11: (1/32) * (5/8) ~ 1.953% traffic received by ATE port-2 VLAN 2

    *   NH100: (31/32) * (1/241) ~ 0.402% traffic received by ATE port-2 VLAN 3

    *   for each VLAN ID in 4...18:

        *   NH: (31/32) * (16/241) ~ 6.432% traffic received by ATE port-2 VLAN
            ID

    *   A tolerance of 0.2% is allowed for each VLAN for now, since we only test
        for 2 mins.

## Config Parameter Coverage

N/A

## OpenConfig Path and RPC Coverage
```yaml
paths:
  ## State Paths ##
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/weight:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
  gribi:
    gRIBI.Get:
    gRIBI.Modify:
    gRIBI.Flush:
```

## Minimum DUT platform requirement

* vRX - virtual router device

