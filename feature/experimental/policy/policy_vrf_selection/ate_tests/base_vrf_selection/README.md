# RT-3.1: Policy based VRF selection

## Testcase summary

Test different VRF selection policies.

## Topology

*   Connect ATE Port-1 to DUT Port-1, and ATE Port-2 to DUT Port-2.

*   Configure DUT with ingress interface as Port-1 and egress interface as Port-2. DUT Port-2 is connected to ATE Port-2 and is configured for VLAN-10 and VLAN-20 such that sub-interfaces Port-2.10 and Port-2.20 are part of VLAN 10 and VLAN 20 respectively.

*   Configure network-instance “VRF-10” of type L3VRF and assign Port2.10 to network-instance "VRF-10" 
        
*   Configure network-instance “DEFAULT” of type DEFAULT_INSTANCE. DUT-Port1 and DUT Port-2.20 should be part of "DEFAULT" Network w/o requiring 
    explicit configuration.

## Procedure

*   Test-Case 1

        *   Configure DUT to match on IPinIP protocol (protocol number 4) in the outer IPv4 header and punt it to 
            network-instance VRF-10. All other traffic should be punted to the Default VRF. These will be, native IPv4, 
            native IPv6 and IPv6inIP (protocol 41 in the outer IPv4 header) traffic.
        
        *   Start all traffic flows defined in the section "Flows" below and validate packet forwarding.

*   Test-Case 2

        *   Configure DUT to match on IPinIP protocol (protocol number 4 in the outer IPv4 header) with specific outer 
            IPv4 source address as "222.222.222.222" and punt it to network-instance VRF-10. All other traffic should be 
            punted to the Default VRF. These will be, IPinIP w/o source as "222.222.222.222", native IPv4, native IPv6 and 
            IPv6inIP (protocol 41 in the outer IPv4 header) traffic.

        *   Start all traffic flows defined in the section "Flows" below and validate packet forwarding.   


*   Test-Case 3

        *   Configure DUT to match on IPv6inIP protocol (protocol number 41 in the outer IPv4 header) and punt it to 
            network-instance VRF-10. All other traffic should be punted to the Default VRF. These will be, native IPv4, 
            native IPv6 and IPinIP (protocol 4 in the outer IPv4 header) traffic.

        *   Start all traffic flows defined in the section "Flows" below and validate packet forwarding.        

* Test-Case 4
          
        *  Configure DUT to match on IPv6inIP protocol (protocol number 41 in the outer IPv4 header) with specific 
           outer IPv4 source address "222.222.222.222" and punt it to the network-instance VRF-10. 
           All other traffic should be punted to the Default VRF. These will be, IPv6inIP w/o source as 
           "222.222.222.222", native IPv4, native IPv6 and IPinIP (protocol 4 in the outer IPv4 header) traffic.

        *   Start all traffic flows defined in the section "Flows" below and validate packet forwarding. 

## Flows

*   IPinIP

        *   Flow#1: IPinIP with outer source as not "222.222.222.222" and outer destination as the directly connected 
            ATE IPv4 address in the VRF that the flow is expected to land.
        *   Flow#2: IPinIP with outer source as "222.222.222.222" and outer destination as the directly connected 
            ATE IPv4 address in the VRF that the flow is expected to land.

*   IPv6inIP

        *   Flow#1: IPv6inIP with outer source as not "222.222.222.222" and outer destination as the directly connected 
            ATE IPv4 address in the VRF that the flow is expected to land.
        *   Flow#2: IPv6IP with outer source as "222.222.222.222" and outer destination as the directly connected ATE 
            IPv4 address in the VRF that the flow is expected to land.

*   Native IPv4

        *   Flow#1: Native IPv4 flow with any source address and destination as the IPv4 address of the Directly connected 
            ATE interface in the DEFAULT VRF.
        
*   Native IPv6

        *   Flow#1: Native IPv6 flow with any source address and destination as the IPv6 address of the Directly connected 
            ATE interface in the DEFAULT VRF.
