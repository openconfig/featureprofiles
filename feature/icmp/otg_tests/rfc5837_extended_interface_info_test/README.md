# RT-9.1: ICMP Extended Interface and Next-Hop Identification (draft-mitchell-intarea-rfc5837bis)

## Summary

Determine that the DUT generates standards-compliant ICMPv4 and ICMPv6
multi-part error messages ([RFC 4884](https://www.rfc-editor.org/rfc/rfc4884))
carrying both the [RFC 5837](https://www.rfc-editor.org/rfc/rfc5837) **Interface
Information Object** (`Class-Num = 2`) and the
[draft-mitchell-intarea-rfc5837bis-02](https://datatracker.ietf.org/doc/draft-mitchell-intarea-rfc5837bis/02/)
**Extended Interface Information Object** (`Class-Num = 7`) for traceroute and
diagnostic probes from explicitly allowed IPv4/IPv6 source hosts and subnets,
supports configurable outer encapsulation decapsulation (initially outer IPv4
IP-in-IP for inner IPv4 and IPv6 datagrams via `IPV4`, extensible via
`openconfig-aft-types:encapsulation-header-type` to `IPV6`, `GRE`, and `MPLS`),
validates source-prefix eligibility against the target IP of the generated ICMP
response (inner source IP when decapsulated, outer source IP otherwise), and
enforces a configurable RFC 4884 original datagram size cap.

### IANA Object Class Allocation (`Class-Num = 7`)

Per Sections 5 and 10 of `draft-mitchell-intarea-rfc5837bis-02`, extending
`Class-Num = 2` in place is not backward-compatible because legacy RFC 5837
receivers strictly validate reserved bits `2-3` in the `Class-Num = 2` `C-Type`
bitmask. Allocating a separate ICMP Extension Object Class under First Come
First Served
([RFC 4884 Section 10](https://www.rfc-editor.org/rfc/rfc4884#section-10)) in
the IANA
[ICMP Extension Object Classes and Class Sub-types](https://www.iana.org/assignments/icmp-parameters#icmp-parameters-ext-classes)
registry allows upgraded routers and traceroute clients to emit and decode new
interface roles alongside `Class-Num = 2`, while legacy receivers safely ignore
the unknown object class.

*   **Assigned Object Class**: `Class-Num = 7` (**Extended Interface Information
    Object**,
    [draft-mitchell-intarea-rfc5837bis-02, Section 5](https://www.iana.org/go/draft-mitchell-intarea-rfc5837bis-02#section-5)).
*   **`C-Type` Bit-Fields and Extended Interface Roles**
    (`draft-mitchell-intarea-rfc5837bis-02`, Sections 5 and 10):
    *   "Sub-types — Class 7 — Extended Interface Information Object"
        (`Bits 0-3`: Extended Interface Role; `Bit 4`: `ifIndex` included;
        `Bit 5`: `IP Address Sub-Object` included; `Bit 6`: `Name Sub-Object`
        included; `Bit 7`: `MTU` included).
    *   "Sub-types — Class 7 — Extended Interface Information Object — Extended
        Interface Roles" (`Value 0`: `Outgoing Sub-IP Component`; `Values 1-15`:
        Unassigned).

--------------------------------------------------------------------------------

## Expected Traceroute Response Wire Format and Populated Fields

### 1. Packet Envelope Layout

```text
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|          Outer IPv4 (Proto=1) or IPv6 (NextHdr=58) Header     |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|     Type      |     Code      |          ICMP Checksum        |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|  ICMPv4: unused (0) | Length  |          unused (0)           |
|  ICMPv6: Length     |              Reserved (0)               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|      Original Datagram (eliciting or decapsulated inner IP    |
|      header + payload, zero-padded to >= 128 octets and       |
|      aligned to 4-octet [ICMPv4] or 8-octet [ICMPv6] words,   |
|      truncated to <= max-original-datagram-size)              |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|Ver=2  |      Reserved (0)     |      Extension Checksum       |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|         Object Length         |  Class-Num=2  |    C-Type     |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|  Class 2 Sub-Objects (ordered: ifIndex, IPAddr, Name, MTU)    |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                              ...                              |
|  (Up to 4 Class-Num=2 objects, at most one per unique Role)   |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|         Object Length         |  Class-Num=7  |    C-Type     |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|   Class 7 Sub-Objects (ordered: ifIndex, IPAddr, Name, MTU)   |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

### 2. Outer IP Header, ICMP Header, and RFC 4884 Extension Header Fields

<!-- disableFinding(LINE_OVER_80) -->

<!-- mdformat off(reason: standard GFM table rendering on GitHub, Critique, and Code Search) -->

| Layer / Header | Field | Expected Value (`ICMPv4`) | Expected Value (`ICMPv6`) |
| :--- | :--- | :--- | :--- |
| **Outer IP Header** | Version / Protocol | IPv4 (`4`), `Protocol = 1` (`ICMP`) | IPv6 (`6`), `Next Header = 58` (`ICMPv6`) |
| **Outer IP Header** | Source IP | Selected per RFC 1812 Section 4.3.2 (numbered IPv4 address of egress/ingress interface, or `Loopback0` fallback if unnumbered) | Selected per RFC 4443 Section 2.2 (unicast IPv6 address of egress/ingress interface, or `Loopback0` fallback if unnumbered) |
| **Outer IP Header** | Destination IP | Source IPv4 of eliciting probe (or **inner** IPv4 Source IP when outer encapsulation is decapsulated) | Source IPv6 of eliciting probe (or **inner** IPv6 Source IP when outer encapsulation is decapsulated) |
| **Outer IP Header** | Total Packet Size | Bounded by `576` octets (RFC 1812) | Bounded by `1280` octets (RFC 4443 IPv6 minimum link MTU) |
| **Outer IP Header** | TTL / Hop Limit & DSCP | Control-plane initial TTL (e.g., `64`); DSCP preserved from eliciting/inner packet or set per DUT control-plane policy | Control-plane initial Hop Limit (e.g., `64`); Traffic Class DSCP preserved from eliciting/inner packet or set per policy |
| **ICMP Header** | Transit Hop Expiry (`TTL/HopLimit == 1`) | `Type = 11` (Time Exceeded), `Code = 0` (TTL exceeded in transit) | `Type = 3` (Time Exceeded), `Code = 0` (Hop limit exceeded in transit) |
| **ICMP Header** | Local UDP Port Closed | `Type = 3` (Destination Unreachable), `Code = 3` (Port Unreachable) | `Type = 1` (Destination Unreachable), `Code = 4` (Port Unreachable) |
| **ICMP Header** | RFC 4884 `Length` | **Octet 5** of ICMPv4 header (octets 4, 6, 7 = `0x00`): length of padded Original Datagram in **32-bit (4-octet) words** (minimum `32`, i.e., `128` octets) | **Octet 4** of ICMPv6 header (octets 5, 6, 7 = `0x00`): length of padded Original Datagram in **64-bit (8-octet) words** (minimum `16`, i.e., `128` octets) |
| **ICMP Header** | `Checksum` | Valid 16-bit one's complement checksum over entire ICMPv4 message | Valid 16-bit checksum over IPv6 pseudo-header + entire ICMPv6 message |
| **Original Datagram** | Payload & Padding | Leading octets of eliciting (or decapsulated inner) IPv4 packet; zero-padded to at least `128` octets and 4-octet boundary; capped at `max-original-datagram-size` | Leading octets of eliciting (or decapsulated inner) IPv6 packet; zero-padded to at least `128` octets and 8-octet boundary; capped at `max-original-datagram-size` |
| **RFC 4884 Ext Header** | `Version`, `Reserved`, `Checksum` | `Version = 2` (bits `0-3`), `Reserved = 0x000` (bits `4-15`), valid 16-bit one's complement checksum over the Extension Structure only | Identical to `ICMPv4` |

<!-- mdformat on -->

<!-- enableFinding(LINE_OVER_80) -->

### 3. Object Classes and `C-Type` Bit-Field Encoding (MSB Bit Numbering)

Both object classes use **network MSB bit numbering** (`Bit 0 = 0x80`,
`Bit 7 = 0x01`):

<!-- disableFinding(LINE_OVER_80) -->

```text
Class-Num = 2 (Interface Information Object, RFC 5837 / draft-mitchell Section 4):
Bit     0       1       2       3       4       5       6       7
    +-------+-------+-------+-------+-------+-------+-------+-------+
    | Interface Role| Rsvd1 | Rsvd2 |ifIndex| IPAddr|  name |  MTU  |
    +-------+-------+-------+-------+-------+-------+-------+-------+
      0 = Incoming IP Interface       0x08    0x04    0x02    0x01
      1 = Incoming Sub-IP Component   (Rsvd1 = 0, Rsvd2 = 0)
      2 = Outgoing IP Interface
      3 = IP Next Hop

Class-Num = 7 (Extended Interface Information Object, draft-mitchell Section 5):
Bit     0       1       2       3       4       5       6       7
    +-------+-------+-------+-------+-------+-------+-------+-------+
    |    Extended Interface Role    |ifIndex| IPAddr|  name |  MTU  |
    +-------+-------+-------+-------+-------+-------+-------+-------+
      0 = Outgoing Sub-IP Component   0x08    0x04    0x02    0x01
      1..15 = Unassigned
```

<!-- enableFinding(LINE_OVER_80) -->

### 4. Sub-Object Ordering and Field Semantics

Both `Class-Num = 2` and `Class-Num = 7` share identical sub-object formats and
**mandatory wire ordering**. Whenever the corresponding `C-Type` bit is `1`,
sub-objects MUST be serialized in exact order (`ifIndex`, then `IP Address`,
then `Name`, then `MTU`) with at most one instance of each sub-object per role:

<!-- disableFinding(LINE_OVER_80) -->

<!-- mdformat off(reason: standard GFM table rendering on GitHub, Critique, and Code Search) -->

| Order | Sub-Object | `C-Type` Bit | Wire Length | Encoding & Expected DUT Telemetry Match |
| :--- | :--- | :--- | :--- | :--- |
| **1** | `ifIndex` | Bit 4 (`0x08`) | 4 octets | Big-endian 32-bit unsigned integer matching `/interfaces/interface/state/ifindex` of the identified L3 interface or physical LAG member port. |
| **2** | `IP Address Sub-Object` | Bit 5 (`0x04`) | 8 octets (IPv4) or 20 octets (IPv6) | 16-bit big-endian `AFI` (`0x0001` for IPv4, `0x0002` for IPv6) + 16-bit `Reserved` (`0x0000`) + 32-bit IPv4 or 128-bit IPv6 address. Per `draft-mitchell` Section 6.1, an `ICMPv4` message MUST only carry `AFI = 1` (IPv4) and an `ICMPv6` message MUST only carry `AFI = 2` (IPv6). Matches `/interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/state/ip` (or `ipv6/.../ip`), or the resolved FIB next-hop IP for Class 2 Role 3. |
| **3** | `Name Sub-Object` | Bit 6 (`0x02`) | 4 to 64 octets (multiple of 4) | 1-octet `Length` (total sub-object length in octets including the `Length` octet and `NULL` padding; max `64`) + UTF-8 interface `ifName` (up to 63 octets, matching `/interfaces/interface/state/name`, e.g., `Port-Channel1` or `Ethernet10/33/1`, **never** the DUT hostname) padded with `0x00` (`NULL`) bytes to a 4-octet boundary. |
| **4** | `MTU` | Bit 7 (`0x01`) | 4 octets | Big-endian 32-bit unsigned integer matching `/interfaces/interface/state/mtu` (in octets) of the identified L3 interface or physical LAG member port. |

<!-- mdformat on -->

<!-- enableFinding(LINE_OVER_80) -->

### 5. Default Role and Sub-Object Population Matrix

When `enabled = true`, the target IP of the generated ICMP response matches
`allowed-source-prefixes`, and all roles and sub-objects are included, the DUT
MUST populate the following objects and sub-objects for each packet scenario:

<!-- disableFinding(LINE_OVER_80) -->

<!-- mdformat off(reason: standard GFM table rendering on GitHub, Critique, and Code Search) -->

| Object Class & Role | `C-Type` (Hex) | Populated Sub-Objects (Default) | Transit: LAG Ingress to LAG Egress (`4` or `6`, plus `4in4` Decap) | Transit: Singleton Ingress to Singleton Egress | Local to DUT (`Port Unreachable`) | Cross-AF `6in4` Decap (IPv6 Inner, IPv4 Outer FIB Next-Hop) |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **Class `2`, Role `0`** (`Incoming IP Interface`) | `0x0f` (numbered in response AF) or `0x0b` (unnumbered in response AF) | `ifIndex`, `IPAddr` (if numbered in response AF), `Name` (L3 `ifName`, e.g., `agg1`), `MTU` | **Included** (`0x0f`) | **Included** (`0x0f`) | **Included** (`0x0f`) | **Included** (`0x0f` with `AFI=2` IPv6 addr if `agg1` has IPv6 configured; `0x0b` omitting `IPAddr` if IPv4-only) |
| **Class `2`, Role `1`** (`Incoming Sub-IP Component`) | `0x4b` | `ifIndex`, `Name` (physical ingress LAG member `ifName`, e.g., `port1`), `MTU` | **Included** (`0x4b`) | **Omitted** (not an aggregate) | **Included** (`0x4b` if received on LAG; omitted on singleton) | **Included** (`0x4b`) |
| **Class `2`, Role `2`** (`Outgoing IP Interface`) | `0x8f` (numbered in response AF) or `0x8b` (unnumbered in response AF) | `ifIndex`, `IPAddr` (if numbered in response AF), `Name` (L3 `ifName`, e.g., `agg2`), `MTU` | **Included** (`0x8f`) | **Included** (`0x8f`) | **Omitted** (not forwarded) | **Included** (`0x8f` with `AFI=2` IPv6 addr if `agg2` has IPv6 configured; `0x8b` omitting `IPAddr` if IPv4-only) |
| **Class `2`, Role `3`** (`IP Next Hop`) | `0xc4` | `IPAddr` (resolved FIB next-hop IP in response AF) | **Included** (`0xc4`) | **Included** (`0xc4`) | **Omitted** (not forwarded) | **Omitted** (outer FIB next-hop is IPv4 `AFI=1`, which is illegal in an `ICMPv6` response per `draft-mitchell` Section 6.1) |
| **Class `7`, Extended Role `0`** (`Outgoing Sub-IP Component`) | `0x0b` | `ifIndex`, `Name` (physical egress LAG member `ifName` selected by hardware hash, e.g., `port3`), `MTU` | **Included** (`0x0b` when forwarding plane reliably determines egress LAG member per `draft-mitchell` Section 8.2) | **Omitted** (not an aggregate) | **Omitted** (not forwarded) | **Included** (`0x0b` when egress is LAG and member is determined by forwarding plane) |

<!-- mdformat on -->

<!-- enableFinding(LINE_OVER_80) -->

### 6. Outer Encapsulation Decapsulation Examples (`IPV4` IP-in-IP and Future Protocol Knob)

In networks using tunnel encapsulation (such as IP-in-IP transit engineering),
traceroute probes from endpoints are encapsulated inside an outer IPv4 tunnel
header. When the **outer** IPv4 header expires in transit (`Outer TTL == 1`,
`Inner TTL / Hop Limit > 1`) on a transit router with outer-header decapsulation
enabled for `IPV4`:

1.  The DUT makes its forwarding decision (outgoing L3 interface, outgoing LAG
    member hash, and FIB next-hop) on the arriving packet, then strips the
    20-octet outer IPv4 header (`Protocol = 4` `IPPROTO_IPIP` for inner IPv4, or
    `Protocol = 41` `IPPROTO_IPV6` for inner IPv6).
2.  The DUT constructs an `ICMPv4` (if inner packet is IPv4) or `ICMPv6` (if
    inner packet is IPv6) Time Exceeded (`Code = 0`) message addressed directly
    to the **inner** packet's Source IP.
3.  The RFC 4884 **Original Datagram** field quotes the **inner** packet
    (starting at the inner IPv4 or IPv6 header, NOT the stripped outer IPv4
    header), padded and capped per `max-original-datagram-size`.

*   **Future Decapsulation Protocol Extensibility**: Outer header decapsulation
    MUST be controlled by an explicit configuration knob (modeled as a
    `decapsulate-headers` `leaf-list` of
    `openconfig-aft-types:encapsulation-header-type`, initially supporting
    `IPV4` to match outer IPv4 tunnel headers with `Protocol = 4` `IPPROTO_IPIP`
    or `Protocol = 41` `IPPROTO_IPV6`) so operators can enable or disable
    decapsulation and extend the exact same behavior to additional
    encapsulations including outer IPv6 IP-in-IP (`IPV6`, `Next Header = 4` or
    `41`), **IPoGRE** (`GRE`, `Protocol = 47`), and **IPoMPLS** (`MPLS`). Note
    that `IPV4` and `IPV6` apply strictly to outer IP-in-IP tunnel headers and
    MUST NOT strip native non-tunneled IPv4 or IPv6 packets. When the knob is
    disabled (empty) or a packet arrives with an encapsulation not enabled in
    the knob (e.g., `GRE` when only `IPV4` is enabled), the DUT MUST NOT strip
    the outer header and MUST generate an `ICMPv4` Time Exceeded response to the
    **outer** IPv4 Source IP quoting the **outer** IPv4 packet.

#### Example A: IPv4-in-IPv4 (`4in4`, `Protocol = 4`) Transit Probe with `IPV4` Decap Enabled

<!-- disableFinding(LINE_OVER_80) -->

```text
Arriving Probe on DUT agg1 (physical member port1):
  [Eth] -> [Outer IPv4: Src=198.18.0.1, Dst=198.51.100.2, Proto=4, TTL=1]
        -> [Inner IPv4: Src=198.18.0.10, Dst=203.0.113.10, Proto=17, TTL=64]
        -> [UDP: SrcPort=33434, DstPort=33435, Payload (e.g. 60B)]
  (Forwarding lookup on 198.51.100.2 -> Egress L3=agg2, Egress member=port3, FIB NH=192.0.2.6)

Generated ICMPv4 Time Exceeded Response from DUT:
  [Outer IPv4]   Src=192.0.2.1 (or egress IP 192.0.2.5), Dst=198.18.0.10 (Inner Src!), Proto=1
  [ICMPv4 Hdr]   Type=11, Code=0, Checksum=valid, Length=32 (128 octets / 4)
  [Orig Dgram]   Inner IPv4 [Src=198.18.0.10, Dst=203.0.113.10, TTL=64] + UDP + zero-pad to 128B
  [Ext Hdr]      Ver=2, Reserved=0, Checksum=valid
  [Obj 1]        Class-Num=2, C-Type=0x0f (Role 0 Incoming IP):     ifIndex(agg1), 192.0.2.1, "agg1", MTU(agg1)
  [Obj 2]        Class-Num=2, C-Type=0x4b (Role 1 Incoming Sub-IP): ifIndex(port1), "port1", MTU(port1)
  [Obj 3]        Class-Num=2, C-Type=0x8f (Role 2 Outgoing IP):     ifIndex(agg2), 192.0.2.5, "agg2", MTU(agg2)
  [Obj 4]        Class-Num=2, C-Type=0xc4 (Role 3 IP Next Hop):     AFI=1, 192.0.2.6
  [Obj 5]        Class-Num=7, C-Type=0x0b (ExtRole 0 Out Sub-IP):   ifIndex(port3), "port3", MTU(port3)
```

<!-- enableFinding(LINE_OVER_80) -->

#### Example B: IPv6-in-IPv4 (`6in4`, `Protocol = 41`) Transit Probe with `IPV4` Decap Enabled

<!-- disableFinding(LINE_OVER_80) -->

```text
Arriving Probe on DUT agg1 (physical member port2):
  [Eth] -> [Outer IPv4: Src=198.18.0.1, Dst=198.51.100.2, Proto=41, TTL=1]
        -> [Inner IPv6: Src=2001:db8:18::10, Dst=2001:db8:203::10, NextHdr=17, HopLimit=64]
        -> [UDP: SrcPort=33434, DstPort=33435, Payload (e.g. 60B)]
  (Forwarding lookup on 198.51.100.2 -> Egress L3=agg2, Egress member=port4, IPv4 FIB NH=192.0.2.6)

Generated ICMPv6 Time Exceeded Response from DUT:
  [Outer IPv6]   Src=2001:db8:1::1 (or 2001:db8:2::1), Dst=2001:db8:18::10 (Inner Src!), NextHdr=58
  [ICMPv6 Hdr]   Type=3, Code=0, Checksum=valid, Length=16 (128 octets / 8)
  [Orig Dgram]   Inner IPv6 [Src=2001:db8:18::10, Dst=2001:db8:203::10, HopLimit=64] + UDP + zero-pad to 128B
  [Ext Hdr]      Ver=2, Reserved=0, Checksum=valid
  [Obj 1]        Class-Num=2, C-Type=0x0f (Role 0 Incoming IP):     ifIndex(agg1), 2001:db8:1::1 (AFI=2), "agg1", MTU(agg1)
  [Obj 2]        Class-Num=2, C-Type=0x4b (Role 1 Incoming Sub-IP): ifIndex(port2), "port2", MTU(port2)
  [Obj 3]        Class-Num=2, C-Type=0x8f (Role 2 Outgoing IP):     ifIndex(agg2), 2001:db8:2::1 (AFI=2), "agg2", MTU(agg2)
  [Obj 4]        (Class-Num=2 Role 3 IP Next Hop OMITTED: outer FIB next-hop 192.0.2.6 is IPv4 AFI=1,
                  which MUST NOT appear in an ICMPv6 message per draft-mitchell Section 6.1; if agg1/agg2
                  lack IPv6 addresses, Obj 1/Obj 3 omit Bit 5 IPAddr and use C-Type=0x0b / 0x8b)
  [Obj 5]        Class-Num=7, C-Type=0x0b (ExtRole 0 Out Sub-IP):   ifIndex(port4), "port4", MTU(port4)
```

<!-- enableFinding(LINE_OVER_80) -->

### 7. Allowed Source Host and Subnet Gating (`allowed-source-prefixes`)

Per RFC 5837 Section 7 and `draft-mitchell-intarea-rfc5837bis` Section 9
(Security Considerations), routers MUST restrict ICMP Interface Information
extensions to authorized diagnostic sources so internal interface topology
(`ifIndex`, `ifName`, physical LAG member ports, internal next-hops, and MTUs)
is not leaked to untrusted hosts:

*   **Prefix Allowlist (`allowed-source-prefixes`)**: Modeled as an inline
    `leaf-list` of `openconfig-inet-types:ip-prefix` (`union` of `ipv4-prefix`
    and `ipv6-prefix`), accepting individual IPv4/IPv6 hosts (`/32` and `/128`)
    as well as arbitrary CIDR subnets (or `0.0.0.0/0` and `::/0` when explicitly
    authorizing all sources).
*   **Fail-Closed Empty List**: When `allowed-source-prefixes` is empty (`[]`)
    or unconfigured, **no** source is eligible to receive extended ICMP
    responses, even when `enabled = true`.
*   **Evaluation on Post-Decapsulation Response Target IP**: Eligibility MUST be
    evaluated against the **destination IP address to which the DUT generates
    the ICMP response**:
    *   When outer encapsulation is decapsulated (`decapsulate-headers` match),
        validation is performed on the **inner** packet's Source IP (regardless
        of whether the stripped outer Source IP was in
        `allowed-source-prefixes`).
    *   When no decapsulation occurs (native packet, `decapsulate-headers = []`,
        or unmatched tunnel protocol), validation is performed on the **outer**
        packet's Source IP (regardless of any inner payload Source IP).
*   **Plain ICMP Response for Unreferenced Sources**: When the ICMP response
    destination IP does **not** match any entry in `allowed-source-prefixes`,
    the DUT still performs configured outer decapsulation (if applicable) and
    transmits a **standard plain ICMPv4 or ICMPv6 error message** (RFC 792 /
    RFC 4443) with `Length = 0` (`0x00000000` in ICMP header octets `4..7`), no
    RFC 4884 Extension Header (`Ver = 2`), and zero `Class-Num = 2` or
    `Class-Num = 7` objects.

--------------------------------------------------------------------------------

## Testbed type

*   [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

*   Connect ATE `port1` and `port2` to DUT `port1` and `port2`, forming 2-member
    static or LACP ingress LAG `agg1`.
*   Connect ATE `port3` and `port4` to DUT `port3` and `port4`, forming 2-member
    static or LACP egress LAG `agg2`.
*   Configure L3 dual-stack addresses and MTUs (`9000` on transit interfaces) on
    DUT and ATE:
    *   Ingress `agg1`: DUT `192.0.2.1/30` and `2001:db8:1::1/126`; ATE
        `192.0.2.2/30` and `2001:db8:1::2/126`.
    *   Egress `agg2`: DUT `192.0.2.5/30` and `2001:db8:2::1/126`; ATE
        `192.0.2.6/30` and `2001:db8:2::2/126`.
    *   DUT `Loopback0`: `192.0.2.254/32` and `2001:db8:ffff::1/128`.
*   Configure static routes in `DEFAULT` network-instance on DUT:
    *   Forwarding routes toward ATE `agg2`: `203.0.113.0/24` → `192.0.2.6`,
        `198.51.100.0/24` → `192.0.2.6`, and `2001:db8:203::/64` →
        `2001:db8:2::2`.
    *   Return routes toward ATE `agg1` for allowed and unreferenced inner/outer
        probe source subnets:
        *   Allowed probe/tunnel-ingress subnets: `198.18.0.0/24` → `192.0.2.2`
            and `2001:db8:18::/64` → `2001:db8:1::2`.
        *   Unreferenced (non-allowed) probe/tunnel-ingress subnets:
            `198.19.0.0/24` → `192.0.2.2` and `2001:db8:19::/64` →
            `2001:db8:1::2`.
*   Start protocols on ATE and wait for IPv4 ARP and IPv6 ND resolution on both
    `agg1` and `agg2` before sending probe traffic.
*   Query and record baseline DUT interface state via `gNMI.Get` (`ifindex`,
    `name`, `mtu`, and configured IPv4/IPv6 addresses) for `agg1`, `agg2`,
    `port1`, `port2`, `port3`, and `port4` to validate against received ICMP
    extension sub-objects.
*   Configure ICMP RFC 5837 / `draft-mitchell-intarea-rfc5837bis` extensions on
    DUT with `enabled = true`, `allowed-source-prefixes` set to
    `["198.18.0.0/24", "2001:db8:18::/64"]`, all roles in `include-roles`, all
    sub-objects in `include-sub-objects`, `decapsulate-headers` set to
    `["IPV4"]`, and `max-original-datagram-size` set to `128`.

#### TODO: Proposed OpenConfig for ICMP Extensions and Traceroute Responder

Public OpenConfig models (`openconfig/public`) do not yet define configuration
and operational state leaves for RFC 5837 / `draft-mitchell-intarea-rfc5837bis`
extensions, source-prefix eligibility filtering, outer-header decapsulation
before ICMP error generation, or the RFC 4884 original datagram truncation cap.
Until standardized in `openconfig-system` (e.g., under `/system/icmp/`), the
following schema is proposed (reusing
`openconfig-aft-types:encapsulation-header-type` for `decapsulate-headers`,
`openconfig-inet-types:ip-prefix` for fail-closed `allowed-source-prefixes`, and
`include-roles` / `include-sub-objects` `leaf-list`s where `Class-Num = 7` is
emitted iff at least one Class `7` role such as `OUTGOING_SUB_IP_COMPONENT` is
listed in `include-roles`; may be applied via `cfgplugins` / vendor deviation in
initial test implementations):

```json
{
  "openconfig-system:system": {
    "icmp": {
      "config": {
        "max-original-datagram-size": 128,
        "decapsulate-headers": [
          "IPV4"
        ]
      },
      "interface-info-extension": {
        "config": {
          "enabled": true,
          "allowed-source-prefixes": [
            "198.18.0.0/24",
            "2001:db8:18::/64"
          ],
          "include-roles": [
            "INCOMING_IP_INTERFACE",
            "INCOMING_SUB_IP_COMPONENT",
            "OUTGOING_IP_INTERFACE",
            "IP_NEXT_HOP",
            "OUTGOING_SUB_IP_COMPONENT"
          ],
          "include-sub-objects": [
            "IFINDEX",
            "IP_ADDRESS",
            "NAME",
            "MTU"
          ]
        }
      }
    }
  }
}
```

### RT-9.1.1: Native IPv4 and IPv6 Transit Time Exceeded over LAG Interfaces (All Sub-Objects Populated)

*   Start packet capture on ATE `port1` and `port2` (registering immediate
    deferred capture stop cleanup).
*   Send the following transit probe bursts with `TTL = 1` (IPv4) and
    `Hop Limit = 1` (IPv6) from ATE `agg1` (`198.18.0.10` / `2001:db8:18::10`)
    toward `203.0.113.10` / `2001:db8:203::10`, varying L4 5-tuple ports across
    at least 16 flows (e.g., UDP destination ports `33434..33449` and ICMP Echo
    Identifier/Sequence pairs) and transmitting on both physical ingress links
    (`port1` and `port2`):
    1.  IPv4 UDP probes (`TTL = 1`).
    2.  IPv4 ICMP Echo Request probes (`TTL = 1`).
    3.  IPv6 UDP probes (`Hop Limit = 1`).
    4.  IPv6 ICMPv6 Echo Request probes (`Hop Limit = 1`).
*   Validate every captured ICMP error packet on ATE:
    *   Outer IP header has valid checksum/length, `Dst IP` matching probe
        `Src IP`, and `Src IP` matching DUT RFC 1812 / RFC 4443 selection.
    *   ICMP header is Time Exceeded (`ICMPv4`
        `Type = 11, Code = 0, Length = 32`; `ICMPv6`
        `Type = 3, Code = 0, Length = 16`) with valid ICMP checksum.
    *   RFC 4884 Extension Header has `Version = 2`, `Reserved = 0`, and valid
        16-bit one's complement checksum.
    *   **Class `2`, Role `0` (`C-Type = 0x0f`)**: `ifIndex`, `IPAddr`
        (`192.0.2.1` with `AFI = 1` for IPv4; `2001:db8:1::1` with `AFI = 2` for
        IPv6), `Name` (DUT `agg1` `ifName`, 4-octet `NULL`-padded), and `MTU`
        (`9000`) match DUT `gNMI.Get` state for `agg1` in exact sub-object
        order.
    *   **Class `2`, Role `1` (`C-Type = 0x4b`)**: `ifIndex`, `Name`, and `MTU`
        match the exact physical DUT ingress member port (`port1` when injected
        on ATE `port1`; `port2` when injected on ATE `port2`), with no `IPAddr`
        sub-object.
    *   **Class `2`, Role `2` (`C-Type = 0x8f`)**: `ifIndex`, `IPAddr`
        (`192.0.2.5` for IPv4; `2001:db8:2::1` for IPv6), `Name` (DUT `agg2`
        `ifName`), and `MTU` (`9000`) match DUT `gNMI.Get` state for `agg2`.
    *   **Class `2`, Role `3` (`C-Type = 0xc4`)**: `IPAddr` matches resolved
        next-hop (`192.0.2.6` with `AFI = 1` for IPv4; `2001:db8:2::2` with
        `AFI = 2` for IPv6).
    *   **Class `7`, Extended Role `0` (`C-Type = 0x0b`)**: `ifIndex`, `Name`,
        and `MTU` match an active physical member of `agg2` (`port3` or
        `port4`), and across the 16 L4 flows both `port3` and `port4` are
        observed.
*   **Egress LAG Member Failover / Admin-Down Verification**:
    *   Set `/interfaces/interface/config/enabled` = `false` on DUT `port3` via
        `gNMI.Set` (with deferred restoration to `true`) and wait via
        `gNMI.Subscribe` (`ON_CHANGE`) for
        `/interfaces/interface/state/oper-status` on `port3` to become `DOWN`.
    *   Re-send the 16-flow IPv4 and IPv6 `TTL / Hop Limit = 1` probe burst and
        verify that **100%** of captured responses now report DUT `port4` in
        `Class-Num = 7`, Extended Role `0`.
    *   Re-enable DUT `port3` (`enabled = true`) and wait via `gNMI.Subscribe`
        for `oper-status == UP` and LAG member viability restoration before
        proceeding.

### RT-9.1.2: Destination Unreachable (Port Unreachable) to DUT Interface / Loopback

*   Send IPv4 and IPv6 UDP probes with `TTL / Hop Limit = 64` from ATE `agg1`
    (`port1`) to an unused high UDP port (e.g., `33434`) on DUT `Loopback0`
    (`192.0.2.254` and `2001:db8:ffff::1`).
*   Validate captured ICMP responses on ATE:
    *   `ICMPv4` Destination Unreachable — Port Unreachable (`Type = 3`,
        `Code = 3`) and `ICMPv6` Destination Unreachable — Port Unreachable
        (`Type = 1`, `Code = 4`).
    *   **Class `2`, Role `0` (`Incoming IP Interface`)** and **Class `2`, Role
        `1` (`Incoming Sub-IP Component`)** are present and match `agg1` and
        `port1`.
    *   **Class `2`, Role `2` (`Outgoing IP Interface`)**, **Class `2`, Role `3`
        (`IP Next Hop`)**, and **Class `7`, Extended Role `0`
        (`Outgoing Sub-IP Component`)** are **omitted** because the packet
        terminated locally on the DUT and would not have been forwarded.

### RT-9.1.3: Sub-Object and Role Suppression Knobs

*   **Selective Sub-Object and Role Suppression Policy Verification**
    (`draft-mitchell` Sections 8.1 and 9, over the active `agg1` / `agg2` LAG
    topology):
    *   Configure suppression of `ifIndex` and `MTU`
        (`include-sub-objects = ["IP_ADDRESS", "NAME"]`). Send IPv4/IPv6
        `TTL / Hop Limit = 1` probes and verify `Bit 4` (`ifIndex`) and `Bit 7`
        (`MTU`) are `0` across all emitted objects
        (`Class 2 Role 0 C-Type = 0x06`, `Role 1 C-Type = 0x42`,
        `Role 2 C-Type = 0x86`, `Class 7 Role 0 C-Type = 0x02`) and only
        `IPAddr` / `Name` sub-objects appear on the wire.
    *   Restore `include-sub-objects = ["IFINDEX", "IP_ADDRESS", "NAME", "MTU"]`
        and configure suppression of the `Extended Interface Information Object`
        by omitting `"OUTGOING_SUB_IP_COMPONENT"` from `include-roles`
        (`include-roles = ["INCOMING_IP_INTERFACE", "INCOMING_SUB_IP_COMPONENT",
        "OUTGOING_IP_INTERFACE", "IP_NEXT_HOP"]`). Verify `Class-Num = 7` is
        omitted while `Class-Num = 2` objects remain intact.
    *   Restore full `include-roles` (re-adding `"OUTGOING_SUB_IP_COMPONENT"`)
        and `include-sub-objects` before proceeding.

### RT-9.1.4: Outer IPv4-in-IPv4 and IPv6-in-IPv4 Decapsulation Before Traceroute Response

*   **`IPV4` Decapsulation Enabled (`decapsulate-headers = ["IPV4"]`)**:
    *   **Subcase A — IPv4-in-IPv4 (`4in4`, `Protocol = 4`)**:
        *   Send IPv4-in-IPv4 packets on ATE `agg1` (`port1`) with outer IPv4
            `Src = 198.18.0.1`, `Dst = 198.51.100.2`, `Protocol = 4`, `TTL = 1`,
            `DSCP = AF21 (18)` encapsulating inner IPv4 `Src = 198.18.0.10`,
            `Dst = 203.0.113.10`, `TTL = 64`, UDP `33434 -> 33435`.
        *   Verify DUT strips the outer IPv4 header and sends an **`ICMPv4` Time
            Exceeded (`Type = 11, Code = 0`)** packet with:
            *   Outer IPv4 `Dst = 198.18.0.10` (inner Source IP, NOT
                `198.18.0.1`).
            *   Original Datagram starting with the **inner** IPv4 header
                (`Src = 198.18.0.10, Dst = 203.0.113.10, TTL = 64`).
            *   Extension objects populated per Example A above (`Class 2` Roles
                `0, 1, 2, 3` with `192.0.2.6` next-hop + `Class 7` Extended Role
                `0`).
    *   **Subcase B — IPv6-in-IPv4 (`6in4`, `Protocol = 41`)**:
        *   Send IPv6-in-IPv4 packets on ATE `agg1` (`port2`) with outer IPv4
            `Src = 198.18.0.1`, `Dst = 198.51.100.2`, `Protocol = 41`, `TTL = 1`
            encapsulating inner IPv6 `Src = 2001:db8:18::10`,
            `Dst = 2001:db8:203::10`, `Hop Limit = 64`, UDP `33434 -> 33435`.
        *   Verify DUT strips the outer IPv4 header and sends an **`ICMPv6` Time
            Exceeded (`Type = 3, Code = 0`)** packet with:
            *   Outer IPv6 `Dst = 2001:db8:18::10` (inner IPv6 Source IP).
            *   Original Datagram starting with the **inner** IPv6 header
                (`Src = 2001:db8:18::10`, `Dst = 2001:db8:203::10`,
                `Hop Limit = 64`).
            *   Extension objects populated per Example B above: `Class 2` Role
                `0` (`agg1` with `AFI = 2` `2001:db8:1::1`), Role `1` (`port2`),
                Role `2` (`agg2` with `AFI = 2` `2001:db8:2::1`), and `Class 7`
                Extended Role `0` (`port3` or `port4`), while **omitting
                `Class 2` Role `3` (`IP Next Hop`)** and containing zero
                `AFI = 1` IPv4 sub-objects anywhere in the `ICMPv6` message.
    *   **Subcase C — Inner Hop Limit Not Expired Control Check**:
        *   Send `4in4` and `6in4` packets with `Outer TTL = 64` and
            `Inner TTL / Hop Limit = 1`. Verify DUT forwards the tunneled
            packets normally out `agg2` (`Outer TTL = 63`, inner TTL/Hop Limit
            untouched) without generating transit Time Exceeded errors.
*   **Decapsulation Knob Disabled and Unmatched Encapsulation Verification**:
    *   Set `decapsulate-headers = []` (disabled) and re-send the `4in4` and
        `6in4` (`Outer Src = 198.18.0.1, Dst = 198.51.100.2, TTL = 1`) probes.
        Verify DUT does **not** strip the outer header: for both `4in4` and
        `6in4`, DUT sends an **`ICMPv4`** Time Exceeded message (with extension
        objects, since `198.18.0.1` matches `198.18.0.0/24`) to outer
        `Dst = 198.18.0.1` whose Original Datagram starts with the **outer**
        IPv4 header (`Src = 198.18.0.1, Dst = 198.51.100.2, TTL = 1`).
    *   Re-enable `decapsulate-headers = ["IPV4"]` and send **IPoGRE** (`Outer
        IPv4 Src = 198.18.0.1, Dst = 198.51.100.2, Protocol = 47, TTL = 1`
        carrying inner IPv4/IPv6) probes. Verify that because `GRE` is not
        enabled in `decapsulate-headers`, DUT responds to the **outer** IPv4
        Source IP (`198.18.0.1`) quoting the outer IPv4 header. (Once future
        `IPV6`, `GRE`, and `MPLS` values are implemented on the knob, separate
        subtests or extensions will enable them to validate outer IPv6/GRE/MPLS
        stripping.)

### RT-9.1.5: Configurable Original Datagram Size Cap (RFC 4884)

*   **Cap = `128` Octets (`max-original-datagram-size = 128`, RFC 4884
    Minimum)**:
    *   Send IPv4 and IPv6 UDP `TTL / Hop Limit = 1` probes of sizes:
        1.  **Short probe (`68` octets IP packet)** — under `128` octets.
        2.  **Large native probe (`512` octets IP packet)** — over `128` octets.
        3.  **Large `4in4` and `6in4` probes (`20`-octet outer IPv4 +
            `300`-octet inner IPv4/IPv6 packet)** with `IPV4` decap enabled.
    *   Verify in all three cases that:
        *   The padded/truncated Original Datagram field is **exactly `128`
            octets** (`ICMPv4` `Length = 32`; `ICMPv6` `Length = 16`).
        *   Short probes (`68` octets) are zero-padded (`0x00`) from octet `68`
            through octet `127`.
        *   Large native (`512B`) and decapsulated inner (`300B`) packets are
            truncated cleanly at `128` octets immediately before the RFC 4884
            Extension Structure Header (`Ver = 2`).
        *   All `Class-Num = 2` and `Class-Num = 7` extension objects follow at
            byte offset `128` of the Original Datagram with valid Extension
            Checksum.
*   **Cap = `256` Octets (`max-original-datagram-size = 256`)**:
    *   Update `max-original-datagram-size` to `256` on DUT.
    *   Send IPv4 and IPv6 UDP `TTL / Hop Limit = 1` probes of sizes:
        1.  **Short probe (`68` octets IP packet)**: Verify Original Datagram is
            still zero-padded to `128` octets (`ICMPv4` `Length = 32`, `ICMPv6`
            `Length = 16`).
        2.  **Unaligned intermediate probe (`190` octets IP packet)**: Verify
            `ICMPv4` zero-pads `2` bytes to `192` octets (`Length = 48` 4-byte
            words) and `ICMPv6` zero-pads `2` bytes to `192` octets
            (`Length = 24` 8-byte words).
        3.  **Large probe (`512` octets IP packet)**: Verify Original Datagram
            is truncated to **`256` octets** (`ICMPv4` `Length = 64`; `ICMPv6`
            `Length = 32`), all extension objects (`Class-Num = 2` and
            `Class-Num = 7`) remain intact, and total outer packet length does
            not exceed `576` octets for IPv4 (RFC 1812) or `1280` octets for
            IPv6 (RFC 4443).
    *   Restore `max-original-datagram-size` to `128` before proceeding.

### RT-9.1.6: Allowed Source Host and Subnet Filtering (Native and Decapsulated)

*   **Subcase A — IPv4 and IPv6 Subnet Match vs. Non-Match**:
    *   With `allowed-source-prefixes = ["198.18.0.0/24", "2001:db8:18::/64"]`,
        send native IPv4 and IPv6 UDP `TTL / Hop Limit = 1` transit probes
        toward `203.0.113.10` / `2001:db8:203::10` and `TTL / Hop Limit = 64`
        Port Unreachable probes toward DUT `Loopback0` from:
        1.  **Allowed subnet sources (`198.18.0.10` and `2001:db8:18::10`)**:
            Verify captured ICMPv4/ICMPv6 Time Exceeded and Port Unreachable
            responses carry the RFC 4884 Extension Header (`Ver = 2`) and
            expected `Class-Num = 2` / `Class-Num = 7` objects.
        2.  **Unreferenced subnet sources (`198.19.0.10` and
            `2001:db8:19::10`)**: Verify DUT still sends standard plain
            ICMPv4/ICMPv6 Time Exceeded and Port Unreachable responses to
            `198.19.0.10` / `2001:db8:19::10`, with `Length = 0` (`0x00000000`
            in ICMP header octets `4..7`) and **zero** RFC 4884 Extension
            Header, `Class-Num = 2`, or `Class-Num = 7` bytes.
*   **Subcase B — Host-Prefix Filtering (`/32` IPv4 and `/128` IPv6)**:
    *   Update `allowed-source-prefixes` to
        `["198.18.0.10/32", "2001:db8:18::10/128"]`.
    *   Send native IPv4 and IPv6 UDP `TTL / Hop Limit = 1` transit probes from:
        1.  **Exact host match (`198.18.0.10` and `2001:db8:18::10`)**: Verify
            captured responses carry full `Class-Num = 2` and `Class-Num = 7`
            extension objects.
        2.  **Adjacent host in same `/24` or `/64` (`198.18.0.11` and
            `2001:db8:18::11`)**: Verify captured responses are plain
            ICMPv4/ICMPv6 Time Exceeded messages (`Length = 0`, no extension
            objects).
*   **Subcase C — Empty Allowlist Fail-Closed and `enabled = false`
    Verification**:
    *   Set `allowed-source-prefixes = []` while keeping `enabled = true`. Send
        native IPv4 and IPv6 UDP `TTL / Hop Limit = 1` probes from `198.18.0.10`
        and `2001:db8:18::10` and verify captured responses are plain
        ICMPv4/ICMPv6 Time Exceeded messages (`Length = 0`, no extension
        objects).
    *   Set `enabled = false` with non-empty
        `allowed-source-prefixes = ["198.18.0.0/24", "2001:db8:18::/64"]`,
        verify `/system/icmp/interface-info-extension/state/enabled = false`,
        re-send native IPv4 and IPv6 UDP `TTL / Hop Limit = 1` probes from
        `198.18.0.10` and `2001:db8:18::10`, and verify captured responses
        remain plain ICMPv4/ICMPv6 Time Exceeded messages (`Length = 0`, no
        extension objects).
    *   Restore `enabled = true` and verify
        `/system/icmp/interface-info-extension/state/enabled = true` before
        proceeding.
*   **Subcase D — Encapsulated Outer vs. Inner Response Target IP Matrix**:
    *   With `enabled = true` and
        `allowed-source-prefixes = ["198.18.0.0/24", "2001:db8:18::/64"]`, send
        `4in4` (`Protocol = 4`) and `6in4` (`Protocol = 41`) probes on ATE
        `agg1` (`Outer Dst = 198.51.100.2, Outer TTL = 1`,
        `Inner Dst = 203.0.113.10` /
        `2001:db8:203::10, Inner TTL / Hop Limit = 64`) across all four
        combinations:
        1.  **`decapsulate-headers = ["IPV4"]`, Unreferenced Outer
            (`198.19.0.1`) + Allowed Inner (`198.18.0.10` /
            `2001:db8:18::10`)**: Verify DUT strips the outer header, targets
            the **inner** Source IP (matched), and sends **extended** `ICMPv4` /
            `ICMPv6` Time Exceeded responses with `Class-Num = 2` and
            `Class-Num = 7` objects.
        2.  **`decapsulate-headers = ["IPV4"]`, Allowed Outer (`198.18.0.1`) +
            Unreferenced Inner (`198.19.0.10` / `2001:db8:19::10`)**: Verify DUT
            strips the outer header, targets the **inner** Source IP
            (unreferenced, despite the outer IP matching `198.18.0.0/24`), and
            sends **plain** `ICMPv4` / `ICMPv6` Time Exceeded responses to
            `198.19.0.10` / `2001:db8:19::10` quoting the inner packet with
            `Length = 0` and **no** extension objects.
        3.  **`decapsulate-headers = []`, Allowed Outer (`198.18.0.1`) +
            Unreferenced Inner (`198.19.0.10` / `2001:db8:19::10`)**: Verify no
            decapsulation occurs, DUT targets the **outer** Source IP
            `198.18.0.1` (matched, despite the inner IP being unreferenced), and
            sends **extended** `ICMPv4` Time Exceeded responses to `198.18.0.1`
            quoting the outer IPv4 header with `Class-Num = 2` and
            `Class-Num = 7` objects.
        4.  **`decapsulate-headers = []`, Unreferenced Outer (`198.19.0.1`) +
            Allowed Inner (`198.18.0.10` / `2001:db8:18::10`)**: Verify no
            decapsulation occurs, DUT targets the **outer** Source IP
            `198.19.0.1` (unreferenced), and sends **plain** `ICMPv4` Time
            Exceeded responses to `198.19.0.1` quoting the outer IPv4 header
            with `Length = 0` and **no** extension objects.
    *   Restore `decapsulate-headers = ["IPV4"]` before proceeding to
        `RT-9.1.7`.

### RT-9.1.7: Singleton L3 Interfaces (Sub-IP Component Omission)

*   Reconfigure DUT and ATE at the end of the test suite so `port1`
    (`192.0.2.1/30`, `2001:db8:1::1/126`) and `port3` (`192.0.2.5/30`,
    `2001:db8:2::1/126`) operate as standalone singleton L3 Ethernet interfaces
    (removing `agg1` and `agg2` LAG membership), resolve IPv4 ARP and IPv6 ND,
    and send IPv4 and IPv6 `TTL / Hop Limit = 1` UDP probes from allowed sources
    `198.18.0.10` / `2001:db8:18::10` toward `203.0.113.10` /
    `2001:db8:203::10`.
*   Verify captured ICMP Time Exceeded responses include **Class `2`, Role `0`**
    (`port1`), **Class `2`, Role `2`** (`port3`), and **Class `2`, Role `3`**
    (`192.0.2.6` / `2001:db8:2::2`), while **omitting** both **Class `2`, Role
    `1` (`Incoming Sub-IP Component`)** and **Class `7`, Extended Role `0`
    (`Outgoing Sub-IP Component`)**.

### Cleanup

*   Stop all active OTG packet captures and traffic flows on ATE.
*   Ensure all DUT and ATE physical interfaces (`port1`..`port4`) and aggregate
    interfaces (`agg1`, `agg2`) are restored to admin-enabled state, and revert
    all modified ICMP extension, interface, LAG, and static-route configurations
    on DUT and ATE to restore the testbed to its baseline operational state.

## Canonical OC

```json
{
  "openconfig-interfaces:interfaces": {
    "interface": [
      {
        "name": "port1",
        "config": {
          "enabled": true,
          "mtu": 9000,
          "name": "port1",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "ethernet": {
          "config": {
            "aggregate-id": "agg1"
          }
        }
      },
      {
        "name": "port2",
        "config": {
          "enabled": true,
          "mtu": 9000,
          "name": "port2",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "ethernet": {
          "config": {
            "aggregate-id": "agg1"
          }
        }
      },
      {
        "name": "port3",
        "config": {
          "enabled": true,
          "mtu": 9000,
          "name": "port3",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "ethernet": {
          "config": {
            "aggregate-id": "agg2"
          }
        }
      },
      {
        "name": "port4",
        "config": {
          "enabled": true,
          "mtu": 9000,
          "name": "port4",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "ethernet": {
          "config": {
            "aggregate-id": "agg2"
          }
        }
      },
      {
        "name": "agg1",
        "aggregation": {
          "config": {
            "lag-type": "STATIC"
          }
        },
        "config": {
          "enabled": true,
          "mtu": 9000,
          "name": "agg1",
          "type": "iana-if-type:ieee8023adLag"
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "config": {
                "enabled": true,
                "index": 0
              },
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "ip": "192.0.2.1",
                      "config": {
                        "ip": "192.0.2.1",
                        "prefix-length": 30
                      }
                    }
                  ]
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "ip": "2001:db8:1::1",
                      "config": {
                        "ip": "2001:db8:1::1",
                        "prefix-length": 126
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
        "name": "agg2",
        "aggregation": {
          "config": {
            "lag-type": "STATIC"
          }
        },
        "config": {
          "enabled": true,
          "mtu": 9000,
          "name": "agg2",
          "type": "iana-if-type:ieee8023adLag"
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "config": {
                "enabled": true,
                "index": 0
              },
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "ip": "192.0.2.5",
                      "config": {
                        "ip": "192.0.2.5",
                        "prefix-length": 30
                      }
                    }
                  ]
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "ip": "2001:db8:2::1",
                      "config": {
                        "ip": "2001:db8:2::1",
                        "prefix-length": 126
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
        "name": "Loopback0",
        "config": {
          "enabled": true,
          "name": "Loopback0",
          "type": "iana-if-type:softwareLoopback"
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "config": {
                "enabled": true,
                "index": 0
              },
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "ip": "192.0.2.254",
                      "config": {
                        "ip": "192.0.2.254",
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
                      "ip": "2001:db8:ffff::1",
                      "config": {
                        "ip": "2001:db8:ffff::1",
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
  },
  "openconfig-network-instance:network-instances": {
    "network-instance": [
      {
        "name": "DEFAULT",
        "config": {
          "name": "DEFAULT"
        },
        "protocols": {
          "protocol": [
            {
              "identifier": "openconfig-policy-types:STATIC",
              "name": "DEFAULT",
              "config": {
                "identifier": "openconfig-policy-types:STATIC",
                "name": "DEFAULT"
              },
              "static-routes": {
                "static": [
                  {
                    "prefix": "203.0.113.0/24",
                    "config": {
                      "prefix": "203.0.113.0/24"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "index": "0",
                          "config": {
                            "index": "0",
                            "next-hop": "192.0.2.6"
                          }
                        }
                      ]
                    }
                  },
                  {
                    "prefix": "198.51.100.0/24",
                    "config": {
                      "prefix": "198.51.100.0/24"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "index": "0",
                          "config": {
                            "index": "0",
                            "next-hop": "192.0.2.6"
                          }
                        }
                      ]
                    }
                  },
                  {
                    "prefix": "198.18.0.0/24",
                    "config": {
                      "prefix": "198.18.0.0/24"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "index": "0",
                          "config": {
                            "index": "0",
                            "next-hop": "192.0.2.2"
                          }
                        }
                      ]
                    }
                  },
                  {
                    "prefix": "198.19.0.0/24",
                    "config": {
                      "prefix": "198.19.0.0/24"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "index": "0",
                          "config": {
                            "index": "0",
                            "next-hop": "192.0.2.2"
                          }
                        }
                      ]
                    }
                  },
                  {
                    "prefix": "2001:db8:203::/64",
                    "config": {
                      "prefix": "2001:db8:203::/64"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "index": "0",
                          "config": {
                            "index": "0",
                            "next-hop": "2001:db8:2::2"
                          }
                        }
                      ]
                    }
                  },
                  {
                    "prefix": "2001:db8:18::/64",
                    "config": {
                      "prefix": "2001:db8:18::/64"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "index": "0",
                          "config": {
                            "index": "0",
                            "next-hop": "2001:db8:1::2"
                          }
                        }
                      ]
                    }
                  },
                  {
                    "prefix": "2001:db8:19::/64",
                    "config": {
                      "prefix": "2001:db8:19::/64"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "index": "0",
                          "config": {
                            "index": "0",
                            "next-hop": "2001:db8:1::2"
                          }
                        }
                      ]
                    }
                  }
                ]
              }
            }
          ]
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
  ## Interface & Sub-Object Correlation Paths ##
  /interfaces/interface/aggregation/config/lag-type:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/mtu:
  /interfaces/interface/ethernet/config/aggregate-id:
  /interfaces/interface/state/ifindex:
  /interfaces/interface/state/mtu:
  /interfaces/interface/state/name:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/state/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/state/ip:

  ## TODO: Proposed OpenConfig ICMP Extension, Decapsulation, and Size Cap Paths ##
  # See https://github.com/openconfig/public for future YANG model standardization:
  # /system/icmp/config/max-original-datagram-size:
  # /system/icmp/config/decapsulate-headers:
  # /system/icmp/state/max-original-datagram-size:
  # /system/icmp/state/decapsulate-headers:
  # /system/icmp/interface-info-extension/config/enabled:
  # /system/icmp/interface-info-extension/config/allowed-source-prefixes:
  # /system/icmp/interface-info-extension/config/include-roles:
  # /system/icmp/interface-info-extension/config/include-sub-objects:
  # /system/icmp/interface-info-extension/state/enabled:
  # /system/icmp/interface-info-extension/state/allowed-source-prefixes:
  # /system/icmp/interface-info-extension/state/include-roles:
  # /system/icmp/interface-info-extension/state/include-sub-objects:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

FFF
