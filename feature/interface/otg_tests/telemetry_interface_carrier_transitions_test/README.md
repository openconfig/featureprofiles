# FNT: Carrier Transitions Test

## Summary

Validates that the `carrier-transitions` counter increments correctly when the
interface link state changes.

## Testbed type

Topology:
https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

ATE port-1 <------> port-1 DUT

## Procedure

### Test Environment Setup

The test environment consists of a DUT connected to an ATE with the following port roles:

*   **DUT Port 1:** Configured with IPv4 and IPv6 addresses. This is the interface under test where the administrative state (`enabled`) will be toggled.
*   **ATE Port 1:** Configured with IPv4 and IPv6 addresses. Acts as the link peer to ensure the DUT interface can transition to `oper-status: UP`.

1.  **Setup:**

    *   Connect DUT port-1 to ATE port-1.
    *   Configure IPv4/IPv6 addresses on the DUT and ATE ports.
    *   Ensure the interface is administratively UP and the link is UP.

2.  **Collection:**

    *   Start a gNMI SAMPLE subscription for `/interfaces/interface/state/counters/carrier-transitions` with a 30s interval.

3.  **Trigger:**

    *   Administratively disable the DUT interface (set `enabled` to `false`).
    *   Verify `oper-status` becomes `DOWN`.
    *   Administratively enable the DUT interface (set `enabled` to `true`).
    *   Verify `oper-status` becomes `UP`.

4.  **Validation:**

    *   Wait for the collection to complete.
    *   Verify that the counter values never decrease (monotonicity).
    *   Verify that the counter values do not increase by more than 100 between samples.
    *   Verify that the final value is greater than the initial value.

#### Canonical OC

```json
{
  "openconfig-interfaces:interfaces": {
    "interface": [
      {
        "config": {
          "enabled": true,
          "name": "Ethernet1"
        },
        "name": "Ethernet1"
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /interfaces/interface/state/counters/carrier-transitions:
  /interfaces/interface/config/enabled:
  /interfaces/interface/state/oper-status:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      mode: sample
      sample_interval: 30s

```

## Required DUT platform

* MFF
* FFF

