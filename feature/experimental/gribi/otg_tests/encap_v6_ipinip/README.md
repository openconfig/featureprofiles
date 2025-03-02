# TE-22.2: gRIBI IPinIP encap of IPv6 Traffic

## Summary

Test encapsulation of IPv6 packets into IPinIP tunnels using
gRIBI programming, with PBF steering via multiple non-default VRFs
in order to apply unique encapsulation parameters to different
traffic classes, relying upon hierarchical (recursive) route
resolution and ensuring that traffic is load-balanced.

### Topology

#### Physical and Link Layer

```
    DUT port-1 <---> port-1 ATE - (port-1)  
    DUT port-2 <---> port-2 ATE \ (LAG-1)  
    DUT port-3 <---> port-3 ATE /  
    DUT port-4 <---> port-4 ATE \  
    DUT port-5 <---> port-5 ATE / (LAG-2)  
```

#### Traffic

```
        ┌─────────────────────────────────────┐                         
        │                                    5◀───────────────┐         
        │                 ATE                4◀──────────────┐│         
 ┌──────■1                                   3◀────┐         ││         
 │      │                                    2◀───┐│         ││         
 │      └─────────────────────────────────────┘   ││         ││         
 │      ┌─────────────────────────────────────┐   ││         ││         
 │      │                 DUT                 │   ││         ││         
 │      │       ┌──────────────┐              │   ││         ││         
 │      │    ┌──▶ENCAP_TE_VRF_A├──────┐       │   ││         ││         
 │      │    │  └──────────────┘      │       │   ││         ││         
 │      │    │  ┌──────────────┐      │       │  (││}LAG-1  (││}LAG-2   
 │      │    │┌─▶ENCAP_TE_VRF_B├─┐    │       │   ││         ││         
 │      │    ││ └──────────────┘ │    │       │   ││         ││         
 │      │┌───┼┼──────────────────┼────┼──────┐│   ││         ││         
 │      ││  .◇◇─.                ▼    ▼      ││   ││         ││         
 └──────┼▶1( PBF )    NHGs: ◍    ◍    ◍    /2■┼───┘│         ││         
        ││  `─◇─'           └◉─▶ ├◉─▶ ├◉─▶ \3■┼────┘         ││         
        ││    └▶                 ├◉─▶ ├◉─▶ /4■┼──────────────┘│         
        ││         DEFAULT VRF   ├◉─▶ ├◉─▶ \5■┼───────────────┘         
        ││                       └◉─▶ └◉─▶   ││                         
        │└───────────────────────────────────┘│                         
        └─────────────────────────────────────┘                         
```

## Procedure

### Setup

1. As shown in Topology above, connect:
    * ATE port-1 to DUT port-1,  
    * ATE port-2 to DUT port-2,  
    * ATE port-3 to DUT port-3,  
    * ATE port-4 to DUT port-4, and
    * ATE port-5 to DUT port-5.

2. Configure ATE and DUT ports 2-3 to be part of a Static LAG (*LAG-1*).

3. Configure ATE and DUT ports 5-6 to be part of a Static LAG (*LAG-2*).

4. Configure IPv4 and IPv6 addresses for each interface on the DUT and ATE,
for the links connecting port-1, LAG-1, and LAG-2, using the following
values:
    * (TODO): tie `encapSrc` to a loopback interface?
    * DUT port-1:
        * IPv4: `dutPort1.IPv4`/`dutPort1.IPv4Len`
        * IPv6: `dutPort1.IPv6`/`dutPort1.IPv6Len`
    * DUT LAG-1:
        * IPv4: `dutLAG1.IPv4`/`dutLAG1.IPv4Len`
        * IPv6: `dutLAG1.IPv6`/`dutLAG1.IPv6Len`
    * DUT LAG-2:
        * IPv4: `dutLAG2.IPv4`/`dutLAG2.IPv4Len`
        * IPv6: `dutLAG2.IPv6`/`dutLAG2.IPv6Len`
    * ATE port-1:
        * IPv4: `atePort1.IPv4`/`atePort1.IPv4Len`
        * IPv6: `atePort1.IPv6`/`atePort1.IPv6Len`
    * ATE LAG-1:
        * IPv4: `ateLAG1.IPv4`/`ateLAG1.IPv4Len`
        * IPv6: `ateLAG1.IPv6`/`ateLAG1.IPv6Len`
    * ATE LAG-2:
        * IPv4: `ateLAG2.IPv4`/`ateLAG2.IPv4Len`
        * IPv6: `ateLAG2.IPv6`/`ateLAG2.IPv6Len`

5. Create the following empty VRFs: `ENCAP_TE_VRF_A`, `ENCAP_TE_VRF_B`

6. Set the Fallback Network Instance to `DEFAULT` for both `ENCAP_TE_VRF_A`
and `ENCAP_TE_VRF_B` VRFs.

7. Configure a static ARP entry on both the `LAG-1` and `LAG-2` bundle
interfaces for the `anchorIPv4` IP address resolving to `anchorMAC`.

8. Configure a static ND entry on both the `LAG-1` and `LAG-2` bundle
interfaces for the `anchorIPv6` IP address resolving to `anchorMAC`.

9. Configure a DSCP based Policy Based Forwarding (PBF) steering policy
applied to `port-1` to route ingress traffic via `ENCAP_TE_VRF_A` if it
matches DSCP values `dscpEncapA1` or `dscpEncapA2`, or via `ENCAP_TE_VRF_B`
if it matches `dscpEncapB1` or `dscpEncapB2` (*vrf_selection_policy*).

10. Establish gRIBI client connection with DUT, negotiating
`RIB_AND_FIB_ACK` as the requested `ack_type` and persistence mode
`PRESERVE`. Make it become leader. Flush all entries.

11. Using gRIBI:
  * Configure IPv4 and IPv6 default routes for `DEFAULT` VRF:
      * Add an NHG Entry in `DEFAULT` VRF comprised of two NH Entries
      which are defined as the IP Addresses `ateLAG1.IPv4` and
      `ateLAG2.IPv4` (*NHG10*).
      * Add an IPv4 Entry in `DEFAULT` VRF for the default destination
      (0.0.0.0/0) via next-hop group NHG10.
      * Add an IPv6 Entry in `DEFAULT` VRF for the default destination (::/0)
      via next-hop group NHG10.
  * Configure routes for the anchor addresses, via the LAG interfaces
  using MAC override:
      * Add an NHG Entry in `DEFAULT` VRF comprised of two NH Entries
      which are defined as the `LAG-1` and `LAG-2` interfaces, with MAC
      address overridden to use `anchorMAC` (*NHG20*).
      * Add an IPv4 Entry in `DEFAULT` VRF for /32 destination `anchorIPv4` via
      next-hop group NHG20.
      * Add an IPv6 Entry in `DEFAULT` VRF for /128 destination `anchorIPv6` via
      next-hop group NHG20.
  * Configure routes for the encap destination addresses, via the IPv4 anchor
  address
      * Add IPv4 Entries in `DEFAULT` VRF for each of the following /32
      destinations via next-hop IPv4 Address `anchorIPv4`:
        * `encapDestD1`, `encapDestD2`, `encapDestD3`, and `encapDestD4`
        * `encapDestA1`, `encapDestA2`, `encapDestA3`, and `encapDestA4`
        * `encapDestB1`, `encapDestB2`, `encapDestB3`, and `encapDestB4`
  * Configure a route for the test flow source prefix via the ATE connected to
  port-1:
      * Add an IPv6 Entry in `DEFAULT` VRF for destination `flowSrcPrefix`
      via next-hop address `atePort1.IPv6`.
  * Create three Next Hop Groups, each with four IPinIP destinations as
  next-hops:
      * Add an NHG Entry in `DEFAULT` VRF comprised of four NH Entries,
      each of which is defined to apply IPinIP encapsulation (encap header type
      IPv4) with the encap source `encapSrc` and the following encap
      destinations: `encapDestD1`, `encapDestD2`, `encapDestD3`, and
      `encapDestD4` (*NHG-D*).
      * Add an NHG Entry in `DEFAULT` VRF comprised of four NH Entries,
      each of which is defined to apply IPinIP encapsulation (encap header type
      IPv4) with the encap source `encapSrc` and the following encap
      destinations: `encapDestA1`, `encapDestA2`, `encapDestA3`, and
      `encapDestA4` (*NHG-A*).
      * Add an NHG Entry in `DEFAULT` VRF comprised of four NH Entrys,
      each of which is defined to apply IPinIP encapsulation (encap header type
      IPv4) with the encap source `encapSrc` and the following encap
      destinations: `encapDestB1`, `encapDestB2`, `encapDestB3`, and
      `encapDestB4` (*NHG-B*).
  * Configure routes for the test flow destination prefix via the IPinIP
  next-hop groups, such that each VRF uses a different next-hop group:
      * Add an IPv6 Entry in `DEFAULT` VRF for destination 
      `flowDestPrefix` via next-hop group `NHG-D`.
      * Add an IPv6 Entry in `ENCAP_TE_VRF_A` VRF for destination
      `flowDestPrefix` via next-hop group `NHG-A`.
      * Add an IPv6 Entry in `ENCAP_TE_VRF_B` VRF for destination
      `flowDestPrefix` via next-hop group `NHG-B`.

#### Example Values

```
    #  
    # Interface Address values  
    #  
    #   DUT port-1  
    dutPort1 = attrs.Attributes{  
        IPv4:    "192.0.2.1",  
        IPv4Len: 30,  
        IPv6:    "2001:db8:1::1",  
        IPv6Len: 64,  
    }  
    #   ATE port-1  
    atePort1 = attrs.Attributes{  
        MAC:     "02:00:01:01:01:01",  
        IPv4:    "192.0.2.2",  
        IPv4Len: 30,  
        IPv6:    "2001:db8:1::2",  
        IPv6Len: 64,  
    }  
    #   DUT LAG-1  
    dutLAG1 = attrs.Attributes{  
        IPv4:    "192.0.2.5",  
        IPv4Len: 30,  
        IPv6:    "2001:db8:2::1",  
        IPv6Len: 64,  
    }  
    #   ATE LAG-1  
    ateLAG1 = attrs.Attributes{  
        MAC:     "02:00:01:01:01:02",  
        IPv4:    "192.0.2.6",  
        IPv4Len: 30,  
        IPv6:    "2001:db8:2::2",  
        IPv6Len: 64,  
    }  
    #   DUT LAG-2  
    dutLAG2 = attrs.Attributes{  
        IPv4:    "192.0.2.9",  
        IPv4Len: 30,  
        IPv6:    "2001:db8:3::1",  
        IPv6Len: 64,  
    }  
    #   DUT LAG-2  
    ateLAG2 = attrs.Attributes{  
        MAC:     "02:00:01:01:01:03",  
        IPv4:    "192.0.2.10",  
        IPv4Len: 30,  
        IPv6:    "2001:db8:3::2",  
        IPv6Len: 64,  
    }  
    #  
    # Addresses used for anchoring upstream routes to the ATE  
    #  
    #   IPv4 Anchor Address  
    anchorIPv4 = "192.0.2.128"  
    #   IPv6 Anchor Address  
    anchorIPv6 = "2001:DB8:0::80"  
    #   MAC Address  
    anchorMAC = "02:00:01:01:01:80"  
    #  
    # IPv6 Address prefixes for test flow packets  
    #  
    flowSrcPrefix = "2001:DB8:17::/64"  
    flowDestPrefix = "2001:DB8:23::/64"  
    #  
    # IPv4 Address values for Encapsulation Headers  
    #  
    #   IP Source Address for Tunnels from DUT  
    encapSrc = "198.51.100.111"  
    #   IP Destination Addresses for Tunnels from DUT  
    #     for traffic tunneled in DEFAULT VRF  
    encapDestD1 = "198.51.100.221"  
    encapDestD2 = "198.51.100.222"  
    encapDestD3 = "198.51.100.223"  
    encapDestD4 = "198.51.100.224"  
    #     for traffic tunneled in ENCAP_TE_VRF_A VRF  
    encapDestA1 = "198.51.100.231"  
    encapDestA2 = "198.51.100.232"  
    encapDestA3 = "198.51.100.233"  
    encapDestA4 = "198.51.100.234"  
    #     for traffic tunneled in ENCAP_TE_VRF_B VRF  
    encapDestB1 = "198.51.100.241"  
    encapDestB2 = "198.51.100.242"  
    encapDestB3 = "198.51.100.243"  
    encapDestB4 = "198.51.100.244"  
    #  
    # PBF match criteria values  
    #  
    #   DSCP values  
    #     for PBF steering to ENCAP_TE_VRF_A  
    dscpEncapA1 = 10  
    dscpEncapA2 = 18  
    #     for PBF steering to ENCAP_TE_VRF_B  
    dscpEncapB1 = 20  
    dscpEncapB2 = 28  
```

#### Example PBF Policy

```
    network-instances {
        network-instance {
            name: DEFAULT
            policy-forwarding {
                policies {
                    policy {
                        policy-id: "vrf_selection_policy"
                        rules {
                            rule {
                                sequence-id: 1
                                ipv4 {
                                    dscp-set: [dscpEncapA1, dscpEncapA2]
                                }
                                action {
                                    network-instance: "ENCAP_TE_VRF_A"
                                }
                            }
                            rule {
                                sequence-id: 2
                                ipv6 {
                                    dscp-set: [dscpEncapA1, dscpEncapA2]
                                }
                                action {
                                    network-instance: "ENCAP_TE_VRF_A"
                                }
                            }
                            rule {
                                sequence-id: 3
                                ipv4 {
                                    dscp-set: [dscpEncapB1, dscpEncapB2]
                                }
                                action {
                                    network-instance: "ENCAP_TE_VRF_B"
                                }
                            }
                            rule {
                                sequence-id: 4
                                ipv6 {
                                    dscp-set: [dscpEncapB1, dscpEncapB2]
                                }
                                action {
                                    network-instance: "ENCAP_TE_VRF_B"
                                }
                            }
                            rule {
                                sequence-id: 5
                                action {
                                    network-instance: "DEFAULT"
                                }
                            }
                        }
                    }
                }
            }
        }
    }
```

#### Example gRIBI Operations

```
operation {
  id: 1
  network_instance: "DEFAULT"
  op: ADD
  next_hop {
    index: 11
    next_hop {
      ip_address: ateLAG1.IPv4
    }
  }
}
operation {
  id: 2
  network_instance: "DEFAULT"
  op: ADD
  next_hop {
    index: 12
    next_hop {
      ip_address: ateLAG2.IPv4
    }
  }
}
operation {
  id: 3
  network_instance: "DEFAULT"
  op: ADD
  next_hop_group {
    id: 10
    next_hop_group {
      next_hop {
        index: 11
        next_hop {
          weight: 1
        }
      }
      next_hop {
        index: 12
        next_hop {
          weight: 1
        }
      }
    }
  }
}
operation {
  id: 4
  network_instance: "DEFAULT"
  op: ADD
  ipv4 {
    prefix: "0.0.0.0/0"
    ipv4_entry {
      next_hop_group: 10
      next_hop_group_network_instance: "DEFAULT"
    }
  }
} 
operation {
  id: 5
  network_instance: "DEFAULT"
  op: ADD
  ipv6 {
    prefix: "::/0"
    ipv6_entry {
      next_hop_group: 10
      next_hop_group_network_instance: "DEFAULT"
    }
  }
} 
operation {
  id: 6
  network_instance: "DEFAULT"
  op: ADD
  next_hop {
    index: 21
    next_hop {
      interface_ref: LAG-1
      mac_address: anchorMAC
    }
  }
}
operation {
  id: 7
  network_instance: "DEFAULT"
  op: ADD
  next_hop {
    index: 22
    next_hop {
      interface_ref: LAG-2
      mac_address: anchorMAC
    }
  }
}
operation {
  id: 8
  network_instance: "DEFAULT"
  op: ADD
  next_hop_group {
    id: 20
    next_hop_group {
      next_hop {
        index: 21
        next_hop {
          weight: 1
        }
      }
      next_hop {
        index: 22
        next_hop {
          weight: 1
        }
      }
    }
  }
}
operation {
  id: 9
  network_instance: "DEFAULT"
  op: ADD
  ipv4 {
    prefix: "anchorIPv4/32"
    ipv4_entry {
      next_hop_group: 20
      next_hop_group_network_instance: "DEFAULT"
    }
  }
}
operation {
  id: 10
  network_instance: "DEFAULT"
  op: ADD
  ipv6 {
    prefix: "anchorIPv6/128"
    ipv6_entry {
      next_hop_group: 20
      next_hop_group_network_instance: "DEFAULT"
    }
  }
}
operation {
  id: 11
  network_instance: "DEFAULT"
  op: ADD
  next_hop {
    index: 31
    next_hop {
      ip_address: anchorIPv4
    }
  }
}
operation {
  id: 12
  network_instance: "DEFAULT"
  op: ADD
  next_hop_group {
    id: 30
    next_hop_group {
      next_hop {
        index: 31
        next_hop {
          weight: 1
        }
      }
    }
  }
}
operation {
  id: 13
  network_instance: "DEFAULT"
  op: ADD
  ipv4 {
    prefix: "encapDestD1/32"
    ipv4_entry {
      next_hop_group: 30
      next_hop_group_network_instance: "DEFAULT"
    }
  }
}
operation {
  id: 14
  network_instance: "DEFAULT"
  op: ADD
  ipv4 {
    prefix: "encapDestD2/32"
    ipv4_entry {
      next_hop_group: 30
      next_hop_group_network_instance: "DEFAULT"
    }
  }
}
operation {
  id: 15
  network_instance: "DEFAULT"
  op: ADD
  ipv4 {
    prefix: "encapDestD3/32"
    ipv4_entry {
      next_hop_group: 30
      next_hop_group_network_instance: "DEFAULT"
    }
  }
}
operation {
  id: 16
  network_instance: "DEFAULT"
  op: ADD
  ipv4 {
    prefix: "encapDestD4/32"
    ipv4_entry {
      next_hop_group: 30
      next_hop_group_network_instance: "DEFAULT"
    }
  }
}
operation {
  id: 17
  network_instance: "DEFAULT"
  op: ADD
  ipv4 {
    prefix: "encapDestA1/32"
    ipv4_entry {
      next_hop_group: 30
      next_hop_group_network_instance: "DEFAULT"
    }
  }
}
operation {
  id: 18
  network_instance: "DEFAULT"
  op: ADD
  ipv4 {
    prefix: "encapDestA2/32"
    ipv4_entry {
      next_hop_group: 30
      next_hop_group_network_instance: "DEFAULT"
    }
  }
}
operation {
  id: 19
  network_instance: "DEFAULT"
  op: ADD
  ipv4 {
    prefix: "encapDestA3/32"
    ipv4_entry {
      next_hop_group: 30
      next_hop_group_network_instance: "DEFAULT"
    }
  }
}
operation {
  id: 20
  network_instance: "DEFAULT"
  op: ADD
  ipv4 {
    prefix: "encapDestA4/32"
    ipv4_entry {
      next_hop_group: 30
      next_hop_group_network_instance: "DEFAULT"
    }
  }
}
operation {
  id: 21
  network_instance: "DEFAULT"
  op: ADD
  ipv4 {
    prefix: "encapDestB1/32"
    ipv4_entry {
      next_hop_group: 30
      next_hop_group_network_instance: "DEFAULT"
    }
  }
}
operation {
  id: 22
  network_instance: "DEFAULT"
  op: ADD
  ipv4 {
    prefix: "encapDestB2/32"
    ipv4_entry {
      next_hop_group: 30
      next_hop_group_network_instance: "DEFAULT"
    }
  }
}
operation {
  id: 23
  network_instance: "DEFAULT"
  op: ADD
  ipv4 {
    prefix: "encapDestB3/32"
    ipv4_entry {
      next_hop_group: 30
      next_hop_group_network_instance: "DEFAULT"
    }
  }
}
operation {
  id: 24
  network_instance: "DEFAULT"
  op: ADD
  ipv4 {
    prefix: "encapDestB4/32"
    ipv4_entry {
      next_hop_group: 30
      next_hop_group_network_instance: "DEFAULT"
    }
  }
}
operation {
  id: 25
  network_instance: "DEFAULT"
  op: ADD
  next_hop {
    index: 41
    next_hop {
      ip_address: atePort1.IPv6
    }
  }
}
operation {
  id: 26
  network_instance: "DEFAULT"
  op: ADD
  next_hop_group {
    id: 40
    next_hop_group {
      next_hop {
        index: 41
        next_hop {
          weight: 1
        }
      }
    }
  }
}
operation {
  id: 27
  network_instance: "DEFAULT"
  op: ADD
  ipv6 {
    prefix: encapSrcPrefix
    ipv6_entry {
      next_hop_group: 40
      next_hop_group_network_instance: "DEFAULT"
    }
  }
}
operation {
  id: 28
  network_instance: "DEFAULT"
  op: ADD
  next_hop {
    index: 101
    next_hop {
      encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
      ip_in_ip {
        dst_ip: encapSrc
        src_ip: encapDestD1
      }
    }
  }
}
operation {
  id: 29
  network_instance: "DEFAULT"
  op: ADD
  next_hop {
    index: 102
    next_hop {
      encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
      ip_in_ip {
        dst_ip: encapSrc
        src_ip: encapDestD2
      }
    }
  }
}
operation {
  id: 30
  network_instance: "DEFAULT"
  op: ADD
  next_hop {
    index: 103
    next_hop {
      encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
      ip_in_ip {
        dst_ip: encapSrc
        src_ip: encapDestD3
      }
    }
  }
}
operation {
  id: 31
  network_instance: "DEFAULT"
  op: ADD
  next_hop {
    index: 104
    next_hop {
      encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
      ip_in_ip {
        dst_ip: encapSrc
        src_ip: encapDestD4
      }
    }
  }
}
operation {
  id: 32
  network_instance: "DEFAULT"
  op: ADD
  next_hop_group {
    id: 100
    next_hop_group {
      next_hop {
        index: 101
        next_hop {
          weight: 1
        }
      }
      next_hop {
        index: 102
        next_hop {
          weight: 1
        }
      }
      next_hop {
        index: 103
        next_hop {
          weight: 1
        }
      }
      next_hop {
        index: 104
        next_hop {
          weight: 1
        }
      }
    }
  }
}
operation {
  id: 33
  network_instance: "DEFAULT"
  op: ADD
  next_hop {
    index: 201
    next_hop {
      encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
      ip_in_ip {
        dst_ip: encapSrc
        src_ip: encapDestA1
      }
    }
  }
}
operation {
  id: 34
  network_instance: "DEFAULT"
  op: ADD
  next_hop {
    index: 202
    next_hop {
      encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
      ip_in_ip {
        dst_ip: encapSrc
        src_ip: encapDestA2
      }
    }
  }
}
operation {
  id: 35
  network_instance: "DEFAULT"
  op: ADD
  next_hop {
    index: 203
    next_hop {
      encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
      ip_in_ip {
        dst_ip: encapSrc
        src_ip: encapDestA3
      }
    }
  }
}
operation {
  id: 36
  network_instance: "DEFAULT"
  op: ADD
  next_hop {
    index: 204
    next_hop {
      encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
      ip_in_ip {
        dst_ip: encapSrc
        src_ip: encapDestA4
      }
    }
  }
}
operation {
  id: 37
  network_instance: "DEFAULT"
  op: ADD
  next_hop_group {
    id: 200
    next_hop_group {
      next_hop {
        index: 201
        next_hop {
          weight: 1
        }
      }
      next_hop {
        index: 202
        next_hop {
          weight: 1
        }
      }
      next_hop {
        index: 203
        next_hop {
          weight: 1
        }
      }
      next_hop {
        index: 204
        next_hop {
          weight: 1
        }
      }
    }
  }
}
operation {
  id: 38
  network_instance: "DEFAULT"
  op: ADD
  next_hop {
    index: 301
    next_hop {
      encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
      ip_in_ip {
        dst_ip: encapSrc
        src_ip: encapDestB1
      }
    }
  }
}
operation {
  id: 39
  network_instance: "DEFAULT"
  op: ADD
  next_hop {
    index: 302
    next_hop {
      encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
      ip_in_ip {
        dst_ip: encapSrc
        src_ip: encapDestB2
      }
    }
  }
}
operation {
  id: 40
  network_instance: "DEFAULT"
  op: ADD
  next_hop {
    index: 303
    next_hop {
      encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
      ip_in_ip {
        dst_ip: encapSrc
        src_ip: encapDestB3
      }
    }
  }
}
operation {
  id: 41
  network_instance: "DEFAULT"
  op: ADD
  next_hop {
    index: 304
    next_hop {
      encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
      ip_in_ip {
        dst_ip: encapSrc
        src_ip: encapDestB4
      }
    }
  }
}
operation {
  id: 42
  network_instance: "DEFAULT"
  op: ADD
  next_hop_group {
    id: 300
    next_hop_group {
      next_hop {
        index: 301
        next_hop {
          weight: 1
        }
      }
      next_hop {
        index: 302
        next_hop {
          weight: 1
        }
      }
      next_hop {
        index: 303
        next_hop {
          weight: 1
        }
      }
      next_hop {
        index: 304
        next_hop {
          weight: 1
        }
      }
    }
  }
}
operation {
  id: 43
  network_instance: "DEFAULT"
  op: ADD
  ipv6 {
    prefix: flowDestPrefix
    ipv6_entry {
      next_hop_group: 100
      next_hop_group_network_instance: "DEFAULT"
    }
  }
}
operation {
  id: 44
  network_instance: "ENCAP_TE_VRF_A"
  op: ADD
  ipv6 {
    prefix: flowDestPrefix
    ipv6_entry {
      next_hop_group: 200
      next_hop_group_network_instance: "DEFAULT"
    }
  }
}
operation {
  id: 45
  network_instance: "ENCAP_TE_VRF_B"
  op: ADD
  ipv6 {
    prefix: flowDestPrefix
    ipv6_entry {
      next_hop_group: 300
      next_hop_group_network_instance: "DEFAULT"
    }
  }
}
```

### Test Flows

1. Create test flows comprised of the following IPv6 packets, to be sent from
ATE port-1 and received by ATE ports 2-5 (via `LAG-1` and `LAG-2`):
    * Flow Group 1: Packets to be encapsulated via the `DEFAULT` VRF:
      * src: N random addresses within flowSrcPrefix
      * dest: N random addresses within flowDestPrefix
      * dscp: 0
      * ttl: N random values >10 and <250
    * Flow Group 2: Packets to be encapsulated via the `ENCAP_TE_VRF_A` VRF:
      * src: N random addresses within flowSrcPrefix
      * dest: N random addresses within flowDestPrefix
      * dscp: randomly chosen dscpEncapA1 or dscpEncapA2 value
      * ttl: N random values >10 and <250
    * Flow Group 3: Packets to be encapsulated via the `ENCAP_TE_VRF_B` VRF:
      * src: N random addresses within flowSrcPrefix
      * dest: N random addresses within flowDestPrefix
      * dscp: randomly chosen dscpEncapB1 or dscpEncapB2 value
      * ttl: N random values >10 and <250
    * Flow Group 4: Packets to be forwarded without encapsulation via the
    `DEFAULT` VRF:
      * src: N random addresses NOT within flowSrcPrefix
      * dest: N random addresses NOT within flowDestPrefix
      * dscp: 0
      * ttl: N random values >10 and <250
    * Flow Group 5: Packets to be forwarded without encapsulation via the
    `ENCAP_TE_VRF_A` VRF:
      * src: N random addresses NOT within flowSrcPrefix
      * dest: N random addresses NOT within flowDestPrefix
      * dscp: randomly chosen dscpEncapA1 or dscpEncapA2 value
      * ttl: N random values >10 and <250
    * Flow Group 6: Packets to be forwarded without encapsulation via the
    `ENCAP_TE_VRF_B` VRF:
      * src: N random addresses NOT within flowSrcPrefix
      * dest: N random addresses NOT within flowDestPrefix
      * dscp: randomly chosen dscpEncapB1 or dscpEncapB2 value
      * ttl: N random values >10 and <250

2. All of the randomly chosen values described above must be temporarily
retained for comparison in subsequent steps. In the event of a test failure,
those values should be logged for further analysis.

2. Send M packets of each of the N random combinations for each of the Flow
Groups described above.

3. Evaluate whether N x M packets were received on the ATE ports 2-5 (via
`LAG-1` and `LAG-2`) with the correct headers and in the correct quantities for
each Flow Group:
    * Flow Group 1: Packets encapsulated via the `DEFAULT` VRF:
      * outer_src: `encapSrc`
      * outer_dest: one of `encapDestD1`, `encapDestD2`, `encapDestD3`, or
      `encapDestD4` in approximate proportion to the weights assigned by `NHG-D`
      * outer_dscp: 0
      * outer_ttl: matches inner_ttl for each packet
      * inner_src: the N addresses chosen from within `flowSrcPrefix`
      * inner_dest: the N addresses chosen from within `flowDestPrefix`
      * inner_dscp: 0
      * inner_ttl: matches the value chosen for each packet src-dest pair, minus
      1 hop
    * Flow Group 2: Packets encapsulated via the `ENCAP_TE_VRF_A` VRF:
      * outer_src: `encapSrc`
      * outer_dest: one of `encapDestA1`, `encapDestA2`, `encapDestA3`, or
      `encapDestA4` in approximate proportion to the weights assigned by `NHG-A`
      * outer_dscp: matches inner_dscp for each packet
      * outer_ttl: matches inner_ttl for each packet
      * inner_src: the N addresses chosen from within `flowSrcPrefix`
      * inner_dest: the N addresses chosen from within `flowDestPrefix`
      * inner_dscp: the `dscpEncapA1` or `dscpEncapA2` value chosen for each 
      packet src-dest pair
      * inner_ttl: matches the value chosen for each packet src-dest pair, minus
      1 hop
    * Flow Group 3: Packets encapsulated via the `ENCAP_TE_VRF_B` VRF:
      * outer_src: `encapSrc`
      * outer_dest: one of `encapDestB1`, `encapDestB2`, `encapDestB3`, or
      `encapDestB4` in approximate proportion to the weights assigned by `NHG-B`
      * outer_dscp: matches inner_dscp for each packet
      * outer_ttl: matches inner_ttl for each packet
      * inner_src: the N addresses chosen from within `flowSrcPrefix`
      * inner_dest: the N addresses chosen from within `flowDestPrefix`
      * inner_dscp: the `dscpEncapB1` or `dscpEncapB2` value chosen for each 
      packet src-dest pair
      * inner_ttl: matches the value chosen for each packet src-dest pair, minus
      1 hop
    * Flow Group 4: Packets forwarded without encapsulation via the
    `DEFAULT` VRF:
      * encapsulation: none (Protocol field matches transmitted value)
      * src: the N addresses chosen NOT within `flowSrcPrefix`
      * dest: the N addresses chosen NOT within `flowDestPrefix`
      * dscp: 0
      * ttl: matches the value chosen for each packet src-dest pair, minus
      1 hop
    * Flow Group 5: Packets forwarded without encapsulation via the
    `ENCAP_TE_VRF_A` VRF:
      * encapsulation: none (Protocol field matches transmitted value)
      * src: the N addresses chosen NOT within `flowSrcPrefix`
      * dest: the N addresses chosen NOT within `flowDestPrefix`
      * dscp: the `dscpEncapA1` or `dscpEncapA2` value chosen for each 
      packet src-dest pair
      * ttl: matches the value chosen for each packet src-dest pair, minus
      1 hop
    * Flow Group 6: Packets forwarded without encapsulation via the
    `ENCAP_TE_VRF_B` VRF:
      * encapsulation: none (Protocol field matches transmitted value)
      * src: the N addresses chosen NOT within `flowSrcPrefix`
      * dest: the N addresses chosen NOT within `flowDestPrefix`
      * dscp: the `dscpEncapB1` or `dscpEncapB2` value chosen for each 
      packet src-dest pair
      * ttl: matches the value chosen for each packet src-dest pair, minus
      1 hop

## Config Parameter coverage



## Telemetry Parameter coverage



## Protocol/RPC Parameter coverage

* gRIBI
  * Modify()
    * ModifyRequest:
      * AFTOperation:
        * next_hop:
          * id
          * next_hop:
            * encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4
            * ip_in_ip:
              * dst_ip
              * src_ip
    * ModifyResponse:
      * AFTResult:
        * id
        * status
