# RT-1.4: BGP Graceful Restart

## Summary

BGP Graceful Restart

## Topology
Follwing connections:
 *   ATE port-1 <--> DUT port-1 
 *   ATE port-2 <--> DUT port-2

## Procedure
 *   Configure EBGP peering between ATE:Port1 and DUT:Port1
 *   Configure IBGP peering between ATE:Port2 and DUT:Port2
 *   Ensure that the EBGP and IBGP peering are setup for IPv4-Unicast and IPv6-unicast AFI-SAFIs
 *   Enable `Graceful-Restart` capability at the `Peer-Group` level.
 *   Ensure that the `restart-time` and the `stale-routes-time` are configured at the `Global` level
 *   Configure allow route-policy under BGP peer-group address-family
 *   Validate received capabilities at DUT and ATE reflect support for graceful
     restart.
 *   TestCase - Restarting DUT speaker 
   * Advertise prefixes between the ATE ports, through the DUT. 
   * Trigger DUT session restart by killing the BGP process in the DUT. Please use the `gNOI.killProcessRequest_Signal_Term` as per [gNOI_proto](https://github.com/openconfig/gnoi/blob/main/system/system.proto#L326).
         *   Please kill the right process to restart BGP. For Juniper it is the `RPD` process. For Arista this is the `BGP` process. For Nokia this is `sr_bgp_mgr`. 
 > TODO: Similar processes need to be included for Cisco.
       *   Once the process is killied, verify that the packets are:
             *   Forwarded between ATE port-1 and DUT port-1 for the duration of the specified stale routes time.
             *   Dropped after the stale routes timer has expired.
             *   Forwarded again between ATE port-1 and DUT port-1 after the session is re-established.
 *   TestCase -  DUT Helper for a restarting IBGP speaker
     * Advertise prefixes between the ATE ports through the DUT. Send Graceful restart trigger from ATE port-2.
       *   Ensure that traffic can be forwarded between ATE port-1 and ATE port-2 during stale routes time.
       *   Ensure that prefixes are withdrawn, and traffic cannot be forwarded between ATE port-1 and port-2 after the stale routes time expires.
*  TestCase - DUT Helper for a restarting EBGP speaker
    * Advertise prefixes between the ATE ports through the DUT. Send Graceful restart trigger from ATE port-1.
     *  Ensure that traffic can be forwarded between ATE port-1 and ATE port-2 during stale routes time.
     *  Ensure that prefixes are withdrawn, and traffic cannot be forwarded between ATE port-1 and port-2 after the stale routes time expires.

## Config Parameter Coverage

For prefixes:

*   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor

Parameters:

*   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/graceful-restart/config/enabled
*   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/graceful-restart/config/helper-only
*   /network-instances/network-instance/protocols/protocol/bgp/global/graceful-restart/config/restart-time
*   /network-instances/network-instance/protocols/protocol/bgp/global/graceful-restart/config/stale-routes-time

## Telemetry Parameter Coverage

*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/afi-safi-name
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/graceful-restart/state/advertised
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/peer-restart-time
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/graceful-restart/state/received
*   /network-instances/network-instance/protocols/protocol/bgp/global/graceful-restart/state/restart-time
*   /network-instances/network-instance/protocols/protocol/bgp/global/graceful-restart/state/stale-routes-time
