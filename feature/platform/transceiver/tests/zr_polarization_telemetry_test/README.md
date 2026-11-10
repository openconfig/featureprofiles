# TRANSCEIVER-110: Telemetry: ZR Optical Channel Polarization Telemetry Streaming

## Summary

This test validates gNMI streaming telemetry for all 12 coherent optical-channel
polarization metrics across interface `DOWN`, `UP`, and flap `DOWN` state
transitions on coherent 400ZR/ZR+ and 800ZR/ZR+ transceivers:

*   `/components/component/optical-channel/state/polarization-mode-dispersion/{instant,avg,min,max}`
*   `/components/component/optical-channel/state/second-order-polarization-mode-dispersion/{instant,avg,min,max}`
*   `/components/component/optical-channel/state/sop-roc/{instant,avg,min,max}`

This standalone operational telemetry test is intentionally separated from the
baseline ZR/ZR+ platform qualification test suites (`TRANSCEIVER-101`,
`TRANSCEIVER-103`, `TRANSCEIVER-105`, `TRANSCEIVER-107`) and is excluded from
standard platform qualification runs.

## Testbed type

*   [`featureprofiles/topologies/dut_400zr.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut_400zr.testbed)

## Procedure

### Test environment setup

*   Connect a pair of coherent ZR/ZR+ ports on the DUT using a duplex LC fiber
    jumper so that TX output power of one module feeds RX input power of the
    peer module (e.g., `Ethernet4/1/1` <-> `Ethernet4/2/1`).
*   Configure the default network instance on the DUT.
*   Provision coherent optical parameters for each port according to PMD and
    platform defaults (or override via flags), such as:
    *   Operational Mode: `1` (400ZR default)
    *   Center Frequency: `196100000` MHz (`196.1` THz)
    *   Target Output Power: `-10` dBm (or `-7` dBm on ZR+/800ZR)
    *   OTN logical channel index `4000` / `4001` (`PROT_OTN`) assigned to
        `Ethernet4/1-Optical0` / `Ethernet4/2-Optical0`, and Ethernet logical
        channel index `40000` / `40001` (`PROT_ETHERNET`, `TRIB_RATE_400G`,
        `PROT_400GE`) assigned to the corresponding OTN channel.

### TRANSCEIVER-110.1: Optical channel polarization telemetry streaming across interface state transitions

*   **Step 1 - Push initial disabled configuration and start telemetry streams**:
    *   Configure the DUT optical channels, terminal-device OTN/Ethernet logical
        channels, and Ethernet interfaces via `gNMI.Set` with
        `/interfaces/interface/config/enabled` initially set to `false`.
    *   Open `gNMI.Subscribe` (`SAMPLE` mode at 10-second intervals) on
        `/components/component[name=<optical-channel>]/state` for each tested
        port.

#### Canonical OC

```json
{
  "openconfig-interfaces:interfaces": {
    "interface": [
      {
        "config": {
          "enabled": false,
          "name": "Ethernet4/1/1",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "Ethernet4/1/1",
        "openconfig-if-ethernet:ethernet": {
          "config": {
            "duplex-mode": "FULL",
            "port-speed": "SPEED_400GB"
          }
        }
      },
      {
        "config": {
          "enabled": false,
          "name": "Ethernet4/2/1",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "Ethernet4/2/1",
        "openconfig-if-ethernet:ethernet": {
          "config": {
            "duplex-mode": "FULL",
            "port-speed": "SPEED_400GB"
          }
        }
      }
    ]
  },
  "openconfig-platform:components": {
    "component": [
      {
        "config": {
          "name": "Ethernet4/1"
        },
        "name": "Ethernet4/1"
      },
      {
        "config": {
          "name": "Ethernet4/2"
        },
        "name": "Ethernet4/2"
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
            "target-output-power": "-10"
          }
        }
      },
      {
        "config": {
          "name": "Ethernet4/2-Optical0"
        },
        "name": "Ethernet4/2-Optical0",
        "openconfig-terminal-device:optical-channel": {
          "config": {
            "frequency": "196100000",
            "operational-mode": 1,
            "target-output-power": "-10"
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
            "index": 4000,
            "logical-channel-type": "openconfig-transport-types:PROT_OTN"
          },
          "index": 4000,
          "logical-channel-assignments": {
            "assignment": [
              {
                "config": {
                  "allocation": "400",
                  "assignment-type": "OPTICAL_CHANNEL",
                  "description": "OTN to Optical Channel",
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
            "index": 40000,
            "logical-channel-type": "openconfig-transport-types:PROT_ETHERNET",
            "rate-class": "openconfig-transport-types:TRIB_RATE_400G",
            "trib-protocol": "openconfig-transport-types:PROT_400GE"
          },
          "index": 40000,
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
                  "allocation": "400",
                  "assignment-type": "LOGICAL_CHANNEL",
                  "description": "ETH to OTN",
                  "index": 1,
                  "logical-channel": 4000
                },
                "index": 1
              }
            ]
          }
        },
        {
          "config": {
            "admin-state": "ENABLED",
            "description": "OTN Logical Channel",
            "index": 4001,
            "logical-channel-type": "openconfig-transport-types:PROT_OTN"
          },
          "index": 4001,
          "logical-channel-assignments": {
            "assignment": [
              {
                "config": {
                  "allocation": "400",
                  "assignment-type": "OPTICAL_CHANNEL",
                  "description": "OTN to Optical Channel",
                  "index": 1,
                  "optical-channel": "Ethernet4/2-Optical0"
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
            "index": 40001,
            "logical-channel-type": "openconfig-transport-types:PROT_ETHERNET",
            "rate-class": "openconfig-transport-types:TRIB_RATE_400G",
            "trib-protocol": "openconfig-transport-types:PROT_400GE"
          },
          "index": 40001,
          "ingress": {
            "config": {
              "interface": "Ethernet4/2/1",
              "transceiver": "Ethernet4/2"
            }
          },
          "logical-channel-assignments": {
            "assignment": [
              {
                "config": {
                  "allocation": "400",
                  "assignment-type": "LOGICAL_CHANNEL",
                  "description": "ETH to OTN",
                  "index": 1,
                  "logical-channel": 4001
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

*   **Step 2 - Validate polarization telemetry in initial DOWN state**:
    *   Wait via `gNMI.Subscribe` until `/interfaces/interface/state/oper-status`
        reports `DOWN` on all tested interfaces.
    *   Await optical-channel sample updates where `operational-mode` matches the
        configured mode and `input-power`/`output-power` statistics (`instant`,
        `avg`, `min`, `max`) are at or below the inactive power threshold (`-25.0`
        dBm).
    *   Validate any populated leaves across `polarization-mode-dispersion`,
        `second-order-polarization-mode-dispersion`, and `sop-roc`
        (`instant`, `avg`, `min`, `max`):
        *   Verify values are finite `decimal64` numbers (not `NaN`, `-inf`, or
            invalid strings).
        *   Verify populated values are within inactive thresholds (`<= 1.0`).
*   **Step 3 - Enable interfaces and validate polarization telemetry in UP state**:
    *   Set `/interfaces/interface/config/enabled` to `true` on all tested ports
        via `gNMI.Set`.
    *   Wait via `gNMI.Subscribe` until `/interfaces/interface/state/oper-status`
        reports `UP` on all tested interfaces.
    *   Await optical-channel `SAMPLE` stream convergence where
        `operational-mode`, `input-power`, and `output-power` are within target
        operational bounds and all three polarization containers
        (`polarization-mode-dispersion`,
        `second-order-polarization-mode-dispersion`, `sop-roc`) are populated.
    *   Validate all 12 polarization telemetry leaves on each port:
        *   `polarization-mode-dispersion/{instant,avg,min,max}`: finite values
            within `[0.0, 200.0]` ps.
        *   `second-order-polarization-mode-dispersion/{instant,avg,min,max}`:
            finite values within `[0.0, 2000.0]` ps^2.
        *   `sop-roc/{instant,avg,min,max}`: finite values within
            `[0.0, 5000.0]` rad/s.
        *   Statistical consistency for every metric container: `min <= max`, and
            both `instant` and `avg` lie within
            `[min - |min| * 0.05, max + |max| * 0.05]`.
*   **Step 4 - Flap interfaces DOWN, validate polarization telemetry, and restore UP**:
    *   Set `/interfaces/interface/config/enabled` to `false` on all tested ports
        via `gNMI.Set`.
    *   Wait via `gNMI.Subscribe` until `/interfaces/interface/state/oper-status`
        transitions to `DOWN` and optical power converges below `-25.0` dBm,
        then re-validate the polarization leaves against inactive thresholds
        (`<= 1.0`).
    *   Re-enable `/interfaces/interface/config/enabled` to `true` via `gNMI.Set`
        and wait via `gNMI.Subscribe` for `/interfaces/interface/state/oper-status`
        to return to `UP`.

### Cleanup

*   Register cleanup via `t.Cleanup()` to restore `/interfaces/interface/config/enabled`
    to `true` on all modified interfaces via `gNMI.Set` upon test completion
    (including early failures or panics), and close all `gNMI.Subscribe`
    telemetry streams so the DUT is returned to its baseline operational state.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  # Config Parameter coverage
  /components/component/optical-channel/config/frequency:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/config/line-port:
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
  /components/component/transceiver/config/module-functional-type:
    platform_type: ["TRANSCEIVER"]
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/type:
  /interfaces/interface/ethernet/config/duplex-mode:
  /interfaces/interface/ethernet/config/port-speed:
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
  /components/component/optical-channel/state/input-power/avg:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/input-power/instant:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/input-power/max:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/input-power/min:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/operational-mode:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/output-power/avg:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/output-power/instant:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/output-power/max:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/output-power/min:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/polarization-mode-dispersion/avg:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/polarization-mode-dispersion/instant:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/polarization-mode-dispersion/max:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/polarization-mode-dispersion/min:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/second-order-polarization-mode-dispersion/avg:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/second-order-polarization-mode-dispersion/instant:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/second-order-polarization-mode-dispersion/max:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/second-order-polarization-mode-dispersion/min:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/sop-roc/avg:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/sop-roc/instant:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/sop-roc/max:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/sop-roc/min:
    platform_type: ["OPTICAL_CHANNEL"]
  /interfaces/interface/state/oper-status:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```

## Required DUT platform

*   FFF
