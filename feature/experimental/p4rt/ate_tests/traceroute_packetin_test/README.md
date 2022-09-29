# P4RT-5.1: Traceroute: PacketIn


## Summary

Verify that Traceroute packets are punted with correct metadata.


## Procedure

*	Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2.
*	TODO: Install the set of routes on the device.
*	Enable the P4RT server on the device.
*	Connect a P4RT client and configure the forwarding pipeline. InstallP4RT table 	entries required for traceroute.
*	Send IPv4 packets from the ATE with TTL={0,1/2} and verify that packets with TTL=0 and at least one of TTL={1/2} are received by the client.
*	Send IPv6 packets from the ATE with HopLimit={0,1/2} and verify that packets with HopLimit=0 and at least one of HopLimit={1/2} are received by the client.
*	Verify that the packets have both ingress_singleton_port and egress_singleton_port metadata set.


## Config Parameter coverage

*    /components/component/integrated-circuit/config/node-id
*    /interfaces/interface/config/id


## Telemetry Parameter coverage

No new telemetry covered.


## Protocol/RPC Parameter coverage

No new Protocol/RPC covered.

