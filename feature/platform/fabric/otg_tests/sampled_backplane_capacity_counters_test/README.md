# gNMI-1.19: gNMI subscribe with sample mode for backplane capacity counters

## Summary
WBB is required to support gNMI Subscribe with sample mode for various counters.
This test if to verify that DUT supports gNMI Subscribe with sample mode, updating
the available backplane capacity counters correctly while forwarding traffic

*   Get backplace capacity before any traffic is sent

*   Send some traffic and wait for the sample to be collected

*   Send more traffic and wait for the sample to be collected

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_9.testbed

## Procedure

*   Connect DUT port-1 through 9 to ATE port-1 through 9

*   Configure IPv4/IPv6 addresses on all the 9 interfaces

*   Using gNMI subscribe with sample mode and an interval of 10 seconds
    validate the backplane capacity before any traffic is sent and during the phases
    This will happen at 0 Seconds 

*   Phase 1 (initiate traffic immediately):

    *   Initiate traffic as per below threshold:
    
         Port number   | Interface1(line rate %)
        -------------- | -----------------------
        Port1          | 20
        Port2          | 20
        Port3          | 20
        Port4          | 20
        Port5          | 20
        Port6          | 20
        Port7          | 20
        Port8          | 20
        Port9          | 20

*   We should get second sample at 10 seconds

*   Phase 2 (after 10 seconds):

    *   Initiate traffic as per below threshold:
    
         Port number   | Interface1(line rate %)
        -------------- | -----------------------
        Port1          | 80
        Port2          | 80
        Port3          | 80
        Port4          | 80
        Port5          | 80
        Port6          | 80
        Port7          | 80
        Port8          | 80
        Port9          | 80

*   We should get third sample at 10 seconds

*   Ensure available backplane capacity shows a decrease at every sample interval

## Config parameter coverage

*   Interfaces

    *   /interfaces/interface/config/enabled
    *   /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled
    *   /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled

## Telemetry parameter coverage (gNMI subscribe with sample every 10 seconds)

*   /components/component/integrated-circuit/backplane-facing-capacity/state/available-pct
*   /components/component/integrated-circuit/backplane-facing-capacity/state/consumed-capacity
*   /components/component/integrated-circuit/backplane-facing-capacity/state/total
*   /components/component/integrated-circuit/backplane-facing-capacity/state/total-operational-capacity
