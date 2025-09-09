# PF-1.11: Rewrite the ingress innner packet TTL

Create a policy-forwarding configuration using gNMI with action ip-ttl.

## Summary

This test verifies using forwarding policy to match on IPv4 and IPv6 traffic
with a particular TTL (hop-limit) value, then encapsulate the matched traffic with MPLSoGRE
IPv4 while setting the inner TTL (hop-limit) to a specified value.

## Topology

* [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Test setup

### Test environment setup

```
                                             -------
                                            |       |
    [ ATE:Port1, ATE:Port2 ] ==== LAG1 ==== |  DUT  | ==== LAG2 ==== [ ATE:Port3, ATE:Port4 ]
                                            |       |
                                             -------
```

*   Traffic is generated from ATE:LAG1 [ATE:Port1 and ATE:Port2]
*   Traffic is encapsulated with MPLSoGRE IPv4 and forwarded using ATE:LAG2
    [ATE:Port3 and ATE:Port4]
*   Constants:
    *   `vrf_name`               = "test_vrf"
    *   `matched_ipv4_src_net`   = "10.10.50.0/24"
    *   `unmatched_ipv4_src_net` = "10.10.51.0/24"
    *   `ipv4_dst_net`           = "10.10.52.0/24"
    *   `matched_ipv6_src_net`   = "2001:f:a::0/120"
    *   `unmatched_ipv6_src_net` = "2001:f:b::0/120"
    *   `ipv6_dst_net`           = "2001:f:c::0/120"
    *   `ipv4_tunnel_src`        = "10.100.100.1"
    *   `ipv4_tunnel_dst_a`      = "10.100.101.1"
    *   `ipv4_tunnel_dst_b`      = "10.100.102.1"
    *   `tunnel_ip_ttl`          = "64"
    *   `matched_ip_ttl`         = "1"
    *   `unmatched_ip_ttl`       = "32"
    *   `rewritten_ip_ttl`       = "1"
    *   `mpls_label`             = "100"
    *   `nexthop_group`          = "NHG-1"

### Configuration

1.  DUT:Port1 and DUT:Port2 are configured as LAG1 towards ATE:Port1 and
    ATE:Port2 respectively.

2.  DUT:LAG1 and ATE:LAG1 are configured with subinterfaces DUT:LAG1.10 and
    ATE:LAG1.10 respectively.
    *   Both are configured with VLAN 10.
    *   DUT:LAG1.10 is configured with VRF `vrf_name`.
    *   DUT:LAG1.10:IPv4 is 192.168.0.1/30.
    *   ATE:LAG1.10:IPv4 is 192.168.0.2/30.

3.  DUT:Port3 and DUT:Port4 are configured as LAG2 towards ATE:Port3 and
    ATE:Port4 respectively.
    *   DUT:LAG2:IPv4 is 192.168.1.1/30.
    *   ATE:LAG2:IPv4 is 192.168.1.2/30.

4.  DUT is configured with default route with nexthop as ATE:LAG2:IPv4. This
    is to ensure the tunnel destination of the MPLSoGRE tunnel is resolvable.

5.  DUT is configured with nexthop-group named `nexthop_group`. The nexthops of
    this nexthop-group are configured as MPLSoGRE IPv4 tunnels:
    *   Outer TTL value: `tunnel_ip_ttl`
    *   Tunnel source: `ipv4_tunnel_src`
    *   Tunnel destination: `ipv4_tunnel_dst_a` or `ipv4_tunnel_dst_b`
    *   MPLS label: `mpls_label`

6.  DUT is configured with a policy-forwarding in `vrf_name` with this rule:
    *   Match condition:
        *    Matching on IPv4 traffic with TTL "matching_ip_ttl"
    *   Actions:
        *   Forward to nexthop-group `nexthop_group`
        *   Inner TTL rewritten as `rewritten_ip_ttl`

7.  Repeat step 6 but for IPv6 traffic.

8.  DUT is configured with default routes (one for IPv4 and one for IPv6) in
    `vrf_name` with nexthop as nexthop-group `nexthop_group`.

## Procedure

**[TODO]** Implement test code.

### TE-1.11.1 Rewrite the ingress innner packet TTL = 1, if the incoming TTL = 1
for IPv4 traffic.

ATE action:
*   Generate total 1000 **IPv4 packets** from ATE:Port1 and ATE:Port2
    with:
    *   Source IP from random addresses in `matched_ipv4_src_net` to destination
        IP addresses in `ipv4_dst_net` addresses.
    *   Use 512 bytes frame size.
    *   Set TTL of all packets to `matched_ip_ttl`.

*   Generate total 1000 **IPv4 packets** from ATE:Port1 and ATE:Port2
    with:
    *   Source IP from random addresses in `unmatched_ipv4_src_net` to
        destination IP addresses in `ipv4_dst_net` addresses.
    *   Use 512 bytes frame size.
    *   Set TTL of all packets to `unmatched_ip_ttl`.

Verify:
*   The total packet count of traffic sent from ATE:Port1 and ATE:Port2 should
    be equal to the sum of all packets received on ATE:Port3 and ATE:Port4.
*   All packets received on ATE:Port3 and ATE:Port4 are encapsulated with
    MPLSoGRE IPv4 with:
    *   MPLS label: `mpls_label`
    *   Source IPv4 address: `ipv4_tunnel_src`
    *   Destination IPv4 address: `ipv4_tunnel_dst_a` or `ipv4_tunnel_dst_b`
    *   Tunnel IP TTL: `tunnel_ip_ttl`
*   Packets with source IPv4 addresses from subnet `matched_ipv4_src_net` has
    inner IP TTL set to `rewritten_ip_ttl`.
*   Packets with source IPv4 addresses from subnet `unmatched_ipv4_src_net` has
    inner IP TTL set to `unmatched_ip_ttl` - 1.

### TE-1.11.2 Rewrite the ingress innner packet TTL = 1, if the incoming TTL = 1
for IPv6 traffic.

ATE action:
*   Generate total 1000 **IPv6 packets** from ATE:Port1 and ATE:Port2
    with:
    *   Source IP from random addresses in `matched_ipv6_src_net` to destination
        IP addresses in `ipv6_dst_net` addresses.
    *   Use 512 bytes frame size.
    *   Set TTL of all packets to `matched_ip_ttl`.

*   Generate total 1000 **IPv6 packets** from ATE:Port1 and ATE:Port2
    with:
    *   Source IP from random addresses in `unmatched_ipv6_src_net` to
        destination IP addresses in `ipv6_dst_net` addresses.
    *   Use 512 bytes frame size.
    *   Set TTL of all packets to `unmatched_ip_ttl`.

Verify:
*   The total packet count of traffic sent from ATE:Port1 and ATE:Port2 should
    be equal to the sum of all packets received on ATE:Port3 and ATE:Port4.
*   All packets received on ATE:Port3 and ATE:Port4 are encapsulated with
    MPLSoGRE IPv4 with:
    *   MPLS label: `mpls_label`
    *   Source IPv4 address: `ipv4_tunnel_src`
    *   Destination IPv4 address: `ipv4_tunnel_dst_a` or `ipv4_tunnel_dst_b`
    *   Tunnel IP TTL: `tunnel_ip_ttl`
*   Packets with source IPv6 addresses from subnet `matched_ipv6_src_net` has
    inner IP TTL set to `rewritten_ip_ttl`.
*   Packets with source IPv6 addresses from subnet `unmatched_ipv6_src_net` has
    inner IP TTL set to `unmatched_ip_ttl` - 1.

## Canonical OC
**[TODO]**: Add MATCH_ACTION policy-forwarding type to OpenConfig Public data
            models.

**[TODO]**: Add ip-ttl policy-forwarding action to OpenConfig Public data
            models, pending https://github.com/openconfig/public/pull/1313.

```json
{
  "network-instances": {
    "network-instance": [
      {
        "name": "test_vrf",
        "config": {
          "name": "test_vrf"
        },
        "policy-forwarding": {
          "policies": {
            "policy": [
              {
                "policy-id": "retain ttl",
                "config": {
                  "policy-id": "retain ttl",
                  "type": "MATCH_ACTION"
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
                          "hop-limit": 1
                        }
                      },
                      "action": {
                        "config": {
                          "next-hop-group": "NHG-1",
                          "ip-ttl": 1
                        }
                      }
                    },
                    {
                      "sequence-id": 2,
                      "config": {
                        "sequence-id": 2
                      },
                      "ipv6": {
                        "config": {
                          "hop-limit": 1
                        }
                      },
                      "action": {
                        "config": {
                          "next-hop-group": "NHG-1",
                          "ip-ttl": 1
                        }
                      }
                    }
                  ]
                }
              }
            ]
          }
        },
        "static": {
          "next-hop-groups": {
            "next-hop-group": [
              {
                "name": "NHG-1",
                "config": {
                  "name": "NHG-1"
                },
                "next-hops": {
                  "next-hop": [
                    {
                      "index": 1,
                      "config": {
                        "index": 1
                      }
                    },
                    {
                      "index": 2,
                      "config": {
                        "index": 2
                      }
                    }
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
                  "index": 1
                },
                "encap-headers": {
                  "encap-header": [
                    {
                      "index": 1,
                      "config": {
                        "index": 1,
                        "type": "MPLS"
                      },
                      "mpls": {
                        "config": {
                          "label": 100
                        }
                      }
                    },
                    {
                      "index": 2,
                      "config": {
                        "index": 2,
                        "type": "GRE"
                      },
                      "gre": {
                        "config": {
                          "src-ip": "10.100.100.1",
                          "dst-ip": "10.100.101.1",
                          "ttl": 64
                        }
                      }
                    }
                  ]
                }
              },
              {
                "index": 2,
                "config": {
                  "index": 2
                },
                "encap-headers": {
                  "encap-header": [
                    {
                      "index": 1,
                      "config": {
                        "index": 1,
                        "type": "MPLS"
                      },
                      "mpls": {
                        "config": {
                          "label": 100
                        }
                      }
                    },
                    {
                      "index": 2,
                      "config": {
                        "index": 2,
                        "type": "GRE"
                      },
                      "gre": {
                        "config": {
                          "src-ip": "10.100.100.1",
                          "dst-ip": "10.100.102.1",
                          "ttl": 64
                        }
                      }
                    }
                  ]
                }
              }
            ]
          }
        },
        "protocols": {
          "protocol": [
            {
              "identifier": "STATIC",
              "name": "STATIC",
              "config": {
                "identifier": "STATIC",
                "name": "STATIC"
              },
              "static-routes": {
                "static": [
                  {
                    "prefix": "0.0.0.0/0",
                    "config": {
                      "prefix": "0.0.0.0/0"
                    },
                    "next-hop-group": {
                      "name": "NHG-1"
                    }
                  },
                  {
                    "prefix": "::/0",
                    "config": {
                      "prefix": "::/0"
                    },
                    "next-hop-group": {
                      "name": "NHG-1"
                    }
                  }
                ]
              }
            }
          ]
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:

  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/hop-limit:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/hop-limit:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/next-hop-group:
  # /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/ip-ttl:  # See TODO

  /network-instances/network-instance/static/next-hop-groups/next-hop-group/config/name:
  /network-instances/network-instance/static/next-hop-groups/next-hop-group/next-hops/next-hop/config/index:
  /network-instances/network-instance/static/next-hops/next-hop/config/index:
  /network-instances/network-instance/static/next-hops/next-hop/encap-headers/encap-header/config/index:
  /network-instances/network-instance/static/next-hops/next-hop/encap-headers/encap-header/config/type:
  /network-instances/network-instance/static/next-hops/next-hop/encap-headers/encap-header/mpls/config/label:
  /network-instances/network-instance/static/next-hops/next-hop/encap-headers/encap-header/gre/config/dst-ip:
  /network-instances/network-instance/static/next-hops/next-hop/encap-headers/encap-header/gre/config/src-ip:
  /network-instances/network-instance/static/next-hops/next-hop/encap-headers/encap-header/gre/config/ttl:

  /network-instances/network-instance/protocols/protocol/identifier:
  /network-instances/network-instance/protocols/protocol/config/identifier:
  /network-instances/network-instance/protocols/protocol/static-routes/static/prefix:
  /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hop-group/name:


rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true

```

## Required DUT platform

* FFF
