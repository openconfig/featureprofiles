# QOS-1.3: Egress Strict Priority scheduler

## Summary

This test validates the proper functionality of an egress strict priority scheduler on a network device. By configuring multiple priority queues with specific traffic classes and generating traffic loads that exceed interface capacity, we will verify that the scheduler adheres to the strict priority scheme, prioritizing higher-priority traffic even under congestion.

## Procedure

* DUT Configuration:
    * Forwarding Classes: Configure six forwarding classes (be1, af1, af2, af3, af4, nc1) based on the classification table provided.
    * Egress Scheduler: Apply a multi-level strict-priority scheduling policy on the desired egress interface. Assign priorities to each forwarding class according to the strict priority test traffic table (be1 - priority 6, af1 - priority 5, ..., nc1 - priority 1).

* Classification table

    IPv4 TOS      |       IPv6 TC           |         MPLS EXP        |    Forwarding class
    ------------- | ----------------------- | ----------------------- | ---------------------
    0             |      0-7                |          0              |         be1
    1             |      8-15               |          1              |         af1
    2             |      16-23              |          2              |         af2
    3             |      24-31              |          3              |         af3
    4,5           |      32-47              |          4,5            |         af4
    6,7           |      48-63              |          6,7            |         nc1

* Traffic Generation:
    * Traffic Profiles: Define traffic profiles for each forwarding class using the ATE, adhering to the linerates (%) specified in the strict priority test traffic table.

* Strict Priority Test traffic table

    Forwarding class  |      Priority        |     Traffic linerate  (%)   |    Expected Output (Ingress Loss %)
    ----------------- |--------------------- | --------------------------- | -----------------------------------
    be1               |      6               |          25                 |         100
    af1               |      5               |          20                 |         100
    af2               |      4               |          15                 |         67%
    af3               |      3               |          20                 |         0
    af4               |      2               |          35                 |         0
    nc1               |      1               |          40                 |         0

* Verification:
    * Loss Rate: Monitor and compare the observed packet loss rate on the ATE's ingress interface against the expected loss rate defined in the traffic table.
    * Telemetry: Utilize OpenConfig telemetry parameters to monitor queue statistics (transmitted packets, dropped packets) and validate the correct operation of the strict priority scheduler.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
  ## Config paths
<<<<<<< HEAD
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/priority:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/sequence:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/type:
=======
  ### Classifiers
  /qos/classifiers/classifier/config/name:
  /qos/classifiers/classifier/config/type:
  /qos/classifiers/classifier/terms/term/config/id:
  /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set:
  /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set:
  /qos/classifiers/classifier/terms/term/conditions/mpls/config/traffic-class:
  /qos/classifiers/classifier/terms/term/actions/config/target-group:
  /qos/classifiers/classifier/terms/term/actions/remark/config/set-dscp:
  /qos/classifiers/classifier/terms/term/actions/remark/config/set-mpls-tc:

  ### Forwarding Groups
  /qos/forwarding-groups/forwarding-group/config/name:
  /qos/forwarding-groups/forwarding-group/config/output-queue:

  ### Queue
  /qos/queues/queue/config/name:

  ### Interfaces
  /qos/interfaces/interface/input/classifiers/classifier/config/type:
  /qos/interfaces/interface/input/classifiers/classifier/config/name:
  /qos/interfaces/interface/output/queues/queue/config/name:
  /qos/interfaces/interface/output/scheduler-policy/config/name:

  ### Scheduler policy
  /qos/scheduler-policies/scheduler-policy/config/name:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/priority:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/sequence:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/id:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/input-type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue:
>>>>>>> d06c9142 (Create a Readme for  QoS-1.1, QoS-1.3 and QoS-1.5)

  ## State paths
  /qos/interfaces/interface/output/queues/queue/state/name:
  /qos/interfaces/interface/output/queues/queue/state/transmit-pkts:
  /qos/interfaces/interface/output/queues/queue/state/transmit-octets:
  /qos/interfaces/interface/output/queues/queue/state/dropped-pkts:
  /qos/interfaces/interface/output/queues/queue/state/dropped-octets:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

* MFF - A modular form factor device containing LINECARDs, FABRIC and redundant CONTROLLER_CARD components
* FFF - fixed form factor
