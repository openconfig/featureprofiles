# TRANSCEIVER-10.1: Telemetry: 400ZR Optics FEC(Forward Error Correction) Uncorrectable Frames Streaming.

## Summary

Validate 400ZR optics module reports uncorrectable FEC frames count.

This observable represents the number of uncorrectable FEC frames,
measured as RS(544,514) equivalent frames, in a short interval.
This is a post-FEC decoder error metric.

## Procedure

### Setup

The following setup applies to the base test and to every subtest below.

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
*   Establish the point-to-point 400ZR link on both ports using `gNMI.Set`:
    *   Interfaces: administrative state `ENABLED` at
        `/interfaces/interface/config/enabled`, type `ethernetCsmacd` at
        `/interfaces/interface/config/type`, and MTU `9000` at
        `/interfaces/interface/config/mtu` (TRANSCEIVER-10.1.2 uses 8184-byte
        frames). Where the platform requires it, also set port speed
        `SPEED_400GB` at `/interfaces/interface/ethernet/config/port-speed` and
        duplex mode `FULL` at `/interfaces/interface/ethernet/config/duplex-mode`.
    *   Transceivers: `/components/component/transceiver/config/enabled` set
        to `true`.
    *   Optical channels: center frequency `193100000` MHz (`193.1` THz) at
        `/components/component/optical-channel/config/frequency`, target TX
        output power `-10.0` dBm at
        `/components/component/optical-channel/config/target-output-power`, and
        the platform-specific 400ZR operational mode at
        `/components/component/optical-channel/config/operational-mode` (the
        canonical OC below uses `1` as an example).
    *   OTN logical channels (`4000` and `4001`): `PROT_OTN`, trib protocol
        `PROT_400GE`, admin state `ENABLED`, and an `OPTICAL_CHANNEL` assignment
        (`index 0`) to `Ethernet1-Optical0` and `Ethernet2-Optical0`
        respectively. Where the platform requires it, also add a
        `LOGICAL_CHANNEL` assignment (`index 1`) to Ethernet logical channel
        `40000` and `40001` respectively.
    *   Ethernet logical channels (`40000` and `40001`): `PROT_ETHERNET`,
        `TRIB_RATE_400G`, `PROT_400GE`, ingress interface and transceiver of the
        port, and a `LOGICAL_CHANNEL` assignment (`index 0`) to OTN logical
        channel `4000` and `4001` respectively.
    *   When the link is already `UP`, an optical channel or logical channel
        whose state already matches this configuration is not replaced again,
        to avoid re-initializing the coherent DSP between subtests. A link that
        is not `UP` is always fully reconfigured.
*   All FEC uncorrectable blocks leaves below are subscribed with
    `gNMI.Subscribe` in `SAMPLE` mode with a 10-second host Performance
    Monitoring (PM) interval. They are of type `yang:gauge64`: each sample is the
    count over the prior PM interval. They must never stream invalid values such
    as `"nil"` or `"-inf"`, including while the module or device is booting.
*   In TRANSCEIVER-10.1.1 and TRANSCEIVER-10.1.2, all subscribed leaves on both
    ports are validated concurrently. Samples are checked in the order they are
    streamed, starting with the first sample received after the step begins, so
    every port is checked over the same PM intervals. A leaf with no new sample
    within 60 seconds fails. The base TRANSCEIVER-10.1 test validates the OTN
    leaf on each port in turn.
*   Platforms with the `non_interval_fec_error_counter` deviation (currently
    Juniper) report a cumulative counter instead of a per-PM-interval count. On
    those platforms, a check that requires `0` instead requires that the value
    does not increase on a stable link. This masks a constant stale non-zero
    value, which cannot be told apart from a cumulative counter with no new
    errors, so the deviation is expected to be removed once the platform
    reports per-PM-interval counts.

### TRANSCEIVER-10.1: Validate FEC Uncorrectable Frames Streaming

*   Verify that `/interfaces/interface/state/oper-status` is `UP` on both
    ports.
*   Subscribe to
    `/terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks`
    on both ports. Discard the first PM interval sample and verify that the next
    sample is `0`.
*   Flap each port by setting `/interfaces/interface/config/enabled` to `false`,
    waiting for `/interfaces/interface/state/oper-status` to be `DOWN`, setting
    it back to `true`, and waiting for `UP`. Discard the first PM interval sample
    after the link is `UP` (it may cover the optical lock transition) and verify
    that the next sample is `0`.

### TRANSCEIVER-10.1.1: Validate 400ZR FEC PM Interval Semantics

This subtest validates that the OpenConfig FEC uncorrectable blocks telemetry
leaves (`yang:gauge64`, the count over the prior Performance Monitoring (PM)
interval defined by the host) keep streaming a valid value every PM interval,
including after link flaps and transceiver resets. Any value, including
non-zero, is allowed; the value must not be missing (`nil`) or invalid.

1.  **Configuration**:
    *   Apply the [Setup](#setup) configuration.
    *   Configure dual-stack Layer 3 subinterfaces (`index 0`) on `DUT port 1`
        (`Ethernet1`) with IPv4 address `192.0.2.1/30` and IPv6 address
        `2001:db8::1/126`, and on `DUT port 2` (`Ethernet2`) with IPv4 address
        `192.0.2.5/30` and IPv6 address `2001:db8::5/126`. Both ports are on the
        same DUT in the default network-instance, so each port uses its own
        subnet.
2.  **Baseline Operational State and PM Interval Telemetry**:
    *   Verify that `/interfaces/interface/state/oper-status` is `UP` on both
        ports.
    *   Use `gNMI.Get` to verify that the configured optical parameters are
        reflected at `/components/component/optical-channel/state/frequency`,
        `/components/component/optical-channel/state/target-output-power`, and
        `/components/component/optical-channel/state/operational-mode`.
    *   Subscribe to the following leaves on both ports:
        *   `/terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks`
        *   `/terminal-device/logical-channels/channel/ethernet/state/fec-uncorrectable-blocks`
        *   `/components/component/optical-channel/state/fec-uncorrectable-blocks`
    *   Verify that each leaf streams a valid value (not `nil`) for 3
        consecutive PM intervals.
3.  **Transient Link Disturbance**:
    *   Flap `DUT port 1` (`Ethernet1`) as in the
        [base test](#transceiver-101-validate-fec-uncorrectable-frames-streaming),
        keeping it `DOWN` for 1 PM interval (10 seconds) before setting
        `/interfaces/interface/config/enabled` back to `true`. Wait for both
        ports to be `UP`.
    *   Toggle `Ethernet1-Transceiver` using
        `/components/component/transceiver/config/enabled` (`false`, wait for
        `DOWN`, then `true`) and wait for both ports to be `UP`.
    *   After each disturbance, verify the next 2 PM intervals once the link is
        `UP`.
    *   **Pass/Fail Criteria**:
        *   **Pass**: Each of the following leaves streams a valid value (not
            `nil`) for every PM interval. Any value is allowed, including
            non-zero values caused by errors during the optical lock/unlock
            transition:
            *   `/terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks`
            *   `/terminal-device/logical-channels/channel/ethernet/state/fec-uncorrectable-blocks`
            *   `/components/component/optical-channel/state/fec-uncorrectable-blocks`
        *   **Fail**: A leaf is not streamed (`nil`) in a PM interval, or streams
            invalid values such as `"nil"` or `"-inf"`.

#### Canonical OC

```json
{
  "openconfig-interfaces:interfaces": {
    "interface": [
      {
        "config": {
          "enabled": true,
          "mtu": 9000,
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
          "mtu": 9000,
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
                        "ip": "192.0.2.5",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.5"
                    }
                  ]
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
            "logical-channel-type": "openconfig-transport-types:PROT_OTN",
            "trib-protocol": "openconfig-transport-types:PROT_400GE"
          },
          "index": 4000,
          "logical-channel-assignments": {
            "assignment": [
              {
                "config": {
                  "allocation": "400.0",
                  "assignment-type": "OPTICAL_CHANNEL",
                  "description": "OTN to optical channel assignment 1",
                  "index": 0,
                  "optical-channel": "Ethernet1-Optical0"
                },
                "index": 0
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
                  "index": 0,
                  "logical-channel": 4000
                },
                "index": 0
              }
            ]
          }
        }
      ]
    }
  }
}
```

### TRANSCEIVER-10.1.2: Validate 400ZR Link Qualification and Post-Qualification Link Health

This subtest is self-contained and independent of prior testcases. It verifies
that packet-based Link Qualification on a 400ZR link succeeds without
false-positive uncorrectable FEC block counts, and that the link and its
dual-stack interface addresses are healthy after the qualification.

1.  **Configuration and gNOI Packet Link Qualification Capabilities**:
    *   Apply the configuration from TRANSCEIVER-10.1.1 step 1. Verify that
        `/interfaces/interface/state/oper-status` is `UP` and that the IPv4 and
        IPv6 addresses are present at
        `/interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/state/ip`
        and
        `/interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/state/ip`
        on both ports. Record `/interfaces/interface/state/counters/in-fcs-errors`
        on both ports.
    *   Invoke `gnoi.packet_link_qualification.LinkQualification.Capabilities`
        and verify that:
        *   The packet generator reports `max_mtu >= 8184` and
            `max_bps >= 100000000000` (100 Gbps, as required by gNOI-2.1).
        *   The reflector supports `AsicLoopback` or `PmdLoopback` (non-zero
            `min_setup_duration`).
2.  **gNOI Packet Link Qualification**:
    *   Use a packet size of `8184` bytes. Use a packet rate equal to 99% of the
        400 Gbps line rate including 20 bytes of per-frame overhead (about
        6.03 Mpps; the 1% headroom absorbs clock offset and control-plane
        frames), capped by the generator `max_bps / (8 × packet size)` and
        `max_pps`.
    *   Invoke `gnoi.packet_link_qualification.LinkQualification.Create` with:
        *   `DUT port 1` (`Ethernet1`) as the packet generator, qualification ID
            `"qual-400zr-near"`, and `RPCSyncedTiming` `PreSyncDuration: 10s`,
            `SetupDuration` = the largest of `30s` and the generator and
            reflector `min_setup_duration`, `Duration: 60s`,
            `PostSyncDuration: 0s`, and `TeardownDuration` = the largest of
            `30s` and the generator and reflector `min_teardown_duration`.
        *   `DUT port 2` (`Ethernet2`) as the reflector, using `AsicLoopback`
            when supported and `PmdLoopback` otherwise, qualification ID
            `"qual-400zr-far"`, and `RPCSyncedTiming` `PreSyncDuration: 0s`,
            the same `SetupDuration`, `Duration: 60s`, `PostSyncDuration: 30s`,
            and the same `TeardownDuration`.
    *   While the qualification is active, verify that
        `/interfaces/interface/state/oper-status` reports `TESTING` on both
        ports.
    *   Once `"qual-400zr-near"` is `QUALIFICATION_STATE_RUNNING`, discard the
        first PM interval sample and verify that the next PM interval sample of
        `/terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks`
        is `0` on both ports.
    *   When both qualifications complete, invoke
        `gnoi.packet_link_qualification.LinkQualification.Get` for
        `"qual-400zr-near"` and `"qual-400zr-far"` and verify:
        *   `state` is `QUALIFICATION_STATE_COMPLETED`.
        *   `status.code` is `0` (`OK`).
        *   `packets_sent` equals `packets_received` and is non-zero.
        *   `packets_error` and `packets_dropped` are `0`.
    *   Delete both qualifications using
        `gnoi.packet_link_qualification.LinkQualification.Delete` and confirm
        with `gnoi.packet_link_qualification.LinkQualification.List` that no
        qualifications remain.
3.  **Post-Qualification Link Health**:
    *   Verify that `/interfaces/interface/state/oper-status` is `UP` on both
        ports.
    *   Verify that the IPv4 and IPv6 addresses are still present in state on
        both ports.
    *   Verify that `/interfaces/interface/state/counters/in-fcs-errors` has not
        changed since step 1.
    *   Discard the first PM interval sample and verify that the next sample of
        `/terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks`
        is `0` on both ports.

### Cleanup

*   Delete any remaining `gnoi.packet_link_qualification.LinkQualification`
    sessions on `DUT port 1` (`Ethernet1`) and `DUT port 2` (`Ethernet2`).
*   For any port whose `/interfaces/interface/state/oper-status` is not `UP`,
    re-apply the [Setup](#setup) configuration (and the subinterface addresses
    for TRANSCEIVER-10.1.1 and TRANSCEIVER-10.1.2) and verify that the port
    returns to `UP`.

## OpenConfig Path and RPC Coverage

```yaml
paths:
    # Config Parameter coverage
    /interfaces/interface/config/enabled:
    /interfaces/interface/config/mtu:
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
        platform_type: ["TRANSCEIVER"]
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
    /terminal-device/logical-channels/channel/state/logical-channel-type:
    /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/logical-channel:
    /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/state/optical-channel:
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
    /components/component/transceiver/physical-channels/channel/state/associated-optical-channel:
        platform_type: ["TRANSCEIVER"]
    /interfaces/interface/state/transceiver:
    /interfaces/interface/state/oper-status:
    /interfaces/interface/state/counters/in-fcs-errors:
    /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/state/ip:
    /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/state/ip:

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
