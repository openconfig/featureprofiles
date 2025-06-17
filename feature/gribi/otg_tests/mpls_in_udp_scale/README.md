# TE-18.3 MPLS in UDP Encapsulation Scale Test

Building on TE-18.1 and TE-18.2, this test focuses on scaling
gRIBI-programmed MPLS-over-UDP tunnels and associated forwarding
entries, parameterized by key scaling dimensions.

## Topology

**Physical Topology:**

- 8 physical ports total (within 12-port hardware constraint)
  - 4 ports as ingress interfaces (port1-port4)
  - 4 ports as egress/uplink interfaces (port5-port8)

**Logical Interface Scale Design:**

- 32 logical ingress interfaces achieved through:
  - 8 VLAN subinterfaces per physical ingress port (4 ports × 8 VLANs =
    32 logical interfaces)
  - VLAN IDs: 100-107 on port1, 200-207 on port2, 300-307 on port3,
    400-407 on port4
- Multiple VRFs mapped to logical interfaces as required by scale
  profiles
- Each logical interface assigned to appropriate VRF based on test
  profile requirements

<!-- -->

    ATE port-1 <------> port-1 DUT (VLANs 100-107)
    ATE port-2 <------> port-2 DUT (VLANs 200-207)
    ATE port-3 <------> port-3 DUT (VLANs 300-307)
    ATE port-4 <------> port-4 DUT (VLANs 400-407)
    DUT port-5 <------> port-5 ATE (Egress)
    DUT port-6 <------> port-6 ATE (Egress)
    DUT port-7 <------> port-7 ATE (Egress)
    DUT port-8 <------> port-8 ATE (Egress)

- 32 ports as the ‘input port set’ (Ingress)
- 4 ports as “uplink facing” (Egress)
- Network Instances (VRFs) will be mapped from ingress
  ports/subinterfaces as needed by scale profiles.

## Test Setup

### DUT Configuration

1.  **Physical Interface Configuration:**

    - Configure ports 1-8 with IPv6 addressing using base scheme
      2001:f:d:e::/126 network
    - Enable all physical interfaces with PMD100GBASEFR-specific
      settings
    - Apply ethernet configuration: AutoNegotiate=false,
      DuplexMode=FULL, PortSpeed=100GB
    - Set MAC addresses using systematic scheme: 02:01:00:00:00:XX for
      DUT ports

2.  **VLAN Subinterface Configuration:**

    - Create 8 VLAN subinterfaces per ingress port (32 total logical
      interfaces)
    - Assign IPv6 addresses using 2001:f:d:e::/126 base with systematic
      increments
    - Configure subinterface-to-VRF mappings based on test profile
      requirements
    - Enable IPv4 protocols on subinterfaces when
      deviations.InterfaceEnabled(dut) is required

3.  **VRF Configuration:**

    - Create required VRFs based on test profile:
      - Profile 1: DEFAULT network instance only
      - Profiles 2-3: 1024 VRFs (VRF_001 through VRF_1024) plus DEFAULT
      - Profile 4: DEFAULT network instance only
      - Profile 5: DEFAULT network instance only
    - Use device-specific default network instance naming conventions
    - Apply policy-based forwarding rules for VRF selection using
      DSCP/source IP criteria

4.  **Static Routes and Forwarding:**

    - Configure static routes using device-specific static protocol
      naming
    - Set up IPv6 static routes with next-hop pointing to ATE port IPv6
      addresses
    - Use standard static route protocol type configuration
    - Configure routes in appropriate network instances based on test
      profile requirements

### ATE Configuration

1.  **Physical Port Setup:**

    - Configure 8 physical ports with IPv6 addresses matching DUT
      interface scheme
    - Use MAC addresses: 02:00:XX:01:01:01 pattern for ATE ports
    - Set up VLAN tagging on ingress ports (port1-4) to match DUT
      subinterface VLANs
    - Configure egress ports (port5-8) for traffic reception and
      MPLS-in-UDP validation
    - Apply PMD100GBASEFR-specific settings: disable FEC, set speed to
      100Gbps, enable auto-negotiate

2.  **Traffic Generation:**

    - Create traffic flows targeting the 20,000 unique destination
      prefixes
    - Use IPv6 flow destination base: 2015:aa8:: as defined in Test
      Parameters
    - Distribute traffic across 32 logical ingress interfaces using VLAN
      tags
    - Configure flows with appropriate DSCP markings for VRF selection
    - Set traffic duration: 15 seconds as defined in Test Parameters

3.  **Packet Capture and Validation:**

    - Enable packet capture on egress ports for MPLS-in-UDP
      encapsulation validation
    - Configure capture filters for MPLS label stack and UDP
      encapsulation verification
    - Validate outer IPv6 headers: source 2001:f:a:1::0, destination
      2001:f:c:e::1 as defined in Test Parameters
    - Verify UDP destination port 5555 as defined in Test Parameters
    - Check outer DSCP marking: 26 and TTL: 64 as defined in Test
      Parameters

### gRIBI Programming Setup

1.  **Client Configuration:**

    - Establish gRIBI client connection with FIBACK: true and
      Persistence: true
    - Use standard gRIBI client configuration pattern
    - Call BecomeLeader and FlushAll before programming entries
    - Set appropriate batch sizes and operation rates per profile
      requirements

2.  **Entry Programming Sequence:**

    - Program Next Hop (NH) entries with MPLS-in-UDP encapsulation
      headers
    - Use NH ID starting from 201 as defined in Test Parameters
    - Create Next Hop Groups (NHGs) starting from ID 10 as defined in
      Test Parameters
    - Install IPv6 prefix entries using 2015:aa8::/128 base prefix
      pattern as defined in Test Parameters
    - Validate FIB_PROGRAMMED status for all programmed entries

3.  **Scale-Specific Configurations:**

    - Profile 1: DEFAULT network instance with 20,000 NHGs, 1 NH per
      NHG, 1 MPLS label
    - Profiles 2-3: 1024 VRFs with distributed NHGs/prefixes, unique
      MPLS labels per VRF
    - Profile 4: DEFAULT network instance with 2,500 NHGs, 8 NHs per NHG
      (ECMP), 1 MPLS label
    - Profile 5: High-rate programming (6,000 ops/sec) with 50% ADD/50%
      DELETE operations

4.  **Device-Specific Considerations:**

    - Handle vendor-specific gRIBI encapsulation header support
      limitations
    - Use CLI configuration for tunnel encapsulation when gRIBI encap
      headers unsupported
    - Apply device-specific interface enablement requirements for IPv4
      protocols
    - Configure tunnel type: “mpls-over-udp udp destination port 5555”
      as defined in Test Parameters

### Test Parameters

**DUT Interface IPv6 Addressing:**

- dut_port_base_ipv6 = “2001:f:d:e::/126”
- dut_port1_ipv6 = “2001:f:d:e::1/126”
- dut_port2_ipv6 = “2001:f:d:e::5/126”
- dut_port3_ipv6 = “2001:f:d:e::9/126”
- dut_port4_ipv6 = “2001:f:d:e::13/126”
- dut_port5_ipv6 = “2001:f:d:e::17/126”
- dut_port6_ipv6 = “2001:f:d:e::21/126”
- dut_port7_ipv6 = “2001:f:d:e::25/126”
- dut_port8_ipv6 = “2001:f:d:e::29/126”

**ATE Interface IPv6 Addressing:**

- ate_port1_ipv6 = “2001:f:d:e::2/126”
- ate_port2_ipv6 = “2001:f:d:e::6/126”
- ate_port3_ipv6 = “2001:f:d:e::10/126”
- ate_port4_ipv6 = “2001:f:d:e::14/126”
- ate_port5_ipv6 = “2001:f:d:e::18/126”
- ate_port6_ipv6 = “2001:f:d:e::22/126”
- ate_port7_ipv6 = “2001:f:d:e::26/126”
- ate_port8_ipv6 = “2001:f:d:e::30/126”

**MAC Address Schemes:**

- dut_mac_pattern = “02:01:00:00:00:XX”
- ate_mac_pattern = “02:00:XX:01:01:01”

**Inner IPv6 Destinations:**

- inner_ipv6_dst_A = “2001:aa:bb::1/128”
- inner_ipv6_dst_B = “2001:aa:bb::2/128”

**Inner IPv4 Destinations:**

- ipv4_inner_dst_A = “10.5.1.1/32”
- ipv4_inner_dst_B = “10.5.1.2/32”

**Outer IPv6 Encapsulation:**

- outer_ipv6_src = “2001:f:a:1::0”
- outer_ipv6_dst_A = “2001:f:c:e::1”
- outer_ipv6_dst_B = “2001:f:c:e::2”
- outer_ipv6_dst_def = “2001:1:1:1::0”
- outer_dst_udp_port = “5555”
- outer_dscp = “26”
- outer_ip_ttl = “64”

**Traffic Flow Parameters:**

- ipv6_flow_base = “2015:aa8::”
- ipv6_prefix_base = “2015:aa8::/128”

**Traffic Parameters:**

- traffic_duration = “15 seconds”
- target_packet_loss = “≤ 1%”

**gRIBI Parameters:**

- nh_id_start = “201”
- nhg_id_start = “10”

## Procedure

### TE-18.3 Overview: Scaling Dimensions and Targets

This test evaluates scaling across the following dimensions using gRIBI.
The test profiles below represent different parameter combinations of
these dimensions.

- **Network Instances (VRFs):** Number of separate routing instances.
- **Next Hop Groups (NHGs):** Total number of NHGs programmed. Target:
  **Up to 20,000** (profile-dependent).
- **Next Hops (NHs):** Total number of NHs programmed. **Constraint:
  Maximum 20,000 total NHs.** When there are more NHs per NHG, there
  will be fewer total NHGs (e.g., 2,500 NHGs if each NHG has 8 NHs).
- **NHs per NHG:** Number of NH entries within each NHG (e.g., 1 or 8).
- **Prefixes:** Total number of unique IPv4/IPv6 exact-match forwarding
  entries (routes) across all VRFs. Target: **20,000 total**.
- **(Unique Destination IP + MPLS) Tuples:** The combination of the
  inner destination IP and the MPLS label used in the NH encapsulation.
  Target: **Up to 20,000 unique tuples**.
- **MPLS Labels:** Number and uniqueness of MPLS labels used in NH
  encapsulation. **Constraint:** The number of unique MPLS labels must
  equal the number of VRFs (#MPLS Labels == \#VRFs).
- **gRIBI Operations Rate (QPS):** Rate of gRIBI Modify requests or
  operations per second.
- **gRIBI Batch Size:** Number of AFT entries (or operations) per
  ModifyRequest.
- **Convergence:** DUT packet forwarding updated within **1 second**
  after receiving FIB_PROGRAMMED acknowledgement for added entries
  (baseline).
- **IP Address Reuse:** Inner IP destination prefixes should be reused
  across different Network Instances where applicable.
- **Multi-VRF Distribution:** In multi-VRF profiles, both NHGs and
  prefixes are distributed across the different VRFs as specified in
  each profile.

### TE-18.3: Scale Profiles

#### Profile 1 (Single VRF)

- **Goal:** Baseline single VRF scale (Exact Label Match scenario).
- **Network Instances (VRFs):** 1 (DEFAULT).
- **Total NHGs:** 20,000.
- **NHs per NHG:** 1.
- **MPLS Labels:** 1 (consistent with \#VRFs = 1). Same label used for
  all NHs.
- **Total Prefixes:** 20,000 (e.g., 10k IPv4, 10k IPv6).
- **Unique (Dest IP + MPLS) Tuples:** 20,000 (different destination IPs,
  same MPLS label).
- **Prefix Mapping:** 1 unique prefix -\> 1 unique NHG (1:1).
- **Total NHs:** 20,000 (20,000 NHGs × 1 NH/NHG = 20,000 total NHs).
- **gRIBI Rate/Batch:** Baseline (e.g., 1 ModifyRequest/sec, 200
  entries/request) - QPS not the primary focus here.

#### Profile 2 (Multi-VRF)

- **Goal:** Scale across multiple VRFs with unique labels per VRF.
- **Network Instances (VRFs):** 1024.
- **Total NHGs:** 20,000 (distributed across VRFs, ~19-20 NHGs/VRF).
- **NHs per NHG:** 1.
- **Total NHs:** 20,000 (20,000 NHGs × 1 NH/NHG = 20,000 total NHs).
- **MPLS Labels:** 1024 unique labels (1 label assigned per VRF,
  consistent with \#VRFs = 1024).
- **Total Prefixes:** 20,000 (distributed across VRFs, ~19-20
  prefixes/VRF).
- **Unique (Dest IP + MPLS) Tuples:** 20,000 (e.g., 20 unique
  destination IPs reused per MPLS label/VRF).
- **Prefix Mapping:** Prefixes within a VRF map to NHGs using that VRF’s
  unique MPLS label.
- **Inner IP Reuse:** Required.
- **gRIBI Rate/Batch:** Baseline - QPS not the primary focus here.

#### Profile 3 (Multi-VRF)

- **Goal:** Similar to Profile 2, but test potentially skewed
  distribution of prefixes/routes per VRF/label.
- **Network Instances (VRFs):** 1024.
- **Total NHGs:** 20,000.
- **NHs per NHG:** 1.
- **Total NHs:** 20,000 (20,000 NHGs × 1 NH/NHG = 20,000 total NHs).
- **MPLS Labels:** 1024 unique labels (1 per VRF).
- **Total Prefixes:** 20,000.
- **Unique (Dest IP + MPLS) Tuples:** 20,000.
- **Prefix Mapping:** Similar to Profile 2, but the distribution of the
  20k prefixes across the 1024 VRFs/labels might be intentionally uneven
  (e.g., some VRFs have many more prefixes than others). *Exact skew
  pattern TBD.*
- **Inner IP Reuse:** Required.
- **gRIBI Rate/Batch:** Baseline - QPS not the primary focus here.

#### Profile 4 (Single VRF)

- **Goal:** Test ECMP scale within a single VRF.
- **Network Instances (VRFs):** 1 (DEFAULT).
- **Total NHGs:** 2,500.
- **NHs per NHG:** 8 (each NH having a different destination IP).
- **Total NHs:** 20,000 (2,500 NHGs × 8 NHs/NHG = 20,000 total NHs,
  respecting the 20k NH constraint).
- **MPLS Labels:** 1 (consistent with \#VRFs = 1). Same label used for
  all NHs.
- **Total Prefixes:** 20,000 (e.g., 10k IPv4, 10k IPv6).
- **Unique (Dest IP + MPLS) Tuples:** 20,000 (different destination IPs
  across all NHs, same MPLS label).
- **Prefix Mapping:** 8 unique prefixes -\> 1 unique NHG (8:1 mapping,
  repeated 2500 times).
- **gRIBI Rate/Batch:** Baseline - QPS not the primary focus here.

#### Profile 5 (Single VRF)

- **Goal:** Test gRIBI control plane QPS scaling and impact on
  dataplane. Uses Profile 1 as the base state.

- **Network Instances (VRFs):** 1 (DEFAULT).

- **Total NHGs:** 20,000.

- **NHs per NHG:** 1.

- **MPLS Labels:** 1.

- **Total Prefixes:** 20,000.

- **Unique (Dest IP + MPLS) Tuples:** 20,000.

- **Prefix Mapping:** 1:1.

- **Total NHs:** 20,000 (20,000 NHGs × 1 NH/NHG = 20,000 total NHs).

- **gRIBI Operations:** Program/Modify the full 20k entries (1 Prefix +
  1 NHG + 1 NH = 3 operations per entry = 60k operations total).

  - Target Rate: **6,000 operations/second** (aiming to update the full
    table in maximum of 60 seconds).
  - Operation Mix: Test with **50% ADD, 50% DELETE** operations during
    high-rate phase.

- **Dataplane Validation:** Ensure live traffic forwarding remains
  stable and correct during high-rate gRIBI operations. The primary
  success criterion is zero packet loss during the update phase. This
  validates that the DUT correctly implements a “make-before-break”
  update sequence, where traffic for a modified prefix is seamlessly
  forwarded using either the old or the new state, without being
  dropped.

### TE-18.3.3 Validation Procedures

#### Procedure - Single VRF Validation (Profiles 1, 4)

- Program all gRIBI entries (NHs, NHGs, Prefixes) according to the
  profile using baseline rate/batch.
- Validate `RIB_PROGRAMMED` status is received from DUT for all entries.
- Verify AFT state on DUT for a sample of entries (NH, NHG, Prefix -\>
  NHG mapping).
- Send traffic matching programmed prefixes from appropriate ingress
  ports.
- Verify traffic is received on egress ports with correct MPLS-over-UDP
  encapsulation (correct outer IPs, UDP port, MPLS label).
- Measure packet loss (target: \<= 1% steady state).
- Delete all gRIBI entries.
- Verify AFT state shows entries removed.
- Verify traffic loss is 100%.

#### Procedure - Multi-VRF Validation (Profiles 2, 3)

- Program all gRIBI entries across all specified VRFs according to the
  profile using baseline rate/batch.
- Validate `FIB_PROGRAMMED` status for all entries.
- Verify AFT state on DUT for a sample of entries within different VRFs.
- Send traffic matching programmed prefixes, ensuring traffic is
  directed to the correct VRF (e.g., via appropriate ingress interface
  mapping).
- Verify traffic is received with correct MPLS-over-UDP encapsulation,
  including the VRF-specific MPLS label.
- Measure packet loss (target: \<= 1% steady state).
- Delete all gRIBI entries.
- Verify AFT state shows entries removed across VRFs.
- Verify traffic loss is 100%.

#### Procedure - ECMP Validation (Profile 4)

- Perform Single VRF Validation steps.
- Additionally, verify that traffic sent towards prefixes mapped to the
  ECMP NHG is distributed across the multiple NHs within that NHG
  (requires ATE support for flow analysis or DUT counter validation for
  NH packet/octet counters).

#### Procedure - gRIBI Rate Validation (Profile 5)

- Establish the baseline state (e.g., program 20k entries as per Profile
  1).

- Start traffic flows matching the programmed entries. Verify baseline
  forwarding and low loss.

- Initiate high-rate gRIBI Modify operations (e.g., 100
  ModifyRequests/sec, 60 ops/request, 50% ADD/50% DELETE mix targeting
  existing/new entries).

- Monitor gRIBI operation results (ACKs) for success/failure and
  latency.

- Continuously monitor traffic forwarding during the high-rate gRIBI
  phase.

  - Verify traffic uses correct encapsulation based on the programmed
    state.
  - Measure packet loss (target: minimal loss, allowing for brief
    transient loss during updates, but stable low loss overall).

- Validate `FIB_PROGRAMMED` status is received promptly for updates.

- Verify AFT state on DUT reflects the changes made during the high-rate
  phase.

- Stop high-rate programming and measure steady-state loss again.

#### Investigation - VRF Impact on QPS

- As an extension, investigate if the number of VRFs impacts gRIBI QPS
  or dataplane stability during high-rate updates. This could involve
  running a variation of Profile 5 using the multi-VRF setup from
  Profile 2 or 3 as the baseline state.

#### OpenConfig Path and RPC Coverage

``` yaml
paths:
  # AFTs Next-Hop state (Verification)
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/state/type:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/mpls/state/mpls-label-stack:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/udp-v6/state/src-ip:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/udp-v6/state/dst-ip:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/udp-v6/state/src-udp-port:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/udp-v6/state/dst-udp-port:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/udp-v6/state/ip-ttl:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/udp-v6/state/dscp:
  /network-instances/network-instance/afts/next-hops/next-hop/state/counters/packets-forwarded:
  /network-instances/network-instance/afts/next-hops/next-hop/state/counters/octets-forwarded:
  /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address: # NH IP
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index:
  /interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/state/link-layer-address:

  # AFTs Next-Hop-Group state (Verification)
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/next-hop: # Verify NHs in NHG

  # AFTs Prefix Entry state (Verification)
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    # Primarily used for verification (Subscribe/Get)
    gNMI.Subscribe:
      on_change: true
    gNMI.Get:
  gribi:
    # Used for programming all AFT entries
    gRIBI.Modify:
    gRIBI.Flush:
```

## Required DUT platform

- FFF
