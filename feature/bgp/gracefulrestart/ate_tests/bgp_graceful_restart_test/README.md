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
A[ATE:Port1] -- EBGP --> B[Port1:DUT:Port2];
B -- IBGP --> C[Port2:ATE];
```

## Procedure

**RT-1.4.1: Enable and validate BGP Graceful restart feature**
*   Configure EBGP peering between ATE:Port1 and DUT:Port1
*   Configure IBGP peering between ATE:Port2 and DUT:Port2
*   Ensure that the EBGP and IBGP peering are setup for IPv4-Unicast and IPv6-unicast AFI-SAFIs. Total 2xpeer-groups (1 per protocol) with 1 BGP session each.  
*   Enable `Graceful-Restart` capability at the `Peer-Group` level.
*   Ensure that the `restart-time` and the `stale-routes-time` are configured at the `Global` level
*   Configure allow route-policy under BGP peer-group address-family
*   Validate received capabilities at DUT and ATE reflect support for graceful
     restart.
    

**RT-1.4.2: Restarting DUT speaker whose BGP process was killed gracefully**
*   Advertise prefixes between the ATE ports, through the DUT. 
*   Trigger DUT session restart by killing the BGP process in the DUT. Please use the `gNOI.killProcessRequest_Signal_Term` as per [gNOI_proto](https://github.com/openconfig/gnoi/blob/main/system/system.proto#L326).
     *   Please kill the right process to restart BGP. For Juniper it is the `RPD` process. For Arista and Cisco this is the `BGP` process. For Nokia this is `sr_bgp_mgr`.
     *   Once the process is killied, verify that the packets are:
          *   Forwarded between ATE port-1 and DUT port-1 for the duration of the specified stale routes time.
          *   Dropped after the stale routes timer has expired.
          *   Forwarded again between ATE port-1 and DUT port-1 after the session is re-established.
     *   Conduct the above steps once for the EBGP peering and once for the IBGP peering


**RT-1.4.3: Restarting DUT speaker whose BGP process was killed abruptly**
*   Follow the same steps as in RT-1.4.2 above but use `gNOI.killProcessRequest_Signal_KILL` this time as per `gNOI proto`
*   Pass/Fail criteria in this case too is the same as that for RT-1.4.2. Router that supports Graceful restart is expected to allow traffic flow w/o any packet drops until the `stale-routes-time` timer expires.


**RT-1.4.4: DUT Helper for a restarting EBGP speaker whose BGP process was gracefully killed**
*   Advertise prefixes between the ATE ports through the DUT. Send Graceful restart trigger from ATE port-1.
*   Ensure that traffic can be forwarded between ATE port-1 and ATE port-2 during stale routes time.
*   Ensure that prefixes are withdrawn, and traffic cannot be forwarded between ATE port-1 and port-2 after the stale routes time expires.
*   Repeat the above for the IBGP peering on ATE port2
 
**RT-1.4.5: DUT Helper for a restarting EBGP speaker whose BGP process was killed abruptly**
*   Advertise prefixes between the ATE ports through the DUT. Use `gNOI.killProcessRequest_Signal_KILL` as per `gNOI proto` to ATE:Port1.
*   Ensure that traffic can be forwarded between ATE port-1 and ATE port-2 during stale routes time.
*   Ensure that prefixes are withdrawn, and traffic cannot be forwarded between ATE port-1 and ATE port-2 after the stale routes time expires.

**RT-1.4.6: Test support for RFC8538 compliance by letting Hold-time expire**

RFC-8538 builds on RFC4724 by adding Graceful restart support for scenarios when the BGP holdtime expires. In order to simulate holdtime expiry, please install an ACL that drops BGP packets from the Peer. Ensure that the packets are,
*   Forwarded between TE port-1 and DUT port-1 for the duration of the specified stale routes time.
*   Dropped after the stale routes timer has expired.
*   Forwarded again between ATE port-1 and DUT port-1 after the session is re-established.
*   Repeat the same process above for the IBGP peering between DUT:Port2 and the ATE:Port2

**RT-1.4.7: (Send Soft Notification) Test support for RFC8538 compliance by sending a BGP Notification message to the peer**

The origial RFC4724 had no coverage for Graceful restart for BGP notification messages. Hence, even though the peers supported Graceful restart, they were expected to flush their FIB for the peering when a BGP Notification is received on the session. However with RFC8538, supporting peers should maintain their FIB even when they receive a Soft Notification. Folowing process to test,
*   Advertise prefixes between the ATE ports, through the DUT. 
*   Trigger BGP soft Notification from DUT. Please use the `gNOI.ClearBGPNeighborRequest_Soft` message as per [gNOI_proto](https://github.com/openconfig/gnoi/blob/main/bgp/bgp.proto#L41).
     *   Once the Notification is sent by the DUT, ensure that the BGP peering is up as well as verify that the packets are:
          *   Forwarded between ATE port-1 and DUT port-1 for the duration of the specified stale routes time.
          *   Dropped after the stale routes timer has expired.
          *   Forwarded again between ATE port-1 and DUT port-1 after the session is re-established.
     *   Test the above procedure on the IBGP peering between DUT port-2 and ATE port-2

**RT-1.4.8: (Receive Soft Notification) Test support for RFC8538 compliance by receiving a BGP Notification message from the peer**
*   Advertise prefixes between the ATE ports, through the DUT. 
*   Trigger BGP soft Notification from ATE port1. Please use the `gNOI.ClearBGPNeighborRequest_Soft` message as per [gNOI_proto](https://github.com/openconfig/gnoi/blob/main/bgp/bgp.proto#L41).
     *   Once the Notification is sent to the DUT, verify that the packets are:
          *   Forwarded between ATE port-1 and DUT port-1 for the duration of the specified stale routes time.
          *   Dropped after the stale routes timer has expired.
          *   Forwarded again between ATE port-1 and DUT port-1 after the session is re-established.
     *   Test the above procedure on the IBGP peering between DUT port-2 and ATE port-2

**RT-1.4.9: (Send hard Notification) Test support for RFC8538 compliance by sending a BGP Hard Notification message to the peer**
*   Advertise prefixes between the ATE ports, through the DUT. 
*   Trigger BGP hard Notification from DUT port1. Please use the `gNOI.ClearBGPNeighborRequest_hard` message as per [gNOI_proto](https://github.com/openconfig/gnoi/blob/main/bgp/bgp.proto#L43).
     *   Once the Notification is sent to the DUT, verify that,
          *   The TCP connection between DUT:Port1 <-> ATE:Port1 is reset
          *   ATE and DUT clear their FIB entries for the session and packet drop is experienced.
          *   Packets are forwarded again between ATE port-1 and DUT port-1 after the session is re-established.
     *   Test the above procedure on the IBGP peering between DUT port-2 and ATE port-2
 
**RT-1.4.10: (Receive hard Notification) Test support for RFC8538 compliance by receiving a BGP Hard Notification message from the peer**
*   Advertise prefixes between the ATE ports, through the DUT. 
*   Trigger BGP hard Notification from ATE port1. Please use the `gNOI.ClearBGPNeighborRequest_hard` message as per [gNOI_proto](https://github.com/openconfig/gnoi/blob/main/bgp/bgp.proto#L43).
     *   Once the Notification is received by the DUT, verify that,
          *   The TCP connection between DUT:Port1 <-> ATE:Port1 is reset
          *   ATE and DUT clear their FIB entries for the session and packet drop is experienced.
          *   Packets are forwarded again between ATE port-1 and DUT port-1 after the session is re-established.
     *   Test the above procedure on the IBGP peering between DUT port-2 and ATE port-2

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
