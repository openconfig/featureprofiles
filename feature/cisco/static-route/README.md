# Telemetry Support for Recurse Leaf

Automation test to verify Telemetry support for Recurse leaf in Static Route configuration.
IPv4 and IPv6 Static Routes are created and verified for Telemetry support.

## Topology

* ATE <-----------> spitfire_d <-----------> spitfire_d <-----------> ATE
                              
## Procedure

* Connect ATE port-1 to DUT1 port-1, ATE port-2 to DUT2 port-1 and ATE port-3 to DUT2 port-2.
* Configure IPv4 and IPv6 interfaces on DUT1 and DUT2.
* Configure ISIS on DUT1 and DUT2 and establish IS-IS adjacency between DUT1 and DUT2.
* Configure BGP on DUT1 and DUT2 and establish BGP peering between DUT1 and DUT2.
* Configure IPv4 and IPv6 static routes on DUT1 and export them to DUT2.
* Configure IPv4 and IPv6 static routes on DUT2 and perform following tests.

## Tests

All CRUD operations are performed on IPv4 and IPv6 static with 5 sec timeout.

### TestIPv4StaticRouteRecurse

* config IPv4-Static-Route-With-Recurse-True-With-NextHop-Connected
* config IPv4-Static-Route-With-Recurse-True-With-NextHop-Static
* config IPv4-Static-Route-With-Recurse-True-With-NextHop-Unreachable
* config IPv4-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Connected
* config IPv4-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Static
* config IPv4-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Unreachable
* config IPv4-Static-Route-With-Recurse-False-With-NextHop-Connected
* config IPv4-Static-Route-With-Recurse-False-With-NextHop-Static
* config IPv4-Static-Route-With-Recurse-False-With-NextHop-Unreachable
* config IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Connected
* config IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Static
* config IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Unreachable
* config IPv4-Static-Route-With-Recurse-True-With-NextHop-Connected-With-Attributes
* config IPv4-Static-Route-With-Recurse-True-With-NextHop-Connected-With-Update-Attributes
* config IPv4-Static-Route-With-Recurse-True-With-NextHop-Connected-With-Delete-Attributes
* config IPv4-Static-Route-With-Recurse-True-With-NextHop-Static-With-Attributes
* config IPv4-Static-Route-With-Recurse-True-With-NextHop-Static-With-Update-Attributes
* config IPv4-Static-Route-With-Recurse-True-With-NextHop-Static-With-Delete-Attributes
* config IPv4-Static-Route-With-Recurse-True-With-NextHop-Unreachable-With-Attributes
* config IPv4-Static-Route-With-Recurse-True-With-NextHop-Unreachable-With-Update-Attributes
* config IPv4-Static-Route-With-Recurse-True-With-NextHop-Unreachable-With-Delete-Attributes
* config IPv4-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Connected-With-Attributes
* config IPv4-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Connected-With-Update-Attributes
* config IPv4-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Connected-With-Delete-Attributes
* config IPv4-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Static-With-Attributes
* config IPv4-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Static-With-Update-Attributes
* config IPv4-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Static-With-Delete-Attributes
* config IPv4-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Unreachable-With-Attributes
* config IPv4-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Unreachable-With-Update-Attributes
* config IPv4-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Unreachable-With-Delete-Attributes
* config IPv4-Static-Route-With-Recurse-False-With-NextHop-Connected-With-Attributes
* config IPv4-Static-Route-With-Recurse-False-With-NextHop-Connected-With-Update-Attributes
* config IPv4-Static-Route-With-Recurse-False-With-NextHop-Connected-With-Delete-Attributes
* config IPv4-Static-Route-With-Recurse-False-With-NextHop-Static-With-Attributes
* config IPv4-Static-Route-With-Recurse-False-With-NextHop-Static-With-Update-Attributes
* config IPv4-Static-Route-With-Recurse-False-With-NextHop-Static-With-Delete-Attributes
* config IPv4-Static-Route-With-Recurse-False-With-NextHop-Unreachable-With-Attributes
* config IPv4-Static-Route-With-Recurse-False-With-NextHop-Unreachable-With-Update-Attributes
* config IPv4-Static-Route-With-Recurse-False-With-NextHop-Unreachable-With-Delete-Attributes
* config IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Connected-With-Attributes
* config IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Connected-With-Update-Attributes
* config IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Connected-With-Delete-Attributes
* config IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Static-With-Attributes
* config IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Static-With-Update-Attributes
* config IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Static-With-Delete-Attributes
* config IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Unreachable-With-Attributes
* config IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Unreachable-With-Update-Attributes
* config IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Unreachable-With-Delete-Attributes
* config IPv4-Static-Route-With-Recurse-True-With-NextHop-Invalid
* config IPv4-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Invalid
* config IPv4-Static-Route-With-Recurse-False-With-NextHop-Invalid
* config IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Invalid
* config IPv4-Static-Route-With-Recurse-True-With-With-BFD 
* config IPv4-Static-Route-With-Recurse-True-With-Interface-With-NextHop-With-BFD
* config IPv4-Static-Route-With-Recurse-False-With-NextHop-With-BFD
* config IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-With-BFD
* config IPv4-Static-Route-No-Recurse-With-NextHop-Connected
* config IPv4-Static-Route-No-Recurse-With-NextHop-Static
* config IPv4-Static-Route-No-Recurse-With-NextHop-Unreachable
* config IPv4-Static-Route-No-Recurse-With-Interface-With-NextHop-Connected
* config IPv4-Static-Route-No-Recurse-With-Interface-With-NextHop-Static
* config IPv4-Static-Route-No-Recurse-With-Interface-With-NextHop-Unreachable

### TestIPv6StaticRouteRecurse

* config IPv6-Static-Route-With-Recurse-True-With-NextHop-Connected
* config IPv6-Static-Route-With-Recurse-True-With-NextHop-Static
* config IPv6-Static-Route-With-Recurse-True-With-NextHop-Unreachable
* config IPv6-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Connected
* config IPv6-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Static
* config IPv6-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Unreachable
* config IPv6-Static-Route-With-Recurse-False-With-NextHop-Connected
* config IPv6-Static-Route-With-Recurse-False-With-NextHop-Static
* config IPv6-Static-Route-With-Recurse-False-With-NextHop-Unreachable
* config IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Connected
* config IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Static
* config IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Unreachable
* config IPv6-Static-Route-With-Recurse-True-With-NextHop-Connected-With-Attributes
* config IPv6-Static-Route-With-Recurse-True-With-NextHop-Connected-With-Update-Attributes
* config IPv6-Static-Route-With-Recurse-True-With-NextHop-Connected-With-Delete-Attributes
* config IPv6-Static-Route-With-Recurse-True-With-NextHop-Static-With-Attributes
* config IPv6-Static-Route-With-Recurse-True-With-NextHop-Static-With-Update-Attributes
* config IPv6-Static-Route-With-Recurse-True-With-NextHop-Static-With-Delete-Attributes
* config IPv6-Static-Route-With-Recurse-True-With-NextHop-Unreachable-With-Attributes
* config IPv6-Static-Route-With-Recurse-True-With-NextHop-Unreachable-With-Update-Attributes
* config IPv6-Static-Route-With-Recurse-True-With-NextHop-Unreachable-With-Delete-Attributes
* config IPv6-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Connected-With-Attributes
* config IPv6-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Connected-With-Update-Attributes
* config IPv6-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Connected-With-Delete-Attributes
* config IPv6-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Static-With-Attributes
* config IPv6-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Static-With-Update-Attributes
* config IPv6-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Static-With-Delete-Attributes
* config IPv6-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Unreachable-With-Attributes
* config IPv6-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Unreachable-With-Update-Attributes
* config IPv6-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Unreachable-With-Delete-Attributes
* config IPv6-Static-Route-With-Recurse-False-With-NextHop-Connected-With-Attributes
* config IPv6-Static-Route-With-Recurse-False-With-NextHop-Connected-With-Update-Attributes
* config IPv6-Static-Route-With-Recurse-False-With-NextHop-Connected-With-Delete-Attributes
* config IPv6-Static-Route-With-Recurse-False-With-NextHop-Static-With-Attributes
* config IPv6-Static-Route-With-Recurse-False-With-NextHop-Static-With-Update-Attributes
* config IPv6-Static-Route-With-Recurse-False-With-NextHop-Static-With-Delete-Attributes
* config IPv6-Static-Route-With-Recurse-False-With-NextHop-Unreachable-With-Attributes
* config IPv6-Static-Route-With-Recurse-False-With-NextHop-Unreachable-With-Update-Attributes
* config IPv6-Static-Route-With-Recurse-False-With-NextHop-Unreachable-With-Delete-Attributes
* config IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Connected-With-Attributes
* config IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Connected-With-Update-Attributes
* config IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Connected-With-Delete-Attributes
* config IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Static-With-Attributes
* config IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Static-With-Update-Attributes
* config IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Static-With-Delete-Attributes
* config IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Unreachable-With-Attributes
* config IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Unreachable-With-Update-Attributes
* config IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Unreachable-With-Delete-Attributes
* config IPv6-Static-Route-With-Recurse-True-With-NextHop-Invalid
* config IPv6-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Invalid
* config IPv6-Static-Route-With-Recurse-False-With-NextHop-Invalid
* config IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Invalid
* config IPv6-Static-Route-With-Recurse-True-With-With-BFD 
* config IPv6-Static-Route-With-Recurse-True-With-Interface-With-NextHop-With-BFD
* config IPv6-Static-Route-With-Recurse-False-With-NextHop-With-BFD
* config IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-With-BFD
* config IPv6-Static-Route-No-Recurse-With-NextHop-Connected
* config IPv6-Static-Route-No-Recurse-With-NextHop-Static
* config IPv6-Static-Route-No-Recurse-With-NextHop-Unreachable
* config IPv6-Static-Route-No-Recurse-With-Interface-With-NextHop-Connected
* config IPv6-Static-Route-No-Recurse-With-Interface-With-NextHop-Static
* config IPv6-Static-Route-No-Recurse-With-Interface-With-NextHop-Unreachable

## Triggers

After each triggers mentioned below, tests mentioned above are validated again.
* IPv4StaticProcessRestart
* IPv6StaticProcessRestart
* RIBMgrProcessRestart
* EmsdProcessRestart
* ReloadDUT
* RPFO
* FlapInterfaces
* DelMemberPort
* AddMemberPort

## Scale Tests

* Create 250 static routes with NH Dynamic
* Create 250 static routes with NH Static
* Create 250 static routes with NH Unreachable
* Create 250 static routes with NH Directly connected

Do CRUD operations on 1000 static routes by updating attributes and deleting them.
Recurse leaf is set to True for all the above operations.
Sampling of static routes is done at 10 sec interval.

## OC Paths 

```
openconfig-network-instance/network-instances/network-instance[name]/protocols/protocol[STATIC DEFAULT]/static-routes/static[prefix]/next-hops/next-hop[index]/config/recurse 
openconfig-network-instance/network-instances/network-instance[name]/protocols/protocol[STATIC DEFAULT]/static-routes/static[prefix]/next-hops/next-hop[index]/state/recurse 
```

## SFS

https://docs.cisco.com/alfresco/service/url?docnum=EDCS-25538180
