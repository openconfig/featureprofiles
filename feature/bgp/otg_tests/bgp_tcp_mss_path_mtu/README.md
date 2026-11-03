# RT-1.21: BGP TCP MSS and PMTUD

## Summary

This test validates that the router supports the TCP maximum segment size (MSS)
and path MTU discovery (PMTUD) settings for BGP sessions:

*   Configuration and state of the TCP MSS on IPv4 and IPv6 eBGP neighbors
    (`transport/config/tcp-mss` and `transport/state/tcp-mss`)
*   Configuration and state of PMTUD on an iBGP neighbor with TCP MD5
    authentication (`transport/config/mtu-discovery` and
    `transport/state/mtu-discovery`)
*   `transport/state/tcp-mss` reports the configured MSS. When no MSS is
    configured, it may instead report the MSS in use on the TCP session.
*   TODO: Add IPv6 link-local BGP neighbors if they are required.

### Testbed type

*   [TESTBED_DUT_DUT_ATE_2LINKS](https://github.com/openconfig/featureprofiles/blob/main/topologies/dutdutate.testbed)

### Topology

```mermaid
graph LR
    ATE["ATE"] <-->|"port-1 to port-1"| DUT1["DUT1"]
    DUT1 <-->|"port-2 to port-1"| DUT2["DUT2"]
```

### DUT and OTG properties

| Device | Port   | IPv4         | IPv6                    | AS    |
| :----- | :----- | :----------- | :---------------------- | :---- |
| DUT1   | Port 1 | 192.0.2.1/30 | 2001:db8::192:0:2:1/126 | 64500 |
| DUT1   | Port 2 | 192.0.2.5/30 | N/A                     | 64500 |
| DUT2   | Port 1 | 192.0.2.6/30 | N/A                     | 64501 |
| ATE    | Port 1 | 192.0.2.2/30 | 2001:db8::192:0:2:2/126 | 64501 |

### Procedure

#### Initial Setup:

*   Connect ATE port-1 to DUT1 port-1, and DUT1 port-2 to DUT2 port-1.
*   Configure the DUT and ATE interfaces per the table above.

### RT-1.21.1: TCP MSS on eBGP sessions

*   Establish IPv4 and IPv6 eBGP sessions between ATE port-1 (AS `64501`) and
    DUT1 port-1 (AS `64500`).
*   If DUT1 reports `transport/state/tcp-mss` for the eBGP neighbors before an
    MSS is configured, verify that the value is non-zero and below the MTU of
    DUT1 port-1.
*   Set the MTU of DUT1 port-1 and ATE port-1 to 5040 bytes.
*   Configure `transport/config/tcp-mss` to 4096 on the IPv4 and IPv6 eBGP
    neighbors on DUT1.
*   Re-establish the eBGP sessions, for example by restarting the ATE BGP
    peers.
*   Verify that both sessions are `ESTABLISHED` and that
    `transport/state/tcp-mss` is 4096 for both neighbors.

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

### RT-1.21.2: PMTUD on a multihop iBGP session with TCP MD5

*   Remove the BGP and MTU configuration of RT-1.21.1 from DUT1, so that DUT1
    port-1 returns to the default MTU of 1500 bytes.
*   Provide reachability between ATE port-1 and DUT2 port-1 for a multihop
    session, for example with IS-IS between ATE port-1 and DUT1 and a static
    route on DUT2 to `192.0.2.2/32` through DUT1.
*   Establish an IPv4 iBGP session with TCP MD5 authentication between ATE
    port-1 (AS `64501`) and DUT2 port-1 (AS `64501`), two hops apart through
    DUT1.
*   Set the MTU of ATE port-1, DUT1 port-2 and DUT2 port-1 to 5040 bytes, and
    leave DUT1 port-1 at 1500 bytes, so that DUT1 port-1 is the smallest MTU on
    the path.
*   Set `transport/config/mtu-discovery` to `true` on the iBGP neighbor on
    DUT2.
*   Re-establish the iBGP session.
*   Verify that the session is `ESTABLISHED` and that
    `transport/state/mtu-discovery` is `true`.
*   If DUT2 reports `transport/state/tcp-mss` for the iBGP neighbor (no MSS is
    configured), verify that the value is non-zero and below 1500, which shows
    that PMTUD has adjusted the MSS to the path.

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
                        "neighbor-address": "192.0.2.2",
                        "peer-as": 64501,
                        "auth-password": "md5-test-password"
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

*   If any of the below conditions are not met, test should be declared as a
    "FAIL":
    1.  A BGP session is not `ESTABLISHED` after it is re-established
    2.  `transport/state/tcp-mss` is not 4096 for the eBGP neighbors in
        `RT-1.21.1`
    3.  A reported `transport/state/tcp-mss` is zero or not below the limit in
        `RT-1.21.1` or `RT-1.21.2`
    4.  `transport/state/mtu-discovery` is not `true` on DUT2 in `RT-1.21.2`

### Cleanup

*   Remove the BGP, IS-IS, static route and MTU configurations from DUT1 and
    DUT2 and stop OTG protocols to restore the baseline state.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  ## Config Parameter Coverage
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/config/tcp-mss:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/config/mtu-discovery:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/auth-password:
  /interfaces/interface/config/mtu:

  ## Telemetry Parameter Coverage
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/state/tcp-mss:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/state/mtu-discovery:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:

  # TODO: Propose a leaf that reports the MSS in use on the session to
  # openconfig/public, similar to timers/state/negotiated-hold-time.
  # /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/state/negotiated-tcp-mss:

rpcs:
  gnmi:
    gNMI.Set:
      replace: true
      delete: true
    gNMI.Subscribe:
      on_change: true
```

## Minimum DUT platform requirement

*   FFF
