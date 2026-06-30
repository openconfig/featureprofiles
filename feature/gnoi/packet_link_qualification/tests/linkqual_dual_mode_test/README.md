# gNOI-2.2: Single-Chassis Dual-Mode Link Qualification

## Summary

Validate that a single DUT can successfully run concurrent gNOI Link Qualification sessions across multiple ports on the same chassis. This test verifies that the device can simultaneously host both generator (`NEAR_END`) and reflector (`FAR_END`) roles over physical loopback cables, ensuring zero packet drops, stable agent execution, and complete hardware resource cleanup without affecting standard forwarding.

This test expands coverage to high-speed interfaces (400G and 800G) to ensure the hardware Service Activation Test (SAT) engines can scale and co-exist without resource leaks or software hangs.

*   TODO: Add coverage for 1.6T and 100G ports once hardware and transceiver support is available.

## Topology

The test requires a single DUT with loopback pairs configured for the supported port speeds.
*   **800G Loopback Pair**: `port1` <--> `port2` (cabled back-to-back, 800G transceiver/optics)
*   **400G Loopback Pair**: `port3` <--> `port4` (cabled back-to-back, 400G transceiver/optics)

All ports can be tested as singleton interfaces or as member links of a Link Aggregation Group (LAG).

## Procedure

### 1. System Health Pre-Check
*   Query `/system/alarms/alarm/state/id` and `/system/alarms/alarm/state/text` via gNMI and verify no active alarms exist.
*   Check `/var/core` (or vendor equivalent) to ensure no pre-existing core dumps exist for the system management or link qualification agents (e.g., `SandOam` on Arista).

### 2. Capabilities Validation
*   Issue `gnoi.LinkQualification.Capabilities` to the DUT.
*   Verify the following capabilities are supported:
    *   `MaxHistoricalResultsPerInterface` is >= 2.
    *   `Generator`:
        *   `MinMtu` <= 64, `MaxMtu` >= 8184.
        *   `MaxBps` >= 8e11 (to support 800G line rate).
        *   `MaxPps` >= 1e9.
        *   `MinSetupDuration` > 0, `MinTeardownDuration` > 0, `MinSampleInterval` > 0.
    *   `Reflector`:
        *   Supports both `AsicLoopback` and `PmdLoopback` configurations.
        *   `MinSetupDuration` > 0, `MinTeardownDuration` > 0.

### 3. Error Handling and Stale Session Cleanup
*   Verify that `Get` and `Delete` requests with a non-existing ID return `5 NOT_FOUND`.
*   Issue `gnoi.LinkQualification.List` to retrieve all active qualifications.
*   For any active qualification found, issue `gnoi.LinkQualification.Delete` to ensure the hardware SAT engine is fully cleared before starting the test.
*   Verify via `List` that the active qualifications count is 0.

### 4. Concurrent Dual-Mode Execution (Tested per speed pair)
For each loopback pair (800G, 400G):
1.  **Configure Reflector (FAR_END)**:
    *   Issue `gnoi.LinkQualification.Create` on the reflector port (e.g., `port2` for 800G) with `EndpointType` set to `FAR_END` (using `AsicLoopback` or `PmdLoopback` based on capabilities).
    *   Set the synchronized timing parameters (`Duration`, `PreSyncDuration`, `SetupDuration`, `PostSyncDuration`, `TeardownDuration`).
    *   Verify via gNMI that `/interfaces/interface[name=port2]/state/oper-status` transitions to `TESTING`.
2.  **Configure Generator (NEAR_END)**:
    *   Issue `gnoi.LinkQualification.Create` on the generator port (e.g., `port1` for 800G) with `EndpointType` set to `NEAR_END` (`PacketGeneratorConfiguration`).
    *   Set the `PacketRate` to the maximum supported by the port speed and `PacketSize` to the maximum supported MTU (e.g., 8184 bytes).
    *   Apply the same synchronized timing parameters as the reflector.
    *   Verify via gNMI that `/interfaces/interface[name=port1]/state/oper-status` transitions to `TESTING`.
3.  **Concurrent Verification**:
    *   Verify that both the generator and reflector sessions are running concurrently on the same chassis.
    *   Verify that the physical link remains up and the hardware SAT engine is actively transmitting/reflecting traffic.

### 5. Agent Health Monitoring
*   While the qualifications are in the `QUALIFICATION_STATE_RUNNING` state:
    *   Query `/system/processes/process/state/name` and `/system/processes/process/state/start-time` via gNMI.
    *   Assert that the link qualification daemon (e.g., `SandOam`) remains `RUNNING` and its `start-time` does not change (confirming no crashes or restarts occurred).
    *   Subscribe to `/system/messages/state/message/msg` and assert that no logs match `EXCESSIVE_WARMUP_DELAY` or `STARTUP_FAILED` for the link qualification agent.

### 6. Results Verification
*   Wait for the sessions to transition to `QUALIFICATION_STATE_COMPLETED`.
*   Issue `gnoi.LinkQualification.Get` for all active session IDs.
*   Verify the response for each session:
    *   `status.code` is 0 (Success).
    *   `packets_sent` matches `packets_received` (within the vendor-defined tolerance, e.g., 0.0001%).
    *   `num_corrupt_packets` is 0.
    *   `num_packets_dropped_by_mmu` is 0.

### 7. Post-Test System Health Audit
*   Query `/system/alarms/alarm/state/id` and verify no new alarms (such as hardware resource leaks or transceiver faults) were raised during the test.
*   Verify no new core dumps exist in `/var/core`.

### 8. Cleanup and Restoration
*   Issue `gnoi.LinkQualification.Delete` for all session IDs.
*   Verify via gNMI that all interfaces transition back from `TESTING` to their pre-test operational state (e.g., `UP`).
*   Verify that standard routing and forwarding on the interfaces resume normally.
*   Verify via `List` that all session IDs have been cleared.

## Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "name": "port1",
        "config": {
          "name": "port1",
          "type": "iana-if-type:ethernetCsmacd",
          "enabled": true,
          "mtu": 9000
        },
        "ethernet": {
          "config": {
            "aggregate-id": "port-channel1"
          }
        }
      },
      {
        "name": "port-channel1",
        "config": {
          "name": "port-channel1",
          "type": "iana-if-type:ieee8023adLag",
          "enabled": true,
          "mtu": 9000
        },
        "aggregation": {
          "config": {
            "lag-type": "STATIC"
          }
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "config": {
                "index": 0,
                "enabled": true
              },
              "ipv4": {
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

The below yaml defines the OC paths and RPCs covered by this test.

```yaml
paths:
  /interfaces/interface/state/oper-status:
  /system/alarms/alarm/state/id:
  /system/alarms/alarm/state/text:
  /system/processes/process/state/name:
  /system/processes/process/state/start-time:
  /system/messages/state/message/msg:
  /system/messages/state/message/app-name:
rpcs:
  gnoi:
    packet_link_qualification.LinkQualification.Capabilities:
    packet_link_qualification.LinkQualification.Create:
    packet_link_qualification.LinkQualification.Delete:
    packet_link_qualification.LinkQualification.Get:
    packet_link_qualification.LinkQualification.List:
```

