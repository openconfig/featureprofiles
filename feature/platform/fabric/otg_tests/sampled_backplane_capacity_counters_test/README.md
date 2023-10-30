# gNMI-1.18: gNMI subscribe with sample mode for backplane capacity counters

## Summary
WBB is required to support gNMI Subscribe with SAMPLE or ON_CHANGE mode for various counters.
This test if to verify that DUT supports gNMI Subscribe with sample mode, updating
the available backplane capacity counters correctly while forwarding traffic

*   Get backplace capacity before any traffic is sent

*   Send some traffic and wait for the sample to be collected

*   Send more traffic and wait for the sample to be collected

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

## Procedure

### gNMI-1.18.1 [TODO: https://github.com/openconfig/featureprofiles/issues/2322]

*   Connect DUT port-1 and 2 to ATE port-1 and 2 respectively

    *   For MFF DUT ports 1 and 2 SHOULD be on different linecards

*   Configure IPv4/IPv6 addresses on the interfaces

*   Using gNMI subscribe with "SAMPLE" mode

    *   Run the test twice, once with a SAMPLE interval of 10 Seconds and once again
        with a SAMPLE interval of 15 seconds for the below telemetry path

    *   /components/component/integrated-circuit/backplane-facing-capacity/state/consumed-capacity

    *   Initiate traffic:

        *   Initiate traffic as per below threshold:
    
             Port number   | Interface1(line rate %)
            -------------- | -----------------------
            Port1          | 20%
            Port2          | 20%

    *   Validate that we are receiving consumed capacity metrics at the selected SAMPLE interval

        *   Increase the traffic as per below threshold

             Port number   | Interface1(line rate %)
            -------------- | -----------------------
            Port1          | 70%
            Port2          | 70%

    *   Validate we are now receiving increased consumed capacity metrics at the selected SAMPLE interval

### gNMI-1.18.2 [TODO: https://github.com/openconfig/featureprofiles/issues/2323]

*   Connect DUT port-1 and 2 to ATE port-1 and 2 respectively

    *   For MFF DUT ports 1 and 2 SHOULD be on different linecards

*   Configure IPv4/IPv6 addresses on the interfaces

*   Use gNMI subscribe with "ON_CHANGE" mode for the below telemetry paths

    *   /components/component/integrated-circuit/backplane-facing-capacity/state/total
    *   /components/component/integrated-circuit/backplane-facing-capacity/state/total-operational-capacity
    *   /components/component/integrated-circuit/backplane-facing-capacity/state/available-pct

*   Disable one of the available FABRIC component

    *   set /components/component/{fabric}/config/power-admin-state to POWER_DISABLED

*   Validate that we recieve changed metrics of a lower value for each of the telemetry paths

*   Enable the FABRIC component that was disabled in the previous step

    *   Set /components/component/{fabric|linecard|controller-card}/config/power-admin-state to POWER_ENABLED

*   Validate that we recieve changed metrics of a higher value for each of the telemetry paths

## Config parameter coverage

*   /interfaces/interface/config/enabled
*   /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled
*   /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled
*   /components/component/{fabric}/config/power-admin-state

## Telemetry parameter coverage (gNMI subscribe with sample every 10 seconds)

*   /components/component/integrated-circuit/backplane-facing-capacity/state/available-pct
*   /components/component/integrated-circuit/backplane-facing-capacity/state/consumed-capacity
*   /components/component/integrated-circuit/backplane-facing-capacity/state/total
*   /components/component/integrated-circuit/backplane-facing-capacity/state/total-operational-capacity

## Protocol/RPC Parameter Coverage

* gNMI
  * Set
  * Update
  * Subscribe

## Required DUT platform

* MFF
