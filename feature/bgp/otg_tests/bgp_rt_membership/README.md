# RT-1.106: BGP RT Membership Constraints (RFC 4684)

## Summary

Validate RFC 4684 Route Target (RT) Membership Constrained Route Distribution
behavior in iBGP to prevent flooding VPN routes. Ensure the DUT only advertises
VPN routes to peers that have requested the corresponding Route Targets, and
that with the **default route-target disabled** a peer that has not advertised
any RT membership receives no VPN routes at all.

## Testbed type

*   `TESTBED_DUT_ATE_2LINKS`

## Topology

```mermaid
graph LR;
  ate1["ATE Port 1 (2001:db8:0:1::2/64)"] -- "iBGP (Peer 1)" --> dut1["DUT Port 1 (2001:db8:0:1::1/64)"]
  ate2["ATE Port 2 (2001:db8:0:2::2/64)"] -- "iBGP (Peer 2)" --> dut2["DUT Port 2 (2001:db8:0:2::1/64)"]
```

*   Connect ATE Port 1 to DUT Port 1.
*   Connect ATE Port 2 to DUT Port 2.
*   ATE will simulate iBGP peers negotiating different capabilities.
*   All iBGP sessions are transported over IPv6 using explicit point-to-point links.

## Procedure

### Configuration

1.  Configure DUT with iBGP peer group (e.g., `RM_DYNAMIC_PEERS`) with `auth-password` and `send-community BOTH` (standard and extended; RT and SoO are extended communities).
2.  Enable `RT_MEMBERSHIP`, `L3VPN_IPV4_UNICAST`, and `L3VPN_IPV6_UNICAST` address families for the peer group; enable `add-paths send` on both L3VPN address families.
3.  Disable the implicit default route-target on the peer group (vendor: `no neighbor <peer-group> default-route-target`): the DUT must **not** assume interest in all VPN routes for peers that have not advertised RT membership.
4.  Configure two VRFs on DUT:
    *   **VRF_100**: Export/Import RT `64496:100`, inject prefixes `198.51.100.10/32` and `2001:db8:2::10/128`.
    *   **VRF_200**: Export/Import RT `64496:200`, inject prefixes `198.51.100.20/32` and `2001:db8:2::20/128`.
5.  Setup ATE to simulate multiple iBGP peers (Simulated Peers generating VPN paths), some negotiating RT Membership and some not.

> [!NOTE]
> In the EdgeBGP profile the iBGP peers are dynamic (listen-range) peers, see
> RT-1.103. Peer provisioning method does not change the RTC behaviour tested
> here.

### Tests

#### RT-1.106.1 - Default RT Membership Behavior (Outbound) with Default Route-Target Disabled

*   **Step 1**: Do NOT negotiate RT Membership capability from ATE Peer 1. Ensure BGP session is ESTABLISHED using `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state`.
*   **Step 2**: With the default route-target disabled on the peer group, verify that the DUT does **NOT** generate a default RT membership route on behalf of Peer 1 and therefore treats Peer 1 as interested in **no** VPN routes.
*   **Step 3**: Verify Peer 1 receives **none** of the exported VPN routes (`198.51.100.10/32`, `2001:db8:2::10/128`, `198.51.100.20/32`, `2001:db8:2::20/128`). Verify using `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/sent` that `0` prefixes are sent to Peer 1 on `L3VPN_IPV4_UNICAST` and `L3VPN_IPV6_UNICAST`.
*   **Step 4 (Reference behaviour)**: Re-enable the default route-target on the peer group and verify Peer 1 now receives all four exported VPN routes (standard RFC 4684 default behaviour). Restore the disabled state before the next sub-test.

#### RT-1.106.2 - Constrained Route Distribution (Outbound Filtering)

*   **Step 1**: Enable RT Membership negotiation on ATE Peer 2. Ensure BGP session is ESTABLISHED using `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state`. Verify on DUT that the RT Membership capability is active for Peer 2 using `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/active`.
*   **Step 2**: ATE Peer 2 advertises Interest in specific Route Target `64496:100` ONLY.
*   **Step 3**: Verify DUT ONLY advertises VPN routes matching this Route Target to Peer 2 (Verify `198.51.100.10/32` and `2001:db8:2::10/128` are received). Verify using `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/sent` that the expected number of prefixes are sent.
*   **Step 4**: Verify DUT does NOT advertise routes with other Route Targets to Peer 2 (Verify `198.51.100.20/32` and `2001:db8:2::20/128` are NOT received). Verify using `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/sent` that the unexpected prefixes are NOT sent.

#### RT-1.106.3 - DUT RTC Origination (Inbound Constraints)

*   **Step 1**: Ensure DUT has `VRF_100` (Imports `64496:100`) and `VRF_200` (Imports `64496:200`) configured and active.
*   **Step 2**: Verify on ATE that it receives `RT_MEMBERSHIP` advertisements from DUT for `64496:100` and `64496:200`.
*   **Step 3**: Verify on ATE that it does **NOT** receive `RT_MEMBERSHIP` advertisements from DUT for other unconfigured Route Targets (e.g., `64496:300`).
*   **Step 4**: Verify using `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received` that DUT only receives prefixes matching the advertised RTCs (assuming ATE respects the RTC advertisements).

#### RT-1.106.4 - Add-Paths over RT-Constrained L3VPN Sessions

*   **Step 1**: Verify `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/state/send` is `true` for `L3VPN_IPV4_UNICAST` and `L3VPN_IPV6_UNICAST` on ATE Peer 2 (RTC-interested in `64496:100`), and that ATE Peer 2 negotiated add-path receive.
*   **Step 2**: Inject two paths for the same prefix (`198.51.100.10/32`) into `VRF_100` (e.g., via two eBGP/ATE sources or two connected next-hops).
*   **Step 3**: Verify ATE Peer 2 receives **both** paths for `198.51.100.10/32` with distinct path-ids, each carrying RT `64496:100`.
*   **Step 4**: Verify ATE Peer 1 (no RT membership advertised, default route-target disabled) still receives no paths.
*   **Step 5**: Verify a standard community and an extended community attached to the VRF route are both present on the paths received by ATE Peer 2 (`send-community BOTH`).

## Canonical OC

> [!NOTE]
> `RT_MEMBERSHIP` is not yet a standard OpenConfig AFI-SAFI type enum value. It is enabled in the test procedure but omitted from this Canonical OC sample to prevent validation failures against standard schemas. See TODO below.

```json
{
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "DEFAULT"
        },
        "name": "DEFAULT",
        "interfaces": {
          "interface": [
            {
              "config": {
                "id": "Port1"
              },
              "id": "Port1"
            },
            {
              "config": {
                "id": "Port2"
              },
              "id": "Port2"
            }
          ]
        },
        "protocols": {
          "protocol": [
            {
              "bgp": {
                "global": {
                  "config": {
                    "as": 64496,
                    "router-id": "192.0.2.1"
                  }
                },
                "peer-groups": {
                  "peer-group": [
                    {
                      "config": {
                        "peer-group-name": "RM_DYNAMIC_PEERS",
                        "peer-as": 64496,
                        "send-community": "BOTH",
                        "auth-password": "ibgp_internal_secret"
                      },
                      "peer-group-name": "RM_DYNAMIC_PEERS",
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:L3VPN_IPV4_UNICAST",
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:L3VPN_IPV4_UNICAST",
                              "enabled": true
                            },
                            "add-paths": {
                              "config": {
                                "send": true
                              }
                            }
                          },
                          {
                            "afi-safi-name": "openconfig-bgp-types:L3VPN_IPV6_UNICAST",
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:L3VPN_IPV6_UNICAST",
                              "enabled": true
                            },
                            "add-paths": {
                              "config": {
                                "send": true
                              }
                            }
                          }
                        ]
                      }
                    }
                  ]
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "config": {
                        "neighbor-address": "2001:db8:0:1::2",
                        "peer-group": "RM_DYNAMIC_PEERS"
                      },
                      "neighbor-address": "2001:db8:0:1::2"
                    },
                    {
                      "config": {
                        "neighbor-address": "2001:db8:0:2::2",
                        "peer-group": "RM_DYNAMIC_PEERS"
                      },
                      "neighbor-address": "2001:db8:0:2::2"
                    }
                  ]
                }
              },
              "config": {
                "identifier": "BGP",
                "name": "BGP"
              },
              "identifier": "BGP",
              "name": "BGP"
            }
          ]
        }
      },
      {
        "config": {
          "name": "VRF_100",
          "type": "openconfig-network-instance-types:L3VRF",
          "route-distinguisher": "64496:100"
        },
        "name": "VRF_100",
        "inter-instance-policies": {
          "import-export-policy": {
            "config": {
              "import-route-target": [
                "64496:100"
              ],
              "export-route-target": [
                "64496:100"
              ]
            }
          }
        },
        "interfaces": {
          "interface": [
            {
              "config": {
                "id": "Loopback100"
              },
              "id": "Loopback100"
            }
          ]
        },
        "protocols": {
          "protocol": [
            {
              "bgp": {
                "global": {
                  "config": {
                    "as": 64496,
                    "router-id": "192.0.2.100"
                  }
                }
              },
              "config": {
                "identifier": "BGP",
                "name": "BGP"
              },
              "identifier": "BGP",
              "name": "BGP"
            }
          ]
        }
      },
      {
        "config": {
          "name": "VRF_200",
          "type": "openconfig-network-instance-types:L3VRF",
          "route-distinguisher": "64496:200"
        },
        "name": "VRF_200",
        "inter-instance-policies": {
          "import-export-policy": {
            "config": {
              "import-route-target": [
                "64496:200"
              ],
              "export-route-target": [
                "64496:200"
              ]
            }
          }
        },
        "interfaces": {
          "interface": [
            {
              "config": {
                "id": "Loopback200"
              },
              "id": "Loopback200"
            }
          ]
        },
        "protocols": {
          "protocol": [
            {
              "bgp": {
                "global": {
                  "config": {
                    "as": 64496,
                    "router-id": "192.0.2.200"
                  }
                }
              },
              "config": {
                "identifier": "BGP",
                "name": "BGP"
              },
              "identifier": "BGP",
              "name": "BGP"
            }
          ]
        }
      }
    ]
  },
  "interfaces": {
    "interface": [
      {
        "config": {
          "name": "Port1",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "Port1",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:0:1::1",
                        "prefix-length": 64
                      },
                      "ip": "2001:db8:0:1::1"
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
          "name": "Port2",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "Port2",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:0:2::1",
                        "prefix-length": 64
                      },
                      "ip": "2001:db8:0:2::1"
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
          "name": "Loopback100",
          "type": "iana-if-type:softwareLoopback"
        },
        "name": "Loopback100",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "198.51.100.10",
                        "prefix-length": 32
                      },
                      "ip": "198.51.100.10"
                    }
                  ]
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:2::10",
                        "prefix-length": 128
                      },
                      "ip": "2001:db8:2::10"
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
          "name": "Loopback200",
          "type": "iana-if-type:softwareLoopback"
        },
        "name": "Loopback200",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "198.51.100.20",
                        "prefix-length": 32
                      },
                      "ip": "198.51.100.20"
                    }
                  ]
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:2::20",
                        "prefix-length": 128
                      },
                      "ip": "2001:db8:2::20"
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

## OpenConfig Path and RPC Coverage

```yaml
paths:
  # Network Instance / VRF Config
  /network-instances/network-instance/config/name:
  /network-instances/network-instance/state/name:
  /network-instances/network-instance/config/type:
  /network-instances/network-instance/state/type:
  /network-instances/network-instance/config/route-distinguisher:
  /network-instances/network-instance/state/route-distinguisher:
  /network-instances/network-instance/inter-instance-policies/import-export-policy/config/import-route-target:
  /network-instances/network-instance/inter-instance-policies/import-export-policy/state/import-route-target:
  /network-instances/network-instance/inter-instance-policies/import-export-policy/config/export-route-target:
  /network-instances/network-instance/inter-instance-policies/import-export-policy/state/export-route-target:
  /network-instances/network-instance/interfaces/interface/config/id:
  /network-instances/network-instance/interfaces/interface/state/id:

  # Interface Config (Loopbacks/Prefix Injection)
  /interfaces/interface/config/name:
  /interfaces/interface/config/type:
  /interfaces/interface/subinterfaces/subinterface/config/index:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:

  # BGP AFI-SAFI Enablement
  /network-instances/network-instance/protocols/protocol/bgp/global/config/as:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/state/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/afi-safi-name:

  # BGP Neighbor State
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/active:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/sent:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/installed:

  # Add-Paths and Communities (New in this version)
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/config/send:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/state/send:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/send-community:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/send-community:

  # MD5 Auth (New in this version)
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/auth-password:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-as:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true

```

## TODO

*   Request addition of `RT_MEMBERSHIP` (or `RTC`) to the standard OpenConfig `AFI_SAFI_TYPE` identities in `openconfig-bgp-types.yang` to support RFC 4684 validation natively.
*   There is no OpenConfig leaf to disable the implicit default route-target for peers without RT membership (vendor CLI only, e.g. `no neighbor <peer-group> default-route-target`); request a model addition.

## Required DUT platform

*   FFF (Fixed Form Factor) or MFF supporting RFC 4684.
