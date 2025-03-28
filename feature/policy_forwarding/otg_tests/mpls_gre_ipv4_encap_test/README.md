# PF-1.14 - MPLSoGRE IPV4 encapsulation of IPV4/IPV6 payload

## Summary

This test verifies MPLSoGRE encapsulation of IP traffic using policy-forwarding configuration. Traffic on ingress to the DUT is encapsulated and forwarded towards the egress with an IPV4 tunnel header, GRE, MPLS label and the incoming IPV4/IPV6 payload.

## Testbed type

* [`featureprofiles/topologies/atedut_8.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_8.testbed)

## Procedure

### Test environment setup

```text
DUT has an ingress and 2 egress aggregate interfaces.

                         |         | --eBGP-- | ATE Ports 3,4 |
    [ ATE Ports 1,2 ]----|   DUT   |          |               |
                         |         | --eBGP-- | ATE Port 5,6  |
```

Test uses aggregate 802.3ad bundled interfaces (Aggregate Interfaces).

* Ingress Port: Traffic is generated from Aggregate1 (ATE Ports 1,2).

* Egress Ports: Aggregate2 (ATE Ports 3,4) and Aggregate3 (ATE Ports 5,6) are used as the destination ports for encapsulated traffic.

## NOTE: All test cases expected to meet following requirements even though they are not explicitly validated in the test.
* BGP multipath routes are programmed and LACP bundles are up without any errors, chassis alarms or exception logs
* There is no recirculation (iow, no impact to line rate traffic. No matter how the port allocation is done) of traffic
* Header fields are as expected without any bit flips
* Device must be able to resolve the ARP and IPV6 neighbors upon receiving traffic from ATE ports

### PF-1.14.1: Generate DUT Configuration

#### Aggregate "customer interface" is the ingress port having following configuration
* Configure DUT ports 1,2 to be a member of aggregate interface named "customer interface"

#### Ten subinterfaces on "customer-interface" with different VLAN-IDs
* Two VLANs with IPV4 link local address only, /29 address

* Two VLANs with IPV4 global /30 address

* Two VLANs with IPV6 address /125 only

* Four VLANs with IPV4 and IPV6 address

#### L3 Address resolution
* Local proxy ARP for IPV4 (Required for traffic forwarding by DUT to any destinations within same subnet shared between DUT and Aggregate1)

* Local proxy for IPV6 or support Secondary address for IPV6 allowing resolution of same subnet IPV6 addresses corresponding to remote Cloud endpoints

* Disable Neighbor discovery router advertisement, duplicate address detection

#### MTU Configuration
* One VLAN with MTU 9080 (including L2 header)

#### LLDP must be disabled

#### Every VLAN has policy-forwarding configuration

* Allow local processing of following packet types:
  * IPV4 and IPV6 echo replies to the host/local address, these packets are processed locally and not forwarded as MPLSoGRE packets

* Policy-forwarding  enabling MPLSoGRE encapsulation of all other incoming traffic:

  * Unique static MPLS label corresponding to every VLAN must be configurable on the device. If the VLAN has IPV4 and IPV6 then separate labels must be provisionable on the device for IPV4 unicast, IPV6 unicast and IPV4 multicast traffic.

  * The forwarding policy must allow forwarding of incoming traffic across 16 tunnels. 16 Tunnels has 16 source address and a single tunnel destination allowing loadbalancing of packets.

  * Source address must be configurable as:
    * Loopback address OR
    * 16 source addresses corresponding to a single tunnel destinations to achieve maximum entropy.

  * Tunnel(s) to be shared across multiple VLANs

  * If TTL of the incoming packet is 1 then the TTL must be preserved as 1 in the inner header while encapsulating the packet. If TTL is greater than 1 TTL may be decremented by 1.

  * DSCP of the innermost IP packet header must be preserved during encapsulation

  * DSCP of the GRE/outermost IP header must be configurable (Default TOS 96) during encapsulation

  * TTL of the outer GRE must be configurable (Default TTL 64)

  * QoS Hardware queues for all traffic must be configurable (default QoS hardaware class selected is 3)

### Aggregate 2 and Aggregate 3 configuration
* IPV4 and IPV6 addresses

* MTU (default 9216)

* LACP Member link configuration

* Lag id

* LACP (default: period short)

* Carrier-delay (default up:3000 down:150)

* Statistics load interval (default:30 seconds)

### Routing
* Advertise default routes from EBGP sessions

* ECMP (Nexthops: Aggregate2 and Aggregate3)

* Static mapping of MPLS label for encapsulation must be configurable

* MPLS label for a single VLAN interface must be unique for encapsulated traffic:
  * IPV4 traffic
  * IPV6 traffic
  * Multicast traffic

* ECMP (Member links in Aggregate1) based on:
  * inner IP packet header  AND/OR
  * MPLS label, Outer IP packet header

* Inner packet TTL and DSCP must be preserved

### MPLS Label
* Entire Label block must be reallocated for static MPLS
* Labels from start/end/mid ranges must be usable and configured corresponding to MPLSoGRE encapsulation

### Multicast
* Multicast traffic must be encapsulated and handled in the same way as unicast traffic

## PF-1.14.2: Verify PF MPLSoGRE encapsulate action for IPv4 traffic
Generate traffic on ATE Ports 1,2 having a random combination of 1000 source addresses to random destination addresses (unicast, multicast) at line rate IPV4 traffic. Use 64, 128, 256, 512, 1024.. MTU bytes frame size.

Verify:
* All traffic received on Aggregate1 other than local traffic gets forwarded as MPLSoGRE-encapsulated packets
* No packet loss when forwarding with counters incrementing corresponding to traffic.
* Traffic equally load-balanced across 16 GRE destinations and distributed equally across 2 egress ports.

## PF-1.14.3: Verify PF MPLSoGRE encapsulate action for IPv6 traffic
Generate traffic on ATE Ports 1,2 having a random combination of 1000 source addresses to random destination addresses at line rate IPV6 traffic. Use 64, 128, 256, 512, 1024.. MTU bytes frame size.

Verify:
* All traffic received on Aggregate1 other than local traffic gets forwarded as MPLSoGRE-encapsulated packets with IPV6 payload.
* No packet loss when forwarding with counters incrementing corresponding to traffic.
* Traffic equally load-balanced across 16 GRE destinations and distributed equally across 2 egress ports.
* There is no impact on existing IPV4 traffic

## PF-1.14.4: Verify PF MPLSoGRE encapsulate action for IPv6 traffic
Send both IPV4 and IPV6 traffic as in PF-1.14.2 and PF-1.14.3

* Remove and add IPV4 interface VLAN configs and verify that there is no IPV6 traffic loss
* Remove and add IPV6 interface VLAN configs and verify that there is no IPV4 traffic loss
* Remove and add IPV4 MPLSoGRE encap configs and verify that there is no IPV6 traffic loss
* Remove and add IPV6 MPLSoGRE encap configs and verify that there is no IPV4 traffic loss

## PF-1.14.5: Verify PF MPLSoGRE DSCP preserve operation
Generate traffic on ATE Ports 1,2 having a random combination of 1000 source addresses to random destination addresses at line rate IPV4 and IPV6 traffic. Use 64, 128, 256, 512, 1024.. MTU bytes frame size and DSCP value in [0, 8, 10..56].

Verify:
* All traffic received on Aggregate1 other than local traffic gets forwarded as MPLSoGRE-encapsulated packets
* Outer GRE IPv4 header has marking as per the config/TOS byte 96.
* Inner packet DSCP value is preserved and not altered

## PF-1.14.6: Verify MTU handling during GRE encap
Generate IPV4 traffic on ATE Ports 1,2  with frame size of 9100 with DF-bit set to random destination addresses.
Generate IPV6 traffic on ATE Ports 1,2 with frame size of 9100 with DF-bit set to random destination addresses.

Verify:
* DUT generates a "Fragmentation Needed" message back to ATE source.

## PF-1.14.7: Verify IPV4/IPV6 nexthop resolution of encap traffic
Clear ARP entries and IPV6 neighbors on the device
Generate IPV4 and IPV6 traffic on ATE Ports 1,2  to random destination addresses
Generate ICMP echo requests to addresses configured on the device
Generate IPV4 and IPV6 traffic on ATE Ports 1,2  to destination addresses within same subnet as the Aggregate1 interface

Verify:
* Device must resolve the ARP and must forward ICMP echo requests, IPV4 and IPV6 traffic to ATE destination ports including the traffic to device’s local L3 addresses
* Device must not forward the echo packets destined to the Aggregate1 interface
 
## PF-1.14.8: Verify IPV4/IPV6 selective local traffic processing
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

## Canonical OpenConfig for policy-forwarding matching ipv4 and decapsulate GRE
TODO: Finalize and update the below paths after the review and testing on any vendor device.

```json
"network-instances": {
  "network-instance": {
    "DEFAULT": {
       "name": "default",
       "policy-forwarding": {
         "policies": {
           "policy": [
              {
                "config": {
                  "policy-id": "customerA"
                },
                "rules": {
                  "rule": [
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
                      "action": {
                        "encapsulate-gre": {
                          "targets": {
                            "target": [
                              {
                                "config": {
                                  "id": "mygre_dest_A"
                                  "destination": "10.1.1.0/29"
                                  "ip-ttl": "1"
                                },
                                "id": "mygre_dest_A"
                              },
                              {
                                "config": {
                                  "id": "mygre_dest_B"
                                  "destination": "10.1.1.8/29"
                                  "ip-ttl": "1"
                                },
                                "id": "mygre_dest_B"
                              },
                            ]
                          }
                        }
                      },
                      "sequence-id": 1
                    }
                  ]
                },

              }
           ]
         }
       }
    }
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-pkts:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-octets:
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

  /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-discarded-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-error-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-forwarded-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-multicast-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-discarded-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-error-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-forwarded-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-multicast-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-pkts:
  
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-discarded-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-error-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-forwarded-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-multicast-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-discarded-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-error-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-forwarded-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-multicast-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-pkts:

  /network-instances/network-instance/policy-forwarding/policies/policy/policy-counters/state/out-pkts:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/sequence-id:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

FFF
