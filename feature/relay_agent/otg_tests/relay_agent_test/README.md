# RELAY-1.1: DHCP Relay functionality

## Summary

This is to validate the DHCP relay functionality on a DUT.  The test validates the following actions -

* DUT receives the IPv4/IPv6 DHCP discovery message over an individual or a LAG port and it will forward the request to the DHCP helper address.
* DUT forwards DHCP exchange messages between the DHCP Client and DHCP server.
* The DHCP client receives a DHCP address.

## Testbed Type

* [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)
  
## Procedure

### Test environment setup

```mermaid
graph LR; 
A[ATE:Port1] --(Vlan 10)-->B[Port1:DUT:Port3];B --Egress-->C[Port3:ATE];
```

```
                                                    ---------
                                                    |       |
    [ ATE:Port1, ATE:Port2 ] ==== LAG  (VLAN 10)=== |  DUT  |----Egress---[ ATE:Port3 ]
                                                    |       |
                                                    |       |
                                                    ---------
```

* Connect ports DUT:Ports[1-3] to ports ATE:Ports[1-3]
* Simulate a scenario of having a DHCP server is behind ATE:Port[3] 
* DUT:Port[3] IPv4 address = 192.0.2.0/31
* DUT:Port[3] IPv6 address = 2001:db8:a::0/127
  
### Configuration

* Configure VLAN 10 on DUT.
    * Have DUT:Port[1] and DUT:Port[2] be a part of vlan 10
    * VLAN10 interface IPv4 address: 192.0.2.33/27
    * VLAN10 interface IPv6 address: 2001:db8:a:1::1/64
    * Configure IPv4 and IPv6 helper address under VLAN10 interface.
        * IPv4 helper address - 192.0.2.254
        * IPv6 dhcp relay destination address : 2001:db8:a:2::1
* Configure IPv4 default route on the DUT pointing to ATE:Port[3] IPv4 address.
* Configure IPv6 default route on the  DUT pointing to ATE:Port[3] IPv6 address.


### RELAY-1.1.1 DHCP request on an individual port

* Step 1 - Have ATE:Port[1] as an individual port and act as a DHCP client.
* Step 2 - Send IPv4 and IPv6 DHCP request (Discover message) from ATE:Port[1].

**Verify that:**

* The DUT:Port[1] receives the DHCP request and forwards it to the helper IPv4 and IPv6 addresses respectively.
* The ATE:Port[1] can successfully obtain an IPv4 address that is a part of the subnet 192.0.2.32/27 with the default gateway set to 192.0.2.33.
* The ATE:Port[1] can successfully obtain an IPv6 address that is a part of the subnet 2001:db8:a:1::/64 with the default gateway set to 2001:db8:a:1::1.


### RELAY-1.1.2 DHCP request on a lag port

* Step 1 - DUT:Port[1] and DUT:Port[2] are configured as a LACP LAG (LAG1) port to ATE:Port[1] and ATE:Port[2] respectively.
* Step 2 - Send IPv4 and IPv6 DHCP request (Discover message) from ATE:Port[1].

**Verify that:**

* The DUT:Port[1] receives the DHCP request and forwards it to the helper IPv4 and IPv6 addresses respectively.
* The ATE:Port[1] can successfully obtain an IPv4 address that is a part of the subnet 192.0.2.32/27 with the default gateway set to 192.0.2.33.
* The ATE:Port[1] can successfully obtain an IPv6 address that is a part of the subnet 2001:db8:a:1::/64 with the default gateway set to 2001:db8:a:1::1.
 
#### Canonical OC

```json
{                                                                                                                                                                                                                                                                                                                          
  "openconfig-relay-agent:dhcp": {                                                                                                                                                                                                                                                                                         
    "interfaces": {                                                                                                                                                                                                                                                                                                        
      "interface": [                                                                                                                                                                                                                                                                                                       
        {                                                                                                                                                                                                                                                                                                                  
          "config": {                                                                                                                                                                                                                                                                                                      
            "helper-address": [                                                                                                                                                                                                                                                                                                             "192.0.2.254"                                                                                                                                                                                                                                                                                             
            ],                                                                                                                                                                                                                                                                                                             
            "id": "Vlan10"                                                                                                                                                                                                                                                                                                 
          },                                                                                                                                                                                                                                                                                                               
          "id": "Vlan10",                                                                                                                                                                                                                                                                                                  
          "state": {                                                                                                                                                                                                                                                                                                       
            "id": "Vlan10"                                                                                                                                                                                                                                                                                                 
          }                                                                                                                                                                                                                                                                                                                
        }                                                                                                                                                                                                                                                                                                                  
      ]                                                                                                                                                                                                                                                                                                                    
    }                                                                                                                                                                                                                                                                                                                      
  },                                                                                                                                                                                                                                                                                                                       
  "openconfig-relay-agent:dhcpv6": {                                                                                                                                                                                                                                                                                       
    "interfaces": {                                                                                                                                                                                                                                                                                                        
      "interface": [                                                                                                                                                                                                                                                                                                       
        {                                                                                                                                                                                                                                                                                                                  
          "config": {                                                                                                                                                                                                                                                                                                      
            "helper-address": [                                                                                                                                                                                                                                                                                            
              "2001:db8:a:2::1"                                                                                                                                                                                                                                                                                         
            ],                                                                                                                                                                                                                                                                                                             
            "id": "Vlan10"                                                                                                                                                                                                                                                                                                 
          },                                                                                                                                                                                                                                                                                                               
          "id": "Vlan10",                                                                                                                                                                                                                                                                                                  
          "state": {                                                                                                                                                                                                                                                                                                                   "id": "Vlan10"                                                                                                                                                                                                                                                                                                 
          }                                                                                                                                                                                                                                                                                                                
        }                                                                                                                                                                                                                                                                                                                  
      ]                                                                                                                                                                                                                                                                                                                    
    }                                                                                                                                                                                                                                                                                                                      
  }                                                                                                                                                                                                                                                                                                                        
}  
```

## OpenConfig Path and RPC Coverage

```yaml
paths:

## Config Paths ##

/relay-agent/dhcp/interfaces/interface[id=Vlan10]/config/id:
/relay-agent/dhcp/interfaces/interface[id=Vlan10]/ipv4/config/helper-address:
/relay-agent/dhcp/interfaces/interface[id=Vlan10]/ipv6/config/helper-address:

## State Paths ##


/relay-agent/dhcp/interfaces/interface/ipv4/state/helper-address:
/relay-agent/dhcp/interfaces/interface/ipv6/state/helper-address:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true

```

## Required DUT platform

* Specify the minimum DUT-type:
  * FFF - Fixed Form Factor
