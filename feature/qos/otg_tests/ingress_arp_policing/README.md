# CPT-1.1: Interface based ARP policer

## Summary

Use the gRIBI applied ip entries from TE-18.1 gRIBI. 
Configure an ingress scheduler to police traffic and attach the scheduler to the interface with a classifier. Match the ARP packets. 
Send ARP traffic to validate the policer.

## Topology

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Test setup

Use TE-18.1 test environment setup.

## Procedure

### CPT-1.1 Generate and push configuration

* Generate config for a scheduler which polices with an input rate 1kbps limit and a classifier matching ARP packets.
* Apply scheduler to DUT subinterface with vlan.
* Use gnmi.Replace to push the config to the DUT.


### CPT-1.1 Test traffic

* Send traffic
  * Send a flow of traffic from ATE port 1 to DUT for dest_A at 1.5Kbps (note cir is 1Kbps).
  * Validate qos counters per DUT.
  * Validate qos counters by ATE port.
  * Validate packets are received by ATE port 2.
    * Validate DUT qos interface scheduler counters count packets as conforming-pkts and conforming-octets
    * Validate at OTG that 0.5Kb packets are lost

```json
# qos classifier config

json
{
    "qos": {
        "classifers": {
            "classifier": {
                "config": {
                    "name": "ARP-match",
                    "type": "ETHERNET"
                },
                "terms": {
                    "term": {
                        "config": {
                            "id": "ARP"
                        },
                        "conditions": {
                            "l2": {
                                "config": {
                                    "ethertype": 2054
                                }
                            }
                        },
                        "actions": {
                            "config": {
                                "target-group": "arp policer"
                            },
                        }
                    }
                }
            }
        }
    }
}

# qos scheduler config

{
    "qos": {
         "scheduler-policies": {
             "scheduler-policy": {
                 "config": {
                   "name": "ARP-policer",
                 }, 
            "Scheduler":{        
                "config":{
                        "type": one-rate-two-color
                    },       
                    "one-rate-two-color": {
                    "config": {
                        "cir": 1000000
                        "bc": 0
                        "queueing-behavior":
                    },
                    "exceed-action":
                     "config":{
                         "drop": True
                        },
                }
              } 
            }
         }
    }
}

  # qos interfaces config

  {
    "qos":{
        "interfaces": {
            "interface": {
                "config": {
                    "interface-id":"ethernet 1/1"
                },
                "input": {
                    "classifiers":{
                        "classifier": {
                            "config": {
                                "name": /qos/classifiers/classifier/config/name:ARP-match
                                "type": "IPV4"
                            },
                        },
                    }
                    "scheduler-policy": {
                        "config": {
                            "name": /qos/scheduler-policies/scheduler-policy/config/name:ARP-policer
                        },
                    }                   
                }
            }
        }
    }
  }
```


#### OpenConfig Path and RPC Coverage

```yaml
paths:

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
