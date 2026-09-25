# MGT-2: Dual-Management SSH Reachability Validation

## Summary

Verify SSH reachability to the DUT via both Inband (typically over a LAG
interface) and Out-of-Band (OOB) management paths within the management VRF.
Ensure that the device remains accessible via both paths, the management VRF
remains intact, and the LAG interface remains routable and unaffected after a
configuration push (simulating a Zero Touch Configuration (ZTC) deployment).

## Testbed type

*   A topology supporting both OOB and Inband management access.
*   Typically requires:
    *   DUT OOB management port connected to the management network (pre-configured).
    *   DUT ports connected to the ATE configured as a Link Aggregation Group (LAG/Port-Channel) for the Inband management path.
    *   Test runner (where the test executes) must have IP connectivity to both the DUT's OOB IP and the Inband IP configured on the LAG.

## Procedure

### 1. Initial Setup

*   **Management VRF & OOB**:
    *   Assumed to be pre-configured by the infrastructure (using `configlet.config` or base config). The `mgmt` VRF (or vendor-specific management VRF) exists, and the OOB interface is associated with it and has `/interfaces/interface/config/management` set to `true`.
*   **Configure Inband management interface (LAG)**:
    *   Create a LAG interface (type `ieee8023adLag`).
    *   Configure physical ports as members of the LAG.
    *   Configure IP address on the LAG interface.
    *   Associate the LAG interface with the existing `mgmt` VRF.
    *   Configure routing (static routes or routing protocols like BGP) in the `mgmt` VRF to ensure that the test runner can route traffic to this Inband IP via the ATE.
*   **SSH Server**:
    *   Assumed to be pre-configured/enabled by the infrastructure.

### Canonical OC for DUT configuration

This section should contain a JSON formatted stanza representing the
canonical OC to configure the dual-management SSH reachability. (See the
[README Template](https://github.com/openconfig/featureprofiles/blob/main/doc/test-requirements-template.md#procedure))

<!-- disableFinding(LINE_OVER_80) -->
```json
{
  "openconfig-system:system": {
    "ssh-server": {
      "config": {
        "enable": true
      }
    }
  },
  "openconfig-interfaces:interfaces": {
    "interface": [
      {
        "name": "Ethernet1",
        "config": {
          "name": "Ethernet1",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "ethernet": {
          "config": {
            "aggregate-id": "Port-Channel1"
          }
        }
      },
      {
        "name": "Port-Channel1",
        "config": {
          "name": "Port-Channel1",
          "type": "iana-if-type:ieee8023adLag"
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "config": {
                "index": 0
              },
              "openconfig-if-ip:ipv4": {
                "addresses": {
                  "address": [
                    {
                      "ip": "192.0.2.1",
                      "config": {
                        "ip": "192.0.2.1",
                        "prefix-length": 30
                      }
                    }
                  ]
                }
              }
            }
          ]
        }
      },
      {
        "name": "Management0",
        "config": {
          "name": "Management0",
          "type": "iana-if-type:ethernetCsmacd",
          "management": true
        }
      }
    ]
  },
  "openconfig-network-instance:network-instances": {
    "network-instance": [
      {
        "name": "mgmt",
        "config": {
          "name": "mgmt",
          "type": "openconfig-network-instance-types:L3VRF"
        },
        "interfaces": {
          "interface": [
            {
              "id": "Management0",
              "config": {
                "id": "Management0",
                "interface": "Management0",
                "subinterface": 0
              }
            },
            {
              "id": "Port-Channel1.0",
              "config": {
                "id": "Port-Channel1.0",
                "interface": "Port-Channel1",
                "subinterface": 0
              }
            }
          ]
        }
      }
    ]
  }
}
```

### 2. Verify Initial State and SSH Reachability

*   **Step 1**: Read and store the configuration and operational state of:
    *   Management VRF (`mgmt`).
    *   OOB management interface.
    *   Inband LAG interface (including member ports status).
*   **Step 2**: Verify the Inband LAG interface is up and active (`oper-status` is `UP`).
*   **Step 3**: Establish an SSH session from the test runner to the DUT's OOB IP.
    *   Assert that the connection is successful.
*   **Step 4**: Establish an SSH session from the test runner to the DUT's Inband IP (on the LAG).
    *   Assert that the connection is successful.

### 3. Simulating ZTC / Config Push

*   **Step 5**: Prepare a configuration push that simulates a typical ZTC update.
*   **Step 6**: Apply this configuration to the DUT using gNMI Set (Replace).
    *   Ensure that this configuration push does NOT contain any changes that would disable SSH, the `mgmt` VRF, or the management interfaces (OOB and Inband LAG).

### 4. Verify Post-Config Push State and SSH Reachability

*   **Step 7**: Read the configuration and operational state of the `mgmt` VRF, OOB interface, and Inband LAG interface, and compare them with the stored pre-push values.
    *   Assert that the configuration and state remain intact and unchanged.
*   **Step 8**: Verify the Inband LAG interface remains up and active.
*   **Step 9**: Establish an SSH session from the test runner to the DUT's OOB IP.
    *   Assert that the connection is successful.
*   **Step 10**: Establish an SSH session from the test runner to the DUT's Inband IP.
    *   Assert that the connection is successful.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
  ## Config paths
  /system/ssh-server/config/enable:
  /interfaces/interface/config/name:
  /interfaces/interface/config/type:
  /interfaces/interface/config/management:
  /interfaces/interface/ethernet/config/aggregate-id:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /network-instances/network-instance/config/name:
  /network-instances/network-instance/config/type:
  /network-instances/network-instance/interfaces/interface/config/interface:

  ## State paths
  /system/ssh-server/state/enable:
  /interfaces/interface/state/management:
  /interfaces/interface/state/oper-status:

rpcs:
  gnmi:
    gNMI.Set:
      Replace:
```

## Minimum DUT platform requirement

* FFF

