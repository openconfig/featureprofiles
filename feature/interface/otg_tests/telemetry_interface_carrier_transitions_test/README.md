# FNT: Carrier Transitions Test

## Summary

Validates that the `carrier-transitions` counter increments correctly when the
interface link state changes.
[TODO: b/391432919] Use `/interfaces/interface/state/counters/interface-transitions` and
`/interfaces/interface/state/counters/link-transitions` for the interface state
changes.

## Testbed type

Topology:
https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

ATE port-1 <------> port-1 DUT

## Procedure

### Test Environment Setup

[TODO: b/391432919] Deprecate this test when `interface-transitions` and `link-transitions` OC
Paths are implemented.

The test environment consists of a DUT connected to an ATE with the following
port roles:

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

#### [TODO: b/391432919] FNT-1 - Admin Flap Test
1. Get Initial Counters: Read the values of:
    *   `/interfaces/interface[name=dutPort1]/state/counters/interface-transitions`
    *   `/interfaces/interface[name=dutPort1]/state/counters/link-transitions`
    via gNMI.Get. Store these as `interface_transitions_initial` and
    `link_transitions_initial`.
2. Admin Down: Set `/interfaces/interface[name=dutPort1]/config/enabled` to
   `false` using gNMI.Set UPDATE.
3. Verify Oper Down: Confirm
   `/interfaces/interface[name=dutPort1]/state/oper-status` transitions to
   `DOWN` via gNMI.Get or Subscribe.
4. Admin Up: Set `/interfaces/interface[name=dutPort1]/config/enabled` to
   `true` using gNMI.Set UPDATE.
5. Verify Oper Up: Confirm
   `/interfaces/interface[name=dutPort1]/state/oper-status` transitions to `UP`.
6. Get Final Counters: Read the counter values:
    *   `/interfaces/interface[name=dutPort1]/state/counters/interface-transitions`
    *   `/interfaces/interface[name=dutPort1]/state/counters/link-transitions`
    again. Store these as `interface_transitions_final` and
    `link_transitions_final`.
7. Validation:
    * `interface_transitions_final` must be equal to `interface_transitions_initial + 2`.
    * `link_transitions_final` must not change from `link_transitions_initial`.

#### [TODO: b/391432919] FNT-2 - ATE Port Flap Test
1. Setup: Ensure the interface is configured as per the basic setup and is
   operationally UP.
2. Get Initial Counters: Read the values of:
    *   `/interfaces/interface[name=dutPort1]/state/counters/interface-transitions`
    *   `/interfaces/interface[name=dutPort1]/state/counters/link-transitions`
3. ATE Port Down: Disable the ATE port `atePort1`.
4. Verify Oper Down: Confirm DUT
   `/interfaces/interface[name=dutPort1]/state/oper-status` transitions to
   `DOWN`.
5. ATE Port Up: Enable the ATE port `atePort1`.
6. Verify Oper Up: Confirm DUT
   `/interfaces/interface[name=dutPort1]/state/oper-status` transitions to `UP`.
7. Get Final Counters: Read the counter values again.
8. Validation:
    * The `interface-transitions` counter must have incremented by exactly 2.
    * The `link-transitions` counter must have incremented by exactly 2.

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
  /interfaces/interface/state/counters/interface-transitions:
  /interfaces/interface/state/counters/link-transitions:
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

