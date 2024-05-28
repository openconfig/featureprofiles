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
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/priority:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/sequence:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config:

  ## State paths
  /qos/interfaces/interface/input/virtual-output-queues/voq-interface/queues/queue/state/name:
  /qos/interfaces/interface/input/virtual-output-queues/voq-interface/queues/queue/state/transmit-pkts:
  /qos/interfaces/interface/input/virtual-output-queues/voq-interface/queues/queue/state/dropped-pkts:
  /qos/interfaces/interface/output/queues/queue/state/name:
  /qos/interfaces/interface/output/queues/queue/state/transmit-pkts:
  /qos/interfaces/interface/output/queues/queue/state/dropped-pkts:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

* MFF - A modular form factor device containing LINECARDs, FABRIC and redundant CONTROLLER_CARD components
* FFF - fixed form factor
