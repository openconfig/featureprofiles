# Feature Profiles Test Plan Style Guide

A test plan is a Markdown file that describes the step-by-step procedure about a
test. It is typically found as the `README.md` in the test directory.

For example:

*   `feature/interface/singleton/ate_tests/singleton_test/README.md` documents
    the test plan for the issue
    [RT-5.1 Singleton Interface](https://github.com/openconfig/featureprofiles/issues/111).
*   `feature/interface/singleton/ate_tests/singleton_test/singleton_test.go`
    implements the issue.
*   `feature/interface/aggregate/ate_tests/aggregate_test/README.md` documents
    the test plan for the issue
    [RT-5.2 Aggregate Interface](https://github.com/openconfig/featureprofiles/issues/112).
*   `feature/interface/aggregate/ate_tests/aggregate_test/aggregate_test.go`
    implements the issue.

## Test Plan Template

The test plan in `README.md` is generally structured like this:

```
# RT-5.1: Singleton Interface

## Summary

[Insert a 1-3 sentence description of the test.]

## Procedure

[Write an ordered list of steps.  Each step may be given a
descriptive name.]

1. Step 1
2. Step 2
3. ...

## Config Parameter Coverage

[Write a list of OpenConfig paths which are expected to be included.]

*   /interfaces/interface/config/name
*   /interfaces/interface/config/description
*   ...

## Telemetry Parameter Coverage

[Write a list of OpenConfig paths which are expected to be included.]

*   /interfaces/interface/state/oper-status
*   /interfaces/interface/state/admin-status
*   ...
```

## Development Approach for Tests

### Vendor Neutrality

Test plans should use a vendor-neutral mechanism to configure the DUT, e.g. in
the OpenConfig public model or other OpenConfig protocols such as gNMI, gRIBI,
and gNOI. It should depend only on the behavior of the open specification.

### Device/Topology Neutrality

Tests are to be built against a minimal topology that consists of a single
device (referred to as DUT) against a reference implementation (referred to as
an ATE) of routing protocols and data plane functionality.

Each test plan must choose from one of the following testbeds and adhere to it
for the entirety of the test plan:

*   atedut\_2: two interconnected ports between ATE and DUT.
*   atedut\_4: four interconnected ports between ATE and DUT.
*   atedut\_12: 4 singleton ports and 8 breakout ports interconnected between
    ATE and DUT. Ondatra testbed models the logical ports, but the test is
    responsible for activating the breakout.
*   dutdut: four interconnected ports between two DUTs.

Test plans must not hardcode assumptions about the presence of specific hardware
components (e.g. switch chip, controller card, line card). If needed, it may
accept a "device profile" which parameterizes the expectations about the device.
If the test plan is about a feature that does not make sense for a given device
profile, the test should be skipped. For example, a test about controller card
switchover should be skipped if the device profile indicates that the device is
expected to have only one controller card, rather than skipping based on the
number of controller cards detected from the device.

### Functional Testing vs. Fuzzing

Initially, the test plan will consist of manually identified functional features
as opposed to automated parameter space coverage known as fuzzing. We are
prioritizing on validating device functionality (e.g. establishing a BGP session
based on a specific input BGP configuration) that are critical to the device
operation, rather than compliance solely based on the input schema (e.g.
enumerating that all uint32 can be used as BGP AS number).

### Telemetry as a Component of a Test

In some previous testing implementations, specific tests have been designed to
specifically cover the correct reporting of telemetry. This approach tends to
duplicate test setup across functional tests and telemetry-specific tests. We
choose to implement tests that cover telemetry testing alongside the
functionality testing. Generally, telemetry from both the DUT and ATE should be
used as pass/fail criteria of the test, such that telemetry is validated as a
core component of each test.

## Readable Language

All code and documentation should follow
[Google developer documentation style guide](https://developers.google.com/style/word-list)
for the use of inclusive language.

Test plan should be written in a natural language. Avoid introducing your own
notation because people who read the test plan are not going to be familiar with
the notation. We need the test plan to be self-explanatory.

### Example: Good

1.  Create VLANs 1-20 between ATE port-2 and DUT port-2. For VLAN number N, the
    IP addresses shall be:

    *   ATE port-2 VLAN N: 192.0.2.(N*4+1)
    *   DUT port-2 VLAN N: 192.0.2.(N*4+2)

    Throughout the test configuration, the traffic destination prefix DEST
    should be 203.0.113.0/24, and the virtual IPs for configuring next hops
    should be VIP-A as 192.0.2.111 and VIP-B as 192.0.2.222. Use the DEFAULT
    network instance for all entries.

2.  Configure an IPv4Entry to DEST referencing a NextHopGroup with index 1
    containing the following next hops:

    *   NextHop with index 1 to VIP-A
    *   NextHop with index 2 to VIP-B

3.  Configure an IPv4Entry to VIP-A referencing a NextHopGroup with index 10
    containing the following next hops:

    *   NextHop with index 11 to the IPv4 of ATE port-2 VLAN 1
    *   NextHop with index 12 to the IPv4 of ATE port-2 VLAN 2

4.  Configure an IPv4Entry to VIP-B referencing a NextHopGroup with index 20
    containing the following next hops:

    *   NextHop with index 21 to the IPv4 of ATE port-2 VLAN 3
    *   NextHop with index 22 to the IPv4 of ATE port-2 VLAN 4

### Example: Bad

```
Create VLANs 1-20.

$DEST=203.0.113.0/24
$VIP_A=192.0.2.111
$VIP_B=192.0.2.222

IPv4Entry $DEST -> NHG#1 -> {
  NH#1 $VIP-A
  NH#2 $VIP-B
}

IPv4Entry $VIP_A -> NHG#10 -> {
  NH#11 $ATE_PORT2_VLAN1_IP
  NH#12 $ATE_PORT2_VLAN2_IP
}

IPv4Entry $VIP_B -> NHG#20 -> {
  NH#21 $ATE_PORT2_VLAN3_IP
  NH#22 $ATE_PORT2_VLAN3_IP
}
```

## Completeness

A complete test plan should specify the values for configuration or telemetry.
This includes IP addresses assignments, autonomous system (AS) numbers, VLAN IDs
and others. The test-plan should be self-sufficient so that two authors
implementing the same test would end up with a comparable implementation.

### Example: Good

*   Push non-overlapping mixed SetRequest specifying CLI for DUT port-1 and
    OpenConfig for DUT port-2.

    *   `origin: "cli"` containing vendor configuration setting the DUT port-1
        description to `"foo1"`.

    *   `origin: ""` (openconfig, default origin) setting the DUT port-2 string
        value at `/interfaces/interface/config/description` to `"foo2"`.

    Validate that the DUT port-1 and DUT port-2 descriptions are `"foo1"` and
    `"foo2"` respectively.

*   Push overlapping mixed SetRequest specifying CLI before OpenConfig for DUT
    port-1.

    *   `origin: "cli"` containing vendor configuration setting the DUT port-1
        description to `"from cli"`.

    *   `origin: ""` (openconfig, default origin) setting the DUT port-1 string
        value at `/interfaces/interface/config/description` to `"from oc"`.

    Validate that DUT port-1 description is `"from oc"`.

*   Push overlapping mixed SetRequest specifying OpenConfig before CLI for DUT
    port-1.

    *   `origin: ""` (openconfig, default origin) setting the DUT port-1 string
        value at `/interfaces/interface/config/description` to `"from oc"`.

    *   `origin: "cli"` containing vendor configuration setting the DUT port-1
        description to `"from cli"`.

    Validate that DUT port-1 description is `"from cli"`.

Note: For Arista and Cisco, the vendor configuration is:

```
interface <DUT port>
  description <text>
```

For Juniper: TBD.

### Example: Bad

The following is bad because it does not specify what needs to be configured,
and what should be checked through telemetry. It is also missing the
permutations needed to check the overlapping and non-overlapping cases, as well
as what the expected behavior should be in each of the respective cases.

```
*   Validate mixed OC/CLI schema by sending both CLI and OC config concurrently
    for two different interfaces.
*   Push configuration using SetRequest specifying:

    *   `origin: "cli"` - containing modelled configuration.
    *   `origin: ""` (openconfig, default origin) - containing modelled
        configuration.

*   Validate the DUT port-1 and DUT port-2 descriptions through telemetry.
```
