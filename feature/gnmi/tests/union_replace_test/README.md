# gNMI-3: union_replace

## Summary

Perform a series of tests that validate the union_replace specification.  

In depth tests for use of union_replace should reside within the features being
tested.

## Testbed type

* [Single DUT testbed](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut.testbed)

## Procedure

### Test environment setup

* Get the DUT configuriation in CLI format and store as `baseline_cfg`.

* gNMI-3 should be a single _test.go file which contains a series of test functions which may be run in any order or individually.  

### gNMI-3.1 - Idempotent configuration
Verify the same configuration as already on the device can be pushed and accepted without changing the configuration.

Steps
Get baseline configuration
Push configuration baseline + A to the DUT
Get configuration A.1
Verify A.1 == baseline + A
Push configuration baseline + A to the DUT
Get configuration A.1
Verify A.1 == baseline + A

### gNMI-3.2 - union_replace add configuration
Generate an interface configuration and add that to the baseline configuration using union_replace. The interface should include description, MTU, and IP address.
#### gNMI-3.2.1 - Add an interface configuration using OC to the baseline (CLI) configuration.

1. Get the baseline configuration (A).
1. Generate a configuration with a new interface using OC (A.1).
1. Push configuration A + A.1 to the DUT.
1. Get configuration as A.2
1. Verify A.2 == A + A.1

#### gNMI-3.2.2 - Add the interface configuration using CLI
Repeat steps in gnmi-3.2.1 but use CLI for the added interface configuration.

### gNMI-3.3 - union_replace change configuration
#### gNMI-3.3.1 - Change the interface description using OC.
Steps
1. Get baseline configuration (B).
1. Change the description of an interface using OC. (B.1)
1. Push configuration baseline + B.1 to the DUT.
1. Get configuration as B.2
1. Verify B.2 == B + B.1

#### gNMI-3.3.2 - Repeat gNMI-3.3.1 but change only the interface description using CLI.
### gNMI-3.4 - union_replace delete configuration through omission
#### gNMI-3.4.1 - Remove an interface by omitting it in OC.

Steps
1. Get baseline configuration (B).
1. Generate OC configuration adding interfaces 1 and 2 (B.1).
1. Push configuration baseline + B.1 to the DUT.
1. Get configuration as B.2
1. Verify B.2 == B + B.1
1. Generate OC configuration adding only interface 1 (B.3).
1. Push configuration baseline + B.3 to the DUT.
1. Get configuration as B.4
1. Verify B.4 == B + B.3

#### gNMI-3.4.2 - Repeat gNMI-3.4.1 but remove the interface by omitting it in CLI.

### gNMI-3.5 - union_replace move configuration
In some scenarios it is observed that moving a configuration from one interface to another can trigger bugs. Particularly if there is some conflicting element in the configuration such as an IP address. This test moves an IP address from interface 1 to interface 2 using union replace.
#### gNMI-3.5.1 - Move IP address between interfaces using OC
#### gNMI-3.5.2 - Move IP address between interfaces using CLI

### gNMI-3.6 - union_replace accepted with hardware mismatch
Interface configurations containing a mismatch with hardware (for example, due to a missing or incompatible transceiver module) must be accepted by a device.  The interface with the mismatched configuration is expected to be in a down operational state as the result of such a configuration commit.  

Steps
1. Get configuration D.1 from DUT
1. Generate a configuration D.2 with a port-speed mismatch in the OC which should be accepted by the device  
1. Push the configuration to the DUT
1. Verify the gnmi.Set is accepted
1. Get configuration D.3
1. Verify  D.2 == D.3.  That is, verify only the interface speed is changed between D.1 and D.3.  The remaining CLI and all OC must be unchanged.

#### gNMI-3.6.1 - Verify configuration with OC hardware mismatch is accepted
Generate a configuration D.2 with a port-speed mismatch in the OC which should be accepted and applied by the DUT.

#### gNMI-3.6.2 - Verify configuration with CLI hardware mismatch is accepted

Generate a configuration D.2 with a port-speed mismatch in the CLI which should be accepted and applied by the DUT.

### gNMI-3.7 - union_replace rejected with error in CLI with OC
Verify a DUT rejects and rolls back a gnmi.Set union_replace with an invalid configuration in origin CLI.  Verify the original configuration is preserved.  

TODO: Decide what configuration error(s) to use.  I think we need cases where there is OC that fails leafref validation, but even more importantly, a scenario where the OC will validate, but contains a semantic error.  

Simple issues like a value out of range or referencing a policy that doesn’t exist in the OC case will be caught with a validation of the structs.  Such an error  is likely a different code path in a DUT vs. processing a configuration that validates but has some semantic error.  

For example of a config that fails validation: referencing a BGP policy that doesn’t exist is an example of data that won’t validate, since BGP policy references in OC are defined as leafrefs.  We can write that test and send an unvalidated config to a DUT, but it seems unlikely to reveal bugs. 

For the case where the config passes validation but contains a semantic error, a test case could be configuring an interface to use a QoS queue that doesn’t exist.  In this case, the queue name is a string, not a leafref.  See qos/interfaces/interface/input/queues/queue/config/name.    (This leaf is a string and not a leafref because a device may expose queues which are not explicitly configured)
Steps

1. Get configuration E.1 from DUT.
1. Generate a configuration E.2  which includes invalid configuration (see sub tests).
1. Push the configuration E.1 + E.2 to the DUT.
1. Confirm the DUT rejects the gnmi.Set.
1. Get configuration E.3 from DUT.
1. Verify E.1 == E.3  (the configuration is unchanged).

#### gnmi-3.7.1 reference validation error in OC
The invalid configuration is OC which references a BGP neighbor import policy that does not exist.

#### gnmi-3.7.2 reference which validates but is an error in OC
The invalid configuration is OC which references an qos queue on an interface which does not exist.

#### gnmi-3.7.3 reference error in CLI
The invalid configuration is CLI which references a BGP neighbor import policy that does not exist.

### gNMI-3.8 - union_replace rejected with error due to configuration item overlap
This test verifies union_replace option 1 or 2 behavior for resolving overlapping configuration items between OC and CLI.  Generate the following configuration item combinations which have overlaps between CLI and OC.  For NOS which implement option 1, the DUT should return a gRPC error of `INVALID_ARGUMENT`.  For NOS with implement option 2, the configuration should be accepted, with the CLI value taking effect and the OC configuration leaf being accepted, but not applied as “state”.  

Steps
1. Get configuration E.1 from DUT.
1. Generate a configuration E.2  which includes the overlapping configuration.
1. Push the configuration E.1 + E.2 to the DUT.
1. Confirm the DUT rejects the gnmi.Set with a gRPC error code of INVALID_ARGUMENT. Log the contents of the optional gRPC error string. 
1. Get configuration E.3 from DUT.
1. Verify the configuration is as expected
1. For option 1 NOS, verify E.1 == E.3  (the configuration is unchanged).
1. For option 2 NOS, verify E.2 == E.3 (the CLI config is updated, the OC state is updated to match the CLI value, the OC config is updated using the OC value.  Note that the OC state leaves do not equal the OC config leaves)


#### gnmi-3.8.1 interface CLI and OC overlap with different values
Test where the configuration overlap is the interface MTU with two different MTU values.

#### gnmi-3.8.2 interface CLI and OC overlap with same value
Test where the configuration overlap is the interface MTU with the same MTU values.

#### gnmi-3.8.3 BGP model overlap
Test where the configuration overlap is /network-instances/network-instance/protocols/protocol/bgp/global/config/as

####gnmi-3.8.4 routing-policy model overlap
Test where the overlap is /routing-policy/policy-definitions/policy-definition/config/name.

###gNMI-3.9 CLI and OC non-overlap in same OC configuration tree
These configurations should be accepted and applied successfully by the DUT.

#### gnmi-3.9.1 interface and MTU in OC and interface description in CLI
Steps
1. Get configuration D.1 from DUT
Generate a configuration D.2 with one interface description set in CLI and a second set using  OC.
1. Push the configuration to the DUT
1. Verify the gnmi.Set is accepted
1. Get configuration D.3
1. Verify OC configuration for interface one and two match the descriptions provided by the CLI and OC respectively.   

### gNMI-3.10 - union_replace accepted with missing hardware
Configure an interface with a missing transceiver module.  The interface with the missing transceiver is expected to contain “config” leaves with the desired values. The “state” leaves should show a down operational state as the result of the configuration commit.  

Steps
1. Identify an interface without a transceiver module installed.  
1. Get configuration D.1 from DUT
1. Generate a configuration D.2 , including a port-speed and breakout mode for the interface without a transceiver.
1. Push the configuration to the DUT
1. Verify the gnmi.Set is accepted
1. Get configuration D.3
1. Verify  D.2 == D.3 configuration.  That is, verify the “config” leaves for breakout mode and port speed are set to the target values.  Verify the state for the interface is oper-state DOWN.    All other CLI and OC config leaves must be unchanged.

#### gnmi3.6.1  verify configuration with OC hardware missing is accepted
Perform the steps where a configuration D.2 where the port-speed and breakout set using OC.

####gnmi3.6.2  verify configuration with CLI hardware missing is accepted
Perform the steps where a configuration D.2 where the port-speed and breakout set using CLI.


## Canonical OC

```json
{
  "components": {
    "component": [
      {
        "config": {
          "name": "Port0"
        },
        "name": "Port0",
        "port": {
          "breakout-mode": {
            "groups": {
              "group": [
                {
                  "config": {
                    "breakout-speed": "SPEED_50GB",
                    "index": 1,
                    "num-breakouts": 2,
                    "num-physical-channels": 2
                  },
                  "index": 1
                }
              ]
            }
          }
        }
      },
      {
        "config": {
          "name": "Port0-Transceiver"
        },
        "name": "Port0-Transceiver",
        "state": {
          "parent": "Port0",
          "type": "TRANSCEIVER"
        },
        "transceiver": {
          "state": {
            "form-factor": "QSFP28"
          }
        }
      }
    ]
  },
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "First 50G breakout",
          "name": "eth0/0"
        },
        "ethernet": {
          "config": {
            "port-speed": "SPEED_50GB"
          }
        },
        "name": "eth0/0"
      },
      {
        "config": {
          "description": "Second 50G breakout with wrong port-speed",
          "name": "eth0/1"
        },
        "ethernet": {
          "config": {
            "port-speed": "SPEED_100GB"
          }
        },
        "name": "eth0/1"
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /interfaces/interface/ethernet/config/port-speed:
  /interfaces/interface/ethernet/state/port-speed:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Get:
    gNMI.Subscribe:
```

## Required DUT platform

vRX
