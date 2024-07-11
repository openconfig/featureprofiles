# RT-1.4: BGP Graceful Restart

## Summary

This is to test the BGP graceful restart capability for BGP. A router that supports BGP graceful restart can work either as a Restarting speaker mode or in helper mode. By advertising BGP graceful restart capability, a router announces to the peer its ability to,
1.  [Restarting Speaker] Maintain forwarding state on all the routes in its FIB even when its BGP process is restarting. Therefore the peer functioning as a helper should continue to direct flows at the subject router undergoing BGP process restart.
2.  [Helper Router] Support a peer whose BGP process is restarting by continuing to direct flows at the peer.
3.  The test checks support for RFC4724 and RFC8538.

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
*   Ensure that the `restart-time` and the `stale-routes-time` are configured at the `Global` level. The `stale-routes-time` should be set at a value less than the BGP Holddown timer.
*   Configure allow route-policy under BGP peer-group address-family
*   Validate received capabilities at DUT and ATE reflect support for graceful
     restart.
    

**RT-1.4.2: Restarting DUT speaker whose BGP process was killed gracefully**
*   Advertise prefixes between the ATE ports, through the DUT. 
*   Trigger DUT session restart by killing the BGP process in the DUT. Please use the `gNOI.killProcessRequest_Signal_Term` as per [gNOI_proto](https://github.com/openconfig/gnoi/blob/main/system/system.proto#L326).
     *   Please kill the right process to restart BGP. For Juniper it is the `RPD` process. For Arista and Cisco this is the `BGP` process. For Nokia this is `sr_bgp_mgr`.
*   Once the BGP process on DUT is killed, configure ATE to delay the BGP reestablishment for a period longer than the `stale-routes-time` and start regular traffic from ATE and verify that the packets are,
     *   Forwarded between ATE port-1 and ATE port-2 for the duration of the specified `stale-routes-time`. Before the stale routes timer expires, stop traffic and ensure that there is zero packet loss.
     *   After the stale routes timer expires, restart traffic and confirm that there is 100% packet loss.
*   Stop traffic, revert ATE configuration to start accepting packets for BGP reestablishement from DUT and wait for the BGP session w/ ATE to be reestablished. Once established, restart traffic to ensure that packets are forwarded again between ATE port-1 and ATE port2 and there is zero packet loss.
*   Conduct the above steps once for the EBGP peering and once for the IBGP peering


**RT-1.4.3: Restarting DUT speaker whose BGP process was killed abruptly**
*   Follow the same steps as in RT-1.4.2 above but use `gNOI.killProcessRequest_Signal_KILL` this time as per `gNOI proto`
*   Pass/Fail criteria in this case too is the same as that for RT-1.4.2. Router that supports Graceful restart is expected to allow traffic flow w/o any packet drops until the `stale-routes-time` timer expires.


**RT-1.4.4: DUT Helper for a restarting EBGP speaker whose BGP process was gracefully killed**
*   Advertise prefixes between the ATE ports through the DUT. Send Graceful restart trigger from ATE port-1.
*   Start traffic between ATE port-1 and ATE port-2 and prior to the expiry of `stale-routes-time`, stop traffic and ensure that there is zero packet loss.
*   Restart traffic post the stale routes timer expiry. Ensure that the subject prefixes are withdrawn, and there is 100% traffic loss between ATE:Port1 and ATE:Port2.
*   Repeat the above for the IBGP peering on ATE port2
 
**RT-1.4.5: DUT Helper for a restarting EBGP speaker whose BGP process was killed abruptly**
*   Advertise prefixes between the ATE ports through the DUT. Use `gNOI.killProcessRequest_Signal_KILL` as per `gNOI proto` to ATE:Port1.
*   Once the BGP process on DUT is killed, configure ATE to delay the BGP reestablishment for a period longer than the `stale-routes-time` and start regular traffic from ATE and verify that the packets are,
     *   Forwarded between ATE port-1 and ATE port-2 for the duration of the specified `stale-routes-time`. Before the stale routes timer expires, stop traffic and ensure that there is zero packet loss.
     *   After the stale routes timer expires, restart traffic and confirm that there is 100% packet loss.
*   Stop traffic, revert ATE configuration to start accepting/sending packets for BGP reestablishement from/to DUT and wait for the BGP session w/ ATE to be reestablished. Once established, restart traffic to ensure that packets are forwarded again between ATE port-1 and ATE port2 and there is zero packet loss.
*   Conduct the above steps once for the IBGP peering between DUT:Port2 and ATE:Port2.

**RT-1.4.6: Test support for RFC8538 compliance by letting Hold-time expire**

RFC-8538 builds on RFC4724 by adding Graceful restart support for scenarios when the BGP holdtime expires. In order to simulate holdtime expiry, please install an ACL on DUT that drops BGP packets from the Peer (i.e. ATE). Also this time, configure the stale-routes-timer to be longer than the hold-timer. Start traffic and ensure that the packets are,
*   Forwarded between ATE port-1 and ATE port-2 for the duration of the specified stale routes time. Stop traffic somtime after the holdtime expires but before the stale-routes-timer expires and confirm that there was zero packet loss.
*   Once the stale-routes-timer expires, start traffic again and confirm that there is 100% packet loss. Stop traffic.
*   Remove the ACL on DUT:Port1 and allow BGP to be reestablished. Start traffic again between ATE port1 and ATE port2. This time ensure that there is zero packet loss. Stop traffic again.
*   Repeat the same process above for the IBGP peering between DUT:Port2 and the ATE:Port2

**RT-1.4.7: (Send Soft Notification) Test support for RFC8538 compliance by sending a BGP Notification message to the peer**

The origial RFC4724 had no coverage for Graceful restart process post send/receive of a Soft BGP notification message. Hence, even though the peers supported Graceful restart, they were expected to flush their FIB for the peering when a BGP Notification is received on the session. However with RFC8538, supporting peers should maintain their FIB even when they receive a Soft Notification. Folowing process to test,
*   Advertise prefixes between the ATE ports, through the DUT. 
*   Trigger BGP soft Notification to/from DUT Port1 towards ATE port1. Please use the `gNOI.ClearBGPNeighborRequest_Soft` message as per [gNOI_proto](https://github.com/openconfig/gnoi/blob/main/bgp/bgp.proto#L41). Once the Notification is received and the TCP connection is reset, configure ATE Port1 to not send/accept any more TCP conenctions from the DUT:Port1 until the stale-routes-timer on the DUT expires. 
     *   Start traffic from ATE Port1 towards ATE Port2 and stop the same right before the stale-routes-timer expires. Confirm there is zero packet loss.
     *   Once the stale-routes-timer expires, restart traffic. Expectations are that there is 100% packet loss. Stop traffic
*   Revert ATE configurtion blocking TCP connection from DUT over TCP-Port:179 so the EBGP peering between ATE:Port1 <> DUT:port1 is reestablished. Restart traffic and confirm that there is zero packet loss.
*   Restart the above procedure for the IBGP peering between DUT port-2 and ATE port-2

**RT-1.4.8: (Receive Soft Notification) Test support for RFC8538 compliance by receiving a BGP Notification message from the peer**
*   Advertise prefixes between the ATE ports, through the DUT. 
*   Trigger BGP soft Notification from ATE port1. Please use the `gNOI.ClearBGPNeighborRequest_Soft` message as per [gNOI_proto](https://github.com/openconfig/gnoi/blob/main/bgp/bgp.proto#L41). Once the Notification is sent and the TCP connection is reset, configure ATE Port1 to not start/accept any more TCP conenctions from the DUT:Port1 until the stale-routes-timer on the DUT expires. 
     *   Start traffic from ATE Port1 towards ATE Port2 and stop the same right before the stale-routes-timer expires. Confirm there is zero packet loss.
     *   Once the stale-routes-timer expires, restart traffic. Expectations are that there is 100% packet loss. Stop traffic.
*   Revert ATE configurtion blocking TCP connection to/from DUT over TCP-Port:179 so the EBGP peering between ATE:Port1 <> DUT:port1 is reestablished. Restart traffic and confirm that there is zero packet loss. 
*   Restart the above procedure for the IBGP peering between DUT port-2 and ATE port-2


**RT-1.4.9: (Send hard Notification) Test support for RFC8538 compliance by sending a BGP Hard Notification message to the peer**
*   Advertise prefixes between the ATE ports, through the DUT. 
*   Trigger BGP hard Notification from DUT port1. Please use the `gNOI.ClearBGPNeighborRequest_hard` message as per [gNOI_proto](https://github.com/openconfig/gnoi/blob/main/bgp/bgp.proto#L43). Once the Notification is sent and the TCP connection is reset, configure ATE Port1 to not start/accept any more TCP conenctions to/from the DUT:Port1.
     *   Start traffic from ATE Port1 towards ATE Port2. Confirm there is zero packet loss. Stop traffic.
*   Revert ATE configurtion blocking TCP connection to/from DUT over TCP-Port:179 so the EBGP peering between ATE:Port1 <> DUT:port1 is reestablished. Restart traffic and confirm that there is zero packet loss. 
*   Restart the above procedure for the IBGP peering between DUT port-2 and ATE port-2


**RT-1.4.10: (Receive hard Notification) Test support for RFC8538 compliance by receiving a BGP Hard Notification message from the peer**
*   Advertise prefixes between the ATE ports, through the DUT. 
*   Trigger BGP hard Notification from ATE port1. Please use the `gNOI.ClearBGPNeighborRequest_hard` message as per [gNOI_proto](https://github.com/openconfig/gnoi/blob/main/bgp/bgp.proto#L43). Once the Notification is sent and the TCP connection is reset, configure ATE Port1 to not start/accept any more TCP conenctions to/from the DUT:Port1.
     *   Start traffic from ATE Port1 towards ATE Port2. Confirm there is zero packet loss. Stop traffic.
*   Revert ATE configurtion blocking TCP connection to/from DUT over TCP-Port:179 so the EBGP peering between ATE:Port1 <> DUT:port1 is reestablished. Restart traffic and confirm that there is zero packet loss. 
*   Restart the above procedure for the IBGP peering between DUT port-2 and ATE port-2
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

## OpenConfig Path and RPC Coverage

```yaml
rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Get:
    gNMI.Subscribe:
  gnoi:
    system.System.KillProcess:
```