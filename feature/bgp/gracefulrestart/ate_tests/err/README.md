# RT-1.35: BGP Graceful Restart Extended route retention (ERR)

## Summary

This is an extension to the RFC8538 tests already conducted under "RT-1.4: BGP Graceful Restart". However, ERR is for projects that need to extend the validity of a route beyond the expiration of the stale routes timer for the BGP GR process. Following are the scenarios when ERR can be considered by a project.
  1. Upon expiration of BGP hold-timer (Hold timer expiry on the Speaker side or when a notification for hold timer expiry is received from the helper)
  2. Upon the BGP session failing to re-establish within the GR restart timer as a helper.
  3. Upon multiple failures on the Speaker side resulting in GR restart timer or the stale path timer not to expire on the helper side.
  4. Upon expiration of the stale path timer
Under the aforementioned conditions, the routes received from the neighbor under failure must be held for a configurable duration and processed through an additional configurable routing policy while being held in a “stale” state.

Since the route retention is purely local action of the receiving speaker, this action should not require any additional capabilities advertisements beyond capability 64 (Graceful Restart), and should not be confused with or require capability 71 (Long-Lived Graceful Restart) from the sending speaker.

**How is this different from LLGR as tested in RT-1.14?**
As per the [IETF Draft on LLGR](https://tools.ietf.org/html/draft-ietf-idr-long-lived-gr), we have the following that is different from EER.

  - Section 4.2 / 4.3 of the draft: mandates what communities are in use and what their specific behavior should be. For example: "The "LLGR_STALE" community must be advertised by the GR helper and also MUST NOT be removed by other receiving peers." and anyone that receives that route MUST treat the route as least-preferred. This isnt the case for ERR. There arent any communities attached to Stale routes thereby mandating their depreference.
  - Section 4.7: Different conditions for partial deployment of LLGR is a no-op for ERR as it builds on the concepts of RFC8538 and hence there arent any special communities expected to be sent or received for the stale routes.

## Testbed type
[atedut_2.testbed](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

* Test environment setup
  ## Topology
Create the following connections:
```mermaid
graph LR; 
A[ATE:Port1] <-- IBGP(ASN100) --> B[Port1:DUT:Port2];
B <-- EBGP(ASN200) --> C[Port2:ATE];
```
* ATE:Port1 runs IBGP and must advertise the following IPv4 and IPv6 prefixes with the corresponding commiunitty attributes
  * IPv4Prefix1 and IPv6Prefix1 with community NO-ERR
  * IPv4Prefix2 and IPv6Prefix2 with community ERR-NO-DEPREF
  * IPv4Prefix3 and IPv6Prefix3 with community TEST-IBGP
* ATE:Port2 runs EBGP and must advertise the following IPv4 and IPv6 prefixes with the corresponding commiunitty attributes
  * IPv4Prefix4 and IPv6Prefix4 with community NO-ERR
  * IPv4Prefix5 and IPv6Prefix5 with community ERR-NO-DEPREF
  * IPv4Prefix6 and IPv6Prefix6 with community TEST-EBGP
* DUT has the following configuration on its IBGP and EBGP peering
  * Extended route retention (ERR) enabled.
  * ERR configuration has the retention time of 300 secs configured
  * ERR has a retention-policy `STALE-ROUTE-POLICY` attached. The DUT **MUST** apply this policy on post-policy adj-rib-in routes. Therefore, import export policy applied to the routes must not be overridden by this policy, it MUST instead be additive.
  * "STALE-ROUTE-POLICY" has policy-statements to
    * identify routes tagged with community `NO-ERR` and have an action of "REJECT" so such routes aren't considered for ERR but only GR
    * identify routes tagged with community `ERR-NO-DEPREF` and have an action of "ACCEPT" so such routes are considered for ERR. Also ADD community `STALE` to the existing community list attached as part of the regular adj-rib-in post policy for the route.
    * Catch-all rule to identify and accept all other prefixes, attach a local-preference of "0" and ADD community `STALE` to the existing community list.
  * DUT has import-policy importibgp and export-policy exportibgp towards the IBGP neighbor applied in the import and export directions respectively.
  *  DUT has import-policy importebgp and export-policy exportebgp towards the EBGP neighbor applied in the import and export directions respectively.
  * "importibgp" policy matches routes with community `testibgp` and updates the local-preference to 200. The policy has a catch-all statement that matches all other routes and accepts them.
  * "exportibgp policy matches routes with MED 50 and sets community "NEW-IBGP"
  * "importebgp" policy matches community "TEST-EBGP" and sets MED 50
  * "exportebgp" policy matches community "TESTIBGP" and sets AS-PATH-PREPEND of the local ASN (100) twice and also attaches a new community "NEW-EBGP"
    * DUT has the follwing added config
      * hold-time 30
      * graceful-restart restart-time = 220 secs
      * graceful-restart stale-routes-timer = 250 secs
     
* Test Flows used for verification
  * IPv4Prefix1 <-> IPv4Prefix4, IPv6Prefix1 <-> IPv6Prefix4
  * IPv4Prefix2 <-> IPv4Prefix5, IPv6Prefix2 <-> IPv6Prefix5
  * IPv4Prefix3 <-> IPv4Prefix6, IPv6Prefix3 <-> IPv6Prefix6


...

* RT-1.35.1 Validate Graceful-Restart (Baseline)
  * Validate received capabilities at DUT and ATE reflect support for graceful restart and also verify that the restart-time = 220 Secs and  stale-routes-timer = 250 Secs.
  * Validate ERR retention-time is as configured i.e. 300s
  * Validate the ERR retention-policy matches "STALE-ROUTE-POLICY"
  * TODO: Following OC-paths need to be added to the Yang model
    ```
    * /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/extended-route-retention/state/retention-time <?>
    * /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/extended-route-retention/state/retention-policy <?>
    ```
  * Ensure the DUT has learnt all the Prefixes over the IBGP and EBGP sessions and has the correct community list attached to the routes in its post-policy ADJ-RIBIN
    * IPv4Prefix1 and IPv6Prefix1 has community NO-ERR
    * IPv4Prefix2 and IPv6Prefix2 has community ERR-NO-DEPREF
    * IPv4Prefix3 and IPv6Prefix3 has community TEST-IBGP and has a local-preference of 200
    * IPv4Prefix4 and IPv6Prefix4 has community NO-ERR
    * IPv4Prefix5 and IPv6Prefix5 has community ERR-NO-DEPREF
    * IPv4Prefix6 and IPv6Prefix6 has community TEST-EBGP and also has a MED value of 50
  * On ATE:Port1, ensure the following received from DUT:
    * IPv4Prefix4 and IPv6Prefix4 with community NO-ERR
    * IPv4Prefix5 and IPv6Prefix5 with community ERR-NO-DEPREF
    * IPv4Prefix6 and IPv6Prefix6 prefixes are received with a MED of 50 and has the community TEST-EBGP and NEW-EBGP in that order.
    * On ATE:Port2, ensure the following received from DUT:
      * IPv4Prefix1 and IPv6Prefix1 has community NO-ERR
      * IPv4Prefix2 and IPv6Prefix2 has community ERR-NO-DEPREF
      * IPv4Prefix3 and IPv6Prefix3 has community TEST-IBGP and NEW-IBGP in that order. Also, ensure that these prefixes have an AS-PATH of "100, 100, 100"
    * Start traffic as per the Test flows above and ensure 100% success

    If any of the above verifications fail, then the test is a failure.


...

    
* RT-1.35.2
  a. Restarting DUT speaker whose BGP process was killed gracefully. In this case ERR policy is attached to the BGP neighborship.
    * Trigger DUT session restart by gracefully killing the BGP process in the DUT. Please use the `gNOI.killProcessRequest_Signal_Term` as per [gNOI_proto](https://github.com/openconfig/gnoi/blob/main/system/system.proto#L326).
    * Please kill the right process to restart BGP. For Juniper it is the `RPD` process. For Arista and Cisco this is the `BGP` process. For Nokia this is `sr_bgp_mgr`.
    * Once the BGP process on DUT is killed, configure both ATEs to delay the BGP reestablishment for 330 secs longer than the `HOLD-TIME` and start regular traffic from between ATEs for the prefixes adevertised and verify that the packets are treated as follows. If not, the Test Must fail.
      * Traffic between prefixes IPv4Prefix1, IPv6Prefix1, IPv4Prefix4 and IPv6Prefix4 MUST be successful until the "restart timer" expires and dropped after that.
      * Traffic between prefixes IPv4Prefix2, IPv6Prefix2, IPv4Prefix5 and IPv6Prefix5 MUST be successful until the ERR retention-time expires and dropped after that. The routes for these prefixes must also have the community STALE added to the end of the community-list as recieved at the ATE end.
      * Traffic between prefixes IPv4Prefix3, IPv6Prefix3, IPv4Prefix6 and IPv6Prefix6 MUST be successful until the retention-time expires and dropped after that. The routes for these prefixes must also have the Local-Preference of "0" and the community value of "STALE" attached to the end of the community-list.
    * Post 330 secs, the ATEs are allowed to form the BGP neighborship with the DUT. Readvertisements of the EBGP and IBGP prefixes will takeplace and the state of the routes and their BGP attributes as well as traffic flow is expected to be the same as the baseline results in RT-1.35.1 above. If not, then the test Must fail.

  b. Restarting DUT speaker whose BGP process was killed gracefully after removing the ERR policy
    * In this case too, Once the BGP process on DUT is killed, configure both ATEs to delay the BGP reestablishment for 330 secs longer than the `HOLD-TIME` and start regular traffic from between ATEs for the prefixes adevertised and verify that the packets are treated as follows. If not, the Test Must fail.
      * When ERR has no ERR policy attached, behavior is expected to be as defined in RFC 8538 and RFC 4724 i.e. traffic flow between prefixes is successful only until the restart timer expires. After that, 100% packet drop is expected.
      * Since there isnt any ERR policy attached, changes to the community and Local-Pref attributes as defined in that policy (STALE-ROUTE-POLICY) isnt expected. That is, the community-list attached to the routes learnt from the DUT will be the same as the baseline test above i.e. RT-1.35.1. If not, then the test Must fail
    * Post 330 secs, the ATEs are allowed to form the BGP neighborship with the DUT. Readvertisements of the EBGP and IBGP prefixes will takeplace and the state of the routes and their BGP attributes as well as traffic flow is expected to be the same as the baseline results in RT-1.35.1 above. If not, then the test Must fail.
 

...

* RT-1.35.3
  a. Restarting DUT speaker whose BGP process was killed abruptly. In this case ERR policy is attached to the BGP neighborship.
    * use `gNOI.killProcessRequest_Signal_KILL` this time as per `gNOI proto`
    * Expected behavior is the same as RT-1.35.2.a
 
  b. Restarting DUT speaker whose BGP process was killed abruptly after ERR policy was removed
    * Expected behavior is the same as RT-1.35.2.b
  

...

* RT-1.35.4
  a. DUT Helper for a restarting EBGP speaker whose BGP process was gracefully killed. In this case ERR policy is attached to the BGP neighborship.
    * Send Graceful restart trigger from ATE:Port1
    * Expected behavior is the same as RT-1.35.2.a
    * Repeat by sending Graceful restart trigger from ATE:Port2 with the same expected behavior as RT-1.35.2.a

  b. DUT Helper for a restarting EBGP speaker whose BGP process was gracefully killed after ERR policy was removed
    * Send Graceful restart trigger from ATE:Port1
    * Expected behavior is the same as RT-1.35.2.b
    * Repeat by sending Graceful restart trigger from ATE:Port2 with the same expected behavior as RT-1.35.2.b
  

...

* RT-1.35.5
  a. DUT Helper for a restarting EBGP speaker whose BGP process was killed abruptly. In this case ERR policy is attached to the BGP neighborship.
    * Start traffic. Send `gNOI.killProcessRequest_Signal_KILL` as per `gNOI proto` to ATE:Port1 to stop its BGP process abruptly. Configure ATE:Port1 to delay the BGP reestablishment for 330 secs over the Hold-time. Expected behavior in this case is the same as RT-1.35.2.a
    * Post 330Secs over Hold-time expiry, BGP on ATE:Port1 is expected to be up and all traffic is expected to be successful.
    * Repeat the same test on ATE:Port2
  
  b. DUT Helper for a restarting EBGP speaker whose BGP process was killed abruptly after ERR policy was removed
    * Start traffic. Send `gNOI.killProcessRequest_Signal_KILL` as per `gNOI proto` to ATE:Port1 to stop its BGP process abruptly. Configure ATE:Port1 to delay the BGP reestablishment for 330 secs over the Hold-time. Expected behavior in this case is the same as RT-1.35.2.b
    * Post 330Secs over Hold-time expiry, BGP on ATE:Port1 is expected to be up and all traffic is expected to be successful.
    * Repeat the same test on ATE:Port2
  


  
* RT-1.35.6
  a. Expected behavior when Soft Notification Sent to the peer and the ERR policy is attached
   *   Start traffic as per the flows above
   *   Trigger BGP soft Notification (code 6 subocde 4) from DUT:Port1 towards ATE:Port1. Please use the `gNOI.ClearBGPNeighborRequest_Soft` message as per [gNOI_proto](https://github.com/openconfig/gnoi/blob/main/bgp/bgp.proto#L41). 
   *   Cease notification of Code 6, subcode 4 will result in tcp connection reset but the routes arent flushed
   *   Configure ATE:Port1 to not send/accept any more TCP conenctions from the DUT:Port1 until the reset timer on the DUT expires.
   *   Expected behavior is the same as RT-1.35.2.a
   *   Revert ATE configurtion to allow for the BGP sessions to be up. Restart traffic and confirm that there is zero packet loss. Expected behavior is same as the base test in RT-1.35.1
   *   Restart the above procedure for the IBGP peering between DUT:Port-2 and ATE:Port-2

  
  * Expected behavior when Soft Notification Sent to the peer when ERR policy removed
* RT-1.35.6
  * Expected behavior when Soft Notification received from the peer when ERR policy attached
  * Expected behavior when Soft Notification received from the peer when ERR policy removed
* RT-1.35.7
  * Expected behavior when Hard Notification Sent to the peer when ERR policy attached
  * Expected behavior when Hard Notification Sent to the peer when ERR policy removed
* RT-1.35.8
  * Expected behavior when Hard Notification received from the peer when ERR policy attached
  * Expected behavior when Hard Notification received from the peer when ERR policy removed
* RT-1.35.9
  * Expected behavior when routes have added communities part of the regular import and export policies apart from the ERR policy

  
