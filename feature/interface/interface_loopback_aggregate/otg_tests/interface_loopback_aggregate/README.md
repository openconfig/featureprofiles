# RT-5.6: Interface Loopback mode

## Summary

Ensure Interface mode can be set to loopback mode and can be added as part of static LAG.

## Procedure

### TestCase-1:

*   Configure DUT port-1 to OTG port-1.
*   Admin down OTG port-1.
*   Verify DUT port-1 is down.
*   On DUT port-1, set interface “loopback mode” to “TERMINAL”.
*   Add port-1 as part of Static LAG (lacp mode static(on)).
*   Validate that port-1 operational status is “UP”.
*   Validate on DUT that LAG interface status is “UP”.

## OpenConfig Path and RPC Coverage

The below YAML defines the OC paths intended to be covered by this test. OC paths used for test setup are not listed here.

```yaml
openconfig_paths:
  ## Config paths
    /interfaces/interface/config/loopback-mode:
    /interfaces/interface/ethernet/config/port-speed:
    /interfaces/interface/ethernet/config/duplex-mode:
    /interfaces/interface/ethernet/config/aggregate-id:
    /interfaces/interface/aggregation/config/lag-type:
    /interfaces/interface/aggregation/config/min-links:

  ## Telemetry paths
    /interfaces/interface/state/loopback-mode:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

#### Canonical OC
```json
{
  "interfaces": {
    "interface": [
      {
        "aggregation": {
          "config": {
            "lag-type": "LACP",
            "min-links": 1
          }
        },
        "config": {
          "name": "ae0"
        },
        "name": "ae0"
      },
      {
        "config": {
          "loopback-mode": "FACILITY",
          "name": "eth0"
        },
        "ethernet": {
          "config": {
            "aggregate-id": "ae0",
            "duplex-mode": "FULL",
            "port-speed": "SPEED_10GB"
          }
        },
        "name": "eth0"
      }
    ]
  }
}
```
