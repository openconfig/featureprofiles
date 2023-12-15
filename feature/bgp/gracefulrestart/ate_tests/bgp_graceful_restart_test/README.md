# RT-1.4: BGP Graceful Restart

## Summary

This is to test the BGP graceful restart capability for BGP. A router that supports BGP graceful restart can work either as a Restarting speaker mode or in helper mode. By advertising BGP graceful restart capability, a router announces to the peer its ability to,
1.  [Restarting Speaker] Maintain forwarding state on all the routes in its FIB even when its BGP process is restarting. Therefore the peer functioning as a helper should continue to direct flows at the subject router undergoing BGP process restart.
2.  [Helper Router] Support a peer whose BGP process is restarting by continuing to direct flows at the peer.

While testing for the above, this test also confirms that the implementation respects stale-routes-time timer setting.


## Topology
Create the following connections:
```mermaid
graph LR; 
A[OTG:Port1] -- EBGP --> B[Port1:DUT:Port2];
B -- IBGP --> C[Port2:OTG];
```

## Procedure

RT-1.4.1: Enable and validate BGP Graceful restart feature
*   Configure EBGP peering between ATE:Port1 and DUT:Port1
*   Configure IBGP peering between ATE:Port2 and DUT:Port2
*   Ensure that the EBGP and IBGP peering are setup for IPv4-Unicast and IPv6-unicast AFI-SAFIs. Total 2xpeer-groups (1 per protocol) with 1 BGP session each.  
*   Enable `Graceful-Restart` capability at the `Peer-Group` level.
*   Ensure that the `restart-time` and the `stale-routes-time` are configured at the `Global` level
*   Configure allow route-policy under BGP peer-group address-family
*   Validate received capabilities at DUT and ATE reflect support for graceful
     restart.

...

RT-1.4.2: Restarting DUT speaker 
*   Advertise prefixes between the ATE ports, through the DUT. 
*   Trigger DUT session restart by killing the BGP process in the DUT. Please use the `gNOI.killProcessRequest_Signal_Term` as per [gNOI_proto](https://github.com/openconfig/gnoi/blob/main/system/system.proto#L326).
     *   Please kill the right process to restart BGP. For Juniper it is the `RPD` process. For Arista and Cisco this is the `BGP` process. For Nokia this is `sr_bgp_mgr`.
     *   Once the process is killied, verify that the packets are:
          *   Forrwarded between ATE port-1 and DUT port-1 for the duration of the specified stale routes time.
          *   Dropped after the stale routes timer has expired.
          *   Forwarded again between ATE port-1 and DUT port-1 after the session is re-established.


...

RT-1.4.3: DUT Helper for a restarting IBGP speaker
*   Advertise prefixes between the ATE ports through the DUT. Send Graceful restart trigger from ATE port-2.
*   Ensure that traffic can be forwarded between ATE port-1 and ATE port-2 during stale routes time.
*   Ensure that prefixes are withdrawn, and traffic cannot be forwarded between ATE port-1 and port-2 after the stale routes time expires.
 

...

RT-1.4.4: DUT Helper for a restarting EBGP speaker
*   Advertise prefixes between the ATE ports through the DUT. Send Graceful restart trigger from ATE port-1.
*   Ensure that traffic can be forwarded between ATE port-1 and ATE port-2 during stale routes time.
*   Ensure that prefixes are withdrawn, and traffic cannot be forwarded between ATE port-1 and ATE port-2 after the stale routes time expires.

## Config Parameter Coverage

For prefixes:

*   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor
*   gNOI.killProcessRequest_Signal_Term [To gracefully kill BGP process]

Parameters:

*   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/graceful-restart/config/enabled
*   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/graceful-restart/config/helper-only
*   /network-instances/network-instance/protocols/protocol/bgp/global/graceful-restart/config/restart-time
*   /network-instances/network-instance/protocols/protocol/bgp/global/graceful-restart/config/stale-routes-time

BGP conifguration:
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/peer-group/
  
* Policy-Definition
    * /routing-policy/policy-definitions/policy-definition/config/name
    * /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
    * /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set
    * /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options
    * /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result/ACCEPT_ROUTE
      
* Apply Policy at Peer-Group level
    * afi-safis/afi-safi/apply-policy/config/import-policy
    * afi-safis/afi-safi/apply-policy/config/export-policy

## Telemetry Parameter Coverage

*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/afi-safi-name
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/graceful-restart/state/advertised
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/peer-restart-time
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/graceful-restart/state/received
*   /network-instances/network-instance/protocols/protocol/bgp/global/graceful-restart/state/restart-time
*   /network-instances/network-instance/protocols/protocol/bgp/global/graceful-restart/state/stale-routes-time
