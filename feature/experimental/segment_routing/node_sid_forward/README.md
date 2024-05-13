# SR-1.1: Transit forwarding to Node-SID via ISIS

## Summary

MPLS-SR transit forwarding to Node-SID distributed over ISIS

## Procedure

Topology: ATE1—DUT1–ATE2
                               
*   Configure Segment Routing Global Block (start: 400000 range: 65001)
*   Enable Segment Routing for the ISIS

Advertise 2 prefixes with SIDs to DUT from ATE2:

*  Prefix (1) with node-SID is advertised by the direct ISIS neighbor
*  Prefix (2) with node-SID is advertised by simulated indirect ISIS speaker

Verify: 
*  DUT receives prefixes with SIDs and has a correct emulated ISIS topology (database).
*  DUT advertises both node-SID to ATE1.


Generate traffic:
*   Send labeled traffic transiting through the DUT matching direct prefix (1). Verify that ATE2 receives traffic with node-SID label popped.
*   Send labeled traffic transiting through the DUT matching indirect prefix (2). Verify that ATE2 receives traffic with the node-SID label intact.
*   Verify that corresponding SID forwarding counters are incremented.

## Config Parameter Coverage

* `/network-instances/network-instance/protocols/protocol/isis/global/segment-routing/config`
* `/network-instances/network-instance/segment-routing/srlbs/srlb/config`

## Telemetry Parameter Coverage

* `/network-instances/network-instance/protocols/protocol/isis/global/segment-routing/state/enabled`
* `/network-instances/network-instance/protocols/protocol/isis/global/segment-routing/state/srgb`
* `/network-instances/network-instance/mpls/signaling-protocols/segment-routing/aggregate-sid-counters/state`