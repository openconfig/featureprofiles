# RT-3.1: Policy based VRF selection

## Testcase summary

Test different VRF selection policies.

## Topology

*   Connect ATE Port-1 to DUT Port-1, and ATE Port-2 to DUT Port-2.

*   Configure DUT with ingress interface as Port-1 and egress interface as Port-2. DUT Port-2 is connected to ATE Port-2 and is configured for VLAN-10 and VLAN-20 such that sub-interfaces Port-2.10 and Port-2.20 are part of VLAN 10 and VLAN 20 respectively.

*   Configure DUT with network-instance “VRF-10” of type L3VRF and assign Port2.10 to network-instance "VRF-10".
        
*   Configure DUT network-instance “DEFAULT” of type DEFAULT_INSTANCE. DUT-Port1 and DUT Port-2.20 should be part of "DEFAULT" Network w/o requiring 
    explicit configuration.
    
*   Configure ATE to advertise following IP addresses.

        *   IPv4: ATE-DEST-IPv4-VLAN10 in VLAN10
        *   IPv4: ATE-DEST-IPv4-VLAN20 in VLAN20
        *   IPv6: ATE-DEST-IPv6 in VLAN20

*   Configure DUT with following static routes for each VRF.

         *   Static route in VRF-10: ATE-DEST-IPv4-VLAN10 next-hop DUT:Port-2.10
         *   Static route in VRF DEFAULT: ATE-DEST-IPv4-VLAN20 next-hop DUT:Port-2.20
         *   Static route in VRF DEFAULT: ATE-DEST-IPv6 next-hop DUT:Port-2.20

## Procedure

*   Test-Case 1

        *   Configure DUT to match on IPinIP protocol (protocol number 4) in the outer IPv4 header and punt it to 
            network-instance VRF-10. All other traffic should be punted to the Default VRF. These will be, native IPv4, 
            native IPv6 and IPv6inIP (protocol 41 in the outer IPv4 header) traffic with and without "198.18.0.1"
            as source.

        *   Start flows, Flow#1, Flow#3, Flow#6, Flow#8, Flow#9 and Flow#10 and validate packet forwarding. Drop of any 
            flows is considered as a failure.

*   Test-Case 2

        *   Configure DUT to match on IPinIP protocol (protocol number 4 in the outer IPv4 header) with specific outer 
            IPv4 source address as "198.18.0.1" and punt it to network-instance VRF-10. All other traffic should be 
            punted to the Default VRF. These will be, IPinIP w/o source as "198.18.0.1", native IPv4, native IPv6 and 
            IPv6inIP (protocol 41 in the outer IPv4 header) traffic with and without "198.18.0.1" as source.

        *   Start flows, Flow#2, Flow#3, Flow#6, Flow#8, Flow#9 and Flow#10 and validate packet forwarding. Drop of any 
            flows is considered as a failure.

*   Test-Case 3

        *   Configure DUT to match on IPv6inIP protocol (protocol number 41 in the outer IPv4 header) and punt it to 
            network-instance VRF-10. All other traffic should be punted to the Default VRF. These will be, native IPv4, 
            native IPv6 and IPinIP (protocol 4 in the outer IPv4 header) traffic with and without "198.18.0.1"
            as source.

        *   Start flows, Flow#2, Flow#4, Flow#5, Flow#7, Flow#9 and Flow#10 and validate packet forwarding. Drop of any 
            flows is considered as a failure.

* Test-Case 4
          
        *  Configure DUT to match on IPv6inIP protocol (protocol number 41 in the outer IPv4 header) with specific 
           outer IPv4 source address "198.18.0.1" and punt it to the network-instance VRF-10. 
           All other traffic should be punted to the Default VRF. These will be, IPv6inIP without source as 
           "198.18.0.1", native IPv4, native IPv6 and IPinIP (protocol 4 in the outer IPv4 header) traffic 
           with and without "198.18.0.1" as source.

        *   Start flows, Flow#2, Flow#4, Flow#6, Flow#7, Flow#9 and Flow#10 and validate packet forwarding. Drop of any 
            flows is considered as a failure.

## Flows

*   IPinIP

        *   Flow#1: IPinIP with outer source as not "198.18.0.1" and outer destination as ATE-DEST-IPv4-VLAN10
        *   Flow#2: IPinIP with outer source as not "198.18.0.1" and outer destination as ATE-DEST-IPv4-VLAN20
        *   Flow#3: IPinIP with outer source as "198.18.0.1" and outer destination as ATE-DEST-IPv4-VLAN10
        *   Flow#4: IPinIP with outer source as "198.18.0.1" and outer destination as ATE-DEST-IPv4-VLAN20

*   IPv6inIP

        *   Flow#5: IPv6inIP with outer source as not "198.18.0.1" and outer destination as ATE-DEST-IPv4-VLAN10
        *   Flow#6: IPv6inIP with outer source as not "198.18.0.1" and outer destination as ATE-DEST-IPv4-VLAN20
        *   Flow#7: IPv6inIP with outer source as "198.18.0.1" and outer destination as ATE-DEST-IPv4-VLAN10
        *   Flow#8: IPv6inIP with outer source as "198.18.0.1" and outer destination as ATE-DEST-IPv4-VLAN20

*   Native IPv4

        *   Flow#9: Native IPv4 flow with any source address and destination as ATE-DEST-IPv4-VLAN20

*   Native IPv6

        *   Flow#10: Native IPv6 flow with any source address and destination as ATE-DEST-IPv6-VLAN20

## OpenConfig Path and RPC Coverage
```yaml
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```
