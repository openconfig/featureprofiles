# TRANSCEIVER-109: 800ZR Operational Mode Transition and Inventory Persistence

## Summary

This test validates that 800G ZR and ZR+ (`SPEED_800GB`, e.g., `OSFP-800G-ZR`,
`OSFP-800G-ZR-M`, `OSFP-800G-ZR-M-HP`) coherent optical transceivers across
multiple line cards properly initialize, transition from a default optical
application selection (`Appsel`) mode to a custom `operational-mode` with a
transceiver state bounce, and retain complete hardware inventory telemetry
across repeated `gNMI.Set` interface deletion and re-creation cycles.
Additionally, the test verifies event-driven `gNMI.Subscribe` state
convergence, negative fault and recovery handling (unsupported
`operational-mode`, optical frequency/mode mismatch, and administrative laser
shutoff), and zero-loss bidirectional dual-stack IPv4 and IPv6 traffic
forwarding across scaled static route prefixes.

## Testbed type

*   [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

### Test environment setup

```text
  +------------+              800G ZR Link 1 (FPC A)      +------------+
  |            | (Port 1: 192.0.2.2/30, 2001:db8::2/126)  |            |
  |            | <======================================> |            |
  |            | (Port 1: 192.0.2.1/30, 2001:db8::1/126)  |            |
  |    ATE     |                                          |    DUT     |
  |            |              800G ZR Link 2 (FPC B)      |            |
  |            | (Port 2: 192.0.2.6/30, 2001:db8::6/126)  |            |
  |            | <======================================> |            |
  |            | (Port 2: 192.0.2.5/30, 2001:db8::5/126)  |            |
  +------------+                                          +------------+
```

*   **Topology & Multi-Line-Card Placement**: Connect two 800G ZR/ZR+ interfaces
    between the DUT and ATE (or connect two 800G ZR/ZR+ interfaces on the DUT
    via a duplex LC fiber jumper with less than 2-3 dB insertion loss while
    anchoring ingress/egress L3 traffic on ATE ports). On modular chassis
    systems, **DUT port 1** and **DUT port 2** should reside on **distinct line
    cards (FPCs)** so multi-FPC transceiver initialization and inventory
    database synchronization are exercised simultaneously:
    *   **Link 1 (DUT Port 1 <-> ATE Port 1)**:
        *   DUT Interface Name: `Ethernet4/1/1` (Subinterface index `0`)
        *   DUT Hardware Port Component: `Ethernet4/1-Port`
        *   DUT Transceiver Component: `Ethernet4/1`
        *   DUT Optical-Channel Component: `Ethernet4/1-Optical0`
        *   DUT Coherent OTN Logical Channel Index: `8000` (`PROT_OTN`)
        *   DUT Ethernet Logical Channel Index: `80000` (`PROT_ETHERNET`)
        *   Optical Center Frequency: `196100000` MHz (`196.1` THz)
        *   Optical Target Output Power: `-7` dBm (`"-7"`)
        *   Default Operational Mode (`Appsel`): `1`
        *   Custom Operational Mode (`Appsel`): `2` (or the platform-specific
            custom 800ZR operational mode supplied via test flag/deviation)
    *   **Link 2 (DUT Port 2 <-> ATE Port 2)**:
        *   DUT Interface Name: `Ethernet5/1/1` (Subinterface index `0`)
        *   DUT Hardware Port Component: `Ethernet5/1-Port`
        *   DUT Transceiver Component: `Ethernet5/1`
        *   DUT Optical-Channel Component: `Ethernet5/1-Optical0`
        *   DUT Coherent OTN Logical Channel Index: `8001` (`PROT_OTN`)
        *   DUT Ethernet Logical Channel Index: `80001` (`PROT_ETHERNET`)
        *   Optical Center Frequency: `196100000` MHz (`196.1` THz)
        *   Optical Target Output Power: `-7` dBm (`"-7"`)
        *   Default Operational Mode (`Appsel`): `1`
        *   Custom Operational Mode (`Appsel`): `2` (matching Link 1)
*   **Explicit L3 Addressing and Scaled Static Routing Plan**:
    *   **DUT port 1 connected to ATE port 1** (`Ethernet4/1/1.0`):
        *   IPv4 address: `192.0.2.1/30`
        *   IPv6 address: `2001:db8::1/126`
    *   **ATE port 1**:
        *   IPv4 address: `192.0.2.2/30` (Default Gateway: `192.0.2.1`)
        *   IPv6 address: `2001:db8::2/126` (Default Gateway: `2001:db8::1`)
    *   **DUT port 2 connected to ATE port 2** (`Ethernet5/1/1.0`):
        *   IPv4 address: `192.0.2.5/30`
        *   IPv6 address: `2001:db8::5/126`
    *   **ATE port 2**:
        *   IPv4 address: `192.0.2.6/30` (Default Gateway: `192.0.2.5`)
        *   IPv6 address: `2001:db8::6/126` (Default Gateway: `2001:db8::5`)
    *   **Scaled Static Routes on DUT (`DEFAULT` network-instance)**:
        *   **Forward Direction (Egressing `Ethernet5/1/1` toward ATE Port 2)**:
            *   16 IPv4 `/24` prefixes: `198.51.100.0/24` through
                `198.51.115.0/24` (where `198.51.100.0/24` is the primary
                canonical prefix), each configured with next-hop index `"0"` and
                next-hop IP `192.0.2.6`.
            *   16 IPv6 `/64` prefixes: `2001:db8:2::/64` (primary canonical
                prefix) and `2001:db8:2:1::/64` through `2001:db8:2:f::/64`,
                each configured with next-hop index `"0"` and next-hop IP
                `2001:db8::6`.
        *   **Reverse Direction (Egressing `Ethernet4/1/1` toward ATE Port 1)**:
            *   16 IPv4 `/24` prefixes: `203.0.113.0/24` through
                `203.0.128.0/24`, each configured with next-hop index `"0"` and
                next-hop IP `192.0.2.2`.
            *   16 IPv6 `/64` prefixes: `2001:db8:1::/64` and
                `2001:db8:1:1::/64` through `2001:db8:1:f::/64`, each configured
                with next-hop index `"0"` and next-hop IP `2001:db8::2`.

### TRANSCEIVER-109.1: Validate 800ZR operational-mode transition with transceiver bounce and bidirectional dual-stack traffic

Some 800G ZR/ZR+ optical pluggables can experience transient module faults
during initial boot when the default application selection (`Appsel`) mode is
applied, and subsequently fail to lock when reconfigured to a custom
`operational-mode` unless the transceiver state is bounced. This subtest
verifies that transitioning from the default `operational-mode` (`1`) to a
custom `operational-mode` (`2`) accompanied by a deterministic transceiver state
bounce cleanly initializes the optical channel on both line cards/ports,
preserves all hardware inventory telemetry, brings the 800G interfaces `UP`, and
forwards bidirectional IPv4 and IPv6 traffic with zero packet loss and zero
uncorrectable FEC errors.

*   **Step 1 - Initial Default Operational Mode Provisioning (Both Ports)**:
    *   Using a batched `gNMI.Set` request, enable the physical transceivers on
        both `Ethernet4/1` and `Ethernet5/1`:
        *   `/components/component[name=Ethernet4/1]/transceiver/config/enabled`:
            `true`
        *   `/components/component[name=Ethernet5/1]/transceiver/config/enabled`:
            `true`
    *   Configure 800G breakout parameters on hardware ports `Ethernet4/1-Port`
        and `Ethernet5/1-Port`:
        *   `/components/component/port/breakout-mode/groups/group/config/index`:
            `1`
        *   `/components/component/port/breakout-mode/groups/group/config/breakout-speed`:
            `SPEED_800GB`
        *   `/components/component/port/breakout-mode/groups/group/config/num-breakouts`:
            `1`
        *   `/components/component/port/breakout-mode/groups/group/config/num-physical-channels`:
            `8`
    *   Configure interfaces `Ethernet4/1/1` and `Ethernet5/1/1` via `gNMI.Set`:
        *   `/interfaces/interface/config/enabled`: `true`
        *   `/interfaces/interface/config/type`: `ethernetCsmacd`
        *   `/interfaces/interface/ethernet/config/port-speed`: `SPEED_800GB`
        *   `/interfaces/interface/ethernet/config/duplex-mode`: `FULL`
    *   Configure optical channels `Ethernet4/1-Optical0` and
        `Ethernet5/1-Optical0` with initial default operational mode `1`, center
        frequency `"196100000"` MHz (196.1 THz), and target output power `"-7"`
        dBm:
        *   `/components/component/optical-channel/config/operational-mode`: `1`
        *   `/components/component/optical-channel/config/frequency`:
            `"196100000"`
        *   `/components/component/optical-channel/config/target-output-power`:
            `"-7"`
    *   Configure the logical-channel hierarchy under
        `/terminal-device/logical-channels/channel` for both ports:
        *   **Port 1 (`Ethernet4/1/1`)**: Coherent OTN logical channel index
            `8000` (`logical-channel-type`: `PROT_OTN`, `admin-state`:
            `ENABLED`, assignment index `1` with `assignment-type`:
            `OPTICAL_CHANNEL`, `optical-channel`: `"Ethernet4/1-Optical0"`,
            `allocation`: `"800"`) and Ethernet logical channel index `80000`
            (`logical-channel-type`: `PROT_ETHERNET`, `rate-class`:
            `TRIB_RATE_800G`, `trib-protocol`: `PROT_800GE`, `admin-state`:
            `ENABLED`, ingress `interface`: `"Ethernet4/1/1"`, ingress
            `transceiver`: `"Ethernet4/1"`, assignment index `1` with
            `assignment-type`: `LOGICAL_CHANNEL`, `logical-channel`: `8000`,
            `allocation`: `"800"`).
        *   **Port 2 (`Ethernet5/1/1`)**: Coherent OTN logical channel index
            `8001` (`logical-channel-type`: `PROT_OTN`, `admin-state`:
            `ENABLED`, assignment index `1` with `assignment-type`:
            `OPTICAL_CHANNEL`, `optical-channel`: `"Ethernet5/1-Optical0"`,
            `allocation`: `"800"`) and Ethernet logical channel index `80001`
            (`logical-channel-type`: `PROT_ETHERNET`, `rate-class`:
            `TRIB_RATE_800G`, `trib-protocol`: `PROT_800GE`, `admin-state`:
            `ENABLED`, ingress `interface`: `"Ethernet5/1/1"`, ingress
            `transceiver`: `"Ethernet5/1"`, assignment index `1` with
            `assignment-type`: `LOGICAL_CHANNEL`, `logical-channel`: `8001`,
            `allocation`: `"800"`).
*   **Step 2 - Baseline Transceiver Inventory Verification (`gNMI.Subscribe` &
    `gNMI.Get`)**:
    *   Establish an `ON_CHANGE` `gNMI.Subscribe` session and perform `gNMI.Get`
        queries on `/components/component[name=Ethernet4/1]` and
        `/components/component[name=Ethernet5/1]` to verify and record baseline
        non-empty values for all inventory leaves on both transceivers:
        *   `/components/component/state/name` (`"Ethernet4/1"`,
            `"Ethernet5/1"`)
        *   `/components/component/state/type` (`TRANSCEIVER`)
        *   `/components/component/state/parent` (non-empty parent port/FPC
            component string)
        *   `/components/component/state/oper-status` (`ACTIVE`)
        *   `/components/component/transceiver/state/enabled` (`true`)
        *   `/components/component/transceiver/state/form-factor` (e.g., `OSFP`)
        *   `/components/component/state/serial-no` (non-empty string)
        *   `/components/component/state/part-no` (non-empty string)
        *   `/components/component/state/mfg-name` (non-empty string)
        *   `/components/component/state/mfg-date` (non-empty string)
        *   `/components/component/state/hardware-version` (non-empty string)
        *   `/components/component/state/firmware-version` (non-empty string)
        *   `/interfaces/interface[name=Ethernet4/1/1]/state/transceiver`
            (`"Ethernet4/1"`)
        *   `/interfaces/interface[name=Ethernet5/1/1]/state/transceiver`
            (`"Ethernet5/1"`)
        *   `/terminal-device/logical-channels/channel[index=80000]/ingress/state/interface`
            (`"Ethernet4/1/1"`)
        *   `/terminal-device/logical-channels/channel[index=80000]/ingress/state/transceiver`
            (`"Ethernet4/1"`)
        *   `/terminal-device/logical-channels/channel[index=80001]/ingress/state/interface`
            (`"Ethernet5/1/1"`)
        *   `/terminal-device/logical-channels/channel[index=80001]/ingress/state/transceiver`
            (`"Ethernet5/1"`)
*   **Step 3 - Reconfigure to Custom Operational Mode (`Appsel`) and Execute
    Event-Driven Transceiver Bounce**:
    *   Using a batched `gNMI.Set`, update
        `/components/component[name=Ethernet4/1-Optical0]/optical-channel/config/operational-mode`
        and
        `/components/component[name=Ethernet5/1-Optical0]/optical-channel/config/operational-mode`
        from `1` to `2` (or the platform-specific custom 800ZR operational mode)
        on both DUT ports and their peer ATE ports.
    *   Bounce both transceivers using event-driven `gNMI.Subscribe`
        (`ON_CHANGE`) state gates (do **not** use fixed sleep timers):
        1.  Set
            `/components/component[name=Ethernet4/1]/transceiver/config/enabled`
            and
            `/components/component[name=Ethernet5/1]/transceiver/config/enabled`
            to `false` via `gNMI.Set`.
        2.  Wait via `gNMI.Subscribe` (`ON_CHANGE`) until
            `/components/component/transceiver/state/enabled` is `false` on both
            `Ethernet4/1` and `Ethernet5/1` and
            `/interfaces/interface/state/oper-status` on both `Ethernet4/1/1`
            and `Ethernet5/1/1` transitions to `DOWN` (or `LOWER_LAYER_DOWN`).
        3.  Set
            `/components/component[name=Ethernet4/1]/transceiver/config/enabled`
            and
            `/components/component[name=Ethernet5/1]/transceiver/config/enabled`
            back to `true` via `gNMI.Set`.
    *   Wait via `gNMI.Subscribe` (`ON_CHANGE` and `SAMPLE`) and verify that:
        *   `/components/component/optical-channel/state/operational-mode`
            reports `2` (the configured custom operational mode) on both
            `Ethernet4/1-Optical0` and `Ethernet5/1-Optical0`.
        *   `/components/component/optical-channel/state/frequency` reports
            `196100000` and
            `/components/component/optical-channel/state/target-output-power`
            reports `-7.0` (within +/- 1.0 dBm).
        *   `/components/component/state/oper-status` for both `Ethernet4/1` and
            `Ethernet5/1` transitions to `ACTIVE`,
            `/components/component/transceiver/state/enabled` is `true`, and all
            inventory leaves (`serial-no`, `part-no`, `mfg-name`, `mfg-date`,
            `firmware-version`, `hardware-version`) remain populated and match
            the exact strings recorded in Step 2.
        *   `/interfaces/interface/state/oper-status` on both `Ethernet4/1/1`
            and `Ethernet5/1/1` transitions to `UP`.
        *   `/components/component/optical-channel/state/input-power/instant`
            (between `-15.0` dBm and `+2.0` dBm) and
            `/components/component/optical-channel/state/output-power/instant`
            (within +/- 1.0 dBm of `-7.0` dBm) report valid `decimal64` optical
            power levels on both optical channels.
        *   `/terminal-device/logical-channels/channel[index=8000]/otn/state/pre-fec-ber/instant`
            and
            `/terminal-device/logical-channels/channel[index=8001]/otn/state/pre-fec-ber/instant`
            report valid pre-FEC BER values below the FEC limit, and
            `/terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks`
            is `0`.
*   **Step 4 - Event-Driven Control-Plane/AFT Convergence and Bidirectional
    Dual-Stack Traffic Validation**:
    *   Configure the IPv4 and IPv6 subinterface addresses and the 32 IPv4 + 32
        IPv6 static routes on the DUT via `gNMI.Set` as specified in **Test
        environment setup**.
    *   **Pre-Traffic gNMI Convergence Gate (No Sleep Timers)**: Before starting
        traffic, wait via `gNMI.Subscribe` (`ON_CHANGE`) until:
        *   `/interfaces/interface[name=Ethernet4/1/1]/subinterfaces/subinterface[index=0]/ipv4/addresses/address[ip=192.0.2.1]/state/ip`
            == `"192.0.2.1"` and
            `/interfaces/interface[name=Ethernet5/1/1]/subinterfaces/subinterface[index=0]/ipv4/addresses/address[ip=192.0.2.5]/state/ip`
            == `"192.0.2.5"`.
        *   `/interfaces/interface[name=Ethernet4/1/1]/subinterfaces/subinterface[index=0]/ipv6/addresses/address[ip=2001:db8::1]/state/ip`
            == `"2001:db8::1"` and
            `/interfaces/interface[name=Ethernet5/1/1]/subinterfaces/subinterface[index=0]/ipv6/addresses/address[ip=2001:db8::5]/state/ip`
            == `"2001:db8::5"`.
        *   ARP and ND neighbor entries are resolved and present in
            `/interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/state/link-layer-address`
            (for `192.0.2.2` and `192.0.2.6`) and
            `/interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/link-layer-address`
            (for `2001:db8::2` and `2001:db8::6`).
        *   OTG neighbor resolution is verified using the standard
            `otgutils.WaitForARP` helper for both IPv4 and IPv6 before starting
            traffic.
        *   All 32 IPv4 static prefixes (`198.51.100.0/24`–`198.51.115.0/24` and
            `203.0.113.0/24`–`203.0.128.0/24`) are installed in AFT at
            `/network-instances/network-instance[name=DEFAULT]/afts/ipv4-unicast/ipv4-entry/state/prefix`,
            and all 32 IPv6 static prefixes (`2001:db8:2::/64`–`2001:db8:2:f::/64`
            and `2001:db8:1::/64`–`2001:db8:1:f::/64`) are installed in AFT at
            `/network-instances/network-instance[name=DEFAULT]/afts/ipv6-unicast/ipv6-entry/state/prefix`.
    *   Record baseline counter values on `Ethernet4/1/1` and `Ethernet5/1/1`
        via `gNMI.Get` for `in-pkts`, `out-pkts`, `in-unicast-pkts`,
        `out-unicast-pkts`, `in-errors`, `in-fcs-errors`, `out-errors`, and OTN
        `fec-uncorrectable-blocks`.
    *   **Bidirectional IPv4 Traffic Flow**:
        *   **Flow 1A (`ATE Port 1 -> ATE Port 2`)**: Source IP `192.0.2.2`,
            Destination IPs cycling uniformly across the 16 prefixes
            `198.51.100.1` through `198.51.115.1`, frame size `512` bytes, rate
            `10,000` pps, duration `10` seconds (`100,000` packets total).
        *   **Flow 1B (`ATE Port 2 -> ATE Port 1`)**: Source IP `192.0.2.6`,
            Destination IPs cycling uniformly across the 16 prefixes
            `203.0.113.1` through `203.0.128.1`, frame size `512` bytes, rate
            `10,000` pps, duration `10` seconds (`100,000` packets total).
    *   **Bidirectional IPv6 Traffic Flow**:
        *   **Flow 2A (`ATE Port 1 -> ATE Port 2`)**: Source IP `2001:db8::2`,
            Destination IPs cycling uniformly across the 16 prefixes
            `2001:db8:2::1` through `2001:db8:2:f::1`, frame size `512` bytes,
            rate `10,000` pps, duration `10` seconds (`100,000` packets total).
        *   **Flow 2B (`ATE Port 2 -> ATE Port 1`)**: Source IP `2001:db8::6`,
            Destination IPs cycling uniformly across the 16 prefixes
            `2001:db8:1::1` through `2001:db8:1:f::1`, frame size `512` bytes,
            rate `10,000` pps, duration `10` seconds (`100,000` packets total).
    *   **Pass/Fail Criteria**:
        *   All four flows (Flow 1A, Flow 1B, Flow 2A, Flow 2B) complete with
            **0% packet loss** (`rx_pkts == tx_pkts == 100,000` per flow).
        *   On both `Ethernet4/1/1` and `Ethernet5/1/1`,
            `/interfaces/interface/state/counters/in-unicast-pkts` and
            `/interfaces/interface/state/counters/out-unicast-pkts` (as well as
            `in-pkts` and `out-pkts`) increment by at least `200,000` packets.
        *   `/interfaces/interface/state/counters/in-errors`,
            `/interfaces/interface/state/counters/in-fcs-errors`,
            `/interfaces/interface/state/counters/out-errors`, and
            `/terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks`
            deltas are strictly `0`.

#### Canonical OC

```json
{
  "openconfig-interfaces:interfaces": {
    "interface": [
      {
        "config": {
          "enabled": true,
          "name": "Ethernet4/1/1",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "ethernet": {
          "config": {
            "duplex-mode": "FULL",
            "port-speed": "SPEED_800GB"
          }
        },
        "name": "Ethernet4/1/1",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "openconfig-if-ip:ipv4": {
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
              "openconfig-if-ip:ipv6": {
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
          "enabled": true,
          "name": "Ethernet5/1/1",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "ethernet": {
          "config": {
            "duplex-mode": "FULL",
            "port-speed": "SPEED_800GB"
          }
        },
        "name": "Ethernet5/1/1",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "openconfig-if-ip:ipv4": {
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
              "openconfig-if-ip:ipv6": {
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
  "openconfig-network-instance:network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "DEFAULT"
        },
        "name": "DEFAULT",
        "protocols": {
          "protocol": [
            {
              "config": {
                "identifier": "STATIC",
                "name": "STATIC"
              },
              "identifier": "STATIC",
              "name": "STATIC",
              "static-routes": {
                "static": [
                  {
                    "config": {
                      "prefix": "198.51.100.0/24"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "config": {
                            "index": "0",
                            "next-hop": "192.0.2.6"
                          },
                          "index": "0"
                        }
                      ]
                    },
                    "prefix": "198.51.100.0/24"
                  },
                  {
                    "config": {
                      "prefix": "2001:db8:2::/64"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "config": {
                            "index": "0",
                            "next-hop": "2001:db8::6"
                          },
                          "index": "0"
                        }
                      ]
                    },
                    "prefix": "2001:db8:2::/64"
                  }
                ]
              }
            }
          ]
        }
      }
    ]
  },
  "openconfig-platform:components": {
    "component": [
      {
        "config": {
          "name": "Ethernet4/1-Port"
        },
        "name": "Ethernet4/1-Port",
        "openconfig-platform-port:port": {
          "breakout-mode": {
            "groups": {
              "group": [
                {
                  "config": {
                    "breakout-speed": "openconfig-if-ethernet:SPEED_800GB",
                    "index": 1,
                    "num-breakouts": 1,
                    "num-physical-channels": 8
                  },
                  "index": 1
                }
              ]
            }
          }
        }
      },
      {
        "config": {
          "name": "Ethernet4/1"
        },
        "name": "Ethernet4/1",
        "openconfig-platform-transceiver:transceiver": {
          "config": {
            "enabled": true
          }
        }
      },
      {
        "config": {
          "name": "Ethernet4/1-Optical0"
        },
        "name": "Ethernet4/1-Optical0",
        "openconfig-terminal-device:optical-channel": {
          "config": {
            "frequency": "196100000",
            "operational-mode": 1,
            "target-output-power": "-7"
          }
        }
      }
    ]
  },
  "openconfig-terminal-device:terminal-device": {
    "logical-channels": {
      "channel": [
        {
          "config": {
            "admin-state": "ENABLED",
            "description": "OTN Logical Channel",
            "index": 8000,
            "logical-channel-type": "openconfig-transport-types:PROT_OTN"
          },
          "index": 8000,
          "logical-channel-assignments": {
            "assignment": [
              {
                "config": {
                  "allocation": "800",
                  "assignment-type": "OPTICAL_CHANNEL",
                  "description": "OTN to optical channel assignment",
                  "index": 1,
                  "optical-channel": "Ethernet4/1-Optical0"
                },
                "index": 1
              }
            ]
          }
        },
        {
          "config": {
            "admin-state": "ENABLED",
            "description": "ETH Logical Channel",
            "index": 80000,
            "logical-channel-type": "openconfig-transport-types:PROT_ETHERNET",
            "rate-class": "openconfig-transport-types:TRIB_RATE_800G",
            "trib-protocol": "openconfig-transport-types:PROT_800GE"
          },
          "index": 80000,
          "ingress": {
            "config": {
              "interface": "Ethernet4/1/1",
              "transceiver": "Ethernet4/1"
            }
          },
          "logical-channel-assignments": {
            "assignment": [
              {
                "config": {
                  "allocation": "800",
                  "assignment-type": "LOGICAL_CHANNEL",
                  "description": "ETH to OTN assignment",
                  "index": 1,
                  "logical-channel": 8000
                },
                "index": 1
              }
            ]
          }
        }
      ]
    }
  }
}
```

### TRANSCEIVER-109.2: Validate scaled multi-port 800ZR transceiver inventory persistence and link recovery across repeated interface delete and re-create cycles

When interfaces associated with 800G ZR/ZR+ optics across multiple line cards
are deleted and re-created, the device software database must retain the
physical transceiver component inventory rather than purging the optic record
(`no-info` / missing from `/components/component`). Building directly on the
established state of `TRANSCEIVER-109.1`, this subtest executes 5 consecutive
cycles of simultaneous batched `gNMI.Set` interface deletion and re-creation
across both 800G ZR interfaces (`Ethernet4/1/1` and `Ethernet5/1/1`), verifying
continuous hardware inventory presence via `ON_CHANGE` `gNMI.Subscribe` and
full bidirectional dual-stack L3 traffic recovery.

*   **Step 1 - Maintain Continuous `ON_CHANGE` Inventory Subscription**:
    *   Starting from the active state of `TRANSCEIVER-109.1`, keep an active
        `ON_CHANGE` `gNMI.Subscribe` subscription monitoring
        `/components/component[name=Ethernet4/1]/state/` and
        `/components/component[name=Ethernet5/1]/state/` and verify that the
        current values match the baseline strings recorded in
        `TRANSCEIVER-109.1` Step 2 (`name`, `type`, `parent`, `oper-status`,
        `serial-no`, `part-no`, `mfg-name`, `mfg-date`, `hardware-version`,
        `firmware-version`).
*   **Step 2 - Execute 5 Consecutive Multi-Port Interface Delete and Re-Create
    Cycles (`Iteration i = 1..5`)**:
    *   For each iteration $i \in \{1, 2, 3, 4, 5\}$:
        1.  **Batched Interface Delete via `gNMI.Set`**:
            *   Issue a single batched `gNMI.Set` (`Delete`) request removing
                `/interfaces/interface[name=Ethernet4/1/1]`,
                `/interfaces/interface[name=Ethernet5/1/1]`,
                `/terminal-device/logical-channels/channel[index=80000]/ingress/config/interface`,
                and
                `/terminal-device/logical-channels/channel[index=80001]/ingress/config/interface`,
                while keeping both physical transceivers enabled
                (`/components/component[name=Ethernet4/1]/transceiver/config/enabled`
                = `true` and
                `/components/component[name=Ethernet5/1]/transceiver/config/enabled`
                = `true`).
        2.  **Verify Transceiver Inventory Persistence While Interfaces Are
            Deleted**:
            *   Inspect the active `ON_CHANGE` `gNMI.Subscribe` stream and query
                `/components/component[name=Ethernet4/1]/state/` and
                `/components/component[name=Ethernet5/1]/state/` via `gNMI.Get`.
            *   **Pass/Fail Criteria (Per Delete Cycle)**:
                *   No gNMI delete notification is emitted for
                    `/components/component[name=Ethernet4/1]`,
                    `/components/component[name=Ethernet5/1]`, or any of their
                    inventory leaves.
                *   `/components/component/state/name`,
                    `/components/component/state/type` (`TRANSCEIVER`),
                    `/components/component/state/parent`,
                    `/components/component/state/serial-no`,
                    `/components/component/state/part-no`,
                    `/components/component/state/mfg-name`,
                    `/components/component/state/mfg-date`,
                    `/components/component/state/hardware-version`, and
                    `/components/component/state/firmware-version` remain
                    populated on both `Ethernet4/1` and `Ethernet5/1` and match
                    the exact baseline strings from `TRANSCEIVER-109.1` Step 2.
        3.  **Batched Interface Re-Creation and Custom Operational Mode
            Verification**:
            *   Using a batched `gNMI.Set`, re-create
                `/interfaces/interface[name=Ethernet4/1/1]` (`SPEED_800GB`,
                `192.0.2.1/30`, `2001:db8::1/126`) and
                `/interfaces/interface[name=Ethernet5/1/1]` (`SPEED_800GB`,
                `192.0.2.5/30`, `2001:db8::5/126`), re-bind
                `/terminal-device/logical-channels/channel[index=80000]/ingress/config/interface`
                to `"Ethernet4/1/1"` and
                `/terminal-device/logical-channels/channel[index=80001]/ingress/config/interface`
                to `"Ethernet5/1/1"`, and ensure
                `/components/component[name=Ethernet4/1-Optical0]/optical-channel/config/operational-mode`
                and
                `/components/component[name=Ethernet5/1-Optical0]/optical-channel/config/operational-mode`
                are set to the custom operational mode (`2`).
            *   Wait via `gNMI.Subscribe` (`ON_CHANGE`) until:
                *   `/interfaces/interface[name=Ethernet4/1/1]/state/oper-status`
                    == `UP` and
                    `/interfaces/interface[name=Ethernet5/1/1]/state/oper-status`
                    == `UP`.
                *   `/interfaces/interface[name=Ethernet4/1/1]/state/transceiver`
                    == `"Ethernet4/1"` and
                    `/interfaces/interface[name=Ethernet5/1/1]/state/transceiver`
                    == `"Ethernet5/1"`.
                *   `/components/component[name=Ethernet4/1]/state/oper-status`
                    == `ACTIVE` and
                    `/components/component[name=Ethernet5/1]/state/oper-status`
                    == `ACTIVE`.
*   **Step 3 - Post-Stress Bidirectional Dual-Stack IPv4 and IPv6 Traffic
    Validation**:
    *   Wait via `gNMI.Subscribe` (`ON_CHANGE`) for ARP (`192.0.2.2`,
        `192.0.2.6`), IPv6 ND (`2001:db8::2`, `2001:db8::6`), and all 32 IPv4 +
        32 IPv6 AFT entries to converge in `DEFAULT`.
    *   Re-run the bidirectional IPv4 flows (Flow 1A and Flow 1B, `100,000`
        packets each) and bidirectional IPv6 flows (Flow 2A and Flow 2B,
        `100,000` packets each).
    *   **Pass/Fail Criteria**: All four flows complete with **0% packet loss**,
        interface unicast packet counters increment by at least `200,000`
        packets per interface, and interface error counters (`in-errors`,
        `in-fcs-errors`, `out-errors`) and OTN `fec-uncorrectable-blocks` deltas
        remain `0`.

### TRANSCEIVER-109.3: Negative Validation — Unsupported Operational Mode, Optical Mismatch Fault Recovery, and Laser Shutoff

Building on the active state from `TRANSCEIVER-109.2`, this subtest validates
that negative optical configurations and fault conditions do not purge
transceiver hardware inventory from `/components/component` and that the 800G ZR
transceivers cleanly recover via custom `operational-mode` provisioning and
transceiver state bounce.

*   **Step 1 - Negative Case 1: Unsupported / Out-of-Range `operational-mode`
    (`Appsel`) Handling**:
    *   Using `gNMI.Set`, attempt to configure an unsupported `operational-mode`
        value (`65535`) on
        `/components/component[name=Ethernet4/1-Optical0]/optical-channel/config/operational-mode`.
    *   **Pass/Fail Criteria**:
        *   The DUT must either reject the `gNMI.Set` request with a gRPC/gNMI
            error (`INVALID_ARGUMENT`) or transition the optical channel out of
            service without crashing the optics daemon.
        *   Query `/components/component[name=Ethernet4/1]/state/` via
            `gNMI.Get` and inspect the `ON_CHANGE` `gNMI.Subscribe` stream:
            verify that `Ethernet4/1` is **not** deleted from
            `/components/component` and that `serial-no`, `part-no`, `mfg-name`,
            `mfg-date`, `hardware-version`, and `firmware-version` remain
            populated and identical to baseline values.
*   **Step 2 - Negative Case 2: Optical Frequency / Mode Mismatch Fault
    Simulation and Subsequent Bounce Recovery**:
    *   Using `gNMI.Set`, configure a mismatched center frequency of
        `"193100000"` MHz (`193.1` THz) on
        `/components/component[name=Ethernet4/1-Optical0]/optical-channel/config/frequency`
        while the peer ATE port remains at `"196100000"` MHz (`196.1` THz),
        forcing an optical loss-of-lock fault on Link 1.
    *   Wait via `gNMI.Subscribe` (`ON_CHANGE`) for
        `/interfaces/interface[name=Ethernet4/1/1]/state/oper-status` to
        transition to `DOWN` (or `LOWER_LAYER_DOWN`).
    *   Verify via `gNMI.Get` that
        `/components/component[name=Ethernet4/1]/state/` retains all static
        transceiver inventory leaves (`serial-no`, `part-no`, `mfg-name`,
        `mfg-date`, `hardware-version`, `firmware-version`) while in the optical
        fault state, and verify that traffic sent on Flow 1A and Flow 2A
        experiences 100% loss while Link 1 is down.
    *   **Recovery from Optical Fault**: Restore
        `/components/component[name=Ethernet4/1-Optical0]/optical-channel/config/frequency`
        to `"196100000"` MHz and
        `/components/component[name=Ethernet4/1-Optical0]/optical-channel/config/operational-mode`
        to the custom operational mode (`2`), and bounce the transceiver via
        `gNMI.Set` by setting
        `/components/component[name=Ethernet4/1]/transceiver/config/enabled` to
        `false`, waiting via `gNMI.Subscribe` for `transceiver/state/enabled` ==
        `false`, and setting `transceiver/config/enabled` to `true`.
    *   Wait via `gNMI.Subscribe` (`ON_CHANGE`) for
        `/interfaces/interface[name=Ethernet4/1/1]/state/oper-status` to
        transition back to `UP` and verify 0% packet loss across Flows 1A, 1B,
        2A, and 2B.
*   **Step 3 - Negative Case 3: Administrative Transceiver Disable Laser Shutoff
    and Inventory Retention**:
    *   Using `gNMI.Set`, set
        `/components/component[name=Ethernet4/1]/transceiver/config/enabled` to
        `false` while keeping
        `/interfaces/interface[name=Ethernet4/1/1]/config/enabled` set to
        `true`.
    *   Wait via `gNMI.Subscribe` for
        `/components/component[name=Ethernet4/1]/transceiver/state/enabled` to
        report `false` and
        `/interfaces/interface[name=Ethernet4/1/1]/state/oper-status` to report
        `DOWN`.
    *   **Pass/Fail Criteria**:
        *   Verify via `gNMI.Get` that
            `/components/component[name=Ethernet4/1-Optical0]/optical-channel/state/output-power/instant`
            drops below `-30.0` dBm (confirming laser shutoff).
        *   Verify via `gNMI.Get` that all static transceiver inventory leaves
            (`serial-no`, `part-no`, `mfg-name`, `mfg-date`, `hardware-version`,
            `firmware-version`) remain present in
            `/components/component[name=Ethernet4/1]/state/`.
    *   **Final Recovery Verification**: Re-enable
        `/components/component[name=Ethernet4/1]/transceiver/config/enabled` to
        `true` via `gNMI.Set`, wait via `gNMI.Subscribe` for
        `/interfaces/interface[name=Ethernet4/1/1]/state/oper-status` == `UP`,
        `/components/component[name=Ethernet4/1-Optical0]/optical-channel/state/output-power/instant`
        within +/- 1.0 dBm of `-7.0` dBm, ARP/ND resolution, and verify 0%
        packet loss across all bidirectional IPv4 and IPv6 flows.

### Cleanup

*   Revert all modified optical-channel `operational-mode`, `frequency`,
    logical-channel, subinterface IPv4/IPv6, and static route configurations to
    the baseline state using `gNMI.Set` (registered via `t.Cleanup()` in test
    automation), and verify via `gNMI.Get` that all tested transceivers and
    interfaces return to their pre-test operational state.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  # Config Parameter coverage
  /components/component/optical-channel/config/frequency:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/config/operational-mode:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/config/target-output-power:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/port/breakout-mode/groups/group/config/breakout-speed:
    platform_type: ["PORT"]
  /components/component/port/breakout-mode/groups/group/config/index:
    platform_type: ["PORT"]
  /components/component/port/breakout-mode/groups/group/config/num-breakouts:
    platform_type: ["PORT"]
  /components/component/port/breakout-mode/groups/group/config/num-physical-channels:
    platform_type: ["PORT"]
  /components/component/transceiver/config/enabled:
    platform_type: ["TRANSCEIVER"]
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/type:
  /interfaces/interface/ethernet/config/duplex-mode:
  /interfaces/interface/ethernet/config/port-speed:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop:
  /terminal-device/logical-channels/channel/config/admin-state:
  /terminal-device/logical-channels/channel/config/description:
  /terminal-device/logical-channels/channel/config/index:
  /terminal-device/logical-channels/channel/config/logical-channel-type:
  /terminal-device/logical-channels/channel/config/rate-class:
  /terminal-device/logical-channels/channel/config/trib-protocol:
  /terminal-device/logical-channels/channel/ingress/config/interface:
  /terminal-device/logical-channels/channel/ingress/config/transceiver:
  /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/allocation:
  /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/assignment-type:
  /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/description:
  /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/index:
  /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/logical-channel:
  /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/optical-channel:

  # Telemetry Parameter coverage
  /components/component/optical-channel/state/frequency:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/input-power/instant:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/operational-mode:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/output-power/instant:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/target-output-power:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/state/firmware-version:
    platform_type: ["TRANSCEIVER"]
  /components/component/state/hardware-version:
    platform_type: ["TRANSCEIVER"]
  /components/component/state/mfg-date:
    platform_type: ["TRANSCEIVER"]
  /components/component/state/mfg-name:
    platform_type: ["TRANSCEIVER"]
  /components/component/state/name:
    platform_type: ["PORT", "TRANSCEIVER", "OPTICAL_CHANNEL"]
  /components/component/state/oper-status:
    platform_type: ["TRANSCEIVER"]
  /components/component/state/parent:
    platform_type: ["PORT", "TRANSCEIVER", "OPTICAL_CHANNEL"]
  /components/component/state/part-no:
    platform_type: ["TRANSCEIVER"]
  /components/component/state/serial-no:
    platform_type: ["TRANSCEIVER"]
  /components/component/state/type:
    platform_type: ["PORT", "TRANSCEIVER", "OPTICAL_CHANNEL"]
  /components/component/transceiver/state/enabled:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/state/form-factor:
    platform_type: ["TRANSCEIVER"]
  /interfaces/interface/state/counters/in-errors:
  /interfaces/interface/state/counters/in-fcs-errors:
  /interfaces/interface/state/counters/in-pkts:
  /interfaces/interface/state/counters/in-unicast-pkts:
  /interfaces/interface/state/counters/out-errors:
  /interfaces/interface/state/counters/out-pkts:
  /interfaces/interface/state/counters/out-unicast-pkts:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/state/transceiver:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/state/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/state/link-layer-address:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/state/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/link-layer-address:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /terminal-device/logical-channels/channel/ingress/state/interface:
  /terminal-device/logical-channels/channel/ingress/state/transceiver:
  /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks:
  /terminal-device/logical-channels/channel/otn/state/pre-fec-ber/instant:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```

## Required DUT platform

*   FFF
