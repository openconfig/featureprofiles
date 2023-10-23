# DP-1.10: gNMI subscribe with sample mode for output queue counters

## Summary
WBB is required to support gNMI Subscribe with sample mode for various counters.
This test if to verify that DUT supports gNMI Subscribe with sample mode while
forwarding QoS traffic and updating the output queue counters correctly

*   Traffic classes:

    *   We will use 7 traffic classes NC1, AF4, AF3, AF2, AF1, BE0 and BE1.

*   Queue types:

    *   NC1 will have strict priority queues
    *   AF4/AF3/AF2/AF1/BE1/BE0 will use WRR queues with weight 48, 12, 8, 4, 1, 1

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

## Procedure

*   Connect DUT port-1 to ATE port-1, DUT port-2 to ATE port-2
    and configure IPv4/v6 addresses

*   Configuration:

    *   Configure strict priority queues for NC1.
    *   Configure WRR for AF4, AF3, AF2, AF1, BE0 and BE1 with weight 48, 12, 8, 4, 1
        and 1 respectively.

*   Initiate traffic as per below thresholds:

     Traffic class | Interface1(line rate %)
    -------------- | -----------------------
    NC1            | 6
    Af4            | 48
    AF3            | 12
    AF2            | 8
    AF1            | 4
    BE1            | 1
    BE0            | 1

*   Counters should be verified using gNMI subscribe with sample mode and an interval of 10 seconds:
*   Ensure counter of all queues increment every 10 seconds

## Config parameter coverage

*   Classifiers

    *   /qos/classifiers/classifier/config/name
    *   /qos/classifiers/classifier/config/type
    *   /qos/classifiers/classifier/terms/term/actions/config/target-group
    *   /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set
    *   qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set
    *   /qos/classifiers/classifier/terms/term/config/id

*   Forwarding Groups

    *   /qos/forwarding-groups/forwarding-group/config/name
    *   /qos/forwarding-groups/forwarding-group/config/output-queue

*   Queue

    *   /qos/queues/queue/config/name

*   Interfaces

    *   /qos/interfaces/interface/input/classifiers/classifier/config/name
    *   /qos/interfaces/interface/output/queues/queue/config/name
    *   /qos/interfaces/interface/output/scheduler-policy/config/name

*   Scheduler policy

    *   /qos/scheduler-policies/scheduler-policy/config/name
    *   /qos/scheduler-policies/scheduler
        -policy/schedulers/scheduler/config/priority
    *   /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/sequence
    *   /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/type
    *   /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/id
    *   /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/input-type
    *   /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue
    *   /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/weight

## Telemetry parameter coverage

*   /qos/interfaces/interface/output/queues/queue/state/transmit-pkts
*   /qos/interfaces/interface/output/queues/queue/state/transmit-octets
*   /qos/interfaces/interface/output/queues/queue/state/dropped-pkts
*   /qos/interfaces/interface/output/queues/queue/state/dropped-octets
