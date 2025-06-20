# RT-1.35: BGP Graceful Restart Extended route retention (ERR)

## Summary

This is an extension to the RFC8538 tests already conducted under "RT-1.4: BGP
Graceful Restart". However, ERR is for projects that need to extend the validity
of a route beyond the expiration of the stale routes timer for the BGP GR
process. Following are the scenarios when ERR can be considered by a project.

1.  Upon expiration of BGP hold-timer (Hold timer expiry on the Speaker side or
    when a notification for hold timer expiry is received from the helper)
2.  Upon the BGP session failing to re-establish within the GR restart timer as
    a helper.
3.  Upon multiple failures on the Speaker side resulting in GR restart timer or
    the stale path timer not to expire on the helper side.
4.  Upon expiration of the stale path timer Under the aforementioned conditions,
    the routes received from the neighbor under failure must be held for a
    configurable duration and processed through an additional configurable
    routing policy while being held in a “stale” state.

Since the route retention is purely local action of the receiving speaker, this
action should not require any additional capabilities advertisements beyond
capability 64 (Graceful Restart), and should not be confused with or require
capability 71 (Long-Lived Graceful Restart) from the sending speaker.

**How is this different from LLGR as tested in RT-1.14?**

As per the [IETF Draft on LLGR](https://tools.ietf.org/html/draft-ietf-idr-long-lived-gr),
we have the following that is different from EER.
*   Section 4.2 / 4.3 of the draft: mandates what communities are in use and
    what their specific behavior should be. For example: "The "LLGR_STALE"
    community must be advertised by the GR helper and also MUST NOT be removed
    by other receiving peers." and anyone that receives that route MUST treat
    the route as least-preferred. This isnt the case for ERR. There arent any
    communities attached to Stale routes thereby mandating their depreference.
*   Section 4.7: Different conditions for partial deployment of LLGR is a no-op
    for ERR as it builds on the concepts of RFC8538 and hence there arent any
    special communities expected to be sent or received for the stale routes.

**More about the ERR policy**

*   This policy can be attached at the Global, Peer-group or Neighbor levels. *
    The routes passed through the retention-policy should be the post-policy
    adj-rib-in of the neighbor. Any other import policy applied to the routes
    must not be overridden by this policy, it should be additive.
*   Default action if no ERR policy is specified should be to follow RFC8538
    behavior.
*   Please Note: In the case of an ERR policy, when the action of a given MATCH
    criteria is REJECT, the matching prefixes will be treated similar to RFC8538
    expectations. Therefore such prefixes wouldnt experience extended Retention.
    Similarly, when the policy match condition translates to an ACCEPT action,
    the prefixes are considered for ERR operation and the configured Retention
    time becomes applicable. The Prefix also gets other attributes as configured
    part of ACTION
*   Yang definitions for ERR is proposed in
    [pull/1319](https://github.com/openconfig/public/pull/1319). Following is a
    representation of how the entire config/state used in this test will look
    like.

### Config OC path for ERR

```
/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor[neighbor-address='192.168.1.1']/config/neighbor-address = "192.168.1.1"
/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor[neighbor-address='192.168.1.1']/config/hold-time = 30
/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor[neighbor-address='192.168.1.1']/graceful-restart/config/enabled = true
/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor[neighbor-address='192.168.1.1']/graceful-restart/config/restart-time = 300
/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor[neighbor-address='192.168.1.1']/graceful-restart/extended-route-retention/config/enabled = True
/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor[neighbor-address='192.168.1.1']/graceful-restart/extended-route-retention/config/retention-time = 15552000
/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor[neighbor-address='192.168.1.1']/graceful-restart/extended-route-retention/config/retention-policy = "STALE-ROUTE-POLICY"
```

````

### State OC path for ERR.

    ```
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor[neighbor-address='192.168.1.1']/state
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor[neighbor-address='192.168.1.1']/state/hold-time
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor[neighbor-address='192.168.1.1']/graceful-restart/state/enabled
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor[neighbor-address='192.168.1.1']/graceful-restart/state/restart-time
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor[neighbor-address='192.168.1.1']/graceful-restart/extended-route-retention/state/enabled
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor[neighbor-address='192.168.1.1']/graceful-restart/extended-route-retention/state/retention-time
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor[neighbor-address='192.168.1.1']/graceful-restart/extended-route-retention/state/retention-policy
    ```

## Testbed type

[atedut_2.testbed](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Topology

Create the following connections:

```mermaid
 graph LR
 A[ATE:Port1] <-- IBGP(ASN100) --> B[Port1:DUT:Port2]
 B <-- EBGP(ASN200) --> C[Port2:ATE]
```

## Test environment setup

*   ATE:Port1 runs IBGP and must advertise the following IPv4 and IPv6 prefixes
    with the corresponding commiunitty attributes
    *   IPv4Prefix1 and IPv6Prefix1 with community NO-ERR
    *   IPv4Prefix2 and IPv6Prefix2 with community ERR-NO-DEPREF
    *   IPv4Prefix3 and IPv6Prefix3 with community TEST-IBGP
*   ATE:Port2 runs EBGP and must advertise the following IPv4 and IPv6 prefixes
    with the corresponding commiunitty attributes
    *   IPv4Prefix4 and IPv6Prefix4 with community NO-ERR
    *   IPv4Prefix5 and IPv6Prefix5 with community ERR-NO-DEPREF
    *   IPv4Prefix6 and IPv6Prefix6 with community TEST-EBGP
*   DUT has the following configuration on its IBGP and EBGP peering

    *   Extended route retention (ERR) enabled.
    *   ERR configuration has the retention time of 300 secs configured
    *   ERR has a retention-policy `STALE-ROUTE-POLICY` attached.
    *   "STALE-ROUTE-POLICY" has policy-statements to identify routes tagged
        with community `NO-ERR` and have an action of "REJECT" so such routes
        aren't considered for ERR but only GR (RFC8538)
    *   identify routes tagged with community `ERR-NO-DEPREF` and have an action
        of "ACCEPT" so such routes are considered for ERR. Also ADD community
        `STALE` to the existing community list attached as part of the regular
        adj-rib-in post policy for the route.
    *   Catch-all rule to identify and accept all other prefixes, attach a
        local-preference of "0" and ADD community `STALE` to the existing
        community list.
    *   DUT has import-policy importibgp and export-policy exportibgp towards
        the IBGP neighbor applied in the import and export directions
        respectively.
    *   DUT has import-policy importebgp and export-policy exportebgp towards
        the EBGP neighbor applied in the import and export directions
        respectively.
    *   "importibgp" policy matches routes with community `testibgp` and updates
        the local-preference to 200. The policy has a catch-all statement that
        matches all other routes and accepts them.
    *   "exportibgp policy matches routes with MED 50 and sets community
        "NEW-IBGP"
    *   "importebgp" policy matches community "TEST-EBGP" and sets MED 50
    *   "exportebgp" policy matches community "TESTIBGP" and sets
        AS-PATH-PREPEND of the local ASN (100) twice and also attaches a new
        community "NEW-EBGP"
    *   DUT has the following added config
        *   hold-time 30
        *   graceful-restart restart-time = 220 secs
        *   graceful-restart stale-routes-timer = 250 secs

*   Test Flows used for verification

    *   IPv4Prefix1 <-> IPv4Prefix4, IPv6Prefix1 <-> IPv6Prefix4
    *   IPv4Prefix2 <-> IPv4Prefix5, IPv6Prefix2 <-> IPv6Prefix5
    *   IPv4Prefix3 <-> IPv4Prefix6, IPv6Prefix3 <-> IPv6Prefix6

> > Tabular representation of the above

### DUT BGP and Graceful Restart/ERR Configuration

| Parameter                | Value                | Description                |
| :----------------------- | :------------------- | :------------------------- |
| **BGP ASN**              | `100`                | The Autonomous System      |
:                          :                      : Number for the DUT's IBGP  :
:                          :                      : sessions.                  :
| **Hold Time**            | 30 seconds           | The BGP session hold       |
:                          :                      : timer.                     :
| **GR Restart Time**      | 220 seconds          | The time a peer should     |
:                          :                      : wait for the BGP session   :
:                          :                      : to re-establish during a   :
:                          :                      : graceful restart.          :
| **GR Stale Routes Time** | 250 seconds          | The duration for which a   |
:                          :                      : peer should hold stale     :
:                          :                      : routes during a graceful   :
:                          :                      : restart.                   :
| **ERR Enabled**          | `True`               | Extended Route Retention   |
:                          :                      : is enabled on the BGP      :
:                          :                      : peerings.                  :
| **ERR Retention Time**   | 300 seconds          | The time for which the DUT |
:                          :                      : will hold stale routes     :
:                          :                      : under ERR conditions.      :
| **ERR Retention Policy** | `STALE-ROUTE-POLICY` | The policy applied to      |
:                          :                      : stale routes when ERR is   :
:                          :                      : triggered.                 :

### ATE Advertised Prefixes to the DUT

**ATE:Port1 (IBGP Peer)** | Prefix | Community | | :--- | :--- | | IPv4Prefix1 /
IPv6Prefix1 | `NO-ERR` | | IPv4Prefix2 / IPv6Prefix2 | `ERR-NO-DEPREF` | |
IPv4Prefix3 / IPv6Prefix3 | `TEST-IBGP` |

**ATE:Port2 (EBGP Peer)** | Prefix | Community | | :--- | :--- | | IPv4Prefix4 /
IPv6Prefix4 | `NO-ERR` | | IPv4Prefix5 / IPv6Prefix5 | `ERR-NO-DEPREF` | |
IPv4Prefix6 / IPv6Prefix6 | `TEST-EBGP` |

--------------------------------------------------------------------------------

### DUT Routing Policies

**`STALE-ROUTE-POLICY` (Applied during ERR)** | Term Name | Match Condition |
Action(s) | | :--- | :--- | :--- | | `no-retention` | Community `NO-ERR` |
`REJECT` (Route is not retained under ERR) | | `err-no-depref` | Community
`ERR-NO-DEPREF` | `ACCEPT`; Add community `STALE` | | `default-retention` | All
other prefixes | `ACCEPT`; Set `local-preference` to 0; Add community `STALE` |

**Standard BGP Policies (Applied during normal operation)** | Policy Name |
Direction | BGP Peer | Match Condition | Action(s) | | :--- | :--- | :--- | :---
| :--- | | `importibgp` | Import | IBGP | Community `testibgp` | Set
`local-preference` to 200 | | `exportibgp` | Export | IBGP | MED `50` | Set
community `NEW-IBGP` | | `importebgp` | Import | EBGP | Community `TEST-EBGP` |
Set `MED` to 50 | | `exportebgp` | Export | EBGP | Community `TESTIBGP` |
Prepend AS-PATH with own ASN (100) twice; Set community `NEW-EBGP` |

--------------------------------------------------------------------------------

### Test Traffic Flows for Verification

Flow Name | Source Prefix | Destination Prefix
:-------- | :------------ | :-----------------
Flow 1    | IPv4Prefix1   | IPv4Prefix4
Flow 2    | IPv6Prefix1   | IPv6Prefix4
Flow 3    | IPv4Prefix2   | IPv4Prefix5
Flow 4    | IPv6Prefix2   | IPv6Prefix5
Flow 5    | IPv4Prefix3   | IPv4Prefix6
Flow 6    | IPv6Prefix3   | IPv6Prefix6

## Procedure

## RT-1.35.1: Baseline Validation and Long-Duration Timer Configuration

*   Validate that BGP peers have exchanged graceful-restart capabilities
    (capability 64) i.e. the "N" bit (the second most significant bit) is set.
    Verify this on the ATEs.
*   Verify that the restart-time = 220 Secs. Check this using OC data.
*   Configure and validate a long-duration retention timer. In addition to the
    300-second timer used for the functional tests here, configure the ERR
    retention-time to a large value (e.g., 180 days or 15552000 seconds) and
    verify that the DUT accepts and reflects this value in its configuration
    state. This validates the requirement for long-lived retention, even if
    waiting for expiration is impractical for testing . Revert to 300 seconds
    for subsequent tests. Validate the configurations using OC.
    ```
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/extended-route-retention/state/enabled
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/extended-route-retention/state/retention-time
    ```
*   Validate ERR retention-policy is set to "STALE-ROUTE-POLICY".
    ```
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/extended-route-retention/state/retention-policy`
    ```
    a. Verify that all prefixes are learned correctly on the DUT. Check the AFT
    entries for the same. On the ATEs, ensure that all the prefixes are learnt
    with the correct initial BGP attributes (communities, local-preference, MED)
    as per the import policies

      *   IPv4Prefix1 and IPv6Prefix1 has community NO-ERR
      *   IPv4Prefix2 and IPv6Prefix2 has community ERR-NO-DEPREF
      *   IPv4Prefix3 and IPv6Prefix3 has community TEST-IBGP and has a
          local-preference of 200
      *   IPv4Prefix4 and IPv6Prefix4 has community NO-ERR
      *   IPv4Prefix5 and IPv6Prefix5 has community ERR-NO-DEPREF
      *   IPv4Prefix6 and IPv6Prefix6 has community TEST-EBGP and also has a MED
          value of 50

    b. On ATE:Port1, ensure the following received from DUT: 
      * IPv4Prefix4 and IPv6Prefix4 with community NO-ERR 
      * IPv4Prefix5 and IPv6Prefix5 with community ERR-NO-DEPREF 
      * IPv4Prefix6 and IPv6Prefix6 prefixes are received with a MED of 50 and has the 
      community TEST-EBGP and NEW-EBGP in that order.

    c. On ATE:Port2, ensure the following received from DUT:
       * IPv4Prefix1 and IPv6Prefix1 has community NO-ERR
       * IPv4Prefix2 and IPv6Prefix2 has community ERR-NO-DEPREF
       * IPv4Prefix3 and IPv6Prefix3 has community TEST-IBGP and NEW-IBGP in that
       order. Also, ensure that these prefixes have an AS-PATH of "100, 100, 100"
       * Start traffic as per the Test flows above and ensure 100% success

    If any of the above verifications fail, then the test is a failure.

    ```
     /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/counters/octets-forwarded
     /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/counters/packets-forwarded
     /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group
     /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/origin-protocol
     /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix

     /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/counters/octets-forwarded
     /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/counters/packets-forwarded
     /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group
     /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/origin-protocol
     /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix
    ```

## RT-1.35.2: DUT BGP Process Restart (Graceful Termination) with ERR Policy

*   Trigger a graceful restart of the BGP process on the DUT using
    `gNOI.killProcessRequest_Signal_Term` as per
    [gNOI_proto](https://github.com/openconfig/gnoi/blob/main/system/system.proto).
    *   Please kill the right process to restart BGP. For Juniper it is the
        `RPD` process. For Arista and Cisco this is the `BGP` process. For Nokia
        this is `sr_bgp_mgr`.
*   Prevent ATEs from re-establishing BGP sessions for 330 seconds.
*   Verify traffic behavior:
    *   Prefixes with community NO-ERR (policy action REJECT) should stop
        forwarding after the restart-timer (220s) expires. This confirms REJECT
        removes the route from ERR consideration.
    *   Prefixes with ERR-NO-DEPREF should forward until the retention-time
        (300s) expires and have the STALE community added.
    *   All other prefixes should forward until retention-time expires, have
        their local-preference set to 0, and have the STALE community added.
*   Allow BGP to re-establish and verify a return to the baseline state.
*   Make sure that the ATEs receive End-of-RIB marker for the v4 and v6 peerings
    from the DUT post advertisement of all routes. If not, then the test must
    fail.
*   Validation for the above is expected to be conducted on the ATEs.

## RT-1.35.3 DUT BGP Process Restart (Abrupt Termination) with ERR Policy

*   Repeat the procedure from RT-1.35.2, but use
    gNOI.killProcessRequest_Signal_KILL for an abrupt termination.
*   The expected behavior and traffic verification steps are identical to
    RT-1.35.2 above.

## RT-1.35.4: DUT as Helper for a Restarting Peer (Graceful)

*   Trigger a graceful restart from ATE:Port1.
*   Expected behavior on the DUT is identical to RT-1.35.2, as it acts as the
    helper holding stale routes.
*   Repeat with a graceful restart from ATE:Port2.

## RT-1.35.5: DUT as Helper for a Restarting Peer (Abrupt)

*   Trigger an abrupt restart from ATE:Port1 using
    gNOI.killProcessRequest_Signal_KILL.
*   Expected behavior on the DUT is identical to RT-1.35.2.
*   Repeat with an abrupt restart from ATE:Port2.

## RT-1.35.6: BGP Notification Handling (Graceful Teardown), "Administrative Reset" Notification (rfc4486) sent by the DUT

`TODO: gNOI.ClearBGPNeighborRequest_GRACEFUL_RESET used in this case is under
review in https://github.com/openconfig/gnoi/pull/214`

*   Start traffic as per the flows above
*   Trigger BGP Notification (code 6 subocde 4) from DUT:Port1 towards
    ATE:Port1. Please use the `gNOI.ClearBGPNeighborRequest_GRACEFUL_RESET`
    message.
*   Cease notification of Code 6, subcode 4 will result in tcp connection reset
    but the routes aren't flushed
*   Configure ATE:Port1 to not send/accept any more TCP connections from the
    DUT:Port1 until the "reset timer" on the DUT expires.
*   Expected behavior is the same as RT-1.35.2
*   Revert ATE configuration to allow for the BGP sessions to be up. Restart
    traffic and confirm that there is zero packet loss. Expected behavior is
    same as the base test in RT-1.35.1
*   Restart the above procedure for the EBGP peering between DUT:Port-2 and
    ATE:Port-2

## RT-1.35.7: BGP Notification Handling (Graceful Teardown), "Administrative Reset" Notification (rfc4486) received by the DUT

`TODO: gNOI.ClearBGPNeighborRequest_GRACEFUL_RESET used in this case is under
review in https://github.com/openconfig/gnoi/pull/214`

*   Follow the same procedure as RT-1.35.6 above. However this time, Trigger BGP
    Notification (code 6 subocde 4) from ATE:Port1 towards DUT:Port1. Please use
    the `gNOI.ClearBGPNeighborRequest_GRACEFUL_RESET` message.
*   Expected result is same as RT-1.35.2 above
*   Revert ATE configurtion to allow for the BGP sessions to be up. Restart
    traffic and confirm that there is zero packet loss. Expected behavior is
    same as the base test in RT-1.35.1
*   Restart the above procedure for the EBGP peering between DUT:Port-2 and
    ATE:Port-2

## RT-1.35.8: BGP Notification Handling (Hard Reset) sent by the DUT.

`TODO: gNOI.ClearBGPNeighborRequest_HARD_RESET used in this case is under review
in https://github.com/openconfig/gnoi/pull/214`

*   Start traffic as per the flows above
*   Trigger BGP "HARD RESET" Notification from the DUT:Port1 and DUT:Port2
    towards ATE:Port1 and ATE:Port2 respectively by using
    `gNOI.ClearBGPNeighborRequest_HARD_RESET` message of the gNOI PROTO.
*   As per
    [rfc8538#section-3.1](https://datatracker.ietf.org/doc/html/rfc8538#section-3.1),
    when "N bit" exchanged between peers (i.e. GR negotiated), the "HARD RESET"
    notification of code 6 subcode 9 must be sent to the peer. However, the
    subcode for "Administrative Reset" i.e. code 6 subcode 4 must be carried in
    the data portion of subcode 9 notification message.
*   On receipt of the "HARD RESET" Notification message from the DUT, the ATEs
    MUST flush all the routes. Hence, 100% packet loss MUST be experienced on
    all the flows irrespective of the ERR configuration and the
    `STALE-ROUTE-POLICY`. The test MUST fail if this isnt the behavior seen.
    Verficiation of packet loss done on the ATE side as well as on the DUT using
    the following OC path.
    `/interfaces/interface/state/counters/out-unicast-pkts`
*   As soon as the BGP peering are up again between the ATEs and the DUT,
    traffic flow must be successful and the expected behavior must be the same
    as RT-1.35.1

## RT-1.35.9: BGP Notification Handling (Hard Reset) received by the DUT

`TODO: gNOI.ClearBGPNeighborRequest_HARD_RESET used in this case is under review
in https://github.com/openconfig/gnoi/pull/214`

*   Start traffic as per the flows above
*   Trigger BGP "HARD RESET" Notification from the ATE:Port1 to DUT:Port1 by
    sending `gNOI.ClearBGPNeighborRequest_HARD` message to ATE:Port1. When this
    happens and the DUT recieves BGP cease notification with subcode 9, the DUT
    is expected to FLUSH all IBGP learnt routes irrespective of the ERR
    configuration and therefore traffic between the flows will see 100% failure.
*   Once the IBGP peering is reestablished, expected behavior is the same as
    RT-1.35.1
*   Repeat the above process by sending gNOI.ClearBGPNeighborRequest_HARD to the
    ATE:Port2. Expected behavior here is the same as seen for the IBGP peering.

## RT-1.35.10: Additive Policy Application

Verify that the ERR retention policy is additive to the standard import policy.

*   In addition to the existing importibgp policy that sets local-preference to
    200, add an action to set the MED to 150 for prefixes with community
    TEST-IBGP.
*   In the baseline state, verify IPv4Prefix3 and IPv6Prefix3 have both
    local-preference 200 and MED 150. Use the loc-rib state paths below.
*   Trigger an ERR state as in RT-1.35.2. The STALE-ROUTE-POLICY should apply a
    local-preference of 0. Verify using the LOC-RIB path below.
*   During the ERR state, verify that the prefixes now have a local-preference
    of 0 (from ERR policy), a MED of 150 (from import policy), and the STALE
    community (from ERR policy).
*   This confirms the retention policy does not override pre-existing attributes
    set by other policies. If the behavior is different from this expectation
    then fail the test.

```
 /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/ext-community-index
 /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/attr-index
```

## RT-1.35.12: Default Reject Behavior

Verify that the default behavior is to drop stale routes when ERR is enabled but
no policy is attached. 

* Enable ERR on a BGP neighbor but do not configure a retention-policy. 
* Trigger a BGP session failure that would normally activate ERR 
(e.g., as in RT-1.35.2). 
* Verify that after the restart-timer expires, all routes from the failed 
neighbor are flushed and traffic for all flows drops to 0%. Validate on the ATE 
as well as using OC.
`/interfaces/interface/state/counters/out-unicast-pkts` 
* Test Must fail if the default action without ERR isn't satisfied.


## RT-1.35.13: Consecutive BGP Restarts

Verify that ERR is correctly triggered during rapid, consecutive session
failures. This test addresses the specific, complex failure scenario where
timers may not expire normally due to flapping.

* Establish a baseline state with active traffic.
* Initiate a loop on the ATE peer: gracefully kill the BGP
process, wait for the session to re-establish with the DUT, and then immediately
kill it again. This cycle should be shorter than the DUT's stale-routes-time.
Check the BGP peering state on the ATEs as well as using OC.
`/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state`
* After several such cycles, stop the loop and prevent the ATE from
reconnecting.
* Verify that the DUT enters the ERR state and holds the routes
according to the STALE-ROUTE-POLICY, as validated in RT-1.35.2.


## RT-1.35.14

Repeat the tests above, with ERR configuration under the peer-group hierarchy.

## Canonical OpenConfig for ERR

```
    {
        "network-instance": [
          {
            "name": "DEFAULT",
            "protocols": {
              "protocol": [
                {
                  "identifier": "BGP",
                  "name": "BGP",
                  "bgp": {
                    "neighbors": {
                      "neighbor": [
                        {
                          "neighbor-address": "192.168.1.1",
                          "graceful-restart": {
                            "config": {
                              "enabled": true
                            },
                            "extended-route-retention": {
                              "config": {
                                "enabled": true,
                                "retention-time": 15552000,
                                "retention-policy": "STALE-ROUTE-POLICY"
                              }
                            }
                          }
                        }
                      ]
                    }
                  }
                }
              ]
            }
          }
        ]
    }
```

## OpenConfig Path and RPC Coverage

```yaml
paths:


   # BGP conifguration:
      /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-group:
      /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/neighbor-address:
      /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as:
      /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/local-as:
      /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/graceful-restart/config/enabled:
      /network-instances/network-instance/protocols/protocol/bgp/global/graceful-restart/config/restart-time:
      /network-instances/network-instance/protocols/protocol/bgp/global/graceful-restart/config/stale-routes-time:
      /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/extended-route-retention/config/enabled:
      /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/extended-route-retention/config/retention-time:
      /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/extended-route-retention/config/retention-policy:

  # Telemetry Parameter Coverage

      /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/graceful-restart/state/advertised:
      /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/peer-restart-time:
      /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/graceful-restart/state/received:
      /network-instances/network-instance/protocols/protocol/bgp/global/graceful-restart/state/restart-time:
      /network-instances/network-instance/protocols/protocol/bgp/global/graceful-restart/state/stale-routes-time:
      /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/community-index:
      /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/restart-time:
      /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/enabled:
      /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/extended-route-retention/state/enabled:
      /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/extended-route-retention/state/retention-time:
      /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/extended-route-retention/state/retention-policy:
      /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/counters/octets-forwarded:
      /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/counters/packets-forwarded:
      /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
      /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/origin-protocol:
      /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
      /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/counters/octets-forwarded:
      /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/counters/packets-forwarded:
      /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group:
      /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/origin-protocol:
      /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
      /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/path-id:
      /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/ext-community-index:
      /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/attr-index:
      /interfaces/interface/state/counters/out-unicast-pkts:


rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Get:
    gNMI.Subscribe:
  gnoi:
    system.System.KillProcess:
    # bgp.ClearBGPNeighborRequest.Hard:
```

