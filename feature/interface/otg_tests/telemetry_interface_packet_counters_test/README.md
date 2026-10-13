# gNMI-1.11: Telemetry: Interface Packet Counters

## Summary

Validate interfaces counters including both IPv4 and IPv6 counters.

## Procedure

In the automated ondatra test, verify the presence of the telemetry paths of the
following features:

*   Configure Interface and add load-interval:

    *   /interfaces/interface/rates/config/load-interval

*   Configure IPv4 and IPv6 addresses under subinterface:

    *   /interfaces/interface/config/enabled
    *   /interfaces/interface/subinterfaces/subinterface/config/enabled
    *   /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled
    *   /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled

    Validate that subinterface and IPv4 and IPv6 addresses are enabled:

    *   /interfaces/interface/subinterfaces/subinterface/state/enabled
    *   /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/state/enabled
    *   /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/state/enabled

*   Validate that Interface has load-interval configured:

    *   /interfaces/interface/rates/state/load-interval

*   Validate if counters are being updated consistently

    Check the presence of packet counter paths and monitor counters every
    30 seconds. Generate traffic to get at least 10 or more samples.

    *   /interfaces/interface[name='port']/state/counters/in-pkts
    *   /interfaces/interface[name='port']/state/counters/out-pkts
    *   /interfaces/interface[name='port']/state/counters/last-clear
    *   /interfaces/interface[name='port']/subinterfaces/subinterface[index='index-id']/ipv4/state/counters/in-pkts
    *   /interfaces/interface[name='port']/subinterfaces/subinterface[index='index-id']/ipv4/state/counters/out-pkts
    *   /interfaces/interface[name='port']/subinterfaces/subinterface[index='index-id']/ipv4/state/counters/out-forwarded-octets
    *   /interfaces/interface[name='port']/subinterfaces/subinterface[index='index-id']/ipv6/state/counters/in-pkts
    *   /interfaces/interface[name='port']/subinterfaces/subinterface[index='index-id']/ipv6/state/counters/out-pkts
    *   /interfaces/interface[name='port']/subinterfaces/subinterface[index='index-id']/ipv6/state/counters/out-forwarded-octets

*   Subinterfaces counters:

    Check the presence of packet counter paths

    *   /interfaces/interface/subinterfaces/subinterface/state/counters/in-octets
    *   /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-pkts
    *   /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-forwarded-octets
    *   /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-pkts
    *   /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-forwarded-octets
    *   TODO:
        /interfaces/interface[name=port]/subinterfaces/subinterface[index='index']/ipv4/state/counters/in-pkts
    *   TODO:
        /interfaces/interface[name=port]/subinterfaces/subinterface[index='index']/ipv4/state/counters/out-pkts
    *   TODO:
        /interfaces/interface[name=port]/subinterfaces/subinterface[index='index']/ipv6/state/counters/in-discarded-pkts
    *   TODO:
        /interfaces/interface[name=port]/subinterfaces/subinterface[index='index']/ipv6/state/counters/out-discarded-pkts

*   Ethernet interface counters

    Check the presence of counter path including 'in-maxsize-exceeded'

    *   TODO: /interfaces/interface/ethernet/state/counters/in-maxsize-exceeded
    *   /interfaces/interface/ethernet/state/counters/in-mac-pause-frames
    *   /interfaces/interface/ethernet/state/counters/out-mac-pause-frames
    *   /interfaces/interface/ethernet/state/counters/in-crc-errors
    *   /interfaces/interface/ethernet/state/counters/in-fragment-frames
    *   /interfaces/interface/ethernet/state/counters/in-jabber-frames
    *   /interfaces/interface/ethernet/state/counters/in-oversize-frames

*   Interface CPU and management

    Check the presence of CPU and management paths

    *   TODO: /interfaces/interface/state/cpu
    *   TODO: /interfaces/interface/state/management

### gNMI-1.11.1: Subinterface IPv4 and IPv6 Egress Streaming Telemetry Counters Validation

This subtest validates that subinterface-level IPv4 and IPv6 egress packet
counters (`out-pkts`) and forwarded octet counters (`out-forwarded-octets`),
along with their ingress counterparts (`in-pkts` and `in-forwarded-octets`),
are actively streamed and monotonically incremented without stale values across
periodic `gNMI.Subscribe` (`SAMPLE` mode) notifications when forwarding
dual-stack traffic (including native IPv4 egress packets and
their IPv6 counterparts) across the DUT.

#### Topology

```text
[ ATE Port 1 (port1) ] ---- (port1) [ DUT ] (port2) ---- [ ATE Port 2 (port2) ]
  198.51.100.1/31        198.51.100.0/31   198.51.100.2/31        198.51.100.3/31
  2001:db8::2/126        2001:db8::1/126   2001:db8::5/126        2001:db8::6/126
```

*   Connect `DUT port 1` (`port1`) to `ATE port 1` (`port1`) as the ingress L3
    singleton interface.
*   Connect `DUT port 2` (`port2`) to `ATE port 2` (`port2`) as the egress L3
    singleton interface.

#### Step 1: Configure Dual-Stack Subinterfaces on DUT and ATE

*   Using `gNMI.Set` (`REPLACE`), configure `DUT port 1` (`port1`) and `DUT
    port 2` (`port2`) with subinterface `0` enabled for both IPv4 and IPv6:
    *   Set `/interfaces/interface/config/enabled` to `true`,
        `/interfaces/interface/config/type` to `iana-if-type:ethernetCsmacd`,
        and `/interfaces/interface/rates/config/load-interval` to `30`.
    *   Set `/interfaces/interface/subinterfaces/subinterface/config/index` to
        `0` and
        `/interfaces/interface/subinterfaces/subinterface/config/enabled` to
        `true`.
    *   On `DUT port 1` subinterface `0`:
        *   Set
            `/interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled`
            to `true`,
            `/interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip`
            to `198.51.100.0`, and
            `/interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length`
            to `31`.
        *   Set
            `/interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled`
            to `true`,
            `/interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip`
            to `2001:db8::1`, and
            `/interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length`
            to `126`.
    *   On `DUT port 2` subinterface `0`:
        *   Set
            `/interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled`
            to `true`,
            `/interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip`
            to `198.51.100.2`, and
            `/interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length`
            to `31`.
        *   Set
            `/interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled`
            to `true`,
            `/interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip`
            to `2001:db8::5`, and
            `/interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length`
            to `126`.
*   Configure the ATE OTG interfaces:
    *   Configure `ATE port 1` (`port1`) with MAC address `02:00:01:01:01:01`,
        IPv4 address `198.51.100.1/31` (gateway `198.51.100.0`), and IPv6
        address `2001:db8::2/126` (gateway `2001:db8::1`).
    *   Configure `ATE port 2` (`port2`) with MAC address `02:00:01:02:01:01`,
        IPv4 address `198.51.100.3/31` (gateway `198.51.100.2`), and IPv6
        address `2001:db8::6/126` (gateway `2001:db8::5`).

#### Step 2: Verify Operational State, Neighbor Resolution, and Baseline Counters

*   Verify via `gNMI.Get` / `gNMI.Subscribe` (`gnmi.Watch`) that both `DUT
    port 1` and `DUT port 2` are operationally up and enabled:
    *   `/interfaces/interface/state/oper-status` is `UP`.
    *   `/interfaces/interface/subinterfaces/subinterface/state/enabled` is
        `true`.
    *   `/interfaces/interface/subinterfaces/subinterface/ipv4/state/enabled` is
        `true`.
    *   `/interfaces/interface/subinterfaces/subinterface/ipv6/state/enabled` is
        `true`.
*   Start OTG protocols on the ATE and wait for both IPv4 ARP and IPv6 Neighbor
    Discovery (ND) resolution to complete (`otgutils.WaitForARP` for both `IPv4`
    and `IPv6`) prior to starting traffic so that initial packets are not
    dropped.
*   Query and record baseline subinterface counter values on `DUT port 1`
    subinterface `0` (ingress) and `DUT port 2` subinterface `0` (egress) via
    `gNMI.Get`:
    *   `/interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-pkts`
    *   `/interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-forwarded-octets`
    *   `/interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-pkts`
    *   `/interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-forwarded-octets`
    *   `/interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-pkts`
    *   `/interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-forwarded-octets`
    *   `/interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-pkts`
    *   `/interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-forwarded-octets`

#### Step 3: Stream Subinterface Egress and Ingress Counters Under Dual-Stack Traffic

*   Start two continuous, constant-rate unicast traffic flows from `ATE port 1`
    to `ATE port 2` routed across the DUT:
    *   **IPv4 Flow (`IPv4_test_flow`)**: Source IPv4 `198.51.100.1`,
        destination IPv4 `198.51.100.3` (representing native IPv4
        egress packets), fixed frame size `100` bytes,
        constant rate `15` pps.
    *   **IPv6 Flow (`IPv6_test_flow`)**: Source IPv6 `2001:db8::2`,
        destination IPv6 `2001:db8::6`, fixed frame size `100` bytes, constant
        rate `15` pps.
*   While both flows are actively transmitting, establish a `gNMI.Subscribe`
    streaming telemetry subscription in `SAMPLE` mode with a `30` second
    sampling interval over a `300` second collection window (collecting at least
    10 consecutive streaming samples) for `DUT port 1` subinterface `0` and
    `DUT port 2` subinterface `0` on the following paths:
    *   `/interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-pkts`
    *   `/interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-forwarded-octets`
    *   `/interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-pkts`
    *   `/interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-forwarded-octets`
    *   `/interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-pkts`
    *   `/interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-forwarded-octets`
    *   `/interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-pkts`
    *   `/interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-forwarded-octets`

#### Step 4: Validate Streaming Telemetry Updates and Post-Traffic Counter Accuracy

*   **Pass/Fail Criteria for Streaming Updates (`SAMPLE` Mode)**:
    *   **Pass**: Every consecutive `30` second streaming sample (`sample[i]` vs
        `sample[i-1]`) for `DUT port 2` subinterface `0` shows a strictly
        positive increment (`sample[i] > sample[i-1]`) for:
        *   `/interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-pkts`
        *   `/interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-forwarded-octets`
        *   `/interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-pkts`
        *   `/interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-forwarded-octets`
        and the egress per-interval delta on `DUT port 2` subinterface `0`
        matches the corresponding ingress per-interval delta on `DUT port 1`
        subinterface `0` (`in-pkts` and `in-forwarded-octets`) within the
        configured rate tolerance.
    *   **Fail**: Any of the subinterface IPv4 or IPv6 egress counter paths fail
        to emit streaming updates, return zero throughout the traffic window, or
        report stale/unchanged values (`sample[i] == sample[i-1]`) across
        consecutive sampling intervals while traffic is flowing.
*   **Pass/Fail Criteria for Post-Traffic Reconciliation**:
    *   Stop ATE traffic flows (`otg.StopTraffic(t)`).
    *   **Pass**:
        *   ATE flow metrics confirm `< 1%` packet loss (`0%` expected) on both
            `IPv4_test_flow` and `IPv6_test_flow`.
        *   On `DUT port 2` subinterface `0`, the total delta
            (`post_traffic - baseline`) for
            `/interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-pkts`
            and
            `/interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-pkts`
            is greater than or equal to the ATE transmitted packet count for the
            respective address family.
        *   On `DUT port 2` subinterface `0`, the total delta
            (`post_traffic - baseline`) for
            `/interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-forwarded-octets`
            and
            `/interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-forwarded-octets`
            is greater than or equal to the total forwarded IP payload octets
            for the respective address family.
    *   **Fail**: Packet loss is `>= 1%`, or the post-traffic subinterface
        `out-pkts` or `out-forwarded-octets` counter deltas are less than the
        expected transmitted packet or octet counts.

### Cleanup

*   Stop all active ATE traffic flows and OTG protocols.
*   Revert all interface and subinterface configurations applied during the test
    using `t.Cleanup()` so that `DUT port 1` and `DUT port 2` are restored to
    their baseline operational state upon test completion.

## Testbed type

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Test environment setup
The test uses a 2 port ATE setup where 2 ports are used as a singleton interface
Ports are configured with ipv4, ipv6 interfaces on DUT and ATE. Traffic is sent
and from ATE to DUT and the counters are verified.

## Canonical OC

```json
{
  "openconfig-interfaces:interfaces": {
    "interface": [
      {
        "config": {
          "description": "Input interface port1",
          "enabled": true,
          "name": "port1",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port1",
        "openconfig-if-rates:rates": {
          "config": {
            "load-interval": 30
          }
        },
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "enabled": true,
                "index": 0
              },
              "index": 0,
              "openconfig-if-ip:ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "198.51.100.0",
                        "prefix-length": 31
                      },
                      "ip": "198.51.100.0"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              },
              "openconfig-if-ip:ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:DB8::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:DB8::1"
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
          "description": "Output interface port2",
          "enabled": true,
          "name": "port2",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port2",
        "openconfig-if-rates:rates": {
          "config": {
            "load-interval": 30
          }
        },
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "enabled": true,
                "index": 0
              },
              "index": 0,
              "openconfig-if-ip:ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "198.51.100.2",
                        "prefix-length": 31
                      },
                      "ip": "198.51.100.2"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              },
              "openconfig-if-ip:ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:DB8::5",
                        "prefix-length": 126
                      },
                      "ip": "2001:DB8::5"
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
  }
}
```

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.
OC paths used for test setup are not listed here.

```yaml
paths:
  ## Config Paths ##
  /interfaces/interface/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled:
  /interfaces/interface/rates/config/load-interval:

  ## State Paths ##
  /interfaces/interface/state/oper-status:
  /interfaces/interface/state/counters/carrier-transitions:
  /interfaces/interface/state/counters/in-broadcast-pkts:
  /interfaces/interface/state/counters/in-discards:
  /interfaces/interface/state/counters/in-errors:
  /interfaces/interface/state/counters/in-fcs-errors:
  /interfaces/interface/state/counters/in-multicast-pkts:
  /interfaces/interface/state/counters/in-octets:
  /interfaces/interface/state/counters/in-pkts:
  /interfaces/interface/state/counters/in-unicast-pkts:
  /interfaces/interface/state/counters/out-broadcast-pkts:
  /interfaces/interface/state/counters/out-discards:
  /interfaces/interface/state/counters/out-errors:
  /interfaces/interface/state/counters/out-multicast-pkts:
  /interfaces/interface/state/counters/out-octets:
  /interfaces/interface/state/counters/out-pkts:
  /interfaces/interface/state/counters/out-unicast-pkts:
  /interfaces/interface/state/counters/last-clear:
  /interfaces/interface/rates/state/load-interval:
  /interfaces/interface/subinterfaces/subinterface/state/enabled:
  /interfaces/interface/subinterfaces/subinterface/state/counters/in-octets:
  /interfaces/interface/subinterfaces/subinterface/state/counters/out-broadcast-pkts:
  /interfaces/interface/subinterfaces/subinterface/state/counters/carrier-transitions:
  /interfaces/interface/subinterfaces/subinterface/state/counters/out-errors:
  /interfaces/interface/subinterfaces/subinterface/state/counters/last-clear:
  /interfaces/interface/subinterfaces/subinterface/state/counters/in-errors:
  /interfaces/interface/subinterfaces/subinterface/state/counters/in-unknown-protos:
  /interfaces/interface/subinterfaces/subinterface/state/counters/in-broadcast-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-forwarded-octets:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-forwarded-octets:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-forwarded-octets:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-forwarded-octets:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-discarded-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-discarded-pkts:
  /interfaces/interface/ethernet/state/counters/in-maxsize-exceeded:
  /interfaces/interface/ethernet/state/counters/in-mac-pause-frames:
  /interfaces/interface/ethernet/state/counters/out-mac-pause-frames:
  /interfaces/interface/ethernet/state/counters/in-crc-errors:
  /interfaces/interface/ethernet/state/counters/in-fragment-frames:
  /interfaces/interface/ethernet/state/counters/in-jabber-frames:
  /interfaces/interface/ethernet/state/counters/in-oversize-frames:
  /interfaces/interface/state/cpu:
  /interfaces/interface/state/management:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Subscribe:
    gNMI.Set:
```

## Required DUT platform

* Specify the minimum DUT-type
    * FFF - fixed form factor is enough for this test. However it can run also
      on a MFF testbed.
      gNMI.Set:

## Minimum DUT platform requirement

* FFF - fixed form factor
