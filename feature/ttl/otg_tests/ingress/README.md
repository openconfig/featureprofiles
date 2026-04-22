# PF-1.8: Ingress handling of TTL

## Summary

This test verifies TTL handling for ingress flows.

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

*   Traffic is generated from ATE:Port1.
*   ATE:Port2 is used as the destination port for flows.

#### Configuration

1.  DUT:Port1 is configured as Singleton IP interface towards ATE:Port1
    with IPv4 and IP6 addresses.

2.  DUT:Port2 is configured as Singleton IP interface towards ATE:Port2
    with IPv4 and IP6 addresses.

3.  DUT is configured with the following static routes:
    a) Destination IPv4-DST-NET/32 next hop ATE:Port2 IPv4 address.
    b) Destination IPv6-DST-NET/32 next hop ATE:Port2 IPv6 address.
    c) Destination IPv4-DST-NET-SERV1/32 next hop ATE:Port2 IPv4 address.
    d) Destination IPv6-DST-NET-SERV1/32 next hop ATE:Port2 IPv6 address.
    c) Destination GRE-IPv4-DST-NET/32 next hop ATE:Port2 IPv4 address.

### PF-1.8.1: IPv4 traffic with no encapsulation on DUT and TTL = 10.

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
*   For ONLY non-encapsulated packets; TTL for all packets received on ATE:Port2
    should be *9*.

### PF-1.8.2: IPv6 traffic with no encapsulation on DUT and TTL = 10.

*   Repeat `PF-1.8.1` with ATE generating IPv6 packets IPv6-DST-NET/128.

### PF-1.8.3: IPv4 traffic with no encapsulation on DUT and TTL = 1.

ATE action:

*   Generate 5 **IPv4 packets** from ATE:Port1 to IPv4-DST-NET/32.
    *   Set TTL of all packets to *1*.

Verify:

*   DUT interface DUT:Port1 `in-unicast-pkts` counters equals the number of
    packets generated from ATE:Port1.
*   ATE:Port1 received ICMP TTL Exceeded packets for all packets sent.

### PF-1.8.4: IPv6 traffic with no encapsulation on DUT and TTL = 1.

*   Repeat `PF-1.8.3` with ATE generating IPv6 packets IPv6-DST-NET/128.

### PF-1.8.5: IPv4 traffic with GRE encapsulation on DUT and TTL = 10.

DUT action:

*   Configure DUT for GRE encapsulation as follows.
    *   Packets destined to IPv4-DST-NET-SERV1/32 should be GRE encapsulated
        to GRE-IPv4-DST-NET/32 with outer IP having TTL = 64.
    *   Packets destined to IPv6-DST-NET-SERV1/128 should be GRE encapsulated
        to GRE-IPv4-DST-NET/32 with outer IP having TTL = 64.

ATE action:

*   Generate 5 **IPv4 packets** from ATE:Port1 to IPv4-DST-NET-SERV1/32.
    *   Set TTL of all packets to *10*.

Verify:

*   Perform same verifications in `PF-1.8.1`.
    *   TTL for inner IP is 10.
    *   TTL for outer IP is 64.
    *   In addition, verify that encapsulation rules counter match number of
    packets from ATE:Port1.

### PF-1.8.6: IPv6 traffic with GRE encapsulation on DUT and TTL = 10.

*   Repeat `PF-1.8.5` with ATE generating IPv6 packets IPv6-DST-NET-SERV1/128.

### PF-1.8.7: IPv4 traffic with GRE encapsulation on DUT and TTL = 1 with DUT configured to process TTL = 1 on receiving interface.

DUT action:

*   Additional configuration on DUT
    *   Packets with TTL = 1 should be processed locally first before encapsulation.

ATE action:

*   Generate 5 **IPv4 packets** from ATE:Port1 to IPv4-DST-NET-SERV1/32.
    *   Set TTL of all packets to *1*.

Verify:

*   DUT interface DUT:Port1 `in-unicast-pkts` counters equals the number of
    packets generated from ATE:Port1.
*   ATE:Port1 received ICMP Time Exceeded packets for all packets sent.

### PF-1.8.8: IPv6 traffic with GRE encapsulation on DUT and TTL = 1 with DUT configured to process TTL = 1 on receiving interface.

*   Repeat `PF-1.8.7` with ATE generating IPv6 packets IPv6-DST-NET-SERV1/128.

### PF-1.8.9: GRE encapsulation of IPv4 traffic with TTL = 1 destined to router interface.

DUT action:

*   Additional configuration on DUT
    *   Update GRE encapsulation configuration so that packets with TTL = 1
        destined to the router interface IP should be encapsulated.

ATE action:

*   Generate 5 **IPv4 packets** from ATE:Port1 to IPv4-DST-NET-SERV1/32.
    *   Set TTL of all packets to *1*.

Verify:

*   Perform same verifications in `PF-1.8.5`.

### PF-1.8.10: GRE encapsulation of IPv6 traffic with TTL = 1 destined to router interface.

*   Repeat `PF-1.8.9` with ATE generating IPv6 packets IPv6-DST-NET-SERV1/128.

### Canonical OpenConfig for policy-forwarding matching IPv4 and encapsulate GRE

TODO: New OC paths to be proposed are present in below JSON
* config/rules/rule/action/count: true
* config/rules/rule/action/next-hop-group
* encap-headers/encap-header/type: "GRE" and associated parameters

#### Canonical OC
```json
{
    "network-instances": {
        "network-instance": [
            {
                "name": "DEFAULT",
                "config": {
                    "name": "DEFAULT"
                },
                "policy-forwarding": {
                    "interfaces": {
                        "interface": [
                            {
                                "config": {
                                    "apply-forwarding-policy": "customer1_gre_encap",
                                    "interface-id": "intf1"
                                },
                                "interface-id": "intf1"
                            }
                        ]
                    },
                    "policies": {
                        "policy": [
                            {
                                "config": {
                                    "policy-id": "customer1_gre_encap"
                                },
                                "rules": {
                                    "rule": [
                                        {
                                            "sequence-id": 0,
                                            "config": {
                                                "sequence-id": 0
                                            },
                                            "ipv4": {
                                                "config": {
                                                    "destination-address": "192.168.1.0/24"
                                                }
                                            },
                                            "action": {
                                              "encapsulate-gre": {
                                                "targets": {
                                                  "target": [
                                                    {
                                                      "config": {
                                                        "destination": "10.10.10.1/32",
                                                        "id": "Destination-A"
                                                      },
                                                      "id": "Destination-A"
                                                    }
                                                  ]
                                                }
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
  # Config
  /network-instances/network-instance/policy-forwarding/interfaces/interface/config/apply-forwarding-policy:
  /network-instances/network-instance/policy-forwarding/interfaces/interface/config/interface-id:
  /network-instances/network-instance/policy-forwarding/policies/policy/config/policy-id:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/destination-address:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encapsulate-gre/targets/target/config/destination:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encapsulate-gre/targets/target/config/id:

  # Telemetry
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-pkts:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-octets:
  /network-instances/network-instance/afts/policy-forwarding/policy-forwarding-entry/state/counters/packets-forwarded:
  /network-instances/network-instance/afts/policy-forwarding/policy-forwarding-entry/state/counters/octets-forwarded:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/sequence-id:

  # TODO: Add new OC for GRE encap headers
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/config/index:
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/config/next-hop:
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/config/index:
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/type:
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/dst-ip:
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/src-ip:
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/dscp:
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/ip-ttl:
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/index:

  # TODO: Add new OC for policy forwarding actions
  #/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/next-hop-group:
  #/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/packet-type:
  #/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/count:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
```

