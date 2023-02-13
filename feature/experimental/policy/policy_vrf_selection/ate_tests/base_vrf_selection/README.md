# RT-3.1: Policy based VRF selection

## Testcase summary

Test different VRF selection policies.

## Procedure

*   Connect ATE Port-1 to DUT Port-1, and ATE Port-2 to DUT Port-2.

*   Configure DUT with ingress interface as Port-1 and egress interface as Port-2. DUT Port-2 is connected to ATE Port-2 and is configured for VLAN-10 and VLAN-20 such that sub-interfaces Port-2.10 and Port-2.20 are part of VLAN 10 and VLAN 20 respectively.

*   Configure network-instance “VRF-10” of type L3VRF and assign Port2.10 to network-instance "VRF-10" 
        
*   Configure network-instance “DEFAULT” of type DEFAULT_INSTANCE. DUT-Port1 and DUT Port-2.20 should be part of "DEFAULT" Network instance by default w/o requiring explicit configuration.

*   Configure Network-instance “VRF-10” with a default route pointing at ATE Port-2.10

*   Configure Network-instance "DEFAULT" with IPv4 and IPv6 default routes pointing at ATE Port-2.20

*   Configure DUT and validate packet forwarding for:

        *   Matching of IPinIP protocol (protocol number 4 in the outer IP header) - to network-instance VRF-10 for all input IPinIP

            *   All other traffic should be punted to the Default VRF. These will be, native IPv4, native IPv6 and IPv6inIP (protocol 41 in the outer IPv4 header) traffic
        
        *   Matching of IPinIP protocol (protocol number 4 in the outer IP header) with specific outer IPv4 source address "222.222.222.222" - to network-instance VRF-10

            *    All other traffic should be punted to the Default VRF. These will be, IPinIP w/o source as "222.222.222.222", native IPv4, native IPv6 and IPv6inIP (protocol 41 in the outer IPv4 header) traffic.

        *   Matching of IPv6inIP protocol (protocol number 41 in the outer IPv4 header) - to network-instance VRF-10 for all input IPv6inIP

            *   All other traffic should be punted to the Default VRF. These will be, native IPv4, native IPv6 and IPinIP (protocol 4 in the outer IPv4 header) traffic.
        
        *   Matching of IPv6inIP protocol (protocol number 41 in the outer IP header) with specific outer IPv4 source address "222.222.222.222" - to network-instance VRF-10

            *   All other traffic should be punted to the Default VRF. These will be, IPv6inIP w/o source as "222.222.222.222", native IPv4, native IPv6 and IPinIP (protocol 4 in the outer IPv4 header) traffic



## Flows

*   IPinIP

        *   Flow#1: IPinIP with outer source as not "222.222.222.222"
        *   Flow#2: IPinIP with outer source as "222.222.222.222"

*   IPv6inIP

        *   Flow#1: IPv6inIP with outer source as not "222.222.222.222"
        *   Flow#2: IPv6IP with outer source as "222.222.222.222"

*   Native IPv4

        *   Flow#1: Native IPv4 flow with any source address
        
*   Native IPv6

        *   Flow#1: Native IPv6 flow with any source address


