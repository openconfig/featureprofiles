# PLT-1.2: Parent component validation test

## Summary

Validate that the parent components of an entity is correct

- Parent component of an interface is a Switch Chip

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

## Procedure

* Connect ATE port-1 to DUT port-1, and ATE port-2 to DUT port-2

### PLT-1.2.1 - Validate parent component of an interface:

* Validate that the correct parent component "SwitchChip" is reported for the DUT interfaces port-1 and port-2

### PLT-1.2.2 - [TODO]:

* Add parent component check for other entities

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths and RPC intended to be covered by this test.

```yaml
paths:
  /components/component/state/parent:

rpcs:
  gnmi:
    gNMI.Get:
```

## Minimum DUT Platform Requirement

- FFF
