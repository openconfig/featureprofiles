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

* TestID-x.y.z - Name of subtest
  * Step 1
  * Step 2
  * Validation and pass fail criteria

* TestID-x.y.z - Name of subtest
  * Step 1
  * Step 2
  * Validation and pass fail criteria

## Config Parameter Coverage

Add list of OpenConfig 'config' paths used in this test, if any.

## Telemetry Parameter Coverage

Add list of OpenConfig 'state' paths used in this test, if any.

## Protocol/RPC Parameter Coverage

Add list of OpenConfig RPC's (gNMI, gNOI, gNSI, gRIBI) used in the list, if any.

For example:

* gNMI
  * Set
  * Subscribe
* gNOI
  * System
    * KillProcess
  * Healthz
    * Get
    * Check
    * Artifact

## Required DUT platform

* Specify the minimum DUT-type:
  * MFF - A modular form factor device containing LINECARDs, FABRIC and redundant CONTROLLER_CARD components
  * FFF - fixed form factor
  * vRX - virtual router device
