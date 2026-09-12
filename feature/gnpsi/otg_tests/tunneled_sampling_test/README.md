# gNPSI-2: gNPSI with Tunneled Traffic

## Summary

* **Objective:** Check if gNPSI can capture and report encapsulated (e.g., IP-in-IP, GRE, MPLS) packets, and whether it provides information about the outer and inner headers. The test covers both IPv4 and IPv6 variants for inner and outer headers.
* **Interfaces Used:** gNPSI, gNMI, ATE (Traffic Generator), and gRIBI (to set up tunnels).
* **Expected Outcome Summary:**
  * Sampled packet reports for tunneled traffic should contain the full packet, including outer and inner headers.
  * Metadata should explicitly indicate the packet was encapsulated.

## Testbed type

* `TESTBED_DUT_ATE_2LINKS`

## Procedure

### Test environment setup

* Configure the DUT with two ports, Port 1 (ingress) and Port 2 (egress),
  connected respectively to ATE Port 1 and ATE Port 2.
* Configure base IPv4 and IPv6 addresses on DUT and ATE interfaces using
  standard test IP addresses (e.g., `198.51.100.0/24`, `192.0.2.1`,
  `2001:db8::1`).
* Enable sFlow using gNMI Set on `/sampling/sflow/config/enabled` = `true`.
* Enable sFlow on the egress port via `/sampling/sflow/interfaces/interface[name=port2]/config/enabled` = `true` and `/sampling/sflow/interfaces/interface[name=port2]/config/name` = `port2`, using a standard sampling rate of 1:1M and a sample size of 256 bytes.
* Connect a gNPSI client and subscribe to the gNPSI service on the DUT.

### gNPSI-2.1 - Sampling IP-in-IP Tunneled Traffic

* Step 1 - Generate DUT configuration

Use `gRIBI.Modify` to program a block of 1,000 IP-in-IP tunnel routes (e.g., a `/22` subnet divided into 1,000 `/32` host routes) on the DUT. The routes should take incoming traffic from ATE Port 1, encapsulate it into an IP-in-IP tunnel (IPv4 outer / IPv4 inner, then IPv6 outer / IPv6 inner), and forward it out to ATE Port 2. Use `AFTOperation` to set up the `AFT_NEXTHOP_GROUP` and `AFT_IPV4_ENTRY`.

#### Canonical OC

```json
{
  "openconfig-interfaces:interfaces": {
    "interface": [
      {
        "config": {
          "enabled": true,
          "name": "port2"
        },
        "name": "port2"
      }
    ]
  },
  "openconfig-sampling:sampling": {
    "sflow": {
      "config": {
        "enabled": true
      },
      "interfaces": {
        "interface": [
          {
            "config": {
              "enabled": true,
              "name": "port2"
            },
            "name": "port2"
          }
        ]
      }
    }
  }
}
```

* Step 2 - Push configuration to DUT using gnmi.Set with REPLACE option
* Step 3 - Send Traffic

Configure the ATE to transmit continuous multiplexed IPv4 and IPv6 traffic from ATE Port 1, destined to the 1,000 programmed IP-in-IP encapsulated route destinations. Embed a unique magic-byte signature in the ATE payload to positively identify test traffic. Send traffic at a deterministic rate and duration.

* Step 4 - Validation with pass/fail criteria

Verify that the ATE Port 2 receives the encapsulated traffic at the expected rate. Calculate the expected number of gNPSI samples (`Total Packets / 1,000,000`) and verify that the gNPSI client successfully receives sampled packet reports within a statistical tolerance of +/- 10% of the expected count.

**Pass Criteria**:

* The ATE successfully receives the encapsulated IP-in-IP traffic on Port 2.
* The sampled datagram contains the complete IP-in-IP encapsulated packet, successfully retaining both the outer IP header and the inner IP header.
* The sampled packet metadata includes indicators or flags explicitly denoting that the packet was encapsulated.
* The sampled packets contain the unique magic-byte signature.
* The number of samples received falls within the +/- 10% statistical tolerance of the expected sample count.

**Fail Criteria**:

* Egress traffic is dropped or blackholed (ATE does not receive traffic).
* The number of samples falls outside the acceptable statistical tolerance.
* The sampled packet only contains the inner header (decapsulation occurred before sampling) or is truncated such that inner headers are unreadable.
* The metadata fails to capture the encapsulation type.

### gNPSI-2.2 - Sampling GRE Tunneled Traffic

* Step 1 - Generate DUT configuration

Building sequentially on the previous subtest, do not flush existing routes. Use `gRIBI.Modify` to program an additional block of 1,000 GRE tunnel routes on the DUT. These routes should take incoming traffic from ATE Port 1, encapsulate it into a GRE tunnel (IPv4 outer / IPv4 inner, then IPv6 outer / IPv6 inner), and forward it out to ATE Port 2.

* Step 2 - Push configuration to DUT using `gRIBI.Modify`
* Step 3 - Send Traffic

Configure the ATE to transmit continuous multiplexed IPv4 and IPv6 traffic from ATE Port 1 destined to both the IP-in-IP and the new GRE encapsulated route destinations. Embed a unique magic-byte signature in the payload.

* Step 4 - Validation with pass/fail criteria

Verify that the gNPSI client successfully receives sampled packet reports for both IP-in-IP and GRE traffic concurrently. 

**Pass Criteria**:

* The ATE successfully receives the encapsulated GRE and IP-in-IP traffic on Port 2.
* The sampled datagrams for GRE traffic contain the complete GRE encapsulated packet, successfully retaining both the outer IP/GRE header and the inner IP header.
* The sampled packet metadata includes indicators or flags explicitly denoting that the packet was encapsulated.
* The number of samples received falls within the +/- 10% statistical tolerance of the expected sample count.

### gNPSI-2.3 - Sampling MPLS Tunneled Traffic

* Step 1 - Generate DUT configuration

Building sequentially, do not flush existing routes. Use `gRIBI.Modify` to program an additional block of 1,000 MPLS tunnel routes on the DUT. These routes should encapsulate incoming traffic into an MPLS tunnel (IPv4 inner, then IPv6 inner), and forward it out to ATE Port 2.

* Step 2 - Push configuration to DUT using `gRIBI.Modify`
* Step 3 - Send Traffic

Configure the ATE to transmit continuous multiplexed IPv4 and IPv6 traffic from ATE Port 1 destined to all programmed route destinations (IP-in-IP, GRE, and MPLS). Embed a unique magic-byte signature in the payload.

* Step 4 - Validation with pass/fail criteria

Verify that the gNPSI client successfully receives sampled packet reports for all three tunnel types concurrently.

**Pass Criteria**:

* The ATE successfully receives the encapsulated MPLS, GRE, and IP-in-IP traffic on Port 2.
* The sampled datagrams for MPLS traffic contain the complete MPLS encapsulated packet, successfully retaining both the MPLS label stack and the inner IP header.
* The sampled packet metadata includes indicators or flags explicitly denoting that the packet was encapsulated.
* The total number of samples received falls within the +/- 10% statistical tolerance of the expected sample count.

### gNPSI-2.4 - Unsampled Port Traffic (Negative Test)

* Step 1 - Setup
Ensure sFlow/gNPSI is explicitly disabled on an egress port (e.g., Port 3).
* Step 2 - Send Traffic
Send tunneled traffic out of the unsampled port.
* Step 3 - Validation
Verify exactly 0 samples are received by the gNPSI client for this traffic stream.

### gNPSI-2.5 - Unresolved Next-Hop Drop (Negative Test)

* Step 1 - Setup
Use `gRIBI.Modify` to program a tunnel route with an unresolved or unreachable next-hop.
* Step 2 - Send Traffic
Send traffic destined to this route from ATE Port 1.
* Step 3 - Validation
Verify the traffic is dropped by the DUT, and that 0 egress samples are generated for this traffic (ensuring we do not sample dropped datapath traffic).

### gNPSI-2.6 - MTU Exceeded (Negative Test)

* Step 1 - Setup
Use any of the programmed tunnel routes.
* Step 2 - Send Traffic
Send traffic from ATE Port 1 where the packet size exceeds the tunnel MTU.
* Step 3 - Validation
Verify the DUT handles fragmentation gracefully, and that the sampled packets correctly reflect the fragmented headers without causing gNPSI to crash or corrupt the payload.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/name:
  /sampling/sflow/config/enabled:
  /sampling/sflow/interfaces/interface/config/enabled:
  /sampling/sflow/interfaces/interface/config/name:
rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
  gnpsi:
    gNPSI.Subscribe:
  gribi:
    gRIBI.Modify:
    gRIBI.Flush:
```

## Required DUT platform

* FFF - fixed form factor
