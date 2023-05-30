# DP-1.3: QoS ECN feature config

## Summary

Verify QoS ECN feature configuration.

## Procedure

*   Connect DUT port-1 to ATE port-1, DUT port-2 to ATE port-2.

*   ECN config:

    *   This test is verifying DUT ability to accept ECN configuration with vertical buffer utilization cut-off line.\ If buffer is utilized below that cut-off value no packet is ECN CE marked.\ If buffer is utilized at of above that cut-off value all packet are ECN CE marked.

    *   ECN profile can be created for different queues. ECN profiles per queue
        can be applied to the output side of interfaces.

        min-threshold | max-threshold | enable-ecn | drop  | weight  | max-drop-probability-percent
        ------------- | ------------- | ---------- | ----- | ------- | ----------------------------
        8MB           | 8MB           | true       | false | not set | 100

    * 8MB max-treshhold is selected as it represents ~640 mico-seconds of Delay bandwidth Buffer on 100GE interfaces. What is O(2%) of buffer depth, hence allows for micro-burst absorbtion without beackpressing senders and at same time leaves enough DBB for accomodate RTT ECN signaling loop delay in global network for longer burst/congestion.

    *   Validate that the following values can be configured

        *   min-threshold
        *   max-threshold
        *   min-threshold = max-threshold (vertical cut-off line)
        *   enable-ecn
        *   drop
        *   max-drop-probability-percent

    *   The following OC config paths can be used to configure the above values:

        *   qos/queue-management-profiles/queue-management-profile/wred/uniform/config/min-threshold
        *   qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-threshold
        *   qos/queue-management-profiles/queue-management-profile/wred/uniform/config/enable-ecn
        *   qos/queue-management-profiles/queue-management-profile/wred/uniform/config/weight
        *   qos/queue-management-profiles/queue-management-profile/wred/uniform/config/drop
        *   qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-drop-probability-percent

*   Interfaces

    *   Validate ECN profile can be applied under output interface queue using
        OC config path:
        *   /qos/interfaces/interface/output/queues/queue/config/queue-management-profile

## Config parameter coverage

*   ECN

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
