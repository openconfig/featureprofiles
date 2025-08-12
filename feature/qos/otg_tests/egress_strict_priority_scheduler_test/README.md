# DP-1.15: Egress Strict Priority scheduler

## Summary

This test validates the proper functionality of an egress strict priority scheduler on a network device. By configuring multiple priority queues with specific traffic classes and generating traffic loads that exceed interface capacity, we will verify that the scheduler adheres to the strict priority scheme, prioritizing higher-priority traffic even under congestion.

## Testbed type

*  [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

*   DUT has 2 ingress ports and 1 egress port with the same port speed. The
    interface can be a physical interface or LACP bundle interface with the
    same aggregated speed.

    ```
                                |         | ---- | ATE Port 1 |
        [ ATE Port 3 ] ----  |   DUT   |      |            |
                                |         | ---- | ATE Port 2 |
    ```

*   Traffic classes:

    *   We will use 6 traffic classes NC1, AF4, AF3, AF2, AF1 and BE1.

*   Traffic types:

    *   All the traffic tests apply to both IPv4 and IPv6 and also MPLS traffic.

*   Queue types:

    *   NC1/AF4/AF3/AF2/AF1/BE1 will have strict priority queues (be1 - priority 6, af1 - priority 5, ..., nc1 - priority 1)

*   Test results should be independent of the location of interfaces. For
    example, 2 input interfaces and output interface could be located on

    *   Same ASIC-based forwarding engine
    *   Different ASIC-based forwarding engine on same line card
    *   Different ASIC-based forwarding engine on different line cards

*   Test results should be the same for port speeds 100G and 400G.

*   Counters should be also verified for each test case:

    *   /qos/interfaces/interface/output/queues/queue/state/transmit-pkts
    *   /qos/interfaces/interface/output/queues/queue/state/dropped-pkts
    *   transmit-pkts should be equal to the number of Rx pkts on Ixia port
    *   dropped-pkts should be equal to diff between the number of Tx and the
        number Rx pkts on Ixia ports

*   Latency:

    *   Should be < 100000ns

#### Configuration

*   Forwarding Classes: Configure six forwarding classes (be1, af1, af2, af3, af4, nc1) based on the classification table provided.
*   Egress Scheduler: Apply a multi-level strict-priority scheduling policy on the desired egress interface. Assign priorities to each forwarding class according to the strict priority test traffic tables (be1 - priority 6, af1 - priority 5, ..., nc1 - priority 1).

* Classification table

    IPv4 TOS      |       IPv6 TC           |         MPLS EXP        |    Forwarding class
    ------------- | ----------------------- | ----------------------- | ---------------------
    0             |      0-7                |          0              |         be1
    1             |      8-15               |          1              |         af1
    2             |      16-23              |          2              |         af2
    3             |      24-31              |          3              |         af3
    4,5           |      32-47              |          4,5            |         af4
    6,7           |      48-63              |          6,7            |         nc1

### DP-1.15.1: Egress Strict Priority scheduler for IPv4 Traffic

*   Traffic Generation:
    *   Traffic Profiles: Define traffic profiles for each forwarding class using the ATE, adhering to the linerates (%) specified in the strict priority test traffic tables.

        *   Strict Priority Test traffic table for ATE Port 1

        Forwarding class  |      Priority        |     Traffic linerate  (%)   |      Frame size        |    Expected Loss %
        ----------------- |--------------------- | --------------------------- |---------------------   | -----------------------------------
        be1               |      6               |          12                 |      512               |         100
        af1               |      5               |          12                 |      512               |         100
        af2               |      4               |          10                 |      512               |         50
        af3               |      3               |          12                 |      512               |         0
        af4               |      2               |          30                 |      512               |         0
        nc1               |      1               |          1                  |      512               |         0

        *   Strict Priority Test traffic table for ATE Port 2

        Forwarding class  |      Priority        |     Traffic linerate  (%)   |      Frame size        |    Expected Loss %
        ----------------- |--------------------- | --------------------------- |---------------------   | -----------------------------------
        be1               |      6               |          12                 |      512               |         100
        af1               |      5               |          12                 |      512               |         100
        af2               |      4               |          10                 |      512               |         50
        af3               |      3               |          12                 |      512               |         0
        af4               |      2               |          30                 |      512               |         0
        nc1               |      1               |          1                  |      512               |         0


* Verification:
    * Loss Rate: Capture packet loss for every generated flow and verify that loss for each flow does not exceed expected loss specified in the tables above.
    * Telemetry: Utilize OpenConfig telemetry parameters to validate that per queue dropped packets statistics corresponds (with error margin) to the packet loss reported for every flow matching that particular queue.

### DP-1.15.2: Egress Strict Priority scheduler for IPv6 Traffic

*   Traffic Generation:
    *   Traffic Profiles: Define traffic profiles for each forwarding class using the ATE, adhering to the linerates (%) specified in the strict priority test traffic tables.

        *   Strict Priority Test traffic table for ATE Port 1

        Forwarding class  |      Priority        |     Traffic linerate  (%)   |      Frame size        |    Expected Loss %
        ----------------- |--------------------- | --------------------------- |---------------------   | -----------------------------------
        be1               |      6               |          12                 |      512               |         100
        af1               |      5               |          12                 |      512               |         100
        af2               |      4               |          10                 |      512               |         50
        af3               |      3               |          12                 |      512               |         0
        af4               |      2               |          30                 |      512               |         0
        nc1               |      1               |          1                  |      512               |         0

        *   Strict Priority Test traffic table for ATE Port 2

        Forwarding class  |      Priority        |     Traffic linerate  (%)   |      Frame size        |    Expected Loss %
        ----------------- |--------------------- | --------------------------- |---------------------   | -----------------------------------
        be1               |      6               |          12                 |      512               |         100
        af1               |      5               |          12                 |      512               |         100
        af2               |      4               |          10                 |      512               |         50
        af3               |      3               |          12                 |      512               |         0
        af4               |      2               |          30                 |      512               |         0
        nc1               |      1               |          1                  |      512               |         0


* Verification:
    * Loss Rate: Capture packet loss for every generated flow and verify that loss for each flow does not exceed expected loss specified in the tables above.
    * Telemetry: Utilize OpenConfig telemetry parameters to validate that per queue dropped packets statistics corresponds (with error margin) to the packet loss reported for every flow matching that particular queue.

### DP-1.15.3: Egress Strict Priority scheduler for MPLS Traffic

*   Traffic Generation:
    *   Traffic Profiles: Define traffic profiles for each forwarding class using the ATE, adhering to the linerates (%) specified in the strict priority test traffic tables.

        *   Strict Priority Test traffic table for ATE Port 1

        Forwarding class  |      Priority        |     Traffic linerate  (%)   |      Frame size        |    Expected Loss %
        ----------------- |--------------------- | --------------------------- |---------------------   | -----------------------------------
        be1               |      6               |          12                 |      512               |         100
        af1               |      5               |          12                 |      512               |         100
        af2               |      4               |          10                 |      512               |         50
        af3               |      3               |          12                 |      512               |         0
        af4               |      2               |          30                 |      512               |         0
        nc1               |      1               |          1                  |      512               |         0

        *   Strict Priority Test traffic table for ATE Port 2

        Forwarding class  |      Priority        |     Traffic linerate  (%)   |      Frame size        |    Expected Loss %
        ----------------- |--------------------- | --------------------------- |---------------------   | -----------------------------------
        be1               |      6               |          12                 |      512               |         100
        af1               |      5               |          12                 |      512               |         100
        af2               |      4               |          10                 |      512               |         50
        af3               |      3               |          12                 |      512               |         0
        af4               |      2               |          30                 |      512               |         0
        nc1               |      1               |          1                  |      512               |         0


* Verification:
    * Loss Rate: Capture packet loss for every generated flow and verify that loss for each flow does not exceed expected loss specified in the tables above.
    * Telemetry: Utilize OpenConfig telemetry parameters to validate that per queue dropped packets statistics corresponds (with error margin) to the packet loss reported for every flow matching that particular queue.

## Canonical OC
```json
{
  "qos": {
    "openconfig-qos:classifiers": {
      "classifier": [
        {
          "config": {
            "name": "dscp_based_classifier_ipv4",
            "type": "IPV4"
          },
          "name": "dscp_based_classifier_ipv4",
          "state": {
            "name": "dscp_based_classifier_ipv4",
            "type": "IPV4"
          },
          "terms": {
            "term": [
              {
                "actions": {
                  "config": {
                    "target-group": "target-group-BE1"
                  },
                  "state": {
                    "target-group": "target-group-BE1"
                  }
                },
                "conditions": {
                  "ipv4": {
                    "config": {
                      "dscp-set": [0]
                    },
                    "state": {
                      "dscp": 0
                    }
                  }
                },
                "config": {
                  "id": "0"
                },
                "id": "0",
                "state": {
                  "id": "0"
                }
              },
              {
                "actions": {
                  "config": {
                    "target-group": "target-group-AF1"
                  },
                  "state": {
                    "target-group": "target-group-AF1"
                  }
                },
                "conditions": {
                  "ipv4": {
                    "config": {
                      "dscp-set": [1]
                    },
                    "state": {
                      "dscp": 1
                    }
                  }
                },
                "config": {
                  "id": "1"
                },
                "id": "1",
                "state": {
                  "id": "1"
                }
              },
              {
                "actions": {
                  "config": {
                    "target-group": "target-group-AF2"
                  },
                  "state": {
                    "target-group": "target-group-AF2"
                  }
                },
                "conditions": {
                  "ipv4": {
                    "config": {
                      "dscp-set": [2]
                    },
                    "state": {
                      "dscp": 2
                    }
                  }
                },
                "config": {
                  "id": "2"
                },
                "id": "2",
                "state": {
                  "id": "2"
                }
              },
              {
                "actions": {
                  "config": {
                    "target-group": "target-group-AF3"
                  },
                  "state": {
                    "target-group": "target-group-AF3"
                  }
                },
                "conditions": {
                  "ipv4": {
                    "config": {
                      "dscp-set": [3]
                    },
                    "state": {
                      "dscp": 3
                    }
                  }
                },
                "config": {
                  "id": "3"
                },
                "id": "3",
                "state": {
                  "id": "3"
                }
              },
              {
                "actions": {
                  "config": {
                    "target-group": "target-group-AF4"
                  },
                  "state": {
                    "target-group": "target-group-AF4"
                  }
                },
                "conditions": {
                  "ipv4": {
                    "config": {
                      "dscp-set": [4, 5]
                    },
                    "state": {
                      "dscp-set": [4, 5]
                    }
                  }
                },
                "config": {
                  "id": "4"
                },
                "id": "4",
                "state": {
                  "id": "4"
                }
              },
              {
                "actions": {
                  "config": {
                    "target-group": "target-group-NC1"
                  },
                  "state": {
                    "target-group": "target-group-NC1"
                  }
                },
                "conditions": {
                  "ipv4": {
                    "config": {
                      "dscp-set": [6, 7]
                    },
                    "state": {
                      "dscp-set": [6, 7]
                    }
                  }
                },
                "config": {
                  "id": "5"
                },
                "id": "5",
                "state": {
                  "id": "5"
                }
              }
            ]
          }
        }
      ]
    },
    "openconfig-qos:forwarding-groups": {
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
    "openconfig-qos:interfaces": {
      "interface": [
        {
          "config": {
            "interface-id": "Ethernet3/1"
          },
          "interface-id": "Ethernet3/1"
        },
        {
          "config": {
            "interface-id": "Ethernet3/1.1"
          },
          "interface-id": "Ethernet3/1.1"
        },
        {
          "config": {
            "interface-id": "Ethernet3/1.2"
          },
          "interface-id": "Ethernet3/1.2"
        },
        {
          "config": {
            "interface-id": "Ethernet3/1.3"
          },
          "interface-id": "Ethernet3/1.3"
        },
        {
          "config": {
            "interface-id": "Ethernet3/1.4"
          },
          "interface-id": "Ethernet3/1.4"
        }
      ]
    },
    "openconfig-qos:queues": {
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
    "openconfig-qos:scheduler-policies": {
      "scheduler-policy": [
        {
          "config": {
            "name": "scheduler"
          },
          "name": "scheduler",
          "schedulers": {
            "scheduler": [
              {
                "config": {
                  "priority": "STRICT",
                  "sequence": 5,
                  "type": "openconfig-qos-types:ONE_RATE_TWO_COLOR"
                },
                "inputs": {
                  "input": [
                    {
                      "config": {
                        "id": "AF1",
                        "input-type": "QUEUE",
                        "queue": "AF1"
                      },
                      "id": "AF1",
                      "state": {
                        "id": "AF1",
                        "input-type": "QUEUE",
                        "queue": "AF1"
                      }
                    }
                  ]
                },
                "sequence": 5,
                "state": {
                  "priority": "STRICT",
                  "sequence": 5,
                  "type": "openconfig-qos-types:ONE_RATE_TWO_COLOR"
                }
              },
              {
                "config": {
                  "priority": "STRICT",
                  "sequence": 4,
                  "type": "openconfig-qos-types:ONE_RATE_TWO_COLOR"
                },
                "inputs": {
                  "input": [
                    {
                      "config": {
                        "id": "AF2",
                        "input-type": "QUEUE",
                        "queue": "AF2"
                      },
                      "id": "AF2",
                      "state": {
                        "id": "AF2",
                        "input-type": "QUEUE",
                        "queue": "AF2"
                      }
                    }
                  ]
                },
                "sequence": 4,
                "state": {
                  "priority": "STRICT",
                  "sequence": 4,
                  "type": "openconfig-qos-types:ONE_RATE_TWO_COLOR"
                }
              },
              {
                "config": {
                  "priority": "STRICT",
                  "sequence": 3,
                  "type": "openconfig-qos-types:ONE_RATE_TWO_COLOR"
                },
                "inputs": {
                  "input": [
                    {
                      "config": {
                        "id": "AF3",
                        "input-type": "QUEUE",
                        "queue": "AF3"
                      },
                      "id": "AF3",
                      "state": {
                        "id": "AF3",
                        "input-type": "QUEUE",
                        "queue": "AF3"
                      }
                    }
                  ]
                },
                "sequence": 3,
                "state": {
                  "priority": "STRICT",
                  "sequence": 3,
                  "type": "openconfig-qos-types:ONE_RATE_TWO_COLOR"
                }
              },
              {
                "config": {
                  "priority": "STRICT",
                  "sequence": 2,
                  "type": "openconfig-qos-types:ONE_RATE_TWO_COLOR"
                },
                "inputs": {
                  "input": [
                    {
                      "config": {
                        "id": "AF4",
                        "input-type": "QUEUE",
                        "queue": "AF4"
                      },
                      "id": "AF4",
                      "state": {
                        "id": "AF4",
                        "input-type": "QUEUE",
                        "queue": "AF4"
                      }
                    }
                  ]
                },
                "sequence": 2,
                "state": {
                  "priority": "STRICT",
                  "sequence": 2,
                  "type": "openconfig-qos-types:ONE_RATE_TWO_COLOR"
                }
              },
              {
                "config": {
                  "priority": "STRICT",
                  "sequence": 6,
                  "type": "openconfig-qos-types:ONE_RATE_TWO_COLOR"
                },
                "inputs": {
                  "input": [
                    {
                      "config": {
                        "id": "BE1",
                        "input-type": "QUEUE",
                        "queue": "BE1"
                      },
                      "id": "BE1",
                      "state": {
                        "id": "BE1",
                        "input-type": "QUEUE",
                        "queue": "BE1"
                      }
                    }
                  ]
                },
                "sequence": 6,
                "state": {
                  "priority": "STRICT",
                  "sequence": 6,
                  "type": "openconfig-qos-types:ONE_RATE_TWO_COLOR"
                }
              },
              {
                "config": {
                  "priority": "STRICT",
                  "sequence": 1,
                  "type": "openconfig-qos-types:ONE_RATE_TWO_COLOR"
                },
                "inputs": {
                  "input": [
                    {
                      "config": {
                        "id": "NC1",
                        "input-type": "QUEUE",
                        "queue": "NC1"
                      },
                      "id": "NC1",
                      "state": {
                        "id": "NC1",
                        "input-type": "QUEUE",
                        "queue": "NC1"
                      }
                    }
                  ]
                },
                "sequence": 1,
                "state": {
                  "priority": "STRICT",
                  "sequence": 1,
                  "type": "openconfig-qos-types:ONE_RATE_TWO_COLOR"
                }
              }
            ]
          },
          "state": {
            "name": "scheduler"
          }
        }
      ]
    }
  }
}
```

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
  ## Config paths
  ### Classifiers
  /qos/classifiers/classifier/config/name:
  /qos/classifiers/classifier/config/type:
  /qos/classifiers/classifier/terms/term/config/id:
  /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set:
  /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set:
  /qos/classifiers/classifier/terms/term/conditions/mpls/config/traffic-class:
  /qos/classifiers/classifier/terms/term/actions/config/target-group:

  ### Forwarding Groups
  /qos/forwarding-groups/forwarding-group/config/name:
  /qos/forwarding-groups/forwarding-group/config/output-queue:

  ### Queue
  /qos/queues/queue/config/name:

  ### Interfaces
  /qos/interfaces/interface/input/classifiers/classifier/config/type:
  /qos/interfaces/interface/input/classifiers/classifier/config/name:
  /qos/interfaces/interface/output/queues/queue/config/name:
  /qos/interfaces/interface/output/scheduler-policy/config/name:

  ### Scheduler policy
  /qos/scheduler-policies/scheduler-policy/config/name:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/priority:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/sequence:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/id:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/input-type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue:

  ## State paths
  /qos/interfaces/interface/output/queues/queue/state/name:
  /qos/interfaces/interface/output/queues/queue/state/transmit-pkts:
  /qos/interfaces/interface/output/queues/queue/state/transmit-octets:
  /qos/interfaces/interface/output/queues/queue/state/dropped-pkts:
  /qos/interfaces/interface/output/queues/queue/state/dropped-octets:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

* MFF - A modular form factor device containing LINECARDs, FABRIC and redundant CONTROLLER_CARD components
* FFF - fixed form factor
