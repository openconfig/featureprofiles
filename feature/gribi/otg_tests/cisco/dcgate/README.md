# DCGate Test Suite

This directory contains the DCGate vendor test suite implementing various encapsulation and traffic engineering test cases for Cisco devices.

## Overview

The DCGate test suite validates the functionality of IP-in-IP encapsulation, ECMP (Equal-Cost Multi-Path) routing, and traffic engineering features on Cisco networking devices. It implements test cases as defined in the TE-16.1 vendor testplan.

Note:
https://miggbo.atlassian.net/wiki/x/1g3TPQ

## Test Files

### Core Test Files

- **`dcgate_base_test.go`** - Base test infrastructure and common utilities
- **`dcgate_encap_test.go`** - Basic encapsulation functionality tests
- **`dcgate_frr_test.go`** - Fast Re-Route (FRR) and backup path tests
- **`dcgate_regionalization_test.go`** - Regionalization and VRF-specific tests

### Configuration Files

- **`metadata.textproto`** - Platform-specific configuration and deviations

## Test Categories

### 1. Basic Encapsulation Tests (`TestBasicEncap`)

The core encapsulation test suite validates fundamental IP-in-IP tunneling functionality with 16 comprehensive test cases:

#### IPv4/IPv6 WCMP Traffic Tests
- **IPv4 Traffic WCMP Encap (DSCP 10)** - Tests weighted cost multi-path load balancing for IPv4 traffic with DSCP marking 10
- **IPv4 Traffic WCMP Encap (DSCP 20)** - Tests weighted cost multi-path load balancing for IPv4 traffic with DSCP marking 20
- **IPv6 Traffic WCMP Encap (DSCP 10)** - Tests weighted cost multi-path load balancing for IPv6 traffic with DSCP marking 10
- **IPv6 Traffic WCMP Encap (DSCP 20)** - Tests weighted cost multi-path load balancing for IPv6 traffic with DSCP marking 20

#### Advanced Encapsulation Tests
- **IPinIP Traffic WCMP Encap** - Tests dual-stack IP-in-IP tunneling (both IPv4-in-IPv4 and IPv6-in-IPv4) with proper traffic distribution
- **No Match DSCP Traffic** - Validates behavior when traffic doesn't match configured DSCP classification rules
- **IPv4 No Prefix In Encap VRF** - Tests fallback behavior when destination prefix doesn't exist in encapsulation VRF
- **IPv6 No Prefix In Encap VRF** - Tests fallback behavior for IPv6 when destination prefix doesn't exist in encapsulation VRF

#### Routing and Recovery Tests
- **Basic Default Route Installation** - Validates basic default route configuration and traffic forwarding to single destination
- **Next-hop Unavailability Recirculation** - Tests failover behavior when primary next-hops become unavailable (ports 3&4 shutdown)
- **LOOKUP NH Backup NHG** - Verifies backup next-hop group functionality with LOOKUP next-hop actions
- **Default Route Lookup Non-Default VRF** - Tests cross-VRF lookup functionality for default routes
- **Process Recovery** - Validates system resilience during EMSD (Enhanced Multicast Service Daemon) process restart
- **Default Route Modification** - Tests dynamic modification of default route configurations

#### Advanced Validation Tests
- **VRF Scale Testing** - Validates scalability and performance with multiple VRF configurations
- **NHG Update Ignored For Existing Chain** - Verifies that next-hop group updates are properly ignored for existing FIB chains
- **Backup NHG LOOKUP Action Maintained** - Ensures backup next-hop groups maintain LOOKUP actions correctly
- **NHG Maintains Non-LOOKUP Action After Switchover** - Validates action preservation during backup path activation

### 2. Fast Re-Route (FRR) Tests (`TestEncapFrr`)

Advanced failover and backup path validation with 7 specialized test cases:

#### Single-Hop FRR Tests
- **EncapWithVIPHavingBackupNHG** - Tests encapsulation with Virtual IP having backup next-hop groups
- **SSFRRTunnelPrimaryPathUnviable** - Single-hop FRR when primary tunnel path becomes unavailable
- **SSFRRTunnelPrimaryAndBackupPathUnviable** - Behavior when both primary and backup paths fail simultaneously
- **SSFRRTunnelPrimaryAndBackupPathUnviableForAllTunnel** - Complete tunnel failure scenario testing

#### Scalable FRR Tests  
- **SFRRBackupNHGTunneltoPrimaryTunnelWhenPrimaryTunnelUnviable** - Scalable FRR backup tunnel to primary tunnel transition
- **SFRRPrimaryBackupNHGforTunnelUnviable** - Primary and backup next-hop group failure scenarios
- **SFRRPrimaryPathUnviableWithooutBNHG** - Primary path failure without backup next-hop group configuration

### 3. Regionalization Tests (`TestRegionalization`)

Geographic and administrative traffic segmentation validation:

- **Cross-Region Traffic Isolation** - Ensures traffic stays within designated regions
- **Region-Specific Policy Application** - Validates region-based policy enforcement
- **Inter-Region Communication Controls** - Tests controlled communication between regions
- **Regional Failover Mechanisms** - Region-specific backup and recovery procedures

### 4. FIB Chain Optimization Tests (`TestFibChains`)

Forwarding Information Base chain optimization and validation with 6 test scenarios:

#### Optimization Tests
- **EncapDcgateOptimized** - Tests optimized encapsulation FIB chain programming
- **TransitDcgateOptimized** - Validates optimized transit traffic FIB chains
- **PopGateOptimized** - Tests optimized pop-gate (decapsulation) FIB chains

#### Unoptimized Baseline Tests
- **TransitDcgateUnoptimized** - Baseline unoptimized transit traffic validation
- **PopGateUnOptimized** - Baseline unoptimized pop-gate functionality

#### Advanced FRR
- **FRRRecycle** - Fast Re-Route recycling and recovery testing

## Key Features Tested

**Weight Distribution Analysis:**
- **Tunnel 1 Total**: 25.11% (6.27% + 18.84%) - 1:3 ratio between ports 2&3
- **Tunnel 2 Total**: 74.89% (30.10% + 44.79%) - 2:3 ratio between ports 4&5
- **Overall Split**: ~25:75 between tunnel groups
- **Hierarchical ECMP**: Two-level load balancing (tunnel selection + intra-tunnel distribution)

### Advanced Encapsulation Types

#### Primary Encapsulation Modes
- **IPv4-in-IPv4 Tunneling** 
  - Outer Header: IPv4 with source `ipv4OuterSrc111` 
  - Inner Payload: Original IPv4 packets
  - TTL Handling: Preserves inner TTL, decrements outer TTL
  - Fragmentation: Supports path MTU discovery

- **IPv6-in-IPv4 Tunneling**
  - Outer Header: IPv4 encapsulation for IPv6 payload
  - Protocol: IP protocol 41 (IPv6-in-IPv4)
  - Hop Limit Mapping: IPv6 hop limit to IPv4 TTL conversion
  - Extension Header Support: Preserves IPv6 extension headers

- **Multi-Protocol Support**
  - Dual-stack operation (IPv4 and IPv6 simultaneously)
  - Protocol-specific DSCP marking preservation
  - QoS inheritance from inner to outer headers


## VRF Configuration and Architecture

The test suite implements a sophisticated multi-VRF architecture supporting complex traffic engineering scenarios:

### Core VRF Instances

#### Encapsulation VRFs
- **ENCAP_TE_VRF_A** (`vrfEncapA`)
  - Purpose: Primary encapsulation domain for tunnel group A
  - Traffic Types: IPv4 and IPv6 customer traffic requiring encapsulation
  - Tunnel Destinations: `tunnelDstIP1` and `tunnelDstIP2`
  - DSCP Classifications: Handles dscpEncapA1 (10) and dscpEncapA2 (18)

- **ENCAP_TE_VRF_B** (`vrfEncapB`) 
  - Purpose: Secondary encapsulation domain for tunnel group B
  - Traffic Types: Backup and overflow traffic from VRF A
  - Tunnel Destinations: Same as VRF A but with different policies
  - DSCP Classifications: Handles dscpEncapB1 (20) and dscpEncapB2 (28)

#### Transit and Transport VRFs  
- **TE_VRF_111** (`vrfTransit`)
  - Purpose: Intermediate transit VRF for tunnel transport
  - Functionality: Routes encapsulated traffic between tunnel endpoints
  - Next-hop Resolution: Resolves tunnel destinations to physical interfaces
  - Load Balancing: Implements WCMP across tunnel infrastructure

- **TE_VRF_222** (`vrfRepaired`) 
  - Purpose: Post-failure repair and recovery VRF
  - Functionality: Handles traffic after FRR (Fast Re-Route) events
  - DecapEncap Operations: Combined decapsulation and re-encapsulation
  - Failover Support: Backup path for critical traffic flows

#### Specialized Processing VRFs
- **DECAP_TE_VRF** (`vrfDecap`)
  - Purpose: Ingress decapsulation and classification
  - Operations: Removes outer tunnel headers, classifies inner traffic
  - Policy Application: Applies ingress policies based on inner packet attributes
  - VRF Selection: Determines target VRF for decapsulated traffic

- **DECAP** (`vrfDecapPostRepaired`)
  - Purpose: Post-repair decapsulation for recovered traffic
  - Functionality: Handles decapsulation after FRR recovery
  - Traffic Restoration: Restores normal forwarding after failures

- **REPAIR_VRF** (`vrfRepair`)
  - Purpose: Temporary holding VRF during repair operations
  - Usage: Stores traffic state during topology changes
  - Recovery Operations: Facilitates hitless failover procedures

### VRF Interaction Patterns

#### Cross-VRF Lookup Operations
```go
// Default route lookup in non-default VRF
c.AddNH(t, defaultRouteNHID, "VRFOnly", defaultVRF, &gribi.NHOptions{VrfName: defaultVRF})
c.AddIPv4(t, "0.0.0.0/0", defaultRouteNHGID, vrfEncapA, defaultVRF)
```

#### Hierarchical VRF Resolution
1. **Traffic Ingress** → Classification VRF determines encapsulation requirements
2. **Encapsulation Selection** → ENCAP_TE_VRF_A/B based on DSCP and policy
3. **Tunnel Transport** → TE_VRF_111 for inter-domain routing
4. **Egress Processing** → DECAP_TE_VRF for tunnel termination


## Test Infrastructure and Architecture

### OTG (Open Traffic Generator) Configuration

#### Port Architecture
- **Source Port**: `port1` 
  - Role: Primary traffic generation interface
  - Capabilities: Multi-protocol traffic generation (IPv4, IPv6, IPinIP)
  - Traffic Types: Supports all DSCP classifications and protocol combinations
  - Rate Control: Configurable traffic rates and burst patterns

- **Destination Ports**: `port2`, `port3`, `port4`, `port5`
  - **port2**: Tunnel 1, Lower weight (6.27% traffic share)
  - **port3**: Tunnel 1, Higher weight (18.84% traffic share) 
  - **port4**: Tunnel 2, Lower weight (30.10% traffic share)
  - **port5**: Tunnel 2, Higher weight (44.79% traffic share)

#### Advanced Traffic Generation
- **Flow Diversity**: Multiple concurrent flows with different characteristics
- **Protocol Stack Support**: IPv4, IPv6, IPinIP, UDP encapsulation
- **DSCP Marking**: Precise DSCP value assignment per flow
- **Packet Capture**: Multi-port simultaneous capture and analysis

#### Capture and Analysis Infrastructure
```go
// Multi-port capture support detection
if otgMutliPortCaptureSupported {
    enableCapture(t, otg.OTG(), topo, tc.capturePorts)
    // Simultaneous capture across all destination ports
} else {
    // Sequential single-port capture for older platforms
    for _, port := range tc.capturePorts {
        enableCapture(t, otg.OTG(), topo, []string{port})
    }
}
```

### gRIBI Integration and FIB Programming

#### gRIBI Configuration Architecture

```
                                    gRIBI Configuration Flow
    ┌─────────────────────────────────────────────────────────────────────────────────────┐
    │                                DUT (Device Under Test)                              │
    │                                                                                     │
    │  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐                │
    │  │   ENCAP_TE_VRF_A│    │   TE_VRF_111    │    │  Default VRF    │                │
    │  │   (vrfEncapA)   │    │  (vrfTransit)   │    │                 │                │
    │  │                 │    │                 │    │                 │                │
    │  │ IPv4Entry──────┐│    │ VIP1 ──────────┐│    │ NH10 ────port2  │                │
    │  │ (nhg10)        ││    │ (nhg2)         ││    │ NH11 ────port3  │                │
    │  │                ││    │                ││    │ NH100────port4  │                │
    │  │ IPv6Entry──────┘│    │ VIP2 ──────────┘│    │ NH101────port5  │                │
    │  │ (nhg10)         │    │ (nhg3)          │    │                 │                │
    │  │                 │    │                 │    │                 │                │
    │  │ DefaultRoute────┼────│                 │    │                 │                │
    │  │ (Lookup to      │    │                 │    │                 │                │
    │  │  Default VRF)   │    │                 │    │                 │                │
    │  └─────────────────┘    └─────────────────┘    └─────────────────┘                │
    │           │                       │                       │                        │
    │           │                       │                       │                        │
    │  ┌─────────────────┐    ┌─────────────────┐               │                        │
    │  │   ENCAP_TE_VRF_B│    │                 │               │                        │
    │  │   (vrfEncapB)   │    │   Encap NHs     │               │                        │
    │  │                 │    │                 │               │                        │
    │  │ IPv4Entry──────┐│    │ NH201 (Encap)───┼───────────────┼────► tunnelDstIP1     │
    │  │ (nhg10)        ││    │ Src: 111.x.x.x │               │      (via TE_VRF_111) │
    │  │                ││    │ Dst: tunDst1    │               │                        │
    │  │ IPv6Entry──────┘│    │                 │               │                        │
    │  │ (nhg10)         │    │ NH202 (Encap)───┼───────────────┼────► tunnelDstIP2     │
    │  └─────────────────┘    │ Src: 111.x.x.x │               │      (via TE_VRF_111) │
    │                         │ Dst: tunDst2    │               │                        │
    │                         └─────────────────┘               │                        │
    └─────────────────────────────────────────────────────────────────────────────────────┘
                                         │
    ┌─────────────────────────────────────────────────────────────────────────────────────┐
    │                                ATE (Traffic Generator)                              │
    │                                                                                     │
    │                    ┌─────────────────────────────────────────┐                    │
    │                    │              Traffic Flows              │                    │
    │                    │                                         │                    │
    │  port1 ──────────► │  IPv4 (DSCP 10) ──────────────────────► │ ──────► port2/3/4/5│
    │  (Source)          │  IPv6 (DSCP 10,20) ───────────────────► │         (Capture)  │
    │                    │  IPinIP (DSCP 10) ────────────────────► │                    │
    │                    │  UDP (No Match DSCP) ─────────────────► │                    │
    │                    │                                         │                    │
    │                    └─────────────────────────────────────────┘                    │
    └─────────────────────────────────────────────────────────────────────────────────────┘


```

#### Packet Flow Diagram

```
                        DCGate Encapsulation Packet Flow
    ┌─────────────────────────────────────────────────────────────────────────────────────┐
    │                              Traffic Generator (ATE)                                │
    │                                                                                     │
    │  ┌─────────┐     ┌─────────────────────────────────────────────────────────────┐   │
    │  │ port1   │────►│  Generated Flows:                                           │   │
    │  │(Source) │     │  • IPv4 (DSCP 10/20) → 198.51.100.0/24                    │   │
    │  │         │     │  • IPv6 (DSCP 10/20) → 2001:db8::/64                      │   │
    │  │         │     │  • IPinIP (DSCP 10)  → Multi-protocol tunneling           │   │
    │  │         │     │  • UDP (No Match)    → Default routing                     │   │
    │  └─────────┘     └─────────────────────────────────────────────────────────────┘   │
    └─────────────────────────────────────────────────────────────────────────────────────┘
                                           │
                                           ▼
    ┌─────────────────────────────────────────────────────────────────────────────────────┐
    │                             DUT Ingress Processing                                 │
    │                                                                                     │
    │  ┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐             │
    │  │   DSCP Lookup   │      │  VRF Selection  │      │ Policy Matching │             │
    │  │                 │      │                 │      │                 │             │
    │  │ DSCP 10 ────────┼─────►│ ENCAP_TE_VRF_A │◄─────┤ IPv4/IPv6 Match │             │
    │  │ DSCP 20 ────────┼─────►│ ENCAP_TE_VRF_B │      │                 │             │
    │  │ Other   ────────┼─────►│ Default VRF    │      │ No Match → Drop │             │
    │  └─────────────────┘      └─────────────────┘      └─────────────────┘             │
    └─────────────────────────────────────────────────────────────────────────────────────┘
                                           │
                                           ▼
    ┌─────────────────────────────────────────────────────────────────────────────────────┐
    │                          Encapsulation VRF Processing                              │
    │                                                                                     │
    │  ┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐             │
    │  │ ENCAP_TE_VRF_A  │      │ Route Lookup    │      │   Tunnel NHG    │             │
    │  │                 │      │                 │      │                 │             │
    │  │ 198.51.100.0/24 ┼─────►│ Match Prefix    ┼─────►│ NHG10 (1:3)     │             │
    │  │ 2001:db8::/64   │      │ → NHG10         │      │ NH201 → Tunnel1 │             │
    │  │ 0.0.0.0/0       │      │ Default → VRF   │      │ NH202 → Tunnel2 │             │
    │  │ (Lookup)        │      │ Lookup          │      │                 │             │
    │  └─────────────────┘      └─────────────────┘      └─────────────────┘             │
    └─────────────────────────────────────────────────────────────────────────────────────┘
                                           │
                                           ▼
    ┌─────────────────────────────────────────────────────────────────────────────────────┐
    │                            Tunnel Encapsulation                                    │
    │                                                                                     │
    │  ┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐             │
    │  │   NH201 (25%)   │      │   NH202 (75%)   │      │  Encap Headers  │             │
    │  │                 │      │                 │      │                 │             │
    │  │ Outer Src:      │      │ Outer Src:      │      │ IP Protocol: 4  │             │
    │  │ 111.x.x.x       │      │ 111.x.x.x       │      │ (IPv4-in-IPv4)  │             │
    │  │ Outer Dst:      │      │ Outer Dst:      │      │ or 41 (IPv6-in- │             │
    │  │ tunnelDstIP1    │      │ tunnelDstIP2    │      │ IPv4)           │             │
    │  │ VRF: TE_VRF_111 │      │ VRF: TE_VRF_111 │      │ TTL: 64         │             │
    │  └─────────────────┘      └─────────────────┘      └─────────────────┘             │
    └─────────────────────────────────────────────────────────────────────────────────────┘
                                           │
                                           ▼
    ┌─────────────────────────────────────────────────────────────────────────────────────┐
    │                            Transit VRF Processing                                  │
    │                                                                                     │
    │  ┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐             │
    │  │   TE_VRF_111    │      │  VIP Resolution │      │ Physical Egress │             │
    │  │                 │      │                 │      │                 │             │
    │  │ tunnelDstIP1/32 ┼─────►│ VIP1 → NHG2     ┼─────►│ NH10:NH11 (1:3) │             │
    │  │ → NHG2          │      │ NH10/NH11       │      │ port2:port3     │             │
    │  │                 │      │                 │      │                 │             │
    │  │ tunnelDstIP2/32 ┼─────►│ VIP2 → NHG3     ┼─────►│ NH100:NH101(2:3)│             │
    │  │ → NHG3          │      │ NH100/NH101     │      │ port4:port5     │             │
    │  └─────────────────┘      └─────────────────┘      └─────────────────┘             │
    └─────────────────────────────────────────────────────────────────────────────────────┘
                                           │
                                           ▼
    ┌─────────────────────────────────────────────────────────────────────────────────────┐
    │                          Final Traffic Distribution                                 │
    │                                                                                     │
    │  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐   │
    │  │     port2       │ │     port3       │ │     port4       │ │     port5       │   │
    │  │    6.27%        │ │    18.84%       │ │    30.10%       │ │    44.79%       │   │
    │  │                 │ │                 │ │                 │ │                 │   │
    │  │ ┌─────────────┐ │ │ ┌─────────────┐ │ │ ┌─────────────┐ │ │ ┌─────────────┐ │   │
    │  │ │Encap Packet │ │ │ │Encap Packet │ │ │ │Encap Packet │ │ │ │Encap Packet │ │   │
    │  │ │Outer:111→IP1│ │ │ │Outer:111→IP1│ │ │ │Outer:111→IP2│ │ │ │Outer:111→IP2│ │   │
    │  │ │Inner:Original│ │ │ │Inner:Original│ │ │ │Inner:Original│ │ │ │Inner:Original│ │   │
    │  │ └─────────────┘ │ │ └─────────────┘ │ │ └─────────────┘ │ │ └─────────────┘ │   │
    │  └─────────────────┘ └─────────────────┘ └─────────────────┘ └─────────────────┘   │
    │           │                    │                    │                    │         │
    │           ▼                    ▼                    ▼                    ▼         │
    │  ┌─────────────────┴─────────────────┐    ┌─────────────────┴─────────────────┐   │
    │  │        Tunnel 1 Group             │    │        Tunnel 2 Group             │   │
    │  │         25.11% Total              │    │         74.89% Total              │   │
    │  │      (1:3 weight ratio)           │    │      (2:3 weight ratio)           │   │
    │  └───────────────────────────────────┘    └───────────────────────────────────┘   │
    └─────────────────────────────────────────────────────────────────────────────────────┘
```

#### System Block Diagram

```
                             DCGate Test System Architecture
    ┌─────────────────────────────────────────────────────────────────────────────────────┐
    │                                Test Controller                                      │
    │  ┌─────────────────────────────────────────────────────────────────────────────┐   │
    │  │                           Go Test Framework                                 │   │
    │  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │   │
    │  │  │TestBasicEnca│  │TestEncapFrr │  │TestRegional │  │TestFibChains│        │   │
    │  │  │p() - 16 TCs │  │p() - 7 TCs  │  │ization()    │  │() - 6 TCs   │        │   │
    │  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘        │   │
    │  │                                                                             │   │
    │  │  ┌─────────────────────────────────────────────────────────────────────┐   │   │
    │  │  │                     Ondatra Test Orchestration                     │   │   │
    │  │  │  • Device Binding & Management                                     │   │   │
    │  │  │  • Topology Configuration                                          │   │   │
    │  │  │  • Test Lifecycle Management                                       │   │   │
    │  │  └─────────────────────────────────────────────────────────────────────┘   │   │
    │  └─────────────────────────────────────────────────────────────────────────────┘   │
    └─────────────────────────────────────────────────────────────────────────────────────┘
                                           │
               ┌───────────────────────────┼───────────────────────────┐
               │                           │                           │
               ▼                           ▼                           ▼
    ┌─────────────────────┐    ┌─────────────────────┐    ┌─────────────────────┐
    │    gRIBI Client     │    │      gNMI Client    │    │     OTG Client      │
    │                     │    │                     │    │                     │
    │ ┌─────────────────┐ │    │ ┌─────────────────┐ │    │ ┌─────────────────┐ │
    │ │ FIB Programming │ │    │ │Config/Telemetry │ │    │ │Traffic Generation│ │
    │ │• NH Management  │ │    │ │• VRF Config     │ │    │ │• Flow Creation  │ │
    │ │• NHG Weighting  │ │    │ │• Interface Cfg  │ │    │ │• Rate Control   │ │
    │ │• Route Install  │ │    │ │• Policy Config  │ │    │ │• Protocol Stack │ │
    │ │• State Verify   │ │    │ │• Counter Query  │ │    │ │• Packet Capture │ │
    │ └─────────────────┘ │    │ └─────────────────┘ │    │ └─────────────────┘ │
    └─────────────────────┘    └─────────────────────┘    └─────────────────────┘
               │                           │                           │
               ▼                           ▼                           ▼
    ┌─────────────────────────────────────────────────────────────────────────────────────┐
    │                                Network Fabric                                       │
    │                         (Lab Infrastructure/Testbed)                               │
    └─────────────────────────────────────────────────────────────────────────────────────┘
               │                           │                           │
               ▼                           ▼                           ▼
    ┌─────────────────────┐                                ┌─────────────────────┐
    │    DUT (Cisco)      │◄──────────── mgmt ────────────►│    ATE (Keysight/   │
    │                     │               │                │     Ixia/Spirent)   │
    │ ┌─────────────────┐ │               │                │ ┌─────────────────┐ │
    │ │     Control     │ │               │                │ │   Traffic Gen   │ │
    │ │ ┌─────────────┐ │ │               │                │ │ ┌─────────────┐ │ │
    │ │ │gRIBI Server │ │ │               │                │ │ │  port1      │ │ │
    │ │ │gNMI Server  │ │ │               │                │ │ │ (Source)    │ │ │
    │ │ │Process Mgmt │ │ │               │                │ │ └─────────────┘ │ │
    │ │ └─────────────┘ │ │               │                │ │                 │ │
    │ └─────────────────┘ │               │                │ │ ┌─────────────┐ │ │
    │                     │               │                │ │ │Capture Ports│ │ │
    │ ┌─────────────────┐ │               │                │ │ │port2/3/4/5  │ │ │
    │ │   Data Plane    │ │◄──── data ────┤                │ │ │Analysis Eng │ │ │
    │ │ ┌─────────────┐ │ │     plane     │                │ │ └─────────────┘ │ │
    │ │ │FIB/RIB Mgmt │ │ │    links      │                │ └─────────────────┘ │
    │ │ │VRF Instances│ │ │               │                └─────────────────────┘
    │ │ │Encap Engine │ │ │               │
    │ │ │ECMP/WCMP    │ │ │               │
    │ │ │Port Mapping │ │ │               │
    │ │ └─────────────┘ │ │               │
    │ │                 │ │               │
    │ │ ┌─────────────┐ │ │               │
    │ │ │   Ports     │ │ │               │
    │ │ │port1 ◄─────┐│ │ │               │
    │ │ │port2 ──────┼┼─┼─┼───────────────┼────► ATE port2
    │ │ │port3 ──────┼┼─┼─┼───────────────┼────► ATE port3  
    │ │ │port4 ──────┼┼─┼─┼───────────────┼────► ATE port4
    │ │ │port5 ──────┼┼─┼─┼───────────────┼────► ATE port5
    │ │ └─────────────┘│ │ │               │
    │ └─────────────────┘ │               │
    └─────────────────────┘               │
            ▲                             │
            │                             │
    ┌─────────────────────┐               │
    │   Platform/OS       │               │
    │ ┌─────────────────┐ │               │
    │ │   IOS-XR/XE     │ │               │
    │            
    │ └─────────────────┘ │               │
    └─────────────────────┘               │
                                          │
                                          ▼
                                ATE port1 ◄────────┘
                                (Traffic Source)
```

#### gRIBI Client Configuration
```go
c := gribi.Client{
    DUT:         dut,           // Device Under Test connection
    FIBACK:      true,          // Enable FIB acknowledgment
    Persistence: true,          // Maintain state across reconnections
}
```

#### Advanced FIB Operations

##### Next-Hop Programming
- **MAC with Interface**: Direct interface-based forwarding
- **MAC with IP**: IP-based next-hop with static ARP override
- **Encap NH**: Tunnel encapsulation next-hops with source/destination specification
- **DecapEncap NH**: Combined decapsulation and re-encapsulation operations
- **VRFOnly NH**: Cross-VRF lookup next-hops

##### Next-Hop Group (NHG) Management
```go
// Weighted ECMP configuration
c.AddNHG(t, nhg1ID, map[uint64]uint64{
    nh1ID: 1,  // Weight 1 (25%)
    nh2ID: 3,  // Weight 3 (75%)
}, defaultVRF, fluent.InstalledInFIB)

// Backup NHG configuration
c.AddNHG(t, primaryNHGID, primaryNHMap, defaultVRF, fluent.InstalledInFIB, 
    &gribi.NHGOptions{BackupNHG: backupNHGID})
```

##### Route Installation Patterns
- **IPv4 Route Programming**: Supports /32 host routes and prefix routes
- **IPv6 Route Programming**: Full IPv6 prefix support including /128 host routes
- **Cross-VRF Route Installation**: Routes installed in one VRF pointing to NHs in another
- **Default Route Handling**: Special handling for 0.0.0.0/0 and ::/0 routes

### Device Configuration Integration

#### DUT Configuration Automation
```go
func configureDUT(t *testing.T, dut *ondatra.DUTDevice, enableEncap bool) {
    // VRF creation and configuration
    // Interface IP assignment  
    // Policy configuration
    // DSCP classification rules
    // Static route configuration
}
```

#### Configuration Components

##### Interface Configuration
- **IP Address Assignment**: Both IPv4 and IPv6 addresses per interface
- **MTU Configuration**: Proper MTU settings for encapsulation overhead
- **DSCP Trust Settings**: Interface-level DSCP trust and remarking

##### Policy Configuration  
- **VRF Selection Policies**: Traffic steering based on ingress criteria
- **Encapsulation Policies**: Tunnel selection and encap parameter configuration
- **QoS Policies**: DSCP-based traffic classification and treatment

#### Platform-Specific Deviations
```go
// Cisco-specific workarounds and optimizations
if deviations.GRIBIMACOverrideWithStaticARP(dut) {
    // Use static ARP entries instead of dynamic resolution
    c.AddNH(t, nhID, "MACwithIp", defaultVRF, fluent.InstalledInFIB, 
        &gribi.NHOptions{Dest: targetIP, Mac: magicMac})
}
```

### Testing Framework Integration

#### Ondatra Test Orchestration
- **Device Binding**: Automatic DUT and ATE device discovery and binding
- **Topology Management**: Dynamic test topology creation and teardown
- **State Validation**: Comprehensive device state verification

#### Test Execution Flow
1. **Environment Setup**: Device binding and initial configuration
2. **gRIBI Client Establishment**: Connection setup and leadership election
3. **FIB Programming**: Route and NH installation with validation
4. **Traffic Generation**: Multi-flow traffic generation with timing control
5. **Validation**: Packet capture analysis and counter verification
6. **Cleanup**: Configuration removal and resource cleanup

#### Error Handling and Recovery
- **Connection Resilience**: Automatic gRIBI reconnection on failures
- **State Verification**: Continuous FIB state validation
- **Failure Detection**: Traffic loss detection and reporting
- **Cleanup Guarantees**: Ensures clean test environment for subsequent tests

## Running Tests

### Prerequisites

1. Cisco device under test (DUT) properly configured
2. Open Traffic Generator (OTG) available  
3. Go test environment with required dependencies
4. Network connectivity between test infrastructure and DUT

## Validation Methods and Analysis

### Traffic Validation Framework

#### Multi-Layer Validation Approach
1. **Packet-Level Validation** - Individual packet header and payload verification
2. **Flow-Level Validation** - End-to-end flow continuity and characteristics
3. **Statistical Validation** - Traffic distribution and performance metrics
4. **State Validation** - FIB programming and device state consistency

#### Packet Capture and Analysis
```go
func validatePacketCapture(t *testing.T, args *testArgs, capturePorts []string, pattr *packetAttr) map[string][]int {
    // Capture packets across multiple ports simultaneously
    // Analyze encapsulation headers (outer IP, inner IP, protocols)
    // Validate DSCP marking preservation and modification
    // Count tunnel-specific packet distribution
    // Return tunnel counter statistics for ratio validation
}
```

**Captured Packet Analysis:**
- **Outer Header Validation**: Source IP, destination IP, protocol, TTL/hop limit
- **Inner Header Preservation**: Original packet headers maintained through encapsulation
- **DSCP Marking Verification**: Correct DSCP values in both outer and inner headers
- **Fragmentation Handling**: Proper fragmentation and reassembly behavior
- **Tunnel Identification**: Packet classification into correct tunnel groups

#### Traffic Distribution Validation
```go
func validateTrafficDistribution(t *testing.T, otg *ondatra.ATEDevice, weights []float64) {
    // Collect traffic statistics from all destination ports
    // Calculate actual traffic ratios
    // Compare against expected weight distribution
    // Account for statistical variance and tolerance
    // Report ratio deviations and pass/fail status
}
```

**Distribution Metrics:**
- **Packet Count Ratios**: Actual vs. expected packet distribution
- **Byte Count Ratios**: Traffic volume distribution across paths
- **Tolerance Validation**: Acceptable variance from target ratios (±15%)
- **Convergence Time**: Time to achieve stable traffic distribution

#### Tunnel Encapsulation Ratio Validation
```go
func validateTunnelEncapRatio(t *testing.T, tunCounter map[string][]int) {
    for port, counts := range tunCounter {
        tunnel1Pkts := float64(counts[0])  // Tunnel 1 packet count
        tunnel2Pkts := float64(counts[1])  // Tunnel 2 packet count
        totalPkts := tunnel1Pkts + tunnel2Pkts
        
        if totalPkts > 0 {
            ratio1 := tunnel1Pkts / totalPkts    // Tunnel 1 ratio
            ratio2 := tunnel2Pkts / totalPkts    // Tunnel 2 ratio
            
            // Validate against expected ratios with tolerance
            validateRatioWithTolerance(t, ratio1, ratio2, port)
        }
    }
}
```

#### Process State Monitoring
```go
func validateProcessRecovery(t *testing.T, dut *ondatra.DUTDevice, processName string) {
    // Monitor process status before restart
    // Trigger controlled process restart
    // Track recovery time and state restoration
    // Validate FIB state consistency post-recovery
    // Measure traffic impact during recovery
}
```

## Troubleshooting

### Common Issues

1. **Process Recovery**: Ensure EMSD process is properly monitored
2. **VRF Configuration**: Verify all required VRFs are configured on DUT
3. **Interface Status**: Check port status and connectivity

### Debug Information

Tests provide detailed logging for:

- Traffic flow configuration
- Packet capture analysis  
- FIB programming status
- Counter validation results
- Process status monitoring
