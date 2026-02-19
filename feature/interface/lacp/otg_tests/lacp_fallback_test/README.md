
# RT-5.15: LACP Fallback Support 


## Summary

This  is to validate the LACP Fallback functionality on a DUT.  The tests validate the following actions -



* DUT will have the LACP bundle with ATE.
* The DUT will only participate in LACP if the LACP PDU is received from the ATE.
* If the DUT doesn’t receive a LACP PDU on the bundle ports until the fallback timeout period, then the DUT ports will act as an individual port.
* As soon as the DUT receives a LACP PDU on one of the bundle ports, the DUT will turn all the individual ports that are a part of the bundle into the bundle. 


## Testbed Type

* [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure


### Test environment setup



**Verify that:**



* DUT:Port[1] and DUT:Port[2] of LAG1 are sending LACP pdu
* After the LACP negotiation timer and fallback timer expire DUT:Port[1] and DUT:Port[2] are transitioned into fallback mode.


### RT-5.15.1 LACP fallback ports receives traffic



* Have ATE:Port[1],ATE:Port[2], and ATE:Port[3] as individual ports
* ATE:Port[1] IPv4 address = 10.10.11.2/27 and default gateway as VLAN10 Interface IPv4 address.
* ATE:Port[1] IPv6 address = 2001:f:b::2/64 and default gateway as VLAN10 Interface IPv6 address.
* Ensure DUT:Port[1] and DUT:Port[2] of LAG1 are already in LACP fallback state
* Send 5 packets from ATE:Port[1] to ipv4 address 10.10.11.3 and 10.10.10.1
* Send 5 packets from ATE:Port[1] to ipv6 address 2001:f:b::3 and 2001:f:a::1


**Verify that:**



* DUT:Ports[1] of LAG1 receives traffic.
* DUT floods traffic to 10.10.11.3 and 2001:f:b::3 to Ports[2] of LAG1 and DUT:Port[3]
* DUT forwards traffic destined to 10.10.10.1 and 2001:f:a::1 to ATE:Port[4].
* DUT:Port[1] and DUT:Port[2] of LAG1 are still sending LACP pdu


### RT-5.15.2 LACP Fallback port receives LACP pdu


* Have ATE:Port[1],ATE:Port[2], and ATE:Port[3] as individual ports
* Ensure ATE:Port[1] doesn't have IPV4 and IPv6 address present, which were configured in test 2
* Ensure DUT:Port[1] and DUT:Port[2] are in LACP fallback state
* Send LACP pdus from ATE:Port[1]


**Verify that:**



* DUT:Port[1] of LAG1 receives LACP PDU.
* Ensure DUT forms LACP over DUT:Ports[LAG11] ⇔ ATE:Ports[LAG1].
* Verify that DUT:Ports[2] of LAG1 will change its state from fallback to LACP detached


### RT-5.15.3 One of the LACP ports times out



* Enable LACP on both the ports of ATE:Port[1] and ATE:Port[2].
* Ensure LACP is established between DUT:Ports[LAG11] ⇔ ATE:Ports[LAG1].
* Stop sending LACP hello from ATE:Port[2] of LAG1 for 5 minutes.

**Verify that:**



* When DUT:Port[2] stops receiving consecutive 3*LACP Hello messages from ATE:Port[2], then DUT:Port[2]  moves from aggregate state to the detached. 
* After 5 minutes when  DUT:Port[2] starts receiving the LACP PDU, the LACP LACP will be formed again between DUT:Ports[LAG1] ⇔ ATE:Ports[LAG1].


### RT-5.15.4 Both LACP ports times out



* Enable LACP on both the ports of ATE:Port[1] and ATE:Port[2].
* Ensure LACP is established between DUT:Ports[LAG1] ⇔ ATE:Ports[LAG1].
* Stop sending LACP hello from ATE:Port[1] and ATE:Port[2] for 5 minutes.

**Verify that:**



* When DUT:Port[1] and DUT:Port[2] stops receiving consecutive 3 LACP Hello messages from ATE:Port[1] and ATE:Port[2], then DUT:Port[1] and DUT:Port[2] fall out from the aggregate state to the detached state. 
* Post LACP fallback timeout, the DUT:Port[1] and DUT:Port[2] are transitioned into fallback state.
* After 5 minutes when  DUT:Port[1] and DUT:Port[2] start receiving the LACP PDU, the LACP will be formed again between DUT:Ports[1] and DUT:Port[2] with ⇔ ATE:Ports[1] and ATE:Port[2].



#### Canonical OC
```json
{
  "openconfig-interfaces:interfaces": {
    "interface": [
      {
        "name": "Ethernet5/1",
        "config": {
          "name": "Ethernet5/1",
          "type": "iana-if-type:ethernetCsmacd",
          "description": "ATE:Port[3]"
        },
        "openconfig-vlan:switched-vlan": {
          "config": {
            "interface-mode": "ACCESS",
            "access-vlan": 10
          }
        },
        "openconfig-if-ethernet:ethernet": {
          "config": {
            "port-speed": "SPEED_100GB", 
            "auto-negotiate": true
          }
        }
      },
      {
        "name": "Ethernet29/1",
        "config": {
          "name": "Ethernet29/1",
          "type": "iana-if-type:ethernetCsmacd",
          "description": "ATE:Port[1]"
        },
        "openconfig-if-ethernet:ethernet": {
          "config": {
            "port-speed": "SPEED_100GB",
            "auto-negotiate": true,
            "aggregate-id": "Port-Channel10"
          }
        }
      },
      {
        "name": "Ethernet30/1",
        "config": {
          "name": "Ethernet30/1",
          "type": "iana-if-type:ethernetCsmacd",
          "description": "ATE:Port[2]"
        },
        "openconfig-if-ethernet:ethernet": {
          "config": {
            "port-speed": "SPEED_100GB",
            "auto-negotiate": true,
            "aggregate-id": "Port-Channel10"
          }
        }
      },
      {
        "name": "Port-Channel10",
        "config": {
          "name": "Port-Channel10",
          "type": "iana-if-type:ieee8023adLag",
          "description": "ATE:Ports[LAG1]"
        },
        "openconfig-if-aggregate:aggregation": {
          "config": {
            "lag-type": "LACP"
          }
        },
        "openconfig-vlan:switched-vlan": {
          "config": {
            "interface-mode": "ACCESS",
            "access-vlan": 10
          }
        }
      },
      {
        "name": "Vlan10",
        "config": {
          "name": "Vlan10",
          "type": "iana-if-type:l3ipvlan",
          "mtu": 9202
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "openconfig-if-ip:ipv4": {
                "addresses": {
                  "address": [
                    {
                      "ip": "10.10.11.1",
                      "config": {
                        "ip": "10.10.11.1",
                        "prefix-length": 27
                      }
                    }
                  ]
                }
              },
              "openconfig-if-ip:ipv6": {
                "addresses": {
                  "address": [
                    {
                      "ip": "2001:f:b::1",
                      "config": {
                        "ip": "2001:f:b::1",
                        "prefix-length": 64
                      }
                    }
                  ]
                }
              }
            }
          ]
        }
      }
    ]
  },
  "openconfig-lacp:lacp": {
    "interfaces": {
      "interface": [
        {
          "name": "Port-Channel10",
          "config": {
            "name": "Port-Channel10",
            "interval": "FAST",
            "lacp-mode": "ACTIVE"
            "fallback" : "TRUE"
          },
          "members": {
            "member": [
              {
                "interface": "Ethernet29/1"
              },
              {
                "interface": "Ethernet30/1"
              }
            ]
          }
        }
      ]
    }
  },
  "openconfig-vlan:vlans": {
    "vlan": [
      {
        "vlan-id": 10,
        "config": {
          "vlan-id": 10,
          "name": "vlan10",
          "status": "ACTIVE"
        }
      }
    ]
  }
}
```
## OpenConfig Path and RPC Coverage

```yaml
paths:

  ## Config Paths ##
   /interfaces/interface/ethernet/config/port-speed:
   /interfaces/interface/ethernet/config/duplex-mode:
   /interfaces/interface/ethernet/config/aggregate-id:
   /interfaces/interface/aggregation/config/lag-type:
   /lacp/interfaces/interface/config/name:
   /lacp/interfaces/interface/config/interval:
   /lacp/interfaces/interface/config/lacp-mode:
   /lacp/interfaces/interface/config/fallback:





  ## State Paths ##
   /lacp/interfaces/interface/state/name:
   /lacp/interfaces/interface/members/member/state/interface:
   /lacp/interfaces/interface/members/member/state/port-num:
   /interfaces/interface/ethernet/state/aggregate-id:
   /lacp/interfaces/interface/state/interval:
  /lacp/interfaces/interface/state/fallback:

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

