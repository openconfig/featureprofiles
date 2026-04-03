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
Generate an interface configuration and add that to the baseline configuration using union_replace.  The interface should include description, MTU, ip address and hold-timer.  
#### gnmi-3.2.1 - Add an interface configuration using OC to the baseline (CLI) configuration.  

1. Get the baseline configuration (A).
1. Generate a configuration with a new interface using OC (A.1).
1. Push configuration A + A.1 to the DUT.
1. Get configuration as A.2
1. Verify A.2 == A + A.1

#### gnmi-3.2.2 - add the interface configuration using CLI
Repeat steps in gnmi-3.2.1 but use CLI for the added interface configuration.

### gNMI-3.3 - union_replace change configuration
#### gnmi-3.3.1 Change the interface description using OC.
Steps
1. Get baseline configuration (B).
1. Change the description of an interface using OC. (B.1)
1. Push configuration baseline + B.1 to the DUT.
1. Get configuration as B.2
1. Verify B.2 == B + B.1

#### gnmi-3.3.2 - repeat gnmi-3.3.1 but change only the interface description using CLI.
### gNMI-3.4 - union_replace delete configuration through omission
#### gnmi-3.4.1 - Remove the interface ip address by omitting it in OC.

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

#### gnmi-3.4.2 - repeat gnmi 3.4.1 but instead remove the interface ip address by omitting it in CLI.

### gNMI-3.5 - union_replace move configuration
In some scenarios it is observed that moving a configuration from one interface to another can trigger bugs. Particularly if there is some conflicting element in the configuration such as an IP address.  This test moves a an IP address from interface 1 to interface 2 using union replace.
#### gnmi-3.5.1 move IP address between interfaces using OC
#### gnmi-3.5.2 move IP address between interfaces using CLI

### gNMI-3.6 - union_replace accepted with hardware mismatch
Interface configurations containing a mismatch with hardware (for example, due to a missing or incompatible transceiver module) must be accepted by a device.  The interface with the mismatched configuration is expected to be in a down operational state as the result of such a configuration commit.  

Steps
1. Get configuration D.1 from DUT
1. Generate a configuration D.2 with a port-speed mismatch in the OC which should be accepted by the device  
1. Push the configuration to the DUT
1. Verify the gnmi.Set is accepted
1. Get configuration D.3
1. Verify  D.2 == D.3.  That is, verify only the interface speed is changed between D.1 and D.3.  The remaining CLI and all OC must be unchanged.

#### gnmi3.6.1  verify configuration with OC hardware mismatch is accepted
Generate a configuration D.2 with a port-speed mismatch in the OC which should be accepted and applied by the DUT.

#### gnmi3.6.2  verify configuration with CLI hardware mismatch is accepted

Generate a configuration D.2 with a port-speed mismatch in the CLI which should be accepted and applied by the DUT.

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
