# gNMI-1.25: Telemetry: Interface Last Change Timestamp

## Summary

The test validates that the `last-change` timestamp for an interface and its
subinterface is updated correctly when the interface state
changes. This is tested for both physical Ethernet interfaces and Link
Aggregation Group (LAG) interfaces.

## Testbed Topology

A single DUT with at least one port connected to an ATE is required.

## Procedure

### gNMI-1.25.1: TestEthernetInterfaceLastChangeState

This test verifies that the `last-change` timestamp for a physical Ethernet
interface and its subinterface is updated correctly when the interface state
changes.

1.  **Configure Ethernet Interface**:
    *   Select a port on the DUT.
    *   Configure it as an Ethernet interface.
    *   Create a subinterface with index 0.
    *   Configure IPv4 and IPv6 addresses on the subinterface.
    *   Enable the interface and the subinterface.

2.  **Common Flap Procedure**:
    For each sub-test below, the following steps are performed:
    *   **Verify Initial State**:
        *   Wait for the `oper-status` of the interface to become `UP`.
        *   Read and store the initial `last-change` timestamp for the interface from `/interfaces/interface[name=<port>]/state/last-change`.
        *   Read and store the initial `last-change` timestamp for the subinterface from `/interfaces/interface[name=<port>]/subinterfaces/subinterface[index=0]/state/last-change`.
    *   **Repeated Flap and Verify**: The interface is flapped multiple times (e.g., 10 cycles of disable/enable).
        *   **Flap Interface**: The interface state is changed (e.g., disabled then enabled) as per the specific sub-test.
        *   **Wait for Oper-Status**: Wait for the `oper-status` of both the interface and subinterface to reflect the flap (e.g., `DOWN` then `UP`).
        *   **Verify Last-Change**: Read the `last-change` timestamps again.
            Verify that the final `last-change` timestamp is greater than the
            timestamp recorded *before* the current flap cycle for both the
            interface and subinterface.

#### Sub-tests:

*   **OCInterfaceFlap**: The interface flap is triggered by updating the
    `/interfaces/interface[name=<port>]/config/enabled` path on the DUT via
    gNMI.

*   **OTGInterfaceFlap**: An ATE port is connected to the DUT port. The flap is
    triggered by changing the link state of the ATE port using OTG controls.

*   **LaserCutFlap**: The interface flap is triggered by simulating a "laser
    cut" on the DUT port. This is done by disabling the transmit laser on the
    DUT port using the path
    `/components/component/transceiver/physical-channels/channel/config/tx-laser`.

    [TODO] Implement when OC path is supported.

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

### gNMI-1.25.2: TestLAGInterfaceLastChangeState

This test verifies that the `last-change` timestamp for a Link Aggregation Group
(LAG) interface and its subinterface is updated correctly when the LAG state
changes.

1.  **Configure LAG Interface**:
    *   Select a port on the DUT to be a member of a LAG.
    *   Create a LAG interface.
    *   Assign the selected port as a member of the LAG.
    *   Create a subinterface with index 0 on the LAG interface.
    *   Configure an IPv4 address on the subinterface.

2.  **Common Flap Procedure**:
    For each sub-test below, the following steps are performed:
    *   **Verify Initial State**:
        *   Ensure the LAG interface is UP.
        *   Wait for the `oper-status` of both the LAG interface and its subinterface to become `UP`.
        *   Read and store the initial `last-change` timestamp for the LAG interface from `/interfaces/interface[name=<lag>]/state/last-change`.
        *   Read and store the initial `last-change` timestamp for the subinterface from `/interfaces/interface[name=<lag>]/subinterfaces/subinterface[index=0]/state/last-change`.
    *   **Repeated Flap and Verify**: The LAG state is flapped multiple times (e.g., 10 cycles of disable/enable).
        *   **Flap LAG State**: The LAG state is changed as per the specific sub-test.
        *   **Wait for Oper-Status**: Wait for the `oper-status` of both the LAG
            interface and its subinterface to reflect the flap.
        *   **Verify Last-Change**: Read the `last-change` timestamps again.
            Verify that the final `last-change` timestamp is greater than the
            timestamp recorded *before* the current flap cycle for both the LAG
            interface and its subinterface.

#### Sub-tests:

*   **LAGInterfaceFlap**: The LAG interface flap is triggered by updating the
    `/interfaces/interface[name=<lag>]/config/enabled` path on the DUT via gNMI.

*   **LAGMemberFlap**: The LAG state change is triggered by flapping the
    `enabled` state of the *member port(s)* of the LAG on the DUT via gNMI.

*   **OTGLAGFlap**: An ATE port is configured as a member of a LAG on the OTG.
    The LAG state change on the DUT is triggered by changing the link state of
    the ATE member port using OTG controls.

*   **LaserCutFlap**: The LAG state change is triggered by simulating a "laser
    cut" on one of the member ports. This is done by disabling the transmit
    laser on the DUT member port using the path
    `/components/component/transceiver/physical-channels/channel/config/tx-laser`.

    [TODO] Implement when OC path is supported.

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
  /interfaces/interface/aggregation/config/lag-type:
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/name:
  /interfaces/interface/config/type:
  /interfaces/interface/ethernet/config/aggregate-id:
  /interfaces/interface/ethernet/config/auto-negotiate:
  /interfaces/interface/ethernet/config/duplex-mode:
  /interfaces/interface/ethernet/config/port-speed:
  /interfaces/interface/state/last-change:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/subinterfaces/subinterface/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/state/last-change:
rpcs:
  gnmi:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

* FFF - fixed form factor

