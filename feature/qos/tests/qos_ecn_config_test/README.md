# DP-1.3: QoS ECN feature config

## Summary

Verify QoS ECN feature configuration.

## Procedure

*   Connect DUT port-1 to ATE port-1, DUT port-2 to ATE port-2.

*   ECN configuration paramentes:

    *   This test is verifying DUT ability to accept ECN configuration with vertical buffer utilization cut-off line.\ If buffer is utilized below that cut-off value no packet is ECN CE marked.\ If buffer is utilized at of above that cut-off value all packet are ECN CE marked.

    *   ECN profile can be created for different queues. ECN profiles per queue
        can be applied to the output side of interfaces.

        Test case|min-threshold | max-threshold | enable-ecn | drop  | weight  | max-drop-probability-percent
        |--------|------------- | ------------- | ---------- | ----- | ------- | ----------------------------
        |#1      |80KB          |80KB          | true       | false | not set | 100
        |[TODO]#2      |3.125MB       | 6.250MB*      | true       | false | not set | 100
        |[TODO]#3      |1%            | 2%            | true       | false | not set | 100

    * 6.25MB max-treshhold is selected as it represents ~500 mico-seconds of Delay bandwidth Buffer on 100GE interfaces. What is O(2%) of buffer depth, hence allows for micro-burst absorbtion without beackpressing senders and at same time leaves enough DBB for accomodate RTT ECN signaling loop delay in global network for longer burst/congestion.

*   Procedure
    * Test Case #1 80KB min-threshold equal max-threshold
        *   Configute queue-management-profile w/ ECN paramenters as above. Attach to queue on DUT-Port1.
        *   Validate that the following values are set as expected using OC telemetry.
            *   min-threshold
            *   max-threshold
            *   min-threshold = max-threshold (vertical cut-off line)
            *   enable-ecn
            *   drop
            *   max-drop-probability-percent
        *   Validate ECN profile can be applied under output interface queue using
            OC telemetry.
    * [TODO]Test Case #2 Treshold in MB, min-threshold not-equal max-threshold
        *   Configute queue-management-profile w/ ECN paramenters as above. Attach to queue on DUT-Port1.
        *   Validate that the following values are set as expected using OC telemetry.
            *   min-threshold
            *   max-threshold
            *   enable-ecn
            *   drop
            *   max-drop-probability-percent
        *   Validate ECN profile can be applied under output interface queue using
            OC telemetry.
    * [TODO]Test Case #3 Treshold in percentage, min-threshold not-equal max-threshold
        *   Configute queue-management-profile w/ ECN paramenters as above. Attach to queue on DUT-Port1.
        *   Validate that the following values are set as expected using OC telemetry.
            *   min-threshold-percent
            *   max-threshold-percent
            *   enable-ecn
            *   drop
            *   max-drop-probability-percent
        *   Validate ECN profile can be applied under output interface queue using
            OC telemetry.

## Config parameter coverage

*   ECN
    *   [TODO] qos/queue-management-profiles/queue-management-profile/wred/uniform/config/min-threshold-percent
    *   [TODO] qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-threshold-percent    
    *   qos/queue-management-profiles/queue-management-profile/wred/uniform/config/min-threshold
    *   qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-threshold
    *   qos/queue-management-profiles/queue-management-profile/wred/uniform/config/enable-ecn
    *   qos/queue-management-profiles/queue-management-profile/wred/uniform/config/weight
    *   qos/queue-management-profiles/queue-management-profile/wred/uniform/config/drop
    *   qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-drop-probability-percent

*   Interfaces

    *   /qos/interfaces/interface/input/classifiers/classifier/config/name
    *   /qos/interfaces/interface/output/queues/queue/config/name
    *   /qos/interfaces/interface/output/queues/queue/config/queue-management-profile

## telemetry parameter coverage

*   ECN

    *   [TODO] qos/queue-management-profiles/queue-management-profile/wred/uniform/state/min-threshold-percent
    *   [TODO] qos/queue-management-profiles/queue-management-profile/wred/uniform/state/max-threshold-percent  
    *   qos/queue-management-profiles/queue-management-profile/wred/uniform/state/min-threshold
    *   qos/queue-management-profiles/queue-management-profile/wred/uniform/state/max-threshold
    *   qos/queue-management-profiles/queue-management-profile/wred/uniform/state/enable-ecn
    *   qos/queue-management-profiles/queue-management-profile/wred/uniform/state/weight
    *   qos/queue-management-profiles/queue-management-profile/wred/uniform/state/drop
    *   qos/queue-management-profiles/queue-management-profile/wred/uniform/state/max-drop-probability-percent

*   Interfaces

    *   /qos/interfaces/interface/input/classifiers/classifier/state/name
    *   /qos/interfaces/interface/output/queues/queue/state/name
    *   /qos/interfaces/interface/output/queues/queue/state/queue-management-profile

## platform

 * vRX
