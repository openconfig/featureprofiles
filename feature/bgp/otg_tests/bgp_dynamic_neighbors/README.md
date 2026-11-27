# RT-1.103: DUT iBGP passive listener FNT with common router id

## Summary

Validate the iBGP passive listener functionality on DUT using dynamic peering
and verify consistent router-id usage across multiple sessions.

> [!NOTE]
> This requirement is discussed/aligned in [PR #5620](https://github.com/openconfig/featureprofiles/pull/5620).

## Testbed type

*   `TESTBED_DUT_ATE_2LINKS` (ATE simulating multiple internal peers).

## Topology

```mermaid
graph LR;
  ate1[ATE Port 1] -- "BGP Initiator (Listen Range)" --> dut1[DUT Port 1]
  ate2[ATE Port 2] -- "BGP Initiator (Listen Range)" --> dut2[DUT Port 2]
```

*   Connect ATE Port 1 to DUT Port 1.
*   Connect ATE Port 2 to DUT Port 2.
*   ATE will initiate multiple BGP sessions from different source IPs within the configured listen ranges (several ranges of different lengths, e.g., `2001:db8:1::/64`, `2001:db8:10::/47`, `2001:db8:20::/46`).
*   The DUT global AS (`64500`) differs from the AS used on the dynamic sessions: the peer-group is configured with `local-as 64510` (no-prepend, replace-as) and `peer-as 64510`, so the sessions are iBGP from the initiator's point of view.
*   Dynamic sessions are sourced from the DUT loopback (`update-source` / `transport/local-address`).
*   Simulated ATE peers may use the **same BGP Identifier (Router ID)** to verify standard transport separation behavior (identified by source IP).
*   One of the ports (e.g., ATE Port 1) will also establish a static BGP session using a direct interface IP/port IP address (outside the listen range).

## Procedure

### Configuration

1.  Configure DUT with a global router-id and global AS `64500`.
2.  Configure a peer-group for dynamic neighbors (e.g., `RM_DYNAMIC_PEERS`) with `peer-as 64510`, `local-as 64510` (no-prepend, replace-as), `transport/local-address` = DUT `Loopback0` and `send-community-type [STANDARD, EXTENDED]`.
3.  Configure multiple dynamic neighbor prefixes (listen ranges, e.g., `2001:db8:1::/64`, `2001:db8:10::/47`, `2001:db8:20::/46`) associated with the peer-group.
4.  Configure MD5 authentication on the peer-group.
5.  Enable `L3VPN_IPV4_UNICAST` and `L3VPN_IPV6_UNICAST` address families on the peer-group (RT-Constraint behaviour is covered in RT-1.106).
6.  Configure a static BGP neighbor (outside the listen range, e.g., `2001:db8:0:1::2`).

### Tests

#### RT-1.103.1 - Dynamic Peering and Router-ID Validation

*   **Step 1**: Enable the dynamic neighbor prefixes on DUT (e.g., `2001:db8:1::/64`, `2001:db8:10::/47`, `2001:db8:20::/46`).
*   **Step 2**: Initiate BGP sessions from multiple ATE interfaces using different specific source IPs (e.g., `2001:db8:1::1`, `2001:db8:1::2`, `2001:db8:10::1`, `2001:db8:20::1`) covering every listen range.
*   **Step 3**: Verify that sessions transition to `ESTABLISHED`.
*   **Step 4**: Verify that the DUT uses the configured router-id (e.g., `192.0.2.1`) for all dynamically established sessions.
*   **Step 5**: Verify concurrent session handling (e.g., 5-10 sessions established simultaneously).
*   **Step 6**: Attempt to initiate a BGP session from an IP in the listen range but using an incorrect/Different ASN (e.g., `64501` instead of `64510`).
*   **Step 7**: Verify that the DUT rejects the connection or that it fails to transition to `ESTABLISHED` state.

#### RT-1.103.2 - Telemetry for Dynamic Neighbors

*   **Step 1**: Verify that the established dynamic peers (e.g., `2001:db8:1::1`) are instantiated and visible under the `neighbors/neighbor` container in telemetry.
*   **Step 2**: Query the state leaf `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/dynamically-configured` for the dynamic peer and verify it returns `true`.
*   **Step 3**: Verify that standard statically configured neighbors return `false` for this leaf.
*   **Step 4**: Verify `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/peer-group` = `RM_DYNAMIC_PEERS` for every dynamic peer, regardless of which listen range it matched.
*   **Step 5**: Verify `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/state/local-address` equals the DUT `Loopback0` address for every dynamic peer.

#### RT-1.103.3 - Secured Dynamic Peers (iBGP MD5)

*   **Step 1**: Configure the peer group `RM_DYNAMIC_PEERS` with `auth-password`.
*   **Step 2**: Initiate BGP sessions from ATE using the correct MD5 password.
*   **Step 3**: Verify that sessions transition to `ESTABLISHED`.
*   **Step 4**: Attempt to initiate a session from ATE with an incorrect password or no password.
*   **Step 5**: Verify that the session FAILS to establish.

#### RT-1.103.4 - Duplicate ATE Router-ID (Transport Separation)

*   **Step 1**: Establish a BGP session from ATE Peer 1 (Source IP `2001:db8:1::1`) with a specific Router-ID (e.g., `192.0.2.200`).
*   **Step 2**: Initiate a BGP session from ATE Peer 2 (Source IP `2001:db8:1::2`) using the **same** Router-ID (`192.0.2.200`).
*   **Step 3**: Verify that **both** sessions transition to `ESTABLISHED` and are maintained independently, relying on transport address separation.

#### RT-1.103.5 - Overlapping Prefix Best-Path Selection and Reconvergence

> [!NOTE]
> This sub-test exercises generic best-path behaviour with a static peer; it is
> not part of the EdgeBGP dynamic-listener profile, where all core peers are
> dynamic.

Verifies that the DUT correctly evaluates BGP best-path parameters (e.g., MED) and executes seamless route reconvergence when the same prefix is received concurrently from both static and dynamic BGP peers.

*   **Step 1**: Establish a static BGP session on one of the current ports (e.g., Port 1) using an IP address outside the listen range (e.g., the direct port IP address/subnet `2001:db8:0:1::2`).
*   **Step 2**: Establish a dynamic BGP session (e.g., on Port 2 or behind Port 1) using an IP within the listen range (e.g., `2001:db8:1::1`).
*   **Step 3**: Advertise the exact same prefix (e.g., `2001:db8:2::/64` for IPv6 or `198.51.100.0/24` for IPv4) from both ATE ports/peers to DUT.
*   **Step 4**: Configure Multi-Exit Discriminator (MED) to influence best-path selection: set **MED = 50** on the static peer and **MED = 100** on the dynamic peer.
*   **Step 5**: Verify DUT selects the best path based on standard BGP decision process (favors path with lower MED, i.e., the static peer).
*   **Step 6**: Withdraw/Disable the preferred path from ATE.
*   **Step 7**: Verify seamless reconvergence to the alternative path on DUT.

#### RT-1.103.6 - Peer-Group Local-AS Substitution (Global AS differs from session AS)

*   **Step 1**: With DUT global AS `64500`, configure `local-as 64510` (no-prepend, replace-as) and `peer-as 64510` on `RM_DYNAMIC_PEERS`.
*   **Step 2**: Initiate sessions from ATE peers in AS `64510`; verify the OPEN message received on ATE carries AS `64510` and the sessions transition to `ESTABLISHED` as **iBGP**.
*   **Step 3**: Verify `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/local-as` = `64510` and `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/peer-type` = `INTERNAL` for the dynamic peers, while `/network-instances/network-instance/protocols/protocol/bgp/global/state/as` remains `64500`.
*   **Step 4**: Advertise a VPN prefix from the DUT to a dynamic peer and verify the AS-Path does **not** contain `64500` (replace-as) and that no additional AS is prepended (no-prepend).
*   **Step 5**: Verify `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/send-community-type` = `[STANDARD, EXTENDED]` on the dynamic peers and that both a standard and an extended community set on the advertised prefix are received by ATE.

#### Cleanup

*   Remove the dynamic neighbor prefixes, peer-group `RM_DYNAMIC_PEERS`
    (including `local-as`, `transport/local-address`, authentication,
    `send-community-type` and L3VPN address families) and the static neighbor /
    `STATIC_PEERS` peer-group from the DUT.
*   Stop all ATE BGP emulations and verify no dynamic or static neighbors remain
    under `neighbors/neighbor` in DUT telemetry.

## Canonical OC

```json
{
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "DEFAULT"
        },
        "name": "DEFAULT",
        "protocols": {
          "protocol": [
            {
              "bgp": {
                "global": {
                  "config": {
                    "as": 64500,
                    "router-id": "192.0.2.1"
                  },
                  "dynamic-neighbor-prefixes": {
                    "dynamic-neighbor-prefix": [
                      {
                        "config": {
                          "prefix": "2001:db8:1::/64",
                          "peer-group": "RM_DYNAMIC_PEERS"
                        },
                        "prefix": "2001:db8:1::/64"
                      },
                      {
                        "config": {
                          "prefix": "2001:db8:10::/47",
                          "peer-group": "RM_DYNAMIC_PEERS"
                        },
                        "prefix": "2001:db8:10::/47"
                      },
                      {
                        "config": {
                          "prefix": "2001:db8:20::/46",
                          "peer-group": "RM_DYNAMIC_PEERS"
                        },
                        "prefix": "2001:db8:20::/46"
                      }
                    ]
                  }
                },
                "peer-groups": {
                  "peer-group": [
                    {
                      "config": {
                        "peer-group-name": "RM_DYNAMIC_PEERS",
                        "peer-as": 64510,
                        "local-as": 64510,
                        "send-community-type": ["STANDARD", "EXTENDED"],
                        "auth-password": "secret_md5_key"
                      },
                      "peer-group-name": "RM_DYNAMIC_PEERS",
                      "transport": {
                        "config": {
                          "local-address": "Loopback0"
                        }
                      },
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:L3VPN_IPV4_UNICAST",
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:L3VPN_IPV4_UNICAST",
                              "enabled": true
                            }
                          },
                          {
                            "afi-safi-name": "openconfig-bgp-types:L3VPN_IPV6_UNICAST",
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:L3VPN_IPV6_UNICAST",
                              "enabled": true
                            }
                          }
                        ]
                      }
                    },
                    {
                      "config": {
                        "peer-group-name": "STATIC_PEERS",
                        "peer-as": 64500
                      },
                      "peer-group-name": "STATIC_PEERS"
                    }
                  ]
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "config": {
                        "neighbor-address": "2001:db8:0:1::2",
                        "peer-as": 64500,
                        "peer-group": "STATIC_PEERS"
                      },
                      "neighbor-address": "2001:db8:0:1::2"
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
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  # BGP Global Config
  /network-instances/network-instance/protocols/protocol/bgp/global/config/as:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id:
  /network-instances/network-instance/protocols/protocol/bgp/global/state/router-id:

  # BGP Dynamic Neighbors
  /network-instances/network-instance/protocols/protocol/bgp/global/dynamic-neighbor-prefixes/dynamic-neighbor-prefix/config/prefix:
  /network-instances/network-instance/protocols/protocol/bgp/global/dynamic-neighbor-prefixes/dynamic-neighbor-prefix/config/peer-group:

  # BGP Peer Group Auth
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/auth-password:

  # BGP Peer Group Local-AS, Transport, Communities and AFI-SAFIs
  /network-instances/network-instance/protocols/protocol/bgp/global/state/as:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-as:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/local-as:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/transport/config/local-address:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/send-community-type:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/afi-safi-name:

  # BGP Neighbor State (Dynamic)
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/dynamically-configured:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/peer-group:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/local-as:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/peer-type:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/send-community-type:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/state/local-address:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

*   FFF (Fixed Form Factor) or MFF supporting dynamic BGP peers.
