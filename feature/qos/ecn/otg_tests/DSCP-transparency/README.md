
# Instructions for the template

Below is the required template for writing test requirements.  Good examples of test
requirements include :

* [TE-3.7: Base Hierarchical NHG Update](/feature/gribi/otg_tests/base_hierarchical_nhg_update/README.md)
* [gNMI-1.13: Telemetry: Optics Power and Bias Current](https://github.com/openconfig/featureprofiles/blob/main/feature/platform/tests/optics_power_and_bias_current_test/README.md)
* [RT-5.1: Singleton Interface](https://github.com/openconfig/featureprofiles/blob/main/feature/interface/singleton/otg_tests/singleton_test/README.md)

# TestID-x.y: DSCP transperency with ECN

## Summary

This test evaluates if all 64 combination of DSCP bits are transparently handled while ECN bits are rewritten.

## Testbed type

* TESTBED_DUT_ATE_4LINKS

## Procedure

* Testbed configuration
  * Connect DUTPort1 with OTGPort1, DUTPort2 with OTGPort2, DUTPort2 with OTGPort2; Assigne IPv4 and IPv6 addresses on all.
  * All 3 ports are of the same speed (100GE)
  * Configure QoS
    * DSCP classifier for IPv4 and IPv4 as below:

      |DSCP (dec)|Traffic-group|
      |--|--|
      |48-63|NC1|
      |40-47||
      |32-40|AF4|
      |24-31|AF3|
      |16-23|AF2|
      |12-17|AF1|
      |8-11|BE0|
      |0-7|BE1|
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
* Sub Test #1 - No-Congestion 
  * Generate 64 flows of traffic form ATEPort1  toward ATEPort3
    * each flow has distinct DSCP value
    * every packet has ECT(0) set
    * all flows of equal bps rate.
    * total load - 60% (60Gbps)
  * Verify using DUT telemetry that:
    * no drops is seen in any of queues on DUTPort3
    * all queues reports non-zero transmit packets, octets.
  * Verify on ATEPort3 that all flows are recived w/o DSCP modification -all 64 values are observed
  * verify on ATEPort3 that all recived packet has ECT(0) value

* Sub Test #2 - Congestion
  * Generate 64 flows of traffic form ATEPort1 and  64 flows of traffic form ATEPort2 toward ATEPort3
    * each flow form ATEPort1 has distinct DSCP value 
    * each flow form ATEPort2 has distinct DSCP value 
    * every packet has ECT(0) set
    * all flows of equal bps rate.
    * Offered load:
      * ATEPort1 - 60% (60Gbps)
      * ATEPort2 - 60% (60Gbps)
      * Note: egress port is congested.
  * Verify using DUT telemetry that:
    * no drops is seen in any of queues on DUTPort3
    * all queues reports non-zero transmit packets, octets.
  * Verify on ATEPort3 that all flows are recived w/o DSCP modification -all 64 values are observed
  * verify on ATEPort3 that all recived packet has ECT(0) value

## Config Parameter Coverage

Add list of OpenConfig 'config' paths used in this test, if any.

## Telemetry Parameter Coverage

Add list of OpenConfig 'state' paths used in this test, if any.

## Protocol/RPC Parameter Coverage

Add list of OpenConfig RPC's (gNMI, gNOI, gNSI, gRIBI) used in the list, if any.

For example:

* gNMI
  * Set
  * Subscribe
* gNOI
  * System
    * KillProcess
  * Healthz
    * Get
    * Check
    * Artifact

## Required DUT platform

* Specify the minimum DUT-type: {MFF, FFF, vRX}