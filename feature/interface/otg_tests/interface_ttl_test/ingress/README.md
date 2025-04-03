# PF-1.8: Ingress handling of TTL

## Summary

This test verifies TTL handling for ingress flows. The ingress flow could be
encapsulated or not encapsulated by DUT.

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

#### Configuration

1.  DUT:Port1 is configured as Singleton IP interface towards ATE:Port1.

2.  DUT:Port2 is configured as Singleton IP interface towards ATE:Port2.

3.  DUT is configured to form one IPv4 and one IPV6 eBGP session
    with ATE:Port1 using the directly connected Singleton interface IPs.

4.  DUT is configured to form one IPv4 and one IPV6 eBGP session
    with ATE:Port2 using the directly connected Singleton interface IPs.

5.  ATE:Port2 is configured to advertise destination networks
    IPv4-DST-NET/32 and IPv6-DST-NET/128 to DUT.

### PF-1.8.1: IPv4 traffic with no encapsulation on DUT and TTL != 1.

ATE action:

*   Generate 5 **IPv4 packets** from ATE:Port1 to the IP address in
    IPv4-DST-NET.
    *   Set TTL of all packets to *10*.

Verify:

*   Both IPv4 and IPv6 BGP sessions between DUT:Port1 and ATE:Port1 are up.
*   Both IPv4 and IPv6 BGP sessions between DUT:Port2 and ATE:Port2 are up.
*   DUT interface DUT:Port1 `in-unicast-pkts` counters equals the number of
    packets generated from ATE:Port1.
*   DUT interface DUT:Port2 `out-unicast-pkts` counters equals the number of
    packets generated from ATE:Port1.
*   The packet count of traffic received on ATE:Port2 should be equal to the
    packets generated from ATE:Port1.
*   TTL for all packets received on ATE:Port2 should be **9**:

### PF-1.8.2: IPv6 traffic with no encapsulation on DUT and TTL != 1.

*   Repeat `PF-1.8.1` using IPv6 packets for ATE action.

### PF-1.8.3: IPv4 traffic with no encapsulation on DUT and TTL = 1.

ATE action:

*   Generate 5 **IPv4 packets** from ATE:Port1 to the IP address in
    IPv4-DST-NET.
    *   Set TTL of all packets to *1*.

Verify:

*   DUT interface DUT:Port1 `in-unicast-pkts` counters equals the number of
    packets generated from ATE:Port1.
*   ATE:Port1 received ICMP TTL exceeded packets for all packets sent.

### PF-1.8.4: IPv6 traffic with no encapsulation on DUT and TTL = 1.

*   Repeat `PF-1.8.3` using IPv6 packets for ATE action.

### PF-1.8.5: IPv4 traffic with GRE encapsulation on DUT and TTL != 1.

ATE action:

*   Generate 5 **IPv4 packets** from ATE:Port1 to the IP address in
    IPv4-DST-NET.
    *   Set TTL of all packets to *10*.

Verify:

*   Perform same verifications in `PF-1.8.1`.
    *   In addition, verify that encapsulation rules match number of packets
    from ATE:Port1.

### PF-1.8.6: IPv6 traffic with GRE encapsulation on DUT and TTL != 1.

*   Repeat `PF-1.8.5` using IPv6 packets for ATE action.

### PF-1.8.7: IPv4 traffic with GRE encapsulation on DUT and TTL = 1.

ATE action:

*   Generate 5 **IPv4 packets** from ATE:Port1 to the IP address in
    IPv4-DST-NET.
    *   Set TTL of all packets to *1*.

Verify:

*   DUT interface DUT:Port1 `in-unicast-pkts` counters equals the number of
    packets generated from ATE:Port1.
*   ATE:Port1 received ICMP TTL exceeded packets for all packets sent.

### PF-1.8.8: IPv6 traffic with GRE encapsulation on DUT and TTL = 1.

*   Repeat `PF-1.8.7` using IPv6 packets for ATE action.


## Canonical OpenConfig for policy-forwarding matching IPv4 and encapsulate GRE

TODO: new OC paths to be proposed are present in below JSON
* config/rules/rule/action/count: true
* config/rules/rule/action/next-hop-group
* encap-headers/encap-header/type: "GRE" and associated parameters

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
                                            "ipv4": {
                                                "config": {
                                                    "destination-address": "inner_dst_ipv4"
                                                }
                                            },
                                            "action": {
                                                "config": {
                                                    "count": true
                                                    "next-hop-group": "customer1_gre_encap_v4_nhg",
                                                }
                                            }
                                        }
                                    ]
                                }
                            },
                        ]
                    }
                },
                "static": {
                    "next-hop-groups": {
                        "net-hop-group": [
                            {
                                "config": {
                                    "name": "customer1_gre_encap_v4_nhg"
                                },
                                "name": "customer1_gre_encap_v4_nhg",
                                "next-hops": {
                                    "next-hop": [
                                        {
                                            "index": 1,
                                            "config": {
                                                "index": 1
                                            }
                                        },
                                    ]
                                }
                            }
                        ]
                    },
                    "next-hops": {
                        "next-hop": [
                            {
                                "index": 1,
                                "config": {
                                    "index": 1,
                                    "encap-headers": {
                                        "encap-header": [
                                            {
                                                "index": 1,
                                                "type": "GRE",
                                                "config": {
                                                    "dst-ip": "outer_ipv4_dst",
                                                    "src-ip": "outer_ipv4_src",
                                                    "dscp": "outer_dscp",
                                                    "ip-ttl": "outer_ip-ttl"
                                                }
                                            },
                                        ]
                                    }
                                }
                            },
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
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-pkts:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-octets:
  /network-instances/network-instance/afts/policy-forwarding/policy-forwarding-entry/state/counters/packets-forwarded:
  /network-instances/network-instance/afts/policy-forwarding/policy-forwarding-entry/state/counters/octets-forwarded:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/sequence-id:
  /network-instances/network-instance/policy-forwarding/interfaces/interface/config/apply-forwarding-policy:
  /network-instances/network-instance/policy-forwarding/interfaces/interface/config/interface-id:
  /network-instances/network-instance/policy-forwarding/policies/policy/config/policy-id:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/destination-address:
  /network-instances/network-instance/static/next-hop-groups/next-hop-group/config/name:

  #TODO: Add new OC for GRE encap headers
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/config/index:
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/config/next-hop:
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/config/index:
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/type:
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/dst-ip:
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/src-ip:
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/dscp:
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/ip-ttl:
  #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/index:

  #TODO: Add new OC for policy forwarding actions
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

