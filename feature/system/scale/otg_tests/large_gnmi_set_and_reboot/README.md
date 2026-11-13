# SYS-5.1: Configuration Commit Validation after Large gNMI-Set and reboot in parallel

## Summary

Perform a large gNMI Set and immediately reboot the device to verify that the configuration is in place after the device comes up and the configuration database is functional and unlocked.

## Testbed type

* [`featureprofiles/topologies/dut.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut.testbed)

## Procedure

### Test environment setup

*   Configure 200 LAG interfaces with 2 member interfaces each on the DUT.
*   Configure another 300 LAG interfaces with 1 member interface each on the DUT.
*   This results in a total of 500 LAG interfaces and 700 member/physical interfaces.

### SYS-5.1.1 - gNMI Batch Set and reboot immediately

*   **Step 1:** Create a gNMI batch configuration to:
    *   Configure description on all the 700 Physical and 500 LAG interfaces.
    *   Configure IPv4 and IPv6 Addresses on all the 500 LAG interfaces.
*   **Step 2:** In a Goroutine, use gNMI Batch Set to set the configuration on the DUT.
*   **Step 3:** In a separate Goroutine (running in parallel), trigger a complete chassis cold reboot using the `gnoi.system.System.Reboot` RPC with the method set to `COLD`.
*   **NOTE:** `rpc error: code = Unavailable desc = transport is closing` or a similar message for the gNMI connection is acceptable since the reboot RPC response is not sent over the same connection from which the gNMI Set request was sent.
*   **Step 4:** Wait for the device to boot up and return to service.

### SYS-5.1.2 - Post-reboot configuration validation

*   **Step 1:** Verify boot time using `/system/state/boot-time` and software versions using `/components/component/state/software-version`.
*   **Step 2:** Using gNMI Get, verify that the interface descriptions and IP addresses applied in SYS-5.1.1 are present in the configuration.
*   **Step 3:** Issue a gNMI Set request to configure a test description on any one DUT interface.
*   **Step 4:** Verify that the gNMI Set request succeeds (indicating the configuration database is unlocked and commits are functional).
*   **Step 5:** Verify the new description is applied correctly using a gNMI Get request.

## Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "name": "port-channel1",
        "config": {
          "name": "port-channel1",
          "enabled": true
        },
        "aggregation": {
          "config": {
            "lag-type": "LACP"
          }
        }
      },
      {
        "name": "port1",
        "config": {
          "name": "port1",
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
      union_replace: true
    gNMI.Subscribe:
      on_change: true
  gnoi:
    system.System.Reboot:
```

## Required DUT platform

* MFF
