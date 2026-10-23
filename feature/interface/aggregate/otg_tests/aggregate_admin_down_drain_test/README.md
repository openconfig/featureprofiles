# RT-5.18: Aggregate Interface (LAG) Drain via Admin Down

## Summary

* Verify that administratively disabling a LAG interface (e.g., `Port-Channel1`) via gNMI brings down associated routing protocols (eBGP and IS-IS), stops all traffic across the aggregate interface while keeping unrelated interfaces stable, and cleanly restores protocol sessions and forwarding when re-enabled.

## Testbed type

* [`TESTBED_DUT_ATE_4LINKS`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Topology

```text
                        +-------------------+
  ATE Port-1 -----\     | DUT Port-1        |
  ATE Port-2 ------+====| DUT Port-2   DUT  |  LAG: Port-Channel1 (3 Member Links)
  ATE Port-3 -----/     | DUT Port-3        |  IPv4: 192.0.2.0/30, IPv6: 2001:db8::/126
                        |                   |
  ATE Port-4 -----------| DUT Port-4        |  Standalone Transit Link
                        +-------------------+  IPv4: 192.0.2.4/30, IPv6: 2001:db8::4/126
```

## Procedure

### Test environment setup

* Connect ATE `Port-1` to DUT `Port-1`, ATE `Port-2` to DUT `Port-2`, ATE `Port-3` to DUT `Port-3`, and ATE `Port-4` to DUT `Port-4`.
* Configure the 3-member LAG interface (`Port-Channel1` - Interface Under Test):
  * Configure aggregate interface `Port-Channel1` on the DUT with `/interfaces/interface[name=Port-Channel1]/config/type` set to `iana-if-type:ieee8023adLag`, `/interfaces/interface[name=Port-Channel1]/aggregation/config/lag-type` set to `LACP`, `/interfaces/interface[name=Port-Channel1]/aggregation/config/min-links` set to `1`, and `/lacp/interfaces/interface[name=Port-Channel1]/config/lacp-mode` set to `ACTIVE`.
  * Assign DUT `Port-1`, `Port-2`, and `Port-3` to `Port-Channel1` by setting `/interfaces/interface[name=<DUT Port-1..Port-3>]/ethernet/config/aggregate-id` to `Port-Channel1`.
  * Configure a matching 3-port LACP (`ACTIVE`) LAG on ATE (`ATE Port-1`, `Port-2`, and `Port-3`).
  * Configure dual-stack Layer 3 addressing on DUT `Port-Channel1` subinterface `0` in the `DEFAULT` network instance:
    * DUT `Port-Channel1`: IPv4 `192.0.2.1/30` (`/interfaces/interface[name=Port-Channel1]/subinterfaces/subinterface[index=0]/ipv4/addresses/address[ip=192.0.2.1]/config/ip` = `192.0.2.1` and `.../config/prefix-length` = `30`) and IPv6 `2001:db8::1/126` (`/interfaces/interface[name=Port-Channel1]/subinterfaces/subinterface[index=0]/ipv6/addresses/address[ip=2001:db8::1]/config/ip` = `2001:db8::1` and `.../config/prefix-length` = `126`).
    * ATE `Port-Channel1`: IPv4 `192.0.2.2/30` and IPv6 `2001:db8::2/126`.
* Configure the standalone Layer 3 transit interface (`Port-4`):
  * Configure dual-stack Layer 3 addressing on DUT `Port-4` subinterface `0` in the `DEFAULT` network instance:
    * DUT `Port-4`: IPv4 `192.0.2.5/30` (`.../ipv4/addresses/address[ip=192.0.2.5]/config/ip` = `192.0.2.5`, `prefix-length` = `30`) and IPv6 `2001:db8::5/126` (`.../ipv6/addresses/address[ip=2001:db8::5]/config/ip` = `2001:db8::5`, `prefix-length` = `126`).
    * ATE `Port-4`: IPv4 `192.0.2.6/30` and IPv6 `2001:db8::6/126`.
* Establish IS-IS (`DEFAULT` network instance, protocol identifier `ISIS`, name `ISIS`, `LEVEL_2`, `WIDE_METRIC`, Area ID `49.0001`):
  * Configure System IDs: DUT `1920.0000.2001` (NET `49.0001.1920.0000.2001.00`), ATE `Port-Channel1` `1920.0000.2002` (NET `49.0001.1920.0000.2002.00`), and ATE `Port-4` `1920.0000.2003` (NET `49.0001.1920.0000.2003.00`).
  * Enable IS-IS (`POINT_TO_POINT`, Level 2 metric `10`, `IPV4_UNICAST` and `IPV6_UNICAST`) on both `Port-Channel1` and `Port-4`.
  * Advertise IS-IS reachability prefixes from ATE `Port-Channel1`: `198.18.0.0/24` (IPv4, next-hop `192.0.2.2`) and `2001:db8:2000::/64` (IPv6, next-hop `2001:db8::2`).
* Establish eBGP (`DEFAULT` network instance, protocol identifier `BGP`, name `BGP`, DUT local AS `64500`) with 4 neighbors (`IPV4_UNICAST` and `IPV6_UNICAST`):
  * Over `Port-Channel1` (ATE AS `64501`): IPv4 neighbor `192.0.2.2` (local-address `192.0.2.1`) and IPv6 neighbor `2001:db8::2` (local-address `2001:db8::1`).
  * Over `Port-4` (ATE AS `64502`): IPv4 neighbor `192.0.2.6` (local-address `192.0.2.5`) and IPv6 neighbor `2001:db8::6` (local-address `2001:db8::5`).
* Advertise deterministic, non-overlapping route pools over eBGP:
  * From ATE `Port-Channel1` (`192.0.2.2` and `2001:db8::2`, AS `64501`), advertise **10,000 IPv4 `/24` prefixes** and **10,000 IPv6 `/64` prefixes**:
    * **IPv4 (`10,000` prefixes):** `100.64.0.0/24` through `100.103.15.0/24` (`100.<64 + floor(i/256)>.<i mod 256>.0/24` for $i \in [0, 9999]$, next-hop `192.0.2.2`).
    * **IPv6 (`10,000` prefixes):** `2001:db8:1000::/64` through `2001:db8:1000:270f::/64` (`2001:db8:1000:<hex(i)>::/64` for $i \in [0, 9999]$, next-hop `2001:db8::2`).
  * From ATE `Port-4` (`192.0.2.6` and `2001:db8::6`, AS `64502`), advertise transit prefixes for reverse traffic routing:
    * **IPv4:** `198.51.100.0/24` (next-hop `192.0.2.6`).
    * **IPv6:** `2001:db8:3000::/64` (next-hop `2001:db8::6`).

### RT-5.18.1 - Admin Down Drain - Disable

* Step 1 - Verify via gNMI `Watch` (`Await`) that all 4 eBGP sessions (`192.0.2.2`, `2001:db8::2`, `192.0.2.6`, `2001:db8::6`) are `ESTABLISHED` and IS-IS Level-2 adjacencies on `Port-Channel1` and `Port-4` are `UP`. Verify that BGP `installed` prefixes on `Port-Channel1` reach `10000` for IPv4 (`192.0.2.2`) and `10000` for IPv6 (`2001:db8::2`) via gNMI telemetry:
  * `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=192.0.2.2]/afi-safis/afi-safi[afi-safi-name=IPV4_UNICAST]/state/prefixes/installed`
  * `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=2001:db8::2]/afi-safis/afi-safi[afi-safi-name=IPV6_UNICAST]/state/prefixes/installed`
* Step 2 - Ensure IPv4 ARP and IPv6 ND resolution completes (`WaitForARP`) on all ATE interfaces, then start continuous bidirectional IPv4 and IPv6 traffic flows between ATE `Port-4` and ATE `Port-Channel1` (`Port-1..Port-3`) at a non-congesting rate (`10%` of a single member link's line rate, so a single active member link can carry 100% of the traffic without congestion):
  * **Forward IPv4 Flow (`Port-4` -> `Port-Channel1`):** Source IP `198.51.100.1` to all 10,000 BGP destination IPs `100.64.0.1` through `100.103.15.1` (plus IS-IS destination IP `198.18.0.1`).
  * **Reverse IPv4 Flow (`Port-Channel1` -> `Port-4`):** Source IPs `100.64.0.1` through `100.103.15.1` to destination IP `198.51.100.1`.
  * **Forward IPv6 Flow (`Port-4` -> `Port-Channel1`):** Source IP `2001:db8:3000::1` to all 10,000 BGP destination IPs `2001:db8:1000:0::1` through `2001:db8:1000:270f::1` (plus IS-IS destination IP `2001:db8:2000::1`).
  * **Reverse IPv6 Flow (`Port-Channel1` -> `Port-4`):** Source IPs `2001:db8:1000:0::1` through `2001:db8:1000:270f::1` to destination IP `2001:db8:3000::1`.
* Step 3 - Verify `0%` steady-state traffic loss across all flows (using gNMI counter `Watch`, with no static sleep wait time).
* Step 4 - Administratively disable the LAG interface by setting `/interfaces/interface[name=Port-Channel1]/config/enabled` to `false` using `gNMI.Set` with `REPLACE` option.

#### Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "name": "Port-Channel1",
        "config": {
          "enabled": false
        }
      }
    ]
  }
}
```

* Step 5 - Verify via gNMI Subscribe (`Watch`) that `/interfaces/interface[name=Port-Channel1]/state/admin-status` is `DOWN` and `/interfaces/interface[name=Port-Channel1]/state/oper-status` is `DOWN`.
* Step 6 - Verify that the associated BGP sessions on `Port-Channel1` (`192.0.2.2` and `2001:db8::2`) transition to `ACTIVE` or `IDLE` (not `ESTABLISHED`) via `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=<192.0.2.2|2001:db8::2>]/state/session-state`, while the BGP sessions on `Port-4` (`192.0.2.6` and `2001:db8::6`) remain `ESTABLISHED`.
* Step 7 - Verify that the IS-IS adjacency on `Port-Channel1` transitions to `DOWN` (or is removed from the active adjacency table) via `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=ISIS]/isis/interfaces/interface[interface-id=Port-Channel1]/levels/level[level-number=2]/adjacencies/adjacency/state/adjacency-state`, while the IS-IS adjacency on `Port-4` remains `UP`.
* Step 8 - Verify that all bidirectional traffic traversing `Port-Channel1` stops completely (steady-state traffic loss is `100%`).

### RT-5.18.2 - Admin Down Drain - Re-enable

* Step 1 - Re-enable the LAG interface by setting `/interfaces/interface[name=Port-Channel1]/config/enabled` to `true` using `gNMI.Set` with `REPLACE` option.
* Step 2 - Verify via gNMI Subscribe (`Watch`) that `/interfaces/interface[name=Port-Channel1]/state/admin-status` is `UP` and `/interfaces/interface[name=Port-Channel1]/state/oper-status` is `UP`.
* Step 3 - Verify that the BGP sessions over `Port-Channel1` (`192.0.2.2` and `2001:db8::2`) re-establish by checking `session-state` is `ESTABLISHED` at the paths from RT-5.18.1 Step 6.
* Step 4 - Verify that the IS-IS Level-2 adjacency on `Port-Channel1` re-establishes by checking `adjacency-state` is `UP` at the path from RT-5.18.1 Step 7.
* Step 5 - Verify via gNMI `Watch` that the BGP `installed` prefix count returns to `10000` for both `IPV4_UNICAST` and `IPV6_UNICAST` at the paths from RT-5.18.1 Step 1.
* Step 6 - Once gNMI confirms control plane and AFT convergence, verify that steady-state bidirectional traffic loss returns to `0%`.

### RT-5.18.3 - Negative Test - Admin Down on Individual Member Links

* Step 1 - Verify BGP `installed` prefixes are at `10000` for IPv4 (`192.0.2.2`) and `10000` for IPv6 (`2001:db8::2`) via gNMI telemetry at the paths from RT-5.18.1 Step 1.
* Step 2 - Send continuous bidirectional IPv4 and IPv6 traffic flows between ATE `Port-4` and ATE `Port-Channel1` (`Port-1..Port-3`) via the advertised prefixes at `10%` of a single member link's line rate.
* Step 3 - Administratively disable 2 out of the 3 individual member links of `Port-Channel1` (DUT `Port-1` and DUT `Port-2`, leaving DUT `Port-3` active) by setting `/interfaces/interface[name=<DUT Port-1|Port-2>]/config/enabled` to `false` using `gNMI.Set` with `REPLACE` option.
* Step 4 - Verify via gNMI that the disabled member links (`Port-1` and `Port-2`) transition to `oper-status = DOWN`, while the aggregate interface `/interfaces/interface[name=Port-Channel1]/state/oper-status` remains `UP` (via active member `Port-3`).
* Step 5 - Verify that the eBGP (`192.0.2.2`, `2001:db8::2`) and IS-IS sessions over `Port-Channel1` remain `ESTABLISHED` and `UP` at their respective paths without flapping.
* Step 6 - Verify that bidirectional traffic continues to flow (`0%` steady-state loss) over the remaining active member link (`DUT Port-3`).
* Step 7 - Re-enable the disabled member links (`DUT Port-1` and `DUT Port-2`) by setting `/interfaces/interface[name=<DUT Port-1|Port-2>]/config/enabled` to `true` using `gNMI.Set` with `REPLACE` option, verify their `oper-status` returns to `UP`, and confirm traffic remains at `0%` loss across all 3 member links.

### RT-5.18.4 - Negative Test - Admin Down on Already Disabled LAG

* Step 1 - Administratively disable the LAG interface by setting `/interfaces/interface[name=Port-Channel1]/config/enabled` to `false` using `gNMI.Set` with `REPLACE` option.
* Step 2 - Verify via gNMI that `/interfaces/interface[name=Port-Channel1]/state/admin-status` is `DOWN`, `/interfaces/interface[name=Port-Channel1]/state/oper-status` is `DOWN`, and traffic loss across `Port-Channel1` is `100%`.
* Step 3 - Send a second idempotent `gNMI.Set` (`REPLACE`) setting `/interfaces/interface[name=Port-Channel1]/config/enabled` to `false` on the already disabled `Port-Channel1` interface.
* Step 4 - Verify that the DUT accepts the redundant `gNMI.Set` configuration without returning any RPC errors.
* Step 5 - Verify that `/interfaces/interface[name=Port-Channel1]/state/admin-status` and `/interfaces/interface[name=Port-Channel1]/state/oper-status` remain `DOWN` and traffic remains at `100%` loss.
* Step 6 - Restore `Port-Channel1` by setting `/interfaces/interface[name=Port-Channel1]/config/enabled` to `true` using `gNMI.Set` with `REPLACE` option, and verify via gNMI that `admin-status` and `oper-status` return to `UP`, eBGP (`192.0.2.2`, `2001:db8::2`) and IS-IS sessions re-establish to `ESTABLISHED`/`UP`, BGP `installed` prefixes return to `10000` (IPv4 and IPv6), and steady-state traffic loss returns to `0%`.

### RT-5.18.5 - Uplink LAG Member Retention and Inband BGP Reachability in Non-Default VRF (`B2_CORE`) Across Drain States

This scenario validates that uplink aggregate interfaces carrying an inband
management and control-plane VRF (`B2_CORE`) over a single-tagged 802.1Q VLAN
subinterface retain their LACP member port bundle assignments (`aggregate-id`)
and forwarding viability (`forwarding-viable: true`) across initial and drained
lifecycle states, preserving continuous dual-stack eBGP reachability and
bidirectional data-plane forwarding.

* Step 1 - Configure a non-default Layer 3 VRF network instance named `B2_CORE`
  and bind `Port-Channel1` subinterface `100` (`Port-Channel1.100`) and `Port-4`
  subinterface `100` (`Port-4.100`) to `B2_CORE`:
  * `/network-instances/network-instance[name=B2_CORE]/config/name` = `B2_CORE`
  * `/network-instances/network-instance[name=B2_CORE]/config/type` = `L3VRF`
  * `/network-instances/network-instance[name=B2_CORE]/interfaces/interface[id=Port-Channel1.100]/config/interface` = `Port-Channel1`
  * `/network-instances/network-instance[name=B2_CORE]/interfaces/interface[id=Port-Channel1.100]/config/subinterface` = `100`
  * `/network-instances/network-instance[name=B2_CORE]/interfaces/interface[id=Port-4.100]/config/interface` = `Port-4`
  * `/network-instances/network-instance[name=B2_CORE]/interfaces/interface[id=Port-4.100]/config/subinterface` = `100`
* Step 2 - Configure single-tagged VLAN `100` and dual-stack Layer 3 addressing
  on `Port-Channel1` subinterface `100` and `Port-4` subinterface `100` on both
  the DUT and ATE:
  * On DUT `Port-Channel1` subinterface `100`:
    * `/interfaces/interface[name=Port-Channel1]/subinterfaces/subinterface[index=100]/config/index` = `100`
    * `/interfaces/interface[name=Port-Channel1]/subinterfaces/subinterface[index=100]/vlan/match/single-tagged/config/vlan-id` = `100`
    * IPv4 address `192.0.2.9/30`:
      * `/interfaces/interface[name=Port-Channel1]/subinterfaces/subinterface[index=100]/ipv4/addresses/address[ip=192.0.2.9]/config/ip` = `192.0.2.9`
      * `/interfaces/interface[name=Port-Channel1]/subinterfaces/subinterface[index=100]/ipv4/addresses/address[ip=192.0.2.9]/config/prefix-length` = `30`
    * IPv6 address `2001:db8::9/126`:
      * `/interfaces/interface[name=Port-Channel1]/subinterfaces/subinterface[index=100]/ipv6/addresses/address[ip=2001:db8::9]/config/ip` = `2001:db8::9`
      * `/interfaces/interface[name=Port-Channel1]/subinterfaces/subinterface[index=100]/ipv6/addresses/address[ip=2001:db8::9]/config/prefix-length` = `126`
    * Configure matching VLAN `100` subinterface on ATE `Port-Channel1` with
      IPv4 `192.0.2.10/30` and IPv6 `2001:db8::a/126`.
  * On DUT `Port-4` subinterface `100`:
    * `/interfaces/interface[name=Port-4]/subinterfaces/subinterface[index=100]/config/index` = `100`
    * `/interfaces/interface[name=Port-4]/subinterfaces/subinterface[index=100]/vlan/match/single-tagged/config/vlan-id` = `100`
    * IPv4 address `192.0.2.13/30`:
      * `/interfaces/interface[name=Port-4]/subinterfaces/subinterface[index=100]/ipv4/addresses/address[ip=192.0.2.13]/config/ip` = `192.0.2.13`
      * `/interfaces/interface[name=Port-4]/subinterfaces/subinterface[index=100]/ipv4/addresses/address[ip=192.0.2.13]/config/prefix-length` = `30`
    * IPv6 address `2001:db8::d/126`:
      * `/interfaces/interface[name=Port-4]/subinterfaces/subinterface[index=100]/ipv6/addresses/address[ip=2001:db8::d]/config/ip` = `2001:db8::d`
      * `/interfaces/interface[name=Port-4]/subinterfaces/subinterface[index=100]/ipv6/addresses/address[ip=2001:db8::d]/config/prefix-length` = `126`
    * Configure matching VLAN `100` subinterface on ATE `Port-4` with IPv4
      `192.0.2.14/30` and IPv6 `2001:db8::e/126`.
* Step 3 - Configure uplink LAG `Port-Channel1` and ensure all 3 uplink member
  ports (`Port-1`, `Port-2`, and `Port-3`) retain their LAG membership and
  forwarding viability in the drained/initial uplink state by pushing via
  `gNMI.Set` (`REPLACE` / `UNION_REPLACE`):
  * `/interfaces/interface[name=Port-Channel1]/aggregation/config/lag-type` = `LACP`
  * `/lacp/interfaces/interface[name=Port-Channel1]/config/lacp-mode` = `ACTIVE`
  * `/lacp/interfaces/interface[name=Port-Channel1]/config/interval` = `FAST`
  * `/interfaces/interface[name=<Port-1..Port-3>]/config/enabled` = `true`
  * `/interfaces/interface[name=<Port-1..Port-3>]/ethernet/config/aggregate-id` = `Port-Channel1`
  * `/interfaces/interface[name=<Port-1..Port-3>]/config/forwarding-viable` = `true`
* Step 4 - Configure dual-stack eBGP in the `B2_CORE` network instance:
  * Set `/network-instances/network-instance[name=B2_CORE]/protocols/protocol[identifier=BGP][name=BGP]/bgp/global/config/as` = `64500`.
  * Over `Port-Channel1.100` (ATE AS `64501`):
    * Configure IPv4 neighbor `192.0.2.10` (advertising prefix `203.0.113.0/24`)
      and IPv6 neighbor `2001:db8::a` (advertising prefix `2001:db8:4000::/64`).
  * Over `Port-4.100` (ATE AS `64502`):
    * Configure IPv4 neighbor `192.0.2.14` (advertising prefix `198.51.100.0/24`)
      and IPv6 neighbor `2001:db8::e` (advertising prefix `2001:db8:3000::/64`).

#### Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "name": "Port-Channel1",
        "config": {
          "name": "Port-Channel1",
          "type": "iana-if-type:ieee8023adLag",
          "enabled": true
        },
        "aggregation": {
          "config": {
            "lag-type": "LACP",
            "min-links": 1
          }
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 100,
              "config": {
                "index": 100
              },
              "vlan": {
                "match": {
                  "single-tagged": {
                    "config": {
                      "vlan-id": 100
                    }
                  }
                }
              },
              "ipv4": {
                "config": {
                  "enabled": true
                },
                "addresses": {
                  "address": [
                    {
                      "ip": "192.0.2.9",
                      "config": {
                        "ip": "192.0.2.9",
                        "prefix-length": 30
                      }
                    }
                  ]
                }
              },
              "ipv6": {
                "config": {
                  "enabled": true
                },
                "addresses": {
                  "address": [
                    {
                      "ip": "2001:db8::9",
                      "config": {
                        "ip": "2001:db8::9",
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
        "name": "Port-1",
        "config": {
          "name": "Port-1",
          "type": "iana-if-type:ethernetCsmacd",
          "enabled": true,
          "forwarding-viable": true
        },
        "ethernet": {
          "config": {
            "aggregate-id": "Port-Channel1"
          }
        }
      },
      {
        "name": "Port-2",
        "config": {
          "name": "Port-2",
          "type": "iana-if-type:ethernetCsmacd",
          "enabled": true,
          "forwarding-viable": true
        },
        "ethernet": {
          "config": {
            "aggregate-id": "Port-Channel1"
          }
        }
      },
      {
        "name": "Port-3",
        "config": {
          "name": "Port-3",
          "type": "iana-if-type:ethernetCsmacd",
          "enabled": true,
          "forwarding-viable": true
        },
        "ethernet": {
          "config": {
            "aggregate-id": "Port-Channel1"
          }
        }
      },
      {
        "name": "Port-4",
        "config": {
          "name": "Port-4",
          "type": "iana-if-type:ethernetCsmacd",
          "enabled": true
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 100,
              "config": {
                "index": 100
              },
              "vlan": {
                "match": {
                  "single-tagged": {
                    "config": {
                      "vlan-id": 100
                    }
                  }
                }
              },
              "ipv4": {
                "config": {
                  "enabled": true
                },
                "addresses": {
                  "address": [
                    {
                      "ip": "192.0.2.13",
                      "config": {
                        "ip": "192.0.2.13",
                        "prefix-length": 30
                      }
                    }
                  ]
                }
              },
              "ipv6": {
                "config": {
                  "enabled": true
                },
                "addresses": {
                  "address": [
                    {
                      "ip": "2001:db8::d",
                      "config": {
                        "ip": "2001:db8::d",
                        "prefix-length": 126
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
  "lacp": {
    "interfaces": {
      "interface": [
        {
          "name": "Port-Channel1",
          "config": {
            "name": "Port-Channel1",
            "lacp-mode": "ACTIVE",
            "interval": "FAST"
          }
        }
      ]
    }
  },
  "network-instances": {
    "network-instance": [
      {
        "name": "B2_CORE",
        "config": {
          "name": "B2_CORE",
          "type": "L3VRF"
        },
        "interfaces": {
          "interface": [
            {
              "id": "Port-Channel1.100",
              "config": {
                "id": "Port-Channel1.100",
                "interface": "Port-Channel1",
                "subinterface": 100
              }
            },
            {
              "id": "Port-4.100",
              "config": {
                "id": "Port-4.100",
                "interface": "Port-4",
                "subinterface": 100
              }
            }
          ]
        }
      }
    ]
  }
}
```

* Step 5 - Verify via gNMI Subscribe (`Watch` with `Await`) that all 3 uplink
  member ports (`Port-1`, `Port-2`, and `Port-3`) remain bound to
  `Port-Channel1` and actively participating in LACP:
  * `/interfaces/interface[name=<Port-1..Port-3>]/ethernet/state/aggregate-id` is `Port-Channel1`.
  * `/interfaces/interface[name=<Port-1..Port-3>]/state/forwarding-viable` is `true`.
  * `/interfaces/interface[name=<Port-1..Port-3>]/state/oper-status` is `UP` and `/interfaces/interface[name=Port-Channel1]/state/oper-status` is `UP`.
  * `/lacp/interfaces/interface[name=Port-Channel1]/members/member[interface=<Port-1..Port-3>]/state/activity` is `ACTIVE`.
  * `/lacp/interfaces/interface[name=Port-Channel1]/members/member[interface=<Port-1..Port-3>]/state/aggregatable` is `true`.
  * `/lacp/interfaces/interface[name=Port-Channel1]/members/member[interface=<Port-1..Port-3>]/state/collecting` is `true`.
  * `/lacp/interfaces/interface[name=Port-Channel1]/members/member[interface=<Port-1..Port-3>]/state/distributing` is `true`.
  * `/lacp/interfaces/interface[name=Port-Channel1]/members/member[interface=<Port-1..Port-3>]/state/synchronization` is `IN_SYNC`.
* Step 6 - Verify via gNMI `Watch` (`Await`) that all 4 eBGP sessions in the
  `B2_CORE` network instance (`192.0.2.10`, `2001:db8::a`, `192.0.2.14`, and
  `2001:db8::e`) reach `ESTABLISHED` and that `installed` prefixes reach `1`
  for both `IPV4_UNICAST` and `IPV6_UNICAST`:
  * `/network-instances/network-instance[name=B2_CORE]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=<neighbor_ip>]/state/session-state` is `ESTABLISHED`.
  * `/network-instances/network-instance[name=B2_CORE]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=192.0.2.10]/afi-safis/afi-safi[afi-safi-name=IPV4_UNICAST]/state/prefixes/installed` is `1`.
  * `/network-instances/network-instance[name=B2_CORE]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=2001:db8::a]/afi-safis/afi-safi[afi-safi-name=IPV6_UNICAST]/state/prefixes/installed` is `1`.
* Step 7 - Confirm IPv4 ARP and IPv6 ND neighbor resolution (`WaitForARP`) on
  all ATE VLAN `100` interfaces, then start bidirectional IPv4 and IPv6 traffic
  flows in the `B2_CORE` VRF between ATE `Port-4` (VLAN `100`) and ATE
  `Port-Channel1` (VLAN `100`) at `10%` of a single member link's line rate:
  * **Forward IPv4 Flow (`Port-4.100` -> `Port-Channel1.100`):** Source IP
    `198.51.100.1` to destination IP `203.0.113.1` (VLAN ID `100`).
  * **Reverse IPv4 Flow (`Port-Channel1.100` -> `Port-4.100`):** Source IP
    `203.0.113.1` to destination IP `198.51.100.1` (VLAN ID `100`).
  * **Forward IPv6 Flow (`Port-4.100` -> `Port-Channel1.100`):** Source IP
    `2001:db8:3000::1` to destination IP `2001:db8:4000::1` (VLAN ID `100`).
  * **Reverse IPv6 Flow (`Port-Channel1.100` -> `Port-4.100`):** Source IP
    `2001:db8:4000::1` to destination IP `2001:db8:3000::1` (VLAN ID `100`).
* Step 8 - Verify via gNMI counter `Watch` that steady-state packet loss is `0%`
  for all four IPv4 and IPv6 flows and that egress traffic on `Port-Channel1` is
  distributed across all 3 retained member ports (`Port-1`, `Port-2`, and
  `Port-3`).

### RT-5.18.6 - Differential Member Drain Behavior: Standard Member Drain (`forwarding-viable: false`) vs. Retained Uplink LAG Members

This scenario validates the differential drain behavior between standard drained
member ports (where `forwarding-viable` is set to `false` to drain egress
traffic off a member port) and retained Access Core uplink member ports (where
`aggregate-id` is preserved and `forwarding-viable` remains `true`), verifying
that inband `B2_CORE` VRF eBGP sessions and bidirectional IPv4/IPv6 traffic
remain uninterrupted.

* Step 1 - Starting from the converged `B2_CORE` VRF topology in RT-5.18.5,
  apply a member drain state to `Port-1` while retaining `Port-2` and `Port-3`
  as active forwarding uplink members via `gNMI.Set`:
  * `/interfaces/interface[name=Port-1]/config/forwarding-viable` = `false`
  * `/interfaces/interface[name=<Port-1..Port-3>]/ethernet/config/aggregate-id` = `Port-Channel1`
  * `/interfaces/interface[name=<Port-2|Port-3>]/config/forwarding-viable` = `true`

#### Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "name": "Port-Channel1",
        "config": {
          "name": "Port-Channel1",
          "type": "iana-if-type:ieee8023adLag",
          "enabled": true
        }
      },
      {
        "name": "Port-1",
        "config": {
          "name": "Port-1",
          "type": "iana-if-type:ethernetCsmacd",
          "enabled": true,
          "forwarding-viable": false
        },
        "ethernet": {
          "config": {
            "aggregate-id": "Port-Channel1"
          }
        }
      }
    ]
  }
}
```

* Step 2 - Verify via gNMI Subscribe (`Watch` with `Await`) the differential
  member state across `Port-Channel1`:
  * `/interfaces/interface[name=Port-1]/state/forwarding-viable` transitions to `false`.
  * `/interfaces/interface[name=Port-2]/state/forwarding-viable` and `/interfaces/interface[name=Port-3]/state/forwarding-viable` remain `true`.
  * `/interfaces/interface[name=<Port-1..Port-3>]/ethernet/state/aggregate-id` remains `Port-Channel1`.
* Step 3 - Verify via gNMI that the dual-stack eBGP sessions in `B2_CORE`
  (`192.0.2.10` and `2001:db8::a`) remain `ESTABLISHED` without flapping:
  * `/network-instances/network-instance[name=B2_CORE]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=<192.0.2.10|2001:db8::a>]/state/session-state`
* Step 4 - Send continuous bidirectional IPv4 and IPv6 traffic in the `B2_CORE`
  VRF between ATE `Port-4.100` and ATE `Port-Channel1.100` (using the flows from
  RT-5.18.5 Step 7) and verify via gNMI interface counter `Watch`:
  * DUT `Port-1` transmits zero egress data-plane traffic
    (`/interfaces/interface[name=Port-1]/state/counters/out-unicast-pkts` rate
    is `0` for routed data traffic), while `Port-2` and `Port-3` carry 100% of
    the egress traffic from the DUT.
  * Steady-state bidirectional IPv4 and IPv6 traffic loss across `B2_CORE` is
    `0%`.
* Step 5 - Restore `Port-1` to the retained Access Core uplink state via
  `gNMI.Set` and verify recovery:
  * Set `/interfaces/interface[name=Port-1]/config/forwarding-viable` = `true`.
  * Verify `/interfaces/interface[name=Port-1]/state/forwarding-viable` returns
    to `true`.
  * Confirm egress traffic resumes across all 3 member links (`Port-1`,
    `Port-2`, and `Port-3`) with `0%` steady-state loss.

### Cleanup

* Stop all ATE traffic flows and OTG protocols.
* Register and execute `t.Cleanup()` routines to ensure `Port-Channel1` and all
  member interfaces (`Port-1` through `Port-4`) are administratively enabled
  (`enabled: true`), `forwarding-viable` is restored to `true`, and revert all
  test-specific gNMI configurations (LAG, subinterfaces, `B2_CORE` and `DEFAULT`
  network instances, BGP, and IS-IS) so the DUT is returned to its exact
  pre-test baseline state.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/forwarding-viable:
  /interfaces/interface/config/name:
  /interfaces/interface/config/type:
  /interfaces/interface/state/admin-status:
  /interfaces/interface/state/counters/in-octets:
  /interfaces/interface/state/counters/in-unicast-pkts:
  /interfaces/interface/state/counters/out-octets:
  /interfaces/interface/state/counters/out-unicast-pkts:
  /interfaces/interface/state/enabled:
  /interfaces/interface/state/forwarding-viable:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/aggregation/config/lag-type:
  /interfaces/interface/aggregation/config/min-links:
  /interfaces/interface/aggregation/state/lag-type:
  /interfaces/interface/ethernet/config/aggregate-id:
  /interfaces/interface/ethernet/state/aggregate-id:
  /interfaces/interface/subinterfaces/subinterface/config/index:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/vlan/match/single-tagged/config/vlan-id:
  /lacp/interfaces/interface/config/interval:
  /lacp/interfaces/interface/config/lacp-mode:
  /lacp/interfaces/interface/config/name:
  /lacp/interfaces/interface/members/member/state/activity:
  /lacp/interfaces/interface/members/member/state/aggregatable:
  /lacp/interfaces/interface/members/member/state/collecting:
  /lacp/interfaces/interface/members/member/state/distributing:
  /lacp/interfaces/interface/members/member/state/synchronization:
  /network-instances/network-instance/config/name:
  /network-instances/network-instance/config/type:
  /network-instances/network-instance/interfaces/interface/config/id:
  /network-instances/network-instance/interfaces/interface/config/interface:
  /network-instances/network-instance/interfaces/interface/config/subinterface:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/installed:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/adjacency-state:
rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Required DUT platform

* vRX
