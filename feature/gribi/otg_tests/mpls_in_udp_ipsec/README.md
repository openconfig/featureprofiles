# TE-18.6 gRIBI MPLS in UDP Encapsulation for IPSec

Use the gRIBI applied ip entries from TE-18.1 gRIBI. Create AFT entries using gRIBI to match on next hop group in a
network-instance and encapsulate matching packets with IPSec encryption in MPLS in UDP.

The MPLS in UDP encapsulation is expected to follow
[rfc7510](https://datatracker.ietf.org/doc/html/rfc7510#section-3),
but relaxing the requirement for a well-known destination UDP port.  gRIBI is
expected to be able to set the destination UDP port.

## Topology

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Test setup

TODO: Complete test environment setup steps

inner_ipv6_dst_A = "2001:aa:bb::1/128"
inner_ipv6_dst_B = "2001:aa:bb::2/128"
inner_ipv6_default = "::/0"

ipv4_inner_dst_A = "10.5.1.1/32"
ipv4_inner_dst_B = "10.5.1.2/32"
ipv4_inner_default = "0.0.0.0/0"

outer_ipv6_src =      "2001:f:a:1::0"
outer_ipv6_dst_A =    "2001:f:c:e::1"
outer_ipv6_dst_B =    "2001:f:c:e::2"
outer_ipv6_dst_def =  "2001:1:1:1::0"
outer_dst_udp_port =  "6635"
outer_dscp =          "26"
outer_ip-ttl =        "64"

## Procedure

### TE-18.1.1 Match and Encapsulate using gRIBI aft modify

#### gRIBI RPC content

The gRIBI client should send this proto message to the DUT to create AFT
entries.  See [OC AFT Encap PR in progress](https://github.com/openconfig/public/pull/1153)
for the new OC AFT model nodes needed for this.  

TODO: The
[gRIBI v1 protobuf defintions](https://github.com/openconfig/gribi/blob/master/v1/proto/README.md)
will be generated from the afts tree.

```proto
network_instances: {
  network_instance: {
    afts {
      #
      # entries used for "group_A"
      ipv6_unicast {
        ipv6_entry {
          prefix: "inner_ipv6_dst_A"   # this is an IPv6 entry for the origin/inner packet.
          next_hop_group: 100
        }
      }
      ipv4_unicast {
        ipv4_entry {
          prefix: "ipv4_inner_dst_A"   # this is an IPv4 entry for the origin/inner packet.
          next_hop_group: 100
        }
      }
      next_hop_groups {
        next_hop_group {
          next_hop_group_id: "nhg_A"  # New OC path /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/next-hop-group-id
          id: 100
          next_hops {            # reference to a next-hop
            next_hop: {
              index: 100
            }
          }
        }
      }
      next_hops {
        next_hop {
          index: 100
          network_instance: "group_A"
          encapsulate_header: OPENCONFIG_AFT_TYPES:MPLS_IN_UDPV6
          encap-headers {
            encap-header {
              index: 1
              pushed_mpls_label_stack: [100,]
            }
          }
          encap-headers {
            encap-header {
              index: 2
              src_ip: "outer_ipv6_src"
              dst_ip: "outer_ipv6_dst_A"
              dst_udp_port: "outer_dst_udp_port"
              ip_ttl: "outer_ip-ttl"
              dscp: "outer_dscp"
              key_chain_name: "ipsec_keychain"
            }
          }
        }
      }
      #
      # entries used for "group_B"
      ipv6_unicast {
        ipv6_entry {
          prefix: "inner_ipv6_dst_B"
          next_hop_group: 200
        }
      }
      ipv4_unicast {
        ipv4_entry {
          prefix: "ipv4_inner_dst_B"
          next_hop_group: 200
        }
      }
      next_hop_groups {
        next_hop_group {
          next_hop_group_id: "nhg_A"  # new OC path /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/next-hop-group-id
          id: 200
          next_hops {            # reference to a next-hop
            next_hop: {
              index: 200
            }
          }
        }
      }
      next_hops {
        next_hop {
          index: 200
          network_instance: "group_B"
          encap-headers {
            encap-header {
              index: 1
              type : OPENCONFIG_AFT_TYPES:MPLS
              mpls {
                pushed_mpls_label_stack: [200,]
              }
            }
          }
          encap-headers {
            encap-header {
              index: 2
              type: OPENCONFIG_AFT_TYPES:UDP
              udp {
                src_ip: "outer_ipv6_src"
                dst_ip: "outer_ipv6_dst_B"
                dst_udp_port: "outer_dst_udp_port"
                ip_ttl: "outer_ip-ttl"
                dscp: "outer_dscp"
                key_chain_name: "ipsec_keychain"
              }
            }
          }
        }
      }
    }
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:

  # qos classifier config
  /qos/classifiers/classifier/config/name:
  /qos/classifiers/classifier/terms/term/config/id:
  #/qos/classifiers/classifier/terms/term/conditions/next-hop-group/config/name: # TODO: new OC leaf to be added

  # afts next-hop counters
  /network-instances/network-instance/afts/next-hops/next-hop/state/counters/packets-forwarded:
  /network-instances/network-instance/afts/next-hops/next-hop/state/counters/octets-forwarded:

  # afts state paths set via gRIBI
  # TODO: https://github.com/openconfig/public/pull/1153
  #/network-instances/network-instance/afts/next-hops/next-hop/mpls-in-udp/state/src-ip:
  #/network-instances/network-instance/afts/next-hops/next-hop/mpls-in-udp/state/dst-ip:
  #/network-instances/network-instance/afts/next-hops/next-hop/mpls-in-udp/state/ip-ttl:
  #/network-instances/network-instance/afts/next-hops/next-hop/mpls-in-udp/state/dst-udp-port:
  #/network-instances/network-instance/afts/next-hops/next-hop/mpls-in-udp/state/dscp:
  #/network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/next-hop-group-id:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
  gribi:
    gRIBI.Modify:
      network-instances:network-instance:afts:next-hops:next-hop:encapsulate_header:
      network-instances:network-instance:afts:next-hops:next-hop:mpls-in-udp:
    gRIBI.Flush:
```

## Required DUT platform

* FFF