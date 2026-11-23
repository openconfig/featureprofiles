# SYS-5.1: Configuration Commit Validation after Large gNMI-Set and reboot in parallel

## Summary

Perform a large gNMI Set and immediately reboot the device to verify that the
configuration is in place after the device comes up and the configuration
database is functional and unlocked.

## Testbed type

*   [`featureprofiles/topologies/dut.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut.testbed)

## Procedure

### Test environment setup

*   Configure LAG interfaces on unused physical interfaces of the DUT, in the
    following proportions:
    *   Up to 200 LAG interfaces with 2 member interfaces each. The 2 members
        of a LAG must have the same port speed.
    *   Up to 300 LAG interfaces with 1 member interface each.
    *   At full scale, this is 500 LAG interfaces and 700 member/physical
        interfaces.
*   If the DUT does not have enough unused physical interfaces or LAG
    interfaces for the full scale, reduce the number of LAGs while keeping the
    2:3 proportion above, and log the reduced scale.
*   The test must configure at least 8 LAG interfaces with 2 member interfaces
    and 12 LAG interfaces with 1 member interface. If the DUT cannot provide
    this, the test fails instead of running at a smaller scale.
*   The LAG count scales with the DUT. The 700 VLAN subinterfaces of SYS-5.1.1
    keep the size of the large gNMI Set fixed regardless of the DUT's port
    count.

### SYS-5.1.1 - gNMI Batch Set and reboot immediately

*   **Step 1:** Create a gNMI batch configuration to:
    *   Configure a description on all the physical member and LAG interfaces
        of the test environment.
    *   Configure 700 802.1Q VLAN subinterfaces, with VLAN IDs 100 to 799,
        distributed round-robin across the LAG interfaces. VLAN IDs start at
        100 to avoid VLAN 1, the default or native VLAN on several platforms.
    *   Configure one IPv4 address (/30) and one IPv6 address (/126) on each of
        the 700 VLAN subinterfaces, which is 1,400 IP addresses. Each VLAN
        subinterface is in its own IPv4 and IPv6 subnet.
*   **Step 2:** In a Goroutine, use gNMI Batch Set to set the configuration on
    the DUT.
*   **Step 3:** In a separate Goroutine (running in parallel), trigger a
    complete chassis cold reboot using the `gnoi.system.System.Reboot` RPC with
    the method set to `COLD`. Send the reboot as soon as the gNMI Set request
    is sent, without waiting for the Set response, so that the reboot can
    interrupt the Set.
*   **NOTE:** `rpc error: code = Unavailable desc = transport is closing` or a
    similar message for the gNMI connection is acceptable since the reboot RPC
    response is not sent over the same connection from which the gNMI Set
    request was sent. The same connection errors are acceptable for the reboot
    RPC itself.
*   **Step 4:** Wait for the device to boot up and return to service. Always
    wait for the device, even if the gNMI Set or the reboot RPC returned an
    error, before validating or cleaning up the configuration.
*   Record whether the gNMI Set returned OK before the reboot or was cut off by
    the reboot. SYS-5.1.2 Step 2 depends on this outcome.

### SYS-5.1.2 - Post-reboot configuration validation

*   **Step 1:** Verify boot time using `/system/state/boot-time` and software
    versions using `/components/component/state/software-version`.
*   **Step 2:** Using gNMI Get, verify the interface descriptions, the 700 VLAN
    subinterfaces and their 1,400 IPv4/IPv6 addresses of SYS-5.1.1 Step 1 in
    the configuration:
    *   If the gNMI Set returned OK, all of this configuration must be present.
    *   If the reboot cut the gNMI Set off, either all of this configuration
        must be present, or none of it (the original configuration is intact).
    *   Any partial configuration, with some but not all of the changes
        applied, is a failure.
*   **Step 3:** Issue a gNMI Set request to configure a test description on any
    one DUT interface.
*   **Step 4:** Verify that the gNMI Set request succeeds (indicating the
    configuration database is unlocked and commits are functional).
*   **Step 5:** Verify the new description is applied correctly using a gNMI Get
    request, and that the DUT reports it in the state with a gNMI Subscribe
    `ON_CHANGE` subscription. If the `ON_CHANGE` subscription does not deliver
    the new description, the test fails.

## Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "name": "port-channel1",
        "config": {
          "name": "port-channel1",
          "description": "SYS-5.1 LAG port-channel1",
          "enabled": true
        },
        "aggregation": {
          "config": {
            "lag-type": "LACP"
          }
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 100,
              "config": {
                "index": 100
              },
              "vlan": {
                "match": {
                  "single-tagged": {
                    "config": {
                      "vlan-id": 100
                    }
                  }
                }
              },
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "ip": "198.18.0.1",
                      "config": {
                        "ip": "198.18.0.1",
                        "prefix-length": 30
                      }
                    }
                  ]
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "ip": "2001:db8:5:1::1",
                      "config": {
                        "ip": "2001:db8:5:1::1",
                        "prefix-length": 126
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
        "name": "port1",
        "config": {
          "name": "port1",
          "description": "SYS-5.1 member port1 of port-channel1",
          "enabled": true
        },
        "ethernet": {
          "config": {
            "aggregate-id": "port-channel1"
          }
        }
      },
      {
        "name": "port2",
        "config": {
          "name": "port2",
          "description": "SYS-5.1 member port2 of port-channel1",
          "enabled": true
        },
        "ethernet": {
          "config": {
            "aggregate-id": "port-channel1"
          }
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
  ## Configuration paths
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/name:
  /interfaces/interface/ethernet/config/aggregate-id:
  /interfaces/interface/aggregation/config/lag-type:
  /interfaces/interface/subinterfaces/subinterface/config/index:
  /interfaces/interface/subinterfaces/subinterface/vlan/match/single-tagged/config/vlan-id:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:

  ## State paths
  /interfaces/interface/state/description:
  /system/state/boot-time:
  /components/component/state/software-version:
    platform_type: ["BIOS", "BOOT_LOADER", "OPERATING_SYSTEM"]

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
      update: true
    gNMI.Subscribe:
      on_change: true
  gnoi:
    system.System.Reboot:
```

## Required DUT platform

*   MFF
