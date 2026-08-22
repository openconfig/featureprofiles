# DP-1.17: DSCP Transparency with ECN

## Summary

This test evaluates if all 64 combination of DSCP bits are transparently handled while ECN bits are rewritten.

## Testbed type

* TESTBED_DUT_ATE_4LINKS

## Procedure

### Testbed configuration
* Connect DUTPort1 with OTGPort1, DUTPort2 with OTGPort2, DUTPort3 with OTGPort3; Assign IPv4 and IPv6 addresses on all.
* All 3 ports are of the same speed (100GE)
* Configure QoS
    * DSCP classifier for IPv4 and IPv6 as below:
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
       * one scheduler of WRR type serving 6 queues AF4, AF3, AF2, AF1, BE0, BE1 with equal weights 10:10:10:10:10:10 respectively
    * queue-management profile of WRED type with:
       * min-threshold: 80KB
       * max-threshold: 3MB
       * max-drop-percentage: 100 
       * ecn: enabled
    * attach queue-management profile to queues NC1, AF4, AF3, AF2, AF1, BE0, BE1;
    * attach scheduler-map to DUTPort1 egress
    * attach classifier to DUTPort2 and DUTPort3 ingress

#### Canonical OC

```json
{
  "qos": {
    "classifiers": {
      "classifier": [
        {
          "config": {
            "name": "dscp_based_classifier_ipv4",
            "type": "IPV4"
          },
          "name": "dscp_based_classifier_ipv4",
          "terms": {
            "term": [
              {
                "actions": {
                  "config": {
                    "target-group": "target-group-BE1"
                  }
                },
                "conditions": {
                  "ipv4": {
                    "config": {
                      "dscp-set": [
                        0,
                        1,
                        2,
                        3
                      ]
                    }
                  }
                },
                "config": {
                  "id": "0"
                },
                "id": "0"
              },
              {
                "actions": {
                  "config": {
                    "target-group": "target-group-BE0"
                  }
                },
                "conditions": {
                  "ipv4": {
                    "config": {
                      "dscp-set": [
                        4,
                        5,
                        6,
                        7
                      ]
                    }
                  }
                },
                "config": {
                  "id": "1"
                },
                "id": "1"
              },
              {
                "actions": {
                  "config": {
                    "target-group": "target-group-AF1"
                  }
                },
                "conditions": {
                  "ipv4": {
                    "config": {
                      "dscp-set": [
                        8,
                        9,
                        10,
                        11,
                        12,
                        13,
                        14,
                        15
                      ]
                    }
                  }
                },
                "config": {
                  "id": "2"
                },
                "id": "2"
              },
              {
                "actions": {
                  "config": {
                    "target-group": "target-group-AF2"
                  }
                },
                "conditions": {
                  "ipv4": {
                    "config": {
                      "dscp-set": [
                        16,
                        17,
                        18,
                        19,
                        20,
                        21,
                        22,
                        23
                      ]
                    }
                  }
                },
                "config": {
                  "id": "3"
                },
                "id": "3"
              },
              {
                "actions": {
                  "config": {
                    "target-group": "target-group-AF3"
                  }
                },
                "conditions": {
                  "ipv4": {
                    "config": {
                      "dscp-set": [
                        24,
                        25,
                        26,
                        27,
                        28,
                        29,
                        30,
                        31
                      ]
                    }
                  }
                },
                "config": {
                  "id": "4"
                },
                "id": "4"
              },
              {
                "actions": {
                  "config": {
                    "target-group": "target-group-AF4"
                  }
                },
                "conditions": {
                  "ipv4": {
                    "config": {
                      "dscp-set": [
                        32,
                        33,
                        34,
                        35,
                        36,
                        37,
                        38,
                        39,
                        40,
                        41,
                        42,
                        43,
                        44,
                        45,
                        46,
                        47
                      ]
                    }
                  }
                },
                "config": {
                  "id": "5"
                },
                "id": "5"
              },
              {
                "actions": {
                  "config": {
                    "target-group": "target-group-NC1"
                  }
                },
                "conditions": {
                  "ipv4": {
                    "config": {
                      "dscp-set": [
                        48,
                        49,
                        50,
                        51,
                        52,
                        53,
                        54,
                        55,
                        56,
                        57,
                        58,
                        59,
                        60,
                        61,
                        62,
                        63
                      ]
                    }
                  }
                },
                "config": {
                  "id": "6"
                },
                "id": "6"
              }
            ]
          }
        },
        {
          "config": {
            "name": "dscp_based_classifier_ipv6",
            "type": "IPV6"
          },
          "name": "dscp_based_classifier_ipv6",
          "terms": {
            "term": [
              {
                "actions": {
                  "config": {
                    "target-group": "target-group-BE1"
                  }
                },
                "conditions": {
                  "ipv6": {
                    "config": {
                      "dscp-set": [
                        0,
                        1,
                        2,
                        3
                      ]
                    }
                  }
                },
                "config": {
                  "id": "0"
                },
                "id": "0"
              },
              {
                "actions": {
                  "config": {
                    "target-group": "target-group-BE0"
                  }
                },
                "conditions": {
                  "ipv6": {
                    "config": {
                      "dscp-set": [
                        4,
                        5,
                        6,
                        7
                      ]
                    }
                  }
                },
                "config": {
                  "id": "1"
                },
                "id": "1"
              },
              {
                "actions": {
                  "config": {
                    "target-group": "target-group-AF1"
                  }
                },
                "conditions": {
                  "ipv6": {
                    "config": {
                      "dscp-set": [
                        8,
                        9,
                        10,
                        11,
                        12,
                        13,
                        14,
                        15
                      ]
                    }
                  }
                },
                "config": {
                  "id": "2"
                },
                "id": "2"
              },
              {
                "actions": {
                  "config": {
                    "target-group": "target-group-AF2"
                  }
                },
                "conditions": {
                  "ipv6": {
                    "config": {
                      "dscp-set": [
                        16,
                        17,
                        18,
                        19,
                        20,
                        21,
                        22,
                        23
                      ]
                    }
                  }
                },
                "config": {
                  "id": "3"
                },
                "id": "3"
              },
              {
                "actions": {
                  "config": {
                    "target-group": "target-group-AF3"
                  }
                },
                "conditions": {
                  "ipv6": {
                    "config": {
                      "dscp-set": [
                        24,
                        25,
                        26,
                        27,
                        28,
                        29,
                        30,
                        31
                      ]
                    }
                  }
                },
                "config": {
                  "id": "4"
                },
                "id": "4"
              },
              {
                "actions": {
                  "config": {
                    "target-group": "target-group-AF4"
                  }
                },
                "conditions": {
                  "ipv6": {
                    "config": {
                      "dscp-set": [
                        32,
                        33,
                        34,
                        35,
                        36,
                        37,
                        38,
                        39,
                        40,
                        41,
                        42,
                        43,
                        44,
                        45,
                        46,
                        47
                      ]
                    }
                  }
                },
                "config": {
                  "id": "5"
                },
                "id": "5"
              },
              {
                "actions": {
                  "config": {
                    "target-group": "target-group-NC1"
                  }
                },
                "conditions": {
                  "ipv6": {
                    "config": {
                      "dscp-set": [
                        48,
                        49,
                        50,
                        51,
                        52,
                        53,
                        54,
                        55,
                        56,
                        57,
                        58,
                        59,
                        60,
                        61,
                        62,
                        63
                      ]
                    }
                  }
                },
                "config": {
                  "id": "6"
                },
                "id": "6"
              }
            ]
          }
        }
      ]
    },
    "forwarding-groups": {
      "forwarding-group": [
        {
          "config": {
            "name": "target-group-AF1",
            "output-queue": "AF1"
          },
          "name": "target-group-AF1"
        },
        {
          "config": {
            "name": "target-group-AF2",
            "output-queue": "AF2"
          },
          "name": "target-group-AF2"
        },
        {
          "config": {
            "name": "target-group-AF3",
            "output-queue": "AF3"
          },
          "name": "target-group-AF3"
        },
        {
          "config": {
            "name": "target-group-AF4",
            "output-queue": "AF4"
          },
          "name": "target-group-AF4"
        },
        {
          "config": {
            "name": "target-group-BE0",
            "output-queue": "BE0"
          },
          "name": "target-group-BE0"
        },
        {
          "config": {
            "name": "target-group-BE1",
            "output-queue": "BE1"
          },
          "name": "target-group-BE1"
        },
        {
          "config": {
            "name": "target-group-NC1",
            "output-queue": "NC1"
          },
          "name": "target-group-NC1"
        }
      ]
    },
    "interfaces": {
      "interface": [
        {
          "config": {
            "interface-id": "port1"
          },
          "interface-id": "port1",
          "output": {
            "queues": {
              "queue": [
                {
                  "config": {
                    "name": "AF1",
                    "queue-management-profile": "queueManagementProfile"
                  },
                  "name": "AF1"
                },
                {
                  "config": {
                    "name": "AF2",
                    "queue-management-profile": "queueManagementProfile"
                  },
                  "name": "AF2"
                },
                {
                  "config": {
                    "name": "AF3",
                    "queue-management-profile": "queueManagementProfile"
                  },
                  "name": "AF3"
                },
                {
                  "config": {
                    "name": "AF4",
                    "queue-management-profile": "queueManagementProfile"
                  },
                  "name": "AF4"
                },
                {
                  "config": {
                    "name": "BE0",
                    "queue-management-profile": "queueManagementProfile"
                  },
                  "name": "BE0"
                },
                {
                  "config": {
                    "name": "BE1",
                    "queue-management-profile": "queueManagementProfile"
                  },
                  "name": "BE1"
                },
                {
                  "config": {
                    "name": "NC1",
                    "queue-management-profile": "queueManagementProfile"
                  },
                  "name": "NC1"
                }
              ]
            },
            "scheduler-policy": {
              "config": {
                "name": "schedulerPolicy"
              }
            }
          }
        },
        {
          "config": {
            "interface-id": "port2"
          },
          "input": {
            "classifiers": {
              "classifier": [
                {
                  "config": {
                    "name": "dscp_based_classifier_ipv4",
                    "type": "IPV4"
                  },
                  "type": "IPV4"
                },
                {
                  "config": {
                    "name": "dscp_based_classifier_ipv6",
                    "type": "IPV6"
                  },
                  "type": "IPV6"
                }
              ]
            }
          },
          "interface-id": "port2"
        },
        {
          "config": {
            "interface-id": "port3"
          },
          "input": {
            "classifiers": {
              "classifier": [
                {
                  "config": {
                    "name": "dscp_based_classifier_ipv4",
                    "type": "IPV4"
                  },
                  "type": "IPV4"
                },
                {
                  "config": {
                    "name": "dscp_based_classifier_ipv6",
                    "type": "IPV6"
                  },
                  "type": "IPV6"
                }
              ]
            }
          },
          "interface-id": "port3"
        }
      ]
    },
    "queue-management-profiles": {
      "queue-management-profile": [
        {
          "config": {
            "name": "queueManagementProfile"
          },
          "name": "queueManagementProfile",
          "wred": {
            "uniform": {
              "config": {
                "enable-ecn": true,
                "max-drop-probability-percent": 100,
                "max-threshold": "3000000",
                "min-threshold": "80000"
              }
            }
          }
        }
      ]
    },
    "queues": {
      "queue": [
        {
          "config": {
            "name": "AF1"
          },
          "name": "AF1"
        },
        {
          "config": {
            "name": "AF2"
          },
          "name": "AF2"
        },
        {
          "config": {
            "name": "AF3"
          },
          "name": "AF3"
        },
        {
          "config": {
            "name": "AF4"
          },
          "name": "AF4"
        },
        {
          "config": {
            "name": "BE0"
          },
          "name": "BE0"
        },
        {
          "config": {
            "name": "BE1"
          },
          "name": "BE1"
        },
        {
          "config": {
            "name": "NC1"
          },
          "name": "NC1"
        }
      ]
    },
    "scheduler-policies": {
      "scheduler-policy": [
        {
          "config": {
            "name": "schedulerPolicy"
          },
          "name": "schedulerPolicy",
          "schedulers": {
            "scheduler": [
              {
                "config": {
                  "priority": "STRICT",
                  "sequence": 0
                },
                "inputs": {
                  "input": [
                    {
                      "config": {
                        "id": "NC1",
                        "input-type": "QUEUE",
                        "queue": "NC1"
                      },
                      "id": "NC1"
                    }
                  ]
                },
                "sequence": 0
              },
              {
                "config": {
                  "sequence": 1
                },
                "inputs": {
                  "input": [
                    {
                      "config": {
                        "id": "AF1",
                        "input-type": "QUEUE",
                        "queue": "AF1",
                        "weight": "10"
                      },
                      "id": "AF1"
                    },
                    {
                      "config": {
                        "id": "AF2",
                        "input-type": "QUEUE",
                        "queue": "AF2",
                        "weight": "10"
                      },
                      "id": "AF2"
                    },
                    {
                      "config": {
                        "id": "AF3",
                        "input-type": "QUEUE",
                        "queue": "AF3",
                        "weight": "10"
                      },
                      "id": "AF3"
                    },
                    {
                      "config": {
                        "id": "AF4",
                        "input-type": "QUEUE",
                        "queue": "AF4",
                        "weight": "10"
                      },
                      "id": "AF4"
                    },
                    {
                      "config": {
                        "id": "BE0",
                        "input-type": "QUEUE",
                        "queue": "BE0",
                        "weight": "10"
                      },
                      "id": "BE0"
                    },
                    {
                      "config": {
                        "id": "BE1",
                        "input-type": "QUEUE",
                        "queue": "BE1",
                        "weight": "10"
                      },
                      "id": "BE1"
                    }
                  ]
                },
                "sequence": 1
              }
            ]
          }
        }
      ]
    }
  }
}
```

### Sub Test #1 - No-Congestion 
* Generate 7 flows of traffic form ATEPort1 toward ATEPort3
    * each flow corresponds to a QoS queue and has multiple distinct DSCP values
    * every packet has ECT(0) set
    * flows are configured with appropriate bps rates.
    * total load - 60% (60Gbps)
* wait 1 minutes; stop traffic generation.
* Verify using DUTPort3 telemetry that:
    * no drops are seen in any of queues on DUTPort3
    * all queues reports non-zero transmit packets, octets.
* Verify on ATEPort3 that all flows are received w/o DSCP modification -all 64 values are observed
* verify on ATEPort3 that all received packet has ECT(0) ECN value

### Sub Test #2 - Congestion
* Generate 7 flows of traffic form ATEPort1 and 7 flows of traffic form ATEPort2 toward ATEPort3
    * each flow form ATEPort1 corresponds to a QoS queue and has multiple distinct DSCP values
    * each flow form ATEPort2 corresponds to a QoS queue and has multiple distinct DSCP values
    * every packet has ECT(0) set
    * flows are configured with appropriate bps rates.
    * Offered load:
        * ATEPort1 - 60% (60Gbps)
        * ATEPort2 - 60% (60Gbps)
    * Note: egress port is congested, so do all queues but NC1 (SP)
* wait 1 minutes; stop traffic generation.
* Verify using DUTPort3 telemetry that:
    * Drops are seen in all queues except NC1 on DUTPort3
    * all queues reports non-zero transmit packets, octets.
* Verify on ATEPort3 that all flows are received w/o DSCP modification - all 64 values are observed
* verify on ATEPort3 that:
    * all received packets with DSCP 48-63 has ECT(0) value
    * vast majority (almost all) packets with DSCP 0-47 has CE ECN value.

### Sub Test #3 - NC1 congestion
* Generate 1 flow of traffic form ATEPort1 and 1 flow of traffic form ATEPort2 toward ATEPort3
    * each flow form ATEPort1 has multiple distinct DSCP values from 48-63 range
    * each flow form ATEPort2 has multiple distinct DSCP values from 48-63 range
    * every packet has ECT(0) set
    * flows are configured with appropriate bps rates.
    * Offered load:
    * ATEPort1 - 60% (60Gbps)
    * ATEPort2 - 60% (60Gbps)
    * Note: egress port is congested, so do NC1 (SP) queue
* wait 1 minutes; stop traffic generation.
* Verify using DUTPort3 telemetry that:
    * Drops are seen in NC1 queue on DUTPort3
    * all queues but NC1 reports zero transmit packets, octets.
    * NC1 queue reports non-zero transmit packets, octets.
* Verify on ATEPort3 that all flows are received w/o DSCP modification - all 16 values are observed.
* verify on ATEPort3 that:
    * all received packets with DSCP has CE value

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC paths used for test setup are not listed here.

```yaml
paths:
  ## Config Paths ##
  /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set:
  /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set:
  /qos/classifiers/classifier/terms/term/actions/config/target-group:
  /qos/queues/queue/config/name:
  /qos/forwarding-groups/forwarding-group/config/name:
  /qos/forwarding-groups/forwarding-group/config/output-queue:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/priority:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/sequence:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/id:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/input-type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/weight:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/enable-ecn:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-drop-probability-percent:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-threshold:
  /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/min-threshold:
  /qos/interfaces/interface/output/queues/queue/config/name:
  /qos/interfaces/interface/output/queues/queue/config/queue-management-profile:
  /qos/interfaces/interface/output/scheduler-policy/config/name:
  /qos/interfaces/interface/input/classifiers/classifier/config/name:
  /qos/interfaces/interface/input/classifiers/classifier/config/type:
    
  ## State Paths ##
  /qos/interfaces/interface/output/queues/queue/state/dropped-octets:
  /qos/interfaces/interface/output/queues/queue/state/dropped-pkts:
  /qos/interfaces/interface/output/queues/queue/state/name:
  /qos/interfaces/interface/output/queues/queue/state/transmit-octets:
  /qos/interfaces/interface/output/queues/queue/state/transmit-pkts:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Required DUT platform

* FFF
