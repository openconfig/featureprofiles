# PF-1.18 - MPLSoGRE and MPLSoGUE QoS 

## Summary
This test verifies quality of service with MPLSoGRE and MPLSoGUE IP traffic on routed VLAN sub interfaces. The classification, marking and queueing of traffic while being encapsulated and decapsulated based on the outer headers and/or inner payload are the major features verified on the test device.

## Testbed type
* [`featureprofiles/topologies/atedut_8.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_8.testbed)

## Procedure
### Test environment setup

```
DUT has 3 aggregate interfaces.


                         |         | --eBGP-- | ATE Ports 3,4 |
    [ ATE Ports 1,2 ]----|   DUT   |          |               |
                         |         | --eBGP-- | ATE Port 5,6  |
```

Test uses aggregate 802.3ad bundled interfaces (Aggregate Interfaces).

* Send bidirectional traffic:
  * IP to Encap Traffic: The IP to Encap traffic is from ATE Ports [1,2] to ATE Ports [3,4,5,6]. 

  * Encap to IP Traffic: The Encap traffic to IP traffic is from ATE Ports [3,4,5,6] to ATE Ports [1,2].  

Please refer to the MPLSoGRE [encapsulation PF-1.14](feature/policy_forwarding/otg_tests/mpls_gre_ipv4_encap_test/README.md) and [decapsulation PF-1.12](feature/policy_forwarding/otg_tests/mpls_gre_ipv4_decap_test/README.md) READMEs for additional information on the test traffic environment setup.

## PF-1.18.1: Generate DUT Configuration
#### Configuration

### QoS

* Classifier Configuration for Encap to IP Traffic:
    * MPLSoGRE traffic classification to 8 traffic classes based on MPLS EXP bits
    * MPLSoGUE traffic classification to 8 traffic classes based on MPLS EXP bits

* Queueing profiles for Encap to IP Traffic:
    * Bandwidth/assured forwarding class
        * Class with minimum assured bandwidth(interface percentage or absolute value) during congestion
        * Shaping must be supported and configurable to limit a bandwidth class with maximum bandwidth allocation irrespective of congestion state 
        * Configure 5 sub-interfaces with 8 bandwidth classes and ensure 4 or more classes have shaping configured.

    * Priority/expedited forwarding class
        * Configure Priority classes from levels 0-7. 
            * Traffic at higher priority will always starve a lower priority class and any bandwidth class
            * Shaping must be supported and configurable to limit a priority class with maximum bandwidth allocation irrespective of congestion state
            * Configure 5 sub-interfaces with 8 priority classes each, level 0-7
            * Configure 5 sub-interfaces with 8 priority classes and ensure 4 or more high priority classes have shaping configured.

* Marking Configuration for IP to Encap Traffic:
    * MPLSoGRE traffic outer header must be DSCP 32
    * MPLSoGUE traffic outer header must be DSCP 32

* Queueing profiles for IP to Encap Traffic:
    * Priority/expedited forwarding class
        * Configure Priority classes from levels 0-7. 
        * Traffic at higher priority will always starve a lower priority class and any bandwidth class
        * Configure both uplinks Aggregate [3,4] and Aggregate [5,6] with 8 priority classes each, level 0-7

* Classifier Configuration for IP to Encap Traffic:
    * Configure one or more interfaces to classify traffic into traffic class 3
    * Configure one or more interfaces to classify traffic into traffic class 4

* Policer Configuration for IP to Encap Traffic:
    * Configure one or more interfaces with two rate 3 color policer

## PF-1.18.2: Verify Classification of MPLSoGRE and MPLSoGUE traffic based on traffic class bits in MPLS header
Generate MPLSoGRE and MPLSoGUE traffic on ATE Ports 3,4,5,6 having:
* Outer source address: random combination of 1000+ IPV4 source addresses
* Outer destination address: Traffic must fall within the configured IPV4 unicast prefix range for MPLSoGRE and MPLSoGUE traffic.
* MPLS Labels: 
    * Various streams must map to every configured IPV4/IPV6/Multicast static MPLS labels on the device
    * Use all combinations of traffic class bits in the MPLS label (0 to 7)
* Inner payload: 
    * Both IPV4 and IPV6 unicast payloads, with random source address, destination address, TCP/UDP source port and destination ports
    * Multicast traffic
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size.

Verify:
* Egress IP traffic after decapsulation gets classified into 8 queues mapped to 8 traffic classes based on MPLS label as configured on the device.
* Inner packet DSCP is not altered by the device
* All traffic received gets decapsulated and forwarded as IPV4/IPV6 unicast, IPV4 multicast on all the interfaces.
* Verify the inner packets are received by the ATE by validating the inner source and dest IP addresses.

## PF-1.18.3: Verify DSCP marking of encapsulated and decapsulated traffic
Generate bidirectional traffic (MPLSoGRE and MPLSoGUE) as highlighted in the test environment setup section.

Verify:
* IP to Encapsulated traffic has a DSCP 32 set on the outer IP header while the inner IP header DSCP is preserved during the encap operation 
* IP to Encapsulated traffic maps to traffic classes TC3 and TC4 based on ingress configuration
* Encapsulated traffic to IP traffic forwarding and the decap operation does not result in any DSCP rewrite of the inner payload

## PF-1.18.4: Verify Assured forwarding (bandwidth class) -  Queueing of decap traffic (MPLSoGRE to IP traffic - decap operation)
This test is to verify the assured forwarding feature on interfaces (bandwidth only classes): 
* Generate MPLSoGRE and MPLSoGUE traffic on ATE Ports 3,4,5,6 with all 8 values of MPLS experimental bits (0-7) 
* The sum of 8 streams of traffic bandwidth must be minimum 10 percent greater than the total interface bandwidth ensuring congestion 
* The egress interfaces must have 5 or more classes with minimum bandwidth configuration
* The streams across all the classes must be greater than the configured minimum bandwidth
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size

Verify:
* The total conformed bandwidth is equal to the interface bandwidth
* Every class is getting the minimum bandwidth as configured for the class
* Stop sending traffic corresponding to one or two classes and ensure that unused bandwidth is equally shared among other classes with traffic
* Reduce traffic allocation on one or more classes and verify that unused bandwidth is equally shared among other classes with traffic  
* Results are same with 64 byte stream, MTU byte stream and mix of 64..MTU byte streams
* Every queue can transmit packets at line rate without any buffer/tail drops

## PF-1.18.5: Verify Assured forwarding (bandwidth class) -  Queueing of decap traffic with minimum and maximum bandwidth (shaper) 
This test is to verify the assured forwarding feature on interfaces with bandwidth only classes and shaper (maximum bandwidth) configured on 2 or more classes: 
* Generate MPLSoGRE and MPLSoGUE traffic on ATE Ports 3,4,5,6 with all 8 values of MPLS experimental bits (0-7) 
* The sum of 8 streams of traffic bandwidth must be minimum 10 percent greater than the total interface bandwidth ensuring congestion 
* The egress interfaces must have 5 or more classes with minimum bandwidth configuration. 3 or more classes with minimum bandwidth configuration must also have a maximum bandwidth (shaper) configuration
* The streams across all the classes must be greater than the configured minimum  and maximum bandwidth
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size

Verify:
* The total conformed bandwidth is equal to the interface bandwidth
* Every class is getting the minimum bandwidth configured for the class
* Every class having a shaper is not exceeding the shaper/maximum bandwidth 
* Stop sending traffic corresponding to one or two classes and ensure that unused bandwidth is equally shared among other classes with traffic
* Reduce traffic allocation on one or more classes and verify that unused bandwidth is equally shared among other classes with traffic and never exceeds the configured maximum bandwidth in any class with shaper configuration 
* Results are same with 64 byte stream, MTU byte stream and mix of 64-MTU byte streams
* Every queue can transmit packets at line rate without any buffer/tail drops

## PF-1.18.6: Verify Expedited forwarding (Priority class) -  Queueing of decap traffic
This test is to verify the expedited forwarding feature on interfaces with priority only classes: 
* Generate MPLSoGRE and MPLSoGUE traffic on ATE Ports 3,4,5,6 with all 8 values of MPLS experimental bits (0-7).
* The sum of 8 streams of traffic bandwidth must be minimum 10 percent greater than the total interface bandwidth ensuring congestion. 
* The egress interfaces must have 6 or more classes with priority class configuration
* The individual streams bandwidth corresponding to PriortyN class must be minimum 10 percent greater than the PriorityN-1 class
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size.

Verify:
* The total conformed bandwidth is equal to the interface bandwidth.
* Every class with priority level N is starving all the classes with a lower priority level (less than N).
* Stop sending traffic corresponding to one or more highest priority classes and ensure that unused bandwidth is allocated to immediate lower priority classes
* Results are same with 64 byte stream, MTU byte stream and mix of 64-MTU byte streams
* Every queue can transmit packets at line rate without any buffer/tail drops

## PF-1.18.7: Verify Expedited forwarding (Priority class) -  Queueing of decap traffic with minimum and maximum bandwidth (shaper)
This test is to verify the expedited forwarding feature on interfaces with priority only classes and shaper (maximum bandwidth) configured on 2 or more classes. 
* Generate MPLSoGRE and MPLSoGUE traffic on ATE Ports 3,4,5,6 with all 8 values of MPLS experimental bits (0-7).
* The sum of 8 streams of traffic bandwidth must be minimum 10 percent greater than the total interface bandwidth ensuring congestion. 
* The egress interfaces must have 6 or more classes with priority class configuration with one or more classes having shaper configuration
* The individual streams bandwidth corresponding to PriortyN class must be:
    * minimum 10 percent greater than the PriorityN-1
    * greater than the configured shaper bandwidth
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size.

Verify:
* The total conformed bandwidth is equal to the interface bandwidth
* Every class having a shaper is not exceeding the shaper/maximum bandwidth
* Every class with priority level N is starving all the classes with a lower priority level (less than N) but traffic is not greater than the configured shaper bandwidth
* Stop sending traffic corresponding to one or more highest priority classes and ensure that unused bandwidth is allocated to immediate lower priority classes
* Results are same with 64 byte stream, MTU byte stream and mix of 64-MTU byte streams
* Every queue can transmit packets at line rate without any buffer/tail drops

## PF-1.18.8: Verify Expedited forwarding (Priority class) -  Queueing of encap traffic 
This test is to verify the expedited forwarding feature on interfaces with priority only classes. 
* Generate IP traffic on ATE Ports 1,2 with all 8 values of MPLS experimental bits (0-7).
* The sum of 8 streams of traffic bandwidth must be minimum 10 percent greater than the total interface bandwidth ensuring congestion. 
* The egress interfaces must have 6 or more classes with priority class configuration
* Ingress interfaces must classify the traffic under TCO-TC7
* All traffic classes must have encapsulated (MPLSoGRE and MPLSoGUE) egress traffic
* The individual streams bandwidth corresponding to PriortyN class must be minimum 10 percent greater than the PriorityN-1 class
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size.

Verify:
* The total conformed bandwidth is equal to the interface bandwidth.
* Every class with priority level N is starving all the classes with a lower priority level (less than N).
* Stop sending traffic corresponding to one or more highest priority classes and ensure that unused bandwidth is allocated to immediate lower priority classes
* Results are same with 64 byte stream, MTU byte stream and mix of 64-MTU byte streams
* Every queue can transmit packets at line rate without any buffer/tail drops

## PF-1.18.9: Verify two rate three color policer -  Ingress rate limiting of encap traffic 
This test is to verify the two rate, three color policer
* Generate IP traffic on ATE Ports 1,2 with all 8 values of MPLS experimental bits (0-7).
* The sum of 8 streams of traffic bandwidth must be minimum 10 percent greater than the configured peak information rate (PIR) and committed information rate (CIR)
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size.

Verify:
* The total conformed bandwidth is equal to the PIR configured on the bundle and rest of the traffic gets dropped
* The traffic conforming to CIR and exceeding CIR can be selectively marked

## PF-1.18.10: Verify two rate three color policer -  Ingress rate limiting of encap traffic 
This test case is to verify that results corresponding to all above test cases are the same and  irrespective of the distribution of ingress and egress links across different packet processing engines.

Verify results are same corresponding to test cases PF-1.18.1 - PF-1.18.8 with:
* Ingress aggregate links on one PPE and egress aggregate links on different PPE
* Ingress and egress aggregate links on same PPE
* Ingress links on multiple PPEs and egress aggregate links on multiple PPEs

## Canonical OpenConfig for MACsec configuration

TODO: 
* Finalize and update the below paths after the review and testing on any vendor device
* MPLSoGRE/MPLSoGUE packet classification OC need to be defined
* OC for Queueing with shaper need to be defined

### JSON Format

```json
"network-instances": {
  "network-instance": {
    "DEFAULT": {
       "name": "default",
       "policy-forwarding": {
         "policies": {
           "policy": [
              {
                "config": {
                  "policy-id": "decap MPLS in GRE"
                },
                "rules": {
                  "rule": [
                    {
                      "config": {
                        "sequence-id": 1
                      },
                      "ipv4": {
                        "config": {
                          "destination-address": "169.254.125.155/28",
                          "protocol": "IP"
                        },
                        }
                    },
                    "action": {
                        "decapsulate-gre": true,
                        "mpls-classifier": true  #TODO: Add to OC data models
                        }
                      },
                      "sequence-id": 1
                    }
                  ]
                },
              }
           ]
         }
       }
    }
  }
}

```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  ### Telemetry 
  /qos/interfaces/interface/output/queues/queue/state/transmit-pkts:
  /qos/interfaces/interface/output/queues/queue/state/transmit-octets:
  /qos/interfaces/interface/output/queues/queue/state/dropped-pkts:
  /qos/interfaces/interface/output/queues/queue/state/dropped-octets:

   ### Scheduler policy - Strict priority
  /qos/scheduler-policies/scheduler-policy/config/name:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/priority:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/sequence:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/id:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/input-type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue:

   ### Scheduler policy - WRR
  /qos/scheduler-policies/scheduler-policy/config/name:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/priority:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/sequence:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/id:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/input-type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/weight:

   ### Scheduler - Policer
  /qos/scheduler-policies/scheduler-policy/config/name:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/one-rate-two-color/config/cir:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/one-rate-two-color/config/bc:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/one-rate-two-color/config/queuing-behavior:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/one-rate-two-color/exceed-action/config/drop:

  ###Classifier  
  #/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/mpls-classifier: TODO: Add OC path
  /qos/interfaces/interface/input/classifiers/classifier/config/name:
  /qos/interfaces/interface/input/classifiers/classifier/config/type:
  /qos/classifiers/classifier/terms/term/conditions/mpls/config/traffic-class:

```
