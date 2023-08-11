


 ## TUN-1.6: Tunnel End Point Resize - Interface Based GRE Tunnel

 ## Summary

 *    Tunnel End Point Resize - Interface Based GRE Tunnel

 ## Procedure

 *   Configure DUT with 16 GRE encapsulation tunnels and configure another router with 12 GRE Decapsulation tunnel interfaces.
 *   Configure 8 tunnel as IPv4 tunnel source and destination address , 8 as IPv6 tunnel source and destination address
 *   Configure static router to point original destination to 16 tunnel interface to do overlay loadbalance
 *   Keep topology tunnel destination will be reachable via two underlay interface on both routers
 *   Send IPv4 flow and IPv6 flow and validate tunnel load balance and physical interface load balance
 *   resize the tunnel fro 16 to 12 and verify the load balance and traffic drop by removing static route to point tunnel interface.
 *   Again resize the tunnel fro 12 to 16 and verify the load balance and traffic drop

 ## Config Parameter coverage

 *   openconfig-interfaces:interfaces/interface
 *   openconfig-interfaces:interfaces/interface/tunnel/
 *   openconfig-interfaces:interfaces/interface/tunnel/gre

 ## Telemetry Parameter coverage

 *   /interfaces/interface[name='fti0']/state/counters/operstatus
 *   /interfaces/interface[name='fti0']/state/counters/in-pkts 
 *   /interfaces/interface[name='fti0']/state/counters/in-octets 
 *   /interfaces/interface[name='fti0']/state/counters/out-pkts 
 *   /interfaces/interface[name='fti0']/state/counters/out-octets 


 ## Topology:
 *   otg:port1 <--> port1:dut1:port3 <--> port3:dut2:port5<--->otg:port5
 *   otg:port2 <--> port2:dut1:port4 <--> port4:dut2:port6<--->otg:port6
