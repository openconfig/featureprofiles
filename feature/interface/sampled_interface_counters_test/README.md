# gNMI-1.18: gNMI subscribe with sample mode for interface counters

## Summary
WBB is required to support gNMI Subscribe with sample mode for various counters.
This test if to verify that DUT supports gNMI Subscribe with sample mode while
forwarding traffic and updating the interface counters correctly

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

## Procedure

*   Connect DUT port-1 to ATE port-1, DUT port-2 to ATE port-2

*   Configure IPv4/IPv6 addresses on the interfaces

*   Initiate traffic

*   Counters should be verified using gNMI subscribe with sample mode and an 
    interval of 10 seconds on inbound port (DUT port-1):

    *   /interfaces/interface/state/counters/in-unicast-pkts
    *   /interfaces/interface/state/counters/in-broadcast-pkts
    *   /interfaces/interface/state/counters/in-multicast-pkts
    *   /interfaces/interface/state/counters/in-octets
    *   /interfaces/interface/state/counters/in-discards
    *   /interfaces/interface/state/counters/in-errors
    *   /interfaces/interface/state/counters/in-fcs-errors

*   Counters should be verified using gNMI subscribe with sample mode and an 
    interval of 10 seconds on outbound port (DUT port-2):

    *   /interfaces/interface/state/counters/out-unicast-pkts
    *   /interfaces/interface/state/counters/out-broadcast-pkts
    *   /interfaces/interface/state/counters/out-multicast-pkts
    *   /interfaces/interface/state/counters/out-octets
    *   /interfaces/interface/state/counters/out-errors
    *   /interfaces/interface/state/counters/out-discards

*   Ensure inbound and outbound unicast counters are the same

*   Ensure counters increment every sample interval of 10 seconds

## Config parameter coverage

*   Interfaces

    *   /interfaces/interface/config/enabled
    *   /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled
    *   /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled

## Telemetry parameter coverage (gNMI subscribe with sample every 10 seconds)

*   /interfaces/interface/state/counters/in-unicast-pkts
*   /interfaces/interface/state/counters/in-broadcast-pkts
*   /interfaces/interface/state/counters/in-multicast-pkts
*   /interfaces/interface/state/counters/in-octets
*   /interfaces/interface/state/counters/in-discards
*   /interfaces/interface/state/counters/in-errors
*   /interfaces/interface/state/counters/in-fcs-errors
*   /interfaces/interface/state/counters/out-unicast-pkts
*   /interfaces/interface/state/counters/out-broadcast-pkts
*   /interfaces/interface/state/counters/out-multicast-pkts
*   /interfaces/interface/state/counters/out-octets
*   /interfaces/interface/state/counters/out-errors
*   /interfaces/interface/state/counters/out-discards
