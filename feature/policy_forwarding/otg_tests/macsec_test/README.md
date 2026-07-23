# PF-1.27: MPLSoGRE/MPLSoGUE MACsec and Line Rate Performance

## Summary

Validate MACsec functionality (including encapsulation and decapsulation of MPLS
over GRE and MPLS over UDP) and line-rate performance on 10G, 100G, and 400G
interfaces. The test ensures that MACsec encryption/decryption, encapsulation,
and decapsulation processes do not introduce packet loss, excessive latency, or
throughput degradation.

All test cases are verified using both IPv4 and IPv6 traffic. Egress
encapsulation and ingress decapsulation types are mapped to interface speeds:

*   **MPLS over GRE** is used for traffic over **10G and 100G** links.
*   **MPLS over UDP** is used for traffic over **400G** links.

## Testbed type

*   `topologies/atedutdutate.testbed` (2-DUT, 1-ATE setup with 10G, 100G, and
    400G links)

```

          ┌──────────┐          ┌──────────┐           ┌──────────┐           ┌──────────┐
          │          │          │          │           │          │           │          │
          │          │   10G    │          │   10G     │          │   10G     │          │
          │         1├──────────│1        2├───────────┤2        1├───────────┤2         │
          │          │          │          │  MACsec   │          │           │          │
          │          │   100G   │          │   100G    │          │   100G    │          │
          │         3├──────────│3        4├───────────┤4        3├───────────┤4         │
          │   ATE    │          │   DUT1   │           │   DUT2   │           │   ATE    │
          │          │   400G   │          │   400G    │          │   400G    │          │
          │         5├──────────│5        6├───────────┤6        5├───────────┤6         │
          │          │          │          │           │          │           │          │
          └──────────┘          └──────────┘           └──────────┘           └──────────┘
```

## Procedure

### Test environment setup

*   Connect the ATE to DUT1 and DUT2, respectively, using 1x10G, 1x100G, and
    1x400G interfaces.
*   Connect DUT1 and DUT2 using 1x10G, 1x100G, and 1x400G interfaces.
*   Enable L3 routing (IPv4 and IPv6) on all interfaces in default VRF.
*   MACsec will be enabled on the links between DUT1 and DUT2.
*   Traffic flows are validated on:
    *   **10G path**: `ATE:port1 <-> DUT1 <-> DUT2 <-> ATE:port2` (using
        MPLSoGRE encapsulation/decapsulation)
    *   **100G path**: `ATE:port3 <-> DUT1 <-> DUT2 <-> ATE:port4` (using
        MPLSoGRE encapsulation/decapsulation)
    *   **400G path**: `ATE:port5 <-> DUT1 <-> DUT2 <-> ATE:port6` (using
        MPLSoUDP encapsulation/decapsulation)

### MACsec Configuration

*   Configure MACsec Static Connectivity Association Key (CAK) Mode on both ends
    of the physical links connecting DUT1 and DUT2:
    *   Define the Policy to cover must-secure scenario.
    *   Use 256-bit cipher GCM-AES-256-XPN.
    *   Set Key server priority: 15.
    *   Set Replay Protection Window size: 64.
    *   Include ICV indicator: True.
    *   Include SCI: True.
    *   Configure keychain with pre-shared keys.

### Policy Forwarding Configuration (Encapsulation)

*   Configure Policy Forwarding on DUT2 ingress interface from DUT1 (for
    encapsulation test cases):
    *   Match incoming traffic (IPv4 and IPv6).
    *   Redirect matched traffic to Next Hop Group.
    *   **For 10G and 100G Paths (MPLSoGRE)**: Next Hop Group
        `MPLS_in_GRE_Encap` pushes MPLS label (e.g., 99998) and encapsulates in
        GRE (tunnel destination `10.99.1.1`, source `10.235.143.208`).
    *   **For 400G Path (MPLSoUDP)**: Next Hop Group `MPLS_in_UDP_Encap` pushes
        MPLS label and encapsulates in UDP (destination port 6635, tunnel
        destination `10.99.1.1`, source `10.235.143.208`).

### Policy Forwarding Configuration (Decapsulation)

*   Configure Policy Forwarding and MPLS on DUT2 ingress interface from ATE (for
    decapsulation test cases):
    *   **For 10G and 100G Paths (MPLSoGRE)**: Match incoming GRE traffic
        (protocol 47) destined to DUT2. Apply action `decapsulate-gre`.
    *   **For 400G Path (MPLSoUDP)**: Match incoming UDP traffic (dest
        port 6635) destined to DUT2. Apply action `decapsulate-gue` (or
        equivalent UDP decap).
    *   Configure Static LSP on DUT2 to match the inner MPLS label (e.g., 99998)
        and perform a **POP** action.
    *   Route the decapsulated and popped IP traffic towards DUT1 via the
        MACsec-secured link.

--------------------------------------------------------------------------------

## Test Cases

### PF-1.27.1 - MPLSoGRE Encap with MACsec and IMIX Traffic (Functional, 10G & 100G)

*   **Traffic Type**: Tested with both IPv4 and IPv6 traffic.
*   **Path**: 10G path (`ATE:port1 -> DUT1 -> DUT2 -> ATE:port2`) and 100G path
    (`ATE:port3 -> DUT1 -> DUT2 -> ATE:port4`).
*   **Procedure**:
    *   Step 1 - Configure MACsec on DUT1 and DUT2 on the interconnecting 10G
        and 100G links.
    *   Step 2 - Configure Policy Forwarding on DUT2 to encapsulate traffic in
        MPLSoGRE.
    *   Step 3 - Verify MACsec session is established and secured.
    *   Step 4 - Generate IPv4 and IPv6 IMIX traffic from ATE (port1/port3)
        destined to a remote IP (routed via DUT1 -> DUT2) at the Maximum
        Non-Drop Rate (NDR). To prevent packet drops due to MACsec overhead on
        the transit links, limit the offered rate to **~92.1%** of physical line
        rate (~8.72 Gbps for 10G ATE Port 1, ~87.2 Gbps for 100G ATE Port 3, for
        a standard 354B average packet size IMIX).
    *   Step 5 - Verify that traffic is received at ATE (port2/port4)
        encapsulated in MPLSoGRE with no packet loss.

### PF-1.27.2 - MACsec Must-Secure Policy Enforcement (Functional, 10G, 100G & 400G)

*   **Traffic Type**: Tested with both IPv4 and IPv6 IMIX traffic.
*   **Path**: Triple-run on 10G path (`ATE:port1 -> DUT1 -> DUT2 -> ATE:port2`),
    100G path (`ATE:port3 -> DUT1 -> DUT2 -> ATE:port4`), and 400G path
    (`ATE:port5 -> DUT1 -> DUT2 -> ATE:port6`).
*   **Procedure**:
    *   Step 1 - Configure MACsec on DUT1 and DUT2 on the interconnecting links
        (10G, 100G, and 400G) with `security-policy` set to `MUST_SECURE`.
    *   Step 2 - Configure Policy Forwarding on DUT2 to encapsulate traffic (GRE
        for 10G/100G, UDP for 400G).
    *   Step 3 - Verify MACsec session is established and secured.
    *   Step 4 - Generate IPv4 and IPv6 IMIX traffic (using a standard 354B
        average packet size IMIX) from ATE (port1/port3/port5) at the Maximum
        Non-Drop Rate (NDR) to prevent queue drops:
        *   For the 10G run: Offer rate at ATE Port 1 limited to **~92.1%**
            (~8.72 Gbps).
        *   For the 100G run: Offer rate at ATE Port 3 limited to **~92.1%**
            (~87.2 Gbps).
        *   For the 400G run: Offer rate at ATE Port 5 limited to **~91.2%**
            (~345.3 Gbps).
    *   Step 5 - Verify that traffic is received at ATE (port2/port4/port6) with
        no packet loss.
    *   Step 6 - Simulating MACsec failure: Modify the pre-shared key (CAK/CKN)
        configuration on DUT2 to introduce a mismatch, bringing the MKA session
        down.
    *   Step 7 - Verify that the MACsec session status on both DUTs transitions
        to DOWN.
    *   Step 8 - Resume traffic generation from ATE.
    *   Step 9 - Verify that **all** traffic is dropped on the DUT1-DUT2 link
        (0% received rate at ATE) due to the `MUST_SECURE` policy.
    *   Step 10 - Restore the correct MACsec keys on DUT2. Verify that the MKA
        session recovers to UP and traffic forwarding resumes with zero packet
        loss.

### PF-1.27.3 - MACsec Should-Secure Policy Enforcement (Functional, 10G, 100G & 400G)

*   **Traffic Type**: Tested with both IPv4 and IPv6 IMIX traffic.
*   **Path**: Triple-run on 10G path (`ATE:port1 -> DUT1 -> DUT2 -> ATE:port2`),
    100G path (`ATE:port3 -> DUT1 -> DUT2 -> ATE:port4`), and 400G path
    (`ATE:port5 -> DUT1 -> DUT2 -> ATE:port6`).
*   **Procedure**:
    *   Step 1 - Configure MACsec on DUT1 and DUT2 on the interconnecting links
        (10G, 100G, and 400G) with `security-policy` set to `SHOULD_SECURE`.
    *   Step 2 - Configure Policy Forwarding on DUT2 to encapsulate traffic (GRE
        for 10G/100G, UDP for 400G).
    *   Step 3 - Verify MACsec session is established and secured.
    *   Step 4 - Generate IPv4 and IPv6 IMIX traffic (using a standard 354B
        average packet size IMIX) from ATE (port1/port3/port5) at the Maximum
        Non-Drop Rate (NDR) to prevent queue drops:
        *   For the 10G run: Offer rate at ATE Port 1 limited to **~92.1%**
            (~8.72 Gbps).
        *   For the 100G run: Offer rate at ATE Port 3 limited to **~92.1%**
            (~87.2 Gbps).
        *   For the 400G run: Offer rate at ATE Port 5 limited to **~91.2%**
            (~345.3 Gbps).
    *   Step 5 - Verify that traffic is received at ATE (port2/port4/port6) with
        no packet loss.
    *   Step 6 - Verify via telemetry/CLI that all packets are transmitted and
        received as encrypted (MACsec-protected).
    *   Step 7 - Simulating MACsec failure: Modify the pre-shared key (CAK/CKN)
        configuration on DUT2 to introduce a mismatch, bringing the MKA session
        down.
    *   Step 8 - Verify that the MACsec session status on both DUTs transitions
        to DOWN.
    *   Step 9 - Resume traffic generation from ATE.
    *   Step 10 - Verify that traffic is **still forwarded** and received at ATE
        with no packet loss, but is transmitted **unencrypted** (cleartext) over
        the DUT1-DUT2 link. Verify that untagged packet counters
        (`tx-untagged-pkts` and `rx-untagged-pkts`) are incrementing.
    *   Step 11 - Restore the correct MACsec keys on DUT2. Verify that the MKA
        session recovers to UP and traffic is again transmitted encrypted.

### PF-1.27.4 - MACsec Security-Association Rekey Timer (Functional, 10G, 100G & 400G)

*   **Traffic Type**: Tested with both IPv4 and IPv6 IMIX traffic.
*   **Path**: Triple-run on 10G path (`ATE:port1 -> DUT1 -> DUT2 -> ATE:port2`),
    100G path (`ATE:port3 -> DUT1 -> DUT2 -> ATE:port4`), and 400G path
    (`ATE:port5 -> DUT1 -> DUT2 -> ATE:port6`).
*   **Procedure**:
    *   Step 1 - Configure MACsec on DUT1 and DUT2 on the interconnecting links
        (10G, 100G, and 400G) with `security-policy` set to `MUST_SECURE` and
        `sak-rekey-interval` set to 28800 seconds.
    *   Step 2 - Configure Policy Forwarding on DUT2 to encapsulate traffic (GRE
        for 10G/100G, UDP for 400G).
    *   Step 3 - Verify the SAK key value is accepted by the DUT (via
        CLI/telemetry).
    *   Step 4 - Verify that MACsec sessions are UP.
    *   Step 5 - Generate IPv4 and IPv6 IMIX traffic (using a standard 354B
        average packet size IMIX) from ATE (port1/port3/port5) at the Maximum
        Non-Drop Rate (NDR) to prevent queue drops:
        *   For the 10G run: Offer rate at ATE Port 1 limited to **~92.1%**
            (~8.72 Gbps).
        *   For the 100G run: Offer rate at ATE Port 3 limited to **~92.1%**
            (~87.2 Gbps).
        *   For the 400G run: Offer rate at ATE Port 5 limited to **~91.2%**
            (~345.3 Gbps).
    *   Step 6 - Verify that traffic is received at ATE (port2/port4/port6) with
        no packet loss.

### PF-1.27.5 - MPLSoUDP Encap with MACsec and IMIX Traffic (Functional, 400G)

*   **Traffic Type**: Tested with both IPv4 and IPv6 traffic.
*   **Path**: 400G path (`ATE:port5 -> DUT1 -> DUT2 -> ATE:port6`).
*   **Procedure**:
    *   Step 1 - Configure MACsec on DUT1 and DUT2 on the interconnecting 400G
        link.
    *   Step 2 - Configure Policy Forwarding on DUT2 to encapsulate traffic in
        MPLSoUDP.
    *   Step 3 - Verify MACsec session is established and secured.
    *   Step 4 - Generate IPv4 and IPv6 IMIX traffic from ATE (port5) destined
        to a remote IP (routed via DUT1 -> DUT2) at the Maximum Non-Drop Rate
        (NDR). To prevent packet drops due to MPLSoUDP encapsulation overhead on
        the 400G egress link, limit the offered rate at ATE Port 5 to **~91.2%**
        of physical line rate (~345.3 Gbps for a standard 354B average packet
        size IMIX).
    *   Step 5 - Verify that traffic is received at ATE (port6) encapsulated in
        MPLSoUDP with no packet loss.

### PF-1.27.6 - MACsec Line Rate Performance with 64B Frames (10G, 100G, 400G)

*   **Traffic Type**: Tested with both IPv4 and IPv6 traffic.
*   **Forwarding Mode**: DUT2 uses encapsulation (MPLSoGRE for 10G and 100G,
    MPLSoUDP for 400G) to forward traffic towards ATE ports 2, 4, and 6.
*   **Procedure**:
    *   Step 1 - Configure MACsec on DUT1 and DUT2 on the interconnecting links
        (10G, 100G, and 400G).
    *   Step 2 - Configure encapsulation on DUT2 (MPLSoGRE for 10G and 100G
        egress, MPLSoUDP for 400G egress).
    *   Step 3 - Generate IPv4 and IPv6 traffic with fixed 64-byte frames from
        the ATE at the Maximum Non-Drop Rate (NDR). Due to bandwidth expansion
        from MACsec and encapsulation overhead, the offered rate at the ATE
        ingress must be limited to prevent egress queue drops:
        *   Test run A: Over 10G path (using MPLSoGRE egress encap). Limit
            offered rate at ATE Port 1 to **~72.4%** of physical line rate
            (~10.77 Mpps) to account for MACsec overhead on the transit link.
        *   Test run B: Over 100G path (using MPLSoGRE egress encap). Limit
            offered rate at ATE Port 3 to **~72.4%** of physical line rate
            (~107.7 Mpps) to account for MACsec overhead on the transit link.
        *   Test run C: Over 400G path (using MPLSoUDP egress encap). Limit
            offered rate at ATE Port 5 to **~70.0%** of physical line rate
            (~416.6 Mpps) to account for MPLSoUDP overhead on the egress link.
    *   Step 4 - Verify that no packet loss occurs over a 10-minute duration for
        each run.
    *   Step 5 - Validate that throughput matches the expected line rate for 64B
        frames (accounting for MACsec and respective encapsulation overhead).

### PF-1.27.7 - MACsec Line Rate Performance with IMIX Traffic (10G, 100G, 400G)

*   **Traffic Type**: Tested with both IPv4 and IPv6 traffic.
*   **Forwarding Mode**: DUT2 uses encapsulation (MPLSoGRE for 10G and 100G,
    MPLSoUDP for 400G) to forward traffic towards ATE ports 2, 4, and 6.
*   **Procedure**:
    *   Step 1 - Maintain the MACsec and encapsulation configuration from
        PF-1.27.6.
    *   Step 2 - Generate IPv4 and IPv6 traffic using an IMIX profile (e.g., a
        mix of 64B, 570B, and 1518B) at the Maximum Non-Drop Rate (NDR). Due to
        overhead, the offered rate at the ATE ingress must be limited:
        *   Test run A: Over 10G path (using MPLSoGRE egress encap). Limit
            offered rate to **~92.1%** of physical line rate (~8.72 Gbps for a
            standard 354B average packet size IMIX) to account for MACsec
            transit link overhead.
        *   Test run B: Over 100G path (using MPLSoGRE egress encap). Limit
            offered rate to **~92.1%** of physical line rate (~87.2 Gbps for a
            standard 354B average packet size IMIX) to account for MACsec
            transit link overhead.
        *   Test run C: Over 400G path (using MPLSoUDP egress encap). Limit
            offered rate to **~91.2%** of physical line rate (~345.3 Gbps for a
            standard 354B average packet size IMIX) to account for MPLSoUDP
            egress link overhead.
    *   Step 3 - Verify zero packet loss and consistent throughput for each run.

### PF-1.27.8 - MACsec Line Rate Performance with Jumbo Frames (10G, 100G, 400G)

*   **Traffic Type**: Tested with both IPv4 and IPv6 traffic.
*   **Forwarding Mode**: DUT2 uses encapsulation (MPLSoGRE for 10G and 100G,
    MPLSoUDP for 400G) to forward traffic towards ATE ports 2, 4, and 6.
*   **Procedure**:
    *   Step 1 - Configure the DUT1<->DUT2 interfaces and DUT2 egress interfaces
        to support a MTU of 9216 bytes.
    *   Step 2 - Generate IPv4 and IPv6 traffic with 9000-byte Jumbo frames at
        the Maximum Non-Drop Rate (NDR). Due to the very large packet size, the
        relative impact of the overhead is very small (<0.4%):
        *   Test run A: Over 10G path (using MPLSoGRE egress encap). Limit
            offered rate to **~99.6%** of physical line rate (~9.94 Gbps) to
            account for MACsec transit link overhead.
        *   Test run B: Over 100G path (using MPLSoGRE egress encap). Limit
            offered rate to **~99.6%** of physical line rate (~99.4 Gbps) to
            account for MACsec transit link overhead.
        *   Test run C: Over 400G path (using MPLSoUDP egress encap). Limit
            offered rate to **~99.6%** of physical line rate (~398.4 Gbps) to
            account for MPLSoUDP egress link overhead.
    *   Step 3 - Verify that the hardware correctly handles large encrypted and
        encapsulated payloads without fragmentation or loss for each run.

### PF-1.27.9 - MPLSoGRE Decap and Label Pop with MACsec (Functional, 10G & 100G)

*   **Traffic Type**: Tested with both IPv4 and IPv6 IMIX traffic.
*   **Path**: 10G path (`ATE:port2 -> DUT2 -> DUT1 -> ATE:port1`) and 100G path
    (`ATE:port4 -> DUT2 -> DUT1 -> ATE:port3`).
*   **Procedure**:
    *   Step 1 - Configure MACsec on DUT1 and DUT2 on the interconnecting 10G
        and 100G links.
    *   Step 2 - Configure GRE decapsulation policy and static LSP with POP
        action on DUT2.
    *   Step 3 - Verify MACsec session is established and secured.
    *   Step 4 - Generate MPLSoGRE encapsulated traffic from ATE (port2/port4)
        destined to DUT2 decap IP (with inner payload destined to ATE
        (port1/port3) via DUT1) at the Maximum Non-Drop Rate (NDR). To prevent
        packet drops due to MACsec overhead on the transit link (which is larger
        than GRE overhead), limit the offered rate to **~99.0%** of physical
        line rate (~9.41 Gbps for 10G ATE Port 2, ~94.1 Gbps for 100G ATE Port
        4, for a standard 354B average inner packet size IMIX).
    *   Step 5 - Verify that decapsulated and decrypted traffic is received at
        ATE (port1/port3) with no packet loss.

### PF-1.27.10 - MPLSoUDP Decap and Label Pop with MACsec (Functional, 400G)

*   **Traffic Type**: Tested with both IPv4 and IPv6 IMIX traffic.
*   **Path**: 400G path (`ATE:port6 -> DUT2 -> DUT1 -> ATE:port5`).
*   **Procedure**:
    *   Step 1 - Configure MACsec on DUT1 and DUT2 on the interconnecting 400G
        link.
    *   Step 2 - Configure UDP decapsulation policy and static LSP with POP
        action on DUT2.
    *   Step 3 - Verify MACsec session is established and secured.
    *   Step 4 - Generate MPLSoUDP encapsulated traffic from ATE (port6)
        destined to DUT2 decap IP (with inner payload destined to ATE (port5)
        via DUT1). Since the UDP encapsulation overhead at ingress is larger
        than the downstream MACsec overhead, the packets shrink as they traverse
        the path. Thus, traffic can be sent at **100%** of the physical line
        rate of the encapsulated traffic (~380.5 Gbps L2 throughput for a
        standard 354B average inner packet size IMIX) without causing packet
        loss.
    *   Step 5 - Verify that decapsulated and decrypted traffic is received at
        ATE (port5) with no packet loss.

### PF-1.27.11 - MACsec Hitless Key Rotation (Functional, 10G, 100G & 400G)

*   **Traffic Type**: Tested with both IPv4 and IPv6 IMIX traffic.
*   **Path**: Triple-run on 10G path (`ATE:port1 -> DUT1 -> DUT2 -> ATE:port2`),
    100G path (`ATE:port3 -> DUT1 -> DUT2 -> ATE:port4`), and 400G path
    (`ATE:port5 -> DUT1 -> DUT2 -> ATE:port6`).
*   **Procedure**:
    *   Step 1 - Configure MACsec on DUT1 and DUT2 on the interconnecting links
        (10G, 100G, and 400G).
    *   Step 2 - Configure a MACsec keychain on DUT1 and DUT2 with 5 unique keys
        (Key ID 01 through 05), each with a distinct, staggered activation time
        (e.g., staggered by 10 minutes).
    *   Step 3 - Apply the keychain to the MACsec profile on the 10G, 100G, and
        400G links using cipher GCM-AES-256-XPN.
    *   Step 4 - Generate IPv4 and IPv6 IMIX traffic (using a standard 354B
        average packet size IMIX) from ATE (port1/port3/port5) through DUT1 and
        DUT2 towards ATE (port2/port4/port6) at the Maximum Non-Drop Rate (NDR)
        to prevent queue drops:
        *   For the 10G run: Limit the offered rate at ATE Port 1 to **~92.1%**
            of physical line rate (~8.72 Gbps) to account for MACsec transit
            link overhead.
        *   For the 100G run: Limit the offered rate at ATE Port 3 to **~92.1%**
            of physical line rate (~87.2 Gbps) to account for MACsec transit
            link overhead.
        *   For the 400G run: Limit the offered rate at ATE Port 5 to **~91.2%**
            of physical line rate (~345.3 Gbps) to account for MPLSoUDP egress
            link overhead.
    *   Step 5 - Monitor ATE receivers for sequence errors or packet loss during
        the key transition windows.
    *   Step 6 - Manually trigger a key rollover by updating the MKA primary-key
        or allow the activation timers to expire.
    *   Step 7 - Verify the active key transition using CLI (`show macsec mka
        session interface <interface> detail`) and gNMI telemetry
        (`/macsec/mka/interfaces/interface/state/active-key-id`).
*   **Expected Result**:
    *   The platform must rotate through all 5 keys in the keychain hitlessly.
    *   Traffic must remain at line-rate with zero packet loss (0% drop) during
        every key transition.
    *   The MACsec session must remain stable across the transitions.



--------------------------------------------------------------------------------

## Canonical OC

```json
{
  "openconfig-keychain:keychains": {
    "keychain": [
      {
        "name": "macsec_keychain",
        "config": {
          "name": "macsec_keychain"
        },
        "keys": {
          "key": [
            {
              "key-id": "0xabcd111122223333444455556666777788889999000011112222333344445555",
              "config": {
                "key-id": "0xabcd111122223333444455556666777788889999000011112222333344445555",
                "secret-key": "ad4rf10kn85fc0adk5dfcsnr1or4cm08q",
                "crypto-algorithm": "AES_256_CMAC"
              }
            }
          ]
        }
      }
    ]
  },
  "openconfig-macsec:macsec": {
    "interfaces": {
      "interface": [
        {
          "name": "Ethernet1/1",
          "config": {
            "name": "Ethernet1/1",
            "enable": true,
            "replay-protection": 64
          },
          "mka": {
            "config": {
              "key-chain": "macsec_keychain",
              "mka-policy": "must_secure_policy"
            }
          }
        }
      ]
    },
    "mka": {
      "policies": {
        "policy": [
          {
            "name": "must_secure_policy",
            "config": {
              "name": "must_secure_policy",
              "key-server-priority": 15,
              "macsec-cipher-suite": [
                "GCM_AES_XPN_256"
              ],
              "confidentiality-offset": "0_BYTES",
              "include-icv-indicator": true,
              "include-sci": true,
              "sak-rekey-interval": 30
            }
          },
          {
            "name": "should_secure_policy",
            "config": {
              "name": "should_secure_policy",
              "key-server-priority": 15,
              "macsec-cipher-suite": [
                "GCM_AES_XPN_256"
              ],
              "confidentiality-offset": "0_BYTES",
              "include-icv-indicator": true,
              "include-sci": true,
              "sak-rekey-interval": 30
            }
          }
        ]
      }
    }
  },
  "openconfig-mpls:mpls": {
    "lsps": {
      "static-lsps": {
        "static-lsp": [
          {
            "name": "static-lsp-pop",
            "config": {
              "name": "static-lsp-pop"
            },
            "egress": {
              "config": {
                "incoming-label": 99998,
                "next-hop": "192.0.2.1",
                "push-label": 3
              }
            }
          }
        ]
      }
    }
  },
  "openconfig-network-instance:network-instances": {
    "network-instance": [
      {
        "name": "default",
        "config": {
          "name": "default",
          "type": "openconfig-network-instance-types:DEFAULT_INSTANCE"
        },
        "policy-forwarding": {
          "interfaces": {
            "interface": [
              {
                "interface-id": "Ethernet1/1.20",
                "config": {
                  "interface-id": "Ethernet1/1.20",
                  "apply-forwarding-policy": "customer1"
                }
              }
            ]
          },
          "policies": {
            "policy": [
              {
                "policy-id": "customer1",
                "config": {
                  "policy-id": "customer1",
                  "type": "PBR_POLICY"
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
                          "protocol": 47
                        }
                      },
                      "action": {
                        "config": {
                          "next-hop-group": "MPLS_in_GRE_Encap"
                        }
                      }
                    }
                  ]
                }
              },
              {
                "policy-id": "decap-policy",
                "config": {
                  "policy-id": "decap-policy",
                  "type": "PBR_POLICY"
                },
                "rules": {
                  "rule": [
                    {
                      "sequence-id": 10,
                      "config": {
                        "sequence-id": 10
                      },
                      "ipv4": {
                        "config": {
                          "protocol": 47
                        }
                      },
                      "action": {
                        "config": {
                          "decapsulate-gre": true
                        }
                      }
                    },
                    {
                      "sequence-id": 20,
                      "config": {
                        "sequence-id": 20
                      },
                      "ipv4": {
                        "config": {
                          "protocol": 17
                        }
                      },
                      "action": {
                        "config": {
                          "decapsulate-gue": true
                        }
                      }
                    }
                  ]
                }
              }
            ]
          },
          "path-selection-groups": {
            "path-selection-group": [
              {
                "group-id": "MPLS_in_GRE_Encap",
                "config": {
                  "group-id": "MPLS_in_GRE_Encap"
                }
              },
              {
                "group-id": "MPLS_in_UDP_Encap",
                "config": {
                  "group-id": "MPLS_in_UDP_Encap"
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
  config:
    /macsec/interfaces/interface/config/enable:
    /macsec/interfaces/interface/config/replay-protection:
    /macsec/mka/policies/policy/config/name:
    /macsec/mka/policies/policy/config/security-policy:
    /macsec/mka/policies/policy/config/macsec-cipher-suite:
    /macsec/mka/policies/policy/config/confidentiality-offset:
    /macsec/mka/policies/policy/config/key-server-priority:
    /macsec/mka/policies/policy/config/sak-rekey-interval:
    /keychains/keychain/keys/key/config/secret-key:
    /keychains/keychain/keys/key/config/crypto-algorithm:
    /interfaces/interface/config/description:
    /interfaces/interface/config/enabled:
    /interfaces/interface/config/name:
    /interfaces/interface/hold-time/config/up:
    /interfaces/interface/hold-time/config/down:
    /interfaces/interface/subinterfaces/subinterface/config/index:
    /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
    /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
    /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
    /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
    /interfaces/interface/subinterfaces/subinterface/vlan/config/vlan-id:
    /lacp/interfaces/interface/config/name:
    /lacp/interfaces/interface/config/lacp-mode:
    /network-instances/network-instance/config/name:
    /network-instances/network-instance/config/type:
    /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix:
    /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/index:
    /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop:
    /network-instances/network-instance/policy-forwarding/policies/policy/config/policy-id:
    /network-instances/network-instance/policy-forwarding/interfaces/interface/config/interface-id:
    /network-instances/network-instance/policy-forwarding/interfaces/interface/config/apply-forwarding-policy:
    /network-instances/network-instance/policy-forwarding/next-hop-groups/next-hop-group/next-hops/next-hop/encap-header/udp-v4/config/dst-udp-port:
    /network-instances/network-instance/policy-forwarding/policies/policy/config/type:
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decapsulate-gre:
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decapsulate-gue:
    /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/config/name:
    /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/config/incoming-label:
    /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/config/next-hop:
    /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/config/push-label:
  state:
    /macsec/mka/interfaces/interface/state/active-key-id:
    /macsec/mka/policies/policy/state/security-policy:
    /macsec/interfaces/interface/state/counters/rx-badtag-pkts:
    /macsec/interfaces/interface/state/counters/rx-late-pkts:
    /macsec/interfaces/interface/state/counters/rx-nosci-pkts:
    /macsec/interfaces/interface/state/counters/rx-unknownsci-pkts:
    /macsec/interfaces/interface/state/counters/rx-untagged-pkts:
    /macsec/interfaces/interface/state/counters/tx-untagged-pkts:
    /macsec/interfaces/interface/mka/state/counters/in-cak-mkpdu:
    /macsec/interfaces/interface/mka/state/counters/in-mkpdu:
    /macsec/interfaces/interface/mka/state/counters/in-sak-mkpdu:
    /macsec/interfaces/interface/mka/state/counters/out-cak-mkpdu:
    /macsec/interfaces/interface/mka/state/counters/out-mkpdu:
    /macsec/interfaces/interface/mka/state/counters/out-sak-mkpdu:
    /macsec/mka/state/counters/in-mkpdu-bad-peer-errors:
    /macsec/mka/state/counters/in-mkpdu-icv-verification-errors:
    /macsec/mka/state/counters/in-mkpdu-peer-list-errors:
    /macsec/mka/state/counters/in-mkpdu-validation-errors:
    /macsec/mka/state/counters/out-mkpdu-errors:
    /macsec/mka/state/counters/sak-cipher-mismatch-errors:
    /macsec/mka/state/counters/sak-decryption-errors:
    /macsec/mka/state/counters/sak-encryption-errors:
    /macsec/mka/state/counters/sak-generation-errors:
    /macsec/mka/state/counters/sak-hash-errors:
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
      sampled: true
```

### Required DUT platform

*   FFF - Fixed Form Factor
*   MFF - Modular Form Factor