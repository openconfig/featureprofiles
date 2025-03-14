# PF-1.4 - MPLSoGRE IPV4 encapsulation of IPV4/IPV6 payload 

## Summary
This test verifies MPLSoGRE encapsulation of IP traffic using policy-forwarding configuration. Traffic on ingress to the DUT is encapsulated and forwarded towards the egress with an IPV4 tunnel header, GRE, MPLS label and the incoming IPV4/IPV6 payload.

## Testbed type
* [`featureprofiles/topologies/atedut_8.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_8.testbed)

## Procedure
### Test environment setup

```
DUT has an ingress and 2 egress EtherChannels.

                         |         | --eBGP-- | ATE Ports 3,4 |
    [ ATE Ports 1,2 ]----|   DUT   |          |               |
                         |         | --eBGP-- | ATE Port 5,6  |
```

Test uses aggregate 802.3ad bundled interfaces (Port Channel).

* Ingress Port: Traffic is generated from Port-Channel1 (ATE Ports 1,2).

* Egress Ports: Port-Channel2 (ATE Ports 3,4) and Port-Channel3 (ATE Ports 5,6) are used as the destination ports for encapsulated traffic.

#### Configuration

#### Port-Channel1 is the ingress ports having following configuration:

#### Ten or more subinterfaces (customer) with different VLAN-IDs

* Two or more VLANs with IPV4 link local address only, /29 address

* Two or more VLANs with IPV4 global /30 address

* Two or more VLANs with IPV6 address /125 only

* Four or more VLANs with IPV4 and IPV6 address



#### L3 Address resolution

* Local proxy ARP for IPV4 (Required for traffic forwarding by DUT to any destinations within same subnet shared between DUT and Port-Channel1)

* Local proxy for IPV6 or support Secondary address for IPV6 allowing resolution of same subnet IPV6 addresses corresponding to remote Cloud endpoints

* Disable Neighbor discovery router advertisement, duplicate address detection

#### MTU Configuration
* One or more VLANs with MTU 9080 (including L2 header)

#### LLDP must be disabled

#### Every VLAN has policy-forwarding configuration

* Allow local processing of following packet types:
    * IPV4 and IPV6 echo replies to the host/local address, these packets are processed locally and not forwarded as MPLSoGRE packets

* Policy-forwarding  enabling MPLSoGRE encapsulation of all other incoming traffic:

    * Unique static MPLS label corresponding to every VLAN must be configurable on the device. If the VLAN has IPV4 and IPV6 then separate labels must be provisionable on the device for IPV4 unicast, IPV6 unicast and IPV4 multicast traffic.

    * The forwarding policy must allow forwarding of incoming traffic across 16 tunnels. 16 Tunnels has 16 source address and a single tunnel destination allowing loadbalancing of packets.

    *  Source address must be configurable as:
            * Loopback address for one or more VLANs OR
            * 16 source addresses corresponding to a single tunnel destinations to achieve maximum entropy. 

    *  Tunnel(s) to be shared across one or more VLANs


    * If TTL of the packet is 1 then the TTL must be preserved as 1 in the inner header while encapsulating the packet. If TTL greater than 1 TTL may be decremented by 1.

    * DSCP of the innermost IP packet header must be preserved

    * DSCP of the GRE/outermost IP header must be configurable (Default TOS 96)

    * TTL of the outer GRE must be configurable (Default TTL 64) 

    * QoS Hardware queues for all traffic must be configurable (default QoS hardaware class selected is 3)


### Port-Channel 2 and Port-Channel 3 configuration

* IPV4 and IPV6 addresses

* MTU (default 9216)

* LACP Member link configuration

* Lag id

* LACP (default: period short)

* Carrier-delay (default up:3000 down:150)

* Statistics load interval (default:30 seconds)

### Routing

* Advertise default routes from EBGP sessions

* ECMP (Nexthops: PortChannel-2 and PortChannel-3)

* Static mapping of MPLS label for encapsulation must be configurable

* MPLS label for a single VLAN interface must be unique for encapsulated traffic:
    * IPV4 traffic
    * IPV6 traffic
    * Multicast traffic

* ECMP (Member links in Port Channel1) based on:
    * inner IP packet header  AND/OR
    * MPLS label, Outer IP packet header 

* Inner packet TTL and DSCP must be preserved

### MPLS Label 

* Entire Label block must be reallocated for static MPLS

* Labels from start/end/mid ranges must be usable and configured corresponding to MPLSoGRE encapsulation

### Multicast

* Multicast traffic must be encapsulated and handled in the same way as unicast traffic


## PF-1.4.1: Verify PF MPLSoGRE encapsulate action for IPv4 traffic
Generate traffic on ATE Ports 1,2 having a random combination of 1000 source addresses to random destination addresses (unicast, multicast) at line rate IPV4 traffic. Use 64, 128, 256, 512, 1024.. MTU bytes frame size.

Verify:

* BGP multipath routes are programmed and LACP bundles are up without any errors, chassis alarms or exception logs
* There is no recirculation (iow, no impact to line rate traffic. No matter how the port allocation is done) of traffic
* All traffic received on Port-Channel1 other than local traffic gets forwarded as MPLSoGRE-encapsulated packets
* IPV4 unicast are preserved during encapsulation.
* No packet loss when forwarding with counters incrementing corresponding to traffic.
* Traffic equally load-balanced across 16 GRE destinations and distributed equally across 2 egress ports.
* Header fields are as expected and traffic has MPLS labels from all the VLANs


## PF-1.4.2: Verify PF MPLSoGRE encapsulate action for IPv6 traffic
Generate traffic on ATE Ports 1,2 having a random combination of 1000 source addresses to random destination addresses at line rate IPV6 traffic. Use 64, 128, 256, 512, 1024.. MTU bytes frame size.

Verify:

* All traffic received on Port-Channel1 other than local traffic gets forwarded as MPLSoGRE-encapsulated packets with IPV6 payload.
* BGP multipath routes are programmed and LACP bundles are up without any errors, chassis alarms or exception logs
* There is no recirculation (iow, no impact to line rate traffic. No matter how the port allocation is done) of traffic
* No packet loss when forwarding with counters incrementing corresponding to traffic.
* IPV6 payloads are preserved during encapsulation.
* Traffic equally load-balanced across 16 GRE destinations and distributed equally across 2 egress ports.
* Header fields are as expected and traffic has MPLS labels from all the VLANs
* There is no impact on existing IPV4 traffic
* Remove and add IPV4 configs and verify that there is no impact on IPV6 traffic
* Remove and add IPV6 configs and verify that there is no impact on IPV4 traffic

## PF-1.4.3: Verify PF MPLSoGRE DSCP preserve operation 
Generate traffic on ATE Ports 1,2 having a random combination of 1000 source addresses to random destination addresses at line rate IPV4 and IPV6 traffic. Use 64, 128, 256, 512, 1024.. MTU bytes frame size and DSCP value in [0, 8, 10..56].

Verify:

* All traffic received on Port-Channel1 other than local traffic gets forwarded as MPLSoGRE-encapsulated packets
* Outer GRE IPv4 header has marking as per the config/TOS byte 96.
* Inner packet DSCP value is preserved and not altered

## PF-1.4.4: Verify MTU handling during GRE encap
Generate IPV4 traffic on ATE Ports 1,2  with frame size of 9100 with DF-bit set to random destination addresses.
Generate IPV6 traffic on ATE Ports 1,2 with frame size of 9100 with DF-bit set to random destination addresses.

Verify:

* DUT generates a "Fragmentation Needed" message back to ATE source.


## PF-1.4.5: Verify IPV4/IPV6 nexthop resolution of encap traffic 
Clear ARP entries and IPV6 neighbors on the device
Generate IPV4 and IPV6 traffic on ATE Ports 1,2  to random destination addresses 
Generate ICMP echo requests to addresses configured on the device
Generate IPV4 and IPV6 traffic on ATE Ports 1,2  to destination addresses within same subnet as the Port-Channel1 interface

Verify:

* Device must resolve the ARP and must forward ICMP echo requests, IPV4 and IPV6 traffic to ATE destination ports including the traffic to device’s local L3 addresses
* Device must not forward the echo packets destined to the Port-Channel1 interface 
	
## PF-1.4.6: Verify IPV4/IPV6 selective local traffic processing 
Generate IPV4 and IPV6 traffic on ATE Ports 1,2  to random destination addresses including addresses configured on the device
Generate ICMP echo requests from the device
Generate traceroute packets with TTL=1 and TTL>1 from ATE ports 1,2
Generate single hop BGP and BFD (TTL=1) session packets (TTL=255)
Generate randandom customer packet with TTL = 1

Verify:
* Device must resolve the ARP and must forward ICMP echo requests, IPV4 and IPV6 traffic to ATE destination ports including the traffic to device’s local L3 addresses
* Device must selectively locally process following IPV4 and IPV6 traffic:
    * Process IPV4 and IPV6 echo replies to the local IPV4|IPV6 as local traffic
    * Respond to traceroute packets with TTL=1
    * Encapsulate(MPLSoGRE) and forward  traceroute packets with TTL>1
    * BGP and BFD packets with TTL=1 must retain the TTL value (1) and must not be decremented on the device while being forwarded as MPLSoGRE traffic

## PF-1.4.7: Verify IPV4/IPV6 traffic scale 
Generate IPV4 and IPV6 traffic on ATE Ports 1,2 to random destination addresses including addresses configured on the device
Increase the number of VLANs on the device and scale traffic across all the new VLANs on Port-Channel1 (ATE Ports 1,2)

Verify:
* BGP multipath routes are programmed and LACP bundles are up without any errors, chassis alarms or exception logs
* There is no recirculation (iow, no impact to line rate traffic. No matter how the port allocation is done) of traffic
* All traffic received on Port-Channel1 other than local traffic gets forwarded as MPLSoGRE-encapsulated packets
* IPV4 unicast are preserved during encapsulation.
* No packet loss when forwarding with counters incrementing corresponding to traffic.
* Traffic equally load-balanced across 16 GRE destinations and distributed equally across 2 egress ports.
* Header fields are as expected and traffic has MPLS labels from all the VLANs
* Verify that device can achieve the maximum interface scale on the device
* Verify that entire static label range is usable and functional by sending traffic across the entire label range


## OpenConfig Path Coverage
TODO: Finalize and update the below paths after the review and testing on any vendor device.

### JSON Format
```
       "name": "default",
        "policy-forwarding": {
          "interfaces": {
            "interface": [
              {
                "config": {
                  "apply-forwarding-policy": "pbr_cloud_id_270225037587",
                  "interface-id": "Port-Channel2.1101"
                },
                "interface-id": "Port-Channel2.1101"
              },
            ]
          },
          "policies": {
            "policy": [                                 
              {
                "config": {
                  "policy-id": "pbr_cloud_id_270225037587"
                },
                "entry-groups": {
                  "entry-group": [
                    {
                      "config": {
                        "address-family": "ACL_IPV4",
                        "group-id": 1
                      },
                      "group-id": 1,
                      "matches": {
                        "match": [
                          {
                            "config": {
                              "sequence-id": 1
                            },
                            "ipv4": {
                              "config": {
                                "destination-address": "169.254.125.155/32",
                                "protocol": "IP_ICMP"
                              },
                              "icmpv4": {
                                "config": {
                                  "type": "ECHO_REPLY"
                                }
                              }
                            },
                            "sequence-id": 1
                          }
                        ]
                      }
                    },
                    {
                      "action": {
                        "encapsulate-gre": {
                          "config": {
                            "target-type": "SHARING"
                          },
                          "targets": {
                            "target": [
                              {
                                "config": {
                                  "id": "5005-917505"
                                },
                                "id": "5005-917505"
                              },
                              {
                                "config": {
                                  "id": "5006-917505"
                                },
                                "id": "5006-917505"
                              },
                              {
                                "config": {
                                  "id": "5007-917505"
                                },
                                "id": "5007-917505"
                              },
                              {
                                "config": {
                                  "id": "5008-917505"
                                },
                                "id": "5008-917505"
                              }
                            ]
                          }
                        }
                      },
                      "arista": {
                        "config": {
                          "class-map-name": "match_all_ip_class",
                          "class-map-type": "SHARING"
                        }
                      },
                      "config": {
                        "address-family": "ACL_IPV4",
                        "group-id": 10
                      },
                      "group-id": 10,
                      "matches": {
                        "match": [
                          {
                            "config": {
                              "sequence-id": 10
                            },
                            "sequence-id": 10
                          }
                        ]
                      }
                    },
                    {
                      "action": {
                        "config": {
                          "ip-ttl": 1
                        },
                        "encapsulate-gre": {
                          "config": {
                            "target-type": "SHARING"
                          },
                          "targets": {
                            "target": [
                              {
                                "config": {
                                  "id": "5005-917505"
                                },
                                "id": "5005-917505"
                              },
                              {
                                "config": {
                                  "id": "5006-917505"
                                },
                                "id": "5006-917505"
                              },
                              {
                                "config": {
                                  "id": "5007-917505"
                                },
                                "id": "5007-917505"
                              },
                              {
                                "config": {
                                  "id": "5008-917505"
                                },
                                "id": "5008-917505"
                              }
                            ]
                          }
                        }
                      },
                      "arista": {
                        "config": {
                          "class-map-name": "match_all_ip_class",
                          "class-map-type": "SHARING"
                        }
                      },
                      "config": {
                        "address-family": "ACL_IPV4",
                        "group-id": 8
                      },
                      "group-id": 8,
                      "matches": {
                        "match": [
                          {
                            "config": {
                              "sequence-id": 8
                            },
                            "ipv4": {
                              "config": {
                                "hop-limit": 1
                              }
                            },
                            "sequence-id": 8
                          }
                        ]
                      }
                    }
                  ]
                },
                "policy-id": "pbr_cloud_id_270225037587"
              },
            }
          }
```

### Interfaces Config

```
interfaces/interface/config/description
interfaces/interface/config/enabled
interfaces/interface/config/name
interfaces/interface/config/type
interfaces/interface/config/id
interfaces/interface/config/load-interval
interfaces/interface/config/mtu
interfaces/interface/ethernet/config
interfaces/interface/ethernet/config/auto-negotiate
interfaces/interface/ethernet/config/duplex-mode
interfaces/interface/ethernet/config/port-speed
interfaces/interface/name
interfaces/interface/ethernet/config/mac-address

interfaces/interface/aggregation/config
interfaces/interface/aggregation/config/lag-type
interfaces/interface/aggregation/config/min-links

interfaces/interface/subinterfaces/subinterface/config
interfaces/interface/subinterfaces/subinterface/config/description
interfaces/interface/subinterfaces/subinterface/config/enabled
interfaces/interface/subinterfaces/subinterface/config/index
interfaces/interface/subinterfaces/subinterface/config/load-interval
interfaces/interface/subinterfaces/subinterface/index
interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config
interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip
interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length
interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/ip
interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/addr-type
interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/type
interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/ip
interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/config/ip
interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/config/link-layer-address

interfaces/interface/subinterfaces/subinterface/ipv4/proxy-arp/config/google-mode (not supported)

interfaces/interface/subinterfaces/subinterface/vlan/config/vlan-id
interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config
interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip
interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length
interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/ip
interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/addr-type
interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/type
interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/ip
interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/config/ip
interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/config/link-layer-address

lacp/interfaces/interface/name
lacp/interfaces/interface/config/interval
lacp/interfaces/interface/config/name

lldp/config/enabled
lldp/interfaces/interface/name
lldp/interfaces/interface/config/enabled
lldp/interfaces/interface/config/name
```

### Policy Config
#### 

```
/network-instances/network-instance/policy-forwarding/interfaces/interface/config/apply-forwarding-policy
/network-instances/network-instance/policy-forwarding/interfaces/interface/config/interface-id
/network-instances/network-instance/policy-forwarding/policies/policy
/network-instances/network-instance/policy-forwarding/policies/policy/policy-id
/network-instances/network-instance/policy-forwarding/policies/policy/config/policy-id
/network-instances/network-instance/policy-forwarding/interfaces/interface/interface-id
/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/network-instance
/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/config/sequence-id
/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encap-headers/encap-header/mpls/config/mpls-label-stack
/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encap-headers/encap-header/gre/config/destination-ip
/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encap-headers/encap-header/gre/config/dscp
/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encap-headers/encap-header/gre/config/id
/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encap-headers/encap-header/gre/config/ip-ttl
/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encap-headers/encap-header/gre/config/source-ip
```

### MPLS Config
#### 

```
​/​network-instances/network-instance/mpls/global/reserved-label-blocks/reserved-label-block/config
/network-instances/network-instance/mpls/global/reserved-label-blocks/reserved-label-block/local-id
/network-instances/network-instance/mpls/global/reserved-label-blocks/reserved-label-block/config/local-id
/network-instances/network-instance/mpls/global/reserved-label-blocks/reserved-label-block/config/lower-bound
/network-instances/network-instance/mpls/global/reserved-label-blocks/reserved-label-block/config/upper-bound
```



### Routing Config
####
```
/network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/use-multiple-paths/config/enabled
/network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/use-multiple-paths/ebgp/config/allow-multiple-as
/network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/use-multiple-paths/ebgp/config/maximum-paths
/network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/send-community-type
/network-instances/network-instance/protocols/protocol/bgp/global/use-multiple-paths/ebgp/link-bandwidth-ext-community/config/enabled
```

## Telemetry Path Coverage
####

```
interfaces/interface/state/counters/in-discards
interfaces/interface/state/counters/in-errors
interfaces/interface/state/counters/in-multicast-pkts
interfaces/interface/state/counters/in-pkts
interfaces/interface/state/counters/in-unicast-pkts
interfaces/interface/state/counters/out-discards
interfaces/interface/state/counters/out-errors
interfaces/interface/state/counters/out-multicast-pkts
interfaces/interface/state/counters/out-pkts
interfaces/interface/state/counters/out-unicast-pkts

interfaces/interface/subinterfaces/subinterface/state/counters/in-discards
interfaces/interface/subinterfaces/subinterface/state/counters/in-errors
interfaces/interface/subinterfaces/subinterface/state/counters/in-multicast-pkts
interfaces/interface/subinterfaces/subinterface/state/counters/in-pkts
interfaces/interface/subinterfaces/subinterface/state/counters/in-unicast-pkts
interfaces/interface/subinterfaces/subinterface/state/counters/out-discards
interfaces/interface/subinterfaces/subinterface/state/counters/out-errors
interfaces/interface/subinterfaces/subinterface/state/counters/out-multicast-pkts
interfaces/interface/subinterfaces/subinterface/state/counters/out-pkts
interfaces/interface/subinterfaces/subinterface/state/counters/out-unicast-pkts

interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-discarded-pkts
interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-error-pkts
interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-forwarded-pkts
interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-multicast-pkts
interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-pkts
interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-discarded-pkts
interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-error-pkts
interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-forwarded-pkts
interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-multicast-pkts
interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-pkts

interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-discarded-pkts
interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-error-pkts
interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-forwarded-pkts
interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-multicast-pkts
interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-pkts
interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-discarded-pkts
interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-error-pkts
interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-forwarded-pkts
interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-multicast-pkts
interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-pkts

network-instances/network-instance/policy-forwarding/policies/policy/policy-counters/state/out-pkts
network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-pkts
network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/sequence-id
```