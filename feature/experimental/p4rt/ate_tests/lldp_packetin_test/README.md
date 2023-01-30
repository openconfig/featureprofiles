# P4RT-7.1: LLDP: PacketIn

## Summary

Verify that LLDP packets are punted with correct metadata.

## Procedure

*	Connect ATE port-1 to DUT port-1, and ATE port-2 to DUT port-2.
*	Disable on-box processing of LLDP.
*	Enable the P4RT server on the device.
*	Connect a P4RT client and configure the forwarding pipeline. Install the P4RT table entry required for LLDP.
*	Send an LLDP packet from the ATE and verify that it is received by the client.
*	Verify that the packet has the ingress_singleton_port metadata set and it corresponds to the interface ID of the port that the packet was received on.



## Config Parameter coverage

No new configuration covered.

## Telemetry Parameter coverage

No new telemetry covered.

## Minimum DUT platform requirement

vRX if the vendor implementation supports FIB-ACK simulation, otherwise FFF.