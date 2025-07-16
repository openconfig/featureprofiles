# DP-2.5: Police traffic on input matching all packets using 2 rate, 3 color marker

## Summary

Use IP address and mac-address from topology shared below. Static Routes can be used for this.
Configure an ingress scheduler to police traffic using a 2 rate, 3 color policer and attach the scheduler to the interface without a classifier.
Send traffic to validate the policer.

## Topology

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Test setup

```mermaid
graph LR;
ATE[ATE] <-- (Port 1) --> DUT[DUT] <-- (Port 2) --> ATE[ATE];
```

## Procedure

### Testbed setup - Generate configuration for ATE and DUT

#### Source & Destination Port for traffic

* ATE (Port1) --- IP Connectivity --- DUT (Dut1),  DUT (Dut2) --- IP Connectivity --- ATE (Port2)
* Use below to configure traffic with following source and destination.

  * Dut1 = Attributes {
		Desc:    "Dut1",
		MAC:     "02:01:00:00:00:01",
		IPv4:    "200.0.0.1/24",
		IPv6:    "2001:f:d:e::1/126",
	}
  * atePort1 = Attributes{
		Desc:    "atePort1",
		MAC:     "02:01:00:00:00:02",
		Vlan:    "100",
		IPv4:    "200.0.0.2/24",
		IPv6:    "2001:f:d:e::2/126",
	}
  * Dut2 = Attributes{
		Desc:    "Dut2",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "100.0.0.1/24",
		IPv6:    "2001:c:d:e::1/126",
	}
  * atePort2 = Attributes{
		Desc:    "atePort2",
		MAC:     "02:00:01:01:01:02",
		IPv4:    "100.0.0.2/24",
		IPv6:    "2001:c:d:e::2/126",
	}

* Create static route from atePort1 to atePort2.

### SetUp

* Generate config for scheduler polices with an input rate 2Gbps limit
* Apply them to DUT interface . Dut1 is LAG in provided setup.
* Use gnmi.Replace to push the config to the DUT.

### Canonical OC for DUT configuration

The configuration required for the 2R3C policer with classifier is included below:

```json
{
  "qos": {
    #
    # output queue (i.e. is QUEUE_3)
    #
    "queues": {
      "queue": [
        {
          "config": {
            "name": "QUEUE_1"
          },
          "name": "QUEUE_1"
        },
        {
          "config": {
            "name": "QUEUE_2"
          },
          "name": "QUEUE_2"
        },
        {
          "config": {
            "name": "QUEUE_3"
          },
          "name": "QUEUE_3"
        }
      ]
    },
    #
    # A single scheduler policy can be applied per interface.
    #
    "scheduler-policies": {
      "scheduler-policy": [
        {
          "config": {
            "name": "group_A_2Gb"
          },
          "name": "group_A_2Gb",
          "schedulers": {
            "scheduler": [
              {
                "config": {
                  "sequence": 1,
                  "type": "TWO_RATE_THREE_COLOR"
                },
                "inputs": {
                  "input": [
                    {
                      "config": {
                        "id": "my input policer 2Gb",
                        "input-type": "QUEUE",
                        "queue": "QUEUE_3"
                      },
                      "id": "my input policer 2Gb"
                    }
                  ]
                },
                "two-rate-three-color": {
                  "config": {
                    "cir": "1000000000",
                    "pir": "2000000000",
                    "bc": 100000,
                    "be": 100000,
                    "queuing-behavior": "POLICE"
                  },
                  "exceed-action": {
                    "config": {
                      "drop": false
                    }
                  },
                  "violate-action": {
                    "config": {
                      "drop": true
                    }
                  }
                }
              }
            ]
          }
        }
      ]
    },
    #
    # Interfaces input are mapped to the desired scheduler.
    "interfaces": {
      "interface": [
        {
          "interface-id": "Dut1.100",
          "config": {
            "interface-id": "Dut1.100"
          },
          "input": {
            "scheduler-policy": {
              "config": {
                "name": "group_A_2Gb"
              }
            }
          }
        }
      ]
    }
  }
}
```

### DP-2.5.1 Test traffic

* Send traffic
  * Send flow traffic from atePort1 to DUT towards atePort2 at 1.5Gbps (note cir is 1Gbps & pir is 2Gbps).
  * Validate qos counters on dut1 of DUT .
    * Validate DUT qos interface scheduler counters count packets as conforming-pkts, conforming-octets, exceeding-pkts & exceeding-octets.
  * Validate packets are received by atePort2.
    * Validate at OTG that 0 packets are lost on flow.
  * Increase traffic on flow to atePort2 to 4Gbps
    * Validate that flow to atePort2 experiences ~50% packet loss (+/- 1%)
    * Validate packet loss count as violating-pkts & violating-octets.


#### OpenConfig Path and RPC Coverage

```yaml
paths:
  # qos scheduler config
  /qos/scheduler-policies/scheduler-policy/config/name:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/two-rate-three-color/config/cir:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/two-rate-three-color/config/cir-pct:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/two-rate-three-color/config/pir:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/two-rate-three-color/config/pir-pct:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/two-rate-three-color/config/bc:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/two-rate-three-color/config/be:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/two-rate-three-color/conform-action/config/set-dscp:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/two-rate-three-color/conform-action/config/set-dot1p:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/two-rate-three-color/conform-action/config/set-mpls-tc:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/two-rate-three-color/exceed-action/config/set-dscp:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/two-rate-three-color/exceed-action/config/set-dot1p:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/two-rate-three-color/exceed-action/config/set-mpls-tc:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/two-rate-three-color/exceed-action/config/drop:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/two-rate-three-color/violate-action/config/set-dscp:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/two-rate-three-color/violate-action/config/set-dot1p:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/two-rate-three-color/violate-action/config/set-mpls-tc:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/two-rate-three-color/violate-action/config/drop:

  # qos interfaces config
  /qos/interfaces/interface/config/interface-id:
  /qos/interfaces/interface/input/scheduler-policy/config/name:

  # qos interface scheduler counters
  /qos/interfaces/interface/input/scheduler-policy/schedulers/scheduler/state/conforming-pkts:
  /qos/interfaces/interface/input/scheduler-policy/schedulers/scheduler/state/conforming-octets:
  /qos/interfaces/interface/input/scheduler-policy/schedulers/scheduler/state/exceeding-pkts:
  /qos/interfaces/interface/input/scheduler-policy/schedulers/scheduler/state/exceeding-octets:
  /qos/interfaces/interface/input/scheduler-policy/schedulers/scheduler/state/violating-pkts:
  /qos/interfaces/interface/input/scheduler-policy/schedulers/scheduler/state/violating-octets:

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
