# PF-1.24: Egress Static MPLS LSP Verification

## Summary

Verify that the Router (DUT) correctly processes incoming MPLSoverGUE traffic, 
matches the configured static LSP label, pops the label, and forwards the remaining payload 
to the correct egress interface.

## Testbed type
* [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure
### Test environment setup

```
DUT has 2 ingress aggregate interfaces and 1 egress aggregate interface.
       Egress                                      Ingress
         |                                            |
                         |         | --eBGP-- | ATE Port 2  |
    [ ATE Ports 1 ]------|   DUT   |          |             |
                         |         | --eBGP-- | ATE Port 3  |
```

Test uses standalone interfaces from DUT to ATEs.

* Ingress ATE Ports: port2 and port3
    * Port 2 and port 3 are used as the source ports for encapsulated traffic.

* Egress ATE Ports: port1
    * Traffic is forwarded (egress) on port1 .

## PF-1.24.1: Generate config for MPLS in GRE decap and push to DUT

#### Configuration

#### ATE Port1 is the egress port having following configuration:

#### Ten subinterfaces (customer) with different VLAN-IDs

* Two VLANs with IPV4 link local address only, /29 address

* Two VLANs with IPV4 global /30 address

* Two VLANs with IPV6 address /125 only

* Four VLANs with IPV4 and IPV6 address

#### MTU Configuration
* One VLAN with MTU 9080 (including L2 header)

#### ATE Port 2 & 3 configuration

* IPV4 and IPV6 addresses

* MTU (default 9216)

* Statistics load interval (default:30 seconds)

### Routing

* MPLSoGUE decapsulation prefix range must be configurable on the device.  MPLSoGUE traffic within prefix ranges must be processed by the device.

* Static mapping of MPLS label to an egress nexthop must be configurable. Egress nexthop is based on the MPLS label/ static LSP.

* MPLS label for a single egress VLAN interface must be unique for decapsulated traffic:
    * IPV4 traffic
    * IPV6 traffic
    * Multicast traffic

* Inner packet TTL and DSCP must be preserved during decap of traffic from MPLSoGUE to IP traffic

### MPLS Label 

* Entire Label block must be reallocated for static MPLS

* Labels from start/end/mid ranges must be usable and configured corresponding to MPLSoGUE decapsulation

## NOTE: All test cases expected to meet following requirements even though they are not explicitly validated in the test.
* Egress routes are programmed and there shouldnt be any chassis alarms or exception logs
* There is no recirculation (iow, no impact to line rate traffic. No matter how the port allocation is done) of traffic
* Header fields are as expected without any bit flips
* Device must be able to resolve the ARP and IPV6 neighbors upon receiving traffic from ATE ports

## PF-1.24.2: Verify MPLSoGUE decapsulate action for IPv4 and IPV6 payload
Generate traffic on ATE Ports 2 & 3 having:
* Outer source address: random combination of 1000+ IPV4 source addresses from 100.64.0.0/22
* Outer destination address: Traffic must fall within the configured IPV4 unicast decap prefix range for MPLSoGUE traffic on the device
* MPLS Labels: Configure streams that map to every egress interface by having associated IPV4/IPV6/Multicast static MPLS labels in the MPLSoGUE header
* Inner payload: 
  * Both IPV4 and IPV6 unicast payloads, with random source address, destination address, TCP/UDP source port and destination ports
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size

**Verify:**
* All traffic received on ATE port2 and port3 gets decapsulated and forwarded as IPV4/IPV6 unicast on the respective egress interfaces i.e ATE port1
* No packet loss when forwarding with counters incrementing corresponding to traffic
* Traffic equally load-balanced across member links of the aggregate interfaces
  * Based on inner payload
  * Based on outer payload
  * Based on inner and outer payload

## PF-1.24.3: Verify MPLSoGUE decapsulate action for IPv4 and IPV6 payload with changes in IPV4 and IPV6 configs
Send traffic as in PF-1.24.2
* Remove and add IPV4 interface VLAN configs and verify that there is no IPV6 traffic loss
* Remove and add IPV6 interface VLAN configs and verify that there is no IPV4 traffic loss
* Remove and add IPV4 MPLSoGUE decap configs and verify that there is no IPV6 traffic loss
* Remove and add IPV6 MPLSoGUE decap configs and verify that there is no IPV4 traffic loss


## PF-1.24.4: Verify MPLSoGUE DSCP/TTL preserve operation 
Generate traffic on ATE Ports 2 & 3 having:
* Outer source address: random combination of 1000+ IPV4 source addresses from 100.64.0.0/22
* Outer destination address: Traffic must fall within the configured IPV4 unicast decap prefix range for MPLSoGUE traffic on the device
* MPLS Labels: Configure streams that map to every egress interface by having associated IPV4/IPV6/Multicast static MPLS labels in the MPLSoGUE header
* Inner payload with all possible DSCP range 0-56 : 
  * Both IPV4 and IPV6 unicast payloads, with random source address, destination address, TCP/UDP source port and destination ports
  * Multicast traffic with random source address, TCP/UDP header with random source and destination ports
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size

**Verify:**
* All traffic received on Aggregate2 and Aggregate3 gets decapsulated and forwarded as IPV4/IPV6 unicast on the respective egress interfaces under Aggregate1
* No packet loss when forwarding with counters incrementing corresponding to traffic
* Traffic equally load-balanced across member links of the aggregate interfaces
* Header fields are as expected without any bit flips
* Inner payload DSCP and TTL values are not altered by the device

## PF-1.24.5: Verify IPV4/IPV6 nexthop resolution of decap traffic
Generate traffic (100K packets at 1000 pps) on ATE Ports 3,4,5,6 having:
* Outer source address: random combination of 1000+ IPV4 source addresses from 100.64.0.0/22
* Outer destination address: Traffic must fall within the configured IPV4 unicast decap prefix range for MPLSoGUE traffic on the device
* MPLS Labels: Configure streams that map to every egress interface by having associated IPV4/IPV6 static MPLS labels in the MPLSoGUE header
* Inner payload with all possible DSCP range 0-56 : 
  * Both IPV4 and IPV6 unicast payloads, with random source address, destination address, TCP/UDP source port and destination ports
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size
* Clear ARP entries and IPV6 neighbors on the device

**Verify:**
* No packet loss when forwarding with counters incrementing corresponding to traffic


## Canonical OC

```json
{
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "default"
        },
        "mpls": {
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
                      "next-hop": "169.254.1.138"
                    }
                  },
                  "name": "Customer IPV4 in:40571 out:pop"
                },
                {
                  "config": {
                    "name": "Customer IPV6 in:40572 out:pop"
                  },
                  "egress": {
                    "config": {
                      "incoming-label": 40572,
                      "next-hop": "2600:2d00:0:1:4000:15:69:2072"
                    }
                  },
                  "name": "Customer IPV6 in:40572 out:pop"
                }
              ]
            }
          }
        },
        "name": "default",
        "policy-forwarding": {
          "policies": {
            "policy": [
              {
                "config": {
                  "policy-id": "customer10"
                },
                "policy-id": "customer10",
                "rules": {
                  "rule": [
                    {
                      "action": {
                        "config": {
                          "decapsulate-mpls-in-udp": true
                        }
                      },
                      "config": {
                        "sequence-id": 10
                      },
                      "ipv4": {
                        "config": {
                          "destination-address": "169.254.125.155/28",
                          "protocol": 4
                        }
                      },
                      "sequence-id": 10
                    }
                  ]
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
TODO: Finalize and update the below paths after the review and testing on any vendor device.

```yaml
paths:
  # Telemetry for MPLSoGUE decap rule   
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-pkts:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-octets:
    
  # Config paths for MPLSoGUE decap
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decapsulate-mpls-in-udp:

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

  /network-instances/network-instance/afts/policy-forwarding/policy-forwarding-entry/state/counters/packets-forwarded:
  /network-instances/network-instance/afts/policy-forwarding/policy-forwarding-entry/state/counters/octets-forwarded:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/sequence-id:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

FFF

