# P4RT-7.2: LLDP: PacketOut

## Summary

Verify that LLDP packets can be sent by the controller.

## Procedure

*	Connect ATE port-1 to DUT port-1, and ATE port-2 to DUT port-2.
*	Disable on-box processing of LLDP.
*	Enable the P4RT server on the device.
*	Connect a P4RT client and configure the forwarding pipeline. Install the P4RT table entry required for LLDP.
*	Send an LLDP packet from the client with egress_singleton_port set to one of the connected interfaces.
*	Verify that the LLDP packet is received on the ATE port connected to the indicated interface.




## Config Parameter coverage

No new configuration covered.

## Telemetry Parameter coverage

No new telemetry covered.

## Minimum DUT platform requirement

vRX if the vendor implementation supports FIB-ACK simulation, otherwise FFF.