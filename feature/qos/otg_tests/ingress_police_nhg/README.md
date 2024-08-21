# TE-18.2 QoS scheduler with 1 rate 2 color policer, classifying on next-hop group

## Summary

Use the gRIBI applied ip entries from TE-18.1 gRIBI. Configure an ingress scheduler
to police traffic using a 1 rate, 2 color policer. Configure a classifier to match
traffic on a next-hop-group.  Apply the configuration to a VLAN on an aggregate
interface.  Send traffic to validate the policer.

## Topology

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Test setup

Use TE-18.1 test environment setup.

## Procedure

### TE-18.3.1 Generate and push configuration

* Generate config for 2 scheduler polices with an input rate limit.  
* Generate config for 2 classifiers which match on next-hop-group.
* Generate config for 2 input policies which map the scheduler and classifers
  together.
* Generate config to apply classifer and scheduler to DUT subinterface with vlan.
* Use gnmi.Replace to push the config to the DUT.

```yaml
---
openconfig-qos:
  policer-policies:
    - policer-policy: "limit_2Gb"
      config:
        name: "limit_2Gb"
      policers:
        - policer: 0
          config:
              type: ONE_RATE_TWO_COLOR
              sequence: 0
          one-rate-two-color:
            config:
              cir: 2000000000           # 2Gbit/sec
              bc: 100000                 # 100 kilobytes
              queuing-behavior: POLICE
            exceed-action:
              config:
                drop: TRUE

    - policer-policy: "limit_1Gb"
      config:
        name: "limit_1Gb"
      policers:
        - policer: 0
          config:
              type: ONE_RATE_TWO_COLOR
              sequence: 0
          one-rate-two-color:
            config:
              cir: 1000000000           # 1Gbit/sec
              bc: 100000                # 100 kilobytes
              queuing-behavior: POLICE
            exceed-action:
              config:
                drop: TRUE
  classifers:
    - classifer: “dest_A”
      config:
        name: “dest_A”
      terms:
        - term:   # repeated for address in destination A
          config:
            id: "match_1_dest_A"
          conditions:
            next-hop-group:
                config:
                    name: "nhg_A"     # new OC path needed, string related to /afts/next-hop-groups/next-hop-group/state/next-hop-group-id
          actions:
            config:
              policer-policy: "limit_group_A_2Gb"  # new OC path needed
    - classifer: “dest_B”
      config:
        name: “dest_B”
      terms:
        - term:
          config:
            id: "match_1_dest_B"
          conditions:
            next-hop-group:
                config:
                    name: "nhg_B"     # new OC path needed, string related to /afts/next-hop-groups/next-hop-group/state/next-hop-group-id
          actions:
            config:
              policer-policy: "limit_group_B_1Gb"  # new OC path needed

  interfaces:                  # this is repeated per subinterface (vlan)
    - interface: "PortChannel1.100"
        config:
          interface-id: "PortChannel1.100"
        input:
          classifers:
            - classifier:
              config:
                name: "dest_A"
                type: "IPV4"
          policer-policy:     # New OC subtree /qos/interfaces/interface/policer-policy
            config:
              name: "limit_2G"
    - interface: "PortChannel1.200"
        config:
          interface-id: "PortChannel1.200"
        input:
          classifers:
            - classifier:
              config:
                name: "dest_B"
                type: "IPV4"
          policer-policy:    # New OC subtree /qos/interfaces/interface/policer-policy
            config:
              name: "limit_1G"
```

### TE-18.3.1 Test traffic

* Send traffic
  * Send traffic from ATE port 1 to DUT for dest_A and is conforming to cir.
  * Send traffic from ATE port 1 to DUT for to dest_B and is conforming to
    cir.
  * Validate packets are received by ATE port 2.
    * Validate qos interface scheduler counters
    * Validate afts next hop counters
  * Validate outer packet ipv6 flow label assignment
    * When the outer packet is IPv6, the flow-label should be inspected on the ATE.
      * If the inner packet is IPv4, the outer IPv6 flow label should be computed based on the IPv4 5 tuple src,dst address and ports, plus protocol
      * If the inner packet is IPv6, the inner flow label should be copied to the outer packet.
  * Increase traffic on flow to dest_B to 2Gbps
    * Validate that flow dest_B experiences ~50% packet loss (+/- 1%)

#### OpenConfig Path and RPC Coverage

```yaml
paths:
  # qos scheduler config
  /qos/scheduler-policies/scheduler-policy/config/name:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/one-rate-two-color/config/cir:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/one-rate-two-color/config/bc:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/one-rate-two-color/config/queuing-behavior:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/one-rate-two-color/exceed-action/config/drop:

  # qos classifier config
  /qos/classifiers/classifier/config/name:
  /qos/classifiers/classifier/terms/term/config/id:
  #/qos/classifiers/classifier/terms/term/conditions/next-hop-group/config/name: # TODO: new OC leaf to be added

  # qos policer config - TODO: a new OC subtree (/qos/policer-policies, essentially copying/moving policer action from schedulers)
  # /qos/policer-policies/policer-policy/config/name:
  # /qos/policer-policies/policer-policy/config/policers/policer/config/sequence:
  # /qos/policer-policies/policer-policy/config/policers/policer/one-rate-two-color/config/cir:
  # /qos/policer-policies/policer-policy/config/policers/policer/one-rate-two-color/config/bc:
  # /qos/policer-policies/policer-policy/config/policers/policer/one-rate-two-color/config/cir:
  # /qos/policer-policies/policer-policy/config/policers/policer/one-rate-two-color/exceed-action/config/drop:

  # qos interface scheduler counters
  /qos/interfaces/interface/input/scheduler-policy/schedulers/scheduler/state/conforming-pkts:
  /qos/interfaces/interface/input/scheduler-policy/schedulers/scheduler/state/conforming-octets:
  /qos/interfaces/interface/input/scheduler-policy/schedulers/scheduler/state/exceeding-pkts:
  /qos/interfaces/interface/input/scheduler-policy/schedulers/scheduler/state/exceeding-octets:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

* FFF

