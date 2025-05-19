# Telemetry Support On sub-interface neighbor state leaves

Automation Test to verify Telemetry Support On sub-interface neighbor state leaves.

Tests are run against the paths mentioned below for ARP and ND modules.

## Topology

* spitfire_d <-----------> spitfire_d

## Tests
* Create IPv4/IPv6 Phy Interface
* Create IPv4/IPv6 Phy Sub Interface
* Create IPv4/IPv6 Bundle Interface
* Create IPv4/IPv6 Bundle Sub Interface

### TestIPv4NeighborsPath
* Verify ipv4/neighbors leaf returns expected value in telemetry response.
* Verify CRUD operations return expected value in telemetry response.
* Dynamic and Static neighbors are verified.

### TestIPv4ProxyARPPath
* Verify ipv4/proxy-arp leaf returns expected value in telemetry response.
* Verify CRUD operations return expected value in telemetry response.

### Triggers
After each triggers mentioned below, tests mentioned above are validated again.
* LCReload
* FlapInterfaces
* DelMemberPort
* AddMemberPort
* ProcessRestart
* ReloadRouter
* RPFO

### TestIPv6NeighborsPath
* Verify ipv6/neighbors leaf returns expected value in telemetry response.
* Verify CRUD operations return expected value in telemetry response.
* Dynamic and Static neighbors are verified.

### TestIPv6NDRouterAdvPath
* Verify ipv6/router-advertisement leaf returns expected value in telemetry response.
* Following Router Adv attributes are verified.
  * RAInterval
  * RALifetime
  * RAOtherConfig
  * RASuppress
* Verify CRUD operations return expected value in telemetry response.

### TestIPv6NDPrefixPath
* Verify ipv6/router-advertisement/prefixes leaf returns expected value in telemetry response.
* Verify CRUD operations return expected value in telemetry response.

### TestIPv6NDDadPath
* Verify ipv6/dad leaf returns expected value in telemetry response.
* Verify CRUD operations return expected value in telemetry response.

### Triggers
After each triggers mentioned below, tests mentioned above are validated again.
* LCReload
* FlapInterfaces
* DelMemberPort
* AddMemberPort
* ProcessRestart
* ReloadRouter
* RPFO

## Scale Tests
* Create 9 IPv4/IPv6 Phy Interface
* Create 9x20 IPv4/IPv6 Phy Sub Interface
* Create 9 IPv4/IPv6 Bundle Interface
* Create 9x20 IPv4/IPv6 Bundle Sub Interface

### TestIPv4Scale
* Verify ipv4/ leaf streams data in 30 secs cadence for all interfaces configured.
* Validate data returned for each interface has expected values

### Triggers
After each triggers mentioned below, tests mentioned above are validated again.
* LCReload
* FlapInterfaces
* DelMemberPort
* AddMemberPort
* ProcessRestart
* ReloadRouter
* RPFO

### TestIPv6cale
* Verify ipv6/ leaf streams data in 30 secs cadence for all interfaces configured.
* Validate data returned for each interface has expected values.

### Triggers
After each triggers mentioned below, tests mentioned above are validated again.
* LCReload
* FlapInterfaces
* DelMemberPort
* AddMemberPort
* ProcessRestart
* ReloadRouter
* RPFO

## OC Paths
```
/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv4/neighbors 
/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv4/neighbors/neighbor 
/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv4/neighbors/neighbor/config 
/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv4/neighbors/neighbor/config/ip 
/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv4/neighbors/neighbor/config/link-layer-address 
/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv4/neighbors/neighbor/ip 
/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv4/neighbors/neighbor/state
/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv4/neighbors/neighbor/state/ip
/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv4/neighbors/neighbor/state/link-layer-address 
/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv4/neighbors/neighbor/state/origin
/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv4/proxy-arp/state/mode

/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv6/neighbors
/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv6/neighbors/neighbor
/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv6/neighbors/neighbor/config
/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv6/neighbors/neighbor/config/ip
/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv6/neighbors/neighbor/config/link-layer-address
/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv6/neighbors/neighbor/ip
/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv6/neighbors/neighbor/state
/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv6/neighbors/neighbor/state/ip
/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv6/neighbors/neighbor/state/is-router
/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv6/neighbors/neighbor/state/link-layer-address
/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv6/neighbors/neighbor/state/neighbor-state
/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv6/neighbors/neighbor/state/origin
/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv6/router-advertisement
/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv6/router-advertisement/prefixes
/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/openconfig-if-ip:ipv6/dad
```

## SFS
https://docs.cisco.com/share/proxy/alfresco/url?docnum=EDCS-25383763