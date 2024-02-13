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


## Config Parameter coverage

No new configuration covered.

## Telemetry Parameter coverage

No new telemetry covered.

## Minimum DUT platform requirement

vRX if the vendor implementation supports FIB-ACK simulation, otherwise FFF.