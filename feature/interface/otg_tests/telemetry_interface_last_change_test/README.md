# gNMI-1.24: Telemetry: Interface Last Change Timestamp

## Summary

The test validates that the `last-change` timestamp for an interface and its
subinterface is updated correctly when the interface state
changes. This is tested for both physical Ethernet interfaces and Link
Aggregation Group (LAG) interfaces.

## Testbed Topology

A single DUT with at least one port connected to an ATE is required.

## Procedure

### gNMI-1.24.1: TestEthernetInterfaceLastChangeState

1.  **Configure Ethernet Interface**:
    *   Select a port on the DUT.
    *   Configure it as an Ethernet interface.
    *   Create a subinterface with index 0.
    *   Configure IPv4 and IPv6 addresses on the subinterface.
    *   Enable the interface and the subinterface.
2.  **Verify Initial State**:
    *   Wait for the `oper-status` of both the interface and subinterface to become `UP`.
    *   Read and store the initial `last-change` timestamp for the interface from `/interfaces/interface[name=<port>]/state/last-change`.
    *   Read and store the initial `last-change` timestamp for the subinterface from `/interfaces/interface[name=<port>]/subinterfaces/subinterface[index=0]/state/last-change`.
3.  **Flap Interface**:
    *   Disable the interface by setting `/interfaces/interface[name=<port>]/config/enabled` to `false`.
    *   Wait for the `oper-status` of both the interface and subinterface to become `DOWN`.
4.  **Verify Final State**:
    *   Read and store the final `last-change` timestamp for the interface and subinterface from the same paths as in step 2.
    *   Verify that the final `last-change` timestamp is greater than the initial timestamp for both the interface and subinterface.

#### Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "Description for et-0/0/2:1",
          "enabled": true,
          "name": "et-0/0/2:1",
          "type": "ethernetCsmacd"
        },
        "name": "et-0/0/2:1",
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
                        "ip": "192.168.1.1",
                        "prefix-length": 30
                      },
                      "ip": "192.168.1.1"
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
                        "ip": "2001:DB8::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:DB8::1"
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
  }
}
```

### gNMI-1.24.2: TestLAGInterfaceLastChangeState

1.  **Configure LAG Interface**:
    *   Select a port on the DUT to be a member of a LAG.
    *   Create a LAG interface.
    *   Assign the selected port as a member of the LAG.
    *   Create a subinterface with index 0 on the LAG interface.
    *   Configure an IPv4 address on the subinterface.
2.  **Verify Initial State**:
    *   Enable the LAG interface.
    *   Wait for the `oper-status` of both the LAG interface and its subinterface to become `UP`.
    *   Read and store the initial `last-change` timestamp for the LAG interface from `/interfaces/interface[name=<lag>]/state/last-change`.
    *   Read and store the initial `last-change` timestamp for the subinterface from `/interfaces/interface[name=<lag>]/subinterfaces/subinterface[index=0]/state/last-change`.
3.  **Flap Interface**:
    *   Disable the LAG interface by setting `/interfaces/interface[name=<lag>]/config/enabled` to `false`.
    *   Wait for the `oper-status` of both the LAG interface and its subinterface to become `DOWN`.
4.  **Verify Final State**:
    *   Read and store the final `last-change` timestamp for the LAG interface and subinterface from the same paths as in step 2.
    *   Verify that the final `last-change` timestamp is greater than the initial timestamp for both the interface and subinterface.

#### Canonical OC
```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "enabled": true,
          "name": "et-0/0/2:1",
          "type": "ethernetCsmacd"
        },
        "ethernet": {
          "config": {
            "aggregate-id": "lag3",
            "auto-negotiate": false,
            "duplex-mode": "FULL",
            "port-speed": "SPEED_100GB"
          }
        },
        "name": "et-0/0/2:1"
      },
      {
        "aggregation": {
          "config": {
            "lag-type": "STATIC"
          }
        },
        "config": {
          "name": "lag3",
          "type": "ieee8023adLag"
        },
        "name": "lag3",
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
                        "ip": "192.168.20.1",
                        "prefix-length": 30
                      },
                      "ip": "192.168.20.1"
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
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
/interfaces/interface/state/oper-status:
/interfaces/interface/state/last-change:
/interfaces/interface/subinterfaces/subinterface/state/last-change:

rpcs:
  gnmi:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

* FFF - fixed form factor

