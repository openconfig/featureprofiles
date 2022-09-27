# P4RT-3.2: Google Discovery Protocol: PacketOut

## Summary

Verify that GDP packets can be sent by the controller.

## Procedure

*	Connect ATE port-1 to DUT port-1, and ATE port-2 to DUT port-2.
*	Enable the P4RT server on the device.
*	Connect two P4RT clients in a master/secondary configuration.
*	Configure the forwarding pipeline and install the P4RT table entry required for GDP.
*	Send a GDP packet from the master with egress_singleton_port set to one of the connected interfaces.
*	Verify that the GDP packet is received on the ATE port connected to the indicated interface.
*	Repeat sending the packet in the same way but from the secondary connection.
*	Verify that the packet is not received on the ATE.



## Config Parameter coverage

No new configuration covered.

## Telemetry Parameter coverage

No new telemetry covered.

## Minimum DUT platform requirement

vRX if the vendor implementation supports FIB-ACK simulation, otherwise FFF.