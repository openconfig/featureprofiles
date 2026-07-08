# gNOI-3.3: Supervisor Switchover

## Summary

Validate that the active supervisor can be successfully switched to the standby supervisor in a dual-supervisor system. This test validates the management plane recovery, zero traffic loss over LACP port-channels, and negative corner cases such as back-to-back switchovers.

## Testbed type

* `featureprofiles/topologies/ate_tests/topology_2_ports.testbed`

## Procedure

### Test environment setup

*   Configure an LACP port-channel across 2 DUT ports connected to the IXIA/ATE.
*   Start continuous data-plane traffic from the IXIA/ATE over the LACP interfaces to the DUT. The traffic must run continuously from the beginning to the end of the entire test suite.

#### Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "name": "Port-Channel1",
        "config": {
          "name": "Port-Channel1",
          "type": "iana-if-type:ieee8023adLag"
        }
      },
      {
        "name": "Ethernet1",
        "config": {
          "name": "Ethernet1"
        },
        "openconfig-if-ethernet:ethernet": {
          "openconfig-if-aggregate:config": {
            "aggregate-id": "Port-Channel1"
          }
        }
      },
      {
        "name": "Ethernet2",
        "config": {
          "name": "Ethernet2"
        },
        "openconfig-if-ethernet:ethernet": {
          "openconfig-if-aggregate:config": {
            "aggregate-id": "Port-Channel1"
          }
        }
      }
    ]
  },
  "lacp": {
    "interfaces": {
      "interface": [
        {
          "name": "Port-Channel1",
          "config": {
            "name": "Port-Channel1",
            "interval": "FAST",
            "lacp-mode": "ACTIVE"
          }
        }
      ]
    }
  }
}
```

### gNOI-3.3.1: Supervisor Switchover and Recovery Validation

*   **Step 1:** Issue `gnoi.SwitchControlProcessor` to the chassis.
*   **Step 2:** Validate the switchover was successful:
    *   Verify the standby RE/SUP becomes active by checking `/components/component/state/redundant-role` transitions to `PRIMARY`.
    *   Verify the old active RE/SUP transitions to `STANDBY`.
*   **Step 3:** Validate traffic and LACP state during and after the switchover:
    *   Verify the LACP session does not flap and connected ports remain up (`/interfaces/interface/state/oper-status`
    *   Validate the member ports are in-sync (`/lacp/interfaces/interface/members/member/state/synchronization` is `IN_SYNC`).
    *   Validate there is **zero traffic loss** from IXIA over the LACP ports during the entire switchover event.
*   **Step 4:** Validate management plane recovery:
    *   Execute a simple `gNMI.Set` (e.g., updating an interface description) and a `gNMI.Get` to ensure the new active supervisor fully processes management operations.

### gNOI-3.3.2: Back-to-Back Switchover (Negative Case)

*   **Step 1:** Trigger an SSO via `gnoi.SwitchControlProcessor`.
*   **Step 2:** Immediately issue a second `gnoi.SwitchControlProcessor` request while the new standby supervisor (the former primary) is still in an unready state.
*   **Step 3:** Validate the system gracefully rejects the second request or handles it safely without crashing. The active supervisor must maintain control and traffic must continue with zero loss.

### gNOI-3.3.3: Switchover with Power-Disabled Standby (Negative Case)

*   **Step 1:** Disable the standby supervisor (e.g., by setting its component `admin-state` or via power down).
*   **Step 2:** Attempt to trigger an SSO via `gnoi.SwitchControlProcessor`.
*   **Step 3:** Verify the switchover request is rejected.
*   **Step 4:** Verify the current active supervisor safely maintains control and there is zero traffic loss.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  ## Component State Paths ##
  /system/state/current-datetime:
  /components/component/state/last-switchover-time:
    platform_type: [ "CONTROLLER_CARD" ]
  /components/component/state/last-switchover-reason/trigger:
    platform_type: [ "CONTROLLER_CARD" ]
  /components/component/state/last-switchover-reason/details:
    platform_type: [ "CONTROLLER_CARD" ]
  /components/component/state/redundant-role:
    platform_type: [ "CONTROLLER_CARD" ]
  /components/component/state/oper-status:
    platform_type: [ "CONTROLLER_CARD" ]
  
  ## Interface and LACP State Paths ##
  /interfaces/interface/state/oper-status:
  /lacp/interfaces/interface/members/member/state/synchronization:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
  gnoi:
    system.System.SwitchControlProcessor:
```

## Required DUT platform

*   **MFF**
