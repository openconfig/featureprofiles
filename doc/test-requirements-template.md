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

* [TE-18.1 gRIBI MPLS in UDP Encapsulation and Decapsulation](https://github.com/openconfig/featureprofiles/blob/main/feature/gribi/otg_tests/mpls_in_udp/README.md)
* [TE-3.7: Base Hierarchical NHG Update](/feature/gribi/otg_tests/base_hierarchical_nhg_update/README.md)
* [gNMI-1.13: Telemetry: Optics Power and Bias Current](https://github.com/openconfig/featureprofiles/blob/main/feature/platform/tests/optics_power_and_bias_current_test/README.md)

# TestID-x.y: Short name of test here

## Summary

Write a few sentences or paragraphs describing the purpose and scope of the test.

## Testbed type

* Specify the .testbed topology file from the [topologies](https://github.com/openconfig/featureprofiles/tree/main/topologies) folder to be used with this test

## Procedure

### Test environment setup

* Description of procedure to configure ATE and DUT with pre-requisites making it possible to cover the intended paths and RPC's.

#### Canonical OpenConfig Configuration for the DUT

NOTE: An example OpenConfig configuration and/or RPC content for any common
DUT configuration which is used across the subtests should be specified
here in JSON format.

```json
{
  "openconfig-qos": {
    "interfaces": [
      {
        "config": {
          "interface-id": "PortChannel1.100"
        },
        "input": {
          "classifiers": [
            {
              "classifier": "dest_A",
              "config": {
                "name": "dest_A",
                "type": "IPV4"
              }
            }
          ],
          "scheduler-policy": {
            "config": {
              "name": "limit_group_A_1Gb"
            }
          }
        },
        "interface": "PortChannel1.100"
      },
    ]
  }
}
```

### TestID-x.y.1 - Name of subtest 1

The following steps are typically present in each subtest.

* Step 1 - Generate DUT configuration
* Step 2 - Push configuration to DUT
* Step 3 - Send Traffic
* Step 4 - Validation with pass/fail criteria

### TestID-x.y.2 - Name of subtest 2

* Step 1 - Generate DUT configuration
* Step 2 - Push configuration to DUT
* Step 3 - Send Traffic
* Step 4 - Validation with pass/fail criteria

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
