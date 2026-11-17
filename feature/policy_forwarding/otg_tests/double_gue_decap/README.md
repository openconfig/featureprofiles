# PF-1.26: Double GUEv1 Decapsulation for Overlay Probing

## Objective

This test validates the router's (DUT) capability to perform double-decapsulation using a soft-loopback mechanism in the DUT. It ensures that overlay probe traffic, dual-encapsulated with GUE headers, can be correctly decapsulated in two stages:

* Stage 1: Decapsulation of the outermost GUE header (H1) and redirection to a loopback interface via LPM lookup.
* Stage 2: Recirculation through the soft-loop and decapsulation of the middle GUE header (H2), exposing the inner payload (H3) for final forwarding.

The test specifically verifies this behavior for both IPv4 and IPv6 middle headers (H2) `[Outer_IP_header][UDP][Middle_IP_header][UDP][Inner_IP_header][PAYLOAD]`

## Topology

* ATE Port 1: Connected to DUT Port 1 (Ingress). Senders of double-encapsulated probe traffic.
* ATE Port 2: Connected to DUT Port 2 (Egress). Receiver of final decapsulated traffic.
* DUT Port 3: Configured in soft-loop mode. This port serves as the recirculation point for the double-decap process.

 ```mermaid
graph LR;
A[ATE:Port1] --Ingress (Double Encap)--> B[DUT];
B --Egress (Decapped)--> C[ATE:Port2];
```

## Procedure

### DUT Configuration
* Decap Aggregates: Configure DUT:Port3 with an aggregate IPv4 /28 and IPv6 /60 block address.
* Soft-Loop: Enable software loopback on DUT Port 3.
* Forwarding Rules:
 - Configure dual decap policies on the DUT to facilitate the decapsulation of Header 1 and Header 2 (both UDP 6080). The decap policies for Header 2 (IPv4 or IPv6 | UDP) and Header 1 (IPv6|UDP) must target specified aggregates: for Header 2, utilize the IPv4 /28 and IPv6 /60 blocks assigned to DUT Port 3; for Header 1, use a /60 subnet of the destination address.
- The destination IP for Header 2 must resolve to the IP address assigned to the soft-loop interface (DUT Port 3) post decapsulation of Header 1 to facilitate recirculation.
- Configure the DUT to decapsulate Header 2 upon re-entry from the soft-loop on DUT Port 3 and then do a LPM lookup on Header 3 that resolves to ATE:Port2 address.

### ATE Configuration

| Flow Type | Header Layer | Source IP | Destination IP | UDP Port | DSCP | TTL |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **Flow Type #1 (IPv6/IPv4/IPv4)** | Outer IP | ATE-P1-IP | DECAP-DST-OUTER | 6080 | 35 | 70 |
| | Middle IP | IPV4-MID-SRC | DECAP-DST-INNER | 6080 | 32 | 60 |
| | Inner IP | IPV4-SRC-HOST | IPV4-DST-HOST | N/A | 20 | 50 |
| **Flow Type #2 (IPv6/IPv6/IPv6)** | Outer IP | ATE-P1-IP | DECAP-DST-OUTER | 6080 | 35 | 70 |
| | Middle IP | IPV6-MID-SRC | DECAP-DST-INNER | 6080 | 32 | 60 |
| | Inner IP | IPV6-SRC-HOST | IPV6-DST-HOST | N/A | 20 | 50 |
| **Flow Type #3 (IPv6/IPv4/IPv4)** | Outer IP | ATE-P1-IP | DECAP-DST-OUTER | 6080 | 35 | 70 |
| | Middle IP | IPV4-MID-SRC | DECAP-DST-INNER | 6085 | 32 | 60 |
| | Inner IP | IPV4-SRC-HOST | IPV4-DST-HOST | N/A | 20 | 50 |

### Traffic Generation:

* Flow #1 (IPv4 H2): ATE Port 1 sends a packet with [H1: GUE][H2: IPv4 GUE][H3: Payload].
** H1 destination = IPv6 address from the configured decap aggregate for Header 1.
** H2 destination = DUT Port 3 IP (IPv4).
* Flow #2 (IPv6 H2): ATE Port 1 sends a packet with [H1: GUE][H2: IPv6 GUE][H3: Payload].
** H1 destination = IPv6 address from the configured decap aggregate for Header 1.
** H2 destination = DUT Port 3 IP (IPv6).

#### PF-1.26.1: Double GUE Decapsulation of IPv4 Traffic
* Configure Flow Type #1 with an IPv4 inner header.
* Initiate traffic.
* **Verification**:
    * DUT decapsulates the outer header.
    * DUT decapsulates the middle header.
    * DUT performs LPM on the inner destination `IPV4-DST-HOST`.
    * ATE Port 2 receives the packet as `[Inner_IP][Payload]`.
    * Verify that the inner TTL is decremented by 1 (or as per forwarding logic) and inner DSCP is preserved.
    * No packet loss observed.

#### PF-1.26.2: Double GUE Decapsulation with IPv6 Inner Payload
* Configure Flow Type #2 with an IPv6 inner header.
* Initiate traffic.
* **Verification**:
    * DUT decapsulates the outer header.
    * DUT decapsulates the middle header.
    * DUT performs LPM on the inner destination `IPV6-DST-HOST`.
    * ATE Port 2 receives the packet as `[Inner_IP][Payload]`.
    * Verify that the inner TTL is decremented by 1 (or as per forwarding logic) and inner DSCP is preserved.
    * No packet loss observed.

#### PF-1.26.3: Negative - Middle Header UDP Port Mismatch
* Configure Flow Type #3 with an IPv4 inner header.
* Initiate traffic where the outer header matches but the middle header has an unconfigured UDP port (e.g., 6085).
* **Verification**:
    * DUT may decapsulate the outer header but should drop the packet (or forward as-is if no other rules match) after failing to match the middle decap criteria.
    * 100% packet loss (or failed decap) on ATE Port 2.
 
#### PF-1.26.4: Negative - Middle Header no IPv4 destination available
* Configure Flow Type #1 with an IPv4 inner header that has an unreachable destination IP.
* Initiate traffic.
* **Verification**:
    * DUT decapsulates the outer header.
    * DUT decapsulates the middle header.
    * DUT performs LPM on the inner destination `IPV4-DST-HOST` and drops the packet due to no route.
    * 100% packet loss on ATE Port 2.

#### PF-1.26.5: Negative - Middle Header no IPv6 destination available
* Configure Flow Type #2 with an IPv6 inner header that has an unreachable destination IP.
* Initiate traffic.
* **Verification**:
    * DUT decapsulates the outer header.
    * DUT decapsulates the middle header.
    * DUT performs LPM on the inner destination `IPV6-DST-HOST` and drops the packet due to no route.
    * 100% packet loss on ATE Port 2.

## Canonical OC

*Note: As of this FNT draft, the OpenConfig model for sequential decapsulation may require applying policies across multiple network instances or sequence IDs.*

```json
{
  "network-instances": {
    "network-instance": [
      {
        "name": "DEFAULT",
        "policy-forwarding": {
          "policies": {
            "policy": [
              {
                "policy-id": "outer-decap-policy",
                "rules": {
                  "rule": [
                    {
                      "sequence-id": 1,
                      "ipv6": {
                        "config": {
                          "protocol": "IP_UDP",
                          "destination-address": "2001:db8::1/128"
                        }
                      },
                      "transport": {
                        "config": {
                          "destination-port": 6080
                        }
                      },
                      "action": {
                        "config": {
                          "decapsulate-gue": true,
                          "network-instance": "DECAP_VRF"
                        }
                      }
                    }
                  ]
                }
              }
            ]
          }
        }
      },
      {
        "name": "DECAP_VRF",
        "policy-forwarding": {
          "policies": {
            "policy": [
              {
                "policy-id": "inner-decap-policy",
                "rules": {
                  "rule": [
                    {
                      "sequence-id": 1,
                      "ipv4": {
                        "config": {
                          "protocol": "IP_UDP",
                          "destination-address": "192.0.2.2/32"
                        }
                      },
                      "transport": {
                        "config": {
                          "destination-port": 6080
                        }
                      },
                      "action": {
                        "config": {
                          "decapsulate-gue": true,
                          "network-instance": "DEFAULT"
                        }
                      }
                    }
                  ]
                }
              }
            ]
          }
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
/network-instances/network-instance/policy-forwarding/config/global-decap-policy:
/network-instances/network-instance/policy-forwarding/policies/policy/config:
/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/destination-address:
/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decapsulate-gue:
/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/transport/config/destination-port:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT Platform

* FFF 
* MFF


