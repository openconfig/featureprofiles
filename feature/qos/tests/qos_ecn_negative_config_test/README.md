# DP-1.20: QoS ECN negative feature config

## Summary

*   **Objective:** Validate that the DUT properly rejects invalid OpenConfig QoS
    Explicit Congestion Notification (ECN) and Weighted Random Early Detection
    (WRED) `queue-management-profile` configurations and enforces referential
    integrity when profiles are assigned to or deleted from interface output
    queues.
*   **Interfaces Used:** gNMI (`gNMI.Set`, `gNMI.Get`, `gNMI.Subscribe`).
*   **Expected Outcome Summary:**
    *   Invalid WRED/ECN profile parameters (`min-threshold > max-threshold` and
        out-of-range `max-drop-probability-percent`) are rejected with a gNMI
        `Set` error.
    *   Referencing a non-existent `queue-management-profile` on an interface
        output queue is rejected with a gNMI `Set` error.
    *   Deleting a `queue-management-profile` while it is actively bound to an
        interface output queue is rejected, leaving the active binding intact
        until properly detached during teardown.

## Testbed type

*   [`2_router_links.testbed`](https://github.com/openconfig/featureprofiles/tree/main/topologies)

## Procedure

### Topology

```text
  ATE port 1 (192.0.2.2/30, 2001:db8::2/126)
      |
      | (Ethernet1: 192.0.2.1/30, 2001:db8::1/126)
     DUT
      | (Ethernet2: 192.0.2.5/30, 2001:db8::5/126)
      |
  ATE port 2 (192.0.2.6/30, 2001:db8::6/126)
```

| Entity | Name / ID | Parameters |
| :--- | :--- | :--- |
| DUT Ingress / Egress Port 1 | `Ethernet1` (`DUT port-1`) | IPv4: `192.0.2.1/30`, IPv6: `2001:db8::1/126` |
| DUT Ingress / Egress Port 2 | `Ethernet2` (`DUT port-2`) | IPv4: `192.0.2.5/30`, IPv6: `2001:db8::5/126` |
| QoS Queues | `0` (`BE1`), `1` (`AF1`) | `/qos/queues/queue/config/name` |
| Forwarding Groups | `target-group-BE1`, `target-group-AF1` | Output queues: `0` and `1` |
| Input Classifiers | `dscp_based_classifier_ipv4`, `dscp_based_classifier_ipv6` | `IPV4` and `IPV6`; DSCP `[0, 1, 2, 3]` -> `target-group-BE1`, DSCP `[8, 9, 10, 11]` -> `target-group-AF1` |
| Baseline Profile Defaults | `ECN_VALID_PROFILE` / `ECN_PROFILE_NEG` | `min-threshold`: `81920`, `max-threshold`: `163840`, `enable-ecn`: `true`, `drop`: `false`, `weight`: `0`, `max-drop-probability-percent`: `100` |

### Test environment setup

*   Step 1 - Interface Configuration: Connect `DUT port-1` (`Ethernet1`) to
    `ATE port 1` and `DUT port-2` (`Ethernet2`) to `ATE port 2`, and configure
    the IPv4 and IPv6 addresses defined in the Topology table.
*   Step 2 - Queues and Forwarding Groups: Configure queues `0` and `1` under
    `/qos/queues/queue/config/name` and map forwarding groups
    `target-group-BE1` (to queue `0`) and `target-group-AF1` (to queue `1`)
    under `/qos/forwarding-groups/forwarding-group/config/output-queue`.
*   Step 3 - Dual-Stack Input Classifiers:
    *   Configure `dscp_based_classifier_ipv4`
        (`/qos/classifiers/classifier/config/type` = `IPV4`) matching
        `/qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set`
        and assigning
        `/qos/classifiers/classifier/terms/term/actions/config/target-group` as
        defined in the Topology table.
    *   Configure `dscp_based_classifier_ipv6`
        (`/qos/classifiers/classifier/config/type` = `IPV6`) matching
        `/qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set`
        and assigning
        `/qos/classifiers/classifier/terms/term/actions/config/target-group` as
        defined in the Topology table.
    *   Attach both classifiers to the input of `Ethernet1` and `Ethernet2`
        using
        `/qos/interfaces/interface/input/classifiers/classifier/config/name`
        and
        `/qos/interfaces/interface/input/classifiers/classifier/config/type`.
    *   Verify classifier attachment using gNMI `Subscribe` (`Watch`) on
        `/qos/interfaces/interface/input/classifiers/classifier/state/name`.
*   Step 4 - Baseline Profile Template for Negative Tests: Each negative profile
    test (`DP-1.20.1` and `DP-1.20.2`) starts with all mandatory leaves
    populated from the Baseline Profile Defaults in the Topology table and
    overrides only the single parameter under test so rejection is caused
    strictly by the invalid parameter and not by missing sibling leaves.

### DP-1.20.1 - Negative Test 1: min-threshold Strictly Greater Than max-threshold

Verify that the DUT rejects a `queue-management-profile` where `min-threshold`
is strictly greater than `max-threshold`. Because some platforms validate
cross-leaf threshold ordering when the profile is bound to a queue, include the
output queue binding in the same gNMI `Set` request.

*   Step 1 - Construct `ECN_PROFILE_NEG` using the Baseline Profile Defaults and
    override the threshold leaves so `min-threshold > max-threshold`:
    *   `/qos/queue-management-profiles/queue-management-profile[name=ECN_PROFILE_NEG]/wred/uniform/config/min-threshold`:
        `81920` (80 KB)
    *   `/qos/queue-management-profiles/queue-management-profile[name=ECN_PROFILE_NEG]/wred/uniform/config/max-threshold`:
        `40960` (40 KB)
    *   `/qos/interfaces/interface[interface-id=Ethernet1]/output/queues/queue[name=0]/config/queue-management-profile`:
        `ECN_PROFILE_NEG`
*   Step 2 - Push the configuration to the DUT using gNMI `Set` (`Update` /
    `Replace`).
*   Step 3 - Validation with pass/fail criteria:
    *   **Pass**: The gNMI `Set` RPC is rejected with a non-nil error.
    *   **Fail**: The gNMI `Set` RPC succeeds without error.

### DP-1.20.2 - Negative Test 2: Invalid max-drop-probability-percent Out-of-Range

Verify that the DUT rejects a `queue-management-profile` when
`max-drop-probability-percent` is outside the valid percentage range `0..100`.

*   Step 1 - Construct `ECN_PROFILE_NEG` using the Baseline Profile Defaults and
    override `max-drop-probability-percent` to an out-of-range value:
    *   `/qos/queue-management-profiles/queue-management-profile[name=ECN_PROFILE_NEG]/wred/uniform/config/max-drop-probability-percent`:
        `101`
*   Step 2 - Push the configuration to the DUT using gNMI `Set` (`Update` /
    `Replace`).
*   Step 3 - Validation with pass/fail criteria:
    *   **Pass**: The gNMI `Set` RPC is rejected with a non-nil error.
    *   **Fail**: The gNMI `Set` RPC succeeds without error.

### DP-1.20.3 - Negative Test 3: Non-Existent Queue-Management-Profile Assignment

Verify that the DUT rejects assigning an undefined `queue-management-profile`
name (`BOGUS_PROFILE`) to an interface output queue.

*   Step 1 - Construct an interface output queue configuration on `Ethernet1`
    queue `0` referencing `BOGUS_PROFILE` (which does not exist under
    `/qos/queue-management-profiles/queue-management-profile`):
    *   `/qos/interfaces/interface[interface-id=Ethernet1]/output/queues/queue[name=0]/config/name`:
        `0`
    *   `/qos/interfaces/interface[interface-id=Ethernet1]/output/queues/queue[name=0]/config/queue-management-profile`:
        `BOGUS_PROFILE`
*   Step 2 - Push the configuration to the DUT using gNMI `Set` (`Update` /
    `Replace`).
*   Step 3 - Validation with pass/fail criteria:
    *   **Pass**: The gNMI `Set` RPC is rejected with a non-nil error.
    *   **Fail**: The gNMI `Set` RPC succeeds without error.

### DP-1.20.4 - Negative Test 4: Invalid Profile Deletion While Actively Applied

Verify that the DUT rejects deleting a valid `queue-management-profile` while it
remains actively applied to an interface output queue, and preserves the active
queue binding.

*   Step 1 - Configure `ECN_VALID_PROFILE` using the Baseline Profile Defaults
    and attach it to `Ethernet1` output queue `0` via gNMI `Set` (`Replace`).

#### Canonical OC

```json
{
  "qos": {
    "classifiers": {
      "classifier": [
        {
          "config": {
            "name": "dscp_based_classifier_ipv4",
            "type": "IPV4"
          },
          "name": "dscp_based_classifier_ipv4",
          "terms": {
            "term": [
              {
                "actions": {
                  "config": {
                    "target-group": "target-group-BE1"
                  }
                },
                "conditions": {
                  "ipv4": {
                    "config": {
                      "dscp-set": [
                        0,
                        1,
                        2,
                        3
                      ]
                    }
                  }
                },
                "config": {
                  "id": "0"
                },
                "id": "0"
              }
            ]
          }
        },
        {
          "config": {
            "name": "dscp_based_classifier_ipv6",
            "type": "IPV6"
          },
          "name": "dscp_based_classifier_ipv6",
          "terms": {
            "term": [
              {
                "actions": {
                  "config": {
                    "target-group": "target-group-BE1"
                  }
                },
                "conditions": {
                  "ipv6": {
                    "config": {
                      "dscp-set": [
                        0,
                        1,
                        2,
                        3
                      ]
                    }
                  }
                },
                "config": {
                  "id": "0"
                },
                "id": "0"
              }
            ]
          }
        }
      ]
    },
    "forwarding-groups": {
      "forwarding-group": [
        {
          "config": {
            "name": "target-group-BE1",
            "output-queue": "0"
          },
          "name": "target-group-BE1"
        }
      ]
    },
    "interfaces": {
      "interface": [
        {
          "config": {
            "interface-id": "Ethernet1"
          },
          "input": {
            "classifiers": {
              "classifier": [
                {
                  "config": {
                    "name": "dscp_based_classifier_ipv4",
                    "type": "IPV4"
                  },
                  "type": "IPV4"
                },
                {
                  "config": {
                    "name": "dscp_based_classifier_ipv6",
                    "type": "IPV6"
                  },
                  "type": "IPV6"
                }
              ]
            }
          },
          "interface-id": "Ethernet1",
          "output": {
            "queues": {
              "queue": [
                {
                  "config": {
                    "name": "0",
                    "queue-management-profile": "ECN_VALID_PROFILE"
                  },
                  "name": "0"
                }
              ]
            }
          }
        }
      ]
    },
    "queue-management-profiles": {
      "queue-management-profile": [
        {
          "config": {
            "name": "ECN_VALID_PROFILE"
          },
          "name": "ECN_VALID_PROFILE",
          "wred": {
            "uniform": {
              "config": {
                "drop": false,
                "enable-ecn": true,
                "max-drop-probability-percent": 100,
                "max-threshold": "163840",
                "min-threshold": "81920"
              }
            }
          }
        }
      ]
    },
    "queues": {
      "queue": [
        {
          "config": {
            "name": "0"
          },
          "name": "0"
        }
      ]
    }
  }
}
```

*   Step 2 - Confirm `ECN_VALID_PROFILE` is active on `Ethernet1` queue `0`
    using gNMI `Subscribe` (`Watch`) on
    `/qos/interfaces/interface[interface-id=Ethernet1]/output/queues/queue[name=0]/state/queue-management-profile`
    (`== ECN_VALID_PROFILE`).
*   Step 3 - While `ECN_VALID_PROFILE` remains applied to `Ethernet1` queue `0`,
    attempt to delete
    `/qos/queue-management-profiles/queue-management-profile[name=ECN_VALID_PROFILE]`
    using gNMI `Set` (`Delete`).
*   Step 4 - Validation with pass/fail criteria:
    *   **Pass**: The gNMI `Set` (`Delete`) RPC is rejected with a non-nil
        error, and gNMI `Subscribe` (`Watch`) on
        `/qos/interfaces/interface[interface-id=Ethernet1]/output/queues/queue[name=0]/state/queue-management-profile`
        confirms `ECN_VALID_PROFILE` remains applied.
    *   **Fail**: The deletion succeeds while `ECN_VALID_PROFILE` is attached,
        or the active binding on `Ethernet1` queue `0` is removed.

### DP-1.20.5 - Teardown and Cleanup Verification

*   Step 1 - Detach `ECN_VALID_PROFILE` from `Ethernet1` output queue `0` using
    gNMI `Set` (`Delete`) on
    `/qos/interfaces/interface[interface-id=Ethernet1]/output/queues/queue[name=0]/config/queue-management-profile`.
*   Step 2 - Verify via gNMI `Subscribe` (`Watch`) on
    `/qos/interfaces/interface[interface-id=Ethernet1]/output/queues/queue[name=0]/state/queue-management-profile`
    that `ECN_VALID_PROFILE` is no longer attached.
*   Step 3 - Delete `ECN_VALID_PROFILE` using gNMI `Set` (`Delete`) on
    `/qos/queue-management-profiles/queue-management-profile[name=ECN_VALID_PROFILE]`.
*   Step 4 - Verify via gNMI `Subscribe` (`Watch`) on
    `/qos/queue-management-profiles/queue-management-profile[name=ECN_VALID_PROFILE]/state/name`
    that the profile is removed and the DUT is restored to its baseline state.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml
paths:
  ## Config paths
  /qos/queue-management-profiles/queue-management-profile/config/name:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/min-threshold:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-threshold:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/enable-ecn:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/weight:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/drop:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-drop-probability-percent:
  /qos/interfaces/interface/config/interface-id:
  /qos/interfaces/interface/input/classifiers/classifier/config/name:
  /qos/interfaces/interface/input/classifiers/classifier/config/type:
  /qos/interfaces/interface/output/queues/queue/config/name:
  /qos/interfaces/interface/output/queues/queue/config/queue-management-profile:

  ## State paths
  /qos/queue-management-profiles/queue-management-profile/state/name:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/state/min-threshold:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/state/max-threshold:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/state/enable-ecn:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/state/weight:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/state/drop:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/state/max-drop-probability-percent:
  /qos/interfaces/interface/input/classifiers/classifier/state/name:
  /qos/interfaces/interface/input/classifiers/classifier/state/type:
  /qos/interfaces/interface/output/queues/queue/state/name:
  /qos/interfaces/interface/output/queues/queue/state/queue-management-profile:

rpcs:
  gnmi:
    gNMI.Set:
      Replace:
    gNMI.Get:
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

*   vRX
