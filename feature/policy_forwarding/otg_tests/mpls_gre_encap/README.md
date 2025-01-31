# PF-1.4 - MPLSoGRE IPV4 encapsulation of IPV4/IPV6 payload

## Summary

This test verifies MPLSoGRE encapsulation of IP traffic on routed VLAN sub interfaces on the test device. The encapsulated egress traffic must have an external IPV4 tunnel header, GRE, MPLS label and the incoming IPV4/IPV6 payload.

## Testbed type

featureprofiles/topologies/atedut_8.testbedfeatureprofiles/topologies/atedut_4.testbed

## Procedure

### Test environment setup

DUT has an ingress and 2 egress EtherChannels.

```
                        |         | --eBGP-- | ATE Ports 3,4 |
    [ ATE Ports 1,2 ]----|   DUT   |          |               |
                         |         | --eBGP-- | ATE Port 5,6  |
```

* Traffic is generated from Port-Channel1 (ATE Ports 1,2) .
* Port-Channel2 (ATE Ports 3,4) and Port-Channel3 (ATE Ports 5,6) are used as the destination ports for encapsulated traffic.

### DUT Configuration

Port-Channel1 is the ingress ports having following configuration:
Ten or more subinterfaces (customer) with different VLAN-IDs
Two or more VLANs with IPV4 link local address only
Two or more VLANs with IPV4 global /30 address
Two or more VLANs with IPV6 address /125 only
Four or more VLANs with IPV4 and IPV6 address (i+iii and i+ii)

#### MACSec

Enable MACSec on ports
Policy to cover must secure or should secure options
5 keys associated with the sessions
Refer to the JSON below
L3 Address resolution
Local proxy ARP for IPV4
Secondary address for IPV6
Disable Neighbor discovery router advertisement, duplicate address detection

#### MTU Configuration

One or more VLANs with MTU 9080 (including L2 header)

#### LLDP must be disabled

#### Every VLAN has policy-forwardingtraffic policy configuration

Allow local processing of following packet types
IPV4 and IPV6 echo replies to the host/local address
Policy-forwarding Filter based forwarding and MPLSoGRE encapsulation of all incoming traffic
Unique static MPLS label corresponding to every VLAN must be configurable on the device
The forwarding nexthop must be a composite nexthop:
16 or more MPLSoGRE tunnel entries where the encapsulated packets (MPLSoGRE) must be loadbalanced across the tunnels to a single destination.
Source address must be configurable as:
Loopback address for one or more VLANs
16 source addresses corresponding to the 16 tunnel destinations to achieve maximum entropy.
Tunnels to be shared across one or more VLANs

If TTL of the packet is 1 then the TTL must be preserved as 1 while encapsulating the packet. If TTL greater than 1 TTL may be decremented by 1.
DSCP of the incoming packet must be preserved
DSCP of the GRE header must be configurable (Default TOS 96)
TTL of the outer GRE must be configurable (Default TTL 64)
Traffic class of all traffic must be configurable (default 3)

Port-Channel 2 and Port-Channel 3 configuration
IPV4 and IPV6 addresses
MTU (default 9216)
Sflow
Member link configuration
Bundle-id
LACP (default: period short)
Carrier-delay (default up:3000 down:150)
Statistics load interval (default:30 seconds)
Routing
Advertise default routes from EBGP sessions
ECMP (Nexthops: PortChannel-2 and PortChannel-3)
Static mapping of MPLS label for encapsulation must be configurable
MPLS label for a single VLAN interface must be unique for encapsulated traffic:
IPV4 traffic
IPV6 traffic
Multicast traffic
ECMP (Member links in Port Channel1) based on:
inner IP packet header  AND/OR
MPLS label, Outer IP packet header
Inner packet TTL and DSCP must be preserved
MPLS Label
Entire Label block must be reallocated for static MPLS
Labels from start/end/mid ranges must be usable and configured corresponding to MPLSoGRE encapsulation
Multicast
Multicast traffic must be encapsulated and handled in the same way as unicast traffic

PF-1.4.1: Verify PF MPLSoGRE encapsulate action for IPv4 traffic
Generate traffic on ATE Ports 1,2 having a random combination of 1000 source addresses to random destination addresses (unicast, multicast) at line rate IPV4 traffic. Use 64, 128, 256, 512, 1024.. MTU bytes frame size.
Verify:
Verify that routes are programmed and LACP bundles are up without any errors, chassis alarms or exception logs
Verify that there is no recirculation of traffic
All traffic received on ATE Ports 3,4,5,6 MPLSoGRE-encapsulated with IPV4 unicast or IPV4 multicast payload.
No packet loss when forwarding.
Traffic equally load-balanced across 16 GRE destinations and distributed equally across 2 egress ports.
Verify PF packet counters matching traffic generated.
Verify header fields are as expected and traffic has MPLS labels from all the VLANs
PF-1.4.2: Verify PF MPLSoGRE encapsulate action for IPv6 traffic
Generate traffic on ATE Ports 1,2 having a random combination of 1000 source addresses to random destination addresses at line rate IPV6 traffic. Use 64, 128, 256, 512, 1024.. MTU bytes frame size.
Verify:
All traffic received on ATE Ports 3,4,5,6 MPLSoGRE-encapsulated with IPV6 payload.
Verify that routes are programmed and LACP bundles are up without any errors, chassis alarms or exception logs
Verify that there is no recirculation of traffic
No packet loss when forwarding.
Traffic equally load-balanced across 16 GRE destinations and distributed equally across 2 egress ports.
Verify PF packet counters matching traffic generated.
Verify header fields are as expected and traffic has MPLS labels from all the VLANs
There is no impact on existing IPV4 traffic
Remove and add IPV4 configs and verify that there is no impact on IPV6 traffic
Remove and add IPV6 configs and verify that there is no impact on IPV4 traffic

PF-1.4.3: Verify PF MPLSoGRE DSCP marking
Generate traffic on ATE Ports 1,2 having a random combination of 1000 source addresses to random destination addresses at line rate IPV4 and IPV6 traffic. Use 64, 128, 256, 512, 1024.. MTU bytes frame size and DSCP value in [0, 8, 10..56].

Verify:
All traffic received on ATE Port 3,4,5,6 MPLSoGRE-encapsulated
Outer GRE IPv4 header has marking as per the config/TOS byte 96
Inner packet DSCP value is preserved and not altered
PF-1.4.4: Verify PF MPLSoGRE DSCP preserve operation
Generate traffic on ATE Ports 1,2 having a random combination of 1000 source addresses to random destination addresses at line rate IPV4 and IPV6 traffic. Use 64, 128, 256, 512, 1024.. MTU bytes frame size and DSCP value in [0, 8, 10..56].

Verify:
All traffic received on ATE Port 3,4,5,6 MPLSOGRE-encapsulated.
Outer GRE IPv4 header has marking as per the config/TOS byte 96.
Inner packet DSCP value is preserved and not altered

PF-1.4.5: Verify MTU handling during GRE encap
Generate IPV4 traffic on ATE Ports 1,2  with frame size of 9100 with DF-bit set to random destination addresses.
Generate IPV6 traffic on ATE Ports 1,2 with frame size of 9100 with DF-bit set to random destination addresses.
Verify:
DUT generates a "Fragmentation Needed" message back to ATE source.

PF-1.4.6: Verify IPV4/IPV6 nexthop resolution
Clear ARP entries and IPV6 neighbors on the device
Generate IPV4 and IPV6 traffic on ATE Ports 1,2  to random destination addresses
Generate ICMP echo requests to addresses configured on the device

Verify:
Device must resolve the ARP and must forward ICMP echo requests, IPV4 and IPV6 traffic to ATE destination ports including the traffic to device’s local L3 addresses

PF-1.4.7: Verify IPV4/IPV6 selective local traffic processing
Generate IPV4 and IPV6 traffic on ATE Ports 1,2  to random destination addresses including addresses configured on the device
Generate ICMP echo requests from the device
Generate traceroute packets with TTL=1 and TTL>1 from ATE ports 1,2
Generate BGP and BFD packets with TTL=1

Verify:
Device must resolve the ARP and must forward ICMP echo requests, IPV4 and IPV6 traffic to ATE destination ports including the traffic to device’s local L3 addresses
Device must selectively locally process following IPV4 and IPV6 traffic:
Process IPV4 and IPV6 echo replies as local traffic
Respond to traceroute packets with TTL=1
Encapsulate(MPLSoGRE) and forward  traceroute packets with TTL>1
BGP and BFD packets with TTL=1 must retain the TTL value (1) and must not be decremented on the device while being forwarded as MPLSoGRE traffic
