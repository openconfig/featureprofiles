# PF-1.5 - MPLSoGRE IPV4 decapsulation of IPV4/IPV6 payload 

## Summary
This test verifies MPLSoGRE decapsulation of IP traffic using static MPLS LSP configuration. MPLSoGRE Traffic on ingress to the DUT is decapsulated and IPV4/IPV6 payload is forwarded towards the IPV4/IPV6 egress nexthop.

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

* Ingress Ports: Port-Channel2 and Port-Channel3
    * Port-Channel2 (ATE Ports 3,4) and Port-Channel3 (ATE Ports 5,6) are used as the source ports for encapsulated traffic.

* Egress Ports: Port-Channel1
    * Traffic is forwarded (egress) on Port-Channel1 (ATE Ports 1,2) .

#### Configuration

#### Port-Channel1 is the egress port having following configuration:

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

### Port-Channel 2 and Port-Channel 3 configuration

* IPV4 and IPV6 addresses

* MTU (default 9216)

* LACP Member link configuration

* Lag id

* LACP (default: period short)

* Carrier-delay (default up:3000 down:150)

* Statistics load interval (default:30 seconds)

### Routing

* MPLSoGRE decapsulation prefix range must be configurable on the device.  MPLSoGRE traffic within one or more prefix ranges must be processed by the device.

* Static mapping of MPLS label to an egress nexthop must be configurable. Egress nexthop is based on the MPLS label/ static LSP.

* MPLS label for a single egress VLAN interface must be unique for decapsulated traffic:
    * IPV4 traffic
    * IPV6 traffic
    * Multicast traffic

* ECMP (Member links in Port Channel1) based on:
    * inner IP packet header  AND/OR
    * MPLS label, Outer IP packet header 

* Inner packet TTL and DSCP must be preserved during decap of traffic from MPLSoGRE to IP traffic

### MPLS Label 

* Entire Label block must be reallocated for static MPLS

* Labels from start/end/mid ranges must be usable and configured corresponding to MPLSoGRE decapsulation

### Multicast

* Multicast traffic must be decapsulated and sent out with L2 header based on the multicast payload address.


## PF-1.5.1: Verify MPLSoGRE decapsulate action for IPv4 and IPV6 payload
Generate traffic on ATE Ports 3,4,5,6 having:
* Outer source address: random combination of 1000+ IPV4 source addresses
* Outer destination address: Traffic must fall within the configured IPV4 unicast decap prefix range for MPLSoGRE traffic on the device
* MPLS Labels: Configure streams that map to every egress interface by having associated IPV4/IPV6/Multicast static MPLS labels in the MPLSoGRE header
* Inner payload: 
  * Both IPV4 and IPV6 unicast payloads, with random source address, destination address, TCP/UDP source port and destination ports
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size

Verify:

* Egress routes are programmed and LACP bundles are up without any errors, chassis alarms or exception logs
* There is no recirculation (iow, no impact to line rate traffic. No matter how the port allocation is done) of traffic
* All traffic received on Port-Channel2 and Port-Channel3 gets decapsulated and forwarded as IPV4/IPV6 unicast on the respective egress interfaces under Port-Channel1
* No packet loss when forwarding with counters incrementing corresponding to traffic
* Traffic equally load-balanced across member links of the port channel
  * Based on inner payload
  * Based on outer payload
  * Based on inner and outer payload
* Header fields are as expected without any bit flips
* Remove and add IPV4 configs and verify that there is no impact on IPV6 traffic
* Remove and add IPV6 configs and verify that there is no impact on IPV4 traffic


## PF-1.5.2: Verify MPLSoGRE decapsulate action for IPv4 multicast payload
Generate traffic on ATE Ports 3,4,5,6 having:
* Outer source address: random combination of 1000+ IPV4 source addresses
* Outer destination address: Traffic must fall within the configured IPV4 unicast decap prefix range for MPLSoGRE traffic on the device
* MPLS Labels: Configure streams that map to every egress interface by having associated IPV4/IPV6/Multicast static MPLS labels in the MPLSoGRE header
* Inner payload: 
  * Multicast traffic with random source address, TCP/UDP header with random source and destination ports
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size

Verify:

* Egress routes are programmed and LACP bundles are up without any errors, chassis alarms or exception logs
* There is no recirculation (iow, no impact to line rate traffic. No matter how the port allocation is done) of traffic
* All traffic received on Port-Channel2 and Port-Channel3 gets decapsulated and forwarded as multicast traffic on the respective egress interfaces under Port-Channel1
* No packet loss when forwarding with counters incrementing corresponding to traffic
* Traffic equally load-balanced across member links of the port channel
  * Based on inner payload
  * Based on outer payload
  * Based on inner and outer payload
* Header fields are as expected without any bit flips
* Verify that multicast L2 rewrite/egress headers are correct based on the multicast payload IPV4 destination address 

## PF-1.5.3: Verify MPLSoGRE DSCP/TTL preserve operation 
Generate traffic on ATE Ports 3,4,5,6 having:
* Outer source address: random combination of 1000+ IPV4 source addresses
* Outer destination address: Traffic must fall within the configured IPV4 unicast decap prefix range for MPLSoGRE traffic on the device
* MPLS Labels: Configure streams that map to every egress interface by having associated IPV4/IPV6/Multicast static MPLS labels in the MPLSoGRE header
* Inner payload with all possible DSCP range 0-56 : 
  * Both IPV4 and IPV6 unicast payloads, with random source address, destination address, TCP/UDP source port and destination ports
  * Multicast traffic with random source address, TCP/UDP header with random source and destination ports
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size

Verify:

* Egress routes are programmed and LACP bundles are up without any errors, chassis alarms or exception logs
* There is no recirculation (iow, no impact to line rate traffic. No matter how the port allocation is done) of traffic
* All traffic received on Port-Channel2 and Port-Channel3 gets decapsulated and forwarded as IPV4/IPV6 unicast on the respective egress interfaces under Port-Channel1
* No packet loss when forwarding with counters incrementing corresponding to traffic
* Traffic equally load-balanced across member links of the port channel
* Header fields are as expected without any bit flips
* Inner payload DSCP and TTL values are not altered by the device

## PF-1.5.4: Verify IPV4/IPV6 nexthop resolution of decap traffic
Generate traffic on ATE Ports 3,4,5,6 having:
* Outer source address: random combination of 1000+ IPV4 source addresses
* Outer destination address: Traffic must fall within the configured IPV4 unicast decap prefix range for MPLSoGRE traffic on the device
* MPLS Labels: Configure streams that map to every egress interface by having associated IPV4/IPV6/Multicast static MPLS labels in the MPLSoGRE header
* Inner payload with all possible DSCP range 0-56 : 
  * Both IPV4 and IPV6 unicast payloads, with random source address, destination address, TCP/UDP source port and destination ports
  * Multicast traffic with random source address, TCP/UDP header with random source and destination ports
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size
* Clear ARP entries and IPV6 neighbors on the device

Verify:

* Device must resolve the ARP and IPV6 neighbors upon receiving traffic
* No packet loss when forwarding with counters incrementing corresponding to traffic


## PF-1.5.5: Verify IPV4/IPV6 traffic scale 
Generate traffic on ATE Ports 3,4,5,6 having:
* Outer source address: random combination of 1000+ IPV4 source addresses
* Outer destination address: Traffic must fall within the configured IPV4 unicast decap prefix range for MPLSoGRE traffic on the device
* MPLS Labels: Configure streams that map to every egress interface by having associated IPV4/IPV6/Multicast static MPLS labels in the MPLSoGRE header
* Inner payload with all possible DSCP range 0-56 : 
  * Both IPV4 and IPV6 unicast payloads, with random source address, destination address, TCP/UDP source port and destination ports
  * Multicast traffic with random source address, TCP/UDP header with random source and destination ports
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size
* Increase the number of VLANs under PortChannel1 with traffic running until the maximum possible VLANs available on the device

Verify:
* Egress routes are programmed and LACP bundles are up without any errors, chassis alarms or exception logs
* There is no recirculation (iow, no impact to line rate traffic. No matter how the port allocation is done) of traffic
* All traffic received on Port-Channel2 and Port-Channel3 gets decapsulated and forwarded as IPV4/IPV6 unicast on the respective egress interfaces under Port-Channel1
* No packet loss when forwarding with counters incrementing corresponding to traffic
* Traffic equally load-balanced across member links of the port channel
* Header fields are as expected without any bit flips
* Inner payload DSCP and TTL values are not altered by the device
* Device can achieve the maximum interface scale on the device
* Entire static label range is usable and functional by sending traffic across the entire label range


## OpenConfig Path Coverage
TODO: Finalize and update the below paths after the review and testing on any vendor device.

### JSON Format
```json
       "name": "default",
        "policy-forwarding": {
              {
                "config": {
                  "policy-id": "pf-decap-range"
                },
                "entry-groups": {
                  "entry-group": [
                    {
                      "action": {
                        "config": {
                          "decapsulate-gre": true
                        }
                      },
                      "config": {
                        "group-id": 0
                      },
                      "group-id": 0,
                      "matches": {
                        "match": [
                          {
                            "config": {
                              "sequence-id": 0
                            },
                            "ipv4": {
                              "config": {
                                "destination-address": "10.1.157.160/28"
                              }
                            },
                            "sequence-id": 0
                          }
                        ]
                      }
                    }
                  ]
                },
                "policy-id": "pf-decap-range"
              }
        }

          "mpls": {
          "global": {
            "interface-attributes": {
              "interface": [
                {
                  "config": {
                    "interface-id": "Port-Channel4",
                    "mpls-enabled": false
                  },
                  "interface-id": "Port-Channel4"
                }
              ]
            }
          },
          "lsps": {
            "static-lsps": {
              "static-lsp": [
                {
                  "config": {
                    "name": "LA_POP 169.254.1.138 in:40571 out:pop"
                  },
                  "egress": {
                    "config": {
                      "incoming-label": 40571,
                      "next-hop": "169.254.1.138",
                      "payload-type": "IPV4"
                    }
                  },
                  "name": "LA_POP 169.254.1.138 in:40571 out:pop"
                },
                {
                  "config": {
                    "name": "LA_POP 169.254.100.138 in:40422 out:pop"
                  },
                  "egress": {
                    "config": {
                      "incoming-label": 40422,
                      "next-hop": "169.254.100.138",
                      "payload-type": "IPV4"
                    }
                  },
                  "name": "LA_POP 169.254.100.138 in:40422 out:pop"
                },
                                {
                  "config": {
                    "name": "LA_POP 2600:2d00:0:1:4000:15:7d:e1ba in:69128 out:pop"
                  },
                  "egress": {
                    "config": {
                      "incoming-label": 69128,
                      "next-hop": "2600:2d00:0:1:4000:15:7d:e1ba",
                      "payload-type": "IPV6"
                    }
                  },
                  "name": "LA_POP 2600:2d00:0:1:4000:15:7d:e1ba in:69128 out:pop"
                },
                {
                  "config": {
                    "name": "LA_POP 2600:2d00:0:1:4000:15:7d:e20a in:69775 out:pop"
                  },
                  "egress": {
                    "config": {
                      "incoming-label": 69775,
                      "next-hop": "2600:2d00:0:1:4000:15:7d:e20a",
                      "payload-type": "IPV6"
                    }
                  },
                  "name": "LA_POP 2600:2d00:0:1:4000:15:7d:e20a in:69775 out:pop"
                },
              ]
            }
          }
        },
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
    # Telemetry for GRE decap rule    
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-pkts:
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-octets:
    
    # Config paths for GRE decap
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decapsulate-gre:

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