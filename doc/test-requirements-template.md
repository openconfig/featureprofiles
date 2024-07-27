---
name: New featureprofiles test requirement
about: Use this template to document the requirements for a new test to be implemented.
title: ''
labels: enhancement
assignees: ''
---

# Instructions for this template

Below is the required template for writing test requirements.  Good examples of test
requirements include:

* [TE-3.7: Base Hierarchical NHG Update](/feature/gribi/otg_tests/base_hierarchical_nhg_update/README.md)
* [gNMI-1.13: Telemetry: Optics Power and Bias Current](https://github.com/openconfig/featureprofiles/blob/main/feature/platform/tests/optics_power_and_bias_current_test/README.md)
* [RT-5.1: Singleton Interface](https://github.com/openconfig/featureprofiles/blob/main/feature/interface/singleton/otg_tests/singleton_test/README.md)

# TestID-x.y: Short name of test here

## Summary

Write a few sentences or paragraphs describing the purpose and scope of the test.

## Testbed type

* Specify the .testbed topology file from the [topologies](https://github.com/openconfig/featureprofiles/tree/main/topologies) folder to be used with this test

## Procedure

### Test environment setup
  * Description of procedure to configure ATE and DUT with pre-requisites making it possible to cover the intended paths and RPC's.

### TestID-x.y.z - Name of subtest
  * Canonical OpenConfig Configuration

An example OpenConfig configuration and/or RPC content should be specified here in YAML format.

```yaml
---
openconfig-qos:
  scheduler-policies:
    - scheduler-policy: "rate-limit-1"
      config:
        name: "rate-limit-1"
      schedulers:
        - scheduler:
          config:
              type: ONE_RATE_TWO_COLOR
          one-rate-three-color:
            config:
              cir: 1000000000           # 1Gbit/sec
              bc: 10000                 # 10 kilobytes
              queuing-behavior: POLICE
            exceed-action:
              config:
                drop: TRUE
```

  * Step 1
  * Step 2
  * Validation and pass/fail criteria

### TestID-x.y.z - Name of subtest
  * Canonical OpenConfig configuration
  * Step 1
  * Step 2
  * Validation and pass/fail criteria

## OpenConfig Path and RPC Coverage

This yaml stanza defines the OC paths intended to be covered by this test.  OC paths used
for test environment setup are not required to be listed here. This content is parsed by
automation to derive the test coverage

```yaml
paths:
  # interface configuration
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  # name of chassis and linecard components
  /components/component/state/name:
    platform_type: ["CHASSIS", "LINECARD"]

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

* Specify the minimum DUT-type:
  * MFF - A modular form factor device containing LINECARDs, FABRIC and redundant CONTROLLER_CARD components
  * FFF - fixed form factor
  * vRX - virtual router device
