


 ## TUN-1.6: Tunnel End Point Resize - Interface Based GRE Tunnel

 ## Summary

 *   Validate of interface based GRE tunnel end point reduction and increment test with load balaning.

 ## Procedure

 *   Configure DUT with 8 GRE encapsulation tunnels and configure another router with 8 GRE Decapsulation tunnel interfaces.
 *   Configure 4 tunnel as IPv4 tunnel source and destination address , 4 as IPv6 tunnel source and destination address
 *   Configure static router to point original destination to 8 tunnel interface to do overlay loadbalance
 *   Keep topology tunnel destination will be reachable via 2 underlay interface on both routers
 *   Send IPv4 flow and IPv6 flow and validate tunnel load balance and physical interface load balance
 *   resize the tunnel fro 8 to 4 and verify the load balance and traffic drop by removing static route to point tunnel interface.
 *   Again resize the tunnel fro 4 to 8 and verify the load balance and traffic drop

 ## Config Parameter coverage

 *   openconfig-interfaces:interfaces/interface[name='fti0']
 *   openconfig-interfaces:interfaces/interface[name='fti0']/tunnel/
 *   openconfig-interfaces:interfaces/interface[name='fti0']/tunnel/gre

 ## Telemetry Parameter coverage

 *   /interfaces/interface[name='fti0']/state/counters/operstatus
 *   /interfaces/interface[name='fti0']/state/counters/in-pkts 
 *   /interfaces/interface[name='fti0']/state/counters/in-octets 
 *   /interfaces/interface[name='fti0']/state/counters/out-pkts 
 *   /interfaces/interface[name='fti0']/state/counters/out-octets 


 ## Topology:
 *   otg:port1 <--> port1:dut1:port3 <--> port3:dut2:port5<--->otg:port5
 *   otg:port2 <--> port2:dut1:port4 <--> port4:dut2:port6<--->otg:port6
