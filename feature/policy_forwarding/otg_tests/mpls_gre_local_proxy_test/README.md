# PF-1.16 - MPLSoGRE IPV4 encapsulation IPV4/IPV6 local proxy test

## Summary

This test verifies whether the device under test can resolve an IPV4 ARP or IPV6 neighbor solicitation corresponding to an address within the same subnet configured on the DUT interface. The traffic is encapsulated and sent to remote end as MPLSoGRE once the L2 resolution is successful. 

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

## PF-1.16.1: Generate DUT Configuration
Please generate config using PF-1.14.1

## PF-1.16.2: Verify IPV4/IPV6 nexthop resolution of encap traffic
Clear ARP entries and IPV6 neighbors on the device
Generate IPV4 and IPV6 traffic on ATE Ports 1,2  to random destination addresses
Generate ICMP echo requests to addresses configured on the device
Generate IPV4 and IPV6 traffic on ATE Ports 1,2  to destination addresses within same subnet as the Aggregate1 interface

Verify:
* Device must resolve the ARP and must forward ICMP echo requests, IPV4 and IPV6 traffic to ATE destination ports including the traffic to device’s local L3 addresses
* Device must not forward the echo packets destined to the Aggregate1 interface and must locally process the packet

## Canonical OpenConfig for policy-forwarding matching ipv4 and decapsulate GRE
TODO: Finalize and update the below paths after the review and testing on any vendor device.
 
```json
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "[AVAILABLE]",
          "enabled": false,
          "name": "Ethernet36/1",
          "type": "ethernetCsmacd"
        },
        "ethernet": {
          "config": {
            "auto-negotiate": false,
            "duplex-mode": "FULL",
            "port-speed": "SPEED_10GB"
          }
        },
        "name": "Ethernet36/1"
      },
      {
        "aggregation": {
          "config": {
            "lag-type": "LACP"
          }
        },
        "config": {
          "description": "IP interface1",
          "enabled": true,
          "mtu": 9080,
          "name": "Port-Channel4",
          "type": "ieee8023adLag"
        },
        "ethernet": {
          "config": {
            "mac-address": "02:00:00:00:00:01"
          }
        },
        "name": "Port-Channel4",
        "rates": {
          "config": {
            "load-interval": 30
          }
        },
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "description": "IP subinterface1",
                "enabled": true,
                "index": 1102,
                "load-interval": 30
              },
              "index": 1102,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "169.254.164.3",
                        "prefix-length": 29
                      },
                      "ip": "169.254.164.3"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "config": {
                        "ip": "169.254.164.2",
                        "link-layer-address": "02:00:00:00:00:01"
                      },
                      "ip": "169.254.164.2"
                    }
                  ]
                },
                "local-proxy-arp": true     # TODO: Add to OC data models
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2600:2d00:0:1:4000:15:69:2071",
                        "prefix-length": 125
                      },
                      "ip": "2600:2d00:0:1:4000:15:69:2071"
                    },
                    {
                      "config": {
                        "ip": "2600:2d00:0:1:4000:15:69:2073",
                        "prefix-length": 125
                      },
                      "ip": "2600:2d00:0:1:4000:15:69:2073"
                    }
                  ]
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "config": {
                        "ip": "2600:2d00:0:1:4000:15:69:2072",
                        "link-layer-address": "02:00:00:00:00:01"
                      },
                      "ip": "2600:2d00:0:1:4000:15:69:2072"
                    }
                  ]
                }
              },
              "vlan": {
                "config": {
                  "vlan-id": 1102
                }
              }
            },
          ]
        }
      },
    ]
  },
  ```

## OpenConfig Path and RPC Coverage
TODO: Finalize and update the below paths after the review and testing on any vendor device.

```yaml
paths:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-pkts:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-octets:
  /network-instances/network-instance/afts/policy-forwarding/policy-forwarding-entry/state/counters/packets-forwarded:
  /network-instances/network-instance/afts/policy-forwarding/policy-forwarding-entry/state/counters/octets-forwarded:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/sequence-id:
  /interfaces/interface/routed-vlan/ipv4/local-proxy-arp:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

FFF