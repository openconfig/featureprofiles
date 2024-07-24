# DP-1.5: Egress Strict Priority scheduler with bursty traffic

## Summary

This test verifies the behavior of an egress strict priority scheduler under bursty traffic conditions. By configuring multiple priority queues with specific traffic classes and generating bursty traffic that exceeds the interface capacity in short durations, we will validate that the scheduler maintains strict priority order, prioritizing the transmission of higher-priority traffic even during bursts, potentially leading to drops in lower-priority traffic.

## Testbed type

*  [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

*   DUT has 2 ingress ports and 1 egress port with the same port speed. The
    interface can be a physical interface or LACP bundle interface with the
    same aggregated speed.

    ```
                                |         | ---- | ATE Port 1 |
        [ ATE Port 3 ] ----  |   DUT   |      |            |
                                |         | ---- | ATE Port 2 |
    ```

*   Traffic classes:
    *   We will use 6 traffic classes NC1, AF4, AF3, AF2, AF1 and BE1.

*   Traffic types:
    *   All the traffic tests apply to both IPv4 and IPv6 and also MPLS traffic.

*   Queue types:
    *   NC1/AF4/AF3/AF2/AF1/BE1 will have strict priority queues (be1 - priority 6, af1 - priority 5, ..., nc1 - priority 1)

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

#### Configuration

*   Forwarding Classes: Configure six forwarding classes (be1, af1, af2, af3, af4, nc1) based on the classification table provided.
*   Classification table

    IPv4 TOS      |       IPv6 TC           |         MPLS EXP        |    Forwarding Group
    ------------- | ----------------------- | ----------------------- | ---------------------
    0             |      0-7                |          0              |         be1
    1             |      8-15               |          1              |         af1
    2             |      16-23              |          2              |         af2
    3             |      24-31              |          3              |         af3
    4,5           |      32-47              |          4,5            |         af4
    6,7           |      48-63              |          6,7            |         nc1

*   Egress Scheduler: Apply a multi-level strict-priority scheduling policy on the desired egress interface. Assign priorities to each forwarding class as below: 
        *   be1 - priority 6
        *   af1 - priority 5
        *   af2 - priority 4
        *   af3 - priority 3
        *   af4 - priority 2
        *   nc1 - priority 1

### DP1-5.1 Egress Strict Priority scheduler with bursty traffic for IPv4

*   Traffic Generation:
    *   Generate the IPv4 traffic as below

        *   Interface Port 1:

        Forwarding Group  |     Traffic linerate  (%)   |      Frame size        |    Expected Loss %
        ----------------- | --------------------------- |---------------------   | -----------------------------------
        be1               |          12                 |      512               |         100
        af1               |          12                 |      512               |         100
        af2               |          15                 |      512               |         50
        af3               |          12                 |      512               |         0
        af4               |          30                 |      512               |         0
        nc1               |          1                  |      512               |         0

        *   Interface Port 2:

        Forwarding Group | Traffic linerate  (%)  | Frame size | Burst size    | Inter-pkt gap | inter-burst gap | Expected Loss %
        --------------   |    --------------      | ---------- | -----------   | ------------- | --------------- | ---------------
              be1        | 20                     | 256        | 50000         | 12            | 100             | 100
              af1        | 13                     | 256        | 50000         | 12            | 100             | 100
              af2        | 17                     | 256        | 50000         | 12            | 100             | 50
              af3        | 10                     | 256        | 50000         | 12            | 100             | 0
              af4        | 20                     | 256        | 50000         | 12            | 100             | 0
              nc1        | 10                     | 256        | 50000         | 12            | 100             | 0

*   Verification:
    *   Loss Rate: Capture packet loss for every generated flow and verify that loss for each flow does not exceed expected loss specified in the tables above.
    *   Telemetry: Utilize OpenConfig telemetry parameters to validate that per queue dropped packets statistics corresponds (with error margin) to the packet loss reported for every flow matching that particular queue.
               
### DP1-5.2 Egress Strict Priority scheduler with bursty traffic for IPv6

*   Traffic Generation:
    *   Generate the IPv6 traffic as below

        *   Interface Port 1:

        Forwarding Group  |     Traffic linerate  (%)   |      Frame size        |    Expected Loss %
        ----------------- | --------------------------- |---------------------   | -----------------------------------
        be1               |          12                 |      512               |         100
        af1               |          12                 |      512               |         100
        af2               |          15                 |      512               |         50
        af3               |          12                 |      512               |         0
        af4               |          30                 |      512               |         0
        nc1               |          1                  |      512               |         0

        *   Interface Port 2:

        Forwarding Group | Traffic linerate  (%)  | Frame size | Burst size    | Inter-pkt gap | inter-burst gap | Expected Loss %
        --------------   |    --------------      | ---------- | -----------   | ------------- | --------------- | ---------------
              be1        | 20                     | 256        | 50000         | 12            | 100             | 100
              af1        | 13                     | 256        | 50000         | 12            | 100             | 100
              af2        | 17                     | 256        | 50000         | 12            | 100             | 50
              af3        | 10                     | 256        | 50000         | 12            | 100             | 0
              af4        | 20                     | 256        | 50000         | 12            | 100             | 0
              nc1        | 10                     | 256        | 50000         | 12            | 100             | 0

*   Verification:
    *   Loss Rate: Capture packet loss for every generated flow and verify that loss for each flow does not exceed expected loss specified in the tables above.
    *   Telemetry: Utilize OpenConfig telemetry parameters to validate that per queue dropped packets statistics corresponds (with error margin) to the packet loss reported for every flow matching that particular queue.

### DP1-5.3 Egress Strict Priority scheduler with bursty traffic for MPLS

*   Traffic Generation:
    *   Generate the MPLS traffic as below

        *   Interface Port 1:

        Forwarding Group  |     Traffic linerate  (%)   |      Frame size        |    Expected Loss %
        ----------------- | --------------------------- |---------------------   | -----------------------------------
        be1               |          12                 |      512               |         100
        af1               |          12                 |      512               |         100
        af2               |          15                 |      512               |         50
        af3               |          12                 |      512               |         0
        af4               |          30                 |      512               |         0
        nc1               |          1                  |      512               |         0

        *   Interface Port 2:

        Forwarding Group | Traffic linerate  (%)  | Frame size | Burst size    | Inter-pkt gap | inter-burst gap | Expected Loss %
        --------------   |    --------------      | ---------- | -----------   | ------------- | --------------- | ---------------
              be1        | 20                     | 256        | 50000         | 12            | 100             | 100
              af1        | 13                     | 256        | 50000         | 12            | 100             | 100
              af2        | 17                     | 256        | 50000         | 12            | 100             | 50
              af3        | 10                     | 256        | 50000         | 12            | 100             | 0
              af4        | 20                     | 256        | 50000         | 12            | 100             | 0
              nc1        | 10                     | 256        | 50000         | 12            | 100             | 0

*   Verification:
    *   Loss Rate: Capture packet loss for every generated flow and verify that loss for each flow does not exceed expected loss specified in the tables above.
    *   Telemetry: Utilize OpenConfig telemetry parameters to validate that per queue dropped packets statistics corresponds (with error margin) to the packet loss reported for every flow matching that particular queue.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
  ## Config paths
  ### Classifiers
  /qos/classifiers/classifier/config/name:
  /qos/classifiers/classifier/config/type:
  /qos/classifiers/classifier/terms/term/config/id:
  /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set:
  /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set:
  /qos/classifiers/classifier/terms/term/conditions/mpls/config/traffic-class:
  /qos/classifiers/classifier/terms/term/actions/config/target-group:

  ### Forwarding Groups
  /qos/forwarding-groups/forwarding-group/config/name:
  /qos/forwarding-groups/forwarding-group/config/output-queue:

  ### Queue
  /qos/queues/queue/config/name:

  ### Interfaces
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
