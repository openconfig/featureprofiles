# QoS-1.1: Ingress traffic classification Test 

## Summary

Validate Ingress traffic classification.

## Procedure

*   Configure DUT with ingress and egress routed interfaces.
*   Configure QOS classifiers to match packets arriving on DUT ingress interface to corresponding forwarding class according to
    classification table.
*   Configure packet re-marking for configured classes according to the marking table.
*   One-by-one send flows containing every TOS/TC/EXP value in the classification table.
*   For every flow sent, verify match-packets counters on the DUT ingress interface
*   verify packet markings on ATE ingress interface
*   verify that no traffic drops in all flows

## Config Parameter coverage

*   /openconfig-qos:qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp
*   /openconfig-qos:qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set
*   /openconfig-qos:qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp
*   /openconfig-qos:qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set
*   /openconfig-qos:qos/classifiers/classifier/terms/term/actions/config/target-group
*   /openconfig-qos:qos/interfaces/interface/input/classifiers/classifier/config
*   /openconfig-qos:qos/interfaces/interface/input/classifiers/classifier/config/name

## Telemetry Parameter coverage

*   /qos/interfaces/interface/input/classifiers/classifier/terms/term/state/matched-packets
*   /qos/interfaces/interface/input/classifiers/classifier/terms/term/state/matched-octets

*   QoS Classification and Marking table

    *  Classification table

    IPv4 TOS      |       IPv6 TC           |         MPLS EXP        |    Forwarding class
    ------------- | ----------------------- | ----------------------- | ---------------------
    0             |      0-7                |          0              |         be1
    1             |      8-15               |          1              |         af1
    2             |      16-23              |          2              |         af2
    3             |      24-31              |          3              |         af3
    4,5           |      32-47              |          4,5            |         af4
    6,7           |      48-63              |          6,7            |         nc1

    *   Marking table

    IPv4 TOS      |       IPv6 TC           |         MPLS EXP        |    Forwarding class
    ------------- | ----------------------- | ----------------------- | ---------------------
    0             |      0                  |          0              |         be1
    1             |      8                  |          1              |         af1
    2             |      16                 |          2              |         af2
    3             |      24                 |          3              |         af3
    4             |      32                 |          4              |         af4
    6             |      48                 |          6              |         nc1
