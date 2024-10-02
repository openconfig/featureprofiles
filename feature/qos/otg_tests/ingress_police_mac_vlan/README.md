# TE-18.4 Classify traffic on input matching all packets and police using 1 rate, 2 color marker

## Summary

Use the gRIBI applied ip entries from TE-18.1 gRIBI. 
Configure an ingress scheduler to police traffic using a 1 rate, 2 color policer and attach the scheduler to the interface without a classifier. Lack of match conditions will cause all packets to be matched. 
Send traffic to validate the policer.

## Topology

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Test setup

Use TE-18.1 test environment setup.

## Procedure

### TE-18.4.1 Generate and push configuration

* Generate config for 2 scheduler polices with an input rate limit.
* Apply scheduler to DUT subinterface with vlan.
* Use gnmi.Replace to push the config to the DUT.

```yaml
---
openconfig-qos:
  scheduler-policies:
    - scheduler-policy: "limit_2Gb"
      config:
        name: "limit_2Gb"
      schedulers:
        - scheduler: 0
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

    - scheduler-policy: "limit_1Gb"
      config:
        name: "limit_1Gb"
      schedulers:
        - scheduler: 0
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
  input-policies:       # new OC subtree input-policies (/qos/input-policies)
    - input-policy: "limit_group_A_2Gb"
      config:
        name: "limit_group_A_2Gb"
        classifer: "dest_A"
        scheduler-policy: "limit_2Gb"
    - input-policy: "limit_dest_group_B_1Gb"
      config:
        name: "limit_dest_group_B_1Gb"
        classifer: "dest_B"
        scheduler-policy: "limit_1Gb"

  interfaces:                  # this is repeated per subinterface (vlan)
    - interface: "PortChannel1"
      interface-ref:
        config:
          subinterface: 100
    input:
      config:
        policies:  [            # new OC leaf-list (/qos/interfaces/interface/input/config/policies)
          limit_dest_group_A_2Gb
        ]
  interfaces:                  # this is repeated per subinterface (vlan)
    - interface: "PortChannel1"
      interface-ref:
        config:
          subinterface: 200
    input:
      config:
        policies:  [            # new OC leaf-list (/qos/interfaces/interface/input/config/policies)
          limit_dest_group_B_1Gb
        ]

```

### TE-18.4.2 Test traffic

* Send traffic
  * Send traffic from ATE port 1 to DUT for dest_A and is conforming to cir.
  * Send traffic from ATE port 1 to DUT for to dest_B and is conforming to
    cir.
  * Validate packets are received by ATE port 2.
    * Validate qos interface scheduler counters
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

