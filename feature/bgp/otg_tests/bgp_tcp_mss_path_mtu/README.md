# RT-1.21: BGP TCP MSS and PMTUD

## Summary

This test validates that the router supports the TCP maximum segment size (MSS)
and path MTU discovery (PMTUD) settings for BGP sessions:

*   Configuration and state of the TCP MSS (`transport/config/tcp-mss` and
    `transport/state/tcp-mss`) and of PMTUD (`transport/config/mtu-discovery`
    and `transport/state/mtu-discovery`), at the neighbor level and at the
    peer-group level
*   IPv4 and IPv6 eBGP and iBGP sessions
*   `transport/state/tcp-mss` reports the configured MSS (the applied
    configuration, including a value inherited from the peer group), in the
    same way as `timers/state/hold-time`. When no MSS is configured, it may
    instead report the MSS in use on the TCP session.
*   PMTUD lowers the TCP segment size when a router between the BGP peers
    reports, with an ICMP Fragmentation Needed or Packet Too Big message, that
    a packet is too big for its next link. This testbed has no router between
    the DUT and the ATE, so the test verifies the PMTUD configuration and
    state, not the PMTUD behavior.
*   TODO: The current test code configures TCP MD5 authentication on the
    iBGP neighbor. Remove it when the code is updated to this README; BGP
    MD5 authentication is covered by RT-1.1.

### Testbed type

*   [TESTBED_DUT_ATE_2LINKS](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

### Topology

```mermaid
graph LR
    ATE["ATE"] <-->|"port-1: eBGP IPv4 and IPv6"| DUT["DUT"]
    ATE <-->|"port-2: iBGP IPv4 and IPv6"| DUT
```

### DUT and OTG properties

| Device | Port   | IPv4         | IPv6                    | AS    |
| :----- | :----- | :----------- | :---------------------- | :---- |
| DUT    | Port 1 | 192.0.2.1/30 | 2001:db8::192:0:2:1/126 | 64500 |
| DUT    | Port 2 | 192.0.2.5/30 | 2001:db8::192:0:2:5/126 | 64500 |
| ATE    | Port 1 | 192.0.2.2/30 | 2001:db8::192:0:2:2/126 | 64501 |
| ATE    | Port 2 | 192.0.2.6/30 | 2001:db8::192:0:2:6/126 | 64500 |

### Procedure

#### Initial Setup:

*   Connect ATE port-1 to DUT port-1, and ATE port-2 to DUT port-2.
*   Configure the DUT and ATE interfaces per the table above.
*   Configure BGP on the DUT (AS `64500`) with two peer groups:
    *   `EBGP-PEERS` (peer AS `64501`) containing the IPv4 and IPv6 eBGP
        neighbors on ATE port-1.
    *   `IBGP-PEERS` (peer AS `64500`) containing the IPv4 and IPv6 iBGP
        neighbors on ATE port-2.
*   Configure the matching BGP peers on the ATE, and verify that all four
    sessions are `ESTABLISHED`.

### RT-1.21.1: TCP MSS at the neighbor level

*   If the DUT reports `transport/state/tcp-mss` for the neighbors before an
    MSS is configured, verify that the value is at least the TCP default MSS
    (536 for IPv4 and 1220 for IPv6) and at most the MTU of the DUT port minus
    40 bytes for IPv4 or 60 bytes for IPv6 (1460 and 1440 for a 1500-byte MTU).
*   Set the MTU of DUT port-1, DUT port-2, ATE port-1 and ATE port-2 to 5040
    bytes, and verify that all four sessions are `ESTABLISHED`.
*   Record `state/established-transitions` for all four neighbors.
*   Configure `transport/config/tcp-mss` to 4096 on all four neighbors.
*   Record whether applying the MSS reset the sessions, that is, whether
    `state/established-transitions` increased or `state/session-state` left
    `ESTABLISHED` within 30 seconds of the change. This shows whether the
    configuration change is disruptive. It is reported only and is not a
    pass/fail criterion.
*   Re-establish the four sessions, for example by restarting the ATE BGP
    peers.
*   Verify that all four sessions are `ESTABLISHED`, and that
    `transport/state/tcp-mss` equals `transport/config/tcp-mss` (4096) for all
    four neighbors.
*   Remove `transport/config/tcp-mss` from the four neighbors.

#### Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "name": "port1",
        "config": {
          "name": "port1",
          "type": "iana-if-type:ethernetCsmacd",
          "mtu": 5040
        }
      },
      {
        "name": "port2",
        "config": {
          "name": "port2",
          "type": "iana-if-type:ethernetCsmacd",
          "mtu": 5040
        }
      }
    ]
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
              "name": "BGP",
              "config": {
                "identifier": "BGP",
                "name": "BGP"
              },
              "bgp": {
                "neighbors": {
                  "neighbor": [
                    {
                      "neighbor-address": "192.0.2.2",
                      "config": {
                        "neighbor-address": "192.0.2.2"
                      },
                      "transport": {
                        "config": {
                          "tcp-mss": 4096
                        }
                      }
                    },
                    {
                      "neighbor-address": "2001:db8::192:0:2:2",
                      "config": {
                        "neighbor-address": "2001:db8::192:0:2:2"
                      },
                      "transport": {
                        "config": {
                          "tcp-mss": 4096
                        }
                      }
                    },
                    {
                      "neighbor-address": "192.0.2.6",
                      "config": {
                        "neighbor-address": "192.0.2.6"
                      },
                      "transport": {
                        "config": {
                          "tcp-mss": 4096
                        }
                      }
                    },
                    {
                      "neighbor-address": "2001:db8::192:0:2:6",
                      "config": {
                        "neighbor-address": "2001:db8::192:0:2:6"
                      },
                      "transport": {
                        "config": {
                          "tcp-mss": 4096
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
  }
}
```

### RT-1.21.2: TCP MSS at the peer-group level

*   Set `transport/config/tcp-mss` to 4000 on the peer groups `EBGP-PEERS` and
    `IBGP-PEERS`, with no MSS configured on the neighbors. The MTU of all four
    ports stays at 5040 bytes from RT-1.21.1.
*   Re-establish the four sessions.
*   Verify that all four sessions are `ESTABLISHED`, that
    `transport/state/tcp-mss` is 4000 for both peer groups, and that all four
    neighbors inherit it (`transport/state/tcp-mss` is 4000).
*   Remove `transport/config/tcp-mss` from both peer groups, and remove the MTU
    configuration from the four ports so that they return to the default MTU.

#### Canonical OC

```json
{
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
              "name": "BGP",
              "config": {
                "identifier": "BGP",
                "name": "BGP"
              },
              "bgp": {
                "peer-groups": {
                  "peer-group": [
                    {
                      "peer-group-name": "EBGP-PEERS",
                      "config": {
                        "peer-group-name": "EBGP-PEERS",
                        "peer-as": 64501
                      },
                      "transport": {
                        "config": {
                          "tcp-mss": 4000
                        }
                      }
                    },
                    {
                      "peer-group-name": "IBGP-PEERS",
                      "config": {
                        "peer-group-name": "IBGP-PEERS",
                        "peer-as": 64500
                      },
                      "transport": {
                        "config": {
                          "tcp-mss": 4000
                        }
                      }
                    }
                  ]
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "neighbor-address": "192.0.2.2",
                      "config": {
                        "neighbor-address": "192.0.2.2",
                        "peer-group": "EBGP-PEERS"
                      }
                    },
                    {
                      "neighbor-address": "2001:db8::192:0:2:2",
                      "config": {
                        "neighbor-address": "2001:db8::192:0:2:2",
                        "peer-group": "EBGP-PEERS"
                      }
                    },
                    {
                      "neighbor-address": "192.0.2.6",
                      "config": {
                        "neighbor-address": "192.0.2.6",
                        "peer-group": "IBGP-PEERS"
                      }
                    },
                    {
                      "neighbor-address": "2001:db8::192:0:2:6",
                      "config": {
                        "neighbor-address": "2001:db8::192:0:2:6",
                        "peer-group": "IBGP-PEERS"
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

### RT-1.21.3: PMTUD at the neighbor and peer-group levels

No MSS is configured in this subtest, and all four ports use the default MTU.

*   Set `transport/config/mtu-discovery` to `true` on all four neighbors.
*   Re-establish the four sessions.
*   Verify that all four sessions are `ESTABLISHED` and that
    `transport/state/mtu-discovery` is `true` for all four neighbors.
*   Remove `transport/config/mtu-discovery` from the neighbors, and set it to
    `true` on the peer groups `EBGP-PEERS` and `IBGP-PEERS` instead.
*   Re-establish the four sessions.
*   Verify that all four sessions are `ESTABLISHED`, that
    `transport/state/mtu-discovery` is `true` for both peer groups, and that
    all four neighbors inherit it.

#### Canonical OC

```json
{
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
              "name": "BGP",
              "config": {
                "identifier": "BGP",
                "name": "BGP"
              },
              "bgp": {
                "peer-groups": {
                  "peer-group": [
                    {
                      "peer-group-name": "EBGP-PEERS",
                      "config": {
                        "peer-group-name": "EBGP-PEERS",
                        "peer-as": 64501
                      },
                      "transport": {
                        "config": {
                          "mtu-discovery": true
                        }
                      }
                    },
                    {
                      "peer-group-name": "IBGP-PEERS",
                      "config": {
                        "peer-group-name": "IBGP-PEERS",
                        "peer-as": 64500
                      },
                      "transport": {
                        "config": {
                          "mtu-discovery": true
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
  }
}
```

#### Test pass fail criteria

*   If any of the below conditions occur, the test should be declared as a
    "FAIL":
    1.  A BGP session is not `ESTABLISHED` after it is re-established
    2.  `transport/state/tcp-mss` is not 4096 for a neighbor in `RT-1.21.1`, or
        not 4000 for a peer group or neighbor in `RT-1.21.2`
    3.  `transport/state/mtu-discovery` is not `true` for a neighbor or peer
        group in `RT-1.21.3`
    4.  A `transport/state/tcp-mss` reported before any MSS is configured is
        outside the range given in `RT-1.21.1`

### Cleanup

*   Remove the BGP and MTU configurations from the DUT and stop OTG protocols
    to restore the baseline state.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  ## Config Parameter Coverage
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/config/tcp-mss:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/config/mtu-discovery:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/transport/config/tcp-mss:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/transport/config/mtu-discovery:

  ## Telemetry Parameter Coverage
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/state/tcp-mss:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/state/mtu-discovery:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/transport/state/tcp-mss:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/transport/state/mtu-discovery:

rpcs:
  gnmi:
    gNMI.Set:
      replace: true
      delete: true
    gNMI.Subscribe:
      on_change: true
```

## Minimum DUT platform requirement

N/A
