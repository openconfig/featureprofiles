# DP-1.3: QoS ECN feature config

## Summary

Verify QoS ECN feature configuration.

## Testbed type

*   [`2_router_links.testbed`](https://github.com/openconfig/featureprofiles/tree/main/topologies)

## Procedure

### Test environment setup

*   Connect DUT port-1 to ATE port-1, DUT port-2 to ATE port-2.
*   Create an input IPv4 classifier to match traffic intended for the QoS queue being tested (e.g., using `dscp` values).
*   Apply the classifier to the input of `DUT port-1` using `/qos/interfaces/interface/input/classifiers/classifier/config/name`.
*   ECN configuration parameters:
    *   This test verifies the DUT's ability to accept an ECN configuration with a vertical buffer utilization cut-off line. If the buffer is utilized below that cut-off value, no packet is ECN CE marked. If the buffer is utilized at or above that cut-off value, all packets are ECN CE marked.
    *   An ECN profile can be created for different queues. ECN profiles per queue can be applied to the output side of interfaces.
    *   6.25MB max-threshold is selected as it represents ~500 micro-seconds of Delay bandwidth Buffer on 100GE interfaces. This is O(2%) of buffer depth, hence allows for micro-burst absorption without backpressing senders and at the same time leaves enough DBB to accommodate RTT ECN signaling loop delay in the global network for longer burst/congestion.

### DP-1.3.1 - 80KB min-threshold equal max-threshold

*   Step 1 - Generate DUT configuration: Configure a `queue-management-profile` with the following ECN parameters. Attach this profile to the target queue (e.g., queue `0`) on the output of `DUT port-1`.
    *   `min-threshold`: `81920` (80KB)
    *   `max-threshold`: `81920` (80KB)
    *   `enable-ecn`: `true`
    *   `drop`: `false`
    *   `max-drop-probability-percent`: `100`

#### Canonical OC

```json
{
  "qos": {
    "queue-management-profiles": {
      "queue-management-profile": [
        {
          "name": "ECN_PROFILE_1",
          "config": {
            "name": "ECN_PROFILE_1"
          },
          "wred": {
            "uniform": {
              "config": {
                "min-threshold": "81920",
                "max-threshold": "81920",
                "enable-ecn": true,
                "drop": false,
                "max-drop-probability-percent": 100
              }
            }
          }
        }
      ]
    },
    "interfaces": {
      "interface": [
        {
          "interface-id": "Ethernet1",
          "config": {
            "interface-id": "Ethernet1"
          },
          "output": {
            "queues": {
              "queue": [
                {
                  "name": "0",
                  "config": {
                    "name": "0",
                    "queue-management-profile": "ECN_PROFILE_1"
                  }
                }
              ]
            }
          }
        }
      ]
    }
  }
}
```

*   Step 2 - Push configuration to DUT using gNMI Set with REPLACE option.
*   Step 3 - Validation with pass/fail criteria: Validate that the profile is created and the values are set as expected using gNMI Get on the state paths.
    *   Verify `/qos/queue-management-profiles/queue-management-profile[name=ECN_PROFILE_1]/wred/uniform/state/min-threshold` == `81920`
    *   Verify `/qos/queue-management-profiles/queue-management-profile[name=ECN_PROFILE_1]/wred/uniform/state/max-threshold` == `81920`
    *   Verify `/qos/queue-management-profiles/queue-management-profile[name=ECN_PROFILE_1]/wred/uniform/state/enable-ecn` == `true`
    *   Verify `/qos/queue-management-profiles/queue-management-profile[name=ECN_PROFILE_1]/wred/uniform/state/drop` == `false`
    *   Verify `/qos/queue-management-profiles/queue-management-profile[name=ECN_PROFILE_1]/wred/uniform/state/max-drop-probability-percent` == `100`
*   Step 4 - Validate ECN profile application: Verify that the profile is successfully applied to the output interface queue.
    *   Verify `/qos/interfaces/interface[interface-id=Ethernet1]/output/queues/queue[name=0]/state/queue-management-profile` == `ECN_PROFILE_1`
*   Step 5 - Trigger a supervisor switchover using gNOI `SwitchControlProcessor`.
*   Step 6 - Once the new supervisor is active, repeat the gNMI Get checks from Step 3 and 4 to verify the configuration persisted.

### DP-1.3.2 - Threshold in MB, min-threshold not-equal max-threshold

*   Step 1 - Generate DUT configuration: Configure a `queue-management-profile` with the following ECN parameters. Attach this profile to the target queue on the output of `DUT port-1`.
    *   `min-threshold`: `3276800` (3.125MB)
    *   `max-threshold`: `6553600` (6.250MB)
    *   `enable-ecn`: `true`
    *   `drop`: `false`
    *   `max-drop-probability-percent`: `100`
*   Step 2 - Push configuration to DUT using gNMI Set with REPLACE option.
*   Step 3 - Validation with pass/fail criteria: Validate that the profile is created and the values are set as expected using gNMI Get on the state paths.
    *   Verify `/qos/queue-management-profiles/queue-management-profile[name=ECN_PROFILE_2]/wred/uniform/state/min-threshold` == `3276800`
    *   Verify `/qos/queue-management-profiles/queue-management-profile[name=ECN_PROFILE_2]/wred/uniform/state/max-threshold` == `6553600`
    *   Verify `/qos/queue-management-profiles/queue-management-profile[name=ECN_PROFILE_2]/wred/uniform/state/enable-ecn` == `true`
    *   Verify `/qos/queue-management-profiles/queue-management-profile[name=ECN_PROFILE_2]/wred/uniform/state/drop` == `false`
    *   Verify `/qos/queue-management-profiles/queue-management-profile[name=ECN_PROFILE_2]/wred/uniform/state/max-drop-probability-percent` == `100`
*   Step 4 - Validate ECN profile application: Verify that the profile is successfully applied to the output interface queue.
    *   Verify `/qos/interfaces/interface[interface-id=Ethernet1]/output/queues/queue[name=0]/state/queue-management-profile` == `ECN_PROFILE_2`

### DP-1.3.3 - Threshold in percentage, min-threshold not-equal max-threshold

*   Step 1 - Generate DUT configuration: Configure a `queue-management-profile` with the following ECN parameters. Attach this profile to the target queue on the output of `DUT port-1`.
    *   `min-threshold-percent`: `1` (1%)
    *   `max-threshold-percent`: `2` (2%)
    *   `enable-ecn`: `true`
    *   `drop`: `false`
    *   `max-drop-probability-percent`: `100`
*   Step 2 - Push configuration to DUT using gNMI Set with REPLACE option.
*   Step 3 - Validation with pass/fail criteria: Validate that the profile is created and the values are set as expected using gNMI Get on the state paths.
    *   Verify `/qos/queue-management-profiles/queue-management-profile[name=ECN_PROFILE_3]/wred/uniform/state/min-threshold-percent` == `1`
    *   Verify `/qos/queue-management-profiles/queue-management-profile[name=ECN_PROFILE_3]/wred/uniform/state/max-threshold-percent` == `2`
    *   Verify `/qos/queue-management-profiles/queue-management-profile[name=ECN_PROFILE_3]/wred/uniform/state/enable-ecn` == `true`
    *   Verify `/qos/queue-management-profiles/queue-management-profile[name=ECN_PROFILE_3]/wred/uniform/state/drop` == `false`
    *   Verify `/qos/queue-management-profiles/queue-management-profile[name=ECN_PROFILE_3]/wred/uniform/state/max-drop-probability-percent` == `100`
*   Step 4 - Validate ECN profile application: Verify that the profile is successfully applied to the output interface queue.
    *   Verify `/qos/interfaces/interface[interface-id=Ethernet1]/output/queues/queue[name=0]/state/queue-management-profile` == `ECN_PROFILE_3`

### DP-1.3.4 - Negative Test Cases

Ensure that invalid configurations are properly rejected by the DUT.

*   **Negative Test 1 (`min-threshold` > `max-threshold`):** Attempt to configure a `queue-management-profile` where `min-threshold` is strictly greater than `max-threshold` (e.g., `min-threshold` = `81920` and `max-threshold` = `40960`). Verify the gNMI Set is rejected.
*   **Negative Test 2 (Invalid `max-drop-probability-percent`):** Attempt to configure `max-drop-probability-percent` to an out-of-range value (e.g., `101`). Verify the gNMI Set is rejected.
*   **Negative Test 3 (Non-existent Profile Assignment):** Attempt to assign a non-existent `queue-management-profile` string (e.g., `BOGUS_PROFILE`) to `/qos/interfaces/interface/output/queues/queue/config/queue-management-profile`. Verify the gNMI Set is rejected.
*   **Negative Test 4 (Invalid Profile Deletion):** Apply a valid `queue-management-profile` to an interface's queue. Then, attempt to delete that `queue-management-profile` while it is still actively applied to the interface's queue. Verify the deletion is rejected.

### DP-1.3.5 - Teardown and Cleanup Verification

*   Step 1 - Detach the `queue-management-profile` from the output queue using gNMI Delete on `/qos/interfaces/interface/output/queues/queue/config/queue-management-profile` or by sending a Replace with an empty profile.
*   Step 2 - Validate the profile is detached by verifying the state path no longer returns the profile name.
*   Step 3 - Delete the `queue-management-profile` entirely using gNMI Delete on `/qos/queue-management-profiles/queue-management-profile[name=ECN_PROFILE_1]`.
*   Step 4 - Validate the profile is removed by verifying a gNMI Get on the profile path returns an error or empty result.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC paths used for test setup are not listed here.

```yaml
paths:
  ## Config paths
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/min-threshold:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-threshold:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/min-threshold-percent:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-threshold-percent:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/enable-ecn:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/weight:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/drop:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-drop-probability-percent:
  /qos/interfaces/interface/input/classifiers/classifier/config/name:
  /qos/interfaces/interface/output/queues/queue/config/name:
  /qos/interfaces/interface/output/queues/queue/config/queue-management-profile:

  ## State paths:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/state/min-threshold:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/state/max-threshold:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/state/min-threshold-percent:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/state/max-threshold-percent:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/state/enable-ecn:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/state/weight:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/state/drop:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/state/max-drop-probability-percent:
  /qos/interfaces/interface/input/classifiers/classifier/state/name:
  /qos/interfaces/interface/output/queues/queue/state/name:
  /qos/interfaces/interface/output/queues/queue/state/queue-management-profile:

rpcs:
  gnmi:
    gNMI.Set:
      Replace:
    gNMI.Get:
  gnoi:
    system.System.SwitchControlProcessor:
```

## Required DUT platform

*   vRX