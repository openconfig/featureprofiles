# PF-1.22 ECMP hashing for GUE flows with IPv4|UDP outer header on decapsulation node
## Summary
This test ensures that only the outer header (IPv4 | UDP) of GUEv1 encapsulated packets are used for hashing on decapsulating nodes. 
Comprehensive static GUEv1 Decapsulation to IPv4 or IPv6 payload test is documented in [#4178](https://github.com/openconfig/featureprofiles/pull/4178).

## Topology
```mermaid
graph LR; 
subgraph DUT [DUT]
    B1[Port1]
    B2[Port2]
    B3[Port3]
    B4[Port4]
    B5[Port5]
    B6[Port6]
    B7[Port7]
end
subgraph ATE2 [ATE2]
    N1[Port1]
end
subgraph ATE3 [ATE3]
    N21[Port2]
    N22[Port3]
end
subgraph ATE4 [ATE4]
    N31[Port4]
    N32[Port5]
end
subgraph ATE5 [ATE5]
    N4[Port6]
end
A1[ATE1:Port1] <--EBGP--> B1; 
B2 <--IBGP--> N1; 
B3 <-- [LAG1] IBGP --> N21;
B4 <-- [LAG1] IBGP --> N22;
B5 <-- [LAG2] EBGP --> N31; 
B6 <-- [LAG2] EBGP --> N32;
B7 <-- EBGP --> N4; 
```

## Configuration generation of DUT and ATE

### Baseline DUT configuration
1. The DUT's loopback interface must be passive for IS-IS.
2. Configure IPv4 and IPv6 EBGP[ASN200:ASN100] between ATE:Port1 <> DUT:Port1 
3. Configure IPv4 and IPv6 IBGP[ASN100] between DUT <> ATE2
4. Configure IPv4 and IPv6 IBGP[ASN100] between DUT <> ATE3
5. Configure IPv4 and IPv6 EBGP[ASN100:ASN200] between DUT <> ATE4
6. Configure IPv4 and IPv6 EBGP[ASN100:ASN200] between DUT <> ATE5
7. Configure DUT as decapsulation node for IP|UDP (GUE v1)using "DUT-DECAP-Address" and decap UDP port as 6080
8. Enable BGP multipath for both EBGP and IBGP sessions to enable load balancing traffic across multiple paths/links
9. Enable BGP multihop for BGP(IBGP/EBGP) sessions on LAG interfaces 
10. DUT has multiple paths to Host2 via multiple nodes, ATE2 & ATE3
11. DUT can reach Host3 via a bundle interface towards ATE3
12. DUT has multiple paths to Host4 via multiple nodes, ATE4 & ATE5
13. Host1(v4/v6) route is installed and active via ATE1
14. Host2(v4/v6) route is installed and active via ATE2 and ATE3, therefore the traffic for Host2 should be load-balanced across both the nodes
15. Host3v4 route is installed and active via ATE3, therefore the traffic for Host3 should be load-balanced across the bundle members
16. Host4(v4/v6) route is installed and active via ATE4 and ATE5, therefore the traffic for Host4 should be load-balanced across both the nodes

### Baseline ATE configuration
1. The ATE's loopback interface must be passive for IS-IS
2. Establish BGP sessions as shown in the DUT configuration section
3. ATE1 hosts Host1v4 and Host1v6
4. ATE2 and ATE3 hosts Host2v4 and Host2v6 
5. ATE3 hosts Host3v4
    - Reachability to Host3v4 from ATE3 is via a static MPLS label
6. ATE4 and ATE5 hosts Host4v4 and Host4v6

#### ATE Route Advertisements
| **Source ATE Nodes** | **Advertisement Type** | **Prefixes**  | **Via BGP Sessions**                   | **Host Mapping** |
| -------------- | ---------------------- | ------------------  | ---------------------------------------| ---------------- |
| ATE1           | EBGP                   | IPv4prefix1-10/24   | IPv4 DUT <--> ATE1                     | Host1v4         |
| ATE1           | EBGP                   | IPv6prefix1-10/24   | IPv6 DUT <--> ATE1                     | Host1v6         |
| ATE1           | EBGP                   | Loopback[1-10]v4/32 | IPv4 DUT <--> ATE1                     | ATE1LO[1-10]v4   |
| ATE1           | EBGP                   | Loopback[1-10]v6/32 | IPv6 DUT <--> ATE1                     | ATE1LO[1-10]v6   |
| ATE2 and ATE3  | IBGP                   | IPv4prefix11-20/24  | IPv4 DUT <--> ATE2 and DUT <--> ATE3   | Host2v4         |
| ATE2 and ATE3  | IBGP                   | IPv6prefix11-20/24  | IPv6 DUT <--> ATE2 and DUT <--> ATE3   | Host2v6         |
| ATE3           | IBGP                   | IPv4prefix21-30/24  | IPv4 DUT <--> ATE2 and DUT <--> ATE3   | Host3v4         |
| ATE4 and ATE5  | EBGP                   | IPv4prefix31-40/24  | IPv4 DUT <--> ATE4 and DUT <--> ATE5   | Host4v4         |
| ATE4 and ATE5  | EBGP                   | IPv6prefix31-40/24  | IPv6 DUT <--> ATE4 and DUT <--> ATE5   | Host4v6         |


**_To simplify this document, Host1, Host2, and Host3 will be referred to as H1, H2, H3 and H4 respectively._**

### Packet types
### IPv4 and IPv6 Packet Constructs Detailed Table

| Packet#  | Layer       | Protocol          | Source Address      | Destination Address | Source Port         | Destination Port          | MPLS Label  |  Notes                                      |
| :------- | :---------- | :---------------- | :------------------ | :------------------ | :------------------ | :------------------------ | :-----------| :------------------------------------------ |
| **1** | **Overall** | **Payload o IPv4\|TCP o MPLS o IPv4\|UDP o IPv4\|UDP(GUE v1)** |                     |                     |                     |                           |                   |
|          | Inner       | IPv4\|TCP         | H1v4 address        | H3v4 address        | 14                  | 15                        |             |Src Port: Any unassigned TCP port; Dst Port: Any App/unassigned TCP port |
|          | MPLS        | MPLS              | N/A                 | N/A                 | N/A                 | N/A                       | Static label for ATE3 to reach H3v4    | *Note: Inner Dst is H3v4* |
|          | Middle      | IPv4\|UDP         | ATE1LO1v4 IPv4 addr | ATE3-port IPv4 addr | 5995 (randomizable) | 6080                      |             | Src Port: Any unreserved UDP port |
|          | Outer       | IPv4\|UDP(GUE v1) | ATE1-port IPv4 addr | DUT-DECAP-Address   | 5996 (randomizable) | 6080                      |             | Src Port: Any unassigned UDP port; GUE v1 encapsulation |
| **2** | **Overall** | **Payload o IPv4\|UDP o IPv4\|UDP(GUE v1)** |                     |                     |                     |                           |                                                         |
|          | Inner       | IPv4\|UDP         | H1v4 address        | H2v4 address        | 14 (randomizable)   | 15                        |              | Src Port: Any unassigned UDP port; Dst Port: Any App/unassigned UDP port |
|          | Outer       | IPv4\|UDP(GUE v1) | ATE1-port IPv4 addr | DUT-DECAP-Address   | 5996 (randomizable) | 6080                      |              | Src Port: Any unassigned UDP port; GUE v1 encapsulation    |
| **3** | **Overall** | **Payload o IPv4\|TCP o IPv4\|UDP(GUE v1)** |                     |                     |                     |                           |                                                         |
|          | Inner       | IPv4\|TCP         | H1v4 address        | H2v4 address        | 14 (randomizable)   | 15                        |              | Src Port: Any unassigned TCP port; Dst Port: Any App/unassigned TCP port |
|          | Outer       | IPv4\|UDP(GUE v1) | ATE1-port IPv4 addr | DUT-DECAP-Address   | 5996 (randomizable) | 6080                      |              | Src Port: Any unassigned UDP port; GUE v1 encapsulation    |
| **4** | **Overall** | **Payload o IPv4\|UDP o IPv4\|UDP(GUE v1)** |                     |                     |                     |                           |                                                         |
|          | Inner       | IPv4\|UDP         | H1v4 address        | H4v4 address        | 14 (randomizable)   | 15                        |              | Src Port: Any unassigned UDP port; Dst Port: Any App/unassigned UDP port|
|          | Outer       | IPv4\|UDP(GUE v1) | ATE1-port IPv4 addr | DUT-DECAP-Address   | 5996 (randomizable) | 6080                      |              | Src Port: Any unassigned UDP port; GUE v1 encapsulation    |
| **5** | **Overall** | **Payload o IPv4\|TCP o IPv4\|UDP(GUE v1)** |                     |                     |                     |                           |                                                         |
|          | Inner       | IPv4\|TCP         | H1v4 address        | H4v4 address        | 14 (randomizable)   | 15                        |              | Src Port: Any unassigned TCP port; Dst Port: Any App/unassigned TCP port |
|          | Outer       | IPv4\|UDP(GUE v1) | ATE1-port IPv4 addr | DUT-DECAP-Address   | 5996 (randomizable) | 6080                      |              | Src Port: Any unassigned UDP port; GUE v1 encapsulation    |
| **6** | **Overall** | **Payload o IPv6\|TCP o MPLS o IPv4\|UDP o IPv4\|UDP(GUE v1)** |                     |                     |                     |                           |                                                             |
|          | Inner       | IPv6\|TCP         | H1v6 address        | H3v6 address        | 14                  | 15                        |             | Src Port: Any unassigned TCP; Dst Port: Any App/unassigned TCP |
|          | MPLS        | MPLS              | N/A                 | N/A                 | N/A                 | N/A                       | Static label for ATE3 to reach H3v4 | *Note: Inner Dst is H3v6* |
|          | Middle      | IPv4\|UDP         | ATE1LO1v4 IPv4 addr | ATE3-port IPv4 addr | 5995 (randomizable) | 6080                      |              | Src Port: Any unassigned UDP port | 
|          | Outer       | IPv4\|UDP(GUE v1) | ATE1-port IPv4 addr | DUT-DECAP-Address   | 5996 (randomizable) | 6080                      |              | Src Port: Any unassigned UDP port; GUE v1 encapsulation     |
| **7** | **Overall** | **Payload o IPv6\|UDP o IPv4\|UDP(GUE v1)** |                     |                     |                     |                   |                      |                                                             |
|          | Inner       | IPv6\|UDP         | H1v6 address        | H2v6 address        | 5995 (randomizable) | 5994 (randomizable)       |              | Src/Dst Ports: Any unassigned UDP port  |
|          | Outer       | IPv4\|UDP(GUE v1) | ATE1-port IPv4 addr | DUT-DECAP-Address   | 5996 (randomizable) | 6080                      |              | Src Port: Any unassigned UDP port; GUE v1 encapsulation  |
| **8** | **Overall** | **Payload o IPv6\|TCP o IPv4\|UDP(GUE v1)** |                     |                     |                     |                   |                      |                                                             |
|          | Inner       | IPv6\|TCP         | H1v6 address        | H2v6 address        | 14 (randomizable)   | 15                        |              | Src Port: Any unassigned TCP port; Dst Port: Any App/unassigned TCP port |
|          | Outer       | IPv4\|UDP(GUE v1) | ATE1-port IPv4 addr | DUT-DECAP-Address   | 5996 (randomizable) | 6080                      |              | Src Port: Any unassigned UDP port; GUE v1 encapsulation |
| **9** | **Overall** | **Payload o IPv6\|UDP o IPv4\|UDP(GUE v1)** |                     |                     |                     |                   |                      |                                                             |
|          | Inner       | IPv6\|UDP         | H1v6 address        | H4v6 address        | 5995 (randomizable) | 5994 (randomizable)       |              | Src/Dst Ports: Any unreserved UDP port  |
|          | Outer       | IPv4\|UDP(GUE v1) | ATE1-port IPv4 addr | DUT-DECAP-Address   | 5996 (randomizable) | 6080                      |              | Src Port: Any unreserved UDP port; GUE v1 encapsulation |
| **10**| **Overall** | **Payload o IPv6\|TCP o IPv4\|UDP(GUE v1)**  |                     |                            |       |                     |                           |                                                             |
|          | Inner       | IPv6\|TCP         | H1v6 address        | H4v6 address        | 14 (randomizable)   | 15                        |              | Src Port: Any unassigned TCP port; Dst Port: Any App/unassigned TCP port |
|          | Outer       | IPv4\|UDP(GUE v1) | ATE1-port IPv4 addr | DUT-DECAP-Address   | 5996 (randomizable) | 6080                      |              | Src Port: Any unreserved UDP port; GUE v1 encapsulation |


### Flow types

| **Flow-type**  | **Description**        | **Packet#**         | 
| -------------- | ---------------------- | ------------------  |
| 1              | H1 --> H3 with IPv4    | Packet#1            | 
| 2              | H1 --> H2 with IPv4    | Packet#2            | 
| 3              | H1 --> H2 with IPv4    | Packet#3            | 
| 4              | H1 --> H4 with IPv4    | Packet#4            | 
| 5              | H1 --> H4 with IPv4    | Packet#5            | 
| 6              | H1 --> H3 with IPv6    | Packet#6            | 
| 7              | H1 --> H2 with IPv6    | Packet#7            | 
| 8              | H1 --> H2 with IPv6    | Packet#8            | 
| 9              | H1 --> H4 with IPv6    | Packet#9            | 
| 10             | H1 --> H4 with IPv6    | Packet#10           |

## Procedure

- Traffic towards a destination is spread evenly across nodes and LAGs (if applicable):
    - Tolerance for delta: 5%
- Start the Ixia traffic as specified for test
    - Sent 1000000 packets at the 10% of the line rate.
    - Packets are generated based on different header field entropy which can be defined by the test case
- Repeat each test with the each ATE Flow-type or explicitly mentioned flow-type
- Conduct each of the following test, using a single flow-type with 1024 flows

### PF-1.22.1: [Baseline] GUE decap and Load-balance test
- Configure the DUT and ATE as stated above
- Initiate a single flow-type and follow the below stated and applicable verification steps
- L4 source port of outer header(GUEv1 encap header) should be randomized for each flow-type that's running
- Repeat the test for all flow-types
- Validations:
-  The outer header destination IP of the traffic is the DUT-DECAP-Address and the destination port of the traffic (UDP 6080) matches the configured UDP decap port criteria
-  Therefore, DUT will decapsulate the outer header and perform a lookup based on the inner IP address
-  The following validations are applicable as per the flow-type that is being tested
    - Flow#1 for H3 should be load-balanced across the bundle members via ATE3
    - Flow#2 for H2 should be load-balanced via ATE2 and ATE3
        - Traffic via ATE3 should be load-balanced across the bundle members 
    - Flow#3 for H2 should be load-balanced via ATE2 and ATE3
        - Traffic via ATE3 should be load-balanced across the bundle members
    - Flow#4 for H4 should be load-balanced via ATE4 and ATE5
        - Traffic forwarded towards ATE4 (via LAG2) should be load-balanced across the LAG members
    - Flow#5 for H4 should be load-balanced via ATE4 and ATE5
        - Traffic forwarded towards ATE4 (via LAG2) should be load-balanced across the LAG members
    - Flow#6 for H3 should be load-balanced across the bundle members via ATE3
    - Flow#7 for H2 should be load-balanced via ATE2 and ATE3
        - Traffic via ATE3 should be load-balanced across the bundle members 
    - Flow#8 for H2 should be load-balanced via ATE2 and ATE3
        - Traffic via ATE3 should be load-balanced across the bundle members
    - Flow#9 for H4 should be load-balanced via ATE4 and ATE5
        - Traffic forwarded towards ATE4 (via LAG2) should be load-balanced across the LAG members
    - Flow#10 for H4 should be load-balanced via ATE4 and ATE5
        - Traffic forwarded towards ATE4 (via LAG2) should be load-balanced across the LAG members
    - No packet loss should be observed

### PF-1.22.2: Randomize the L4 source port field in immediate next header to outer header and verify load-balance behavior
- Configure the DUT and ATE as stated above
- L4 source port of inner/middle header(immediate next header to outer header) should be randomized for each flow-type thats running
- Initiate a flow-type and follow the below stated and applicable verification steps
- Repeat the test for all flow types with above-stated modified field
- Validations:
    - Flow validations are same as captured in sub-test#PF-1.22.1
    - Traffic distribution should remain consistent with baseline test results, i.e. no deviation seen because of the modified field 

### PF-1.22.3: Randomize the L4 source port field and SIP in immediate next header to outer header (middle header) and verify load-balance behavior [Applicable to Packet#1 and Packet#6]
- Configure the DUT and ATE as stated above
- L4 source port of middle header(immediate next header to outer header) should be randomized for each flow-type that's running
- SIP(immediate next header to outer header) should be randomized between ATE1LO[1-10] addresses for each flow-type that's running
- Execute the test for flow-type1 and flow-type6 with above-stated modified fields
- Initiate a single flow-type and follow the below stated and applicable verification steps
- Validations:
    -  Flow#1 for H3 should be load-balanced across the bundle members via ATE3
    -  Flow#6 for H3 should be load-balanced across the bundle members via ATE3
    -  No packet loss should be observed
    -  Traffic distribution should remain consistent with baseline test results, i.e. no deviation seen because of the modified field

### PF-1.22.4: Randomize the L4 source port field and SIP in immediate next header to outer header (inner header) and verify load-balance behavior [NA to Packet#1 and Packet#6]
- Configure the DUT and ATE as stated above
- L4 source port of inner header should be randomized for each flow-type that's running
- SIP of inner header should be randomized between the v4/v6 prefixes advertised by H1 for each flow-type that's running
- Initiate a flow-type and follow the below stated and applicable verification steps
- Execute the test for flow-type2, flow-type3, flow-type4, flow-type5, flow-type7, flow-type8, flow-type9 and flow-type10 with above-stated modified fields
- Validations:
    -  Flow#2 for H2 should be load-balanced via ATE2 and ATE3
        -  Traffic via ATE3 should be load-balanced across the bundle members 
    -  Flow#3 for H2 should be load-balanced via ATE2 and ATE3
        -  Traffic via ATE3 should be load-balanced across the bundle members
    -  Flow#4 for H4 should be load-balanced via ATE4 and ATE5
        -  Traffic via ATE4 should be load-balanced across the bundle members
    -  Flow#5 for H4 should be load-balanced via ATE4 and ATE5
        -  Traffic via ATE4 should be load-balanced across the bundle members
    -  Flow#7 for H2 should be load-balanced via ATE2 and ATE3
        -  Traffic via ATE3 should be load-balanced across the bundle members 
    -  Flow#8 for H2 should be load-balanced via ATE2 and ATE3
        -  Traffic via ATE3 should be load-balanced across the bundle members
    -  Flow#9 for H4 should be load-balanced via ATE4 and ATE5
        -  Traffic via ATE4 should be load-balanced across the bundle members
    -  Flow#10 for H4 should be load-balanced via ATE4 and ATE5
        -  Traffic via ATE4 should be load-balanced across the bundle members
    -  No packet loss should be observed
    -  Traffic distribution should remain consistent with baseline test results, i.e. no deviation seen because of the modified field



## Canonical OpenConfig for GUEv1 Decapsulation configuration
TODO: decap policy to be updated by https://github.com/openconfig/public/pull/1288

```json
{
    "network-instances": {
        "network-instance": {
            "config": {
                "name": "DEFAULT"
            },
            "name": "DEFAULT",
            "policy-forwarding": {
                "policies": {
                    "policy": [
                        {
                            "config": {
                                "policy-id": "decap-policy"
                            },
                            "rules": {
                                "rule": [
                                    {
                                        "sequence-id": 1,
                                        "config": {
                                            "sequence-id": 1
                                        },
                                        "ipv4": {
                                            "config": {
                                                "destination-address-prefix-set": "dst_prefix",
                                                "protocol": "IP_UDP"
                                            }
                                        },
                                        "transport": {
                                            "config": {
                                                "destination-port": 6080
                                            }
                                        }
                                        "action": {
                                            "decapsulate-gue": true
                                        },
                                    },
                                ]
                            }
                        }
                    ]
                }
            }
        }
    }
}
```
## OpenConfig Path and RPC Coverage
```yaml
paths:

/network-instances/network-instance/policy-forwarding/policies/policy/config/policy-id:
/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/config/ipv4/config/destination-address-prefix-set:
/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/config/ipv4/config/protocol:
/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/transport/config/destination-port:
/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/decapsulate-gue:

# telemetry
openconfig-interfaces/interfaces/interface/state/counters/out-pkts:
openconfig-interfaces/interfaces/interface/state/counters/out-unicast-pkts:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
```
## Required DUT platform
* Specify the minimum DUT-type:
  * FFF - fixed form factor
