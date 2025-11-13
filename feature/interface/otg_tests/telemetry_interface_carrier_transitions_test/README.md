# FNT: Carrier Transitions Test

## Summary

Validates that the `carrier-transitions` counter increments correctly when the
interface link state changes.

## Testbed type

Topology:
https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

ATE port-1 <------> port-1 DUT

## Procedure

1.  **Setup:**

    *   Connect DUT port-1 to ATE port-1.
    *   Configure IPv4/IPv6 addresses on the DUT and ATE ports.
    *   Ensure the interface is administratively UP and the link is UP.

2.  **Baseline:**

    *   Retrieve the initial value of
        `/interfaces/interface/state/counters/carrier-transitions`.

3.  **Trigger:**

    *   Administratively disable the DUT interface (set `enabled` to `false`).
    *   Verify `oper-status` becomes `DOWN`.
    *   Administratively enable the DUT interface (set `enabled` to `true`).
    *   Verify `oper-status` becomes `UP`.

4.  **Validation:**

    *   Retrieve the new value of
        `/interfaces/interface/state/counters/carrier-transitions`.
    *   Verify that the new value is greater than the initial value.

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
      on_change: true
```
