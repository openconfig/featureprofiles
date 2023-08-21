
 ##  TUN-1.7: Tunnel Interfaces disable and enable - Interface Based GRE Tunnel
 ## Summary
   Tunnel Interfaces disable and enable - Interface Based GRE Tunnel
 ## Procedure

 *   Configure DUT with 8 GRE encapsulation tunnels and configure another router with 8 GRE Decapsulation tunnel interfaces.(Tunnel-1.1)
 *   Configure 4 tunnel as IPv4 tunnel source and destination address (Tunnel-1.3), 4 as IPv6 tunnel source and destination address (Tunnel-1.4)
 *   Configure static router to point original destination to 8 tunnel interface to do overlay loadbalance
 *   Keep topology tunnel destination will be reachable via 2 underlay interface on both routers
 *   Disable and enable  the numbers of Tunnel interface being used
 *   Disabling Tunnel interfaces:
     Disable 5 tunnel interfaces
      *  One by one
      *  All 5 at once
 *   Static route using the disable interface should become invalid and should not be used in forwarding traffic
 *   Incoming traffic on DUT-PORT1 should be load balanced to available Tunnel interfaces for encapsulation
 *   Incoming traffic flow should be equally distributed for Encapsulation(ECMP)
 *   No traffic loss expected
 *   Enable the tunnel interfaces:
 *   Enable or bring up the same interfaces which were disabled in previous step
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

 ## Config Parameter coverage

 *   openconfig-interfaces:interfaces/interface[name='fti0']
 *   openconfig-interfaces:interfaces/interface[name='fti0']/tunnel/
 *   openconfig-interfaces:interfaces/interface[name='fti0']/tunnel/gre

 ## Telemetry Parameter coverage


 *   /interfaces/interface[name='fti0']/state/counters/in-pkts
 *   /interfaces/interface[name='fti0']/state/counters/in-octets
 *   /interfaces/interface[name='fti0']/state/counters/out-pkts
 *   /interfaces/interface[name='fti0']/state/counters/out-octets

	## Topology:

 *   otg:port1 <--> port1:dut1:port3 <--> port3:dut2:port5<--->otg:port5
 *   otg:port2 <--> port2:dut1:port4 <--> port4:dut2:port6<--->otg:port6 
