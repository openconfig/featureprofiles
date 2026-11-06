# TRANSCEIVER-10.1: Telemetry: 400ZR Optics FEC(Forward Error Correction) Uncorrectable Frames Streaming.

## Summary

Validate 400ZR optics module reports uncorrectable FEC frames count.

This observable represents the number of uncorrectable FEC frames,
measured as RS(544,514) equivalent frames, in a short interval.
This is a post-FEC decoder error metric.

## Procedure

*   Connect two ZR interfaces using a duplex LC fiber jumper such that TX
    output power of one is the RX input power of the other module.
*   To establish a point to point ZR link ensure the following:
      * Both transceivers state is enabled
      * Both transceivers are set to a valid target TX output power
        example -10 dBm
      * Both transceivers are tuned to a valid centre frequency
        example 193.1 THz
*   With the ZR link established as explained above, verify that the
    following ZR transceiver telemetry path exist and is streamed for both
    the ZR optics.
    *   /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks
*   Verify that the reported data should be of type yang:counter64.
*   When the modules or the devices are still in a boot stage, they must not
    stream any invalid string values like "nil" or "-inf".
*   Toggle the interface state using /interfaces/interface/config/enabled and
    verify relevant FEC uncorrectable frame count is streamed. If there are no
    errors a value of 0 should be streamed for no FEC uncorrectable frames.

### Topology and Test Environment Setup

*   Connect two 400ZR optical interfaces on the DUT (`DUT port 1` named
    `Ethernet1` and `DUT port 2` named `Ethernet2`) in a point-to-point
    loopback topology using a duplex LC fiber jumper such that the TX output
    power of `DUT port 1` (`Ethernet1`) feeds the RX input power of `DUT port 2`
    (`Ethernet2`), and the TX output power of `DUT port 2` (`Ethernet2`) feeds
    the RX input power of `DUT port 1` (`Ethernet1`), consistent with
    [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed).
*   Associate `DUT port 1` (`Ethernet1`) with transceiver component
    `Ethernet1-Transceiver`, optical channel component `Ethernet1-Optical0`,
    OTN logical channel `4000`, and Ethernet logical channel `40000`.
*   Associate `DUT port 2` (`Ethernet2`) with transceiver component
    `Ethernet2-Transceiver`, optical channel component `Ethernet2-Optical0`,
    OTN logical channel `4001`, and Ethernet logical channel `40001`.

### TRANSCEIVER-10.1.1: Validate 400ZR FEC PM Interval Semantics

This subtest validates that the OpenConfig FEC uncorrectable blocks telemetry
leafs report the cumulative count over the prior Performance Monitoring (PM)
interval defined by the host (`yang:gauge64` semantics), rather than retaining
stale historical counts across subsequent error-free PM intervals after link
flaps or transceiver resets.

1.  **Configure 400ZR Interfaces, Optical Channels, and Logical Channels**:
    *   Using `gNMI.Set` (`REPLACE`), configure `DUT port 1` (`Ethernet1`) and
        `DUT port 2` (`Ethernet2`) with administrative state `ENABLED` at
        `/interfaces/interface/config/enabled`, interface type `ethernetCsmacd`
        at `/interfaces/interface/config/type`, port speed `SPEED_400GB` at
        `/interfaces/interface/ethernet/config/port-speed`, and duplex mode
        `FULL` at `/interfaces/interface/ethernet/config/duplex-mode`.
    *   Configure dual-stack Layer 3 subinterfaces (`index 0`) on `DUT port 1`
        (`Ethernet1`) with IPv4 address `192.0.2.1/30` and IPv6 address
        `2001:db8::1/126`, and on `DUT port 2` (`Ethernet2`) with IPv4 address
        `192.0.2.2/30` and IPv6 address `2001:db8::2/126` using:
        *   `/interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip`
        *   `/interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length`
        *   `/interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip`
        *   `/interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length`
    *   Enable both 400ZR transceivers (`Ethernet1-Transceiver` and
        `Ethernet2-Transceiver`) by setting
        `/components/component/transceiver/config/enabled` to `true`.
    *   Configure optical channels `Ethernet1-Optical0` and `Ethernet2-Optical0`
        with center frequency `193100000` MHz (`193.1` THz) at
        `/components/component/optical-channel/config/frequency`, target TX
        output power `-10.0` dBm at
        `/components/component/optical-channel/config/target-output-power`, and
        operational mode `1` at
        `/components/component/optical-channel/config/operational-mode`.
    *   Configure terminal-device OTN logical channels (`index 4000` and
        `index 4001`) with `PROT_OTN` and `ENABLED` state, assigned to
        `Ethernet1-Optical0` and `Ethernet2-Optical0` respectively using:
        *   `/terminal-device/logical-channels/channel/config/logical-channel-type`
        *   `/terminal-device/logical-channels/channel/config/admin-state`
        *   `/terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/optical-channel`
    *   Configure terminal-device Ethernet logical channels (`index 40000` and
        `index 40001`) with `PROT_ETHERNET`, `TRIB_RATE_400G`, and `PROT_400GE`,
        mapped to `Ethernet1` and `Ethernet2` using:
        *   `/terminal-device/logical-channels/channel/config/logical-channel-type`
        *   `/terminal-device/logical-channels/channel/config/rate-class`
        *   `/terminal-device/logical-channels/channel/config/trib-protocol`
        *   `/terminal-device/logical-channels/channel/ingress/config/interface`
        *   `/terminal-device/logical-channels/channel/ingress/config/transceiver`
2.  **Verify Baseline Operational State and PM Interval Telemetry**:
    *   Use `gNMI.Subscribe` (`ON_CHANGE` / `SAMPLE`) to verify that
        `/interfaces/interface/state/oper-status` transitions to `UP` on both
        `DUT port 1` (`Ethernet1`) and `DUT port 2` (`Ethernet2`).
    *   Use `gNMI.Get` to verify that configured optical parameters are
        reflected in state at
        `/components/component/optical-channel/state/frequency`,
        `/components/component/optical-channel/state/target-output-power`, and
        `/components/component/optical-channel/state/operational-mode`.
    *   Subscribe (`gNMI.Subscribe` in `SAMPLE` mode with a 10-second PM
        sampling interval) to the FEC uncorrectable blocks telemetry paths on
        both ends of the 400ZR link:
        *   `/terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks`
        *   `/terminal-device/logical-channels/channel/ethernet/state/fec-uncorrectable-blocks`
        *   `/components/component/optical-channel/state/fec-uncorrectable-blocks`
        *   `/interfaces/interface/ethernet/state/counters/fec-uncorrectable-blocks`
        *   `/interfaces/interface/ethernet/state/counters/fec-corrected-blocks`
    *   Verify that over 3 consecutive 10-second PM intervals on the stable,
        error-free 400ZR link,
        `/terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks`
        consistently streams an unsigned 64-bit integer (`yang:gauge64`) value
        of `0`.
3.  **Induce Transient Link Disturbance and Verify PM Interval Reset**:
    *   Flap `DUT port 1` (`Ethernet1`) by setting
        `/interfaces/interface/config/enabled` to `false` via `gNMI.Set`,
        verifying `/interfaces/interface/state/oper-status` transitions to
        `DOWN`, waiting for 1 PM interval (10 seconds), and re-enabling
        `/interfaces/interface/config/enabled` to `true`.
    *   Wait for `/interfaces/interface/state/oper-status` on both `DUT port 1`
        (`Ethernet1`) and `DUT port 2` (`Ethernet2`) to return to `UP`.
    *   Repeat the transient disturbance by toggling the transceiver module
        power state on `DUT port 1` (`Ethernet1-Transceiver`) using
        `/components/component/transceiver/config/enabled` (`false` then `true`)
        and waiting for `/interfaces/interface/state/oper-status` to return to
        `UP`.
    *   Wait for at least 2 complete host Performance Monitoring (PM) intervals
        (20 seconds) after the link has stabilized in the `UP` state.
    *   **Pass/Fail Criteria**:
        *   **Pass**: Even if transient framing or codeword errors occurred
            during the optical lock/unlock transition, once the 400ZR link is
            stable and error-free for a full PM interval, the following paths
            MUST report `0` for each subsequent error-free PM interval,
            confirming that the leaf reflects the cumulative count over the
            prior PM interval rather than a non-resetting lifetime counter:
            *   `/terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks`
            *   `/terminal-device/logical-channels/channel/ethernet/state/fec-uncorrectable-blocks`
            *   `/components/component/optical-channel/state/fec-uncorrectable-blocks`
        *   **Fail**: The telemetry leaf continues to report a non-zero stale
            value (e.g., `4`) across subsequent error-free PM intervals after
            the link has stabilized, or streams invalid string values such as
            `"nil"` or `"-inf"`.

#### Canonical OC

```json
{
  "openconfig-interfaces:interfaces": {
    "interface": [
      {
        "config": {
          "enabled": true,
          "name": "Ethernet1",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "ethernet": {
          "config": {
            "duplex-mode": "FULL",
            "port-speed": "openconfig-if-ethernet:SPEED_400GB"
          }
        },
        "name": "Ethernet1",
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
                }
              }
            }
          ]
        }
      },
      {
        "config": {
          "enabled": true,
          "name": "Ethernet2",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "ethernet": {
          "config": {
            "duplex-mode": "FULL",
            "port-speed": "openconfig-if-ethernet:SPEED_400GB"
          }
        },
        "name": "Ethernet2",
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
                        "ip": "192.0.2.2",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.2"
                    }
                  ]
                }
              },
              "openconfig-if-ip:ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8::2",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8::2"
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
  "openconfig-platform:components": {
    "component": [
      {
        "config": {
          "name": "Ethernet1-Transceiver"
        },
        "name": "Ethernet1-Transceiver",
        "openconfig-platform-transceiver:transceiver": {
          "config": {
            "enabled": true
          }
        }
      },
      {
        "config": {
          "name": "Ethernet2-Transceiver"
        },
        "name": "Ethernet2-Transceiver",
        "openconfig-platform-transceiver:transceiver": {
          "config": {
            "enabled": true
          }
        }
      },
      {
        "config": {
          "name": "Ethernet1-Optical0"
        },
        "name": "Ethernet1-Optical0",
        "openconfig-terminal-device:optical-channel": {
          "config": {
            "frequency": "193100000",
            "operational-mode": 1,
            "target-output-power": "-10.0"
          }
        }
      },
      {
        "config": {
          "name": "Ethernet2-Optical0"
        },
        "name": "Ethernet2-Optical0",
        "openconfig-terminal-device:optical-channel": {
          "config": {
            "frequency": "193100000",
            "operational-mode": 1,
            "target-output-power": "-10.0"
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
            "description": "OTN Logical Channel 1",
            "index": 4000,
            "logical-channel-type": "openconfig-transport-types:PROT_OTN"
          },
          "index": 4000,
          "logical-channel-assignments": {
            "assignment": [
              {
                "config": {
                  "allocation": "400.0",
                  "assignment-type": "OPTICAL_CHANNEL",
                  "description": "OTN to optical channel assignment 1",
                  "index": 1,
                  "optical-channel": "Ethernet1-Optical0"
                },
                "index": 1
              }
            ]
          }
        },
        {
          "config": {
            "admin-state": "ENABLED",
            "description": "ETH Logical Channel 1",
            "index": 40000,
            "logical-channel-type": "openconfig-transport-types:PROT_ETHERNET",
            "rate-class": "openconfig-transport-types:TRIB_RATE_400G",
            "trib-protocol": "openconfig-transport-types:PROT_400GE"
          },
          "index": 40000,
          "ingress": {
            "config": {
              "interface": "Ethernet1",
              "transceiver": "Ethernet1-Transceiver"
            }
          },
          "logical-channel-assignments": {
            "assignment": [
              {
                "config": {
                  "allocation": "400.0",
                  "assignment-type": "LOGICAL_CHANNEL",
                  "description": "ETH to OTN assignment 1",
                  "index": 1,
                  "logical-channel": 4000
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

### TRANSCEIVER-10.1.2: Validate 400ZR Link Qualification and Dual-Stack Traffic

This subtest is self-contained and independent of prior testcases. It verifies
that executing packet-based Link Qualification and subsequent dual-stack (IPv4
and IPv6) forwarding on a 400ZR link succeeds without false-positive
`UncorrectableErrorsFrames` failures caused by historical prior-interval FEC
block counts.

1.  **Configure 400ZR Link and Verify gNOI Packet Link Qualification Capabilities**:
    *   Independently configure `DUT port 1` (`Ethernet1`) and `DUT port 2`
        (`Ethernet2`) with 400ZR interface parameters (`SPEED_400GB`, `FULL`,
        `ethernetCsmacd`), dual-stack Layer 3 subinterfaces (`192.0.2.1/30` ↔
        `192.0.2.2/30` and `2001:db8::1/126` ↔ `2001:db8::2/126`), static routes
        for `198.51.100.0/24` and `2001:db8:1::/64`, enabled transceivers
        (`Ethernet1-Transceiver`, `Ethernet2-Transceiver`), optical channels
        (`193100000` MHz, `-10.0` dBm, `operational-mode`), and OTN/Ethernet
        logical channels (`4000`/`40000` and `4001`/`40001`), and verify
        `/interfaces/interface/state/oper-status` reaches `UP` on both ports.
    *   Invoke `gnoi.packet_link_qualification.LinkQualification.Capabilities`
        on the DUT and verify that `DUT port 1` (`Ethernet1`) and `DUT port 2`
        (`Ethernet2`) support packet generator and reflector roles with
        `MaxBps >= 4e11` (400 Gbps) and `MaxMtu >= 8184`.
2.  **Execute gNOI Packet Link Qualification on the 400ZR Link**:
    *   Invoke `gnoi.packet_link_qualification.LinkQualification.Create` on
        `DUT port 1` (`Ethernet1`) configured as `NEAR_END`
        (`PacketGeneratorConfiguration`) with qualification ID
        `"qual-400zr-near"`, `PacketRate: 50000000` pps, `PacketSize: 8184`
        bytes, `SetupDuration: 5s`, `PreSyncDuration: 10s`, `Duration: 30s`,
        `PostSyncDuration: 5s`, and `TeardownDuration: 5s`.
    *   Invoke `gnoi.packet_link_qualification.LinkQualification.Create` on
        `DUT port 2` (`Ethernet2`) configured as `FAR_END`
        (`PacketInjectorLoopbackConfiguration` reflector) with qualification ID
        `"qual-400zr-far"` and matching `RPCSyncedTiming` timers.
    *   While the qualification is active, verify via `gNMI.Subscribe` that
        `/interfaces/interface/state/oper-status` reports `TESTING` on both
        `DUT port 1` (`Ethernet1`) and `DUT port 2` (`Ethernet2`).
    *   Upon completion, invoke
        `gnoi.packet_link_qualification.LinkQualification.Get` for both
        `"qual-400zr-near"` and `"qual-400zr-far"` and verify:
        *   `state` is `QUALIFICATION_STATE_COMPLETED`.
        *   `status.code` is `0` (`OK`).
        *   `packets_sent` equals `packets_received` (zero packet loss).
        *   `packets_error` (`num_corrupt_packets`) and `packets_dropped`
            (`num_packets_dropped_by_mmu`) are `0`.
    *   Query the OTN FEC uncorrectable blocks path via `gNMI.Get` and
        `gNMI.Subscribe` over the PM interval during and immediately following
        the qualification run and verify the reported value is `0` on both
        `Ethernet1` and `Ethernet2`:
        *   `/terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks`
    *   Clean up the completed qualification records using
        `gnoi.packet_link_qualification.LinkQualification.Delete` and confirm
        deletion via `gnoi.packet_link_qualification.LinkQualification.List`.
3.  **Verify Dual-Stack (IPv4/IPv6) Forwarding and Post-Traffic FEC Telemetry**:
    *   Verify IPv4 and IPv6 neighbor resolution and bidirectional reachability
        across the 400ZR link between `DUT port 1` (`192.0.2.1/30`,
        `2001:db8::1/126`) and `DUT port 2` (`192.0.2.2/30`,
        `2001:db8::2/126`), forwarding IPv4 traffic destined to RFC 5737
        documentation prefix `198.51.100.0/24` (source `192.0.2.1`) and IPv6
        traffic destined to RFC 3849 documentation prefix `2001:db8:1::/64`
        (source `2001:db8::1`).
    *   Verify via `gNMI.Get` that interface packet counters
        `/interfaces/interface/state/counters/in-pkts` and
        `/interfaces/interface/state/counters/out-pkts` increment identically
        with `0` `/interfaces/interface/state/counters/in-fcs-errors` and `0`
        `/terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks`
        over the active PM intervals.

### Cleanup

*   Delete any remaining `gnoi.packet_link_qualification.LinkQualification`
    sessions on `DUT port 1` (`Ethernet1`) and `DUT port 2` (`Ethernet2`).
*   Restore `DUT port 1` (`Ethernet1`), `DUT port 2` (`Ethernet2`),
    transceivers (`Ethernet1-Transceiver`, `Ethernet2-Transceiver`), optical
    channels (`Ethernet1-Optical0`, `Ethernet2-Optical0`), and logical channels
    to their baseline configuration and verify that
    `/interfaces/interface/state/oper-status` is `UP`.

## OpenConfig Path and RPC Coverage

```yaml
paths:
    # Config Parameter coverage
    /interfaces/interface/config/enabled:
    /interfaces/interface/config/name:
    /interfaces/interface/config/type:
    /interfaces/interface/ethernet/config/duplex-mode:
    /interfaces/interface/ethernet/config/port-speed:
    /interfaces/interface/subinterfaces/subinterface/config/index:
    /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
    /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
    /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
    /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
    /components/component/config/name:
        platform_type: ["OPTICAL_CHANNEL", "TRANSCEIVER"]
    /components/component/transceiver/config/enabled:
        platform_type: ["OPTICAL_CHANNEL", "TRANSCEIVER"]
    /components/component/optical-channel/config/frequency:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/config/operational-mode:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/config/target-output-power:
        platform_type: ["OPTICAL_CHANNEL"]
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
    /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks:
    /terminal-device/logical-channels/channel/ethernet/state/fec-uncorrectable-blocks:
    /components/component/optical-channel/state/fec-uncorrectable-blocks:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/frequency:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/operational-mode:
        platform_type: ["OPTICAL_CHANNEL"]
    /components/component/optical-channel/state/target-output-power:
        platform_type: ["OPTICAL_CHANNEL"]
    /interfaces/interface/state/oper-status:
    /interfaces/interface/state/counters/in-pkts:
    /interfaces/interface/state/counters/out-pkts:
    /interfaces/interface/state/counters/in-fcs-errors:
    /interfaces/interface/ethernet/state/counters/fec-uncorrectable-blocks:
    /interfaces/interface/ethernet/state/counters/fec-corrected-blocks:

rpcs:
    gnmi:
        gNMI.Get:
        gNMI.Set:
        gNMI.Subscribe:
    gnoi:
        packet_link_qualification.LinkQualification.Capabilities:
        packet_link_qualification.LinkQualification.Create:
        packet_link_qualification.LinkQualification.Delete:
        packet_link_qualification.LinkQualification.Get:
        packet_link_qualification.LinkQualification.List:
```

## Required DUT platform

*   FFF or MFF