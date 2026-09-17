# QOS-1.2: DSCP trust classification test

## Summary

Validate DSCP trust based classification test.

## Procedure

*   Configure DUT with ingress and egress routed interfaces.
*   Configure “DSCP trust” feature on the DUT ingress interface.
*   Look up and record device default queue classification maps.
*   One-by-one send flows containing every IPv4 TOS and IPv6 TC value in the classification table.
*   For every flow sent,
*   Verify that the flow is placed into correct egress interface queues, with the mapping matching to the device's classification map.
*   Verify that the DUT does not change traffic marking.
*   Verify that no traffic drops in all flows

## Config Parameter coverage

*   /openconfig-qos:qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp
*   /openconfig-qos:qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set
*   /openconfig-qos:qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp
*   /openconfig-qos:qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set
*   /openconfig-qos:qos/classifiers/classifier/terms/term/actions/config/target-group
*   /openconfig-qos:qos/interfaces/interface/input/classifiers/classifier/config
*   /openconfig-qos:qos/interfaces/interface/input/classifiers/classifier/config/name

## Telemetry Parameter coverage

*   /openconfig-qos:qos/interfaces/interface/input/virtual-output-queues/voq-interface/queues/queue/state/name
*   /openconfig-qos:qos/interfaces/interface/input/virtual-output-queues/voq-interface/queues/queue/state/transmit-pkts
*   /openconfig-qos:qos/interfaces/interface/input/virtual-output-queues/voq-interface/queues/queue/state/dropped-pkts

*   QoS Classification

    *  Classification table

    IPv4 TOS      |       IPv6 TC           |    Forwarding class
    ------------- | ----------------------- |  ---------------------
    0             |      0-7                |         be1
    1             |      8-15               |         af1
    2             |      16-23              |         af2
    3             |      24-31              |         af3
    4,5           |      32-47              |         af4
    6,7           |      48-63              |         nc1