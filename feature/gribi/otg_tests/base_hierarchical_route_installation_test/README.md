# TE-3.1: Base Hierarchical Route Installation

## Summary

Validate IPv4 AFT support in gRIBI with recursion.

## Procedure

Topology:

*   Connect ATE port-1 to DUT port-1 and ATE port-2 to DUT port-2.
*   Create a non-default VRF (VRF-1) 
*   Configure PBF policy in DEFAULT to match src address and redirect traffic to VRF-1
*   Apply PBF policy on ingress interface DUT port-1
*   Establish gRIBI client connection with DUT using default parameters and
    persistence mode `PRESERVE`, make it become leader and flush all entries
    after each case.

Validate hierarchical resolution across VRFs:

1.  Using gRIBI Modify RPC install the following IPv4Entry set per the following
    order, and ensure FIB ACK is received for each of the AFTOperation:

    1.  Add 203.0.113.1/32 (default VRF) to NextHopGroup (default VRF)
        containing one NextHop (default VRF) specified to be the address of ATE
        port-2.
    2.  Add 198.51.100.1/32 (VRF-1) to NextHopGroup (default VRF)
        containing one NextHop (default VRF) specified to be 203.0.113.1/32 in
        the default VRF.

2.  Forward packets between ATE port-1 and ATE port-2 (destined to
    198.51.100.1/32) and determine that packets are forwarded successfully.

3.  Validate that both routes are shown as installed via AFT telemetry.

4.  Ensure that removing the IPv4Entry 203.0.113.1/32 with a DELETE operation
    results in traffic loss, and removal from AFT.

Validate hierarchical resolution using egress interface and MAC:

1.  Using gRIBI Modify RPC install the following IPv4Entry set per the following
    order, and ensure FIB ACK is received for each of the AFTOperation:

    1.  Add 203.0.113.1/32 (default VRF) to NextHopGroup (default VRF)
        containing one NextHop (default VRF) that specifies DUT port-2 as the
        egress interface and `00:1A:11:00:0A:BC` as the destination MAC address.
    2.  Add 198.51.100.1/32 (VRF-1) to NextHopGroup (default VRF) containing one
        NextHop (default VRF) specified to be 203.0.113.1/32 in the default VRF.

2.  Forward packets between ATE port-1 and ATE port-2 (destined to
    198.51.100.1/32) and ensure that ATE port-2 receives packet with
    `00:1A:11:00:00:01` as the destination MAC address.

3.  Repeat the above tests with one additional scenario with the following changes, and it should
    not change the expected test result.

    *   Add an empty decap VRF, `DECAP_TE_VRF`.
    *   Add 4 empty encap VRFs, `ENCAP_TE_VRF_A`, `ENCAP_TE_VRF_B`, `ENCAP_TE_VRF_C`,
        and `ENCAP_TE_VRF_D`.
    *   Add 2 empty transit VRFs, `TE_VRF_111` and `TE_VRF_222`.
    *   Program route 198.51.100.1/32 through gribi in `TE_VRF_111` instead of `VRF-1`.
    *   Replace the existing VRF selection policy with `vrf_selection_policy_w` as in
        <https://github.com/openconfig/featureprofiles/pull/2217>.
    *   Send IP-In-IP traffic with source IP to ipv4_outer_src_111 (`198.51.100.111`) and DSCP to
        dscp_encap_a_1(10).
    
## Config Parameter coverage

No configuration relevant.

## Telemetry Parameter coverage

### For prefix:

*   /network-instances/network-instance/afts/

### Parameters:

*   ipv4-unicast/ipv4-entry/state
*   ipv4-unicast/ipv4-entry/state/next-hop-group
*   ipv4-unicast/ipv4-entry/state/origin-protocol
*   ipv4-unicast/ipv4-entry/state/prefix
*   next-hop-groups/next-hop-group/id
*   next-hop-groups/next-hop-group/next-hops/next-hop/index
*   next-hop-groups/next-hop-group/next-hops/next-hop/state
*   next-hop-groups/next-hop-group/next-hops/next-hop/state/index
*   next-hop-groups/next-hop-group/state/id
*   next-hop-groups/next-hop-group/state/programmed-id
*   next-hops/next-hop/index
*   next-hops/next-hop/interface-ref/state/interface
*   next-hops/next-hop/interface-ref/state/subinterface
*   next-hops/next-hop/state/index
*   next-hops/next-hop/state/state/programmed-id 
*   next-hops/next-hop/state/ip-address
*   next-hops/next-hop/state/mac-address

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
            *   mac_address
            *   interface_ref
    *   ModifyResponse:
    *   AFTResult:
        *   id
        *   status

## Minimum DUT platform requirement

vRX if the vendor implementation supports FIB-ACK simulation, otherwise FFF.

## OpenConfig Path and RPC Coverage
```yaml
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
