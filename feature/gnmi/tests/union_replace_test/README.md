# gNMI-3: union_replace 

## Summary

Perform a series of tests that validate the union_replace specification.  

In depth tests for use of union_replace should reside within the features being
tested.

## Testbed type

* Single DUT testbed

## Procedure

### Test environment setup

* Get the DUT configuriation in CLI format and store as `baseline_cfg`.

### gNMI-3.6 - union_replace accepted with warning

Interface configurations containing a mismatch with hardware (for example, due
to a missing or incompatible transceiver module) must be accepted by a device.
The interface with a configuration which doesn't match the hardware is expected
to be in a down operational state as the result of such a configuration commit.

Steps

* Generate a configuration baseline_cfg + D.1 where D1. contains a port-speed in
  OC which does not match the DUT hardware.
* Push the configuration to the DUT Verify the gnmi.Set is accepted 
* Get configuration D.2
* Verify  D.1 == D.2.  That is, verify only the interface speed is changed
between baseline_cfg and D.3.  The remaining CLI and all OC must be unchanged.

gnmi3.6.1  verify configuration with OC with warning is accepted Generate a
configuration D.2 with a port-speed mismatch in the OC which should be accepted
and applied by the DUT.

gnmi3.6.2  verify configuration with CLI with warning is accepted

Generate a configuration D.2 with a port-speed mismatch in the CLI which should
be accepted and applied by the DUT.

## Canonical OC

```json
{
  "components": {
    "component": [
      {
        "config": {
          "name": "Port0/1"
        },
        "name": "eth0",
        "port": {
          "breakout-mode": {
            "groups": {
              "group": [
                {
                  "config": {
                    "breakout-speed": "SPEED_100GB",
                    "index": 1,
                    "num-breakouts": 4,
                    "num-physical-channels": 1
                  },
                  "index": 1
                }
              ]
            }
          }
        }
      }
    ]
  },
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "Customer A",
          "mtu": 1500,
          "name": "eth0/0"
        },
        "ethernet": {
          "config": {
            "port-speed": "SPEED_100GB"
          }
        },
        "name": "eth0/0"
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
    gNMI.Get:
    gNMI.Subscribe:
```
