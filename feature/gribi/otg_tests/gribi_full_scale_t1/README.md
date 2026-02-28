# TE-14.3: gRIBI Scaling - full scale setup, target T1

## Summary

Validate gRIBI scaling requirements (Target T1).

## Topology & Baseline

Use the same topology as TE-14.2 but in increased scale:

- DUT [port1] <-> ATE [port1]
- DUT [port2] <-> ATE [port2]
- DUT [port1] -> 1 L3 sub-interface <-> ATE [port1] 1 L3 sub-interface , subnet `192.0.2.0/30`
- DUT [port2] -> 640 L3 sub-interfaces <-> ATE [port2] 640 L3 sub-interfaces, Use Vlan tagging for differentiation - `198.51.100.0/22` subdivided into `/30` chunks

## Variables
```
# Magic source IP addresses used in this test
  * ipv4_outer_src_111 = 198.51.100.111
  * ipv4_outer_src_222 = 198.51.100.222
  * magic_mac = 02:00:00:00:00:01
  * magic_ip = 192.168.1.1
```

gRIBI client is established with DUT.

DUT [port1] has scaled `vrf_selection_policy_w` configured:

- 16 Encap VRFs: from `ENCAP_TE_VRF_A` to `ENCAP_TE_VRF_P`
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

# Rules 1-4 are repeated for the range ENCAP_TE_VRF_B through ENCAP_TE_VRF_P, 
# using the corresponding DSCP sets (dscp_encap_b_1/2 through dscp_encap_p_1/2). 
# This generates 60 additional rule stanzas (ommitted here).


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
# Rules 69-70 are repeated for the range ENCAP_TE_VRF_B through ENCAP_TE_VRF_P, 
# using the corresponding DSCP sets (dscp_encap_b_1/2 through dscp_encap_p_1/2). 
# This generates 30 additional rule stanzas (ommitted here).
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

## Procedure

_Default (fictitious level) VRF setup:_

- A) Install 1000 NextHops, egressing out different interfaces.
- B) Install 1000 NextHopGroups. Each points to 64 NextHops from A): the weights
specified in the NextHopGroup should be co-prime and the sum of the weights
should be at granularity:

  - T1)
      - 80% (800) NHGs should have granularity 1/512
      - 20% (200) NHGs should have granularity 1/1K

- C) Install 1000 IPv4 Entries, each pointing at a unique NHG from B).

_Static groups:_

- S1) Install 1 NHG pointing to a NH. The NH should be a reference to
`REPAIR_VRF`
- S2) Install 1 NHG pointing to a NH. The NH should do decapsulation and point
to Default VRF

_Transit VRFs setup:_

- Add 3 VRFs: `TE_VRF_111`, `TE_VRF_222` and `REPAIR_VRF`
- Default VRF setup for `TE_VRF_111` / `TE_VRF_222`:
    - D.1) Install 1536 NextHops. Each will redirect to an IP from C).
    - D.2) Install 1536 NextHops. Each will redirect to an IP from C).
    - E.1) Install 768 NextHopGroups. Each will contain 1 NextHops from D.1 with
    weights 1 and 1 NextHop from D.1 with weight 63. The backup NextHopGroup
    should be S1).
    - E.2) Install 768 NextHopGroups. Each will contain 1 NextHops from D.1 with
    weights 1 and 1 NextHop from D.1 with weight 63. The backup NextHopGroup
    should be S2).
- `TE_VRF_111`:
     - Install 200K `/32` IPv4Entries (no IPv6Entries). Each points to a NextHopGroup from E.1).

- `TE_VRF_222`:
     - Install 200K `/32` IPv4Entries (no IPv6Entries). Each points to a NextHopGroup from E.2).

- Default VRF setup for `REPAIR_VRF`:
     - F) Install X NextHopGroup. 50% of the NHG should point to 1 NH, and 50%
     should point to 2 NHs.Each NH should update src address to
     `ipv4_outer_src_222` re-encap to an IPv4 Entry from Repaired VRF. Backup
     NHG should be S2).
         - T2) X = 1K

- `REPAIR_VRF`:
     - Install 200K IPv4Entries. Each points to a NextHopGroup from F)

_Encap / Decap VRFs gRIBI setup:_

- Add 16 VRFs for encapsulations: from `ENCAP_TE_VRF_A` - to `ENCAP_TE_VRF_P`.
- Add 1 VRF for decapsulation, `DECAP_TE_VRF`.
- Inject 10K IPv4Entry-ies and 10K IPv6Entry-ies to each of the 16 Encap VRFs.
- The entries in the Encap VRFs should point to NextHopGroups in the default
VRF. Inject NextHopGroups in the default VRF:
      - T3) 4K
- Each NextHopGroup should have a number of NextHops where each NextHop should
do encapsulation, update src ip to `ipv4_outer_src_111` and point to a tunnel
in the `TE_VRF_111`. In addition, the weights specified in the NextHopGroup
should be co-prime and the sum of the weights should be 1/granularity:
     - 75% NHGs should each point to 8 NHs with granularity 1/64
     - 20% NHGs should each point to 32 NHs with granularity 1/128
     - 5% NHGs should each point to 32 NHs with granularity 1/256
- Overall the number of unique NHs should be:
    - T4) 16K
- Inject 48 ipv4 entries in the `DECAP_TE_VRF` where the entries have a mix of
prefix lengths `/22`, `/24`, `/26`, and `/28`.
- Each NHG points to 1 NH to decapsulate and output to a port

## Test cases

- Validate that each entry is installed as `FIB_PROGRAMMED`
- Validate the correct recursive routes installation:
  - using `/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group`,  `/network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/backup-next-hop-group`, `	/network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops`, `/network-instances/network-instance/afts/next-hops/next-hop/state/` verify correctness of a few installed prefixes 
- Validate the traffic follows the programmed paths. For all the use-cases send
the traffic in 2 tests, each for 5 minutes of total 30 Mpps across interfaces
with _0%_ traffic drop tolerance:
  - packet size of 64 bytes
  - IMIX traffic as a mix of 7:4:1 to 3K:1.5K:0.5K

- _Encap_
    - Send un-encapsulated traffic to all IPv4 and IPv6 entries in all the
      Encap VRFs
    - For all the `ENCAP_TE_VRF_A` - `ENCAP_TE_VRF_P` (here `VRF_X`), the flows are:
        - src_id=DUT-1, dst_ip=[all IPv4s from `VRF_X`], dscp=`encap_vrf_dscp_x_1`
        - src_id=DUT-1, dst_ip=[all IPv4s from `VRF_X`], dscp=`encap_vrf_dscp_x_2`
        - src_id=DUT-1, dst_ip=[all IPv6s from `VRF_X`], dscp=`encap_vrf_dscp_x_1`
        - src_id=DUT-1, dst_ip=[all IPv6s from `VRF_X`], dscp=`encap_vrf_dscp_x_2`
    - Verify traffic received by ATE is encapsulated

- _Decap_
    - Send encapsulated traffic to all the IPv4 expanded from all the prefix
      lengths (`/22`, `/24`, `/26`, and `/28`) in Decap VRF:
    - For all the `ENCAP_TE_VRF_A` - `ENCAP_TE_VRF_P` (here `VRF_X`), the flows are:
        - outer_src_ip=`ipv4_outer_src_111`, outer_dst_ip=[expanded Decap IPv4s],outer_dscp=`encap_vrf_dscp_x_1`, inner_src_ip=DUT-1, inner_dst_ip=DUT-2, inner_dscp=`encap_vrf_dscp_x_1`
        - outer_src_ip=`ipv4_outer_src_111`, outer_dst_ip=[expanded Decap IPv4s],outer_dscp=`encap_vrf_dscp_x_2`, inner_src_ip=DUT-1, inner_dst_ip=DUT-2, inner_dscp=`encap_vrf_dscp_x_2`
    - Verify traffic received by ATE was de-encapsulated

- _Re-encap_
    - Send encapsulated traffic to all the IPv4 expanded from all the prefix lengths (`/22`, `/24`, `/26`, and `/28`) in Decap VRF to all the Encap VRFs:
        - For all the `ENCAP_TE_VRF_A` - `ENCAP_TE_VRF_P` (here `VRF_X`), the flows are:
            - outer_src_ip=`ipv4_outer_src_111`, outer_dst_ip=[expanded Decap IPv4s], outer_dscp=`encap_vrf_dscp_x_1`,  inner_src_ip=DUT-1, inner_dst_ip=[all IPv4s from `VRF_X`], inner_dscp=`encap_vrf_dscp_x_1`
            - outer_src_ip=`ipv4_outer_src_111`, outer_dst_ip=[expanded Decap IPv4s], outer_dscp=`encap_vrf_dscp_x_2`, inner_src_ip=DUT-1, inner_dst_ip=[all IPv4s from `VRF_X`], inner_dscp=`encap_vrf_dscp_x_2`
            - outer_src_ip=`ipv4_outer_src_111`, outer_dst_ip=[expanded Decap IPv4s],  outer_dscp=`encap_vrf_dscp_x_1`,  inner_src_ip=DUT-1, inner_dst_ip=[all IPv6s from `VRF_X`], inner_dscp=`encap_vrf_dscp_x_1
            - outer_src_ip=`ipv4_outer_src_111`, outer_dst_ip=[expanded Decap IPv4s],  outer_dscp=`encap_vrf_dscp_x_2`,  inner_src_ip=DUT-1, inner_dst_ip=[all IPv6s from `VRF_X`], inner_dscp=`encap_vrf_dscp_x_2
            - outer_src_ip=`ipv4_outer_src_222`, outer_dst_ip=[expanded Decap IPv4s],  outer_dscp=`encap_vrf_dscp_x_1`,  inner_src_ip=DUT-1, inner_dst_ip=[all IPv4s from `VRF_X`], inner_dscp=`encap_vrf_dscp_x_1
            - outer_src_ip=`ipv4_outer_src_222`, outer_dst_ip=[expanded Decap IPv4s],  outer_dscp=`encap_vrf_dscp_x_2`,  inner_src_ip=DUT-1, inner_dst_ip=[all IPv4s from `VRF_X`], inner_dscp=`encap_vrf_dscp_x_2`
            - outer_src_ip=`ipv4_outer_src_222`, outer_dst_ip=[expanded Decap IPv4s],  outer_dscp=`encap_vrf_dscp_x_1`,  inner_src_ip=DUT-1, inner_dst_ip=[all IPv6s from `VRF_X`], inner_dscp=`encap_vrf_dscp_x_1`
            - outer_src_ip=`ipv4_outer_src_222`, outer_dst_ip=[expanded Decap IPv4s],  outer_dscp=`encap_vrf_dscp_x_2`,  inner_src_ip=DUT-1, inner_dst_ip=[all IPv6s from `VRF_X`], inner_dscp=`encap_vrf_dscp_x_2`
    - Verify that traffic received by ATE is encapsulated and outer_dst_ip is not from the expanded Decap IPv4 set.

- _Transit_
    - Send encapsulated traffic to all the IPv4 Entries from `TE_VRF_111`)`:
        - For all the `ENCAP_TE_VRF_A` - `ENCAP_TE_VRF_P` (here `VRF_X`), the flows are:
            - outer_src_ip=`ipv4_outer_src_111`, outer_dst_ip=[all IPv4s from Repaired], outer_dscp=`encap_vrf_dscp_x_1`, inner_src_ip=DUT-1, inner_dst_ip=DUT-2,inner_dscp=`encap_vrf_dscp_x_1`
            - outer_src_ip=`ipv4_outer_src_111`, outer_dst_ip=[all IPv4s from Repaired], outer_dscp=`encap_vrf_dscp_x_2`, inner_src_ip=DUT-1,inner_dst_ip=DUT-2,inner_dscp=`encap_vrf_dscp_x_2`
    - Verify  that traffic received by ATE stays encapsulated with the outer header having the same source IP and destination IP is from the Transit VRF IPv4 entry set.

- _Repaired (incoming after FRR)_:
    - Send encapsulated traffic to all the IPv4 Entries from `TE_VRF_222`:
    - For all the `ENCAP_TE_VRF_A` - `ENCAP_TE_VRF_P` (here `VRF_X`), the flows are:
        - outer_src_ip=`ipv4_outer_src_222`, outer_dst_ip=[all IPv4s from Repaired], outer_dscp=`encap_vrf_dscp_x_1`, inner_src_ip=DUT-1, inner_dst_ip=DUT-2, inner_dscp=`encap_vrf_dscp_x_1`
        - outer_src_ip=`ipv4_outer_src_222`, outer_dst_ip=[all IPv4s from Repaired], outer_dscp=`encap_vrf_dscp_x_2`, inner_src_ip=DUT-1,inner_dst_ip=DUT-2, inner_dscp=`encap_vrf_dscp_x_2`
    - Verify that traffic received by ATE stays encapsulated with the outer header having the same source IP and destination IP is from the Repaired VRF IPv4 entry set.

## Canonical OC
```json
{}
```

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
