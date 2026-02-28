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

### gNMI-3.6 - union_replace accepted with warning

Interface configurations containing a mismatch with hardware (for example, due
to a missing or incompatible transceiver module) must be accepted by a device.
The interface with a configuration which doesn't match the hardware is expected
to be in a down operational state as the result of such a configuration commit.

Generic test steps

* Generate a configuration baseline_cfg + D.1 where D1. contains a port-speed in
  which does not match the DUT hardware.
* Push the configuration to the DUT
* Verify the gnmi.Set is accepted
* Get configuration D.2
* Verify  D.1 == D.2.  That is, verify only the interface speed is changed
between baseline_cfg and D.3.  The remaining CLI and all OC must be unchanged.

#### gnmi3.6.1  verify configuration with OC with warning is accepted

Perform the steps where the with a port-speed mismatch is in OC.

#### gnmi3.6.2  verify configuration with CLI with warning is accepted

Perform the steps where the with a port-speed mismatch is in CLI.

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
