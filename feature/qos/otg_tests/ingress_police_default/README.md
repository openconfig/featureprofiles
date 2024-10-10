# DP-2.4 Police traffic on input matching all packets using 1 rate, 2 color marker

## Summary

Use the gRIBI applied ip entries from TE-18.1 gRIBI. 
Configure an ingress scheduler to police traffic using a 1 rate, 2 color policer and attach the scheduler to the interface without a classifier.
Lack of match conditions will cause all packets to be matched. 
Send traffic to validate the policer.

## Topology

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Test setup

Use TE-18.1 test environment setup.

## Procedure

### DP-2.4.1 Generate and push configuration

* Generate config for 2 scheduler polices with an input rate limit.
* Apply scheduler to DUT subinterface with vlan.
* Use gnmi.Replace to push the config to the DUT.

```json
{
  "openconfig-qos": {
    "scheduler-policies": [
      {
        "scheduler-policy": null,
        "config": {
          "name": "limit_1Gb"
        },
        "schedulers": [
          {
            "scheduler": null,
            "config": {
              "sequence": 1,
              "type": "ONE_RATE_TWO_COLOR"
            },
            "inputs": [
              {
                "input": "my input policer 1Gb",
                "config": {
                  "id": "my input policer 1Gb",
                  "input-type": "QUEUE",
                  "queue": "dummy_input_queue_A"
                }
              }
            ],
            "one-rate-two-color": {
              "config": {
                "cir": 1000000000,
                "bc": 100000,
                "queuing-behavior": "POLICE"
              },
              "exceed-action": {
                "config": {
                  "drop": true
                }
              }
            }
          }
        ]
      },
      {
        "scheduler-policy": null,
        "config": {
          "name": "limit_2Gb"
        },
        "schedulers": [
          {
            "scheduler": null,
            "config": {
              "sequence": 1,
              "type": "ONE_RATE_TWO_COLOR"
            },
            "inputs": [
              {
                "input": "my input policer 2Gb",
                "config": {
                  "id": "my input policer 2Gb",
                  "input-type": "QUEUE",
                  "queue": "dummy_input_queue_B"
                }
              }
            ],
            "one-rate-two-color": {
              "config": {
                "cir": 2000000000,
                "bc": 100000,
                "queuing-behavior": "POLICE"
              },
              "exceed-action": {
                "config": {
                  "drop": true
                }
              }
            }
          }
        ]
      }
    ],
    #
    # Interfaces input are mapped to the desired scheduler.
    "interfaces": [
      {
        "interface": null,
        "config": {
          "interface-id": "PortChannel1.100"
        },
        "input": {
          "scheduler-policy": {
            "config": {
              "name": "limit_group_A_1Gb"
            }
          }
        }
      },
      {
        "interface": null,
        "config": {
          "interface-id": "PortChannel1.200"
        },
        "input": {
          "scheduler-policy": {
            "config": {
              "name": "limit_group_B_1Gb"
            }
          }
        }
      }
    ]
  }
}
```

### DP-2.4.2 Test traffic

* Send traffic
  * Send flow A traffic from ATE port 1 to DUT for dest_A at 0.7Gbps (note cir is 1Gbps).
  * Send flow B traffic from ATE port 1 to DUT for to dest_B at 1.5Gbps (note cir is 2Gbps).
  * Validate qos counters per DUT.
  * Validate qos counters by ATE port.
  * Validate packets are received by ATE port 2.
    * Validate DUT qos interface scheduler counters count packets as conforming-pkts and conforming-octets
    * Validate at OTG that 0 packets are lost on flow A and flow B
  * When the outer packet is IPv6, the flow-label should be inspected on the ATE.
    * If the inner packet is IPv4, the outer IPv6 flow label should be computed based on the IPv4 5 tuple src,dst address and ports, plus protocol.
    * If the inner packet is IPv6, the inner flow label should be copied to the outer packet.
    * To validate the flow label, use the ATE to verify that the packets for 
      * flow A all have the same flow label
      * flow B have the same flow label
      * flow A and B labels do not match
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

  # qos interfaces config
  /qos/interfaces/interface/config/interface-id:
  /qos/interfaces/interface/input/scheduler-policy/config/name:

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


