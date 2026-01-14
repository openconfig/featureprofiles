# **RT-7.51: BGP Auto-Generated Link-Bandwidth Community**


## **Summary**

These tests validate the functionality of the BGP auto-generated link-bandwidth extended community feature, which is configured using OpenConfig. The primary goal is to verify that the DUT correctly performs **Weighted Equal-Cost Multi-Path (wECMP)** forwarding based on the dynamic bandwidth of its BGP paths.

The tests ensure that a DUT correctly generates and attaches the link-bandwidth extended community to routes learned from eBGP peers. They also validate that separate IPv4 and IPv6 traffic flows are load-balanced across these BGP paths in direct proportion to the bandwidths indicated. The plan covers LAG dynamics, the hold-down timer, transitivity configuration, and precedence rules for both peer-advertised communities and local configuration hierarchy (neighbor vs. peer-group).


## **Testbed Topology**

**Testbed Type:**[ atedut5](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_5.testbed).

The test requires a single DUT with at least 5 ports connected to an ATE/OTG with at least 5 ports. The topology consists of one traffic-source ATE port and two sets of BGP peer links, each configured as a LAG.



*   `ATE Port 1 <--> DUT Port 1`: Used as the source for the test traffic.
*   `ATE Ports 2,3 <--> DUT Ports 2,3`: Forms `LAG-1` for the eBGP session with Peer 1.
*   `ATE Ports 4,5 <--> DUT Ports 4,5`: Forms `LAG-2` for the eBGP session with Peer 2.


## **RPCs and Protocols**



*   **gNMI:** Used for all configuration and telemetry operations.
*   **eBGP:** Used for advertising routes from ATE Peers 1 and 2 to the DUT.
*   **LACP: **Used for logically grouping physical links between DUT and Peers 1 and 2. 


## **Design & Implementation Details**

This feature enables wECMP by automatically attaching a BGP extended community to learned routes, with a value representing the **real-time bandwidth** of the ingress interface. The BGP decision process can then use this bandwidth value as a weight.

This test validates this functionality by creating a classic wECMP scenario:



1. ATE Peer 1 (on `LAG-1`) and ATE Peer 2 (on `LAG-2`) advertise the **same destination prefix** to the DUT, creating two paths.
2. The DUT, with `auto-link-bandwidth` enabled on the peer-group, attaches an LBW community to the routes learned from each peer.
3. ATE Source (on `Port 1`) sends traffic to the destination prefix.
4. The tests measure the traffic received by ATE Peer 1 and ATE Peer 2 to verify that the DUT load-balanced the traffic in proportion to the bandwidths specified in the LBW communities.


## Canonical OC
```json
{
  "network-instances": {
    "network-instance": [
      {
        "name": "DEFAULT",
        "protocols": {
          "protocol": [
            {
              "identifier": "BGP",
              "name": "BGP",
              "bgp": {
                "global": {
                  "config": {
                    "as": 65501,
                    "router-id": "192.0.2.1"
                  },
                  "afi-safis": {
                    "afi-safi": [
                      {
                        "afi-safi-name": "IPV4_UNICAST",
                        "config": {
                          "afi-safi-name": "IPV4_UNICAST",
                          "enabled": true
                        },
                        "use-multiple-paths": {
                          "ebgp": {
                            "config": {
                              "maximum-paths": 2
                            }
                          }
                        }
                      },
                      {
                        "afi-safi-name": "IPV6_UNICAST",
                        "config": {
                          "afi-safi-name": "IPV6_UNICAST",
                          "enabled": true
                        },
                        "use-multiple-paths": {
                          "ebgp": {
                            "config": {
                              "maximum-paths": 2
                            }
                          }
                        }
                      }
                    ]
                  }
                },
                "peer-groups": {
                  "peer-group": [
                    {
                      "peer-group-name": "ATE-PEERS-V4",
                      "auto-link-bandwidth": {
                        "import": {
                          "config": {
                            "enabled": true,
                            "transitive": false
                          }
                        }
                      }
                    },
                    {
                      "peer-group-name": "ATE-PEERS-V6",
                      "auto-link-bandwidth": {
                        "import": {
                          "config": {
                            "enabled": true,
                            "transitive": false
                          }
                        }
                      }
                    }
                  ]
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "neighbor-address": "192.0.2.2",
                      "config": {
                        "peer-group": "ATE-PEERS-V4"
                      }
                    },
                    {
                      "neighbor-address": "192.0.2.4",
                      "config": {
                        "peer-group": "ATE-PEERS-V4"
                      }
                    },
                    {
                      "neighbor-address": "2001:db8::2",
                      "config": {
                        "peer-group": "ATE-PEERS-V6"
                      }
                    },
                    {
                      "neighbor-address": "2001:db8::4",
                      "config": {
                        "peer-group": "ATE-PEERS-V6"
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
}

```



## **Feature Coverage**

This test plan covers the following OpenConfig paths under `/network-instances/network-instance/protocols/protocol/bgp/`:


```
peer-groups/peer-group/auto-link-bandwidth/import/config/enabled
peer-groups/peer-group/auto-link-bandwidth/import/config/hold-down-time
peer-groups/peer-group/auto-link-bandwidth/import/config/transitive
neighbors/neighbor/auto-link-bandwidth/import/config/enabled
neighbors/neighbor/auto-link-bandwidth/import/config/hold-down-time
neighbors/neighbor/auto-link-bandwidth/import/config/transitive

```



## **Test Cases**


### **RT-7.51.1: Baseline wECMP Forwarding**



*   **Objective:** Verify that with two healthy, equal-sized LAGs, traffic is balanced proportionally (50/50) for both IPv4 and IPv6.
*   **Procedure Details:** Both LAGs consist of 2 member ports of the same speed. Both ATE Peer 1 and Peer 2 advertise prefix `P1` (IPv4) and `P2` (IPv6).

<table>
  <tr>
   <td style="background-color: null">
<strong>Test Case</strong>
   </td>
   <td style="background-color: null"><strong>Procedure</strong>
   </td>
   <td style="background-color: null"><strong>Validation</strong>
   </td>
  </tr>
  <tr>
   <td style="background-color: null"><strong>7.51.1.1 - Equal Balancing (IPv4)</strong>
   </td>
   <td style="background-color: null">1. Establish both eBGP sessions (IPv4 AF) over <code>LAG-1</code> and <code>LAG-2</code>.
<p>
2. On DUT, configure a peer-group for IPv4 and enable <code>auto-link-bandwidth</code>. Assign both neighbors to this peer-group.
<p>
3. ATE Peers 1 & 2 advertise IPv4 prefix <code>P1</code>.
<p>
4. ATE Source sends a baseline rate of IPv4 traffic to <code>P1</code>.
   </td>
   <td style="background-color: null"><strong>Data Plane (Primary):</strong> Verify that ATE Peer 1 and ATE Peer 2 each receive approximately 50% of the sent traffic (+/- 5% tolerance).
<p>
<strong>Control Plane (Optional):</strong> If DUT streaming telemetry is enabled, query the <code>.../adj-rib-in-post/.../ext-community-index</code> for the routes to <code>P1</code> from both peers. Verify the index points to a valid LBW community with a value for the full LAG bandwidth (2x member link speed). Also, check the <code>/network-instances/network-instance/afts/...</code> for equal <code>weight</code> for both next-hops.
   </td>
  </tr>
  <tr>
   <td style="background-color: null"><strong>7.51.1.2 - Equal Balancing (IPv6)</strong>
   </td>
   <td style="background-color: null">1. Establish both eBGP sessions (IPv6 AF) over <code>LAG-1</code> and <code>LAG-2</code>.
<p>
2. On DUT, configure a peer-group for IPv6 and enable <code>auto-link-bandwidth</code>. Assign both neighbors to this peer-group.
<p>
3. ATE Peers 1 & 2 advertise IPv6 prefix <code>P2</code>.
<p>
4. ATE Source sends a baseline rate of IPv6 traffic to <code>P2</code>.
   </td>
   <td style="background-color: null"><strong>Data Plane (Primary):</strong> Verify that ATE Peer 1 and ATE Peer 2 each receive approximately 50% of the sent traffic (+/- 5% tolerance).
<p>
<strong>Control Plane (Optional):</strong> Verify the <code>ext-community-index</code> for routes to <code>P2</code> from both peers points to a valid LBW community with a value for the full LAG bandwidth. Check AFT for equal <code>weight</code>.
   </td>
  </tr>
</table>



### **RT-7.51.2: Dynamic wECMP Re-balancing**



*   **Objective:** Verify that when one LAG's capacity is reduced, the DUT adjusts the traffic split proportionally for both IPv4 and IPv6.
*   **Procedure Details:** Both LAGs consist of 2 member ports of the same speed. Both ATE Peer 1 and Peer 2 advertise prefix `P1` (IPv4) and `P2` (IPv6).

<table>
  <tr>
   <td style="background-color: null">
<strong>Test Case</strong>
   </td>
   <td style="background-color: null"><strong>Procedure</strong>
   </td>
   <td style="background-color: null"><strong>Validation</strong>
   </td>
  </tr>
  <tr>
   <td style="background-color: null"><strong>7.51.2.1 - Unequal Balancing</strong>
   </td>
   <td style="background-color: null">1. From the state in TC 7.51.1.1, disable one member port on <code>LAG-1</code> (link to ATE Peer 1).
   </td>
   <td style="background-color: null"><strong>Data Plane (Primary):</strong> Verify the IPv4 traffic is re-balanced to a 1:2 ratio. ATE Peer 1 should receive ~33% and ATE Peer 2 should receive ~67% of the sent traffic (+/- 5% tolerance).
<p>
<strong>Control Plane (Optional):</strong> Verify the <code>ext-community-index</code> for the path via Peer 1 points to an LBW community with a value for the new runtime bandwidth (1x member link speed).
   </td>
  </tr>
  <tr>
   <td style="background-color: null"><strong>7.51.2.2 - Restore Balancing</strong>
   </td>
   <td style="background-color: null">1. Re-enable the member port on <code>LAG-1</code>.
   </td>
   <td style="background-color: null"><strong>Data Plane (Primary):</strong> Verify the IPv4 traffic split returns to an equal 50/50 balance (+/- 5% tolerance).
<p>
<strong>Control Plane (Optional):</strong> Verify the <code>ext-community-index</code> for the path via Peer 1 points to an LBW community with a value for the full LAG bandwidth.
   </td>
  </tr>
</table>


### **RT-7.51.3: Dynamic wECMP Re-balancing - Capacity Building from 1-Member LAG**


*   **Objective:** Verify wECMP re-balancing when starting from a 1-member LAG baseline and adding capacity.
*   **Procedure Details:** This test starts by configuring <code>LAG-1</code> and <code>LAG-2</code> with only one member port each.

<table>
  <tr>
   <td style="background-color: null">
<strong>Test Case</strong></li></ul>

   </td>
   <td style="background-color: null"><strong>Procedure</strong>
   </td>
   <td style="background-color: null"><strong>Validation</strong>
   </td>
  </tr>
  <tr>
   <td style="background-color: null"><strong>7.51.3.1 - Baseline 1:1 Balancing</strong>
   </td>
   <td style="background-color: null">1. Configure <code>LAG-1</code> with 1 member port (e.g., DUT Port 2) and <code>LAG-2</code> with 1 member port (e.g., DUT Port 4).
<p>
2. Establish BGP sessions (IPv4 & IPv6). Enable <code>auto-link-bandwidth</code>.
<p>
3. ATE Peers 1 & 2 advertise <code>P1</code> (IPv4) and <code>P2</code> (IPv6).
<p>
4. Send IPv4 and IPv6 traffic.
   </td>
   <td style="background-color: null"><strong>Data Plane (Primary):</strong> Verify both IPv4 and IPv6 traffic streams are balanced 50/50 (+/- 5% tolerance) between Peer 1 and Peer 2.
   </td>
  </tr>
  <tr>
   <td style="background-color: null"><strong>7.51.3.2 - Capacity Addition (1:1 -> 1:2)</strong>
   </td>
   <td style="background-color: null">1. From the state in 7.51.7.1, add a second member port to <code>LAG-2</code> (e.g., DUT Port 5).
   </td>
   <td style="background-color: null"><strong>Data Plane (Primary):</strong> Verify both IPv4 and IPv6 traffic streams re-balance to a 1:2 ratio, with ~33.3% to Peer 1 and ~66.7% to Peer 2 (+/- 5% tolerance).
<p>
<strong>Control Plane (Optional):</strong> Verify the LBW community for Peer 2's path is updated to 2x link speed, while Peer 1's remains 1x.
   </td>
  </tr>
  <tr>
   <td style="background-color: null"><strong>7.51.3.3 - Capacity Addition (1:2 -> 2:2)</strong>
   </td>
   <td style="background-color: null">1. From the state in 7.51.7.2, add a second member port to <code>LAG-1</code> (e.g., DUT Port 3).
   </td>
   <td style="background-color: null"><strong>Data Plane (Primary):</strong> Verify both IPv4 and IPv6 traffic streams re-balance to a 50/50 split (+/- 5% tolerance).
<p>
<strong>Control Plane (Optional):</strong> Verify the LBW community for Peer 1's path is updated to 2x link speed.
   </td>
  </tr>
</table>


### **RT-7.51.4: Hold-Down Timer Impact on Forwarding**



*   **Objective:** Verify the asymmetric hold-down timer logic (immediate update on link-down, delayed update on link-up) for both IPv4 and IPv6 traffic.
*   **Procedure Details:** Both LAGs consist of 2 member ports of the same speed. Both ATE Peer 1 and Peer 2 advertise prefix `P1` (IPv4) and `P2` (IPv6).

<table>
  <tr>
   <td style="background-color: null">
<strong>Test Case</strong>
   </td>
   <td style="background-color: null"><strong>Procedure</strong>
   </td>
   <td style="background-color: null"><strong>Validation</strong>
   </td>
  </tr>
  <tr>
   <td style="background-color: null"><strong>7.51.4.1 - Link Down (Immediate Update - IPv4)</strong>
   </td>
   <td style="background-color: null">1. From state TC 7.51.1.1, disable a member port on <code>LAG-1</code>.
   </td>
   <td style="background-color: null"><strong>Data Plane (Primary):</strong> Verify the traffic split shifts to the unbalanced 1:2 ratio (+/- 5% tolerance) <strong>immediately</strong> (without any hold-down delay).
   </td>
  </tr>
  <tr>
   <td style="background-color: null"><strong>7.51.4.2 - Link Up (Delayed Update - IPv4)</strong>
   </td>
   <td style="background-color: null">1. From state in TC 7.51.3.1, configure <code>hold-down-time: 30</code> on the DUT peer-group. 2. Re-enable the failed member port on <code>LAG-1</code>.
   </td>
   <td style="background-color: null"><strong>Data Plane (Primary):</strong> Verify the traffic split <strong>remains</strong> in the unbalanced 1:2 ratio (+/- 5% tolerance) for the full 30s. After the timer expires, verify the split returns to the balanced 50/50 state.
   </td>
  </tr>
  <tr>
   <td style="background-color: null"><strong>7.51.4.3 - Transient Flap (IPv4)</strong>
   </td>
   <td style="background-color: null">1. From state TC 7.51.1.1, configure a 30s <code>hold-down-time</code> on the peer-group. 2. Disable a member port on <code>LAG-1</code> for 5s, then re-enable it.
   </td>
   <td style="background-color: null"><strong>Data Plane (Primary):</strong>
<p>
1. Verify traffic shifts to 1:2 (+/- 5% tolerance) <strong>immediately</strong> when the link goes down.
<p>
2. Verify traffic <strong>remains</strong> at 1:2 (+/- 5% tolerance) for the full 30s after the link comes up.
<p>
3. Verify traffic returns to 50/50 after the timer expires.
   </td>
  </tr>
  <tr>
   <td style="background-color: null"><strong>7.51.4.4 - Link Down (Immediate Update - IPv6)</strong>
   </td>
   <td style="background-color: null">1. From state TC 7.51.1.2, disable a member port on <code>LAG-1</code>.
   </td>
   <td style="background-color: null"><strong>Data Plane (Primary):</strong> Verify the IPv6 traffic split shifts to the unbalanced 1:2 ratio (+/- 5% tolerance) <strong>immediately</strong> (without any hold-down delay).
   </td>
  </tr>
  <tr>
   <td style="background-color: null"><strong>7.51.4.5 - Link Up (Delayed Update - IPv6)</strong>
   </td>
   <td style="background-color: null">1. From state in TC 7.51.4.4, configure <code>hold-down-time: 30</code> on the DUT peer-group for IPv6.
<p>
2. Re-enable the failed member port on <code>LAG-1</code>.
   </td>
   <td style="background-color: null"><strong>Data Plane (Primary):</strong> Verify the IPv6 traffic split <strong>remains</strong> in the unbalanced 1:2 ratio (+/- 5% tolerance) for the full 30s. After the timer expires, verify the split returns to the balanced 50/50 state.
   </td>
  </tr>
  <tr>
   <td style="background-color: null"><strong>7.51.4.6 - Transient Flap (IPv6)</strong>
   </td>
   <td style="background-color: null">1. From state TC 7.51.1.2, configure a 30s <code>hold-down-time</code> on the peer-group for IPv6.
<p>
2. Disable a member port on <code>LAG-1</code> for 5s, then re-enable it.
   </td>
   <td style="background-color: null"><strong>Data Plane (Primary):</strong>
<p>
1. Verify IPv6 traffic shifts to 1:2 (+/- 5% tolerance) <strong>immediately</strong> when the link goes down.
<p>
2. Verify traffic <strong>remains</strong> at 1:2 (+/- 5% tolerance) for the full 30s after the link comes up. 3. Verify traffic returns to 50/50 (+/- 5% tolerance) after the timer expires.
   </td>
  </tr>
</table>



### **RT-7.51.5: Precedence of Peer-Advertised Community**



*   **Objective:** Verify that a link-bandwidth community advertised by an eBGP peer takes precedence over the locally auto-generated one for both IPv4 and IPv6.
*   **Procedure Details:** Both LAGs consist of 2 member ports of the same speed. Both ATE Peer 1 and Peer 2 advertise prefix `P1` (IPv4) and `P2` (IPv6).

<table>
  <tr>
   <td style="background-color: null">
<strong>Test Case</strong>
   </td>
   <td style="background-color: null"><strong>Procedure</strong>
   </td>
   <td style="background-color: null"><strong>Validation</strong>
   </td>
  </tr>
  <tr>
   <td style="background-color: null"><strong>7.51.5.1 - Peer-Advertised Precedence (IPv4)</strong>
   </td>
   <td style="background-color: null">1. Establish BGP sessions (IPv4 AF). On DUT, enable <code>auto-link-bandwidth</code> on the peer-group.
<p>
2. Configure ATE Peer 1 to advertise <code>P1</code> <strong>with a static LBW community</strong> (value equivalent to 1x link speed).
<p>
3. Configure ATE Peer 2 to advertise <code>P1</code> <strong>without</strong> any LBW community.
<p>
4. ATE Source sends IPv4 traffic to <code>P1</code>.
   </td>
   <td style="background-color: null"><strong>Data Plane (Primary):</strong> Verify IPv4 traffic is forwarded in a 1:2 ratio, with ~33% going to Peer 1 and ~67% going to Peer 2 (+/- 5% tolerance). This confirms the DUT used the lower, peer-advertised value for Peer 1's path and the higher, auto-generated value for Peer 2's path.
<p>
<strong>Control Plane (Optional):</strong> Verify the <code>ext-community-index</code> for the path via Peer 1 points to the static LBW from the ATE, while the path via Peer 2 points to the auto-generated LBW.
   </td>
  </tr>
  <tr>
   <td style="background-color: null"><strong>7.51.5.2 - Peer-Advertised Precedence (IPv6)</strong>
   </td>
   <td style="background-color: null">1. Establish BGP sessions (IPv6 AF). On DUT, enable <code>auto-link-bandwidth</code> on the peer-group.
<p>
2. Configure ATE Peer 1 to advertise <code>P2</code> <strong>with a static LBW community</strong> (value equivalent to 1x link speed).
<p>
3. Configure ATE Peer 2 to advertise <code>P2</code> <strong>without</strong> any LBW community.
<p>
4. ATE Source sends IPv6 traffic to <code>P2</code>.
   </td>
   <td style="background-color: null"><strong>Data Plane (Primary):</strong> Verify IPv6 traffic is forwarded in a 1:2 ratio, with ~33% going to Peer 1 and ~67% going to Peer 2 (+/- 5% tolerance).
<p>
<strong>Control Plane (Optional):</strong> Verify the <code>ext-community-index</code> for the IPv6 path via Peer 1 points to the static LBW from the ATE, while the path via Peer 2 points to the auto-generated LBW.
   </td>
  </tr>
</table>



### **RT-7.51.6: Configuration Precedence (Neighbor vs. Peer-Group)**



*   **Objective:** Verify that per-neighbor `auto-link-bandwidth` configuration overrides a disabled setting inherited from its peer-group for both IPv4 and IPv6.
*   **Procedure Details:** Both LAGs consist of 2 member ports of the same speed. Both ATE Peer 1 and Peer 2 advertise prefix `P1` (IPv4) and `P2` (IPv6).

<table>
  <tr>
   <td style="background-color: null">
<strong>Test Case</strong>
   </td>
   <td style="background-color: null"><strong>Procedure</strong>
   </td>
   <td style="background-color: null"><strong>Validation</strong>
   </td>
  </tr>
  <tr>
   <td style="background-color: null"><strong>7.51.6.1 - Neighbor Override of Disabled Peer-Group (IPv4)</strong>
   </td>
   <td style="background-color: null">1. From state TC 7.51.1.1, configure <code>enabled: false</code> under the <code>auto-link-bandwidth</code> hierarchy for the <strong>peer-group</strong>. 2. Configure <code>enabled: true</code> under the <code>auto-link-bandwidth</code> hierarchy for <strong>both IPv4 neighbors</strong> (on <code>LAG-1</code> and <code>LAG-2</code>).
   </td>
   <td style="background-color: null"><strong>Data Plane (Primary):</strong> Verify that IPv4 traffic is forwarded in a 50/50 ratio towards ATE Peer 1 and ATE Peer 2 (+/- 5% tolerance). This confirms wECMP is active because both neighbor configurations overrode the disabled peer-group setting. <strong>Control Plane (Optional):</strong> Verify that the routes to <code>P1</code> via <strong>both</strong> Peer 1 and Peer 2 <strong>do</strong> have a valid <code>ext-community-index</code>.
   </td>
  </tr>
  <tr>
   <td style="background-color: null"><strong>7.51.6.2 - Neighbor Override of Disabled Peer-Group (IPv6)</strong>
   </td>
   <td style="background-color: null">1. From state TC 7.51.1.2, configure <code>enabled: false</code> under the <code>auto-link-bandwidth</code> hierarchy for the <strong>peer-group</strong>. 2. Configure <code>enabled: true</code> under the <code>auto-link-bandwidth</code> hierarchy for <strong>both IPv6 neighbors</strong> (on <code>LAG-1</code> and <code>LAG-2</code>).
   </td>
   <td style="background-color: null"><strong>Data Plane (Primary):</strong> Verify that IPv6 traffic is forwarded in a 50/50 ratio towards ATE Peer 1 and ATE Peer 2 (+/- 5% tolerance). <strong>Control Plane (Optional):</strong> Verify that the routes to <code>P2</code> via <strong>both</strong> Peer 1 and Peer 2 <strong>do</strong> have a valid <code>ext-community-index</code>.
   </td>
  </tr>
</table>



### **RT-7.51.7: Transitive Flag Configuration (UNDER DEVELOPMENT)**



*   **Objective:** Verify that the DUT correctly generates a non-transitive community when `transitive: false` is explicitly configured for both IPv4 and IPv6.
*   **Procedure Details:** This is a control-plane focused test.

<table>
  <tr>
   <td style="background-color: null">
<strong>Test Case</strong>
   </td>
   <td style="background-color: null"><strong>Procedure</strong>
   </td>
   <td style="background-color: null"><strong>Validation</strong>
   </td>
  </tr>
  <tr>
   <td style="background-color: null"><strong>7.51.7.1 - Non-Transitive Behavior (IPv4)</strong>
   </td>
   <td style="background-color: null">1. From state TC 7.51.1.1, ensure <code>transitive: false</code> is configured on the peer-group.
<p>
2. ATE advertises prefix <code>P1</code>.
   </td>
   <td style="background-color: null"><strong>Control Plane (Optional):</strong> If DUT telemetry is enabled, query the <code>ext-community-index</code> for the route to <code>P1</code>. Verify the index points to an extended community that is correctly formatted as non-transitive.
   </td>
  </tr>
  <tr>
   <td style="background-color: null"><strong>7.51.7.2 - Non-Transitive Behavior (IPv6)</strong>
   </td>
   <td style="background-color: null">1. From state TC 7.51.1.2, ensure <code>transitive: false</code> is configured on the peer-group.
<p>
2. ATE advertises prefix <code>P2</code>.
   </td>
   <td style="background-color: null"><strong>Control Plane (Optional):</strong> If DUT telemetry is enabled, query the <code>ext-community-index</code> for the route to <code>P2</code>. Verify the index points to an extended community that is correctly formatted as non-transitive.
   </td>
  </tr>
</table>



## OpenConfig Path and RPC Coverage

```yaml

paths:

# configuration
/network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/auto-link-bandwidth/import/config/enabled:
/network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/auto-link-bandwidth/import/config/hold-down-time:
/network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/auto-link-bandwidth/import/config/transitive:
/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/auto-link-bandwidth/import/config/enabled:
/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/auto-link-bandwidth/import/config/hold-down-time:
/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/auto-link-bandwidth/import/config/transitive:

# telemetry
/interfaces/interface/subinterfaces/subinterface/state/counters/in-octets:
/network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/ext-community-index:
/network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/ext-community-index:
/network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/weight:
/interfaces/interface/state/oper-status:
/interfaces/interface/aggregation/state/member:
/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```
