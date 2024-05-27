# RT-1.55: BGP session mode (active/passive)

## Summary

BGP session mode (active/passive)

## Topology

DUT Port1 (AS 65501) ---eBGP --- ATE Port1 (AS 65502)

## Procedure

*  Configure both DUT and ATE to operate in BGP passive mode under the neighbor section.
*  Verify that the BGP adjacency will not be established.
*  Verify the telemetry path output to confirm that the neighbor's BGP transport mode is displayed as "passive for the DUT.
*  Configure BGP session on ATE to operate in BGP active mode when interacting with DUT.
*  Verify that a BGP adjacency is established between the ATE and DUT
*  Verify the telemetry path output to confirm that the neighbor's BGP transport mode is displayed as "passive for the DUT.
*  Redo the same above steps but configure the passive mode under the peer group instead of the  bgp neighbor configuration.

## Config Parameter coverage

*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/config/passive-mode
*   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/transport/config/passive-mode

## Telemetry Parameter coverage

*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/state/passive-mode
*   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/transport/state/passive-mode

## Protocol/RPC Parameter coverage

*   BGP
