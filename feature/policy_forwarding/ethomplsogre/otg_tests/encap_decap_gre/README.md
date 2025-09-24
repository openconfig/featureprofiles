# PF-1.23: EthoCWoMPLSoGRE IPV4 forwarding of IPV4/IPV6 payload

## Summary

This test verifies "EthoCWoMPLSoGRE" forwarding of IP traffic using policy-forwarding configuration. 

"EthoCWoMPLSoGRE" describes the encapsulation for IPv4/IPv6 packets, including the Ethernet header, all contained within GRE and MPLS headers. In addition 4-byte zero control word (CW) is inserted between the MPLS header and the inner Ethernet header.

       +------------------------------------+
       |         Outer IP Header            |
       +------------------------------------+
       |         GRE Header                 |
       +------------------------------------+
       |         MPLS Label Stack           |
       +------------------------------------+
       |         Control Word (CW)          |
       |        (4 bytes - all '0')         |
       +------------------------------------+
       |        Inner Ethernet Header       |
       +------------------------------------+
       |        Inner IP Header             |
       +------------------------------------+
       |        Layer-4 Header              |
       +------------------------------------+
       |        Payload                     |
       +------------------------------------+

## Testbed type

*  [`featureprofiles/topologies/atedut_5.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_5.testbed)

## Procedure

### Test environment setup

```text
DUT has an ingress and 2 egress aggregate interfaces.

                         |         | --eBGP-- | ATE Ports 2,3 |
    [ ATE Ports 1 ]----  |   DUT   |          |               |
                         |         | --eBGP-- | ATE Port 4,5  |
```

Test uses aggregate 802.3ad bundled interfaces (Aggregate Interfaces).

* Ingress Port: Traffic is generated from Aggregate1 (ATE Ports 1).

* Egress Ports: Aggregate2 (ATE Ports 2,3) and Aggregate3 (ATE Ports 4,5) are used as the destination ports for encapsulated traffic.

### PF-1.23.1: Generate DUT Configuration

Aggregate 1 "customer interface" is the ingress port that could either have port mode configuration or attachment mode configuration as described below. 

EACH test should be run twice - once with port mode configuration and once with attachment mode configuration.

#### Aggregate 1 "customer interface" Port mode configuration

* Configure DUT port 1 to be a member of aggregate interface named "customer interface"
* "customer interface" is a static Layer 2 bundled interface part of pseudowire that accepts packets from all VLANs.
* MTU default 9216

#### Aggregate 1 "customer interface" Attachment mode configuration

* Configure DUT port 1 to be a member of aggregate interface named "customer interface"
* Create a sub interface of the aggregated interface and assign a VLAN ID to it. 
* This sub interface will be a static Layer 2 bundled interface part of pseudowire that accepts packets from vlan ID associated with it. 
* MTU default 9216

#### Policy Forwarding Configuration 

* Policy-forwarding enabling EthoMPLSoGRE encapsulation of all incoming traffic:

  * The forwarding policy must allow forwarding of incoming traffic across 16 tunnels. 16 Tunnels has 16 source address and a single tunnel destination.

  * Source address must be configurable as:
    * Loopback address OR
    * 16 source addresses corresponding to a single tunnel destinations to achieve maximum entropy.

  * DSCP of the innermost IP packet header must be preserved during encapsulation

  * DSCP of the GRE/outermost IP header must be configurable (Default TOS 96) during encapsulation

  * TTL of the outer GRE must be configurable (Default TTL 64)

  * QoS Hardware queues for all traffic must be configurable (default QoS hardaware class selected is 3)

### Pseudowire (PW) Configuration 

* "Customer interface" is endpoint 1 and endpoint 2  is an IP address pointing towards Aggregate2, Aggregare3
* Two unique static MPLS label for local label and remote label. 
* Enable control word

### Aggregate 2 configuration

* IPV4 and IPV6 addresses

* MTU (default 9216)

* LACP Member link configuration

* Lag id

* LACP (default: period short)

* Carrier-delay (default up:3000 down:150)

* Statistics load interval (default:30 seconds)

### Routing

* Create static route for tunnel destination pointing towards Aggregate 2. 
* Static mapping of MPLS label for encapsulation must be configurable

### MPLS Label

* Entire Label block must be reallocated for static MPLS
* Labels from start/end/mid ranges must be usable and configured corresponding to EthoMPLSoGRE encapsulation

### PF-1.23.2: Verify PF EthoMPLSoGRE encapsulate action for unencrytped IPv4, IPv6 traffic with entropy on ethernet headers

* Generate 1000 different traffic flows on ATE Port 1 at line rate with a mix of both IPV4 and IPv6 traffic. Use 64, 128, 256, 512, 1024 MTU bytes frame size. 
* Flows should have entropy on Source MAC address, Destination MAC address. Other headers are fixed. 

Verify:

*  All traffic received on Aggregate2, Aggregate3 are EthoCWoMPLSoGRE-encapsulated.
*  No packet loss when forwarding.
*  Traffic equally load-balanced across 16 GRE destinations and distributed equally across 2 egress ports.

Run the test separately for both port mode and attachment mode "customer interface" configuration. 


### PF-1.23.3: Verify no hashing of EthoCWoMPLSoGRE encapsulation for unencrytped IPv4 traffic without entropy

* Generate single traffic flow on ATE Port 1 at line rate with IPV4 traffic. Use 64, 128, 256, 512, 1024 MTU bytes frame size. 
* Flows should have NOT have entropy on any headers. 

Verify:

*  All traffic received on either Aggregate2 only or Aggregate3 and is EthoCWoMPLSoGRE-encapsulated. 
*  No hashing of traffic between Aggregate2 and Aggregate3.
*  No packet loss when forwarding.
*  Traffic only takes one of the 16 GRE destinations.

Run the test separately for both port mode and attachment mode "customer interface" configuration. 


### PF-1.23.4: Verify PF EthoCWoMPLSoGRE encapsulate action for MACSec encrytped IPv4, IPv6 traffic 

* Generate 1000 different traffic flows on ATE Port 1 at line rate with a mix of both IPV4 and IPv6 traffic. Use 64, 128, 256, 512, 1024 MTU bytes frame size. 
* Flows are MACSec encrypted when sent from ATE Port 1. 
* MACSec should encrypt all headers and paylaod of traffic flow, including source mac, destination mac and VLAN tag. 
* Flows should have entropy on Source MAC address, Destination MAC address. Other headers are fixed. 

Verify:

*  All traffic received on either Aggregate2 only or Aggregate3 only and is EthoCWoMPLSoGRE-encapsulated.
*  No hashing of traffic between Aggregate2 and Aggregate3
*  No packet loss when forwarding.
*  Traffic only takes one of the 16 GRE destinations.

Run the test separately for both port mode and attachment mode "customer interface" configuration. 

### PF-1.23.5: Verify PF EthoCWoMPLSoGRE encapsulate action with Jumbo MTU
* Use the same traffic profile as PF-1.23.2. However, set the packet size to 9000 bytes 

Verify:

*  All traffic received on Aggregate2, Aggregate3 are EthoCWoMPLSoGRE-encapsulated.
*  No packet loss when forwarding.
*  Traffic equally load-balanced across 16 GRE destinations and distributed equally across 2 egress ports.

Run the test separately for both port mode and attachment mode "customer interface" configuration. 

### PF-1.23.6: Verify Control word for unencrypted traffic flow

* Use the same traffic profile as PF-1.23.2.

Verify:

*  Verify a “0” (32-bit field ) control word is inserted between the MPLS label stack and the Layer 2 payload (the Ethernet frame). 
*  All traffic received on Aggregate2, Aggregate3 are EthoCWoMPLSoGRE-encapsulated.
*  No packet loss when forwarding.
*  Traffic equally load-balanced across 16 GRE destinations and distributed equally across 2 egress ports.

Run the test separately for both port mode and attachment mode "customer interface" configuration. 

### PF-1.23.7: Verify Control word for encrypted traffic flow

* Use the same traffic profile as PF-1.23.4

Verify:

*  Verify a “0” (32-bit field ) control word is inserted between the MPLS label stack and the Layer 2 payload (the Ethernet frame). 
*  All traffic received on either Aggregate2 only or Aggregate3 only and is EthoCWoMPLSoGRE-encapsulated.
*  No hashing of traffic between Aggregate2 and Aggregate3
*  No packet loss when forwarding.
*  Traffic only takes one of the 16 GRE destinations.

Run the test separately for both port mode and attachment mode "customer interface" configuration. 

### PF-1.23.8: Verify DSCP of EthoCWoMPLSoGRE encapsulated packets

* Use the same traffic profile as PF-1.23.2. 

Verify:

*  DSCP of encapsulated packets is set to 96. 
*  All traffic received on Aggregate2, Aggregate3 are EthoCWoMPLSoGRE-encapsulated.
*  No packet loss when forwarding.
*  Traffic equally load-balanced across 16 GRE destinations and distributed equally across 2 egress ports.

Run the test separately for both port mode and attachment mode "customer interface" configuration. 

### PF-1.23.9: Verify DSCP of EthoCWoMPLSoGRE encapsulated packets

* Use the same traffic profile as PF-1.23.2. 

Verify:

*  DSCP of encapsulated packets is set to 96. 
*  All traffic received on Aggregate2, Aggregate3 are EthoCWoMPLSoGRE-encapsulated.
*  No packet loss when forwarding.
*  Traffic equally load-balanced across 16 GRE destinations and distributed equally across 2 egress ports.

Run the test separately for both port mode and attachment mode "customer interface" configuration. 

### PF-1.23.10: Verify PF EthoCWoMPLSoGRE encapsulate after single GRE tunnel destination shutdown. 

* Use the same traffic profile as PF-1.23.2. 
* Start the traffic profile on ATE. 
* Shutdown or remove a single GRE tunnel destination on the DUT. 

Verify: 

*  All traffic received on Aggregate2, Aggregate3 are EthoCWoMPLSoGRE-encapsulated.
*  No packet loss when forwarding.
*  Traffic load-balanced across remaining 15 GRE destinations and distributed equally across 2 egress ports.

### PF-1.23.11: Verify PF EthoCWoMPLSoGRE decapsulate action 

Generate traffic on ATE Aggregate2 and Aggregate3 having:

* Outer source address: random combination of 1000+ IPV4 source addresses from 100.64.0.0/22
* Outer destination address: Traffic must fall within the configured GRE tunnel sources in PF-1.23.1 so it cuold be decapsulated.
* MPLS Label: Should be same as the local label configured in PF-1.23.1
Inner payload:
* Both IPV4 and IPV6 unicast payloads, with random source address, destination address, TCP/UDP source port and destination ports
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size

* Verify:

* All traffic received on Aggregate2 and Aggregate3 gets decapsulated and forwarded as IPV4/IPV6 unicast on the respective egress interfaces under Aggregate1
* No packet loss when forwarding with counters incrementing corresponding to traffic

Run the test separately for both port mode and attachment mode "customer interface" configuration. 

### PF-1.23.12: Verify PF EthoCWoMPLSoGRE decapsulate action 

Generate traffic on ATE Aggregate2 and Aggregate3 having:

* Outer source address: random combination of 1000+ IPV4 source addresses from 100.64.0.0/22
* Outer destination address: Traffic must fall within the configured GRE tunnel sources in PF-1.23.1 so it cuold be decapsulated.
* MPLS Label: Should be same as the local label configured in PF-1.23.1
Inner payload:
* Both IPV4 and IPV6 unicast payloads, with random source address, destination address, TCP/UDP source port and destination ports
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size

* Verify:

* All traffic received on Aggregate2 and Aggregate3 gets decapsulated and forwarded as IPV4/IPV6 unicast on the respective egress interface under Aggregate1
* No packet loss when forwarding with counters incrementing corresponding to traffic

Run the test separately for both port mode and attachment mode "customer interface" configuration. 

### PF-1.23.13: Verify VLAN tag after PF EthoCWoMPLSoGRE decapsulate action

* Use the same traffic profile as PF-1.23.12.
* Ensure inner payload Ethernet header has a VLAN tag attached to it. 

* Verify:

* In port mode configuration: Traffic flow is decapsulated and mapped to the resperctive pseudowire and egress interface based on the MPLS label. Inner payload VLAN tag is retained after decapsulation. 
* In attachment mode configuration: Traffic flow is decapsulated and mapped to the resperctive pseudowire and egress interface based on the MPLS label. VLAN tag of decapsulated packet is same as the VLAN-ID associated with the egress sub-interface.  
* No packet loss when forwarding with counters incrementing corresponding to traffic

Run the test separately for both port mode and attachment mode "customer interface" configuration. 


## Canonical OC  

Port mode Interface configs  

```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "CLOUD-CSI",
          "enabled": true,
          "mtu": 9080,
          "name": "Bundle-Ether8",
          "type": "ieee8023adLag"
        },
        "name": "Bundle-Ether8",
        "rates": {
          "config": {
            "load-interval": 30
          }
        },
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "description": "CLOUD-CSI",
                "enabled": true,
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "config": {
                  "mtu": 9066
                }
              },
              "ipv6": {
                "config": {
                  "mtu": 9066
                }
              }
            }
          ]
        },
        "aggregation": {
          "config": {
            "lag-type": "STATIC"
          }
        }
      }
    ]
  }
}
```

## Canonical OC  

VLAN mode Interface configs  

```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "CLOUD-CSI",
          "enabled": true,
          "mtu": 9080,
          "name": "Bundle-Ether9",
          "type": "ieee8023adLag"
        },
        "name": "Bundle-Ether9",
        "rates": {
          "config": {
            "load-interval": 30
          }
        },
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "description": "CLOUD-GEO-PRIVATE [T=qp1309122726287]",
                "enabled": true,
                "index": 1709
              },
              "index": 1709,
              "vlan": {
                "match": {
                  "single-tagged": {
                    "config": {
                      "vlan-id": 1709
                    }
                  }
                }
              }
            }
          ]
        }
      }
    ]
  }
}
```

## Canonical OC  

Pseudowire configs Port mode  

```json
{
  "network-instances": {
    "network-instance": [
      {
        "name": "GEO_4",
        "config": {
          "name": "GEO_4",
          "type": "L2P2P"
        },
        "connection-points": {
          "connection-point": [
            {
              "connection-point-id": "GEO_4",
              "config": {
                "connection-point-id": "GEO_4"
              },
              "endpoints": {
                "endpoint": [
                  {
                    "endpoint-id": "LOCAL",
                    "config": {
                      "endpoint-id": "LOCAL",
                      "type": "LOCAL"
                    },
                    "local": {
                      "config": {
                        "interface": "Bundle-Ether9",
                        "subinterface": 0
                      }
                    }
                  },
                  {
                    "endpoint-id": "REMOTE",
                    "config": {
                      "endpoint-id": "REMOTE",
                      "type": "REMOTE"
                    },
                    "remote": {
                      "config": {
                        "virtual-circuit-identifier": 4
                      }
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
  "interfaces": {
    "interface": [
      {
        "name": "Bundle-Ether9",
        "config": {
          "name": "Bundle-Ether9",
          "type": "IANA_INTERFACE_TYPE:ieee8023adLag",
          "enabled": true
        },
        "ethernet": {
          "config": {
            "aggregate-id": "Bundle-Ether9"
          }
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "config": {
                "index": 0,
                "description": "L2P2P Service GEO_4 - Local Endpoint"
              }
            }
          ]
        }
      }
    ]
  }
}
```

## Canonical OC  

Pseudowire configs VLAN mode  

```json
{
  "network-instances": {
    "network-instance": [
      {
        "name": "GEO_4",
        "config": {
          "name": "GEO_4",
          "type": "L2P2P"
        },
        "connection-points": {
          "connection-point": [
            {
              "connection-point-id": "GEO_4",
              "config": {
                "connection-point-id": "GEO_4"
              },
              "endpoints": {
                "endpoint": [
                  {
                    "endpoint-id": "LOCAL",
                    "config": {
                      "endpoint-id": "LOCAL",
                      "type": "LOCAL"
                    },
                    "local": {
                      "config": {
                        "interface": "Bundle-Ether9",
                        "subinterface": 1709
                      }
                    }
                  },
                  {
                    "endpoint-id": "REMOTE",
                    "config": {
                      "endpoint-id": "REMOTE",
                      "type": "REMOTE"
                    },
                    "remote": {
                      "config": {
                        "virtual-circuit-identifier": 4
                      }
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
  "interfaces": {
    "interface": [
      {
        "name": "Bundle-Ether9",
        "config": {
          "name": "Bundle-Ether9",
          "type": "IANA_INTERFACE_TYPE:ieee8023adLag",
          "enabled": true
        },
        "ethernet": {
          "config": {
            "aggregate-id": "Bundle-Ether9"
          }
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 1709,
              "config": {
                "index": 1709,
                "description": "L2P2P Service GEO_4 - Local Endpoint (VLAN implied by index)"
              }
            }
          ]
        }
      }
    ]
  }
}
```

## Open Config

Tunnels/Next-hop group configs  

```json
{
    "network-instances": {
        "network-instance": [
            {
                "name": "DEFAULT",
                "config": {
                    "name": "DEFAULT"
                },
                "static": {
                    "next-hop-groups": {
                        "next-hop-group": [
                            {
                                "config": {
                                    "name": "MPLS_in_GRE_Encap"
                                },
                                "name": "MPLS_in_GRE_Encap",
                                "next-hops": {
                                    "next-hop": [
                                        {
                                            "index": "1",
                                            "config": {
                                                "index": "1"
                                            }
                                        },
                                        {
                                            "index": "2",
                                            "config": {
                                                "index": "2"
                                            }
                                        }
                                    ]
                                }
                            }
                        ]
                    },
                    "next-hops": {
                        "next-hop": [
                            {
                                "index": "1",
                                "config": {
                                    "index": "1",
                                    "next-hop": "nh_ip_addr_1",
                                    "encap-headers": {
                                        "encap-header": [
                                            {
                                                "index": "1",
                                                "type": "GRE",
                                                "config": {
                                                    "dst-ip": "outer_ipv4_dst_def",
                                                    "src-ip": "outer_ipv4_src1",
                                                    "dscp": "outer_dscp",
                                                    "ip-ttl": "outer_ip-ttl"
                                                }
                                            }
                                        ]
                                    }
                                }
                            },
                            {
                                "index": "2",
                                "config": {
                                    "index": "2",
                                    "next-hop": "nh_ip_addr_2",
                                    "encap-headers": {
                                        "encap-header": [
                                            {
                                                "index": "2",
                                                "type": "GRE",
                                                "config": {
                                                    "dst-ip": "outer_ipv4_dst_def",
                                                    "src-ip": "outer_ipv4_src2",
                                                    "dscp": "outer_dscp",
                                                    "ip-ttl": "outer_ip-ttl"
                                                }
                                            }
                                        ]
                                    }
                                }
                            }
                        ]
                    }
                }
            }
        ]
    }
}
```
## OpenConfig Path and RPC Coverage  

```yaml
paths:
    # interface configs
    /interfaces/interface/config/description:
    /interfaces/interface/config/enabled:
    /interfaces/interface/config/mtu:
    /interfaces/interface/config/name:
    /interfaces/interface/config/type:
    /interfaces/interface/rates/config/load-interval:
    /interfaces/interface/subinterfaces/subinterface/config/description:
    /interfaces/interface/subinterfaces/subinterface/config/enabled:
    /interfaces/interface/subinterfaces/subinterface/config/index:
    /interfaces/interface/subinterfaces/subinterface/ipv4/config/mtu:
    /interfaces/interface/subinterfaces/subinterface/ipv6/config/mtu:
    /interfaces/interface/aggregation/config/lag-type:

    # psuedowire configs
    /network-instances/network-instance/config/name:
    /network-instances/network-instance/config/type:
    /network-instances/network-instance/connection-points/connection-point/config/connection-point-id:
    /network-instances/network-instance/connection-points/connection-point/endpoints/endpoint/config/endpoint-id:
    /network-instances/network-instance/connection-points/connection-point/endpoints/endpoint/local/config/interface:
    /network-instances/network-instance/connection-points/connection-point/endpoints/endpoint/local/config/subinterface:
    /network-instances/network-instance/connection-points/connection-point/endpoints/endpoint/remote/config/virtual-circuit-identifier:
    
    #TODO: Add new OCs for labels and next-hop-group under connection-point 
    #/network-instances/network-instance/connection-points/connection-point/endpoints/endpoint/local/config/local-label 
    #/network-instances/network-instance/connection-points/connection-point/endpoints/endpoint/remote/config/remote-label
    #/network-instances/network-instance/connection-points/connection-point/endpoints/endpoint/remote/config/next-hop-group


    #Tunnels/Next-hop group configs

    #TODO: Add new OC for GRE encap headers
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/config/index:
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/config/next-hop:
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/config/index:
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/type:          
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/dst-ip:
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/src-ip:
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/dscp:
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/ip-ttl:
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/index:


    # Telemetry paths
    /interfaces/interface/state/counters/in-discards:
    /interfaces/interface/state/counters/in-errors:
    /interfaces/interface/state/counters/in-multicast-pkts:
    /interfaces/interface/state/counters/in-pkts:
    /interfaces/interface/state/counters/in-unicast-pkts:
    /interfaces/interface/state/counters/out-discards:
    /interfaces/interface/state/counters/out-errors:
    /interfaces/interface/state/counters/out-multicast-pkts:
    /interfaces/interface/state/counters/out-pkts:
    /interfaces/interface/state/counters/out-unicast-pkts:

    /interfaces/interface/subinterfaces/subinterface/state/counters/in-discards:
    /interfaces/interface/subinterfaces/subinterface/state/counters/in-errors:
    /interfaces/interface/subinterfaces/subinterface/state/counters/in-multicast-pkts:
    /interfaces/interface/subinterfaces/subinterface/state/counters/in-pkts:
    /interfaces/interface/subinterfaces/subinterface/state/counters/in-unicast-pkts:
    /interfaces/interface/subinterfaces/subinterface/state/counters/out-discards:
    /interfaces/interface/subinterfaces/subinterface/state/counters/out-errors:
    /interfaces/interface/subinterfaces/subinterface/state/counters/out-multicast-pkts:
    /interfaces/interface/subinterfaces/subinterface/state/counters/out-pkts:
    /interfaces/interface/subinterfaces/subinterface/state/counters/out-unicast-pkts:

    # Config paths for GRE decap
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decapsulate-gre:    

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

* MFF
* FFF
