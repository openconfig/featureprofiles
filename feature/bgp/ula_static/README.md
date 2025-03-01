# RT-1.59: Support for configuring static IPv6 ULA/64 routes with higher administrative distance

## Summary

*   Static IPv6 ULA/64 routes TEST
Google is planning to use ULA space addresses (RFC4193) for internal purposes that will requires DCGate and POPGate devices to support static IPv6 ULA/64 routes.
    *   Learn ULA /64 prefixes from eBGP and/or iBGP sessions.
    *   Advertise ULA /64 to eBGP and iBGP sessions
    *   Configure static route for ULA /64 prefixes with IPv6 next-hop and high value of preference (admin distance - floating static).
    *   Install FIB entries for best route for ULA /64 prefix. Including eBGP multipath and static route ECMP.


## Procedure

*   When operating in "openconfig mode", NOS (network operating system) defaults should match what OC

*   Topology:
    *   ATE port-1 <------> port-1 DUT
    *   DUT port-2 <------> port-2 ATE
    *   DUT port-3 <------> port-3 ATE
    *   DUT port-4 <------> port-4 ATE
    *   DUT port-5 <------> port-5 ATE
    *   DUT port-6 <------> port-6 ATE

*   Test variables:
    # DSCP value that will be matched to ENCAP_TE_VRF_A
    * dscp_encap_a_1 = 10
    * dscp_encap_a_2 = 18

    # DSCP value that will be matched to ENCAP_TE_VRF_B
    * dscp_encap_b_1 = 20
    * dscp_encap_b_2 = 28

    # DSCP value that will NOT be matched to any VRF for encapsulation.
    * dscp_encap_no_match = 30

    # Magic source IP addresses used in VRF selection policy
    * ipv4_outer_src_111 = 198.51.100.111
    * ipv4_outer_src_222 = 198.51.100.222

    # Magic destination MAC address
    * magic_mac = 02:00:00:00:00:01`

*   [Test case-1.1] Configure static IPv6 ULA/64 routes:
    *   Push EBGP and IBGP OC configuration to the DUT 
        *   Configure static IPv6 ULA/64 routes and point to IPv6 next-hop       

    *   Verification:
        *   For each ULA /64 prefix, verify that the FIB entry is installed
            with the best route.
            *   If there is a static route, verify that the static route is
                installed.
            *   If there is no static route, verify that the best route from
                eBGP or iBGP is installed.
            *   Verify that the FIB entry is installed with the best route.
            *   Verify that the FIB entry is installed with the best route.

*   [Test case-1.2] Configure flows to IPv6 ULA/64 routes:
    *   Configure flows to IPv6 ULA/64 routes
        *   Configure flows to IPv6 ULA/64 routes with different DSCP values
            *   dscp_encap_a_1
            *   dscp_encap_a_2
            *   dscp_encap_b_1
            *   dscp_encap_b_2
            *   dscp_encap_no_match
        *   Configure flows to IPv6 ULA/64 routes with different source IPs
            *   ipv4_outer_src_111
            *   ipv4_outer_src_222
        *   Configure flows to IPv6 ULA/64 routes with different source MAC
            *   magic_mac
    *   Verification:
        *   Verify that the flows are encapped to the correct VRFs.
            *   dscp_encap_a_1 and dscp_encap_a_2 should be encapped to VRF-A
            *   dscp_encap_b_1 and dscp_encap_b_2 should be encapped to VRF-B
            *   dscp_encap_no_match should not be encapped to any VRF



## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

```yaml
paths:
    ## Config Parameter coverage

    /network-instances/network-instance/protocols/protocol/bgp/global/config/as:
    /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id:
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/auth-password:
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/neighbor-address:
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as:
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/neighbor-address:
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/config/enabled:
    /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/auth-password:
    /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-as:
    /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/config/enabled:
    /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/enabled:
    /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix:

    ## Telemetry Parameter coverage

    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/supported-capabilities: 
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/peer-type:
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/peer-as:
    /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/state/peer-type:
    /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/state/peer-as:
    /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/state/local-as:
    /network-instances/network-instance/protocols/protocol/static-routes/static/state/prefix:
rpcs:
    gnmi:
        gNMI.Set:
        gNMI.Get:
        gNMI.Subscribe:
```
## Minimum DUT Required

vRX - Virtual Router Device

