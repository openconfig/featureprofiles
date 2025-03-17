# PLT-1.2: Parent component validation test

## Summary

Validate that the parent components of an entity is correct before and after reboot

- Verify parent component of an interface is a Switch Chip
- [TODO]: Add parent component verification for other hardware entities

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

## Procedure

* Connect ATE port-1 to DUT port-1, and ATE port-2 to DUT port-2

### PLT-1.2.1 - Validate parent component of an interface:

* Validate that the correct parent component "SwitchChip" is reported for DUT port1 and port2
* Reboot the DUT and verify the DUT finished rebooting by checking the uptime
* Re-validate that the correct parent component "SwitchChip" is reported for DUT port1 and port2

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths and RPC intended to be covered by this test.

```yaml
paths:
  /components/component/state/parent:
    platform_type: ["INTEGRATED_CIRCUIT"]
rpcs:
  gnmi:
    gNMI.Get:
```

## Minimum DUT Platform Requirement

- FFF
