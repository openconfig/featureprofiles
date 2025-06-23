# TE-18.1: gRIBI MPLS-in-UDP Encapsulation

## Summary

Test MPLS-in-UDP encapsulation using gRIBI to create AFT entries that
match IPv6 traffic and encapsulate it with MPLS labels inside UDP
packets with IPv6 outer headers. The test validates that the DUT
correctly implements RFC 7510 MPLS-in-UDP encapsulation with
configurable UDP destination ports.

## Topology

ATE port-1 \<——\> port-1 DUT

DUT port-2 \<——\> port-2 ATE

- [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Test Configuration

The test uses the following key parameters:

    # MPLS-in-UDP configuration
    mpls_label = 100
    outer_ipv6_src = "2001:db8::1"
    outer_ipv6_dst = "2001:db8::100"
    outer_dst_udp_port = 6635  # RFC 7510 standard MPLS-in-UDP port
    outer_ip_ttl = 64
    outer_dscp = 10
    inner_ipv6_prefix = "2001:db8:1::/64"

    # Static ARP configuration for gRIBI next hop resolution
    magic_ip = "192.168.1.1"
    magic_mac = "02:00:00:00:00:01"

## Baseline Setup

- Apply VRF selection policy to DUT port-1 to route traffic to the
  DEFAULT network instance
- Configure static ARP entries for gRIBI next hop resolution
  (device-specific)
- Set up basic routing infrastructure to forward encapsulated packets to
  port-2

Using gRIBI, install the following AFT entries:

    # Basic routing infrastructure for encapsulated packet forwarding
    IPv6Entry {2001:db8::100/128 (DEFAULT VRF)} -> NHG#400 (DEFAULT VRF) -> {
      {NH#300, DEFAULT VRF, interface: port-2, mac: magic_mac}
    }

    # MPLS-in-UDP encapsulation entries
    IPv6Entry {2001:db8:1::/64 (DEFAULT VRF)} -> NHG#2001 (DEFAULT VRF) -> {
      {NH#1001, DEFAULT VRF}
    }

    NH#1001 -> {
      encap_headers {
        encap_header {
          index: 1
          mpls {
            pushed_mpls_label_stack: [100]
          }
        }
        encap_header {
          index: 2
          udp_v6 {
            src_ip: "2001:db8::1"
            dst_ip: "2001:db8::100"
            dst_udp_port: 6635
            ip_ttl: 64
            dscp: 10
          }
        }
      }
    }

## Procedure

### TE-18.1.1 MPLS-in-UDP IPv6 Traffic Encapsulation

Send IPv6 packets from ATE port-1 to DUT port-1 with:

- Source IPv6: 2001:0db8::192:0:2:2 (OTG port-1)
- Destination IPv6: 2001:db8:1:: (matches inner_ipv6_prefix)
- DSCP: 10
- UDP payload with random source/destination ports

Validate that ATE port-2 receives MPLS-in-UDP encapsulated packets with:

**Outer IPv6 Header:**

- Source IP: 2001:db8::1
- Destination IP: 2001:db8::100
- Hop Limit: 64
- Traffic Class: DSCP 10

**UDP Header:**

- Source Port: random, no need to validate
- Destination Port: 6635 (RFC 7510 standard)
- Protocol: UDP (17)

**MPLS Header (inside UDP payload):**

- Label: 100
- Bottom of Stack: 1 (true)
- TTL: 99 (decremented from inner packet TTL of 100)

**Inner IPv6 Packet:**

- Original IPv6 packet with decremented hop limit

### TE-18.1.2 Traffic Flow Validation

1.  **Positive Test**: Verify traffic flows successfully when
    MPLS-in-UDP entries are installed
    - Send traffic matching the IPv6 prefix 2001:db8:1::/64
    - Validate 0% packet loss
    - Confirm packets are properly encapsulated and forwarded
2.  **Negative Test**: Verify traffic fails when MPLS-in-UDP entries are
    removed
    - Delete the gRIBI entries in reverse order
    - Send the same traffic
    - Validate 100% packet loss (no forwarding path)

## Packet Validation Details

The test performs detailed packet capture validation:

1.  **Capture Analysis**: Uses OTG packet capture on port-2 to analyze
    encapsulated packets
2.  **Header Parsing**: Validates both gopacket parsing and manual
    byte-level parsing of UDP headers
3.  **MPLS Validation**: Decodes MPLS header from UDP payload and
    validates label, BoS bit, and TTL
4.  **Error Detection**: Identifies common issues like incorrect UDP
    ports or malformed MPLS headers

## Expected Behavior

- **RFC 7510 Compliance**: DUT must use UDP destination port 6635 for
  MPLS-in-UDP encapsulation
- **Proper Encapsulation**: Inner IPv6 packets are encapsulated with
  MPLS label 100 inside UDP
- **Header Preservation**: DSCP and TTL values are correctly set in
  outer headers
- **Forwarding**: Encapsulated packets are properly routed to the egress
  port

## Required DUT platform

- FFF (supports gRIBI and MPLS-in-UDP encapsulation)

## OpenConfig Path and RPC Coverage

``` yaml
paths:
  ## Config paths
  /network-instances/network-instance/policy-forwarding/policies/policy/config/policy-id:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/config/sequence-id:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/network-instance:

  ## State paths
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/state/type:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/mpls/state/mpls-label-stack:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/udp-v6/state/src-ip:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/udp-v6/state/dst-ip:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/udp-v6/state/src-udp-port:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/udp-v6/state/dst-udp-port:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/udp-v6/state/ip-ttl:
  /network-instances/network-instance/afts/next-hops/next-hop/encap-headers/encap-header/udp-v6/state/dscp:
  /interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/state/link-layer-address:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
  gribi:
    gRIBI.Modify:
    gRIBI.Flush:
```
