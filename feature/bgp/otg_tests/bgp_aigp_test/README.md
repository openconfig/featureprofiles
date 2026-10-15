# RT-1.37: BGP AIGP Feature Support and Precedence

## Summary

This test validates Accumulated IGP Metric (AIGP) attribute support for BGP in both `DEFAULT` and non-default (`test-instance`) network instances across IPv4 and IPv6 address families at scale (1,000 IPv4 prefixes and 1,000 IPv6 prefixes), ensuring the following use cases:

1. Per-address-family AIGP exchange enablement and disablement on BGP neighbors and BGP peer-groups.
2. AIGP enabled by default on iBGP peers and disabled by default on eBGP peers in accordance with RFC 7311.
3. Most-specific configuration precedence when AIGP is configured on both a BGP peer-group and an individual BGP neighbor within that peer-group (concurrently validating overridden and inherited neighbors within the same peer-group).
4. Modification of the AIGP attribute using BGP import and export routing policies (`set-aigp`) together with `set-next-hop: SELF`.
5. BGP best-path selection using the lowest AIGP metric prior to AS-PATH length comparison across scaled prefix sets, and tie-breaking via AS-PATH length when AIGP metrics are equal.
6. AIGP metric recalculation across iBGP route reflection with `next-hop-self`, both when a valid IS-IS IGP metric exists to the original BGP next-hop and when the IGP metric to the original next-hop is zero (`+1` increment behavior).
7. Omission of the AIGP attribute from BGP UPDATE messages when AIGP is disabled on a BGP neighbor.
8. Negative validation of inbound AIGP attribute stripping when AIGP is disabled by default on eBGP peers (RFC 7311 §3.4) and route withdrawal when the original BGP next-hop becomes unresolvable in the IGP during route reflection (RFC 7311 §3.2).

## Testbed type

*   [dutdutate.testbed](https://github.com/openconfig/featureprofiles/blob/main/topologies/dutdutate.testbed)

## Procedure

### Test environment setup

#### Test Topology

```text
+---------+      lag1 [eBGP]        +---------+      lag2 [iBGP]        +---------+
|   ATE   |=========================|  DUT 1  |=========================|  DUT 2  |
+---------+                         +---------+                         +---------+
```

#### Port and LAG Mapping (Table 0)

| Device | Ondatra Port ID | Aggregate Interface (`/interfaces/interface/config/name`) | Interface Type (`/interfaces/interface/config/type`) | LACP Mode (`/lacp/interfaces/interface/config/lacp-mode`) | LACP Interval (`/lacp/interfaces/interface/config/interval`) | Peer Device & Port |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| ATE | `ate:port1` | `lag1` | `iana-if-type:ieee8023adLag` | `ACTIVE` | `FAST` | `dut1:port1` |
| DUT 1 | `dut1:port1` | `lag1` | `iana-if-type:ieee8023adLag` | `ACTIVE` | `FAST` | `ate:port1` |
| DUT 1 | `dut1:port2` | `lag2` | `iana-if-type:ieee8023adLag` | `ACTIVE` | `FAST` | `dut2:port1` |
| DUT 2 | `dut2:port1` | `lag2` | `iana-if-type:ieee8023adLag` | `PASSIVE` | `FAST` | `dut1:port2` |

#### IPv4 Addresses `lag1` (DUT 1 towards ATE) (Table 1)

| Device | VLAN 10 (`lag1.10` / `eth1.10`) | VLAN 20 (`lag1.20` / `eth1.20`) | VLAN 30 (`lag1.30` / `eth1.30`) | VLAN 40 (`lag1.40` / `eth1.40`) |
| :--- | :--- | :--- | :--- | :--- |
| Network Instance | `DEFAULT` | `DEFAULT` | `test-instance` | `test-instance` |
| DUT 1 | 198.51.100.1/30 | 198.51.100.5/30 | 198.51.100.9/30 | 198.51.100.13/30 |
| ATE | 198.51.100.2/30 | 198.51.100.6/30 | 198.51.100.10/30 | 198.51.100.14/30 |

#### IPv6 Addresses `lag1` (DUT 1 towards ATE) (Table 2)

| Device | VLAN 10 (`lag1.10` / `eth1.10`) | VLAN 20 (`lag1.20` / `eth1.20`) | VLAN 30 (`lag1.30` / `eth1.30`) | VLAN 40 (`lag1.40` / `eth1.40`) |
| :--- | :--- | :--- | :--- | :--- |
| Network Instance | `DEFAULT` | `DEFAULT` | `test-instance` | `test-instance` |
| DUT 1 | 2001:db8::1/126 | 2001:db8::5/126 | 2001:db8::9/126 | 2001:db8::13/126 |
| ATE | 2001:db8::2/126 | 2001:db8::6/126 | 2001:db8::10/126 | 2001:db8::14/126 |

#### IP Addresses `lag2` (DUT 1 towards DUT 2 `DEFAULT` and `test-instance` NIs) (Table 3)

| Device | VLAN 10 (`lag2.10`) IPv4 (`DEFAULT` NI) | VLAN 10 (`lag2.10`) IPv6 (`DEFAULT` NI) | VLAN 20 (`lag2.20`) IPv4 (`test-instance` NI) | VLAN 20 (`lag2.20`) IPv6 (`test-instance` NI) |
| :--- | :--- | :--- | :--- | :--- |
| DUT 1 | 198.51.100.17/30 | 2001:db8::17/126 | 198.51.100.21/30 | 2001:db8::21/126 |
| DUT 2 | 198.51.100.18/30 | 2001:db8::18/126 | 198.51.100.22/30 | 2001:db8::22/126 |

#### IP Addresses `lag2` (DUT 1 towards DUT 2 `test-originate` NI) (Table 4)

| Device | VLAN 30 (`lag2.30`) IPv4 | VLAN 30 (`lag2.30`) IPv6 | VLAN 40 (`lag2.40`) IPv4 | VLAN 40 (`lag2.40`) IPv6 |
| :--- | :--- | :--- | :--- | :--- |
| DUT 1 (`DEFAULT` / `test-instance` NI) | 198.51.100.25/30 (`DEFAULT` NI) | 2001:db8::25/126 (`DEFAULT` NI) | 198.51.100.29/30 (`test-instance` NI) | 2001:db8::29/126 (`test-instance` NI) |
| DUT 2 (`test-originate` NI) | 198.51.100.26/30 | 2001:db8::26/126 | 198.51.100.30/30 | 2001:db8::30/126 |

#### BGP Autonomous System Numbers (ASN) and Router IDs (Table 5)

| Device / Network Instance | NI Type (`/network-instances/network-instance/config/type`) | ASN (`/bgp/global/config/as`) | Router ID (`/bgp/global/config/router-id`) |
| :--- | :--- | :--- | :--- |
| ATE | N/A | 64496 | 198.51.100.2 |
| DUT 1 `DEFAULT` NI | `openconfig-network-instance-types:DEFAULT_INSTANCE` | 64497 | 192.0.2.10 |
| DUT 1 `test-instance` NI | `openconfig-network-instance-types:L3VRF` | 64498 | 192.0.2.20 |
| DUT 2 `DEFAULT` NI | `openconfig-network-instance-types:DEFAULT_INSTANCE` | 64497 | 192.0.2.104 |
| DUT 2 `test-instance` NI | `openconfig-network-instance-types:L3VRF` | 64498 | 192.0.2.105 |
| DUT 2 `test-originate` NI | `openconfig-network-instance-types:L3VRF` | 64499 | 192.0.2.103 |

#### IS-IS Network Entity Titles (NET) (Table 6)

| Device / Network Instance | IS-IS Instance Name | Entity (`/isis/global/config/net`) | Level Capability | Metric Style |
| :--- | :--- | :--- | :--- | :--- |
| DUT 1 `DEFAULT` NI | `DEFAULT` | `49.0001.1980.5110.0025.00` | `LEVEL_2` | `WIDE_METRIC` |
| DUT 1 `test-instance` NI | `DEFAULT` | `49.0001.1980.5110.0029.00` | `LEVEL_2` | `WIDE_METRIC` |
| DUT 2 `DEFAULT` NI | `DEFAULT` | `49.0001.1980.5110.0026.00` | `LEVEL_2` | `WIDE_METRIC` |
| DUT 2 `test-instance` NI | `DEFAULT` | `49.0001.1980.5110.0030.00` | `LEVEL_2` | `WIDE_METRIC` |
| DUT 2 `test-originate` NI | `DEFAULT` | `49.0001.1980.5110.0100.00` | `LEVEL_2` | `WIDE_METRIC` |

#### Scaled BGP Advertised Prefix Sets and ATE Traffic Profile (Table 7)

The ATE advertises **1,000 IPv4 prefixes** (from RFC 2544 benchmarking range `198.18.0.0/15`: `198.18.0.0/24` through `198.21.231.0/24`) and **1,000 IPv6 prefixes** (from RFC 3849 documentation range `2001:db8:1000::/64` through `2001:db8:13e7::/64`) across all eBGP sessions on `lag1`. Traffic flows are sent into DUT 1 with **Destination IPs matching the BGP-advertised prefix ranges** so that DUT 1 performs a hardware FIB lookup on the BGP routes and forwards packets out the winning BGP next-hop subinterface (`eth1.20` / `eth1.40` in `RT-1.37.1`, and `eth1.10` / `eth1.30` in `RT-1.37.2`).

| Flow Name | Ingress Interface | Expected Egress (`RT-1.37.1`) | Expected Egress (`RT-1.37.2`) | Source IP | Destination IP Range (1,000 Destinations) | Frame Size (Bytes) | Packet Rate (pps) | Total Packets (`fixed_packets`) | VLAN ID | Source MAC |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| Flow 1 (IPv4 `DEFAULT` NI) | `eth1.10` | `eth1.20` (`lag1.20`) | `eth1.10` (`lag1.10`) | `198.51.100.2` | `198.18.0.1` – `198.21.231.1` (step `0.0.1.0`, count `1000`) | 512 | 10,000 | 100,000 | 10 | `02:00:03:01:01:01` |
| Flow 2 (IPv4 `test-instance` NI) | `eth1.30` | `eth1.40` (`lag1.40`) | `eth1.30` (`lag1.30`) | `198.51.100.10` | `198.18.0.1` – `198.21.231.1` (step `0.0.1.0`, count `1000`) | 512 | 10,000 | 100,000 | 30 | `02:00:03:01:03:03` |
| Flow 3 (IPv6 `DEFAULT` NI) | `eth1.10` | `eth1.20` (`lag1.20`) | `eth1.10` (`lag1.10`) | `2001:db8::2` | `2001:db8:1000::1` – `2001:db8:13e7::1` (step `0:0:1::`, count `1000`) | 512 | 10,000 | 100,000 | 10 | `02:00:03:01:01:01` |
| Flow 4 (IPv6 `test-instance` NI) | `eth1.30` | `eth1.40` (`lag1.40`) | `eth1.30` (`lag1.30`) | `2001:db8::10` | `2001:db8:1000::1` – `2001:db8:13e7::1` (step `0:0:1::`, count `1000`) | 512 | 10,000 | 100,000 | 30 | `02:00:03:01:03:03` |

#### ATE Interfaces MAC Addresses (Table 8)

| Interface | Subinterface VLAN ID | MAC Address |
| :--- | :--- | :--- |
| `eth1.10` | 10 | `02:00:03:01:01:01` |
| `eth1.20` | 20 | `02:00:03:01:02:02` |
| `eth1.30` | 30 | `02:00:03:01:03:03` |
| `eth1.40` | 40 | `02:00:03:01:04:04` |

#### Loopback IP Addresses (Table 9)

To support scale and concurrent peer-group vs. neighbor precedence validation in `RT-1.37.6`, DUT 2 is configured with two client loopbacks in `DEFAULT` NI (`Loopback40`, `Loopback41`) and two client loopbacks in `test-instance` NI (`Loopback50`, `Loopback51`), in addition to the route-originating loopbacks in `test-originate` NI (`Loopback10`, `Loopback20`, `Loopback30`).

| Device | Network Instance (NI) | Loopback Name (`/interfaces/interface/config/name`) | Subinterface ID | IPv4 Address | IPv6 Address | Role |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| DUT 1 | `DEFAULT` NI | `Loopback10` | `Loopback10.0` | `192.0.2.10/32` | `2001:db8:10::1/128` | RR local-address (`DEFAULT` NI) |
| DUT 1 | `test-instance` NI | `Loopback20` | `Loopback20.0` | `192.0.2.20/32` | `2001:db8:20::1/128` | RR local-address (`test-instance` NI) |
| DUT 2 | `test-originate` NI | `Loopback10` | `Loopback10.0` | `192.0.2.101/32` | `2001:db8:10::2/128` | Originated prefix 1 |
| DUT 2 | `test-originate` NI | `Loopback20` | `Loopback20.0` | `192.0.2.102/32` | `2001:db8:20::2/128` | Originated prefix 2 |
| DUT 2 | `test-originate` NI | `Loopback30` | `Loopback30.0` | `192.0.2.103/32` | `2001:db8:30::2/128` | BGP local-address (`test-originate` NI) |
| DUT 2 | `DEFAULT` NI | `Loopback40` | `Loopback40.0` | `192.0.2.104/32` | `2001:db8:40::2/128` | RR Client A (`DEFAULT` NI) |
| DUT 2 | `DEFAULT` NI | `Loopback41` | `Loopback41.0` | `192.0.2.106/32` | `2001:db8:41::2/128` | RR Client B (`DEFAULT` NI - Peer-Group Inheritance) |
| DUT 2 | `test-instance` NI | `Loopback50` | `Loopback50.0` | `192.0.2.105/32` | `2001:db8:50::2/128` | RR Client A (`test-instance` NI) |
| DUT 2 | `test-instance` NI | `Loopback51` | `Loopback51.0` | `192.0.2.107/32` | `2001:db8:51::2/128` | RR Client B (`test-instance` NI - Peer-Group Inheritance) |

---

### RT-1.37.1 - AIGP Modification with BGP Policy, Per-Neighbor Propagation Control, and Next-Hop-Self

*   **Step 1 - Generate DUT and ATE configuration**

#### DUT 1 - Configuration

##### Interface and Network-Instance Configuration

*   Create two aggregate interfaces `/interfaces/interface[name=lag1]` and `/interfaces/interface[name=lag2]` with:
    *   `/interfaces/interface/config/type` = `iana-if-type:ieee8023adLag`
    *   `/interfaces/interface/config/enabled` = `true`
*   Configure LACP on `/lacp/interfaces/interface[name=lag1]` and `/lacp/interfaces/interface[name=lag2]` with:
    *   `/lacp/interfaces/interface/config/name` = `"lag1"` / `"lag2"`
    *   `/lacp/interfaces/interface/config/lacp-mode` = `ACTIVE`
    *   `/lacp/interfaces/interface/config/interval` = `FAST`
*   Bind physical port `dut1:port1` to `lag1` (`/interfaces/interface[name=port1]/ethernet/config/aggregate-id = "lag1"`) and `dut1:port2` to `lag2` (`/interfaces/interface[name=port2]/ethernet/config/aggregate-id = "lag2"`).
*   Configure the `DEFAULT` network instance (`/network-instances/network-instance[name=DEFAULT]/config/type = openconfig-network-instance-types:DEFAULT_INSTANCE`) and create a non-default network instance `/network-instances/network-instance[name=test-instance]` with:
    *   `/network-instances/network-instance/config/name` = `"test-instance"`
    *   `/network-instances/network-instance/config/type` = `openconfig-network-instance-types:L3VRF`
    *   `/network-instances/network-instance/config/enabled` = `true`
    *   Initialize protocol tables `/network-instances/network-instance/tables/table` in both `DEFAULT` and `test-instance` NIs for combinations of `protocol` $\in$ `{DIRECTLY_CONNECTED, BGP, ISIS}` and `address-family` $\in$ `{IPV4, IPV6}`.
*   On `lag1`, create subinterfaces `index = 10` (`lag1.10`) and `index = 20` (`lag1.20`):
    *   Set `/interfaces/interface[name=lag1]/subinterfaces/subinterface[index=10]/vlan/match/single-tagged/config/vlan-id = 10` and `index=20` `vlan-id = 20`.
    *   Associate `lag1.10` (`interface = "lag1"`, `subinterface = 10`) and `lag1.20` (`interface = "lag1"`, `subinterface = 20`) with `/network-instances/network-instance[name=DEFAULT]/interfaces/interface`.
    *   Configure IPv4 (`/subinterface/ipv4/addresses/address/config/ip` and `prefix-length`) and IPv6 (`/subinterface/ipv6/addresses/address/config/ip` and `prefix-length`) addresses as specified in Table 1 and Table 2.
*   On `lag1`, create subinterfaces `index = 30` (`lag1.30`) and `index = 40` (`lag1.40`):
    *   Set `vlan-id = 30` and `vlan-id = 40`, associate both with `/network-instances/network-instance[name=test-instance]/interfaces/interface`, and configure IPv4 and IPv6 addresses as specified in Table 1 and Table 2.
*   On `lag2`, create subinterface `index = 10` (`lag2.10`, `vlan-id = 10`):
    *   Associate `lag2.10` with `/network-instances/network-instance[name=DEFAULT]/interfaces/interface` and configure IPv4 (`198.51.100.17/30`) and IPv6 (`2001:db8::17/126`) addresses per Table 3.
*   On `lag2`, create subinterface `index = 20` (`lag2.20`, `vlan-id = 20`):
    *   Associate `lag2.20` with `/network-instances/network-instance[name=test-instance]/interfaces/interface` and configure IPv4 (`198.51.100.21/30`) and IPv6 (`2001:db8::21/126`) addresses per Table 3.
*   Create loopback interface `/interfaces/interface[name=Loopback10]` (`type = iana-if-type:softwareLoopback`, `enabled = true`), subinterface `0` (`Loopback10.0`) with IPv4 `192.0.2.10/32` and IPv6 `2001:db8:10::1/128` per Table 9, and associate `Loopback10.0` with `DEFAULT` NI.
*   Create loopback interface `/interfaces/interface[name=Loopback20]` (`type = iana-if-type:softwareLoopback`, `enabled = true`), subinterface `0` (`Loopback20.0`) with IPv4 `192.0.2.20/32` and IPv6 `2001:db8:20::1/128` per Table 9, and associate `Loopback20.0` with `test-instance` NI.

##### Routing Policy Configuration on DUT 1

*   Create routing policy `/routing-policy/policy-definitions/policy-definition[name=test-import-policy_aigp_20]`:
    *   Statement `name = "test-import-statement"`:
    *   Set `/routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-aigp = 20` and `/actions/config/policy-result = ACCEPT_ROUTE`.
*   Create routing policy `/routing-policy/policy-definitions/policy-definition[name=test-import-policy_aigp_150]`:
    *   Statement `name = "test-import-statement"`:
    *   Set `/routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-aigp = 150` and `/actions/config/policy-result = ACCEPT_ROUTE`.
*   Create routing policy `/routing-policy/policy-definitions/policy-definition[name=test-export-policy]`:
    *   Statement `name = "test-export-statement"`:
    *   Set `/routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-aigp = 200`, `/actions/bgp-actions/config/set-next-hop = "SELF"`, and `/actions/config/policy-result = ACCEPT_ROUTE`.

##### BGP Configuration on DUT 1 (`DEFAULT` Network Instance)

*   Configure `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=BGP][name=DEFAULT]`:
    *   `/bgp/global/config/as = 64497` and `/bgp/global/config/router-id = "192.0.2.10"`.
    *   Enable global AFI-SAFIs `/bgp/global/afi-safis/afi-safi[afi-safi-name=IPV4_UNICAST]/config/enabled = true` and `/bgp/global/afi-safis/afi-safi[afi-safi-name=IPV6_UNICAST]/config/enabled = true`.
*   Create four BGP peer-groups under `/bgp/peer-groups/peer-group`:
    *   `peer-group-name = "uplink"`: `config/peer-as = 64496`, AFI-SAFI `IPV4_UNICAST` `enabled = true`.
    *   `peer-group-name = "uplink6"`: `config/peer-as = 64496`, AFI-SAFI `IPV6_UNICAST` `enabled = true`.
    *   `peer-group-name = "downlink"`: `config/peer-as = 64497`, `route-reflector/config/route-reflector-client = true`, `route-reflector/config/route-reflector-cluster-id = "192.0.2.1"`, AFI-SAFI `IPV4_UNICAST` `enabled = true`.
    *   `peer-group-name = "downlink6"`: `config/peer-as = 64497`, `route-reflector/config/route-reflector-client = true`, `route-reflector/config/route-reflector-cluster-id = "192.0.2.1"`, AFI-SAFI `IPV6_UNICAST` `enabled = true`.
*   Create eBGP neighbors under `/bgp/neighbors/neighbor` over `lag1.10` and `lag1.20` towards the ATE:
    *   `neighbor-address = "198.51.100.2"`: `peer-group = "uplink"`, `apply-policy/config/import-policy = ["test-import-policy_aigp_150"]`, `afi-safis/afi-safi[afi-safi-name=IPV4_UNICAST]/config/enabled = true`, `afi-safis/afi-safi[afi-safi-name=IPV4_UNICAST]/config/enable-aigp = true`.
    *   `neighbor-address = "198.51.100.6"`: `peer-group = "uplink"`, `apply-policy/config/import-policy = ["test-import-policy_aigp_20"]`, `afi-safis/afi-safi[afi-safi-name=IPV4_UNICAST]/config/enabled = true`, `afi-safis/afi-safi[afi-safi-name=IPV4_UNICAST]/config/enable-aigp = true`.
    *   `neighbor-address = "2001:db8::2"`: `peer-group = "uplink6"`, `apply-policy/config/import-policy = ["test-import-policy_aigp_150"]`, `afi-safis/afi-safi[afi-safi-name=IPV6_UNICAST]/config/enabled = true`, `afi-safis/afi-safi[afi-safi-name=IPV6_UNICAST]/config/enable-aigp = true`.
    *   `neighbor-address = "2001:db8::6"`: `peer-group = "uplink6"`, `apply-policy/config/import-policy = ["test-import-policy_aigp_20"]`, `afi-safis/afi-safi[afi-safi-name=IPV6_UNICAST]/config/enabled = true`, `afi-safis/afi-safi[afi-safi-name=IPV6_UNICAST]/config/enable-aigp = true`.
*   Create iBGP neighbors under `/bgp/neighbors/neighbor` over `lag2.10` towards DUT 2:
    *   `neighbor-address = "198.51.100.18"`: `peer-group = "downlink"`, `apply-policy/config/export-policy = ["test-export-policy"]`, `afi-safis/afi-safi[afi-safi-name=IPV4_UNICAST]/config/enabled = true`, `afi-safis/afi-safi[afi-safi-name=IPV4_UNICAST]/config/enable-aigp = true`.
    *   `neighbor-address = "2001:db8::18"`: `peer-group = "downlink6"`, `apply-policy/config/export-policy = ["test-export-policy"]`, `afi-safis/afi-safi[afi-safi-name=IPV6_UNICAST]/config/enabled = true`, `afi-safis/afi-safi[afi-safi-name=IPV6_UNICAST]/config/enable-aigp = true`.

##### BGP Configuration on DUT 1 (`test-instance` Network Instance)

*   Configure `/network-instances/network-instance[name=test-instance]/protocols/protocol[identifier=BGP][name=DEFAULT]`:
    *   `/bgp/global/config/as = 64498` and `/bgp/global/config/router-id = "192.0.2.20"`.
    *   Enable global AFI-SAFIs `IPV4_UNICAST` and `IPV6_UNICAST` (`enabled = true`).
*   Create four BGP peer-groups under `/bgp/peer-groups/peer-group`:
    *   `peer-group-name = "uplink"`: `config/peer-as = 64496`, AFI-SAFI `IPV4_UNICAST` `enabled = true`.
    *   `peer-group-name = "uplink6"`: `config/peer-as = 64496`, AFI-SAFI `IPV6_UNICAST` `enabled = true`.
    *   `peer-group-name = "downlink"`: `config/peer-as = 64498`, `route-reflector/config/route-reflector-client = true`, `route-reflector/config/route-reflector-cluster-id = "192.0.2.1"`, AFI-SAFI `IPV4_UNICAST` `enabled = true`.
    *   `peer-group-name = "downlink6"`: `config/peer-as = 64498`, `route-reflector/config/route-reflector-client = true`, `route-reflector/config/route-reflector-cluster-id = "192.0.2.1"`, AFI-SAFI `IPV6_UNICAST` `enabled = true`.
*   Create eBGP neighbors over `lag1.30` and `lag1.40` towards the ATE:
    *   `neighbor-address = "198.51.100.10"`: `peer-group = "uplink"`, `import-policy = ["test-import-policy_aigp_150"]`, `IPV4_UNICAST` `enabled = true`, `enable-aigp = true`.
    *   `neighbor-address = "198.51.100.14"`: `peer-group = "uplink"`, `import-policy = ["test-import-policy_aigp_20"]`, `IPV4_UNICAST` `enabled = true`, `enable-aigp = true`.
    *   `neighbor-address = "2001:db8::10"`: `peer-group = "uplink6"`, `import-policy = ["test-import-policy_aigp_150"]`, `IPV6_UNICAST` `enabled = true`, `enable-aigp = true`.
    *   `neighbor-address = "2001:db8::14"`: `peer-group = "uplink6"`, `import-policy = ["test-import-policy_aigp_20"]`, `IPV6_UNICAST` `enabled = true`, `enable-aigp = true`.
*   Create iBGP neighbors over `lag2.20` towards DUT 2:
    *   `neighbor-address = "198.51.100.22"`: `peer-group = "downlink"`, `export-policy = ["test-export-policy"]`, `IPV4_UNICAST` `enabled = true`, `enable-aigp = true`.
    *   `neighbor-address = "2001:db8::22"`: `peer-group = "downlink6"`, `export-policy = ["test-export-policy"]`, `IPV6_UNICAST` `enabled = true`, `enable-aigp = true`.

#### DUT 2 - Configuration

##### Interface and Network-Instance Configuration

*   Create aggregate interface `/interfaces/interface[name=lag2]` (`type = iana-if-type:ieee8023adLag`, `enabled = true`), configure `/lacp/interfaces/interface[name=lag2]` with `lacp-mode = PASSIVE` and `interval = FAST`, and bind `dut2:port1` to `lag2` (`/interfaces/interface[name=port1]/ethernet/config/aggregate-id = "lag2"`).
*   Create non-default network instance `/network-instances/network-instance[name=test-instance]` (`type = openconfig-network-instance-types:L3VRF`, `enabled = true`) and initialize protocol tables (`DIRECTLY_CONNECTED`, `BGP`, `ISIS` for `IPV4` and `IPV6`) in both `DEFAULT` and `test-instance` NIs.
*   On `lag2`, create subinterface `index = 10` (`lag2.10`, `vlan-id = 10`), associate it with `DEFAULT` NI, and configure IPv4 `198.51.100.18/30` and IPv6 `2001:db8::18/126` per Table 3.
*   On `lag2`, create subinterface `index = 20` (`lag2.20`, `vlan-id = 20`), associate it with `test-instance` NI, and configure IPv4 `198.51.100.22/30` and IPv6 `2001:db8::22/126` per Table 3.

##### BGP Configuration on DUT 2

*   In `DEFAULT` NI:
    *   Configure BGP with `/bgp/global/config/as = 64497`, `/bgp/global/config/router-id = "192.0.2.104"`, and enable global AFI-SAFIs `IPV4_UNICAST` and `IPV6_UNICAST`.
    *   Create peer-groups `uplink` (`peer-as = 64497`, `IPV4_UNICAST` `enabled = true`) and `uplink6` (`peer-as = 64497`, `IPV6_UNICAST` `enabled = true`).
    *   Create iBGP neighbors `198.51.100.17` (`peer-group = "uplink"`, `IPV4_UNICAST` `enable-aigp = true`) and `2001:db8::17` (`peer-group = "uplink6"`, `IPV6_UNICAST` `enable-aigp = true`).
*   In `test-instance` NI:
    *   Configure BGP with `/bgp/global/config/as = 64498`, `/bgp/global/config/router-id = "192.0.2.105"`, and enable global AFI-SAFIs `IPV4_UNICAST` and `IPV6_UNICAST`.
    *   Create peer-groups `uplink` (`peer-as = 64498`, `IPV4_UNICAST` `enabled = true`) and `uplink6` (`peer-as = 64498`, `IPV6_UNICAST` `enabled = true`).
    *   Create iBGP neighbors `198.51.100.21` (`peer-group = "uplink"`, `IPV4_UNICAST` `enable-aigp = true`) and `2001:db8::21` (`peer-group = "uplink6"`, `IPV6_UNICAST` `enable-aigp = true`).

#### ATE - Configuration

1.  Create LACP aggregate interface `lag1` (`ACTIVE` mode) on `ate:port1`.
2.  Create 4 emulated router Ethernet subinterfaces (`eth1.10`, `eth1.20`, `eth1.30`, `eth1.40`) on `lag1` with VLAN IDs `10`, `20`, `30`, `40`, MAC addresses per Table 8, and IPv4/IPv6 addresses and gateways per Table 1 and Table 2.
3.  Configure 8 eBGP peers (local AS `64496`) towards DUT 1:
    *   `eth1.10`: IPv4 peer `198.51.100.1` and IPv6 peer `2001:db8::1` (remote AS `64497`)
    *   `eth1.20`: IPv4 peer `198.51.100.5` and IPv6 peer `2001:db8::5` (remote AS `64497`)
    *   `eth1.30`: IPv4 peer `198.51.100.9` and IPv6 peer `2001:db8::9` (remote AS `64498`)
    *   `eth1.40`: IPv4 peer `198.51.100.13` and IPv6 peer `2001:db8::13` (remote AS `64498`)
4.  Configure each ATE eBGP peer to advertise the **1,000 IPv4 prefixes** (`198.18.0.0/24` – `198.21.231.0/24`, prefix count `1000`) and **1,000 IPv6 prefixes** (`2001:db8:1000::/64` – `2001:db8:13e7::/64`, prefix count `1000`) per Table 7.
5.  Configure traffic flows `Flow 1`, `Flow 2`, `Flow 3`, and `Flow 4` according to Table 7.

#### TODO: https://github.com/openconfig/public/pull/1451 - Add AIGP enable and routing-policy set-aigp paths

```json
{
  "routing-policy": {
    "policy-definitions": {
      "policy-definition": [
        {
          "name": "test-import-policy_aigp_20",
          "config": {
            "name": "test-import-policy_aigp_20"
          },
          "statements": {
            "statement": [
              {
                "name": "test-import-statement",
                "config": {
                  "name": "test-import-statement"
                },
                "actions": {
                  "bgp-actions": {
                    "config": {
                      "set-aigp": 20
                    }
                  },
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                }
              }
            ]
          }
        }
      ]
    }
  },
  "network-instances": {
    "network-instance": [
      {
        "name": "DEFAULT",
        "config": {
          "name": "DEFAULT"
        },
        "protocols": {
          "protocol": [
            {
              "identifier": "BGP",
              "name": "DEFAULT",
              "bgp": {
                "neighbors": {
                  "neighbor": [
                    {
                      "neighbor-address": "198.51.100.6",
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                              "enabled": true,
                              "enable-aigp": true
                            }
                          }
                        ]
                      }
                    }
                  ]
                }
              }
            }
          ]
        }
      }
    ]
  }
}
```

*   **Step 2 - Push configuration to DUT using gnmi.Set**
    *   Push the generated OpenConfig configuration to DUT 1 and DUT 2 using `gNMI.Set` (`union_replace: true`).
    *   Push the OTG configuration to the ATE and start OTG protocols.
    *   Use `gNMI.Subscribe` (`ON_CHANGE` mode) on DUT 1 and DUT 2 (instead of static sleeps) to await control-plane and FIB convergence before starting traffic:
        1.  Wait until `/interfaces/interface[name=lag1]/state/oper-status == UP` and `/interfaces/interface[name=lag2]/state/oper-status == UP` on DUT 1 and DUT 2.
        2.  Wait until `/network-instances/network-instance/protocols/protocol[identifier=BGP][name=DEFAULT]/bgp/neighbors/neighbor/state/session-state == ESTABLISHED` for all configured eBGP and iBGP peers in `DEFAULT` and `test-instance` NIs on DUT 1 and DUT 2.
        3.  Wait until `/network-instances/network-instance/protocols/protocol[identifier=BGP][name=DEFAULT]/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/installed == 1000` on DUT 1 (for winning peers `198.51.100.6`, `2001:db8::6` in `DEFAULT` NI and `198.51.100.14`, `2001:db8::14` in `test-instance` NI) and on DUT 2 (for iBGP peers `198.51.100.17`, `2001:db8::17` in `DEFAULT` NI and `198.51.100.21`, `2001:db8::21` in `test-instance` NI).
        4.  Verify via `gNMI.Get` / `gNMI.Subscribe` on `/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group` and `/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group` that all 1,000 IPv4 and 1,000 IPv6 prefixes are installed in DUT 1's AFT pointing to the next-hop group of `198.51.100.6` / `2001:db8::6` (`DEFAULT` NI) and `198.51.100.14` / `2001:db8::14` (`test-instance` NI).

*   **Step 3 - Send Traffic**
    *   Record baseline `/interfaces/interface[name=lag1]/subinterfaces/subinterface[index=10|20|30|40]/state/counters/out-unicast-pkts` on DUT 1.
    *   Start OTG traffic flows `Flow 1`, `Flow 2`, `Flow 3`, and `Flow 4` (each transmitting `fixed_packets = 100000` at `10000 pps`).
    *   Poll OTG flow telemetry until `transmit == stopped` for all 4 flows.

*   **Step 4 - Validation with pass/fail criteria**

##### Pass Criteria on DUT 1

1.  `/interfaces/interface/state/admin-status` and `/interfaces/interface/state/oper-status` must be `UP` for `port1`, `port2`, `lag1`, `lag2`, `Loopback10`, and `Loopback20`.
2.  In `DEFAULT` NI:
    *   `/bgp/neighbors/neighbor[neighbor-address=<peer>]/state/session-state` must be `ESTABLISHED` for `198.51.100.2`, `2001:db8::2`, `198.51.100.6`, `2001:db8::6`, `198.51.100.18`, and `2001:db8::18`.
    *   `/bgp/neighbors/neighbor[neighbor-address=<peer>]/afi-safis/afi-safi/state/enable-aigp` must be `true` for all 6 neighbors.
    *   For all 1,000 IPv4 (`198.18.0.0/24`–`198.21.231.0/24`) and 1,000 IPv6 (`2001:db8:1000::/64`–`2001:db8:13e7::/64`) routes in `adj-rib-in-post/routes/route`:
        *   Routes from `198.51.100.2` and `2001:db8::2` must resolve via `state/attr-index` to `/bgp/rib/attr-sets/attr-set[index=<attr-index>]/state/aigp == 150` and have `state/best-path == false`.
        *   Routes from `198.51.100.6` and `2001:db8::6` must resolve via `state/attr-index` to `/bgp/rib/attr-sets/attr-set[index=<attr-index>]/state/aigp == 20` and have `state/best-path == true`.
3.  In `test-instance` NI:
    *   `/bgp/neighbors/neighbor[neighbor-address=<peer>]/state/session-state` must be `ESTABLISHED` and `state/enable-aigp` must be `true` for `198.51.100.10`, `2001:db8::10`, `198.51.100.14`, `2001:db8::14`, `198.51.100.22`, and `2001:db8::22`.
    *   All 1,000 IPv4 and 1,000 IPv6 routes from `198.51.100.10` and `2001:db8::10` must have `state/aigp == 150` and `state/best-path == false`.
    *   All 1,000 IPv4 and 1,000 IPv6 routes from `198.51.100.14` and `2001:db8::14` must have `state/aigp == 20` and `state/best-path == true`.
4.  Subinterface counter validation on DUT 1:
    *   `/interfaces/interface[name=lag1]/subinterfaces/subinterface[index=20]/state/counters/out-unicast-pkts` must increment by at least `200,000` packets (`Flow 1` + `Flow 3`), while `subinterface[index=10]/state/counters/out-unicast-pkts` must not increment for data-plane traffic (`delta == 0`).
    *   `/interfaces/interface[name=lag1]/subinterfaces/subinterface[index=40]/state/counters/out-unicast-pkts` must increment by at least `200,000` packets (`Flow 2` + `Flow 4`), while `subinterface[index=30]/state/counters/out-unicast-pkts` must not increment for data-plane traffic (`delta == 0`).

##### Pass Criteria on DUT 2

1.  In `DEFAULT` NI:
    *   Peers `198.51.100.17` and `2001:db8::17` must have `state/session-state == ESTABLISHED`, `state/enable-aigp == true`, and `state/prefixes/installed == 1000`.
    *   All 1,000 IPv4 and 1,000 IPv6 routes received in `adj-rib-in-post` from `198.51.100.17` and `2001:db8::17` must resolve via `state/attr-index` to `state/aigp == 200` and `state/next-hop == "198.51.100.17"` (IPv4) / `"2001:db8::17"` (IPv6).
2.  In `test-instance` NI:
    *   Peers `198.51.100.21` and `2001:db8::21` must have `state/session-state == ESTABLISHED`, `state/enable-aigp == true`, and `state/prefixes/installed == 1000`.
    *   All 1,000 IPv4 and 1,000 IPv6 routes received in `adj-rib-in-post` from `198.51.100.21` and `2001:db8::21` must resolve via `state/attr-index` to `state/aigp == 200` and `state/next-hop == "198.51.100.21"` (IPv4) / `"2001:db8::21"` (IPv6).

##### Pass Criteria on ATE

*   For each of `Flow 1`, `Flow 2`, `Flow 3`, and `Flow 4`, `frames_rx` on the expected egress interface (`eth1.20` for `Flow 1` and `Flow 3`; `eth1.40` for `Flow 2` and `Flow 4`) must equal `frames_tx` (`100,000` packets, **0% packet loss**), and `0` flow packets must be received on the non-selected path (`eth1.10` and `eth1.30`).

#### Canonical OC

```json
{
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "DEFAULT"
        },
        "interfaces": {
          "interface": [
            {
              "config": {
                "id": "Loopback10.0",
                "interface": "Loopback10",
                "subinterface": 0
              }
            }
          ]
        },
        "protocols": {
          "protocol": [
            {
              "identifier": "BGP",
              "config": {
                "identifier": "BGP",
                "name": "DEFAULT"
              },
              "bgp": {
                "global": {
                  "config": {
                    "as": 64497
                  }
                },
                "peer-groups": {
                  "peer-group": [
                    {
                      "peer-group-name": "uplink",
                      "config": {
                        "peer-group-name": "uplink",
                        "peer-as": 64496
                      }
                    },
                    {
                      "peer-group-name": "uplink6",
                      "config": {
                        "peer-group-name": "uplink6",
                        "peer-as": 64496
                      }
                    },
                    {
                      "peer-group-name": "downlink",
                      "config": {
                        "peer-group-name": "downlink",
                        "peer-as": 64497
                      },
                      "route-reflector": {
                        "config": {
                          "route-reflector-client": true,
                          "route-reflector-cluster-id": "192.0.2.1"
                        }
                      }
                    },
                    {
                      "peer-group-name": "downlink6",
                      "config": {
                        "peer-group-name": "downlink6",
                        "peer-as": 64497
                      },
                      "route-reflector": {
                        "config": {
                          "route-reflector-client": true,
                          "route-reflector-cluster-id": "192.0.2.1"
                        }
                      }
                    }
                  ]
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "config": {
                        "neighbor-address": "198.51.100.2",
                        "peer-group": "uplink"
                      },
                      "apply-policy": {
                        "config": {
                          "import-policy": [
                            "test-import-policy_aigp_150"
                          ]
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "198.51.100.6",
                        "peer-group": "uplink"
                      },
                      "apply-policy": {
                        "config": {
                          "import-policy": [
                            "test-import-policy_aigp_20"
                          ]
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "2001:db8::2",
                        "peer-group": "uplink6"
                      },
                      "apply-policy": {
                        "config": {
                          "import-policy": [
                            "test-import-policy_aigp_150"
                          ]
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "2001:db8::6",
                        "peer-group": "uplink6"
                      },
                      "apply-policy": {
                        "config": {
                          "import-policy": [
                            "test-import-policy_aigp_20"
                          ]
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "198.51.100.18",
                        "peer-group": "downlink"
                      },
                      "apply-policy": {
                        "config": {
                          "export-policy": [
                            "test-export-policy"
                          ]
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "2001:db8::18",
                        "peer-group": "downlink6"
                      },
                      "apply-policy": {
                        "config": {
                          "export-policy": [
                            "test-export-policy"
                          ]
                        }
                      }
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
          "name": "test-instance"
        },
        "interfaces": {
          "interface": [
            {
              "config": {
                "id": "Loopback20.0",
                "interface": "Loopback20",
                "subinterface": 0
              }
            }
          ]
        },
        "protocols": {
          "protocol": [
            {
              "identifier": "BGP",
              "config": {
                "identifier": "BGP",
                "name": "DEFAULT"
              },
              "bgp": {
                "global": {
                  "config": {
                    "as": 64498
                  }
                },
                "peer-groups": {
                  "peer-group": [
                    {
                      "peer-group-name": "uplink",
                      "config": {
                        "peer-group-name": "uplink",
                        "peer-as": 64496
                      }
                    },
                    {
                      "peer-group-name": "uplink6",
                      "config": {
                        "peer-group-name": "uplink6",
                        "peer-as": 64496
                      }
                    },
                    {
                      "peer-group-name": "downlink",
                      "config": {
                        "peer-group-name": "downlink",
                        "peer-as": 64498
                      },
                      "route-reflector": {
                        "config": {
                          "route-reflector-client": true,
                          "route-reflector-cluster-id": "192.0.2.1"
                        }
                      }
                    },
                    {
                      "peer-group-name": "downlink6",
                      "config": {
                        "peer-group-name": "downlink6",
                        "peer-as": 64498
                      },
                      "route-reflector": {
                        "config": {
                          "route-reflector-client": true,
                          "route-reflector-cluster-id": "192.0.2.1"
                        }
                      }
                    }
                  ]
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "config": {
                        "neighbor-address": "198.51.100.10",
                        "peer-group": "uplink"
                      },
                      "apply-policy": {
                        "config": {
                          "import-policy": [
                            "test-import-policy_aigp_150"
                          ]
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "198.51.100.14",
                        "peer-group": "uplink"
                      },
                      "apply-policy": {
                        "config": {
                          "import-policy": [
                            "test-import-policy_aigp_20"
                          ]
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "2001:db8::10",
                        "peer-group": "uplink6"
                      },
                      "apply-policy": {
                        "config": {
                          "import-policy": [
                            "test-import-policy_aigp_150"
                          ]
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "2001:db8::14",
                        "peer-group": "uplink6"
                      },
                      "apply-policy": {
                        "config": {
                          "import-policy": [
                            "test-import-policy_aigp_20"
                          ]
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "198.51.100.22",
                        "peer-group": "downlink"
                      },
                      "apply-policy": {
                        "config": {
                          "export-policy": [
                            "test-export-policy"
                          ]
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "2001:db8::22",
                        "peer-group": "downlink6"
                      },
                      "apply-policy": {
                        "config": {
                          "export-policy": [
                            "test-export-policy"
                          ]
                        }
                      }
                    }
                  ]
                }
              }
            }
          ]
        }
      }
    ]
  },
  "routing-policy": {
    "policy-definitions": {
      "policy-definition": [
        {
          "config": {
            "name": "test-import-policy_aigp_150"
          },
          "statements": {
            "statement": [
              {
                "config": {
                  "name": "test-import-statement"
                },
                "actions": {
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                }
              }
            ]
          }
        },
        {
          "config": {
            "name": "test-import-policy_aigp_20"
          },
          "statements": {
            "statement": [
              {
                "config": {
                  "name": "test-import-statement"
                },
                "actions": {
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                }
              }
            ]
          }
        },
        {
          "config": {
            "name": "test-export-policy"
          },
          "statements": {
            "statement": [
              {
                "config": {
                  "name": "test-export-statement"
                },
                "actions": {
                  "bgp-actions": {
                    "config": {
                      "set-next-hop": "SELF"
                    }
                  },
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                }
              }
            ]
          }
        }
      ]
    }
  },
  "interfaces": {
    "interface": [
      {
        "name": "Loopback10",
        "config": {
          "name": "Loopback10",
          "type": "iana-if-type:softwareLoopback",
          "enabled": true
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.10",
                        "prefix-length": 32
                      }
                    }
                  ]
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:10::1",
                        "prefix-length": 128
                      }
                    }
                  ]
                }
              }
            }
          ]
        }
      },
      {
        "name": "Loopback20",
        "config": {
          "name": "Loopback20",
          "type": "iana-if-type:softwareLoopback",
          "enabled": true
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.20",
                        "prefix-length": 32
                      }
                    }
                  ]
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:20::1",
                        "prefix-length": 128
                      }
                    }
                  ]
                }
              }
            }
          ]
        }
      }
    ]
  }
}
```

---

### RT-1.37.2 - Validate AS-PATH Attribute as Tie-Breaker When AIGP Metric is Equal

The following configuration steps build sequentially upon test case `RT-1.37.1` above.

*   **Step 1 - Generate DUT configuration**
    *   On DUT 1, modify routing policy `/routing-policy/policy-definitions/policy-definition[name=test-import-policy_aigp_20]/statements/statement[name=test-import-statement]`:
        *   Update `/actions/bgp-actions/config/set-aigp` from `20` to `150` so that both ATE eBGP paths into each network instance have an identical AIGP metric of `150`.
        *   Configure `/actions/bgp-actions/set-as-path-prepend/config/asn = 64496` and `/actions/bgp-actions/set-as-path-prepend/config/repeat-n = 5` to prepend ASN `64496` `5` times on routes received over `lag1.20` (`198.51.100.6`, `2001:db8::6`) and `lag1.40` (`198.51.100.14`, `2001:db8::14`).
        *   Keep `/actions/config/policy-result = ACCEPT_ROUTE`.

*   **Step 2 - Push configuration to DUT using gnmi.Set**
    *   Push the updated `test-import-policy_aigp_20` configuration to DUT 1 using `gNMI.Set` (`UPDATE` / `union_replace: true`).
    *   Use `gNMI.Subscribe` (`ON_CHANGE` mode) on DUT 1 to await control-plane and FIB convergence before starting traffic:
        1.  Wait until `/network-instances/network-instance/protocols/protocol[identifier=BGP][name=DEFAULT]/bgp/rib/afi-safis/afi-safi/ipv4-unicast/neighbors/neighbor[neighbor-address=198.51.100.2]/adj-rib-in-post/routes/route/state/best-path == true` and `ipv6-unicast/neighbors/neighbor[neighbor-address=2001:db8::2]/adj-rib-in-post/routes/route/state/best-path == true` in `DEFAULT` NI (and `198.51.100.10` / `2001:db8::10` in `test-instance` NI) for all 1,000 IPv4 and 1,000 IPv6 prefixes.
        2.  Verify via `/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group` and `/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group` that all 1,000 IPv4 and 1,000 IPv6 AFT entries have updated their next-hop group to point to `198.51.100.2` / `2001:db8::2` (`lag1.10`) in `DEFAULT` NI and `198.51.100.10` / `2001:db8::10` (`lag1.30`) in `test-instance` NI.

*   **Step 3 - Send Traffic**
    *   Record baseline `/interfaces/interface[name=lag1]/subinterfaces/subinterface[index=10|20|30|40]/state/counters/out-unicast-pkts` on DUT 1.
    *   Start OTG traffic flows `Flow 1`, `Flow 2`, `Flow 3`, and `Flow 4` (each transmitting `fixed_packets = 100000` at `10000 pps`).
    *   Poll OTG flow telemetry until `transmit == stopped` for all 4 flows.

*   **Step 4 - Validation with pass/fail criteria**

##### Pass Criteria on DUT 1

1.  All BGP peers must remain in `ESTABLISHED` state.
2.  In `DEFAULT` NI:
    *   `/bgp/neighbors/neighbor/afi-safis/afi-safi/state/enable-aigp` must be `true` for peers `198.51.100.2`, `2001:db8::2`, `198.51.100.6`, and `2001:db8::6`.
    *   All 1,000 IPv4 and 1,000 IPv6 routes received in `adj-rib-in-post` from `198.51.100.2`, `2001:db8::2`, `198.51.100.6`, and `2001:db8::6` must resolve via `state/attr-index` to `/bgp/rib/attr-sets/attr-set[index=<attr-index>]/state/aigp == 150`.
    *   Because the AIGP metric is tied at `150` and routes from `198.51.100.6` and `2001:db8::6` have a longer AS-PATH (`64496` prepended 5 additional times), all 1,000 IPv4 and 1,000 IPv6 routes received from `198.51.100.2` and `2001:db8::2` must have `state/best-path == true`, while routes from `198.51.100.6` and `2001:db8::6` must have `state/best-path == false`.
3.  In `test-instance` NI:
    *   `/bgp/neighbors/neighbor/afi-safis/afi-safi/state/enable-aigp` must be `true` for peers `198.51.100.10`, `2001:db8::10`, `198.51.100.14`, and `2001:db8::14`.
    *   All 1,000 IPv4 and 1,000 IPv6 routes received in `adj-rib-in-post` from `198.51.100.10`, `2001:db8::10`, `198.51.100.14`, and `2001:db8::14` must resolve via `state/attr-index` to `state/aigp == 150`.
    *   All 1,000 IPv4 and 1,000 IPv6 routes received from `198.51.100.10` and `2001:db8::10` must have `state/best-path == true`, while routes from `198.51.100.14` and `2001:db8::14` must have `state/best-path == false`.
4.  Subinterface counter validation on DUT 1:
    *   `/interfaces/interface[name=lag1]/subinterfaces/subinterface[index=10]/state/counters/out-unicast-pkts` must increment by at least `200,000` packets (`Flow 1` + `Flow 3`), while `subinterface[index=20]/state/counters/out-unicast-pkts` must not increment for data-plane traffic (`delta == 0`).
    *   `/interfaces/interface[name=lag1]/subinterfaces/subinterface[index=30]/state/counters/out-unicast-pkts` must increment by at least `200,000` packets (`Flow 2` + `Flow 4`), while `subinterface[index=40]/state/counters/out-unicast-pkts` must not increment for data-plane traffic (`delta == 0`).

##### Pass Criteria on ATE

*   For each of `Flow 1`, `Flow 2`, `Flow 3`, and `Flow 4`, `frames_rx` on the new winning egress interface (`eth1.10` for `Flow 1` and `Flow 3`; `eth1.30` for `Flow 2` and `Flow 4`) must equal `frames_tx` (`100,000` packets, **0% packet loss**), and `0` flow packets must be received on the prepended path (`eth1.20` and `eth1.40`).

#### Canonical OC

```json
{
  "routing-policy": {
    "policy-definitions": {
      "policy-definition": [
        {
          "config": {
            "name": "test-import-policy_aigp_20"
          },
          "statements": {
            "statement": [
              {
                "config": {
                  "name": "test-import-statement"
                },
                "actions": {
                  "bgp-actions": {
                    "set-as-path-prepend": {
                      "config": {
                        "repeat-n": 5,
                        "asn": 64496
                      }
                    }
                  },
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                }
              }
            ]
          }
        }
      ]
    }
  }
}
```

---

### RT-1.37.3 - Validate AIGP Enabled by Default on iBGP Peers and IGP Metric Addition on Next-Hop-Self

The following configuration steps build sequentially upon test case `RT-1.37.2` above (keeping the ATE eBGP sessions on `lag1` active). This subtest validates that AIGP is enabled by default on newly created iBGP neighbors (without explicit `enable-aigp` configuration) and that when a route reflector (DUT 1) re-advertises an AIGP route with `set-next-hop: SELF`, the IS-IS Level 2 IGP cost (`30`) to the original BGP next-hop (`Loopback30` in `test-originate` NI) is added to the received AIGP metric (`200`), resulting in an advertised AIGP metric of `230` across all route-reflector clients.

#### Effective Emulated Test Topology

```text
+--------------------------+ lag2 (VLAN 30 & 40) [iBGP] +-------------+          lag2 [iBGP]          +---------+
| DUT 2 (test-originate NI)|============================|  DUT 1 (RR) |===============================|  DUT 2  |
+--------------------------+            IS-IS           +-------------+             IS-IS             +---------+
```

*   **Step 1 - Generate DUT configuration**

##### Interface Configuration on DUT 1

*   On `/interfaces/interface[name=lag2]`, create subinterface `index = 30` (`lag2.30`, `vlan-id = 30`), associate it with `/network-instances/network-instance[name=DEFAULT]/interfaces/interface`, and configure IPv4 (`198.51.100.25/30`) and IPv6 (`2001:db8::25/126`) addresses per Table 4.
*   On `/interfaces/interface[name=lag2]`, create subinterface `index = 40` (`lag2.40`, `vlan-id = 40`), associate it with `/network-instances/network-instance[name=test-instance]/interfaces/interface`, and configure IPv4 (`198.51.100.29/30`) and IPv6 (`2001:db8::29/126`) addresses per Table 4.

##### Routing Policy Configuration on DUT 1

*   Create routing policy `/routing-policy/policy-definitions/policy-definition[name=export-next-hop-self]`:
    *   Statement `name = "test-statement"`:
    *   Set `/actions/bgp-actions/config/set-next-hop = "SELF"` and `/actions/config/policy-result = ACCEPT_ROUTE`.

##### IS-IS Configuration on DUT 1

*   In `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=DEFAULT]`:
    *   Set `/isis/global/config/net = ["49.0001.1980.5110.0025.00"]` (Table 6), `/isis/global/config/level-capability = LEVEL_2`, and `/isis/levels/level[level-number=2]/config/metric-style = WIDE_METRIC`.
    *   Add `/isis/interfaces/interface[interface-id=Loopback10.0]` (`config/enabled = true`).
    *   Add `/isis/interfaces/interface[interface-id=lag2.10]` and `/isis/interfaces/interface[interface-id=lag2.30]` with:
        *   `config/circuit-type = POINT_TO_POINT`, `config/enabled = true`
        *   `levels/level[level-number=2]/config/enabled = true`
        *   `levels/level[level-number=2]/afi-safi/af[afi-name=openconfig-isis-types:IPV4][safi-name=openconfig-isis-types:UNICAST]/config/metric = 30`
        *   `levels/level[level-number=2]/afi-safi/af[afi-name=openconfig-isis-types:IPV6][safi-name=openconfig-isis-types:UNICAST]/config/metric = 30`
*   In `/network-instances/network-instance[name=test-instance]/protocols/protocol[identifier=ISIS][name=DEFAULT]`:
    *   Set `/isis/global/config/net = ["49.0001.1980.5110.0029.00"]` (Table 6), `/isis/global/config/level-capability = LEVEL_2`, and `/isis/levels/level[level-number=2]/config/metric-style = WIDE_METRIC`.
    *   Add `/isis/interfaces/interface[interface-id=Loopback20.0]` (`config/enabled = true`).
    *   Add `/isis/interfaces/interface[interface-id=lag2.20]` and `/isis/interfaces/interface[interface-id=lag2.40]` with `circuit-type = POINT_TO_POINT`, `enabled = true`, `level-number = 2` `enabled = true`, and metric `30` for both `IPV4` `UNICAST` and `IPV6` `UNICAST`.

##### BGP Configuration on DUT 1

*   In `DEFAULT` NI:
    *   Delete point-to-point iBGP neighbors `198.51.100.18` and `2001:db8::18`.
    *   Create iBGP neighbors towards DUT 2 `test-originate` NI (`Loopback30`) with `/bgp/neighbors/neighbor/transport/config/local-address = "Loopback10.0"` (`192.0.2.10` / `2001:db8:10::1`) and **do not** explicitly configure `enable-aigp`:
        *   `neighbor-address = "192.0.2.103"`, `peer-group = "downlink"`, `afi-safis/afi-safi[afi-safi-name=IPV4_UNICAST]/config/enabled = true`
        *   `neighbor-address = "2001:db8:30::2"`, `peer-group = "downlink6"`, `afi-safis/afi-safi[afi-safi-name=IPV6_UNICAST]/config/enabled = true`
    *   Create two pairs of iBGP route-reflector client neighbors towards DUT 2 `DEFAULT` NI (`Loopback40` Client A and `Loopback41` Client B) with `transport/config/local-address = "Loopback10.0"`, `apply-policy/config/export-policy = ["export-next-hop-self"]`, and **do not** explicitly configure `enable-aigp`:
        *   Client A: `neighbor-address = "192.0.2.104"` (`peer-group = "downlink"`) and `neighbor-address = "2001:db8:40::2"` (`peer-group = "downlink6"`)
        *   Client B: `neighbor-address = "192.0.2.106"` (`peer-group = "downlink"`) and `neighbor-address = "2001:db8:41::2"` (`peer-group = "downlink6"`)
*   In `test-instance` NI:
    *   Delete point-to-point iBGP neighbors `198.51.100.22` and `2001:db8::22`.
    *   Create iBGP neighbors towards DUT 2 `test-originate` NI (`Loopback30`) with `transport/config/local-address = "Loopback20.0"` (`192.0.2.20` / `2001:db8:20::1`) and **do not** explicitly configure `enable-aigp`:
        *   `neighbor-address = "192.0.2.103"`, `peer-group = "downlink"`
        *   `neighbor-address = "2001:db8:30::2"`, `peer-group = "downlink6"`
    *   Create two pairs of iBGP route-reflector client neighbors towards DUT 2 `test-instance` NI (`Loopback50` Client A and `Loopback51` Client B) with `transport/config/local-address = "Loopback20.0"`, `apply-policy/config/export-policy = ["export-next-hop-self"]`, and **do not** explicitly configure `enable-aigp`:
        *   Client A: `neighbor-address = "192.0.2.105"` (`peer-group = "downlink"`) and `neighbor-address = "2001:db8:50::2"` (`peer-group = "downlink6"`)
        *   Client B: `neighbor-address = "192.0.2.107"` (`peer-group = "downlink"`) and `neighbor-address = "2001:db8:51::2"` (`peer-group = "downlink6"`)

##### Interface and Network-Instance Configuration on DUT 2

*   Create non-default network instance `/network-instances/network-instance[name=test-originate]` (`config/type = openconfig-network-instance-types:L3VRF`, `config/enabled = true`) and initialize protocol tables (`DIRECTLY_CONNECTED`, `BGP`, `ISIS` for `IPV4` and `IPV6`).
*   On `/interfaces/interface[name=lag2]`, create subinterfaces `index = 30` (`lag2.30`, `vlan-id = 30`) and `index = 40` (`lag2.40`, `vlan-id = 40`), associate both with `test-originate` NI, and configure IPv4 and IPv6 addresses per Table 4.
*   Create loopback interfaces `Loopback10`, `Loopback20`, `Loopback30`, `Loopback40`, `Loopback41`, `Loopback50`, and `Loopback51` (`type = iana-if-type:softwareLoopback`, `enabled = true`) with IPv4 and IPv6 addresses per Table 9:
    *   Associate `Loopback10.0`, `Loopback20.0`, and `Loopback30.0` with `test-originate` NI.
    *   Associate `Loopback40.0` and `Loopback41.0` with `DEFAULT` NI.
    *   Associate `Loopback50.0` and `Loopback51.0` with `test-instance` NI.

##### Routing Policy Configuration on DUT 2

*   Create prefix-set `/routing-policy/defined-sets/prefix-sets/prefix-set[name=Loopback-prefix-v4]` (`config/mode = IPV4`):
    *   Prefix 1: `ip-prefix = "192.0.2.101/32"`, `masklength-range = "exact"`
    *   Prefix 2: `ip-prefix = "192.0.2.102/32"`, `masklength-range = "exact"`
*   Create prefix-set `/routing-policy/defined-sets/prefix-sets/prefix-set[name=Loopback-prefix-v6]` (`config/mode = IPV6`):
    *   Prefix 1: `ip-prefix = "2001:db8:10::2/128"`, `masklength-range = "exact"`
    *   Prefix 2: `ip-prefix = "2001:db8:20::2/128"`, `masklength-range = "exact"`
*   Create routing policy `/routing-policy/policy-definitions/policy-definition[name=test-export-policy]`:
    *   Statement `name = "match-export-statement-v4"`: `conditions/match-prefix-set/config/prefix-set = "Loopback-prefix-v4"`, `actions/bgp-actions/config/set-aigp = 200`, `actions/config/policy-result = ACCEPT_ROUTE`.
    *   Statement `name = "match-export-statement-v6"`: `conditions/match-prefix-set/config/prefix-set = "Loopback-prefix-v6"`, `actions/bgp-actions/config/set-aigp = 200`, `actions/config/policy-result = ACCEPT_ROUTE`.

##### IS-IS Configuration on DUT 2

*   In `DEFAULT` NI:
    *   Create `/protocols/protocol[identifier=ISIS][name=DEFAULT]` with `net = ["49.0001.1980.5110.0026.00"]`, `level-capability = LEVEL_2`, `metric-style = WIDE_METRIC`.
    *   Add `Loopback40.0`, `Loopback41.0`, and `lag2.10` (`circuit-type = POINT_TO_POINT`, Level 2 metric `30` for `IPV4` `UNICAST` and `IPV6` `UNICAST`).
*   In `test-instance` NI:
    *   Create `/protocols/protocol[identifier=ISIS][name=DEFAULT]` with `net = ["49.0001.1980.5110.0030.00"]`, `level-capability = LEVEL_2`, `metric-style = WIDE_METRIC`.
    *   Add `Loopback50.0`, `Loopback51.0`, and `lag2.20` (`circuit-type = POINT_TO_POINT`, Level 2 metric `30` for `IPV4` `UNICAST` and `IPV6` `UNICAST`).
*   In `test-originate` NI:
    *   Create `/protocols/protocol[identifier=ISIS][name=DEFAULT]` with `net = ["49.0001.1980.5110.0100.00"]`, `level-capability = LEVEL_2`, `metric-style = WIDE_METRIC`.
    *   Add `Loopback30.0`, `lag2.30`, and `lag2.40` (`circuit-type = POINT_TO_POINT`, Level 2 metric `30` for `IPV4` `UNICAST` and `IPV6` `UNICAST`).

##### BGP and `table-connections` Configuration on DUT 2

*   In `DEFAULT` NI:
    *   Delete neighbors `198.51.100.17` and `2001:db8::17`.
    *   Create iBGP neighbors `192.0.2.10` (`peer-group = "uplink"`) and `2001:db8:10::1` (`peer-group = "uplink6"`) using `transport/config/local-address = "Loopback40.0"` without explicitly configuring `enable-aigp`.
*   In `test-instance` NI:
    *   Delete neighbors `198.51.100.21` and `2001:db8::21`.
    *   Create iBGP neighbors `192.0.2.20` (`peer-group = "uplink"`) and `2001:db8:20::1` (`peer-group = "uplink6"`) using `transport/config/local-address = "Loopback50.0"` without explicitly configuring `enable-aigp`.
*   In `test-originate` NI:
    *   Create `/protocols/protocol[identifier=DIRECTLY_CONNECTED][name=DEFAULT]`.
    *   Create `/protocols/protocol[identifier=BGP][name=DEFAULT]` with `/bgp/global/config/as = 64499`, `/bgp/global/config/router-id = "192.0.2.103"`, and global AFI-SAFIs `IPV4_UNICAST` and `IPV6_UNICAST` (`enabled = true`).
    *   Create peer-groups `default-peer-group` and `default-peer-group6` with `config/peer-as = 64497`, `config/local-as = 64497`, and `apply-policy/config/export-policy = ["test-export-policy"]`.
    *   Create peer-groups `test-instance-peer-group` and `test-instance-peer-group6` with `config/peer-as = 64498`, `config/local-as = 64498`, and `apply-policy/config/export-policy = ["test-export-policy"]`.
    *   Create iBGP neighbors `192.0.2.10` (`default-peer-group`), `2001:db8:10::1` (`default-peer-group6`), `192.0.2.20` (`test-instance-peer-group`), and `2001:db8:20::1` (`test-instance-peer-group6`) with `transport/config/local-address = "Loopback30.0"`, without explicitly configuring `enable-aigp`.
    *   Configure `/network-instances/network-instance[name=test-originate]/table-connections/table-connection`:
        *   Entry 1: `src-protocol = openconfig-policy-types:DIRECTLY_CONNECTED`, `dst-protocol = openconfig-policy-types:BGP`, `address-family = openconfig-types:IPV4`, `default-import-policy = ACCEPT_ROUTE`.
        *   Entry 2: `src-protocol = openconfig-policy-types:DIRECTLY_CONNECTED`, `dst-protocol = openconfig-policy-types:BGP`, `address-family = openconfig-types:IPV6`, `default-import-policy = ACCEPT_ROUTE`.

*   **Step 2 - Push configuration to DUT using gnmi.Set**
    *   Push the incremental configuration to DUT 1 and DUT 2 using `gNMI.Set` (`union_replace: true`).
    *   Use `gNMI.Subscribe` (`ON_CHANGE` mode) on DUT 1 and DUT 2 to await IS-IS and BGP convergence:
        1.  Wait until `/network-instances/network-instance/protocols/protocol[identifier=ISIS][name=DEFAULT]/isis/interfaces/interface/levels/level[level-number=2]/adjacencies/adjacency/state/adjacency-state == UP` on `lag2.10` and `lag2.30` (`DEFAULT` NI) and `lag2.20` and `lag2.40` (`test-instance` NI) on DUT 1, and in `DEFAULT`, `test-instance`, and `test-originate` NIs on DUT 2.
        2.  Wait until `/bgp/neighbors/neighbor/state/session-state == ESTABLISHED` on all iBGP neighbors across `DEFAULT`, `test-instance`, and `test-originate` NIs on DUT 1 and DUT 2.
        3.  Wait until `/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/installed >= 2` on DUT 2 (`DEFAULT` and `test-instance` NIs) for the reflected loopback prefixes (`192.0.2.101/32`, `192.0.2.102/32` and `2001:db8:10::2/128`, `2001:db8:20::2/128`).

*   **Step 3 - Send Traffic**
    *   Verify control-plane and RIB/AFT state via gNMI (ATE eBGP sessions remain active from `RT-1.37.2`).

*   **Step 4 - Validation with pass/fail criteria**

##### Pass Criteria on DUT 1

1.  All IS-IS Level 2 adjacencies on `lag2.10` and `lag2.30` (`DEFAULT` NI) and on `lag2.20` and `lag2.40` (`test-instance` NI) must have `state/adjacency-state == UP`.
2.  In both `DEFAULT` and `test-instance` NIs:
    *   `/bgp/peer-groups/peer-group[peer-group-name=downlink]/route-reflector/state/route-reflector-client` and `downlink6` `state/route-reflector-client` must be `true`, and `state/route-reflector-cluster-id` must be `"192.0.2.1"`.
    *   All iBGP peers (`192.0.2.103`, `2001:db8:30::2`, `192.0.2.104`, `2001:db8:40::2`, `192.0.2.106`, `2001:db8:41::2` in `DEFAULT` NI; `192.0.2.103`, `2001:db8:30::2`, `192.0.2.105`, `2001:db8:50::2`, `192.0.2.107`, `2001:db8:51::2` in `test-instance` NI) must have `state/session-state == ESTABLISHED`.
    *   `/bgp/neighbors/neighbor/afi-safis/afi-safi/state/enable-aigp` must be `true` by default on all iBGP peers.
    *   Routes `192.0.2.101/32`, `192.0.2.102/32` (from `192.0.2.103`) and `2001:db8:10::2/128`, `2001:db8:20::2/128` (from `2001:db8:30::2`) in `adj-rib-in-post` must resolve via `state/attr-index` to `/bgp/rib/attr-sets/attr-set[index=<attr-index>]/state/aigp == 200`.

##### Pass Criteria on DUT 2

1.  All IS-IS Level 2 adjacencies in `DEFAULT`, `test-instance`, and `test-originate` NIs must have `state/adjacency-state == UP`.
2.  In `DEFAULT` NI:
    *   Peers `192.0.2.10` and `2001:db8:10::1` must have `state/session-state == ESTABLISHED` and `state/enable-aigp == true` by default.
    *   Reflected routes `192.0.2.101/32`, `192.0.2.102/32` (from `192.0.2.10`) and `2001:db8:10::2/128`, `2001:db8:20::2/128` (from `2001:db8:10::1`) in `adj-rib-in-post` must resolve via `state/attr-index` to `state/aigp == 230` (`200` + IS-IS Level 2 metric `30` to `192.0.2.103` / `2001:db8:30::2`) and `state/next-hop == "192.0.2.10"` (IPv4) / `"2001:db8:10::1"` (IPv6).
3.  In `test-instance` NI:
    *   Peers `192.0.2.20` and `2001:db8:20::1` must have `state/session-state == ESTABLISHED` and `state/enable-aigp == true` by default.
    *   Reflected routes `192.0.2.101/32`, `192.0.2.102/32` (from `192.0.2.20`) and `2001:db8:10::2/128`, `2001:db8:20::2/128` (from `2001:db8:20::1`) in `adj-rib-in-post` must resolve via `state/attr-index` to `state/aigp == 230` (`200` + IS-IS Level 2 metric `30`) and `state/next-hop == "192.0.2.20"` (IPv4) / `"2001:db8:20::1"` (IPv6).
4.  In `test-originate` NI:
    *   Peers `192.0.2.10`, `2001:db8:10::1`, `192.0.2.20`, and `2001:db8:20::1` must have `state/session-state == ESTABLISHED`, `state/enable-aigp == true`, and `/bgp/neighbors/neighbor/state/messages/sent/UPDATE > 0`.

#### Canonical OC

```json
{
  "routing-policy": {
    "policy-definitions": {
      "policy-definition": [
        {
          "config": {
            "name": "export-next-hop-self"
          },
          "statements": {
            "statement": [
              {
                "config": {
                  "name": "test-statement"
                },
                "actions": {
                  "bgp-actions": {
                    "config": {
                      "set-next-hop": "SELF"
                    }
                  },
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                }
              }
            ]
          }
        }
      ]
    }
  },
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "DEFAULT"
        },
        "interfaces": {
          "interface": [
            {
              "config": {
                "id": "Loopback10.0",
                "interface": "Loopback10",
                "subinterface": 0
              }
            }
          ]
        },
        "protocols": {
          "protocol": [
            {
              "identifier": "BGP",
              "config": {
                "identifier": "BGP",
                "name": "DEFAULT"
              },
              "bgp": {
                "peer-groups": {
                  "peer-group": [
                    {
                      "peer-group-name": "downlink",
                      "config": {
                        "peer-group-name": "downlink",
                        "peer-as": 64497
                      },
                      "route-reflector": {
                        "config": {
                          "route-reflector-client": true,
                          "route-reflector-cluster-id": "192.0.2.1"
                        }
                      }
                    },
                    {
                      "peer-group-name": "downlink6",
                      "config": {
                        "peer-group-name": "downlink6",
                        "peer-as": 64497
                      },
                      "route-reflector": {
                        "config": {
                          "route-reflector-client": true,
                          "route-reflector-cluster-id": "192.0.2.1"
                        }
                      }
                    }
                  ]
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "config": {
                        "neighbor-address": "192.0.2.104",
                        "peer-group": "downlink"
                      },
                      "apply-policy": {
                        "config": {
                          "export-policy": [
                            "export-next-hop-self"
                          ]
                        }
                      },
                      "transport": {
                        "config": {
                          "local-address": "Loopback10"
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "2001:db8:40::2",
                        "peer-group": "downlink6"
                      },
                      "apply-policy": {
                        "config": {
                          "export-policy": [
                            "export-next-hop-self"
                          ]
                        }
                      },
                      "transport": {
                        "config": {
                          "local-address": "Loopback10"
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "192.0.2.103",
                        "peer-group": "downlink"
                      },
                      "transport": {
                        "config": {
                          "local-address": "Loopback10"
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "2001:db8:30::2",
                        "peer-group": "downlink6"
                      },
                      "transport": {
                        "config": {
                          "local-address": "Loopback10"
                        }
                      }
                    }
                  ]
                }
              }
            },
            {
              "identifier": "DIRECTLY_CONNECTED",
              "name": "DEFAULT",
              "config": {
                "identifier": "DIRECTLY_CONNECTED",
                "name": "DEFAULT"
              }
            },
            {
              "identifier": "ISIS",
              "config": {
                "identifier": "ISIS",
                "name": "DEFAULT"
              },
              "isis": {
                "global": {
                  "config": {
                    "level-capability": "LEVEL_2",
                    "net": [
                      "49.0001.1980.5110.0025.00"
                    ]
                  }
                },
                "interfaces": {
                  "interface": [
                    {
                      "config": {
                        "circuit-type": "POINT_TO_POINT",
                        "enabled": true,
                        "interface-id": "port-channel2.10"
                      },
                      "levels": {
                        "level": [
                          {
                            "config": {
                              "enabled": true,
                              "level-number": 2
                            },
                            "afi-safi": {
                              "af": [
                                {
                                  "config": {
                                    "afi-name": "openconfig-isis-types:IPV4",
                                    "safi-name": "openconfig-isis-types:UNICAST",
                                    "metric": 30
                                  }
                                },
                                {
                                  "config": {
                                    "afi-name": "openconfig-isis-types:IPV6",
                                    "safi-name": "openconfig-isis-types:UNICAST",
                                    "metric": 30
                                  }
                                }
                              ]
                            }
                          }
                        ]
                      }
                    },
                    {
                      "config": {
                        "circuit-type": "POINT_TO_POINT",
                        "enabled": true,
                        "interface-id": "port-channel2.30"
                      },
                      "levels": {
                        "level": [
                          {
                            "config": {
                              "enabled": true,
                              "level-number": 2
                            },
                            "afi-safi": {
                              "af": [
                                {
                                  "config": {
                                    "afi-name": "openconfig-isis-types:IPV4",
                                    "safi-name": "openconfig-isis-types:UNICAST",
                                    "metric": 30
                                  }
                                },
                                {
                                  "config": {
                                    "afi-name": "openconfig-isis-types:IPV6",
                                    "safi-name": "openconfig-isis-types:UNICAST",
                                    "metric": 30
                                  }
                                }
                              ]
                            }
                          }
                        ]
                      }
                    },
                    {
                      "config": {
                        "enabled": true,
                        "interface-id": "Loopback10"
                      }
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
          "name": "test-instance"
        },
        "interfaces": {
          "interface": [
            {
              "config": {
                "id": "Loopback20.0",
                "interface": "Loopback20",
                "subinterface": 0
              }
            }
          ]
        },
        "protocols": {
          "protocol": [
            {
              "identifier": "BGP",
              "config": {
                "identifier": "BGP",
                "name": "DEFAULT"
              },
              "bgp": {
                "peer-groups": {
                  "peer-group": [
                    {
                      "peer-group-name": "downlink",
                      "config": {
                        "peer-group-name": "downlink",
                        "peer-as": 64498
                      },
                      "route-reflector": {
                        "config": {
                          "route-reflector-client": true,
                          "route-reflector-cluster-id": "192.0.2.1"
                        }
                      }
                    },
                    {
                      "peer-group-name": "downlink6",
                      "config": {
                        "peer-group-name": "downlink6",
                        "peer-as": 64498
                      },
                      "route-reflector": {
                        "config": {
                          "route-reflector-client": true,
                          "route-reflector-cluster-id": "192.0.2.1"
                        }
                      }
                    }
                  ]
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "config": {
                        "neighbor-address": "192.0.2.105",
                        "peer-group": "downlink"
                      },
                      "apply-policy": {
                        "config": {
                          "export-policy": [
                            "export-next-hop-self"
                          ]
                        }
                      },
                      "transport": {
                        "config": {
                          "local-address": "Loopback20"
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "2001:db8:50::2",
                        "peer-group": "downlink6"
                      },
                      "apply-policy": {
                        "config": {
                          "export-policy": [
                            "export-next-hop-self"
                          ]
                        }
                      },
                      "transport": {
                        "config": {
                          "local-address": "Loopback20"
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "192.0.2.103",
                        "peer-group": "downlink"
                      },
                      "transport": {
                        "config": {
                          "local-address": "Loopback20"
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "2001:db8:30::2",
                        "peer-group": "downlink6"
                      },
                      "transport": {
                        "config": {
                          "local-address": "Loopback20"
                        }
                      }
                    }
                  ]
                }
              }
            },
            {
              "identifier": "DIRECTLY_CONNECTED",
              "name": "DEFAULT",
              "config": {
                "identifier": "DIRECTLY_CONNECTED",
                "name": "DEFAULT"
              }
            },
            {
              "identifier": "ISIS",
              "config": {
                "identifier": "ISIS",
                "name": "DEFAULT"
              },
              "isis": {
                "global": {
                  "config": {
                    "level-capability": "LEVEL_2",
                    "net": [
                      "49.0001.1980.5110.0029.00"
                    ]
                  }
                },
                "interfaces": {
                  "interface": [
                    {
                      "config": {
                        "circuit-type": "POINT_TO_POINT",
                        "enabled": true,
                        "interface-id": "port-channel2.20"
                      },
                      "levels": {
                        "level": [
                          {
                            "config": {
                              "enabled": true,
                              "level-number": 2
                            },
                            "afi-safi": {
                              "af": [
                                {
                                  "config": {
                                    "afi-name": "openconfig-isis-types:IPV4",
                                    "safi-name": "openconfig-isis-types:UNICAST",
                                    "metric": 30
                                  }
                                },
                                {
                                  "config": {
                                    "afi-name": "openconfig-isis-types:IPV6",
                                    "safi-name": "openconfig-isis-types:UNICAST",
                                    "metric": 30
                                  }
                                }
                              ]
                            }
                          }
                        ]
                      }
                    },
                    {
                      "config": {
                        "circuit-type": "POINT_TO_POINT",
                        "enabled": true,
                        "interface-id": "port-channel2.40"
                      },
                      "levels": {
                        "level": [
                          {
                            "config": {
                              "enabled": true,
                              "level-number": 2
                            },
                            "afi-safi": {
                              "af": [
                                {
                                  "config": {
                                    "afi-name": "openconfig-isis-types:IPV4",
                                    "safi-name": "openconfig-isis-types:UNICAST",
                                    "metric": 30
                                  }
                                },
                                {
                                  "config": {
                                    "afi-name": "openconfig-isis-types:IPV6",
                                    "safi-name": "openconfig-isis-types:UNICAST",
                                    "metric": 30
                                  }
                                }
                              ]
                            }
                          }
                        ]
                      }
                    },
                    {
                      "config": {
                        "enabled": true,
                        "interface-id": "Loopback20"
                      }
                    }
                  ]
                }
              }
            }
          ]
        }
      }
    ]
  },
  "interfaces": {
    "interface": [
      {
        "name": "Loopback10",
        "config": {
          "name": "Loopback10",
          "type": "iana-if-type:softwareLoopback",
          "enabled": true
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.10",
                        "prefix-length": 32
                      }
                    }
                  ]
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:10::1",
                        "prefix-length": 128
                      }
                    }
                  ]
                }
              }
            }
          ]
        }
      },
      {
        "name": "Loopback20",
        "config": {
          "name": "Loopback20",
          "type": "iana-if-type:softwareLoopback",
          "enabled": true
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.20",
                        "prefix-length": 32
                      }
                    }
                  ]
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:20::1",
                        "prefix-length": 128
                      }
                    }
                  ]
                }
              }
            }
          ]
        }
      }
    ]
  }
}
```

---

### RT-1.37.4 - Validate Plus-1 AIGP Increment When IGP Metric to Original Next-Hop is Zero

The following configuration steps build sequentially on test case RT-1.37.3 above. When IS-IS is removed from the links connecting DUT 1 to DUT 2's `test-originate` NI and iBGP sessions are established directly over the connected point-to-point subinterface addresses (`198.51.100.25/30` $\leftrightarrow$ `198.51.100.26/30` and `2001:db8::25/126` $\leftrightarrow$ `2001:db8::26/126` on `lag2.30`; `198.51.100.29/30` $\leftrightarrow$ `198.51.100.30/30` and `2001:db8::29/126` $\leftrightarrow$ `2001:db8::30/126` on `lag2.40`), the recursive next-hop resolves via a `DIRECTLY_CONNECTED` route whose IGP metric is `0`. Per the vendor AIGP implementation requirement, when DUT 1 reflects a route with `set-next-hop: SELF` and the IGP metric to the original next-hop is `0`, it increments the AIGP metric by `1` (`200 + 1 = 201`).

#### Effective Emulated Test Topology

```text
+--------------------------+ Lag2 (VLAN 30 & 40) [iBGP] +-------------+          Lag2 [iBGP]          +---------+
| DUT 2 (test-originate NI)|============================|  DUT 1 (RR) |===============================|  DUT 2  |
+--------------------------+                            +-------------+             IS-IS             +---------+
```

*   **Step 1 - Generate DUT configuration**
    *   **DUT 1 Configuration:**
        *   In `DEFAULT` NI:
            *   Delete `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=DEFAULT]/isis/interfaces/interface[interface-id=lag2.30]`.
            *   Delete BGP neighbors `192.0.2.103` and `2001:db8:30::2` under `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors`.
            *   Create iBGP neighbor `198.51.100.26` (`peer-group = downlink`, `IPV4_UNICAST` `enabled = true`, `enable-aigp = true`, `export-policy = [nhopself]`) and iBGP neighbor `2001:db8::26` (`peer-group = downlink6`, `IPV6_UNICAST` `enabled = true`, `enable-aigp = true`, `export-policy = [nhopself6]`) over `lag2.30`.
        *   In `test-instance` NI:
            *   Delete `/network-instances/network-instance[name=test-instance]/protocols/protocol[identifier=ISIS][name=DEFAULT]/isis/interfaces/interface[interface-id=lag2.40]`.
            *   Delete BGP neighbors `192.0.2.103` and `2001:db8:30::2` under `/network-instances/network-instance[name=test-instance]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors`.
            *   Create iBGP neighbor `198.51.100.30` (`peer-group = downlink`, `IPV4_UNICAST` `enabled = true`, `enable-aigp = true`, `export-policy = [nhopself]`) and iBGP neighbor `2001:db8::30` (`peer-group = downlink6`, `IPV6_UNICAST` `enabled = true`, `enable-aigp = true`, `export-policy = [nhopself6]`) over `lag2.40`.
    *   **DUT 2 Configuration:**
        *   In `test-originate` NI:
            *   Delete `/network-instances/network-instance[name=test-originate]/protocols/protocol[identifier=ISIS][name=DEFAULT]`.
            *   Delete BGP neighbors `192.0.2.10`, `2001:db8:10::1`, `192.0.2.20`, and `2001:db8:20::1` under `/network-instances/network-instance[name=test-originate]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors`.
            *   Create iBGP neighbors over the directly connected `lag2.30` and `lag2.40` subinterfaces:
                *   Neighbor `198.51.100.25`: `peer-group = default-peer-group`, `IPV4_UNICAST` `enabled = true`, `enable-aigp = true`, `export-policy = [loopback30-export]`.
                *   Neighbor `2001:db8::25`: `peer-group = default-peer-group6`, `IPV6_UNICAST` `enabled = true`, `enable-aigp = true`, `export-policy = [loopback30-export6]`.
                *   Neighbor `198.51.100.29`: `peer-group = test-instance-peer-group`, `IPV4_UNICAST` `enabled = true`, `enable-aigp = true`, `export-policy = [loopback30-export]`.
                *   Neighbor `2001:db8::29`: `peer-group = test-instance-peer-group6`, `IPV6_UNICAST` `enabled = true`, `enable-aigp = true`, `export-policy = [loopback30-export6]`.
*   **Step 2 - Push configuration to DUT using gnmi.Set**
    *   Apply the incremental configuration from Step 1 to DUT 1 and DUT 2 using `gNMI.Set` (`union_replace: true` / `delete` for removed IS-IS interfaces and loopback BGP neighbors).
*   **Step 3 - Send Traffic**
    *   Verify via `gNMI.Subscribe` (`ON_CHANGE` mode) that the new point-to-point iBGP neighbors (`198.51.100.26`, `2001:db8::26`, `198.51.100.30`, `2001:db8::30` on DUT 1; `198.51.100.25`, `2001:db8::25`, `198.51.100.29`, `2001:db8::29` on DUT 2) reach `ESTABLISHED` and `state/prefixes/installed` converges to `>= 1`.
*   **Step 4 - Validation with pass/fail criteria**
    *   **Pass Criteria on DUT 1:**
        *   IS-IS Level 2 adjacencies on `lag2.10` (`DEFAULT` NI) and `lag2.20` (`test-instance` NI) must remain `UP`.
        *   In `DEFAULT` NI, routes `192.0.2.103/32` (from `198.51.100.26`) and `2001:db8:30::2/128` (from `2001:db8::26`) must have `/bgp/rib/attr-sets/attr-set[index=<attr-index>]/state/aigp` equal to `200`.
        *   In `test-instance` NI, routes `192.0.2.103/32` (from `198.51.100.30`) and `2001:db8:30::2/128` (from `2001:db8::30`) must have `/bgp/rib/attr-sets/attr-set[index=<attr-index>]/state/aigp` equal to `200`.
    *   **Pass Criteria on DUT 2:**
        *   IS-IS Level 2 adjacencies on `lag2.10` (`DEFAULT` NI) and `lag2.20` (`test-instance` NI) must remain `UP`.
        *   In `DEFAULT` NI, reflected routes `192.0.2.103/32` (from `192.0.2.10`) and `2001:db8:30::2/128` (from `2001:db8:10::1`) must have `/bgp/rib/attr-sets/attr-set[index=<attr-index>]/state/aigp` equal to `201` (`200 + 1`).
        *   In `test-instance` NI, reflected routes `192.0.2.103/32` (from `192.0.2.20`) and `2001:db8:30::2/128` (from `2001:db8:20::1`) must have `/bgp/rib/attr-sets/attr-set[index=<attr-index>]/state/aigp` equal to `201` (`200 + 1`).

---

### RT-1.37.5 - Validate AIGP Attribute is Omitted When AIGP Propagation is Disabled on iBGP Peers

The following configuration steps build sequentially on test case RT-1.37.4 above. When `enable-aigp` is set to `false` on an outbound iBGP neighbor session, DUT 1 must strip/omit the AIGP attribute when advertising or reflecting routes to that neighbor (RFC 7311 §3.4).

*   **Step 1 - Generate DUT configuration**
    *   **DUT 1 Configuration:**
        *   In the `DEFAULT` network instance, set `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=192.0.2.104]/afi-safis/afi-safi[afi-safi-name=IPV4_UNICAST]/config/enable-aigp` to `false` and `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=2001:db8:40::2]/afi-safis/afi-safi[afi-safi-name=IPV6_UNICAST]/config/enable-aigp` to `false` (Client A). Also set `enable-aigp: false` on `192.0.2.106` (`IPV4_UNICAST`) and `2001:db8:41::2` (`IPV6_UNICAST`) (Client B).
        *   In the `test-instance` network instance, set `/network-instances/network-instance[name=test-instance]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=192.0.2.105]/afi-safis/afi-safi[afi-safi-name=IPV4_UNICAST]/config/enable-aigp` to `false` and `/network-instances/network-instance[name=test-instance]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=2001:db8:50::2]/afi-safis/afi-safi[afi-safi-name=IPV6_UNICAST]/config/enable-aigp` to `false` (Client A). Also set `enable-aigp: false` on `192.0.2.107` (`IPV4_UNICAST`) and `2001:db8:51::2` (`IPV6_UNICAST`) (Client B).
*   **Step 2 - Push configuration to DUT using gnmi.Set**
    *   Apply the configuration from Step 1 to DUT 1 using `gNMI.Set` (`union_replace: true`).
*   **Step 3 - Send Traffic**
    *   Use `gNMI.Subscribe` (`ON_CHANGE` mode) on DUT 1 and DUT 2 to verify BGP sessions remain `ESTABLISHED` and route updates converge on DUT 2.
*   **Step 4 - Validation with pass/fail criteria**
    *   **Pass Criteria on DUT 1:**
        *   In `DEFAULT` NI:
            *   Peers `192.0.2.104`, `2001:db8:40::2`, `192.0.2.106`, `2001:db8:41::2`, `198.51.100.26`, and `2001:db8::26` must be `ESTABLISHED`.
            *   `afi-safis/afi-safi/state/enable-aigp` must be `false` on `192.0.2.104`, `2001:db8:40::2`, `192.0.2.106`, and `2001:db8:41::2`, and `true` on `198.51.100.26` and `2001:db8::26`.
            *   Routes `192.0.2.103/32` (from `198.51.100.26`) and `2001:db8:30::2/128` (from `2001:db8::26`) must still have `/bgp/rib/attr-sets/attr-set[index=<attr-index>]/state/aigp` equal to `200`.
        *   In `test-instance` NI:
            *   Peers `192.0.2.105`, `2001:db8:50::2`, `192.0.2.107`, `2001:db8:51::2`, `198.51.100.30`, and `2001:db8::30` must be `ESTABLISHED`.
            *   `afi-safis/afi-safi/state/enable-aigp` must be `false` on `192.0.2.105`, `2001:db8:50::2`, `192.0.2.107`, and `2001:db8:51::2`, and `true` on `198.51.100.30` and `2001:db8::30`.
            *   Routes `192.0.2.103/32` (from `198.51.100.30`) and `2001:db8:30::2/128` (from `2001:db8::30`) must still have `/bgp/rib/attr-sets/attr-set[index=<attr-index>]/state/aigp` equal to `200`.
    *   **Pass Criteria on DUT 2:**
        *   In `DEFAULT` NI, reflected routes `192.0.2.103/32` (received from `192.0.2.10`) and `2001:db8:30::2/128` (received from `2001:db8:10::1`) must NOT have the AIGP attribute (`/bgp/rib/attr-sets/attr-set[index=<attr-index>]/state/aigp` is absent or `0`).
        *   In `test-instance` NI, reflected routes `192.0.2.103/32` (received from `192.0.2.20`) and `2001:db8:30::2/128` (received from `2001:db8:20::1`) must NOT have the AIGP attribute (`/bgp/rib/attr-sets/attr-set[index=<attr-index>]/state/aigp` is absent or `0`).

---

### RT-1.37.6 - Validate Per-Address-Family AIGP in Peer-Groups and Most-Specific Configuration Precedence

The following configuration steps build sequentially on test case RT-1.37.5 above and validate both most-specific configuration precedence (neighbor-level `enable-aigp: false` on Client A overriding peer-group `enable-aigp: true`) and concurrent peer-group AIGP inheritance on Client B (which has no neighbor-level `enable-aigp` override), followed by removing the neighbor-level override on Client A.

*   **Step 1 - Generate DUT configuration**
    *   **Phase 1 Configuration (Concurrent Precedence vs. Inheritance):**
        *   On DUT 1, in both `DEFAULT` and `test-instance` NIs:
            *   Set `/network-instances/network-instance/protocols/protocol[identifier=BGP][name=BGP]/bgp/peer-groups/peer-group[peer-group-name=downlink]/afi-safis/afi-safi[afi-safi-name=IPV4_UNICAST]/config/enable-aigp` to `true`.
            *   Set `/network-instances/network-instance/protocols/protocol[identifier=BGP][name=BGP]/bgp/peer-groups/peer-group[peer-group-name=downlink6]/afi-safis/afi-safi[afi-safi-name=IPV6_UNICAST]/config/enable-aigp` to `true`.
            *   Keep explicit neighbor-level `enable-aigp: false` on **Client A** (`192.0.2.104` and `2001:db8:40::2` in `DEFAULT` NI; `192.0.2.105` and `2001:db8:50::2` in `test-instance` NI).
            *   Delete the neighbor-level `enable-aigp` leaf on **Client B** (`192.0.2.106` and `2001:db8:41::2` in `DEFAULT` NI; `192.0.2.107` and `2001:db8:51::2` in `test-instance` NI) so Client B inherits `enable-aigp: true` from peer-groups `downlink` and `downlink6`.
    *   **Phase 2 Configuration (Full Peer-Group Inheritance on Client A):**
        *   On DUT 1, delete the neighbor-level `enable-aigp: false` override from **Client A** (`192.0.2.104` and `2001:db8:40::2` in `DEFAULT` NI; `192.0.2.105` and `2001:db8:50::2` in `test-instance` NI) so both Client A and Client B inherit `enable-aigp: true` from peer-groups `downlink` and `downlink6`.
*   **Step 2 - Push configuration to DUT using gnmi.Set**
    *   Apply Phase 1 configuration to DUT 1 using `gNMI.Set`, perform Phase 1 validations in Step 4, then apply Phase 2 configuration to DUT 1 using `gNMI.Set` and perform Phase 2 validations in Step 4.
*   **Step 3 - Send Traffic**
    *   Use `gNMI.Subscribe` (`ON_CHANGE` mode) after each `gNMI.Set` operation to verify BGP sessions remain `ESTABLISHED` and route updates converge.
*   **Step 4 - Validation with pass/fail criteria**
    *   **Pass Criteria (Phase 1 — Concurrent Precedence vs. Inheritance):**
        *   On DUT 1, in `DEFAULT` NI:
            *   Client A neighbors `192.0.2.104` and `2001:db8:40::2` (with explicit neighbor `enable-aigp: false`) must report effective `afi-safis/afi-safi/state/enable-aigp = false` (most-specific config wins over peer-group `enable-aigp: true`).
            *   Concurrently, Client B neighbors `192.0.2.106` and `2001:db8:41::2` (in the same peer-groups `downlink` / `downlink6` without a neighbor-level override) must report effective `afi-safis/afi-safi/state/enable-aigp = true`.
        *   On DUT 1, in `test-instance` NI:
            *   Client A neighbors `192.0.2.105` and `2001:db8:50::2` must report effective `afi-safis/afi-safi/state/enable-aigp = false`.
            *   Concurrently, Client B neighbors `192.0.2.107` and `2001:db8:51::2` must report effective `afi-safis/afi-safi/state/enable-aigp = true`.
        *   On DUT 2, routes received on Client A sessions (anchored on `Loopback40` / `Loopback50`) must omit the AIGP attribute (`state/aigp` absent or `0`), whereas routes received on Client B sessions (anchored on `Loopback41` / `Loopback51`) must carry `state/aigp = 201`.
    *   **Pass Criteria (Phase 2 — Full Peer-Group Inheritance):**
        *   On DUT 1:
            *   In `DEFAULT` NI, all peers (`192.0.2.104`, `2001:db8:40::2`, `192.0.2.106`, `2001:db8:41::2`, `198.51.100.26`, `2001:db8::26`) must be `ESTABLISHED` with `state/enable-aigp = true`, and routes from `198.51.100.26` and `2001:db8::26` must have `state/aigp = 200`.
            *   In `test-instance` NI, all peers (`192.0.2.105`, `2001:db8:50::2`, `192.0.2.107`, `2001:db8:51::2`, `198.51.100.30`, `2001:db8::30`) must be `ESTABLISHED` with `state/enable-aigp = true`, and routes from `198.51.100.30` and `2001:db8::30` must have `state/aigp = 200`.
        *   On DUT 2:
            *   In `DEFAULT` NI, reflected routes received on both Client A and Client B sessions from `192.0.2.10` and `2001:db8:10::1` must have `/bgp/rib/attr-sets/attr-set[index=<attr-index>]/state/aigp` equal to `201`.
            *   In `test-instance` NI, reflected routes received on both Client A and Client B sessions from `192.0.2.20` and `2001:db8:20::1` must have `/bgp/rib/attr-sets/attr-set[index=<attr-index>]/state/aigp` equal to `201`.

#### TODO: https://github.com/openconfig/public/pull/1451 - Add peer-group AIGP enable paths

```json
{
  "openconfig-network-instance:network-instances": {
    "network-instance": [
      {
        "name": "DEFAULT",
        "protocols": {
          "protocol": [
            {
              "identifier": "openconfig-policy-types:BGP",
              "name": "BGP",
              "bgp": {
                "peer-groups": {
                  "peer-group": [
                    {
                      "peer-group-name": "downlink",
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                              "enabled": true,
                              "enable-aigp": true
                            }
                          }
                        ]
                      }
                    },
                    {
                      "peer-group-name": "downlink6",
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                              "enabled": true,
                              "enable-aigp": true
                            }
                          }
                        ]
                      }
                    }
                  ]
                }
              }
            }
          ]
        }
      }
    ]
  }
}
```

---

### RT-1.37.7 - Validate Inbound AIGP Attribute is Discarded When Disabled by Default on eBGP Peers

The following negative test case builds sequentially on test case RT-1.37.6 above and verifies RFC 7311 §3.4 security/isolation behavior on eBGP sessions: by default (`enable-aigp: false` or unconfigured on an eBGP neighbor AFI-SAFI), if an external BGP peer (ATE AS 65537 / AS 65538 on `lag1`) advertises routes containing an AIGP attribute (`aigp = 10` on `lag1.10`/`lag1.30`, `aigp = 20` on `lag1.20`/`lag1.40`), DUT 1 must discard/ignore the inbound AIGP attribute upon reception unless `enable-aigp: true` is explicitly configured on that eBGP neighbor's AFI-SAFI.

*   **Step 1 - Generate DUT configuration**
    *   **DUT 1 Configuration:**
        *   In the `DEFAULT` network instance, set `enable-aigp: false` (or delete the `enable-aigp` leaf so it reverts to its default disabled state) on eBGP neighbors `198.51.100.2`, `2001:db8::2` (`lag1.10`, AS 65537) and `198.51.100.6`, `2001:db8::6` (`lag1.20`, AS 65538). Also remove the `aspath-prepend` and `aspath-prepend6` import policies from `198.51.100.2` and `2001:db8::2` so both eBGP paths have equal AS-PATH length (`1`).
        *   In the `test-instance` network instance, set `enable-aigp: false` (or delete the `enable-aigp` leaf) on eBGP neighbors `198.51.100.10`, `2001:db8::10` (`lag1.30`, AS 65537) and `198.51.100.14`, `2001:db8::14` (`lag1.40`, AS 65538), and remove the `aspath-prepend` and `aspath-prepend6` import policies from `198.51.100.10` and `2001:db8::10`.
    *   **ATE Configuration:**
        *   Keep advertising the 1,000 IPv4 and 1,000 IPv6 prefixes from ATE on `lag1.10`/`lag1.30` with `aigp = 10` and on `lag1.20`/`lag1.40` with `aigp = 20`.
*   **Step 2 - Push configuration to DUT using gnmi.Set**
    *   Apply the configuration from Step 1 to DUT 1 using `gNMI.Set`.
*   **Step 3 - Send Traffic**
    *   Wait via `gNMI.Subscribe` (`ON_CHANGE` mode) for eBGP sessions on `lag1` to re-converge (`state/session-state == ESTABLISHED` and `state/prefixes/installed == 1000`).
*   **Step 4 - Validation with pass/fail criteria**
    *   **Pass Criteria on DUT 1:**
        *   All eBGP sessions on `lag1.10`–`lag1.40` must remain `ESTABLISHED` (`0` BGP session resets/notifications caused by the inbound AIGP attribute).
        *   All 1,000 IPv4 and 1,000 IPv6 prefixes received from ATE on `lag1.10`–`lag1.40` must be accepted as valid routes (`state/valid-route == true`), but their associated attribute sets (`/bgp/rib/attr-sets/attr-set[index=<attr-index>]/state/aigp`) must NOT contain the AIGP attribute (`state/aigp` is absent or `0`), proving the inbound AIGP attribute from the untrusted eBGP peers was discarded at ingress.
        *   When DUT 1 reflects these eBGP routes to DUT 2 over `lag2`, the reflected routes on DUT 2 must also omit the AIGP attribute (`state/aigp` is absent or `0`), because no AIGP attribute exists on the route in DUT 1's Loc-RIB.

---

### RT-1.37.8 - Validate Route Withdrawal When Original Next-Hop Becomes Unresolvable in IGP During Route Reflection

The following negative test case builds sequentially on test case RT-1.37.7 above and verifies RFC 7311 §3.2 failure handling when the recursive next-hop of an AIGP-enabled route becomes unresolvable. First, the IS-IS topology from `RT-1.37.3` is restored so DUT 1 receives `192.0.2.103/32` and `2001:db8:30::2/128` from `test-originate` NI with `NEXT_HOP = 192.0.2.103` / `2001:db8:30::2` resolved via IS-IS (`aigp = 200`, reflected to DUT 2 with `aigp = 210`). Then, IS-IS is disabled on `Loopback30` in `test-originate` NI while keeping the `lag2.30` and `lag2.40` interfaces up, rendering `192.0.2.103/32` and `2001:db8:30::2/128` unresolvable in DUT 1's IGP RIB.

*   **Step 1 - Generate DUT configuration**
    *   **Sub-step 1a (Restore RT-1.37.3 IS-IS Next-Hop Resolution Baseline):**
        *   Restore the IS-IS configuration on `lag2.30`, `lag2.40`, and `Loopback30.0` (`metric = 10` on `lag2.30`/`lag2.40`) and restore the loopback-peered iBGP sessions between DUT 1 (`192.0.2.10`/`2001:db8:10::1`, `192.0.2.20`/`2001:db8:20::1`) and DUT 2 `test-originate` NI (`192.0.2.103`/`2001:db8:30::2`) as defined in `RT-1.37.3`, while adding a static route on DUT 1 to `192.0.2.103/32` and `2001:db8:30::2/128` only after verifying initial IS-IS convergence, OR keeping the point-to-point iBGP session from `RT-1.37.4` (`198.51.100.26`/`2001:db8::26` and `198.51.100.30`/`2001:db8::30`) and setting an import policy on DUT 1 that sets `NEXT_HOP` to `192.0.2.103` / `2001:db8:30::2` (resolved via IS-IS on `lag2.30`/`lag2.40`). Specifically, keep the point-to-point iBGP sessions from `RT-1.37.4` (`198.51.100.26`/`2001:db8::26` on `lag2.30` and `198.51.100.30`/`2001:db8::30` on `lag2.40`) so the BGP TCP transport stays `ESTABLISHED` over the directly connected `/30` and `/126` subnets, re-enable IS-IS on `lag2.30`, `lag2.40`, and `Loopback30.0` (`metric = 10`), and configure `local-address = 192.0.2.103` / `2001:db8:30::2` on `test-originate`'s export next-hop so the advertised BGP `NEXT_HOP` is `192.0.2.103` / `2001:db8:30::2` (resolved recursively via IS-IS on DUT 1 with IGP metric `10`, producing reflected `state/aigp = 210` on DUT 2).
    *   **Sub-step 1b (Trigger Unresolvable Next-Hop):**
        *   On DUT 2 in `test-originate` NI, disable IS-IS on `Loopback30.0` (`/network-instances/network-instance[name=test-originate]/protocols/protocol[identifier=ISIS][name=DEFAULT]/isis/interfaces/interface[interface-id=Loopback30.0]/config/enabled = false`) so `192.0.2.103/32` and `2001:db8:30::2/128` are withdrawn from IS-IS while the point-to-point iBGP sessions on `lag2.30` and `lag2.40` remain `ESTABLISHED`.
*   **Step 2 - Push configuration to DUT using gnmi.Set**
    *   Apply Sub-step 1a via `gNMI.Set`, verify via `gNMI.Subscribe` that DUT 2 receives reflected routes with `state/aigp = 210`, and then apply Sub-step 1b via `gNMI.Set`.
*   **Step 3 - Send Traffic**
    *   Monitor BGP RIB state on DUT 1 and DUT 2 via `gNMI.Subscribe` (`ON_CHANGE` mode) upon withdrawal of `192.0.2.103/32` and `2001:db8:30::2/128` from IS-IS.
*   **Step 4 - Validation with pass/fail criteria**
    *   **Pass Criteria on DUT 1:**
        *   When `192.0.2.103/32` and `2001:db8:30::2/128` become unresolvable in DUT 1's IGP (`DEFAULT` and `test-instance` NIs), DUT 1 must mark the BGP routes received with `NEXT_HOP = 192.0.2.103` / `2001:db8:30::2` as invalid/ ineligible (`state/valid-route == false` or `state/best-path == false`) in `adj-rib-in-post` and must withdraw the reflected routes from DUT 2 rather than advertising them with a stale `210` or erroneously falling back to the `+1` (`201`) rule.
    *   **Pass Criteria on DUT 2:**
        *   In `DEFAULT` NI and `test-instance` NI, the reflected routes previously received from `192.0.2.10`/`2001:db8:10::1` and `192.0.2.20`/`2001:db8:20::1` must be withdrawn (`state/prefixes/installed == 0` for the unresolvable prefix).

---

### Cleanup

*   Stop all OTG protocols and traffic flows on the ATE.
*   Remove BGP and IS-IS protocol instances, routing policies, prefix-sets, loopback interfaces (`Loopback10`–`Loopback51`), LAG subinterfaces (`lag1.10`–`lag1.40`, `lag2.10`–`lag2.40`), and non-default network instances (`test-instance` and `test-originate`) from DUT 1 and DUT 2 to restore the devices to their baseline state.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  # Configuration coverage
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/vlan/match/single-tagged/config/vlan-id:
  /network-instances/network-instance/tables/table/config/protocol:
  /network-instances/network-instance/tables/table/config/address-family:
  /routing-policy/defined-sets/prefix-sets/prefix-set/config/name:
  /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/repeat-n:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/asn:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range:
  /routing-policy/policy-definitions/policy-definition/statements/statement/config/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-next-hop:
  /network-instances/network-instance/protocols/protocol/config/identifier:
  /network-instances/network-instance/protocols/protocol/config/name:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/as:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/afi-safi-name:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-group-name:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-as:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/config/afi-safi-name:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/route-reflector/config/route-reflector-cluster-id:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/route-reflector/config/route-reflector-client:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/neighbor-address:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-group:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/config/afi-safi-name:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/export-policy:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/config/local-address:
  /network-instances/network-instance/protocols/protocol/isis/global/config/net:
  /network-instances/network-instance/protocols/protocol/isis/global/config/level-capability:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/config/metric-style:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/config/interface-id:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/config/circuit-type:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/afi-safi/af/config/metric:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/afi-safi/af/config/afi-name:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/afi-safi/af/config/safi-name:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/config/level-number:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/config/enabled:
  /network-instances/network-instance/table-connections/table-connection/config/address-family:
  /network-instances/network-instance/table-connections/table-connection/config/src-protocol:
  /network-instances/network-instance/table-connections/table-connection/config/dst-protocol:
  /network-instances/network-instance/table-connections/table-connection/config/default-import-policy:
  # TODO: Create path for enabling AIGP on neighbor and peer-group basis and also bgp action for AIGP. See [PR/1451](https://github.com/openconfig/public/pull/1451)
  # TODO: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/config/enable-aigp
  # TODO: /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-aigp
  # TODO: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/config/enable-aigp

  # Telemetry coverage
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/adjacency-state:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/best-path:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/attr-index:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/valid-route:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/best-path:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/attr-index:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/valid-route:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/installed:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/messages/sent/UPDATE:
  /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/aigp:
  /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/next-hop:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address:
  /interfaces/interface/state/counters/out-pkts:
  /interfaces/interface/subinterfaces/subinterface/state/counters/out-unicast-pkts:
  /interfaces/interface/state/admin-status:
  /interfaces/interface/state/oper-status:
  # TODO: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/enable-aigp
  # TODO: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/state/enable-aigp

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

*   FFF

