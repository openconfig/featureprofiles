# MGT-1: Management HA solution test

## Summary

- Test management HA

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed

## Procedure

### Applying configuration

For each section of configuration below, prepare a gnmi.SetBatch with all the configuration items appended to one SetBatch. Then apply the configuration to the DUT in one gnmi.Set
using the `replace` option

### Initial Setup:

*   Connect DUT port-1, 2 and 3 to ATE port-1, 2 and 3
*   Create VRF "mgmt" on DUT
    *   /network-instances/network-instance[name=mgmt]/config/name = mgmt
    *   /network-instances/network-instance[name=mgmt]/config/route-distinguisher = 64512:100
*   Create an IPv6 networks ```ateNet``` attached to ATE port-1, 2 and 3
*   Create a loopback interface "lo1" on DUT and assign it an IPv6 address
    *   /interfaces/interface[name=lo1]/config/name = lo1
    *   /interfaces/interface[name=lo1]/config/type = softwareLoopback
    *   /interfaces/interface[name=lo1]/subinterfaces/subinterface[index=0]/ipv6/addresses/address/config/ip
    *   /interfaces/interface[name=lo1]/subinterfaces/subinterface[index=0]/ipv6/addresses/address/config/prefix-length
*   Configure the loopback interface to participate in the VRF "mgmt"
    *   /network-instances/network-instance[name=mgmt]/interfaces/interface[name=lo1]/config/interface = lo1

##### Configure linecard ports to ATE using BGP

*   Configure IPv6 addresses on DUT and ATE ports 1 and 2. Configure them to participate in the VRF "mgmt"
    *   /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip
    *   /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length
    *   /network-instances/network-instance[name=mgmt]/interfaces/interface/config/interface
*   Configure IPv6 eBGP between DUT Port-1 <--> ATE Port-1 and DUT Port-2 <--> ATE Port-2 in VRF "mgmt"
    *   /network-instances/network-instance[name=mgmt]/protocols/protocol[identifier=BGP, name=BGP]/global/config/as = 64512
    *   /network-instances/network-instance[name=mgmt]/protocols/protocol[identifier=BGP, name=BGP]/global/config/router-id = <router_id>
    *   /network-instances/network-instance[name=mgmt]/protocols/protocol[identifier=BGP, name=BGP]/neighbor/config/peer-as = 64511
*   Set default import and export policy to ```ACCEPT_ROUTE``` for the eBGP sessions
    *   /network-instances/network-instance[name=mgmt]/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance[name=mgmt]/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Advertise a default route from ATE to DUT throught both the BGP sessions
*   Redistribute the loopback interface from DUT to ATE through both the BGP sessions
    ##### Configure redistribution
    *   Set address-family to ```IPV6```  
        *   /network-instances/network-instance/table-connections/table-connection/config/address-family
    *   Configure source protocol to ```CONNECTED```
        *   /network-instances/network-instance/table-connections/table-connection/config/src-protocol
    *   Configure destination protocol to ```BGP```
        *   /network-instances/network-instance/table-connections/table-connection/config/dst-protocol
    *   Configure default export policy to ```ACCEPT_ROUTE```
        *   /network-instances/network-instance/table-connections/table-connection/config/default-export-policy
    *   Disable metric propogation by setting it to ```true```
        *   /network-instances/network-instance/table-connections/table-connection/config/disable-metric-propagation
    ##### Configure redistribution
    *   Configure an IPv6 route-policy definition with the name ```route-policy```
        *   /routing-policy/policy-definitions/policy-definition/config/name
    *   For routing-policy ```route-policy``` configure a statement with the name ```statement```
        *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
    *   For routing-policy ```route-policy``` statement ```statement``` set policy-result as ```ACCEPT_ROUTE```
        *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
    ##### Configure a prefix-set for route-filtering/matching
    *   Configure a prefix-set with the name ```prefix-set``` and mode ```IPV6```
        *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/name
        *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode
    *   For prefix-set ```prefix-set``` set the ip-prefix to ```loopback0 IPv6 address/mask``` and masklength to ```exact```
        *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
        *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range
    ##### Attach the prefix-set to route-policy
    *   For routing-policy ```route-policy``` statement ```statement``` set match options to ```ANY```
        *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options
    *   For routing-policy ```route-policy``` statement ```statement``` set prefix set to ```prefix-set```
        *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set
    ##### Attach the route-policy to the redistribution export-policy
    *   Apply routing policy ```route-policy``` for redistribution to BGP
        *   /network-instances/network-instance/table-connections/table-connection/config/export-policy

##### Configure linecard port to ATE

*   Configure IPv6 addresses on DUT Port-3 and ATE Port-3
    *   /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip
    *   /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length
*   Configure DUT Port-3 to participate in the VRF "mgmt"
    *   /network-instances/network-instance[name=mgmt]/interfaces/interface/config/interface
*   Configure a static route on ATE device of Port-3 destined to the DUT loopback with next-hop address of DUT Port-3 with administrative-distance or preference of 220
*   Configure a IPv6 default static route on DUT in VRF "mgmt" pointing towards the IPv6 address of ATE Port-3 with Administrative Distance or Preference of 220
    *   /network-instances/network-instance[name=mgmt]/protocols/protocol[identifier=STATIC, name=static]/static-routes/static/config/prefix = ::/0
	*   /network-instances/network-instance[name=mgmt]/protocols/protocol[identifier=STATIC, name=static]/static-routes/static/next-hops/next-hop/index = 1
	*   /network-instances/network-instance[name=mgmt]/protocols/protocol[identifier=STATIC, name=static]/static-routes/static/next-hops/next-hop[index=1]/config/next-hop = ATE Port-3 IP
	*   /network-instances/network-instance[name=mgmt]/protocols/protocol[identifier=STATIC, name=static]/static-routes/static/next-hops/next-hop/config/preference    = 220

### MGT-1.1 [TODO: https://github.com/openconfig/featureprofiles/issues/2762]
#### Testing reachability to the DUT loopback with no failures in the network
---

*   Generate ICMP echo (ping) sourced from the ```ateNet``` network destined towards the DUT loopback1 IPv6 address
*   Validate ICMP echo-reply is received by the ATE on Port-1 or Port-2

### MGT-1.2 [TODO: https://github.com/openconfig/featureprofiles/issues/2762]
#### Testing BGP redundancy
---

*   Shutdown BGP session on Port-1
*   Generate ICMP echo (ping) sourced from the ```ateNet``` network destined towards the DUT loopback1 IPv6 address
*   Validate ICMP echo-reply is received by the ATE on Port-2
*   Bring up BGP session on Port-1 and shutdowm BGP on Port-2
*   Generate ICMP echo (ping) sourced from the ```ateNet``` network destined towards the DUT loopback1 IPv6 address
*   Validate ICMP echo-reply is received by the ATE on Port-1

### MGT-1.3 [TODO: https://github.com/openconfig/featureprofiles/issues/2762]
#### Testing failover between BGP and Static route
---

*   Shutdown BGP session on Port-1 and Port-2
*   Generate ICMP echo (ping) sourced from the ```ateNet``` network destined towards the DUT loopback1 IPv6 address
*   Validate ICMP echo-reply is received by the ATE on Port-3
*   Bring up BGP session on Port-1 and Port-2
*   Generate ICMP echo (ping) sourced from the ```ateNet``` network destined towards the DUT loopback1 IPv6 address
*   Validate ICMP echo-reply is received by the ATE on Port-1 or Port-2

## Config parameter coverage

*   /network-instances/network-instance/config/name
*   /network-instances/network-instance/config/route-distinguisher
*   /network-instances/network-instance/interfaces/interface/config/interface
*   /interfaces/interface/config/name
*   /interfaces/interface/config/type
*   /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip
*   /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length
*   /network-instances/network-instance/protocols/protocol/global/config/as
*   /network-instances/network-instance/protocols/protocol/global/config/router-id
*   /network-instances/network-instance/protocols/protocol/neighbor/config/peer-as
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   /network-instances/network-instance/table-connections/table-connection/config/address-family
*   /network-instances/network-instance/table-connections/table-connection/config/src-protocol
*   /network-instances/network-instance/table-connections/table-connection/config/dst-protocol
*   /network-instances/network-instance/table-connections/table-connection/config/default-export-policy
*   /network-instances/network-instance/table-connections/table-connection/config/disable-metric-propagation
*   /routing-policy/policy-definitions/policy-definition/config/name
*   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
*   /routing-policy/defined-sets/prefix-sets/prefix-set/config/name
*   /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode
*   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
*   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range
*   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options
*   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set
*   /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix
*   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/index
*   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop
*   /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/preference

## Telemetry parameter coverage

*   NA

## Protocol/RPC Parameter Coverage

* gNMI
  * Set (replace)

## Required DUT platform

* FFF
