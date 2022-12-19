# DP-1.3: QoS ECN feature config

## Summary

Verify QoS ECN feature configuration.

## Procedure

*   Connect DUT port-1 to ATE port-1, DUT port-2 to ATE port-2.

*   ECN config:

    *   ECN profile can be created for different queues. ECN profiles per queue
        can be applied to the output side of interfaces.

        min-threshold | max-threshold | enable-ecn | drop  | weight  | max-drop-probability-percent
        ------------- | ------------- | ---------- | ----- | ------- | ----------------------------
        80000         | 2^64-1        | true       | false | not set | 1

        *   Note: max-threshold is set to max uint64 value 2^64-1
            or 18446744073709551615.

    *   Validate that the following values can be configured

        *   min-threshold
        *   max-threshold
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
