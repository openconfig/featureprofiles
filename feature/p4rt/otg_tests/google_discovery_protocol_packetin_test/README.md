# P4RT-3.1: Google Discovery Protocol: PacketIn

## Summary

Verify that GDP packets are punted with correct metadata.

## Procedure

*	Connect ATE port-1 to DUT port-1, and ATE port-2 to DUT port-2.
*	Enable the P4RT server on the device.
*	Connect two P4RT clients in a master/secondary configuration.
*	Configure the forwarding pipeline and install the P4RT table entry required for GDP.
*	Send a GDP packet from the ATE and verify that it is received on the master and not the secondary client.
*	Verify that the packet has the ingress_singleton_port metadata set and it corresponds to the interface ID of the port that the packet was received on.


## OpenConfig Path and RPC Coverage
```yaml
paths:
  /components/component/integrated-circuit/config/node-id:
    platform_type: [INTEGRATED_CIRCUIT]
  /interfaces/interface/aggregation/switched-vlan/config/native-vlan:
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/type:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/vlan/match/single-tagged/config/vlan-id:
  /lldp/config/enabled:
  /system/mac-address/config/routing-mac:
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

vRX if the vendor implementation supports FIB-ACK simulation, otherwise FFF.