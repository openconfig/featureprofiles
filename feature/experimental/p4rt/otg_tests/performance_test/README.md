# P4RT-6.1: Required Packet I/O rate: Performance


## Summary

Verify that both Packetin and Packetout traffic is handled by the P4RT server at the required rate.


## Procedure

*	Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2.
*	Enable the P4RT server on the device.
*	Connect a P4RT client and configure the forwarding pipeline. InstallP4RT table 	entries required for traceroute, GDP and LLDP.
*	Setup packetin GDP, LLDP and traceroute traffic from the ATE at the rate 200kbps, 100kbps and 324 pps respectively. 
*	Setup packetout packets for GDP, LLDP and traceroute from the P4RT client.
*   Start both packetin and packetout traffic at the same rate simultaneously. 
*	Verify no packetloss for both directions of traffic.
*   Verify the metadata ID and the value for all three traffic types on the P4RT client for packetin. 


## Config Parameter coverage

*    /components/component/integrated-circuit/config/node-id
*    /interfaces/interface/config/id


## Telemetry Parameter coverage

No new telemetry covered.


## Protocol/RPC Parameter coverage

No new Protocol/RPC covered.

