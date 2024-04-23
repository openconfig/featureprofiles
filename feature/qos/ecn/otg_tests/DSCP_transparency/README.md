
# DP-1.14: DSCP transperency with ECN

## Summary

This test evaluates if all 64 combination of DSCP bits are transparently handled while ECN bits are rewritten.

## Testbed type

* TESTBED_DUT_ATE_4LINKS

## Procedure

### Testbed configuration
* Connect DUTPort1 with OTGPort1, DUTPort2 with OTGPort2, DUTPort2 with OTGPort2; Assigne IPv4 and IPv6 addresses on all.
* All 3 ports are of the same speed (100GE)
* Configure QoS
    * DSCP classifier for IPv4 and IPv4 as below:
        |DSCP (dec)|Traffic-group|
        |--|--|
        |48-63|NC1|
        |32-47|AF4|
        |24-31|AF3|
        |16-23|AF2|
        |8-15|AF1|
        |4-7|BE0|
        |0-3|BE1|
    * 7 queues and 7 corresponding forwarding group
    * Scheduler policy with
       * one scheduler of STRICT priority type serving NC1 queue
       * one scheduler of WRR type serving 6 queues AF4, AF3, AF2, AF1, BE0, BE1 with equal weights 10:10:10:10:10:10 respectivly
    * queue-management profile of WRED type with:
       * min-treshold: 80KB
       * max-treshold: 3MB
       * max-drop-percentage: 100 
       * ecn: enabled
    * attach queue-management profile to queues NC1, AF4, AF3, AF2, AF1, BE0, BE1;
    * attach scheduler-map to DUTPort3 egress
    * attach classifier to DUTPort1 nad DUTPort2 ingress

### Sub Test #1 - No-Congestion 
* Generate 64 flows of traffic form ATEPort1  toward ATEPort3
    * each flow has distinct DSCP value
    * every packet has ECT(0) set
    * all flows of equal bps rate.
    * total load - 60% (60Gbps)
* wait 1 minutes; stop traffic generation.
* Verify using DUTPort3 telemetry that:
    * no drops are seen in any of queues on DUTPort3
    * all queues reports non-zero transmit packets, octets.
* Verify on ATEPort3 that all flows are recived w/o DSCP modification -all 64 values are observed
* verify on ATEPort3 that all recived packet has ECT(0) ECN value
### Sub Test #2 - Congestion
* Generate 64 flows of traffic form ATEPort1 and  64 flows of traffic form ATEPort2 toward ATEPort3
    * each flow form ATEPort1 has distinct DSCP value 
    * each flow form ATEPort2 has distinct DSCP value 
    * every packet has ECT(0) set
    * all flows are of equal bps rate.
    * Offered load:
        * ATEPort1 - 60% (60Gbps)
        * ATEPort2 - 60% (60Gbps)
    * Note: egress port is congested, so do all queues but NC1 (SP)
* wait 1 minutes; stop traffic generation.
* Verify using DUTPort3 telemetry that:
    * Drops are seen in all queues except NC1 on DUTPort3
    * all queues reports non-zero transmit packets, octets.
* Verify on ATEPort3 that all flows are recived w/o DSCP modification - all 64 values are observed
* verify on ATEPort3 that:
    * all recived packets with DSCP 48-63 has ECT(0) value
    * vast majority (almost all) packets with DSCP 0-47 has CE ECN value.
### Sub Test #3 - NC1 congestion
* Generate 16 flows of traffic form ATEPort1 and  16 flows of traffic form ATEPort2 toward ATEPort3
    * each flow form ATEPort1 has distinct DSCP value from 48-63 range
    * each flow form ATEPort2 has distinct DSCP value from 48-63 range
    * every packet has ECT(0) set
    * all flows are of equal bps rate.
    * Offered load:
    * ATEPort1 - 60% (60Gbps)
    * ATEPort2 - 60% (60Gbps)
    * Note: egress port is congested, so do NC1 (SP) queue
* wait 1 minutes; stop traffic generation.
* Verify using DUTPort3 telemetry that:
    * Drops are seen in NC1 queue on DUTPort3
    * all queues but NC1 reports nzero transmit packets, octets.
    * NC1 queue reports non-zero transmit packets, octets.
* Verify on ATEPort3 that all flows are recived w/o DSCP modification - all 16 values are observed.
* verify on ATEPort3 that:
    * all recived packets with DSCP has CE value

## Config Parameter Coverage

  *  qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set
  *  qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set
  *  qos/classifiers/classifier/terms/term/actions/config/target-group
  *  qos/queues/queue/config/name
  *  qos/forwarding-groups/forwarding-group/config/name
  *  qos/forwarding-groups/forwarding-group/config/output-queue
  *  qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/priority
  *  qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/sequence
  *  qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/id
  *  qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/input-type
  *  qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue
  *  qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/weight
  *  qos/queue-management-profiles/queue-management-profile/wred/uniform/config/enable-ecn
  *  qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-drop-probability-percent
  *  qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-threshold
  *  qos/queue-management-profiles/queue-management-profile/wred/uniform/config/min-threshold
  *  qos/interfaces/interface/output/queues/queue/config/name
  *  qos/interfaces/interface/output/queues/queue/config/queue-management-profile
  *  qos/interfaces/interface/output/scheduler-policy/config/name
  *  qos/interfaces/interface/input/classifiers/classifier/config/name
  *  qos/interfaces/interface/input/classifiers/classifier/config/type

## Telemetry Parameter Coverage

  *  qos/interfaces/interface/output/queues/queue/state/dropped-octets
  *  qos/interfaces/interface/output/queues/queue/state/dropped-pkts
  *  qos/interfaces/interface/output/queues/queue/state/name
  *  qos/interfaces/interface/output/queues/queue/state/transmit-octets
  *  qos/interfaces/interface/output/queues/queue/state/transmit-pkts

## Protocol/RPC Parameter Coverage

  * NONE.

## Required DUT platform

* FFF
