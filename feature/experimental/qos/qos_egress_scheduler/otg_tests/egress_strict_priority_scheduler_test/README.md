# QoS-1.3: Egress strict priority scheduler test

## Summary

Egress strict priority scheduler test

## Procedure

*   Configure 6 forwarding classes according to the classification table.
*   Configure egress multi-level strict-priority scheduling policy for the above classes.
*   Send traffic flows according to the strict-priority test traffic table.
*   Verify loss-rate for traffic on the ingress ATE interface to match the expected loss
*   Verify queue drop counters on DUT egress interface.

## Config parameter coverage

*   /openconfig-qos:qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config
*   /openconfig-qos:qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/priority
*   /openconfig-qos:qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/sequence
*   /openconfig-qos:qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/type
*   /openconfig-qos:qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config

## Telemetry parameter coverage

*   /openconfig-qos:qos/interfaces/interface/input/virtual-output-queues/voq-interface/queues/queue/state/name
*   /openconfig-qos:qos/interfaces/interface/input/virtual-output-queues/voq-interface/queues/queue/state/transmit-pkts
*   /openconfig-qos:qos/interfaces/interface/input/virtual-output-queues/voq-interface/queues/queue/state/dropped-pkts
*   /openconfig-qos:qos/interfaces/interface/output/queues/queue/state/name
*   /openconfig-qos:qos/interfaces/interface/output/queues/queue/state/transmit-pkts
*   /openconfig-qos:qos/interfaces/interface/output/queues/queue/state/dropped-pkts

*   Classification table

    IPv4 TOS      |       IPv6 TC           |         MPLS EXP        |    Forwarding class
    ------------- | ----------------------- | ----------------------- | ---------------------
    0             |      0-7                |          0              |         be1
    1             |      8-15               |          1              |         af1
    2             |      16-23              |          2              |         af2
    3             |      24-31              |          3              |         af3
    4,5           |      32-47              |          4,5            |         af4
    6,7           |      48-63              |          6,7            |         nc1

*   Strict Priority Test traffic table

    Forwarding class  |      Priority        |     Traffic linerate  (%)   |    Forwarding class
    ----------------- |--------------------- | --------------------------- | ---------------------
    be1               |      6               |          25                 |         99
    af1               |      5               |          20                 |         80
    af2               |      4               |          5                  |         0
    af3               |      3               |          25                 |         0
    af4               |      2               |          35                 |         0
    nc1               |      1               |          1                  |         0