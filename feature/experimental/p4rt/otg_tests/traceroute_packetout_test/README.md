# P4RT-5.2: Traceroute Packetout


## Summary

Verify that traceroute packets can be sent by the controller.


## Procedure

*   Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2, and ATE port-3 to
    DUT port-3.

*   Install a set of routes on the device in both the default and TE VRFs.

*   Enable the P4RT server on the device..

*   Connect a P4RT client and configure the forwarding pipeline. Install
P4RT table entries required for traceroute.

*   Send an IPv4 traceroute reply from the client with submit_to_egress_pipeline metadata  set to true.

*   Verify that the packet is received on the ATE on the portcorresponding to the routing table in the default VRF.

*   Repeat with an IPv6 traceroute reply and verify that it is receivedcorrectly by the ATE.


*   Validate:

    *   Traffic can continue to be forwarded between ATE port-1 and port-2.

    *   Through AFT telemetry that the route entries remain present.

    *   Following daemon restart, the gRIBI client connection can be re-established.

    *   Issuing a gRIBI Get RPC results in 203.0.113.0/24 being returned.


## Protocol/RPC Parameter Coverage

*  No new configuration covered.


## Telemetry Parameter Coverage

*  No new telemetry covered.
