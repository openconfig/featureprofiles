## TE-18.5 gRIBI encapsulation for mpls in UDP with inner TTL manipulation

* Create AFT entries to perform mpls-in-udp encapsulation, but add a condition that the DUT must
  match destination IP to be local and retain TTL = 1 if the incoming TTL = 1.

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

### TE-18.5.1 Retain inner packet TTL=1 if inner TTL=1

The gRIBI client should send this proto message to the DUT to create AFT
entries.  See [OC PR in progress](https://github.com/openconfig/public/pull/1153)
for the new OC AFT model nodes needed for this.  The
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
              }
            }
          }
        }
      }
    }
  }
}
```

* Send traffic from ATE port 1 to DUT port 1 with inner TTL=1
* Validate afts next hop counters
* Using OTG, validate ATE port 2 receives MPLS-IN-UDP packets
  * Validate destination IPs are outer_ipv6_dst_A and outer_ipv6_dst_B
  * Validate inner packet TTL=1 if inner TTL=1

#### OpenConfig Path and RPC Coverage

```yaml
paths:

  # afts state paths set via gRIBI
  #/network-instances/network-instance/afts/next-hops/next-hop/mpls-in-udp/state/ip-ttl:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
  gribi:
    gRIBI.Modify:
    gRIBI.Flush:
```

## Required DUT platform

* FFF
