# PF-1.12 - MPLSoGRE IPV4 decapsulation of IPV4/IPV6 payload 

## Summary
This test verifies MPLSoGRE decapsulation of IP traffic using static MPLS LSP configuration. MPLSoGRE Traffic on ingress to the DUT is decapsulated and IPV4/IPV6 payload is forwarded towards the IPV4/IPV6 egress nexthop.

## Testbed type
* [`featureprofiles/topologies/atedut_8.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_8.testbed)

## Procedure
### Test environment setup

```
DUT has 2 ingress aggregate interfaces and 1 egress aggregate interface.

                         |         | --eBGP-- | ATE Ports 3,4 |
    [ ATE Ports 1,2 ]----|   DUT   |          |               |
                         |         | --eBGP-- | ATE Port 5,6  |
```

Test uses aggregate 802.3ad bundled interfaces (Aggregate Interfaces).

* Ingress Ports: Aggregate2 and Aggregate3
    * Aggregate2 (ATE Ports 3,4) and Aggregate3 (ATE Ports 5,6) are used as the source ports for encapsulated traffic.

* Egress Ports: Aggregate1
    * Traffic is forwarded (egress) on Aggregate1 (ATE Ports 1,2) .

## PF-1.12.1 Generate config for MPLS in GRE decap and push to DUT
#### Configuration

#### Aggregate1 is the egress port having following configuration:

#### Ten subinterfaces (customer) with different VLAN-IDs

* Two VLANs with IPV4 link local address only, /29 address

* Two VLANs with IPV4 global /30 address

* Two VLANs with IPV6 address /125 only

* Four VLANs with IPV4 and IPV6 address

#### MTU Configuration
* One VLAN with MTU 9080 (including L2 header)

#### LLDP must be disabled

### Aggregate 2 and Aggregate 3 configuration

* IPV4 and IPV6 addresses

* MTU (default 9216)

* LACP Member link configuration

* Lag id

* LACP (default: period short)

* Carrier-delay (default up:3000 down:150)

* Statistics load interval (default:30 seconds)

### Routing

* MPLSoGRE decapsulation prefix range must be configurable on the device.  MPLSoGRE traffic within prefix ranges must be processed by the device.

* Static mapping of MPLS label to an egress nexthop must be configurable. Egress nexthop is based on the MPLS label/ static LSP.

* MPLS label for a single egress VLAN interface must be unique for decapsulated traffic:
    * IPV4 traffic
    * IPV6 traffic
    * Multicast traffic

* ECMP (Member links in Aggregate1) based on:
    * inner IP packet header  AND/OR
    * MPLS label, Outer IP packet header 

* Inner packet TTL and DSCP must be preserved during decap of traffic from MPLSoGRE to IP traffic

### MPLS Label 

* Entire Label block must be reallocated for static MPLS

* Labels from start/end/mid ranges must be usable and configured corresponding to MPLSoGRE decapsulation

### Multicast

* Multicast traffic must be decapsulated and sent out with L2 header based on the multicast payload address.

## NOTE: All test cases expected to meet following requirements even though they are not explicitly validated in the test.
* Egress routes are programmed and LACP bundles are up without any errors, chassis alarms or exception logs
* There is no recirculation (iow, no impact to line rate traffic. No matter how the port allocation is done) of traffic
* Header fields are as expected without any bit flips
* Multicast L2 rewrite/egress headers are correct based on the multicast payload IPV4 destination address
* Device must be able to resolve the ARP and IPV6 neighbors upon receiving traffic from ATE ports


## PF-1.12.2: Verify MPLSoGRE decapsulate action for IPv4 and IPV6 payload
Generate traffic on ATE Ports 3,4,5,6 having:
* Outer source address: random combination of 1000+ IPV4 source addresses from 100.64.0.0/22
* Outer destination address: Traffic must fall within the configured IPV4 unicast decap prefix range for MPLSoGRE traffic on the device
* MPLS Labels: Configure streams that map to every egress interface by having associated IPV4/IPV6/Multicast static MPLS labels in the MPLSoGRE header
* Inner payload: 
  * Both IPV4 and IPV6 unicast payloads, with random source address, destination address, TCP/UDP source port and destination ports
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size

Verify:
* All traffic received on Aggregate2 and Aggregate3 gets decapsulated and forwarded as IPV4/IPV6 unicast on the respective egress interfaces under Aggregate1
* No packet loss when forwarding with counters incrementing corresponding to traffic
* Traffic equally load-balanced across member links of the aggregate interfaces
  * Based on inner payload
  * Based on outer payload
  * Based on inner and outer payload

## PF-1.12.3: Verify MPLSoGRE decapsulate action for IPv4 and IPV6 payload with changes in IPV4 and IPV6 configs
Send traffic as in PF-1.12.2
* Remove and add IPV4 interface VLAN configs and verify that there is no IPV6 traffic loss
* Remove and add IPV6 interface VLAN configs and verify that there is no IPV4 traffic loss
* Remove and add IPV4 MPLSoGRE decap configs and verify that there is no IPV6 traffic loss
* Remove and add IPV6 MPLSoGRE decap configs and verify that there is no IPV4 traffic loss

## PF-1.12.4: Verify MPLSoGRE decapsulate action for IPv4 multicast payload
Generate traffic on ATE Ports 3,4,5,6 having:
* Outer source address: random combination of 1000+ IPV4 source addresses from 100.64.0.0/22
* Outer destination address: Traffic must fall within the configured IPV4 unicast decap prefix range for MPLSoGRE traffic on the device
* MPLS Labels: Configure streams that map to every egress interface by having associated IPV4/IPV6/Multicast static MPLS labels in the MPLSoGRE header
* Inner payload: 
  * Multicast traffic with random source address, TCP/UDP header with random source and destination ports
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size

Verify:
* All traffic received on Aggregate2 and Aggregate3 gets decapsulated and forwarded as multicast traffic on the respective egress interfaces under Aggregate1
* No packet loss when forwarding with counters incrementing corresponding to traffic
* Traffic equally load-balanced across member links of the aggregate interfaces
  * Based on inner payload
  * Based on outer payload
  * Based on inner and outer payload

## PF-1.12.5: Verify MPLSoGRE DSCP/TTL preserve operation 
Generate traffic on ATE Ports 3,4,5,6 having:
* Outer source address: random combination of 1000+ IPV4 source addresses from 100.64.0.0/22
* Outer destination address: Traffic must fall within the configured IPV4 unicast decap prefix range for MPLSoGRE traffic on the device
* MPLS Labels: Configure streams that map to every egress interface by having associated IPV4/IPV6/Multicast static MPLS labels in the MPLSoGRE header
* Inner payload with all possible DSCP range 0-56 : 
  * Both IPV4 and IPV6 unicast payloads, with random source address, destination address, TCP/UDP source port and destination ports
  * Multicast traffic with random source address, TCP/UDP header with random source and destination ports
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size

Verify:
* All traffic received on Aggregate2 and Aggregate3 gets decapsulated and forwarded as IPV4/IPV6 unicast on the respective egress interfaces under Aggregate1
* No packet loss when forwarding with counters incrementing corresponding to traffic
* Traffic equally load-balanced across member links of the aggregate interfaces
* Header fields are as expected without any bit flips
* Inner payload DSCP and TTL values are not altered by the device

## PF-1.12.6: Verify IPV4/IPV6 nexthop resolution of decap traffic
Generate traffic (100K packets at 1000 pps) on ATE Ports 3,4,5,6 having:
* Outer source address: random combination of 1000+ IPV4 source addresses from 100.64.0.0/22
* Outer destination address: Traffic must fall within the configured IPV4 unicast decap prefix range for MPLSoGRE traffic on the device
* MPLS Labels: Configure streams that map to every egress interface by having associated IPV4/IPV6/Multicast static MPLS labels in the MPLSoGRE header
* Inner payload with all possible DSCP range 0-56 : 
  * Both IPV4 and IPV6 unicast payloads, with random source address, destination address, TCP/UDP source port and destination ports
  * Multicast traffic with random source address, TCP/UDP header with random source and destination ports
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size
* Clear ARP entries and IPV6 neighbors on the device

Verify:
* No packet loss when forwarding with counters incrementing corresponding to traffic

## Canonical OpenConfig for policy-forwarding matching ipv4 and decapsulate GRE
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
                  "policy-id": "decap MPLS in GRE"
                },
                "rules": {
                  "rule": [
                    {
                      "config": {
                        "sequence-id": 1
                      },
                      "ipv4": {
                        "config": {
                          "destination-address": "169.254.125.155/28",
                          "protocol": "IP"
                        },
                        }
                    },
                    "action": {
                        "decapsulate-gre": true
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
"mpls": {
  "global": {
            "interface-attributes": {
              "interface": [
                {
                  "config": {
                    "interface-id": "Aggregate4",
                    "mpls-enabled": false
                  },
                  "interface-id": "Aggregate4"
                }
              ]
            }
          },
          "lsps": {
            "static-lsps": {
              "static-lsp": [
                {
                  "config": {
                    "name": "Customer IPV4 in:40571 out:pop"
                  },
                  "egress": {
                    "config": {
                      "incoming-label": 40571,
                      "next-hop": "169.254.1.138",
                      "pipe-mode":true, # TODO: Add to OC data models, following https://datatracker.ietf.org/doc/html/rfc3270#section-2.6.2
                    }
                  }
                },
                {
                  "config": {
                    "name": "Customer IPV6 in:40572 out:pop"
                  },
                  "egress": {
                    "config": {
                      "incoming-label": 40572,
                      "next-hop": "2600:2d00:0:1:4000:15:69:2072",
                      "pipe-mode":true, # TODO: Add to OC data models, following https://datatracker.ietf.org/doc/html/rfc3270#section-2.6.2
                    }
                  }
                },
                {
                  "config": {
                    "name": "Customer multicast in:40573 out:pop"
                  },
                  "egress": {
                    "config": {
                      "incoming-label": 40573,
                      "next-hop": "239.0.1.1", # Multicast traffic must be sent out with L2 multicast header based on IP Multicast address even though there is no PIM on the egress interface
                      "pipe-mode":true, # TODO: Add to OC data models, following https://datatracker.ietf.org/doc/html/rfc3270#section-2.6.2
                    }
                  }
                },
              ]
            }
          }
    }
```

## OpenConfig Path and RPC Coverage
TODO: Finalize and update the below paths after the review and testing on any vendor device.

```yaml
paths:
  # Telemetry for GRE decap rule    
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-pkts:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-octets:
    
  # Config paths for GRE decap
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decapsulate-gre:

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
```
