# MGT-1: Management HA solution test

## Summary

- Test management HA

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed

## Procedure

### Applying configuration

For each section of configuration below, prepare a gnmi.SetBatch with all the configuration items appended to one SetBatch. Then apply the configuration to the DUT in one gnmi.Set using the `replace` option

### Initial Setup:

*   Connect DUT port-1, 2, 3 and 4 to ATE port-1, 2, 3 and 4
*   Create VRF "mgmt" on DUT
    *   /network-instances/network-instance[name=mgmt]/vrfs/vrf[name=mgmt]/config/name = mgmt
    *   /network-instances/network-instance[name=mgmt]/vrfs/vrf[name=mgmt]/config/route-distinguisher = 64512:100
*   Create an IPv6 networks ```ateNet``` attached to ATE port-1, 2, 3 and 4
*   Create a loopback interface "lo0" on DUT and assign it an IPv6 address
    *   /interfaces/interface[name=lo1]/config/name = lo1
    *   /interfaces/interface[name=lo1]/config/type = softwareLoopback
    *   /interfaces/interface[name=lo1]/subinterfaces/subinterface[index=0]/ipv6/addresses/address/config/ip
    *   /interfaces/interface[name=lo1]/subinterfaces/subinterface[index=0]/ipv6/addresses/address/config/prefix-length
*   Configure the loopback interface to participate in the VRF "mgmt"
    *   /interfaces/interface[name=lo1]/subinterfaces/subinterface[index=0]/ipv6/vrf/name = mgmt

##### Configure linecard ports to ATE using BGP

*   Configure IPv6 addresses on DUT and ATE ports 1 and 2. Configure them to participate in the VRF "mgmt"
    *   /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip
    *   /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length
    *   /interfaces/interface/subinterfaces/subinterface/ipv6/vrf/name = mgmt
*   Configure IPv6 eBGP between DUT Port-1 <--> ATE Port-1 and DUT Port-2 <--> ATE Port-2 in VRF "mgmt"
    *   /network-instances/network-instance[name=mgmt]/protocols/protocol[identifier=BGP, name=BGP]/global/config/as = 64512
    *   /network-instances/network-instance[name=mgmt]/protocols/protocol[identifier=BGP, name=BGP]/global/config/router-id = <router_id>
    *   /network-instances/network-instance[name=mgmt]/protocols/protocol[identifier=BGP, name=BGP]/neighbor[ip=1<neighbor_ip>]/config/peer-as = 64511
    * /network-instances/network-instance[name=mgmt]/protocols/protocol[identifier=BGP, name=BGP]/neighbor[ip=<neigh_ip>]/afi-safis/afi-safi[afi-safi-name=ipv6-unicast]/config/route-distinguisher = 64512:100 
*   Set default import and export policy to ```ACCEPT_ROUTE``` for the eBGP sessions
    *   /network-instances/network-instance[name=mgmt]/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance[name=mgmt]/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Advertise a default route from ATE to DUT throught both the BGP sessions
*   Advertise the loopback interface from DUT to ATE through both the BGP sessions
    *   /network-instances/network-instance[name=default]/protocols/protocol[identifier=BGP, name=BGP]/neighbor[ip=<neigh_ip>]/afi-safis/afi-safi[afi-safi-name=ipv6-unicast]/config/prefix = <lo0 IPv6 Address>

##### Configure linecard ports to ATE using VRRP

*   Configure IPv6 addresses on DUT and ATE ports 3 and 4 from one IPv6 subnet ```vrrp_subnet```
    *   /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip
    *   /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length
*   Configure ATE Port-3 and Port-4 to run VRRP
    *   Configure a VRRP group-1 with a virtual IP address from the ```vrrp_subnet```, priority 100 and preempt enabled on ATE Port-3
    *   Configure a VRRP group-1 with the same virtual IP address, priority 120 and preempt enabled on ATE Port-4
*   Configure DUT Port-3 and Port-4 to participate in the VRF "mgmt"
    *   /interfaces/interface/subinterfaces/subinterface/ipv6/vrf/name = mgmt
*   Configure a static route on ATE devices for port-3 destined to the DUT loopback with next-hop address of DUT port-3 with administrative-distance or preference of 220
*   Configure a static route on ATE devices for port-4 destined to the DUT loopback with next-hop address of DUT port-4 with administrative-distance or preference of 220
*   Configure a default static route on DUT in VRF "mgmt" pointing towards the VRRP virtual IP address with Administrative Distance or Preference of 220
    *   /network-instances/network-instance[name=mgmt]/protocols/protocol[identifier=STATIC, name=static]/static-routes/static/config/prefix = 0.0.0.0/0
	*   /network-instances/network-instance[name=mgmt]/protocols/protocol[identifier=STATIC, name=static]/static-routes/static/config/next-hops/next-hop/index = 1
	*   /network-instances/network-instance[name=mgmt]/protocols/protocol[identifier=STATIC, name=static]/static-routes/static/config/next-hops/next-hop[index=1]/config/next-hop = <VRRP Virtual IP>
	*   /network-instances/network-instance[name=mgmt]/protocols/protocol[identifier=STATIC, name=static]/static-routes/static/config/next-hops/next-hop[index=1]/config/admin-distance = 220

### MGT-1.1 [TODO: https://github.com/openconfig/featureprofiles/issues/]
#### Testing reachability to the DUT loopback with no failures in the network
---

*   Generate ICMP echo (ping) sourced from the ```ateNet``` network destined towards the DUT loopback0 IPv6 address
*   Validate ICMP echo-reply is received by the ATE on Port-1 or Port-2

### MGT-1.2 [TODO: https://github.com/openconfig/featureprofiles/issues/]
#### Testing BGP redundancy
---

*   Shutdown BGP session on Port-1
*   Generate ICMP echo (ping) sourced from the ```ateNet``` network destined towards the DUT loopback0 IPv6 address
*   Validate ICMP echo-reply is received by the ATE on Port-2
*   Bring up BGP session on Port-1 and shutdowm BGP on Port-2
*   Generate ICMP echo (ping) sourced from the ```ateNet``` network destined towards the DUT loopback0 IPv6 address
*   Validate ICMP echo-reply is received by the ATE on Port-1

### MGT-1.3 [TODO: https://github.com/openconfig/featureprofiles/issues/]
#### Testing failover between BGP and Static route
---

*   Shutdown BGP session on Port-1 and Port-2
*   Generate ICMP echo (ping) sourced from the ```ateNet``` network destined towards the DUT loopback0 IPv6 address
*   Validate ICMP echo-reply is received by the ATE on either Port-4 (Port-4 should be active due to higher priority)
*   Bring up BGP session on Port-1 and Port-2
*   Generate ICMP echo (ping) sourced from the ```ateNet``` network destined towards the DUT loopback0 IPv6 address
*   Validate ICMP echo-reply is received by the ATE on Port-1 or Port-2

### MGT-1.4 [TODO: https://github.com/openconfig/featureprofiles/issues/]
#### Testing VRRP redundancy
---

*   Shutdown BGP session on Port-1 and Port-2
*   Shutdown Port-4 (Active due to higher priority)
*   Generate ICMP echo (ping) sourced from the ```ateNet``` network destined towards the DUT loopback0 IPv6 address
*   Validate ICMP echo-reply is received by the ATE on Port-3
*   Bring up Port-4 
*   Generate ICMP echo (ping) sourced from the ```ateNet``` network destined towards the DUT loopback0 IPv6 address
*   Validate ICMP echo-reply is received by the ATE on Port-4 (It has higher priority with preempt)

## Config parameter coverage

*   /interfaces/interface/config/name
*   /interfaces/interface/config/type

*   /network-instances/network-instance/vrfs/vrf/config/name
*   /network-instances/network-instance/vrfs/vrf/config/route-distinguisher

*   /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip
*   /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length
*   /interfaces/interface/subinterfaces/subinterface/ipv6/vrf/name

*   /network-instances/network-instance/protocols/protocol/global/config/as
*   /network-instances/network-instance/protocols/protocol/global/config/router-id
*   /network-instances/network-instance/protocols/protocol/neighbor/config/peer-as
*   /network-instances/network-instance/protocols/protocol/neighbor/afi-safis/afi-safi/config/route-distinguisher
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   /network-instances/network-instance/protocols/protocol/neighbor/afi-safis/afi-safi/config/prefix

*   /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix
*   /network-instances/network-instance/protocols/protocol/static-routes/static/config/next-hops/next-hop/index
*   /network-instances/network-instance/protocols/protocol/static-routes/static/config/next-hops/next-hop/config/next-hop
*   /network-instances/network-instance/protocols/protocol/static-routes/static/config/next-hops/next-hop/config/admin-distance


## Telemetry parameter coverage

*   NA

## Protocol/RPC Parameter Coverage

* gNMI
  * Set (replace)

## Required DUT platform

* FFF
