# TE-18.1 gRIBI MPLS in UDP Encapsulation and Decapsulation

Create AFT entries using gRIBI to match on next hop group in a
network-instance and encapsulate the matching packets in MPLS in UDP.

Create a policy routing configuration using gNMI to decapsulate MPLS
in UDP packets which are sent to a loopback address and apply to
the DUT.

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
entries.  See [OC PR in progress](https://github.com/openconfig/public/pull/1153)
for the new OC AFT model nodes needed for this.  The
[gRIBI v1 protobuf defintions](https://github.com/openconfig/gribi/blob/master/v1/proto/README.md)
will be generated from the afts tree.

TODO: Update gRIBI protobuf.

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
          mpls_in_udp {
            src_ip: "outer_ipv6_src"
            dst_ip: "outer_ipv6_dst_A"
            pushed_mpls_label_stack: [100,]
            dst_udp_port: "outer_dst_udp_port"
            ip_ttl: "outer_ip-ttl"
            dscp: "outer_dscp"
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
          encapsulate_header: OPENCONFIG_AFT_TYPES:MPLS_IN_UDPV6
          mpls_in_udp {
            src_ip: "outer_ipv6_src"
            dst_ip: "outer_ipv6_dst_B"
            pushed_mpls_label_stack: [200,]
            dst_udp_port: "outer_dst_udp_port"
            ip_ttl: "outer_ip-ttl"
            dscp: "outer_dscp"
          }
        }
      }
    }
  }
}
```

* Send traffic from ATE port 1 to DUT port 1
* Validate afts next hop counters
* Using OTG, validate ATE port 2 receives MPLS-IN-UDP packets
  * Validate destination IPs are outer_ipv6_dst_A and outer_ipv6_dst_B
  * Validate MPLS label is set

### TE-18.1.2 Validate prefix match rule for MPLS in GRE encap using default route

Canonical OpenConfig for policy forwarding, matching IP prefix with action
encapsulate in GRE.

```yaml
openconfig-network-instance:
  network-instances:
    - network-instance: "group_A"
      afts:
        policy-forwarding:
          policies:
            policy: "default encap rule"
              config: 
                policy-id: "default encap rule"
                type: PBR_POLICY
              rules:
                rule: 1
                  config:
                    sequence-id: 1
                  ipv6:
                    config:
                      destination-address: "inner_ipv6_default"
                  action:
                    encapsulate-mpls-in-gre:              # TODO: add to OC model/PR in progress
                      targets:
                        target: "default_dst_1"
                          config:
                            id: "default_dst_1"
                            network-instance: "DEFAULT"
                            source-ip: "outer_ipv6_src"
                            destination-ip: "outer_ipv6_dst_def"
                            ip-ttl: outer_ip-ttl
                            dscp: outer_dscp
                            inner-ttl-min: 2
```

* Generate the policy forwarding configuration
* Push the configuration to DUT using gnmi.Set with REPLACE option
* Configure ATE port 1 with traffic flow which does not match any AFT next hop route
* Generate traffic from ATE port 1 to ATE port 2
* Validate ATE port 2 receives GRE traffic with correct inner and outer IPs

### TE-18.1.3 - MPLS in GRE decapsulation set by gNMI

Canonical OpenConfig for policy forwarding, matching IP prefix with action
decapsulate in GRE.

```yaml
openconfig-network-instance:
  network-instances:
    - network-instance: "DEFAULT"
      afts:
        policy-forwarding:
          policies:
            policy: "default decap rule"
              config: 
                policy-id: "default decap rule"
                type: PBR_POLICY
              rules:
                rule: 1
                  config:
                    sequence-id: 1
                  ipv6:
                    config:
                      destination-address: "decap_loopback_ipv6"
                  action:
                    decapsulate-mpls-in-gre: TRUE             # TODO: add to OC model/PR in progress
```

* Push the gNMI the policy forwarding configuration
* Push the configuration to DUT using gnmi.Set with REPLACE option
* Configure ATE port 1 with traffic flow which matches the decap loopback IP address
* Generate traffic from ATE port 1
* Validate ATE port 2 receives packets with correct VLAN and the inner inner_decap_ipv6

### TE-18.1.4 - MPLS in UDP decapsulation set by gNMI

Canonical OpenConfig for policy forwarding, matching IP prefix with action
decapsulate MPLS in UDP.

```yaml
openconfig-network-instance:
  network-instances:
    - network-instance: "DEFAULT"
      afts:
        policy-forwarding:
          policies:
            policy: "default decap rule"
              config: 
                policy-id: "default decap rule"
                type: PBR_POLICY
              rules:
                rule: 1
                  config:
                    sequence-id: 1
                  ipv6:
                    config:
                      destination-address: "decap_loopback_ipv6"
                  action:
                    decapsulate-mpls-in-udp: TRUE
```

* Push the gNMI the policy forwarding configuration
* Push the configuration to DUT using gnmi.Set with REPLACE option
* Configure ATE port 1 with traffic flow
  * Flow should have a packet encap format : outer_decap_udp_ipv6 <- MPLS label <- inner_decap_ipv6
* Generate traffic from ATE port 1
* Validate ATE port 2 receives the innermost IPv4 traffic with correct VLAN and inner_decap_ipv6

### TE-18.1.5 - Policy forwarding to encap and forward for BGP packets

TODO: Specify a solution for ensuring BGP packets are matched, encapsulated
and forwarding to a specified  destination using OC policy-forwarding terms.

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
      network-instances:network-instance:afts:next-hops:next-hop:decapsulate_header:
    gRIBI.Flush:
```

## Required DUT platform

* FFF
