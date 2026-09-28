# P4RT-5.1: Traceroute: PacketIn


## Summary

Verify that Traceroute packets are punted with correct metadata.
Verify that P4RT configurations persist after client connectivity drops. After client re-connection without programming the P4RT config verify traceroute packets are still punted.


## Procedure

*	Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2.
*	TODO: Install the set of routes on the device.
*	Enable the P4RT server on the device.
*	Connect a P4RT client and configure the forwarding pipeline. InstallP4RT table 	entries required for traceroute.
*	Send IPv4 packets from the ATE with TTL=1 and verify that packets with TTL=1 are received by the client.
*	Send IPv6 packets from the ATE with HopLimit=1 and verify that packets with HopLimit=1 are received by the client.
*	Verify that the packets have both ingress_singleton_port and egress_singleton_port metadata set.
* Disconnect clients
* Reconnect clients without setting P4RT config using SetForwardingPipelineConfig
* Start traffic and verify packets are punted as before

## Canonical OC
```json
{
  "components": {
    "component": [
      {
        "config": {
          "name": "P4RT_NODE"
        },
        "integrated-circuit": {
          "config": {
            "node-id": "1"
          }
        },
        "name": "P4RT_NODE"
      }
    ]
  },
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "dutPort1",
          "enabled": true,
          "name": "port1",
          "type": "ethernetCsmacd"
        },
        "name": "port1",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "enabled": true,
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.1",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.1"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8::1"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              }
            }
          ]
        }
      },
      {
        "config": {
          "description": "dutPort2",
          "enabled": true,
          "name": "port2",
          "type": "ethernetCsmacd"
        },
        "name": "port2",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "enabled": true,
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.5",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.5"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8::5",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8::5"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              }
            }
          ]
        }
      }
    ]
  },
  "lldp": {
    "config": {
      "enabled": false
    }
  },
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "DEFAULT"
        },
        "interfaces": {
          "interface": [
            {
              "config": {
                "id": "port1",
                "interface": "port1",
                "subinterface": 0
              },
              "id": "port1"
            },
            {
              "config": {
                "id": "port2",
                "interface": "port2",
                "subinterface": 0
              },
              "id": "port2"
            }
          ]
        },
        "name": "DEFAULT"
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage
```yaml
paths:
  /components/component/integrated-circuit/config/node-id:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /interfaces/interface/config/id:
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```
