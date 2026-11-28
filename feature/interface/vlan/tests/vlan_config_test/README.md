# RT-5.20: VLAN and native VLAN configuration

## Summary

Create and delete a VLAN, then place one Ethernet port in access mode and in
trunk mode with a native VLAN. This is the config round-trip behind the
SpyTest VLAN cases, plus access mode, which those cases do not set.

## Testbed type

*   [`featureprofiles/topologies/dut.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut.testbed)

## Procedure

*   Create VLAN 4090 in the default network instance. Read `vlan-id` from
    config and state. Delete the VLAN and check it is gone.
*   Create the VLAN again. Replace the first Ethernet port's switched VLAN
    with access mode and access VLAN 4090. Read both leaves from config and
    the access VLAN from state.
*   Replace that switched VLAN with trunk mode and native VLAN 4090. Read
    both leaves from config and the native VLAN from state.
*   Delete the switched VLAN and the VLAN.

#### Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "name": "Ethernet0",
        "ethernet": {
          "switched-vlan": {
            "config": {
              "interface-mode": "TRUNK",
              "native-vlan": 4090
            }
          }
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /network-instances/network-instance/vlans/vlan/config/vlan-id:
  /network-instances/network-instance/vlans/vlan/state/vlan-id:
  /interfaces/interface/ethernet/switched-vlan/config/interface-mode:
  /interfaces/interface/ethernet/switched-vlan/config/access-vlan:
  /interfaces/interface/ethernet/switched-vlan/config/native-vlan:
  /interfaces/interface/ethernet/switched-vlan/state/access-vlan:
  /interfaces/interface/ethernet/switched-vlan/state/native-vlan:
rpcs:
  gnmi:
    gNMI.Set:
      replace: true
      delete: true
    gNMI.Subscribe:
```
