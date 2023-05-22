# RT-3.1: Policy based VRF selection

## Testcase summary

Ensure that protocol and DSCP based VRF selection is configured correctly.

## Testcase procedure

*   Configure DUT with 1 input interface and egress VLANs via a single DUT port-2 connected to ATE port-2 corresponding to:
    *   network-instance “10”, default route to egress via VLAN 10
    *   network-instance “20”, default route to egress via VLAN 20
*   Configure DUT and validate packet forwarding for:
    *   Matching of IPv4 protocol - to network-instance 10 for all input IPv4.
        No received IPv6 in network-instance 10.
    *   Matching of a specific IPv4 source address - to network-instance 10. 
        No other received IPv4 in network-instance 10
    *   Matching of IPinIP protocol - to network-instance 10 for all input IPinIP
        No received IPv4 or IPv6 in network-instance 10.
    *   Matching of IPv4 to network-instance 10, and IPv6 to network-instance 20 
        Ensure that only IPv4 packets are seen at egress VLAN 10, and IPv6 at received VLAN 20.
    *   Match IPinIP, single DSCP 46 - routed to network-instance 10
        Ensure that only DSCP 46 packets are received at VLAN 10.
        Ensure that DSCP 0 packets are not received at VLAN 10.
    *   Match IPinIP, DSCP 42, 46 - routed to network-instance 10, ensure that DSCP 42 and 46 packets are received at VLAN
        Ensure that DSCP 0 packets are not received at VLAN 10.
