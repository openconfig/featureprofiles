# RT-1.111: BGP Autonomous System Confederations (RFC 5065)

## Summary

This test validates support for BGP Autonomous System Confederations as defined
in [RFC 5065](https://datatracker.ietf.org/doc/html/rfc5065) using OpenConfig
data models. A BGP confederation subdivides a single Autonomous System
(identified by a global Confederation Identifier ASN) into multiple smaller
Member Autonomous Systems (`member-as`) to reduce the internal BGP (iBGP)
full-mesh session scale requirement while presenting a single unified Autonomous
System to external BGP (eBGP) peers.

Specifically, this test validates the following functional and telemetry
requirements across IPv4 and IPv6 unicast address families:

1.  **Configuration & Operational State Telemetry**: Setting and verifying the
    OpenConfig BGP global confederation `identifier` and `member-as` parameters
    via gNMI `Set` and gNMI `Subscribe`/`Get`.
2.  **Intra-Confederation eBGP Peering & `AS_CONFED_SEQUENCE` Propagation**:
    Establishing a confederation-eBGP session between two devices (DUT 1 and
    DUT 2) configured in distinct Member Autonomous Systems within the same
    confederation, and verifying that routes advertised across the member-AS
    boundary carry the `AS_CONFED_SEQUENCE` path attribute segment containing
    the originating Member-AS number.
3.  **Confederation Boundary Stripping Toward External Peers**: Verifying that
    when DUT 1 advertises confederation-internal routes across a standard eBGP
    session to an external Autonomous System (emulated on the ATE), all
    internal `AS_CONFED_SEQUENCE` and `AS_CONFED_SET` segments (and Member-AS
    numbers) are stripped from the `AS_PATH` attribute, leaving only the global
    Confederation Identifier ASN visible to the external peer, and that
    end-to-end data plane traffic is forwarded without loss.

## Testbed type

*   [`featureprofiles/topologies/dutdutate.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dutdutate.testbed) (`TESTBED_DUT_DUT_ATE_2LINKS`)

## Procedure

### Test environment setup

#### Test Topology

The testbed consists of an Automated Test Equipment (ATE) device and two Devices
Under Test (DUT 1 and DUT 2) connected serially:

*   `ATE:port1` is connected to `DUT1:port1` (External eBGP boundary link).
*   `DUT1:port2` is connected to `DUT2:port1` (Intra-confederation eBGP
    member-AS link).

```text
                    External eBGP                 Confederation eBGP
   +-----------+                    +-----------+                    +-----------+
   |    ATE    |====================|   DUT 1   |====================|   DUT 2   |
   |  AS 64510 | port1        port1 | Member-AS | port2        port1 | Member-AS |
   |           |                    |   64501   |                    |   64502   |
   +-----------+                    +-----------+                    +-----------+
                                     \                              /
                                      \__ Confederation ID 64500 __/
```

#### BGP Autonomous System Numbers (Table 1)

All Autonomous System Numbers are allocated from the RFC 5398 documentation
range:

| Device | Network Instance | BGP Role | Local Member ASN (`global/config/as`) | Confederation Identifier (`confederation/config/identifier`) | Confederation Member-AS List (`confederation/config/member-as`) |
| :--- | :--- | :--- | :--- | :--- | :--- |
| DUT 1 | `DEFAULT` | Confederation Member & Boundary Router | `64501` | `64500` | `[64502]` |
| DUT 2 | `DEFAULT` | Confederation Member & Route Originator | `64502` | `64500` | `[64501]` |
| ATE | Emulated Router | External eBGP Peer | `64510` | N/A | N/A |

#### Interface IPv4 and IPv6 Addressing (Table 2)

IPv4 addresses are allocated from the RFC 5737 documentation blocks and IPv6
addresses from the RFC 3849 documentation prefix:

| Link | Device & Port | IPv4 Address / Prefix | IPv6 Address / Prefix |
| :--- | :--- | :--- | :--- |
| Link 1 (`ATE:port1` <-> `DUT1:port1`) | `DUT1:port1` | `192.0.2.1/30` | `2001:db8::1/126` |
| Link 1 (`ATE:port1` <-> `DUT1:port1`) | `ATE:port1` | `192.0.2.2/30` | `2001:db8::2/126` |
| Link 2 (`DUT1:port2` <-> `DUT2:port1`) | `DUT1:port2` | `192.0.2.5/30` | `2001:db8::5/126` |
| Link 2 (`DUT1:port2` <-> `DUT2:port1`) | `DUT2:port1` | `192.0.2.6/30` | `2001:db8::6/126` |
| Loopback (`Loopback0`) | `DUT1:Loopback0` | `198.51.100.1/32` | `2001:db8:1::1/128` |
| Loopback (`Loopback0`) | `DUT2:Loopback0` | `198.51.100.2/32` | `2001:db8:2::1/128` |

#### Advertised BGP Route Prefixes (Table 3)

| Originating Device | IPv4 Prefix | IPv6 Prefix | Purpose |
| :--- | :--- | :--- | :--- |
| DUT 2 (AS `64502`) | `198.51.100.0/24` | `2001:db8:2::/48` | Originated inside Member-AS `64502`, propagated to DUT 1 (`64501`), and advertised externally to ATE (`64510`). |
| ATE (AS `64510`) | `203.0.113.0/24` | `2001:db8:3::/48` | Originated by external AS `64510`, advertised into the confederation via DUT 1 to DUT 2. |

#### ATE Traffic Profile (Table 4)

| Flow Name | Address Family | Ingress Port | Source IP | Destination IP | Frame Size (Bytes) | Rate | Duration |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `flow-ipv4-ext-to-confed` | IPv4 | `ATE:port1` | `203.0.113.10` | `198.51.100.2` | `512` | `1000 pps` | `30s` |
| `flow-ipv6-ext-to-confed` | IPv6 | `ATE:port1` | `2001:db8:3::10` | `2001:db8:2::1` | `512` | `1000 pps` | `30s` |

---

### RT-1.111.1: Confederation Configuration and Telemetry State Verification

This subtest verifies that both DUT 1 and DUT 2 accept the OpenConfig BGP global
confederation configuration parameters (`identifier` and `member-as`) via gNMI
`Set` and accurately reflect those values in the corresponding operational
`state` leaves via gNMI telemetry.

#### Step 1: Configure Interfaces and BGP Confederation Parameters on DUT 1 and DUT 2

1.  On **DUT 1**, configure physical interfaces `DUT1:port1`, `DUT1:port2`, and
    loopback interface `Loopback0` with the IPv4 and IPv6 addresses defined in
    Table 2, and bind them to the `DEFAULT` network instance.
2.  On **DUT 2**, configure physical interface `DUT2:port1` and loopback
    interface `Loopback0` with the IPv4 and IPv6 addresses defined in Table 2,
    and bind them to the `DEFAULT` network instance.
3.  On **DUT 1**, configure BGP in the `DEFAULT` network instance using gNMI
    `Set`:
    *   Set the local Member Autonomous System number to `64501` at
        `/network-instances/network-instance/protocols/protocol/bgp/global/config/as`.
    *   Set the BGP router ID to `198.51.100.1` at
        `/network-instances/network-instance/protocols/protocol/bgp/global/config/router-id`.
    *   Set the BGP confederation identifier to `64500` at
        `/network-instances/network-instance/protocols/protocol/bgp/global/confederation/config/identifier`.
    *   Configure the confederation member-AS leaf-list with `[64502]` at
        `/network-instances/network-instance/protocols/protocol/bgp/global/confederation/config/member-as`.
4.  On **DUT 2**, configure BGP in the `DEFAULT` network instance using gNMI
    `Set`:
    *   Set the local Member Autonomous System number to `64502` at
        `/network-instances/network-instance/protocols/protocol/bgp/global/config/as`.
    *   Set the BGP router ID to `198.51.100.2` at
        `/network-instances/network-instance/protocols/protocol/bgp/global/config/router-id`.
    *   Set the BGP confederation identifier to `64500` at
        `/network-instances/network-instance/protocols/protocol/bgp/global/confederation/config/identifier`.
    *   Configure the confederation member-AS leaf-list with `[64501]` at
        `/network-instances/network-instance/protocols/protocol/bgp/global/confederation/config/member-as`.

#### Step 2: Verify Operational State Telemetry

1.  Using gNMI `Subscribe` (or `Get`), query the operational state paths on
    **DUT 1** and **DUT 2**:
    *   `/network-instances/network-instance/protocols/protocol/bgp/global/state/as`
    *   `/network-instances/network-instance/protocols/protocol/bgp/global/confederation/state/identifier`
    *   `/network-instances/network-instance/protocols/protocol/bgp/global/confederation/state/member-as`

#### Subtest RT-1.111.1 Pass/Fail Criteria

*   **Pass**:
    *   On **DUT 1**:
        *   `/network-instances/network-instance/protocols/protocol/bgp/global/state/as`
            equals `64501`.
        *   `/network-instances/network-instance/protocols/protocol/bgp/global/confederation/state/identifier`
            equals `64500`.
        *   `/network-instances/network-instance/protocols/protocol/bgp/global/confederation/state/member-as`
            equals `[64502]`.
    *   On **DUT 2**:
        *   `/network-instances/network-instance/protocols/protocol/bgp/global/state/as`
            equals `64502`.
        *   `/network-instances/network-instance/protocols/protocol/bgp/global/confederation/state/identifier`
            equals `64500`.
        *   `/network-instances/network-instance/protocols/protocol/bgp/global/confederation/state/member-as`
            equals `[64501]`.
*   **Fail**: Any of the state leaves is missing, unsupported, nil, or returns a
    value that does not match the configured value.

---

### RT-1.111.2: Intra-Confederation Peering and `AS_CONFED_SEQUENCE` Verification

This subtest verifies that DUT 1 (Member-AS `64501`) and DUT 2 (Member-AS
`64502`) establish dual-stack (IPv4 and IPv6) confederation-eBGP sessions over
Link 2, and that prefixes originated by DUT 2 are received in the BGP RIB on
DUT 1 with an `AS_PATH` segment of type `AS_CONFED_SEQUENCE` containing
Member-AS `64502`.

#### Step 1: Configure Intra-Confederation BGP Neighbors and Route Origination

1.  Create a routing policy named `ALLOW` on both **DUT 1** and **DUT 2** with a
    default statement that sets `policy-result` to `ACCEPT_ROUTE`.
2.  On **DUT 1**, configure peer-groups `CONFED-PEER-V4` and `CONFED-PEER-V6`
    with `peer-as` set to `64502` and apply import/export policy `ALLOW` under
    `IPV4_UNICAST` and `IPV6_UNICAST` AFI/SAFIs respectively.
3.  On **DUT 1**, configure BGP neighbors on Link 2 towards DUT 2:
    *   IPv4 neighbor `192.0.2.6` assigned to `CONFED-PEER-V4` with `peer-as`
        set to `64502` at
        `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as`.
    *   IPv6 neighbor `2001:db8::6` assigned to `CONFED-PEER-V6` with `peer-as`
        set to `64502` at
        `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as`.
4.  On **DUT 2**, configure peer-groups `CONFED-PEER-V4` and `CONFED-PEER-V6`
    with `peer-as` set to `64501` and apply import/export policy `ALLOW` under
    `IPV4_UNICAST` and `IPV6_UNICAST` AFI/SAFIs respectively.
5.  On **DUT 2**, configure BGP neighbors on Link 2 towards DUT 1:
    *   IPv4 neighbor `192.0.2.5` assigned to `CONFED-PEER-V4` with `peer-as`
        set to `64501`.
    *   IPv6 neighbor `2001:db8::5` assigned to `CONFED-PEER-V6` with `peer-as`
        set to `64501`.
6.  On **DUT 2**, originate the IPv4 prefix `198.51.100.0/24` and IPv6 prefix
    `2001:db8:2::/48` (e.g., via static route redistribution or connected
    loopback prefixes into BGP) so that they are advertised over the
    confederation-eBGP sessions to DUT 1.

#### Step 2: Verify Session Establishment and RIB `AS_CONFED_SEQUENCE` Telemetry on DUT 1

1.  Await
    `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state`
    reaching `ESTABLISHED` for both `192.0.2.6` and `2001:db8::6` on **DUT 1**,
    and `192.0.2.5` and `2001:db8::5` on **DUT 2**.
2.  On **DUT 1**, subscribe to the BGP RIB attribute sets and Loc-RIB /
    Adj-RIB-In tables for `198.51.100.0/24` and `2001:db8:2::/48`, and inspect
    the `as-path` segments at:
    *   `/network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/index`
    *   `/network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/type`
    *   `/network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/member`

#### Subtest RT-1.111.2 Pass/Fail Criteria

*   **Pass**:
    *   All IPv4 and IPv6 intra-confederation BGP sessions between DUT 1 and
        DUT 2 transition to `ESTABLISHED`.
    *   On **DUT 1**, the BGP RIB entries for `198.51.100.0/24` and
        `2001:db8:2::/48` reference an `attr-set` whose `as-segment` list has
        `/network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/type`
        set to `AS_CONFED_SEQUENCE` and
        `/network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/member`
        containing `64502`.
*   **Fail**:
    *   Either the IPv4 or IPv6 session fails to reach `ESTABLISHED`; or
    *   The received route is missing from DUT 1's BGP RIB; or
    *   The segment `type` is `AS_SEQ` instead of `AS_CONFED_SEQUENCE`, or
        `64502` is missing from `state/member`.

---

### RT-1.111.3: Confederation Boundary Stripping Toward External eBGP Peer and Data Plane Verification

This subtest verifies the primary external behavior required by RFC 5065
Section 5.3: when a confederation boundary router (DUT 1) advertises a route
learned from inside the confederation (`198.51.100.0/24` and `2001:db8:2::/48`
from Member-AS `64502`) to an external eBGP peer outside the confederation (ATE
in AS `64510`), DUT 1 strips all `AS_CONFED_SEQUENCE` and `AS_CONFED_SET`
segments from the `AS_PATH` and prepends the Confederation Identifier (`64500`)
as a standard `AS_SEQ` segment. Consequently, the external peer observes only
AS `64500` in the `AS_PATH` and never observes internal Member-AS numbers
`64501` or `64502`.

#### Step 1: Configure External eBGP Sessions Between DUT 1 and ATE

1.  On **DUT 1**, configure peer-groups `EBGP-PEER-V4` and `EBGP-PEER-V6` with
    `peer-as` set to `64510` and apply import/export policy `ALLOW` under
    `IPV4_UNICAST` and `IPV6_UNICAST` AFI/SAFIs.
2.  On **DUT 1**, configure BGP neighbors on Link 1 (`DUT1:port1`) towards
    `ATE:port1`:
    *   IPv4 neighbor `192.0.2.2` assigned to `EBGP-PEER-V4` with `peer-as` set
        to `64510` at
        `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as`.
    *   IPv6 neighbor `2001:db8::2` assigned to `EBGP-PEER-V6` with `peer-as`
        set to `64510` at
        `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as`.
3.  Note: When peering with a true external eBGP neighbor (`64510`, which is
    NOT in `confederation/config/member-as`), DUT 1 must use its Confederation
    Identifier (`64500`) as its externally presented AS number per RFC 5065.
4.  On the **ATE** (`ATE:port1`), configure an emulated router in AS `64510`
    with IPv4 address `192.0.2.2/30` and IPv6 address `2001:db8::2/126`:
    *   Configure an IPv4 eBGP peer to `192.0.2.1` with remote AS `64500` (the
        Confederation Identifier of DUT 1).
    *   Configure an IPv6 eBGP peer to `2001:db8::1` with remote AS `64500`.
    *   Advertise external IPv4 prefix `203.0.113.0/24` and IPv6 prefix
        `2001:db8:3::/48` from the ATE towards DUT 1.

#### Step 2: Verify External `AS_PATH` Stripping on ATE and Bidirectional Forwarding

1.  Start protocols on OTG/ATE and wait for both IPv4 (`192.0.2.2`) and IPv6
    (`2001:db8::2`) external eBGP sessions on **DUT 1** to reach `ESTABLISHED`
    at
    `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state`.
2.  Query the OTG/ATE BGP RIB telemetry for the received IPv4 route
    `198.51.100.0/24` and IPv6 route `2001:db8:2::/48` advertised by DUT 1:
    *   Inspect the received `AS_PATH` segments on the ATE.
    *   Verify that the `AS_PATH` contains a single `AS_SEQ` segment with AS
        number `[64500]`.
    *   Verify that neither `64501` nor `64502` appears anywhere in the
        received `AS_PATH`, and that no `AS_CONFED_SEQUENCE` or `AS_CONFED_SET`
        segment is present.
3.  Wait for ARP and IPv6 Neighbor Discovery resolution to complete, then start
    the IPv4 and IPv6 traffic flows defined in Table 4
    (`flow-ipv4-ext-to-confed` and `flow-ipv6-ext-to-confed`) from `ATE:port1`
    destined to DUT 2 (`198.51.100.2` and `2001:db8:2::1`) for 30 seconds.
4.  Stop traffic and verify packet counters.

#### Subtest RT-1.111.3 Pass/Fail Criteria

*   **Pass**:
    *   Both IPv4 and IPv6 eBGP sessions between `DUT1:port1` and `ATE:port1`
        reach `ESTABLISHED`.
    *   On the **ATE**, the BGP UPDATE received for `198.51.100.0/24` and
        `2001:db8:2::/48` has an `AS_PATH` equal to `[64500]` (type `AS_SEQ`),
        contains **neither** `64501` **nor** `64502`, and contains **zero**
        `AS_CONFED_SEQUENCE` or `AS_CONFED_SET` segments.
    *   Both IPv4 and IPv6 traffic flows traverse `ATE -> DUT 1 -> DUT 2` with
        0% packet loss.
*   **Fail**:
    *   External eBGP sessions fail to establish against Confederation
        Identifier `64500`; or
    *   Member-AS `64501` or `64502` leaks into the `AS_PATH` visible at the
        ATE; or
    *   Any `AS_CONFED_SEQUENCE` or `AS_CONFED_SET` attribute segment is sent to
        the external peer; or
    *   Traffic loss is greater than 0%.

### Cleanup

1.  Revert all BGP configurations on **DUT 1** and **DUT 2** (including the
    BGP confederation `identifier`, `member-as` lists, BGP neighbors, and
    peer-groups) via gNMI `Set` to restore both devices to their baseline
    operational state.
2.  Remove the configured IPv4/IPv6 interface addresses, loopback interfaces,
    and routing policies (`ALLOW`) on both **DUT 1** and **DUT 2**, and stop
    all protocols and traffic flows on the **ATE**.

## Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "DUT1 to ATE External eBGP Link",
          "enabled": true,
          "name": "eth1",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "eth1",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "enabled": true,
                "index": 0
              },
              "index": 0,
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
                },
                "config": {
                  "enabled": true
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8::1"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              }
            }
          ]
        }
      },
      {
        "config": {
          "description": "DUT1 to DUT2 Confederation eBGP Link",
          "enabled": true,
          "name": "eth2",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "eth2",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "enabled": true,
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.5",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.5"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8::5",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8::5"
                    }
                  ]
                },
                "config": {
                  "enabled": true
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
          "name": "DEFAULT",
          "type": "openconfig-network-instance-types:DEFAULT_INSTANCE"
        },
        "name": "DEFAULT",
        "protocols": {
          "protocol": [
            {
              "bgp": {
                "global": {
                  "afi-safis": {
                    "afi-safi": [
                      {
                        "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                        "config": {
                          "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                          "enabled": true
                        }
                      },
                      {
                        "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                        "config": {
                          "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                          "enabled": true
                        }
                      }
                    ]
                  },
                  "confederation": {
                    "config": {
                      "identifier": 64500,
                      "member-as": [
                        64502
                      ]
                    }
                  },
                  "config": {
                    "as": 64501,
                    "router-id": "198.51.100.1"
                  }
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "config": {
                        "enabled": true,
                        "neighbor-address": "192.0.2.2",
                        "peer-as": 64510,
                        "peer-group": "EBGP-PEER-V4"
                      },
                      "neighbor-address": "192.0.2.2"
                    },
                    {
                      "config": {
                        "enabled": true,
                        "neighbor-address": "2001:db8::2",
                        "peer-as": 64510,
                        "peer-group": "EBGP-PEER-V6"
                      },
                      "neighbor-address": "2001:db8::2"
                    },
                    {
                      "config": {
                        "enabled": true,
                        "neighbor-address": "192.0.2.6",
                        "peer-as": 64502,
                        "peer-group": "CONFED-PEER-V4"
                      },
                      "neighbor-address": "192.0.2.6"
                    },
                    {
                      "config": {
                        "enabled": true,
                        "neighbor-address": "2001:db8::6",
                        "peer-as": 64502,
                        "peer-group": "CONFED-PEER-V6"
                      },
                      "neighbor-address": "2001:db8::6"
                    }
                  ]
                },
                "peer-groups": {
                  "peer-group": [
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ALLOW"
                                ],
                                "import-policy": [
                                  "ALLOW"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                              "enabled": true
                            }
                          }
                        ]
                      },
                      "config": {
                        "peer-as": 64510,
                        "peer-group-name": "EBGP-PEER-V4"
                      },
                      "peer-group-name": "EBGP-PEER-V4"
                    },
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ALLOW"
                                ],
                                "import-policy": [
                                  "ALLOW"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                              "enabled": true
                            }
                          }
                        ]
                      },
                      "config": {
                        "peer-as": 64510,
                        "peer-group-name": "EBGP-PEER-V6"
                      },
                      "peer-group-name": "EBGP-PEER-V6"
                    },
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ALLOW"
                                ],
                                "import-policy": [
                                  "ALLOW"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                              "enabled": true
                            }
                          }
                        ]
                      },
                      "config": {
                        "peer-as": 64502,
                        "peer-group-name": "CONFED-PEER-V4"
                      },
                      "peer-group-name": "CONFED-PEER-V4"
                    },
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ALLOW"
                                ],
                                "import-policy": [
                                  "ALLOW"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                              "enabled": true
                            }
                          }
                        ]
                      },
                      "config": {
                        "peer-as": 64502,
                        "peer-group-name": "CONFED-PEER-V6"
                      },
                      "peer-group-name": "CONFED-PEER-V6"
                    }
                  ]
                }
              },
              "config": {
                "identifier": "openconfig-policy-types:BGP",
                "name": "BGP"
              },
              "identifier": "openconfig-policy-types:BGP",
              "name": "BGP"
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
            "name": "ALLOW"
          },
          "name": "ALLOW",
          "statements": {
            "statement": [
              {
                "actions": {
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                },
                "config": {
                  "name": "accept-all"
                },
                "name": "accept-all"
              }
            ]
          }
        }
      ]
    }
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  ## Config Parameter Coverage
  /network-instances/network-instance/protocols/protocol/bgp/global/config/as:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id:
  /network-instances/network-instance/protocols/protocol/bgp/global/confederation/config/identifier:
  /network-instances/network-instance/protocols/protocol/bgp/global/confederation/config/member-as:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/neighbor-address:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-group:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-as:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-group-name:

  ## Telemetry Parameter Coverage
  /network-instances/network-instance/protocols/protocol/bgp/global/state/as:
  /network-instances/network-instance/protocols/protocol/bgp/global/confederation/state/identifier:
  /network-instances/network-instance/protocols/protocol/bgp/global/confederation/state/member-as:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/index:
  /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/type:
  /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/member:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Required DUT platform

*   vRX or FFF
