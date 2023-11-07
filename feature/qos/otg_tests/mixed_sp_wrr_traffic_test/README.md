# DP-1.10: Mixed strict priority and WRR traffic test

## Summary

Verify that DUT drops AF3, AF2, AF1, BE1 and BE0 before AF4 before NC1 and
forwards AF3, AF2, AF1, BE1 and BE0 based on weight.

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

*   Configuration:

    *   Configure strict priority queues for NC1 and AF4 with NC1 having higher
        priority.
    *   Configure WRR for AF3, AF2, AF1, BE1 and BE0 with weight 32, 16, 8, 4
        and 1 respectively.

*   Non-oversubscription traffic test case

    Traffic class | Interface1(line rate %) | Interface2(line rate %) | Rx from interface1(%) | Rx from interface2(%)
    ------------- | ----------------------- | ----------------------- | --------------------- | ---------------------
    NC1           | 0.1                     | 0.9                     | 100                   | 100
    Af4           | 18                      | 20                      | 100                   | 100
    AF3           | 16                      | 16                      | 100                   | 100
    AF2           | 8                       | 8                       | 100                   | 100
    AF1           | 4                       | 4                       | 100                   | 100
    BE0           | 2                       | 2                       | 100                   | 100
    BE1           | 0.5                     | 0.5                     | 100                   | 100

*   Oversubscription traffic test case 1

    Traffic class | Interface1(line rate %) | Interface2(line rate %) | Rx from interface1(%) | Rx from interface2(%)
    ------------- | ----------------------- | ----------------------- | --------------------- | ---------------------
    NC1           | 0.1                     | 0.9                     | 100                   | 100
    Af4           | 50                      | 49                      | 100                   | 100
    AF3           | 20                      | 14                      | 0                     | 0
    AF2           | 14                      | 24                      | 0                     | 0
    AF1           | 12                      | 4                       | 0                     | 0
    BE0           | 1                       | 7                       | 0                     | 0
    BE1           | 1                       | 1                       | 0                     | 0

*   Oversubscription traffic test case 2

    Traffic class | Interface1(line rate %) | Interface2(line rate %) | Aggregate throughput(%)
    ------------- | ----------------------- | ----------------------- | -----------------------
    NC1           | 0.1                     | 0.9                     | 100
    Af4           | 18                      | 20                      | 100
    AF3           | 40                      | 24                      | 50
    AF2           | 8                       | 24                      | 50
    AF1           | 12                      | 4                       | 50
    BE0           | 1                       | 7                       | 50
    BE1           | 1                       | 1                       | 50

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
