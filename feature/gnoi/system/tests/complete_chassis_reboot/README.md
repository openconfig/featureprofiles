# gNOI-3.1: Complete Chassis Reboot

## Summary

Validate gNOI RPC can reboot the entire chassis under various conditions. This test establishes a pre-reboot baseline for system status, P4RT, and SSH sessions, and verifies that these states are successfully recovered and remain stable after a cold reboot without delay and a cold reboot with delay.

## Testbed type

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

### Test environment setup

*   Configure ATE port-1 connected to DUT port-1 with the relevant IPv4 and IPv6 addresses.
*   Configure P4RT node-id (device_id) for the linecard associated with port-1.

### gNOI-3.1.1 - Pre-reboot baseline validation

Establish and verify the baseline status of the system, P4RT, and SSH connectivity before triggering any reboot.

*   **Step 1:** Record the initial system boot-time using `/system/state/boot-time`.
*   **Step 2:** Retrieve and record the software version for all components using `/components/component/state/software-version`.
*   **Step 3:** Verify that all components have their operational status set to `OPER_STATUS_ACTIVE` using `/components/component/state/oper-status`.
*   **Step 4 (P4RT Baseline):** Establish a P4RT session from the controller to the DUT. Send a P4RT `ReadRequest` for port-1 counters and ensure a valid `ReadResponse` is received.
*   **Step 5 (SSH Baseline):** Establish an SSH connection to the DUT's management IP address. Run a basic command (e.g. show version) and verify the correct response is returned.

### gNOI-3.1.2 - Complete Chassis Cold Reboot without delay

Validate that the chassis reboots immediately upon request, and recovers all baseline system, P4RT, and SSH states successfully.

*   **Step 1:** Issue a gnoi.system `Reboot` RPC to the chassis with the method set to `COLD` and delay set to 0.
*   **Step 2:** Monitor telemetry and verify that the device becomes unreachable.
*   **Step 3:** Wait for the device to boot up and return to service.
*   **Step 4:** Verify that the system boot-time is newer than the recorded baseline boot-time.
*   **Step 5:** Validate that all components return to their baseline operational status.
*   **Step 6:** Validate that the component software versions match the baseline software versions.
*   **Step 7 (P4RT Validation - b/286086308):** Establish a new P4RT session to the DUT. Send a `ReadRequest` for port-1 counters and verify a valid `ReadResponse` is received.
*   **Step 8 (SSH Validation - b/399612422):** Establish a new SSH connection to the DUT's management IP. Verify the connection is successful, run basic commands, and maintain the session open for 5 minutes to ensure the connection remains stable and does not drop.

### gNOI-3.1.3 - Complete Chassis Cold Reboot with delay

Validate that the chassis remains reachable during the reboot delay, triggers reboot after the delay expires, and recovers all baseline system, P4RT, and SSH states successfully.

*   **Step 1:** Issue a gnoi.system `Reboot` RPC to the chassis with the method set to `COLD` and a populated delay of N seconds (e.g., 120 seconds).
*   **Step 2:** Verify that the system remains reachable and responsive to gNMI/gNOI requests for the duration of the N-second delay (e.g., by polling `/system/state/current-datetime`).
*   **Step 3:** Monitor the device until it becomes unreachable after the delay expires.
*   **Step 4:** Wait for the device to boot up and return to service.
*   **Step 5:** Verify that the system boot-time is newer than the boot-time from the previous reboot.
*   **Step 6:** Validate that all components return to their baseline operational status.
*   **Step 7:** Validate that the component software versions match the baseline software versions.
*   **Step 8 (P4RT Validation - b/286086308):** Establish a new P4RT session to the DUT. Send a `ReadRequest` for port-1 counters and verify a valid `ReadResponse` is received.
*   **Step 9 (SSH Validation - b/399612422):** Establish a new SSH connection to the DUT's management IP. Verify the connection is successful, run basic commands, and maintain the session open for 5 minutes to ensure the connection remains stable and does not drop.

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
            "node-id": "111"
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
        "name": "port1"
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC paths used for test setup are not listed here.

```yaml
paths:
  ## State paths
  /system/state/boot-time:
  /system/state/current-datetime:
  /components/component/state/oper-status:
    platform_type: ["CONTROLLER_CARD", "CPU", "FABRIC", "FAN", "LINECARD", "POWER_SUPPLY"]
  /components/component/state/software-version:
    platform_type: ["BIOS", "BOOT_LOADER", "OPERATING_SYSTEM"]
  /components/component/integrated-circuit/config/node-id:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /interfaces/interface/config/id:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
  gnoi:
    system.System.Reboot:
```

## Required DUT platform

* MFF
