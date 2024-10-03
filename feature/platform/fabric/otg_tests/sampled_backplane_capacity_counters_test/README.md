# gNMI-1.18: gNMI subscribe with sample mode for backplane capacity counters

## Summary
WBB is required to support gNMI Subscribe with SAMPLE or ON_CHANGE mode for various counters.
This test if to verify that DUT supports gNMI Subscribe with ON_CHANGE for backplane-facing-capacity

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

## Procedure

### gNMI-1.18.1

*   Connect DUT port-1 and 2 to ATE port-1 and 2 respectively

    *   For MFF DUT ports 1 and 2 SHOULD be on different linecards

*   Configure IPv4/IPv6 addresses on the interfaces

*   Using gNMI subscribe with "ON_CHANGE" mode

    *   /components/component/integrated-circuit/backplane-facing-capacity/state/consumed-capacity

        *   consumed-capacity is the sum of the admin-up front panel interface speeds connected to the integrated-circuit

*   Ensure that we receieve the initial consumed-capacity metric

*   Set port-2 enabled=false

    *   Validate that we recieve a changed consumed-capacity metric of a higher value

*   Set port-2 enable=true

    *   Validate that we recieve a changed consumed-capacity metric matching the initial value

### gNMI-1.18.2

*   Use gNMI subscribe with "ON_CHANGE" mode for the below telemetry paths

    *   /components/component/integrated-circuit/backplane-facing-capacity/state/total
    *   /components/component/integrated-circuit/backplane-facing-capacity/state/total-operational-capacity
    *   /components/component/integrated-circuit/backplane-facing-capacity/state/available-pct

*   Make sure we recieve the initial metric for each of the telemetry paths

*   Disable two FABRIC components

    *   set /components/component/{fabric}/config/power-admin-state to POWER_DISABLED

*   Validate that we recieve changed metric of a lower value for the below telemetry paths

    *   /components/component/integrated-circuit/backplane-facing-capacity/state/total-operational-capacity
    *   /components/component/integrated-circuit/backplane-facing-capacity/state/available-pct

*   Ensure that the metric for the below telemetry path should not have changed hence no updated metric should be received

    *   /components/component/integrated-circuit/backplane-facing-capacity/state/total

*   Enable the FABRIC components that were disabled previously

    *   Set /components/component/{fabric}/config/power-admin-state to POWER_ENABLED

*   Validate that we recieve changed metric matching the initial metric for the below of the telemetry paths

    *   /components/component/integrated-circuit/backplane-facing-capacity/state/total-operational-capacity
    *   /components/component/integrated-circuit/backplane-facing-capacity/state/available-pct

*   Ensure that the metric for the below telemetry path should not have changed hence no updated metric should be received

    *   /components/component/integrated-circuit/backplane-facing-capacity/state/total

## OpenConfig Path and RPC Coverage

This example yaml defines the OC paths intended to be covered by this test.  OC paths used for test environment setup are not required to be listed here.
```yaml
paths:
  ## Config parameter coverage
  /interfaces/interface/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled:
  /components/component/fabric/config/power-admin-state:
    platform_type: ["FABRIC"]

  ## Telemetry parameter coverage
  /components/component/integrated-circuit/backplane-facing-capacity/state/available-pct:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/backplane-facing-capacity/state/consumed-capacity:
    platform_type: [ "INTEGRATED_CIRCUIT" ]
  /components/component/integrated-circuit/backplane-facing-capacity/state/total:
    platform_type: [ "INTEGRATED_CIRCUIT" ]
  /components/component/integrated-circuit/backplane-facing-capacity/state/total-operational-capacity:
    platform_type: [ "INTEGRATED_CIRCUIT" ]

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
          Mode: [ "ON_CHANGE", "SAMPLE" ]
```

## Required DUT platform

* MFF
