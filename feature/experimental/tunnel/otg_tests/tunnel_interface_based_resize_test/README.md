# TUN-1.6: Tunnel End Point Resize - Interface Based GRE Tunnel

## Summary

*    Tunnel End Point Resize - Interface Based GRE Tunnel

## Procedure

*   Apply the config mentioned in Tunnel-1.1
*   Send the traffic as mentioned in Tunnel-1.3 and Tunnel-1.4 with TP-1.1 and TP-1.2
*   Modify the numbers of Tunnel interface being used 
*   Reduce number of Tunnel interfaces(e.g. From 16 to 12):
        - Incoming traffic on DUT-PORT1 should be load balanced to available Tunnel interfaces for encapsulation
        - Incoming traffic flow should be equally distributed for Encapsulation(ECMP)
*   If the static routes are used to forward traffic to tunnel, please disable or delete the static route in this test to simulate the reduction in available paths
*   Increase number of Tunnel interfaces(e.g. From 16 to 20): 
*   Incoming traffic on DUT-PORT1 should start using additional Tunnel Interfaces  for encapsulation
*   Incoming traffic flow should be equally distributed for Encapsulation(ECMP)
*   Increasing and decreasing the number of tunnel interfaces and related static route shouldn’t case traffic drops
*   Verify the next hop counters for packet being diverted or sent for encapsulation
*   Verify the tunnel interfaces counters to confirm the traffic encapsulation
*   Verify the tunnel interfaces traffic/flow for equal distribution for optimal load balancing
*   After decapsulation, traffic should be load balanced/hash to all available L3 ECMP or LAG or combination of both features
*   Verify the tunnel interfaces counters to confirm the traffic decapsulation

*   Validate system for: 
*   Health-1.1 
*   No feature related error or drop counters incrementing, 
*   discussion with vendors required to highlight additional fields to monitor based on implementation and architecture

## Config Parameter coverage

*   openconfig-interfaces:interfaces/interface 
*   gre/ 
*   gre/decap-group/ 
*   gre/dest/ 
*   gre/dest/address/ 
*   gre/dest/address/ipv4/ 
*   gre/dest/address/ipv6/ 
*   gre/dest/nexthop-group/ 
*   gre/source/ 
*   gre/source/address/ 
*   gre/source/address/ipv4/ 
*   gre/source/address/ipv6/ 
*   gre/source/interface/ 

## Telemetry Parameter coverage

*   state/counters/in-pkts 
*   state/counters/in-octets 
*   state/counters/out-pkts 
*   state/counters/out-octets 
*   state/counters/in-error-pkts 
*   state/counters/in-forwarded-pkts 
*   state/counters/in-forwarded-octets 
*   state/counters/in-discarded-pkts 
*   state/counters/out-error-pkts 
*   state/counters/out-forwarded-pkts 
*   state/counters/out-forwarded-octets 
*   state/counters/out-discarded-pkts 

## Topology:
*   otg:port1 <--> port1:dut1:port3 <--> port3:dut2:port5<--->otg:port5
*   otg:port2 <--> port2:dut1:port4 <--> port4:dut2:port6<--->otg:port6
