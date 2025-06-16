# PF-1.9 Egress handling of TTL

## Summary

This test verifies TTL handling for egress flows.

## Testbed Type

*  [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

### Test environment setup

*   DUT has one ingress port and one egress port, both connected to ATE.

    ```
                              -------
                             |       |
          [ ATE:Port1 ] ---- |  DUT  | ---- [ ATE:Port2 ]
                             |       |
                              -------
    ```

*   Routes are advertised from ATE:Port2.
*   Traffic is generated from ATE:Port1.
*   ATE:Port2 is used as the destination port for flows.
*   IPv4-DST-NET is 10.1.1.1
*   IPv6-DST-NET is fc00:10:1:1::1
*   IPv4-DST-DECAP is 10.2.2.2
*   ATE:Port1 interface. IPv4 is 192.168.10.1/30, IPv6 is 2001:DB8::192:168:10:1/126
*   DUT:Port1 interface. IPv4 is 192.168.10.2/30, IPv6 is 2001:DB8::192:168:10:2/126
*   ATE:Port2 interface. IPv4 is 192.168.20.1/30, IPv6 is 2001:DB8::192:168:20:1/126
*   DUT:Port2 interface. IPv4 is 192.168.20.2/30, IPv6 is 2001:DB8::192:168:20:2/126
*   Frame size for packets generated from ATE:Port1 is 128 bytes

#### Configuration

1.  DUT:Port1 is configured as Singleton IP interface towards ATE:Port1.

2.  DUT:Port2 is configured as Singleton IP interface towards ATE:Port2.

3.  DUT is configured with the following static routes:
    *   Destination IPv4-DST-NET/32 next hop ATE:Port2 IPv4 address.
    *   Destination IPv6-DST-NET/128 next hop ATE:Port2 IPv6 address.

4.  DUT is configured to decapsulate packets destined to IPv4-DST-DECAP/32

5.  DUT is configured with static LSP with label 100010 pointing to ATE:Port2
    IPv4 address. This should be used for encapsulated packets with inner IPv4.

5.  DUT is configured with static LSP with label 100020 pointing to ATE:Port2 
    IPv6 address. This should be used for encapsulated packets with inner IPv6.


### PF-1.9.1: IPv4 non-encapsulated traffic with TTL = 10.

ATE action:

*   Generate 5 **IPv4 packets** from ATE:Port1 to IPv4-DST-NET/32.
    *   Set TTL of all packets to *10*.

Verify:

*   DUT interface DUT:Port1 `in-unicast-pkts` counters equals the number of
    packets generated from ATE:Port1.
*   DUT interface DUT:Port2 `out-unicast-pkts` counters equals the number of
    packets generated from ATE:Port1.
*   The packet count of traffic received on ATE:Port2 should be equal to the
    packets generated from ATE:Port1.
*   TTL for all packets received on ATE:Port2 should be *9*.

### PF-1.9.2: IPv6 non-encapsulated traffic with TTL = 10.

*   Repeat `PF-1.9.1` with ATE generating IPv6 packets IPv6-DST-NET/128.

### PF-1.9.3: IPv4 non-encapsulated traffic with TTL = 1.

ATE action:

*   Generate 5 **IPv4 packets** from ATE:Port1 to IPv4-DST-NET/32.
    *   Set TTL of all packets to *1*.

Verify:

*   DUT interface DUT:Port1 `in-unicast-pkts` counters equals the number of
    packets generated from ATE:Port1.
*   ATE:Port1 received ICMP TTL Exceeded packets for all packets sent.

### PF-1.9.4: IPv6 non-encapsulated traffic with TTL = 1.

*   Repeat `PF-1.9.3` with ATE generating IPv6 packets IPv6-DST-NET/128.

### PF-1.9.5: IPv4oGRE traffic with inner TTL = 10 and outer TTL = 30.

ATE action:

*   Generate 5 **IPv4oGRE packets** from ATE:Port1 with below headers.
    *   Set the inner IP header destination to IPv4-DST-NET/32.
    *   Set the inner IP header TTL to *10*.
    *   Set the outer IP header destination to IPv4-DST-DECAP/32.
    *   Set the outer IP header TTL to *30*.

Verify:

*   Perform same verifications in `PF-1.9.1`.
*   Verify that decapsulation rule counters match the number of
    packets from ATE:Port1.

### PF-1.9.6: IPv6oGRE traffic with inner TTL = 10 and outer TTL = 30.

*   Repeat `PF-1.9.5` using IPv6oGRE with inner header destination IP of
    IPv6-DST-NET/128.

### PF-1.9.7: IPv4oGRE traffic with inner TTL = 1 and outer TTL = 30.

ATE action:

*   Generate 5 **IPv4oGRE packets** from ATE:Port1 with below header settings.
    *   Set the inner IP header destination to IPv4-DST-NET/32.
    *   Set the inner IP header TTL to *1*.
    *   Set the outer IP header destination to IPv4-DST-DECAP/32.
    *   Set the outer IP header TTL to *30*.

Verify:

*   DUT interface DUT:Port1 `in-unicast-pkts` counters equals the number of
    packets generated from ATE:Port1.
*   ATE:Port1 received ICMP TTL Exceeded packets for all packets sent.

### PF-1.9.8: IPv6oGRE traffic with inner TTL = 1 and outer TTL = 30.

*   Repeat `PF-1.9.7` using IPv6oGRE with inner header destination IP of
    IPv6-DST-NET/128.

### PF-1.9.9: IPv4oGRE traffic with inner TTL = 10 and outer TTL = 1.

ATE action:

*   Generate 5 **IPv4oGRE packets** from ATE:Port1 with below header settings.
    *   Set the inner IP header destination to IPv4-DST-NET/32.
    *   Set the inner IP header TTL to *10*.
    *   Set the outer IP header destination to IPv4-DST-DECAP/32.
    *   Set the outer IP header TTL to *1*.

Verify:

*   Perform same verifications in `PF-1.9.1`.
*   Verify that decapsulation rules counter match number of
    packets from ATE:Port1.

### PF-1.9.10: IPv6oGRE traffic with inner TTL = 10 and outer TTL = 1.

*   Repeat `PF-1.9.9` using IPv6oGRE with inner header destination IP of
    IPv6-DST-NET/128.

### PF-1.9.11: IPv4oMPLSoGRE traffic with inner TTL = 10, MPLS TTL = 20 and outer TTL = 30.

ATE action:

*   Generate 5 **IPv4oMPLSoGRE packets** from ATE:Port1 with below headers settings.
    *   Set the inner IP header destination to IPv4-DST-NET/32.
    *   Set the inner IP header TTL to *10*.
    *   Set the MPLS label to *100010*. 
    *   Set the MPLS TTL to *20*.
    *   Set the outer IP header destination to IPv4-DST-DECAP/32.
    *   Set the outer IP header TTL to *30*.

Verify:

*   DUT interface DUT:Port1 `in-unicast-pkts` counters equals the number of
    packets generated from ATE:Port1.
*   DUT interface DUT:Port2 `out-unicast-pkts` counters equals the number of
    packets generated from ATE:Port1.
*   The packet count of traffic received on ATE:Port2 should be equal to the
    packets generated from ATE:Port1.
*   Verify that decapsulation rule counters match number of packets from ATE:Port1.
*   TTL for all packets received on ATE:Port2 should *10*.

### PF-1.9.12: IPv6oMPLSoGRE traffic with inner TTL = 10, MPLS TTL = 20 and outer TTL = 30.

*   Repeat `PF-1.9.11` using IPv6oMPLSoGRE with inner header destination IP of
    IPv6-DST-NET/128 and MPLS label of 100020.

### PF-1.9.13: IPv4oMPLSoGRE traffic with inner TTL = 1, MPLS TTL = 20 and outer TTL = 30.

ATE action:

*   Generate 5 **IPv4oMPLSoGRE packets** from ATE:Port1 with below headers settings.
    *   Set the inner IP header destination to IPv4-DST-NET/32.
    *   Set the inner IP header TTL to *1*.
    *   Set the MPLS label to *100010*.
    *   Set the MPLS TTL to *20*.
    *   Set the outer IP header destination to IPv4-DST-DECAP/32.
    *   Set the outer IP header TTL to *30*.

Verify:

*   Perform same verifications in `PF-1.9.11`.

### PF-1.9.14: IPv6oMPLSoGRE traffic with inner TTL = 1, MPLS TTL = 20 and outer TTL = 30.

*   Repeat `PF-1.9.11` using IPv6oMPLSoGRE with inner header destination IP of
    IPv6-DST-NET/128 and MPLS label of 100020.

### PF-1.9.15: IPv4oMPLSoGRE traffic with inner TTL = 10, MPLS TTL = 1 and outer TTL = 30.

ATE action:

*   Generate 5 **IPv4oMPLSoGRE packets** from ATE:Port1 with below headers settings.
    *   Set the inner IP header destination to IPv4-DST-NET/32.
    *   Set the inner IP header TTL to *10*.
    *   Set the MPLS label to *100010*.
    *   Set the MPLS TTL to *1*.
    *   Set the outer IP header destination to IPv4-DST-DECAP/32.
    *   Set the outer IP header TTL to *30*.

Verify:

*   DUT interface DUT:Port1 `in-unicast-pkts` counters equals the number of
    packets generated from ATE:Port1.
*   ATE:Port1 received ICMP TTL Exceeded packets for all packets sent.

### PF-1.9.16: IPv6oMPLSoGRE traffic with inner TTL = 10, MPLS TTL = 1 and outer TTL = 30.

*   Repeat `PF-1.9.15` using IPv6oMPLSoGRE with inner header destination IP of
    IPv6-DST-NET/128 and MPLS label of 100020.

### PF-1.9.17: IPv4oUDP traffic with inner TTL = 10 and outer TTL = 30.

*   Repeat `PF-1.9.5` using IPv4oUDP (GUE Variant 1)

### PF-1.9.18: IPv6oUDP traffic with inner TTL = 10 and outer TTL = 30.

*   Repeat `PF-1.9.6` using IPv6oUDP (GUE Variant 1)

### PF-1.9.19: IPv4oUDP traffic with inner TTL = 1 and outer TTL = 30.

*   Repeat `PF-1.9.7` using IPv4oUDP (GUE Variant 1)

### PF-1.9.20: IPv6oUDP traffic with inner TTL = 1 and outer TTL = 30.

*   Repeat `PF-1.9.8` using IPv6oUDP (GUE Variant 1)

### PF-1.9.21: IPv4oUDP traffic with inner TTL = 10 and outer TTL = 1.

*   Repeat `PF-1.9.9` using IPv4oUDP (GUE Variant 1)

### PF-1.9.22: IPv6oUDP traffic with inner TTL = 10 and outer TTL = 1.

*   Repeat `PF-1.9.10` using IPv6oUDP (GUE Variant 1)

### PF-1.9.23: IPv4oMPLSoUDP traffic with inner TTL = 10, MPLS TTL = 20 and outer TTL = 30.

*   Repeat `PF-1.9.11` using IPv4oMPLSoUDP (GUE Variant 1)

### PF-1.9.24: IPv6oMPLSoUDP traffic with inner TTL = 10, MPLS TTL = 20 and outer TTL = 30.

*   Repeat `PF-1.9.12` using IPv6oMPLSoUDP (GUE Variant 1)

### PF-1.9.25: IPv4oMPLSoUDP traffic with inner TTL = 1, MPLS TTL = 20 and outer TTL = 30.

*   Repeat `PF-1.9.13` using IPv4oMPLSoUDP (GUE Variant 1)

### PF-1.9.26: IPv6oMPLSoUDP traffic with inner TTL = 1, MPLS TTL = 20 and outer TTL = 30.

*   Repeat `PF-1.9.14` using IPv6oMPLSoUDP (GUE Variant 1)

### PF-1.9.27: IPv4oMPLSoUDP traffic with inner TTL = 10, MPLS TTL = 1 and outer TTL = 30.

*   Repeat `PF-1.9.15` using IPv6oMPLSoUDP (GUE Variant 1)

### PF-1.9.28: IPv6oMPLSoUDP traffic with inner TTL = 10, MPLS TTL = 1 and outer TTL = 30.

*   Repeat `PF-1.9.16` using IPv6oMPLSoUDP (GUE Variant 1)


## Canonical OpenConfig

```json
{
    "network-instances": {
        "network-instance": [
            {
                "name": "DEFAULT",
                "config": {
                    "name": "DEFAULT"
                },
                "mpls": {
                   "lsps": {
                      "static-lsps": {
                         "static-lsp": [
                            {
                               "config": {
                                  "name": "ipv4-static-lsp"
                               }
                               "egress": {
                                  "config": {
                                     "incoming-label": 100010
                                     "next-hop": "10.1.1.1"
                                  }
                               }
                            },
                            {
                               "config": {
                                  "name": "ipv6-static-lsp"
                               }
                               "egress": {
                                  "config": {
                                     "incoming-label": 100020
                                     "next-hop": "fc00:10:1:1::1"
                                  }
                               }
                            }
                         ]
                      }
                   }
                }
                "policy-forwarding": {
                    "policies": {
                        "policy": [
                            {
                                "config": {
                                    "policy-id": "gre_decap"
                                },
                                "rules": {
                                    "rule": [
                                        {
                                            "sequence-id": 0
                                            "config": {
                                                "sequence-id": 0
                                            }
                                            "ipv4": {
                                                "config": {
                                                    "destination-address": "10.2.2.2"
                                                    "protocol": IP_GRE
                                                }
                                            },
                                            "action": {
                                                "config": {
                                                    "decapsulate-gre": true
                                                }
                                            }
                                        },
                                        {
                                            "sequence-id": 1
                                            "config": {
                                                "sequence-id": 1
                                            }
                                            "ipv4": {
                                                "config": {
                                                    "destination-address": "10.2.2.2"
                                                    "protocol": IP_UDP
                                                }
                                            },
                                            "transport": {
                                                "config": {
                                                    "destination-port": "6080"
                                                }
                                            },
                                            "action": {
                                                "config": {
                                                    "decapsulate-gue": true
                                                }
                                            }
                                        },
                                        {
                                            "sequence-id": 2
                                            "config": {
                                                "sequence-id": 2
                                            }
                                            "ipv4": {
                                                "config": {
                                                    "destination-address": "10.2.2.2"
                                                    "protocol": IP_UDP
                                                }
                                            },
                                            "transport": {
                                                "config": {
                                                    "destination-port": "6635"
                                                }
                                            },
                                            "action": {
                                                "config": {
                                                    "decapsulate-mpls-in-udp": true
                                                }
                                            }
                                        }
                                    ]
                                }
                            },
                        ]
                    }
                },
            }
        ]
    }
}
```


## OpenConfig Path and RPC Coverage

```yaml
paths:
  # Config
  /network-instances/network-instance/policy-forwarding/policies/policy/config/policy-id:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/config/sequence-id:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/destination-address:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/protocol:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/transport/config/destination-port:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decapsulate-gre:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decapsulate-gue:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decapsulate-mpls-in-udp:

  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/config/incoming-label:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/config/next-hop:

  # Telemetry
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-pkts:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-octets:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
```

