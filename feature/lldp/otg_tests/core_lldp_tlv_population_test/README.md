# RT-6.1: Core LLDP TLV Population

## Summary

Determine LLDP advertisement and reception operates correctly.

## Procedure

*   Connect ATE port-1 to DUT port-1, enable LLDP on each port.
*   Configure LLDP enabled=true for DUT port-1 and ATE port-1
*   Determine that telemetry from:
    *   ATE reports correct values from DUT.
    *   DUT reports correct values from ATE.
*   Configure LLDP is disabled at global level for DUT.
    *   Set /lldp/config/enabled to FALSE.
    *   Ensure that DUT does not generate any LLDP messages irrespective of the
        configuration of lldp/interfaces/interface/config/enabled (TRUE or
        FALSE) on any interface.

## Canonical OC

```json
{
  "openconfig-lldp:lldp": {
    "config": {
      "enabled": true,
      "system-description": "DUT"
    },
    "interfaces": {
      "interface": [
        {
          "name": "port1",
          "config": {
            "enabled": true,
            "name": "port1"
          }
        }
      ]
    }
  },
  "openconfig-interfaces:interfaces": {
    "interface": [
      {
        "name": "port1",
        "config": {
          "enabled": true,
          "name": "port1",
          "type": "iana-if-type:ethernetCsmacd"
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.
OC paths used for test setup are not listed here.

```yaml
paths:
  ## Config Paths ##
  /lldp/config/enabled:
  /lldp/config/system-description:
  /lldp/interfaces/interface/config/enabled:
  /lldp/interfaces/interface/config/name:

  ## State Paths ##
  /lldp/interfaces/interface/neighbors/neighbor/state/chassis-id:
  /lldp/interfaces/interface/neighbors/neighbor/state/chassis-id-type:
  /lldp/interfaces/interface/neighbors/neighbor/state/port-id:
  /lldp/interfaces/interface/neighbors/neighbor/state/port-id-type:
  /lldp/interfaces/interface/neighbors/neighbor/state/system-description:
  /lldp/interfaces/interface/neighbors/neighbor/state/system-name:
  /lldp/interfaces/interface/state/name:
  /lldp/state/chassis-id:
  /lldp/state/chassis-id-type:
  /lldp/state/system-description:
  /lldp/state/system-name:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:

```

## Minimum DUT platform requirement

vRX
