
 ##  TUN-1.7: Tunnel Interfaces disable and enable - Interface Based GRE Tunnel
 ## Summary
 *   Tunnel Interfaces disable and enable - Interface Based GRE Tunnel
 ## Procedure
 
 *   Apply the config mentioned in Tunnel-1.1 
 *   Send the traffic as mentioned in Tunnel-1.3 and Tunnel-1.4 with TP-1.1 and TP-1.2 
 *   Disable and enable  the numbers of Tunnel interface being used  
 *   Disabling Tunnel interfaces: 
     *  Disable 5 tunnel interfaces 
        *  One by one  
        *  All 5 at once 
 *   Static route using the disable interface should become invalid and should not be used in forwarding traffic 
 *   Incoming traffic on DUT-PORT1 should be load balanced to available Tunnel interfaces for encapsulation 
 *   Incoming traffic flow should be equally distributed for Encapsulation(ECMP) 
 *   No traffic loss expected 
 *   Enable the tunnel interfaces:  
     *  Enable or bring up the same interfaces which were disabled in previous step 
        *  One by one 
        *  All 5 at once 
 *   Incoming traffic on DUT-PORT1 should start using additional Tunnel Interfaces  for encapsulation 
 *   Incoming traffic flow should be equally distributed for Encapsulation(ECMP) 
 *   No traffic loss expected 
 *   Disabling and enabling the number of tunnel interfaces and related static route shouldn’t case traffic drops 
 *   Verify the Next hop counters for packet being diverted or sent for encapsulation 
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

