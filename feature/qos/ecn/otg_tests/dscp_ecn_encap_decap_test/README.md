# DP-1.13: DSCP and ECN bits are copied over during IPinIP encap and decap

## Summary

Verify that DUT copies and preserves DSCP and ECN bits during IPinIP encap and
decap process.

## QoS traffic test setup:

*   Topology:

    *   2 input interfaces and 1 output interface with the same port speed. The
        interface can be a physical interface or LACP bundle interface with the
        same aggregated speed.

    ```
      ATE port 1
          |
         DUT--------ATE port 3
          |
      ATE port 2
    ```

*   Traffic classes:

    *   We will use 7 traffic classes NC1, AF4, AF3, AF2, AF1, BE1 and BE0.

*   Traffic types:

    *   All the traffic tests apply to both IPv4 and IPv6 traffic.

*   Queue types:

    *   NC1 will have strict priority queues
        *   AF4/AF3/AF2/AF1/BE1/BE0 will use WRR queues.
    *   NC1 and AF4 will have strict priority queues with NC1 having higher
        priority.
        *   AF3, AF2, AF1, BE1 and BE0 will use WRR queues.

*   Test results should be independent of the location of interfaces. For
    example, 2 input interfaces and output interface could be located on

    *   Same ASIC-based forwarding engine
    *   Different ASIC-based forwarding engine on same line card
    *   Different ASIC-based forwarding engine on different line cards

*   Test results should be the same for port speeds 100G and 400G.

*   Counters should be also verified for each test case:

    *   /qos/interfaces/interface/output/queues/queue/state/transmit-pkts
    *   /qos/interfaces/interface/output/queues/queue/state/dropped-pkts
    *   transmit-pkts should be equal to the number of Rx pkts on Ixia port
    *   dropped-pkts should be equal to diff between the number of Tx and the
        number Rx pkts on Ixia ports

*   Latency:

    *   Should be < 100000ns

## Procedure

*   Connect DUT port-1 to ATE port-1, DUT port-2 to ATE port-2 and DUT port-3 to
    ATE port-3.

*   Configuration

    *   ECN profile can be created for different queues. ECN profiles per queue
        can be applied to the output side of interfaces.

        min-threshold | max-threshold | enable-ecn | drop  | weight  | max-drop-probability-percent
        ------------- | ------------- | ---------- | ----- | ------- | ----------------------------
        80000         | 2^64-1        | true       | false | not set | 1

        *   Note: max-threshold is set to max uint64 value 2^64-1
            or 18446744073709551615.

*   Validate that traffic-class(IPv6) and IPv4(DSCP+ECN) are always copied
    between inner and outer headers during encap/decap in IPinIP case:

    *   IPv4 encaped to IPv4 (copy from inner to outer)
    *   IPv6 encaped to IPv4 (copy from inner to outer)
    *   IPv4inIPv4 decaped to IPv4 (copy from outer to inner)
    *   IPv6inIPv4 decided to IPv6 (copy from outer to inner)
    *   IPv4inIPv4 decaped-then-encaped to IPv4 (copy from outer to inner then
        new outer)
    *   IPv6inIPv4 decaped-then-encaped to IPv4 (copy from outer to inner then
        new outer)

*   Send oversubscribed traffic with ECT or CE set to 1 from 2 input interfaces
    to trigger ECN bits to be set to 1.

    Traffic class | Interface1(line rate %) | Interface2(line rate %) | ECN bits
    ------------- | ----------------------- | ----------------------- | --------
    NC1           | 51                      | 50                      | 1

*   Verify that ECN bits are set in both inner and outer headers in encap case.

*   Repeat the above test cases for other traffic classes:

    *   AF4
    *   AF3
    *   AF2
    *   AF1
    *   BE1
    *   BE0

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
    *   /qos/interfaces/interface/output/queues/queue/config/queue-management-profile

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

*   ECN

    *   qos/queue-management-profiles/queue-management-profile/wred/uniform/config/min-threshold
    *   qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-threshold
    *   qos/queue-management-profiles/queue-management-profile/wred/uniform/config/enable-ecn
    *   qos/queue-management-profiles/queue-management-profile/wred/uniform/config/weight
    *   qos/queue-management-profiles/queue-management-profile/wred/uniform/config/drop
    *   qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-drop-probability-percent

## Telemetry parameter coverage

*   /qos/interfaces/interface/output/queues/queue/state/transmit-pkts
*   /qos/interfaces/interface/output/queues/queue/state/transmit-octets
*   /qos/interfaces/interface/output/queues/queue/state/dropped-pkts
*   /qos/interfaces/interface/output/queues/queue/state/dropped-octets
