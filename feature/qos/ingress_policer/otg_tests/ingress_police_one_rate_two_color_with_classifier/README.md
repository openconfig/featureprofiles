# DP-2.7: Police traffic on input matching specific packets using 1 rate, 2 color marker with policy-forwarding

## Summary

Use IP address and mac-address from topology shared below. Static Routes can be used for this.
Configure a policer-group to police traffic using a 1 rate, 2 color policer and apply it using a policy-forwarding rule.
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

* Generate config for policer-groups with an input rate 2Gbps limit and a policy-forwarding rule.
* Apply them to DUT interface . Dut1 is LAG in provided setup.
* Use gnmi.Replace to push the config to the DUT.

### Canonical OC for DUT configuration

TODO: The following OC relies on the pending `go/oc-policer-group` schema (introducing shared policer actions and QoS buckets) which is not yet merged to the OpenConfig data models.

The configuration required for the 1R2C policer with policy-forwarding is included below:

```json
{
  "openconfig-qos:qos": {
    "policer-groups": {
      "policer-group": [
        {
          "name": "group-policer-B",
          "config": {
            "name": "group-policer-B"
          },
          "one-rate-two-color": {
            "config": {
              "cir": 2000000000,
              "bc": 268435456
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
  },
  "openconfig-network-instance:network-instances": {
    "network-instance": [
      {
        "name": "DEFAULT",
        "policy-forwarding": {
          "policies": {
            "policy": [
              {
                "policy-id": "pbr_cloud_id_2",
                "config": {
                  "policy-id": "pbr_cloud_id_2",
                  "type": "MATCH_ACTION_POLICY"
                },
                "rules": {
                  "rule": [
                    {
                      "sequence-id": 10,
                      "config": {
                        "sequence-id": 10,
                        "address-family": "openconfig-types:IPV4"
                      },
                      "ipv4": {
                        "config": {
                          "destination-address-prefix-set": "field-set-B"
                        }
                      },
                      "action": {
                        "config": {
                          "policer-group": "group-policer-B"
                        }
                      }
                    }
                  ]
                }
              }
            ]
          }
        }
      }
    ]
  }
}
```

### DP-2.7.1 Test traffic

* Send traffic
  * Send flow traffic from atePort1 to DUT towards atePort2 at 1.5Gbps (note cir is 2Gbps).
  * Validate qos counters on dut1 of DUT .
    * Validate DUT qos policer-group counters count packets as conforming-pkts, conforming-octets, exceeding-pkts & exceeding-octets.
  * Validate packets are received by atePort2.
    * Validate at OTG that 0 packets are lost on flow.
  * Increase traffic on flow to atePort2 to 4Gbps
    * Validate that flow to atePort2 experiences ~50% packet loss (+/- 1%)
    * Validate packet loss count as exceeding-pkts & exceeding-octets.


#### OpenConfig Path and RPC Coverage

```yaml
paths:
  # qos policer-group config
  /qos/policer-groups/policer-group/config/name:
  /qos/policer-groups/policer-group/one-rate-two-color/config/cir:
  /qos/policer-groups/policer-group/one-rate-two-color/config/bc:
  /qos/policer-groups/policer-group/one-rate-two-color/exceed-action/config/drop:

  # qos policer-group state counters
  /qos/policer-groups/policer-group/state/conforming-pkts:
  /qos/policer-groups/policer-group/state/conforming-octets:
  /qos/policer-groups/policer-group/state/exceeding-pkts:
  /qos/policer-groups/policer-group/state/exceeding-octets:

  # policy-forwarding match & action config
  /network-instances/network-instance/policy-forwarding/policies/policy/config/type:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/destination-address-prefix-set:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/policer-group:

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
