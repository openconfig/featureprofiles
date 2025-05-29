# DP-1.4: QoS Interface Output Queue Counters

## Summary

Validate QoS interface output queue counters.

## Procedure

*   Configure ATE port-1 connected to DUT port-1, and ATE port-2 connected to DUT port-2, with the relevant IPv4 addresses.
*   Send the traffic with forwarding class NC1, AF4, AF3, AF2, AF1 and BE1 over the DUT.
*   Verify that the following telemetry paths exist on the QoS output interface of the DUT.
    *   /qos/interfaces/interface/output/queues/queue/state/transmit-pkts
    *   /qos/interfaces/interface/output/queues/queue/state/transmit-octets
    *   /qos/interfaces/interface/output/queues/queue/state/dropped-pkts
    *   /qos/interfaces/interface/output/queues/queue/state/dropped-octets

## Config Parameter coverage

*   /interfaces/interface/config/enabled
*   /interfaces/interface/config/name
*   /interfaces/interface/config/description

## Telemetry Parameter coverage

*   /qos/interfaces/interface/output/queues/queue/state/transmit-pkts
*   /qos/interfaces/interface/output/queues/queue/state/transmit-octets
*   /qos/interfaces/interface/output/queues/queue/state/dropped-pkts
*   /qos/interfaces/interface/output/queues/queue/state/dropped-octets

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml
paths:
  ## Config paths:
  /qos/forwarding-groups/forwarding-group/config/name:
  /qos/forwarding-groups/forwarding-group/config/output-queue:
  /qos/queues/queue/config/name:
  /qos/classifiers/classifier/config/name:
  /qos/classifiers/classifier/config/type:
  /qos/classifiers/classifier/terms/term/actions/config/target-group:
  /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set:
  /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set:
  /qos/classifiers/classifier/terms/term/config/id:
  /qos/interfaces/interface/output/queues/queue/config/name:
  /qos/interfaces/interface/input/classifiers/classifier/config/name:
  /qos/interfaces/interface/output/scheduler-policy/config/name:
  /qos/scheduler-policies/scheduler-policy/config/name:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/priority:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/sequence:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/id:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/input-type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/weight:

  ## State paths:
  /qos/forwarding-groups/forwarding-group/state/name:
  /qos/forwarding-groups/forwarding-group/state/output-queue:
  /qos/queues/queue/state/name:
  /qos/classifiers/classifier/state/name:
  /qos/classifiers/classifier/state/type:
  /qos/classifiers/classifier/terms/term/actions/state/target-group:
  /qos/classifiers/classifier/terms/term/conditions/ipv4/state/dscp-set:
  /qos/classifiers/classifier/terms/term/conditions/ipv6/state/dscp-set:
  /qos/classifiers/classifier/terms/term/state/id:
  /qos/interfaces/interface/output/queues/queue/state/name:
  /qos/interfaces/interface/input/classifiers/classifier/state/name:
  /qos/interfaces/interface/output/scheduler-policy/state/name:
  /qos/scheduler-policies/scheduler-policy/state/name:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/state/priority:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/state/sequence:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/state/type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/state/id:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/state/input-type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/state/queue:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/state/weight:

rpcs:
  gnmi:
    gNMI.Set:
      Replace: