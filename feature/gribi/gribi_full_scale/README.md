# gRIBI Scaling - Full Scale Setup Common Specification

This document provides a generic specification of the topology, base configuration, procedures, test cases, and OpenConfig path coverage for the gRIBI scaling tests. 

The scaling parameters (e.g., number of VRFs, next hops, entries, and traffic rates) are specific to each target level and are defined in the respective target subdirectories:
* [Target T0 README](./otg_tests/gribi_full_scale_t0/README.md)
* [Target T1 README](./otg_tests/gribi_full_scale_t1/README.md)
* [Target T2 README](./otg_tests/gribi_full_scale_t2/README.md)
* [Target Down README](./otg_tests/gribi_full_scale_down/README.md)

---

## Topology & Baseline

- DUT [port1] <-> ATE [port1]
- DUT [port2] <-> ATE [port2]
- DUT [port1] -> 1 L3 sub-interface <-> ATE [port1] 1 L3 sub-interface, subnet `192.0.2.0/30`
- DUT [port2] -> `NumPort2VLANs` L3 sub-interfaces <-> ATE [port2] `NumPort2VLANs` L3 sub-interfaces.

### Variables
```
# Magic source IP addresses used in this test
* ipv4_outer_src_111 = 198.51.100.111
* ipv4_outer_src_222 = 198.51.100.222
* magic_mac = 02:00:00:00:00:01
```

A gRIBI client is established with the DUT.
The DUT [port1] has a scaled `vrf_selection_policy_c` policy configured:
- `NumEncapVRFs` Encap VRFs: from `ENCAP_TE_VRF_A` to `ENCAP_TE_VRF_<last_char>`
- 3 Transit VRFs: `TE_VRF_111` / `TE_VRF_222` / `REPAIR_VRF`
- 1 Decap VRF: `DECAP_TE_VRF`
- 1 Default VRF

```
network-instances {
    network-instance {
        name: DEFAULT
        policy-forwarding {
            policies {
                policy {
                    policy-id: "vrf_selection_policy_c"
                    rules {
                        rule {
                            sequence-id: 1
                            ipv4 {
                                protocol: 4
                                dscp-set: [dscp_encap_a_1, dscp_encap_a_2]
                                source-address: "ipv4_outer_src_222"
                            }
                            action {
                                decap-network-instance: "DECAP_TE_VRF"
                                post-network-instance: "ENCAP_TE_VRF_A"
                                decap-fallback-network-instance: "TE_VRF_222"
                            }
                        }
                        rule {
                            sequence-id: 2
                            ipv4 {
                                protocol: 41
                                dscp-set: [dscp_encap_a_1, dscp_encap_a_2]
                                source-address: "ipv4_outer_src_222"
                            }
                            action {
                                decap-network-instance: "DECAP_TE_VRF"
                                post-network-instance: "ENCAP_TE_VRF_A"
                                decap-fallback-network-instance: "TE_VRF_222"
                            }
                        }
                        rule {
                            sequence-id: 3
                            ipv4 {
                                protocol: 4
                                dscp-set: [dscp_encap_a_1, dscp_encap_a_2]
                                source-address: "ipv4_outer_src_111"
                            }
                            action {
                                decap-network-instance: "DECAP_TE_VRF"
                                post-network-instance: "ENCAP_TE_VRF_A"
                                decap-fallback-network-instance: "TE_VRF_111"
                            }
                        }
                        rule {
                            sequence-id: 4
                            ipv4 {
                                protocol: 41
                                dscp-set: [dscp_encap_a_1, dscp_encap_a_2]
                                source-address: "ipv4_outer_src_111"
                            }
                            action {
                                decap-network-instance: "DECAP_TE_VRF"
                                post-network-instance: "ENCAP_TE_VRF_A"
                                decap-fallback-network-instance: "TE_VRF_111"
                            }
                        }

# Rules 1-4 are repeated for the range ENCAP_TE_VRF_B through the maximum ENCAP VRF,
# using the corresponding DSCP sets.

                        rule {
                            sequence-id: 65
                            ipv4 {
                                protocol: 4
                                source-address: "ipv4_outer_src_222"
                            }
                            action {
                                decap-network-instance: "DECAP_TE_VRF"
                                post-network-instance: "DEFAULT"
                                decap-fallback-network-instance: "TE_VRF_222"
                            }
                        }
                        rule {
                            sequence-id: 66
                            ipv4 {
                                protocol: 41
                                source-address: "ipv4_outer_src_222"
                            }
                            action {
                                decap-network-instance: "DECAP_TE_VRF"
                                post-network-instance: "DEFAULT"
                                decap-fallback-network-instance: "TE_VRF_222"
                            }
                        }
                        rule {
                            sequence-id: 67
                            ipv4 {
                                protocol: 4
                                source-address: "ipv4_outer_src_111"
                            }
                            action {
                                decap-network-instance: "DECAP_TE_VRF"
                                post-network-instance: "DEFAULT"
                                decap-fallback-network-instance: "TE_VRF_111"
                            }
                        }
                        rule {
                            sequence-id: 68
                            ipv4 {
                                protocol: 41
                                source-address: "ipv4_outer_src_111"
                            }
                            action {
                                decap-network-instance: "DECAP_TE_VRF"
                                post-network-instance: "DEFAULT"
                                decap-fallback-network-instance: "TE_VRF_111"
                            }
                        }
                        rule {
                            sequence-id: 69
                            ipv4 {
                                dscp-set: [dscp_encap_a_1, dscp_encap_a_2]
                            }
                            action {
                                network-instance: "ENCAP_TE_VRF_A"
                            }
                        }
                        rule {
                            sequence-id: 70
                            ipv6 {
                                dscp-set: [dscp_encap_a_1, dscp_encap_a_2]
                            }
                            action {
                                network-instance: "ENCAP_TE_VRF_A"
                            }
                        }
# Rules 69-70 are repeated for the range of ENCAP VRFs using the corresponding DSCP sets.
                        rule {
                            sequence-id: 101
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

---

## Procedure

### Default VRF setup:
- **A)** Install `NumDefaultNH` NextHops, egressing out different interfaces.
- **B)** Install `NumDefaultNHG` NextHopGroups. Each points to a set of NextHops from **A)**; weight parameters are defined per target scaling.
- **C)** Install `NumDefaultIPv4` IPv4 Entries, each pointing at a unique NHG from **B)**.

### Static groups:
- **S1)** Install 1 NHG pointing to a NH. The NH should be a reference to `REPAIR_VRF`.
- **S2)** Install 1 NHG pointing to a NH. The NH should do decapsulation and point to the Default VRF.

### Transit VRFs setup:
- Add 3 VRFs: `TE_VRF_111`, `TE_VRF_222` and `REPAIR_VRF`.
- Default VRF setup for `TE_VRF_111` / `TE_VRF_222`:
    - **D.1)** Install `NumTransitNH`/2 NextHops. Each will redirect to an IP from **C)**.
    - **D.2)** Install `NumTransitNH`/2 NextHops. Each will redirect to an IP from **C)**.
    - **E.1)** Install `NumTransitNHG`/2 NextHopGroups. Each will contain 1 NextHop from **D.1)** with weight 1 and 1 NextHop from **D.2)** with weight 63. The backup NextHopGroup should be **S1)**.
    - **E.2)** Install `NumTransitNHG`/2 NextHopGroups. Each will contain 1 NextHop from **D.1)** with weight 1 and 1 NextHop from **D.2)** with weight 63. The backup NextHopGroup should be **S2)**.
- `TE_VRF_111`:
    - Install `NumTransitIPv4` `/32` IPv4Entries (no IPv6Entries), each pointing to a NextHopGroup from **E.1)**.
- `TE_VRF_222`:
    - Install `NumTransitIPv4` `/32` IPv4Entries (no IPv6Entries), each pointing to a NextHopGroup from **E.2)**.
- Default VRF setup for `REPAIR_VRF`:
    - **F)** Install `NumRepairNHG` NextHopGroups. 50% of the NHGs should point to 1 NH, and 50% should point to 2 NHs. Each NH should update the src address to `ipv4_outer_src_222`, re-encap to an IPv4 Entry from Repaired VRF. Backup NHG should be **S2)**.
- `REPAIR_VRF`:
    - Install `NumRepairIPv4` IPv4Entries. Each points to a NextHopGroup from **F)**.

### Encap / Decap VRFs gRIBI setup:
- Add `NumEncapVRFs` VRFs for encapsulations (e.g. from `ENCAP_TE_VRF_A` to `ENCAP_TE_VRF_<last_char>`).
- Add 1 VRF for decapsulation: `DECAP_TE_VRF`.
- Inject `NumEncapIPv4PerVRF` IPv4Entries and `NumEncapIPv6PerVRF` IPv6Entries to each of the Encap VRFs.
- The entries in the Encap VRFs point to NextHopGroups in the default VRF. Inject `NumEncapDefaultNHG` NextHopGroups in the default VRF.
- Each NextHopGroup should have a number of NextHops where each NextHop should do encapsulation, update src ip to `ipv4_outer_src_111` and point to a tunnel in the `TE_VRF_111`. In addition, weights specified in the NextHopGroup should be co-prime and have the granularity defined in each target README.
- Overall the number of unique encapsulation NHs should be `NumUniqueEncapNH`.
- Inject `NumDecapEntries` IPv4 entries in the `DECAP_TE_VRF` with a mix of prefix lengths (`/22`, `/24`, `/26`, and `/28`).
- Each NHG points to 1 NH to decapsulate and output to a port.

---

## Test Cases / Verification

- **FIB Programming Validation:** Validate that each entry is installed as `FIB_PROGRAMMED`.
- **Hierarchical Route Structure Validation:** Verify that the route structure is resolved correctly by checking random addresses from `TE_VRF_111` and ensuring they recursively resolve to the expected interfaces.
- **Traffic Validation:** Send traffic across interfaces at `TrafficRateMpps` for `TrafficDuration` with `TrafficLossTol` loss tolerance under the following scenarios:
    - **Encap:** 
      - Send un-encapsulated traffic to all IPv4 and IPv6 entries in all the Encap VRFs
      - For all the `ENCAP_TE_VRF_A` - `ENCAP_TE_VRF_P` (here `VRF_X`), the flows are:
          - src_id=DUT-1, dst_ip=[all IPv4s from `VRF_X`], dscp=`encap_vrf_dscp_x_1`
          - src_id=DUT-1, dst_ip=[all IPv4s from `VRF_X`], dscp=`encap_vrf_dscp_x_2`
          - src_id=DUT-1, dst_ip=[all IPv6s from `VRF_X`], dscp=`encap_vrf_dscp_x_1`
          - src_id=DUT-1, dst_ip=[all IPv6s from `VRF_X`], dscp=`encap_vrf_dscp_x_2`
      - Verify traffic received by ATE is encapsulated

    - **Decap**
      - Send encapsulated traffic to all the IPv4 expanded from all the prefix
        lengths (`/22`, `/24`, `/26`, and `/28`) in Decap VRF:
      - For all the `ENCAP_TE_VRF_A` - `ENCAP_TE_VRF_P` (here `VRF_X`), the flows are:
          - outer_src_ip=`ipv4_outer_src_111`, outer_dst_ip=[expanded Decap IPv4s],outer_dscp=`encap_vrf_dscp_x_1`, inner_src_ip=DUT-1, inner_dst_ip=DUT-2, inner_dscp=`encap_vrf_dscp_x_1`
          - outer_src_ip=`ipv4_outer_src_111`, outer_dst_ip=[expanded Decap IPv4s],outer_dscp=`encap_vrf_dscp_x_2`, inner_src_ip=DUT-1, inner_dst_ip=DUT-2, inner_dscp=`encap_vrf_dscp_x_2`
      - Verify traffic received by ATE was de-encapsulated

    - **Re-encap**
        - Send encapsulated traffic to all the IPv4 expanded from all the prefix lengths (`/22`, `/24`, `/26`, and `/28`) in Decap VRF to all the Encap VRFs:
            - For all the `ENCAP_TE_VRF_A` - `ENCAP_TE_VRF_P` (here `VRF_X`), the flows are:
                - outer_src_ip=`ipv4_outer_src_111`, outer_dst_ip=[expanded Decap IPv4s], outer_dscp=`encap_vrf_dscp_x_1`,  inner_src_ip=DUT-1, inner_dst_ip=[all IPv4s from `VRF_X`], inner_dscp=`encap_vrf_dscp_x_1`
                - outer_src_ip=`ipv4_outer_src_111`, outer_dst_ip=[expanded Decap IPv4s], outer_dscp=`encap_vrf_dscp_x_2`, inner_src_ip=DUT-1, inner_dst_ip=[all IPv4s from `VRF_X`], inner_dscp=`encap_vrf_dscp_x_2`
                - outer_src_ip=`ipv4_outer_src_111`, outer_dst_ip=[expanded Decap IPv4s],  outer_dscp=`encap_vrf_dscp_x_1`,  inner_src_ip=DUT-1, inner_dst_ip=[all IPv6s from `VRF_X`], inner_dscp=`encap_vrf_dscp_x_1`
                - outer_src_ip=`ipv4_outer_src_111`, outer_dst_ip=[expanded Decap IPv4s],  outer_dscp=`encap_vrf_dscp_x_2`,  inner_src_ip=DUT-1, inner_dst_ip=[all IPv6s from `VRF_X`], inner_dscp=`encap_vrf_dscp_x_2`
                - outer_src_ip=`ipv4_outer_src_222`, outer_dst_ip=[expanded Decap IPv4s],  outer_dscp=`encap_vrf_dscp_x_1`,  inner_src_ip=DUT-1, inner_dst_ip=[all IPv4s from `VRF_X`], inner_dscp=`encap_vrf_dscp_x_1`
                - outer_src_ip=`ipv4_outer_src_222`, outer_dst_ip=[expanded Decap IPv4s],  outer_dscp=`encap_vrf_dscp_x_2`,  inner_src_ip=DUT-1, inner_dst_ip=[all IPv4s from `VRF_X`], inner_dscp=`encap_vrf_dscp_x_2`
                - outer_src_ip=`ipv4_outer_src_222`, outer_dst_ip=[expanded Decap IPv4s],  outer_dscp=`encap_vrf_dscp_x_1`,  inner_src_ip=DUT-1, inner_dst_ip=[all IPv6s from `VRF_X`], inner_dscp=`encap_vrf_dscp_x_1`
                - outer_src_ip=`ipv4_outer_src_222`, outer_dst_ip=[expanded Decap IPv4s],  outer_dscp=`encap_vrf_dscp_x_2`,  inner_src_ip=DUT-1, inner_dst_ip=[all IPv6s from `VRF_X`], inner_dscp=`encap_vrf_dscp_x_2`
        - Verify that traffic received by ATE is encapsulated and outer_dst_ip is not from the expanded Decap IPv4 set.

    - **Transit**
        - Send encapsulated traffic to all the IPv4 Entries from `TE_VRF_111`)`:
            - For all the `ENCAP_TE_VRF_A` - `ENCAP_TE_VRF_P` (here `VRF_X`), the flows are:
                - outer_src_ip=`ipv4_outer_src_111`, outer_dst_ip=[all IPv4s from Repaired], outer_dscp=`encap_vrf_dscp_x_1`, inner_src_ip=DUT-1, inner_dst_ip=DUT-2,inner_dscp=`encap_vrf_dscp_x_1`
                - outer_src_ip=`ipv4_outer_src_111`, outer_dst_ip=[all IPv4s from Repaired], outer_dscp=`encap_vrf_dscp_x_2`, inner_src_ip=DUT-1,inner_dst_ip=DUT-2,inner_dscp=`encap_vrf_dscp_x_2`
        - Verify  that traffic received by ATE stays encapsulated with the outer header having the same source IP and destination IP is from the Transit VRF IPv4 entry set.

    - **Repaired** (incoming after FRR):
        - Send encapsulated traffic to all the IPv4 Entries from `TE_VRF_222`:
        - For all the `ENCAP_TE_VRF_A` - `ENCAP_TE_VRF_P` (here `VRF_X`), the flows are:
            - outer_src_ip=`ipv4_outer_src_222`, outer_dst_ip=[all IPv4s from Repaired], outer_dscp=`encap_vrf_dscp_x_1`, inner_src_ip=DUT-1, inner_dst_ip=DUT-2, inner_dscp=`encap_vrf_dscp_x_1`
            - outer_src_ip=`ipv4_outer_src_222`, outer_dst_ip=[all IPv4s from Repaired], outer_dscp=`encap_vrf_dscp_x_2`, inner_src_ip=DUT-1,inner_dst_ip=DUT-2, inner_dscp=`encap_vrf_dscp_x_2`
        - Verify that traffic received by ATE stays encapsulated with the outer header having the same source IP and destination IP is from the Repaired VRF IPv4 entry set.
---

## Canonical OC
```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "DUT port1",
          "name": "port1",
          "type": "ethernetCsmacd"
        },
        "name": "port1",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 1
              },
              "index": 1,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.1",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.1"
                    }
                  ]
                }
              }
            }
          ]
        }
      },
      {
        "config": {
          "description": "DUT port2 with 640 sub-interfaces",
          "name": "port2",
          "type": "ethernetCsmacd"
        },
        "name": "port2",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 1
              },
              "index": 1,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "198.18.0.1",
                        "prefix-length": 30
                      },
                      "ip": "198.18.0.1"
                    }
                  ]
                }
              },
              "vlan": {
                "config": {
                  "vlan-id": 1
                },
                "match": {
                  "single-tagged": {
                    "config": {
                      "vlan-id": 1
                    }
                  }
                }
              }
            }
          ]
        }
      }
    ]
  },
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "DECAP_TE_VRF",
          "type": "L3VRF"
        },
        "name": "DECAP_TE_VRF"
      },
      {
        "config": {
          "name": "DEFAULT",
          "type": "DEFAULT_INSTANCE"
        },
        "name": "DEFAULT",
        "policy-forwarding": {
          "interfaces": {
            "interface": [
              {
                "config": {
                  "apply-vrf-selection-policy": "vrf_selection_policy_c",
                  "interface-id": "port1"
                },
                "interface-id": "port1",
                "interface-ref": {
                  "config": {
                    "interface": "port1",
                    "subinterface": 1
                  }
                }
              }
            ]
          },
          "policies": {
            "policy": [
              {
                "config": {
                  "policy-id": "vrf_selection_policy_c",
                  "type": "PBR_POLICY"
                },
                "policy-id": "vrf_selection_policy_c",
                "rules": {
                  "rule": [
                    {
                      "action": {
                        "config": {
                          "decap-fallback-network-instance": "TE_VRF_222",
                          "decap-network-instance": "DECAP_TE_VRF",
                          "post-decap-network-instance": "ENCAP_TE_VRF_A"
                        }
                      },
                      "config": {
                        "sequence-id": 1
                      },
                      "ipv4": {
                        "config": {
                          "dscp-set": [
                            10,
                            11
                          ],
                          "protocol": 4,
                          "source-address": "198.51.100.222/32"
                        }
                      },
                      "sequence-id": 1
                    },
                    {
                      "action": {
                        "config": {
                          "network-instance": "DEFAULT"
                        }
                      },
                      "config": {
                        "sequence-id": 101
                      },
                      "sequence-id": 101
                    },
                    {
                      "action": {
                        "config": {
                          "decap-fallback-network-instance": "TE_VRF_222",
                          "decap-network-instance": "DECAP_TE_VRF",
                          "post-decap-network-instance": "ENCAP_TE_VRF_A"
                        }
                      },
                      "config": {
                        "sequence-id": 2
                      },
                      "ipv4": {
                        "config": {
                          "dscp-set": [
                            10,
                            11
                          ],
                          "protocol": 41,
                          "source-address": "198.51.100.222/32"
                        }
                      },
                      "sequence-id": 2
                    },
                    {
                      "action": {
                        "config": {
                          "decap-fallback-network-instance": "TE_VRF_111",
                          "decap-network-instance": "DECAP_TE_VRF",
                          "post-decap-network-instance": "ENCAP_TE_VRF_A"
                        }
                      },
                      "config": {
                        "sequence-id": 3
                      },
                      "ipv4": {
                        "config": {
                          "dscp-set": [
                            10,
                            11
                          ],
                          "protocol": 4,
                          "source-address": "198.51.100.111/32"
                        }
                      },
                      "sequence-id": 3
                    },
                    {
                      "action": {
                        "config": {
                          "decap-fallback-network-instance": "TE_VRF_111",
                          "decap-network-instance": "DECAP_TE_VRF",
                          "post-decap-network-instance": "ENCAP_TE_VRF_A"
                        }
                      },
                      "config": {
                        "sequence-id": 4
                      },
                      "ipv4": {
                        "config": {
                          "dscp-set": [
                            10,
                            11
                          ],
                          "protocol": 41,
                          "source-address": "198.51.100.111/32"
                        }
                      },
                      "sequence-id": 4
                    },
                    {
                      "action": {
                        "config": {
                          "decap-fallback-network-instance": "TE_VRF_222",
                          "decap-network-instance": "DECAP_TE_VRF",
                          "post-decap-network-instance": "DEFAULT"
                        }
                      },
                      "config": {
                        "sequence-id": 65
                      },
                      "ipv4": {
                        "config": {
                          "protocol": 4,
                          "source-address": "198.51.100.222/32"
                        }
                      },
                      "sequence-id": 65
                    },
                    {
                      "action": {
                        "config": {
                          "decap-fallback-network-instance": "TE_VRF_222",
                          "decap-network-instance": "DECAP_TE_VRF",
                          "post-decap-network-instance": "DEFAULT"
                        }
                      },
                      "config": {
                        "sequence-id": 66
                      },
                      "ipv4": {
                        "config": {
                          "protocol": 41,
                          "source-address": "198.51.100.222/32"
                        }
                      },
                      "sequence-id": 66
                    },
                    {
                      "action": {
                        "config": {
                          "decap-fallback-network-instance": "TE_VRF_111",
                          "decap-network-instance": "DECAP_TE_VRF",
                          "post-decap-network-instance": "DEFAULT"
                        }
                      },
                      "config": {
                        "sequence-id": 67
                      },
                      "ipv4": {
                        "config": {
                          "protocol": 4,
                          "source-address": "198.51.100.111/32"
                        }
                      },
                      "sequence-id": 67
                    },
                    {
                      "action": {
                        "config": {
                          "decap-fallback-network-instance": "TE_VRF_111",
                          "decap-network-instance": "DECAP_TE_VRF",
                          "post-decap-network-instance": "DEFAULT"
                        }
                      },
                      "config": {
                        "sequence-id": 68
                      },
                      "ipv4": {
                        "config": {
                          "protocol": 41,
                          "source-address": "198.51.100.111/32"
                        }
                      },
                      "sequence-id": 68
                    },
                    {
                      "action": {
                        "config": {
                          "network-instance": "ENCAP_TE_VRF_A"
                        }
                      },
                      "config": {
                        "sequence-id": 69
                      },
                      "ipv4": {
                        "config": {
                          "dscp-set": [
                            10,
                            11
                          ]
                        }
                      },
                      "sequence-id": 69
                    },
                    {
                      "action": {
                        "config": {
                          "network-instance": "ENCAP_TE_VRF_A"
                        }
                      },
                      "config": {
                        "sequence-id": 70
                      },
                      "ipv6": {
                        "config": {
                          "dscp-set": [
                            10,
                            11
                          ]
                        }
                      },
                      "sequence-id": 70
                    }
                  ]
                }
              }
            ]
          }
        }
      },
      {
        "config": {
          "name": "ENCAP_TE_VRF_A",
          "type": "L3VRF"
        },
        "name": "ENCAP_TE_VRF_A"
      },
      {
        "config": {
          "name": "REPAIR_VRF",
          "type": "L3VRF"
        },
        "name": "REPAIR_VRF"
      },
      {
        "config": {
          "name": "TE_VRF_111",
          "type": "L3VRF"
        },
        "name": "TE_VRF_111"
      },
      {
        "config": {
          "name": "TE_VRF_222",
          "type": "L3VRF"
        },
        "name": "TE_VRF_222"
      }
    ]
  }
}
```

---

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/type:
  /interfaces/interface/ethernet/config/port-speed:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/vlan/config/vlan-id:
  /interfaces/interface/subinterfaces/subinterface/vlan/match/single-tagged/config/vlan-id:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address:
  /network-instances/network-instance/afts/next-hops/next-hop/interface-ref/state/interface:
  /network-instances/network-instance/config/type:
  /network-instances/network-instance/policy-forwarding/interfaces/interface/config/apply-vrf-selection-policy:
  /network-instances/network-instance/policy-forwarding/interfaces/interface/interface-ref/config/interface:
  /network-instances/network-instance/policy-forwarding/interfaces/interface/interface-ref/config/subinterface:
  /network-instances/network-instance/policy-forwarding/policies/policy/config/type:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/network-instance:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/source-address:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/state/decap-fallback-network-instance:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/state/dscp-set:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/state/source-address:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/state/dscp-set:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
  gribi:
    gRIBI.Get:
    gRIBI.Modify:
    gRIBI.Flush:
```
