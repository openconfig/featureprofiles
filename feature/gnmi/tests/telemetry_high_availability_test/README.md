# gNMI-1.26: Telemetry: Firewall High Availability.

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

* configure FW cluster between DUT1 and DUT2 with preemption is enabled
* DUT1 with low priority (ACTIVE)
* DUT2 with high priority (PASSIVE)
* Configure a link group with 4 links between DUT1 and DUT2

### gNMI-1.26.1: Verify FW Cluster correctly reports the active/primary state, control/data links state, interface groups state.

* Verify FW Cluster correctly reports the active/primary state
* Verify control/data links state
* Verify interface groups state

### gNMI-1.26.2: FW Cluster correctly reports HA state changes in the event of an operator triggered failover.

* On Active device, suspend high-availability.
* Passive device should detect suspension and become Active
* Verify the state change the Firewall device.
* Bring back the suspended device to functional state.
* Verify the cluster status.

### gNMI-1.26.3: FW Cluster correctly reports HA state changes in the event of a failure either of the FW Cluster or its links.

* On the Active device verify link monitoring
* Trigger restart system on Active FW cluster
* Verify state/ha-state on Passive device, the state should change to active
* Wait for previous active to come up, verify the active state is preempted once the device is "UP"

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

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
    gNMI.Get:
```
