# HA-1.0: Telemetry: Firewall High Availability.

## Summary

Telemetry: Firewall High Availability

## Testbed type

*  [`featureprofiles/topologies/dutdut.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dutdut.testbed)

## Procedure

### Test environment setup

    ```
      |         |                                 |         |
      |   DUT1  |----------4 links--------------  |   DUT2  |
      |         |----------control link---------  |         |
      |         |----------data link------------  |         |
    ```

#### Configuration

* We assume FW1 and FW2 are configured with high availability.
* We assume FW1 to be have the below configurions.
  - FW1 is configured with priority 90
  - FW2 is configured with priority 100

### HA-1.0: Verify FW Cluster correctly reports active/primary state before and after event, verify config ha-enabled and ha-mode works as expected.

* Verify FW Cluster correctly reports the active/primary state
  - Initially FW1 is expected to be in ACTIVE state
  - FW2 is expected to be in PASSIVE state
* Trigger an event to change the HA state of FW1 and FW2
  - After the event validate the HA state on FW1 is PASSIVE
  - and FW2 HA state is ACTIVE
* config ha-enabled and ha-mode and verify the below oc path hold the
  configured value
  -/ha-groups/ha-group/config/ha-enabled
  -/ha-groups/ha-group/config/ha-mode

### TODO: validate /ha-groups/ha-group/state/ha-peer-state which is currently not supported in openconfig-fw-high-availability

#### Canonical OC

```json
{
  "ha-groups": {
    "ha-group": [
      {
        "config": {
          "ha-enabled": true,
          "ha-mode": "ACTIVE_PASSIVE",
          "id": 1,
          "preempt": true,
          "priority": 100
        },
        "id": 1,
        "state": {
          "ha-state": "ACTIVE"
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /ha-groups/ha-group/state/ha-state:
  /ha-groups/ha-group/config/ha-enabled:
  /ha-groups/ha-group/config/ha-mode:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Get:
```
