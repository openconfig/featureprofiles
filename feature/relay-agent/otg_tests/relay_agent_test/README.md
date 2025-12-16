# DHCP Relay functionality

# Summary

This  is to validate the DHCP relay functionality on a DUT.  The test validates the following actions -

* DUT receives the IPv4/IPv6 DHCP discovery message over an individual or a LAG port and it will forward the request to the DHCP helper address.
* DUT forwards DHCP exchange messages between the DHCP Client and DHCP server.
* The DHCP client receives a DHCP address.

# Testbed Type

* [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)
  
# Procedure

# Test environment setup

```mermaid
graph LR; 
A[ATE:Port1] --(Vlan 10)-->B[Port1:DUT:Port2];B --Egress-->C[Port3:ATE];
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
* DUT:Port[4] IP address = IPv4-DST-B/31 / IPv6-DST-B/127
* Simulate a scenario of having a DHCP server is behind ATE:Port[3] 
* DUT:Port[3] IPv4 address = 10.10.10.0/31
* DUT:Port[3] IPv6 address = 2001:f:a::0/127
  
# Configuration

* Configure VLAN 10 on DUT.
    * Have DUT:Port[1] and DUT:Port[2] be a part of vlan 10
    * VLAN10 interface IPv4 address: 10.10.11.1/27
    * VLAN10 interface IPv6 address: 2001:f:b::1/64
    * Configure IPv4 and IPv6 helper address under VLAN10 interface.
        * IPv4 helper address - 10.10.0.67
        * IPv6 dhcp relay destination address : 2001:f:c::67 
* Configure IPv4 default route on the DUT pointing to ATE:Port[3] IPv4 address.
* Configure IPv6 default route on the  DUT pointing to ATE:Port[3] IPv6 address.


# Test - 1 DHCP request on an individual port

* Have ATE:Port[1] as an individual port and act as a DHCP client.
* Send IPv4 and IPv6 DHCP request (Discover message) from ATE:Port[1].

**Verify that:**

* The DUT:Port[1] receives the DHCP request and forwards it to the helper IPv4 and IPv6 addresses respectively.
* The ATE:Port[1] can successfully obtain an IPv4 address that is a part of the subnet 10.10.11.0/27 with the default gateway set to 10.10.11.1.
* The ATE:Port[1] can successfully obtain an IPv6 address that is a part of the subnet 2001:f:b::/64 with the default gateway set to 2001:f:b::1.


# Test - 2 DHCP request on a lag port

* DUT:Port[1] and DUT:Port[2] are configured as a LACP LAG (LAG1) port to ATE:Port[1] and ATE:Port[2] respectively.
* Send IPv4 and IPv6 DHCP request (Discover message) from ATE:Port[1].

**Verify that:**

* The DUT:Port[1] receives the DHCP request and forwards it to the helper IPv4 and IPv6 addresses respectively.
* The ATE:Port[1] can successfully obtain an IPv4 address that is a part of the subnet 10.10.11.0/27 with the default gateway set to 10.10.11.1.
* The ATE:Port[1] can successfully obtain an IPv6 address that is a part of the subnet 2001:f:b::/64 with the default gateway set to 2001:f:b::1.
 
#### Canonical OC

```json
{
  "openconfig-interfaces:interfaces": {
    "interface": [
      {
        "name": "Ethernet1",
        "config": {
          "name": "Ethernet1",
          "type": "iana-if-type:ethernetCsmacd",
          "description": "ATE:Port[1]"
        },
        "openconfig-vlan:switched-vlan": {
          "config": {
            "interface-mode": "ACCESS",
            "access-vlan": 10
          }
        }
      },
      {
        "name": "Ethernet3",
        "config": {
          "name": "Ethernet3",
          "type": "iana-if-type:ethernetCsmacd",
          "description": "ATE:Port[3]"
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "openconfig-if-ip:ipv4": {
                "addresses": {
                  "address": [
                    {
                      "ip": "10.10.10.0",
                      "config": {
                        "ip": "10.10.10.0",
                        "prefix-length": 31
                      }
                    }
                  ]
                }
              },
              "openconfig-if-ip:ipv6": {
                "addresses": {
                  "address": [
                    {
                      "ip": "2001:f:a::0",
                      "config": {
                        "ip": "2001:f:a::0",
                        "prefix-length": 127
                      }
                    }
                  ]
                }
              }
            }
          ]
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
  },
  "openconfig-network-instance:network-instances": {
    "network-instance": [
      {
        "name": "default",
        "config": {
          "name": "default"
        },
        "protocols": {
          "protocol": [
            {
              "identifier": "STATIC",
              "name": "static",
              "config": {
                "identifier": "STATIC",
                "name": "static"
              },
              "static-routes": {
                "static": [
                  {
                    "prefix": "0.0.0.0/0",
                    "config": {
                      "prefix": "0.0.0.0/0"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "index": "10.10.10.1",
                          "config": {
                            "index": "10.10.10.1",
                            "next-hop": "10.10.10.1"
                          }
                        }
                      ]
                    }
                  },
                  {
                    "prefix": "::/0",
                    "config": {
                      "prefix": "::/0"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "index": "2001:f:a::1",
                          "config": {
                            "index": "2001:f:a::1",
                            "next-hop": "2001:f:a::1"
                          }
                        }
                      ]
                    }
                  }
                ]
              }
            }
          ]
        }
      }
    ]
  },
  "openconfig-relay-agent:relay-agent": {
    "dhcp": {
      "interfaces": {
        "interface": [
          {
            "id": "Vlan10",
            "config": {
              "id": "Vlan10"
            },
            "ipv4": {
              "config": {
                "helper-address": [
                  "10.10.0.67"
                ]
              }
            },
            "ipv6": {
              "config": {
                "helper-address": [
                  "2001:f:c::67"
                ]
              }
            }
          }
        ]
      }
    }
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:

  ## Config Paths ##
   /relay-agent/dhcp/config/enable-relay-agent:
   /relay-agent/dhcp/interfaces/interface/config/helper-address:
   /relay-agent/dhcpv6/config/enable-relay-agent:
   /relay-agent/dhcpv6/interfaces/interface/config/helper-address:

  ## State Paths ##
   	/relay-agent/dhcp/interfaces/interface/state/helper-address:
   	/relay-agent/dhcpv6/interfaces/interface/state/helper-address:

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
