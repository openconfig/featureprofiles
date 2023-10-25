# TE-2.1.1: Single Decap gRIBI Entry

## Summary

Support for decap action installed through gRIBI.

## Procedure

*   Connect ATE port-1 to DUT port-1, and ATE port-2 to DUT port-2.

*   Apply vrf_selectioin_policy_w to DUT port-1.

*   Using gRIBI, install the following gRIBI AFTs, and validate the specified behavior. This is a 
    parameterized test. Specifically, we want to repeat this test with different values for the subnet masks as specified below.

    *   Using gRIBI, install an  IPv4Entry for the prefix 192.51.100.1/24 that points to a NextHopGroup
        that contains a single NextHop that specifies decapsulating the IPv4 header and specifies the DEFAULT network instance. This IPv4Entry should be installed into the DECAP_TE_VRF.  

    *   Send both 6in4 and 4in4 packets to  DUT port-1. The outer v4 header should be 
        `{dst:192.51.100.64, src:ipv4_outer_src_111}`. Pick some inner header destination address for which there’s a route in the DEFAULT VRF.

    *   Verify that the packets have their outer v4 header stripped and are forwarded according to the
        route in the DEFAULT VRF that matches the inner IP address.

    *   Verify that the TTL value is copied from the outer header to the inner header. 

    *   Change the subnet mask from /24 and repeat the test for the masks  /32, /22, and /28 and verify 
        again that the packets are decapped and forwarded correctly. 

    *   Repeat the test with packets with a destination address such as 192.58.200.7 that does not
        match the decap route, and verify that such packets are not decapped.

## Protocol/RPC Parameter coverage

## Config parameter coverage

## Telemery parameter coverage
