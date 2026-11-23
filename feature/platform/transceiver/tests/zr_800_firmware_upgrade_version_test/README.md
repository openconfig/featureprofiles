# TRANSCEIVER-3.3: 800ZR Optics firmware upgrade and firmware-version decoding

## Summary

This test validates that, after an in-service firmware upgrade or downgrade of
a coherent 800G ZR (`PMD_800GBASE_ZR`, `SPEED_800GB`, OSFP or QSFP-DD form
factor) optical transceiver, the DUT reports the module firmware version
through `/components/component/state/firmware-version` exactly as encoded by
the module, and that the reported value remains correct across transceiver
re-initialization, interface flaps, and firmware rollback.

Common Management Interface Specification (CMIS) modules report the running
firmware image revision as a 1-byte major number, a 1-byte minor number, and a
**build number encoded as a 2-byte, big-endian, unsigned 16-bit integer** (for
example, the image build number field of the CDB "Get Firmware Info"
response). The DUT is expected to render the version string as
`<major>.<minor>.<build>`, where `<build>` is the decimal value of the
combined 16-bit integer. For example, build bytes `0x1D 0x00` represent build
`7424`, so a module running firmware `1.1` build `7424` must be reported as
`1.1.7424`.

Two classes of host-software decoding defects are targeted:

*   **Byte-concatenation**: each build byte is converted to decimal
    independently and the strings are concatenated (`0x1D` → `"29"`, `0x00` →
    `"0"`), producing `1.1.290`.
*   **Byte-order (little-endian) decoding**: the two build bytes are combined
    in the wrong order (`0x001D`), producing `1.1.29`.

Both defects leave the upgrade itself successful and are invisible to tests
that only check that `firmware-version` is a non-empty string. This test
therefore compares every reported value against an exact, operator-supplied
expected string and explicitly rejects the byte-concatenated and little-endian
renderings. Every subtest also verifies that unrelated inventory leaves are
unchanged, that the DUT did not reboot unexpectedly, and that IPv4 and IPv6
packets physically traverse the 800ZR link without errors.

A single CMIS CDB firmware upgrade of an 800ZR module can take 30–60 minutes or
more, and every flash cycle adds wear and risk to lab optics. The test is
therefore limited to four subtests and the minimum number of firmware installs
needed to cover the upgrade, re-initialization, and rollback paths.

## Testbed type

*   [`featureprofiles/topologies/dut_800zr.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut_800zr.testbed)

## Procedure

### Test environment setup

```text
  +-----------------------------------------------------------------+
  |                               DUT                               |
  |                                                                 |
  |  DEFAULT network-instance            VRF-ZR network-instance    |
  |  DUT port 1 (Ethernet4/1/1)          DUT port 2 (Ethernet5/1/1) |
  |  192.0.2.1/30                        192.0.2.2/30               |
  |  2001:db8::1/126                     2001:db8::2/126            |
  |  [800ZR transceiver Ethernet4/1]     [800ZR transceiver         |
  |                                       Ethernet5/1]              |
  +-----------------|-----------------------------|-----------------+
                    |                             |
                    +====== 800ZR coherent link ==+
                       (duplex LC fiber jumper)
```

#### Topology

*   Connect **DUT port 1** to **DUT port 2** with a duplex LC fiber jumper so
    that the TX output of each 800ZR module is the RX input of the other
    (insertion loss less than 3 dB).
*   On modular (MFF) chassis, DUT port 1 and DUT port 2 should preferably
    reside on different `LINECARD` components when the testbed cabling allows
    it; this is not required.
*   Both transceivers are upgraded in this test. Neither transceiver is a
    fixed "reference"; the peer of the transceiver being acted on is always
    checked for unintended side effects.
*   This testbed has no ATE. Data-plane health is verified with gNOI ping
    across the physical link, combined with interface counter deltas that
    prove traversal of the fiber and error/FEC counters that prove link
    quality (see **Dual-stack link verification procedure**).

#### Component and interface names

| Role | DUT port 1 | DUT port 2 |
|---|---|---|
| Interface (subinterface `0`) | `Ethernet4/1/1` | `Ethernet5/1/1` |
| Port component (`PORT`) | `Ethernet4/1-Port` | `Ethernet5/1-Port` |
| Transceiver component (`TRANSCEIVER`) | `Ethernet4/1` | `Ethernet5/1` |
| Optical-channel component (`OPTICAL_CHANNEL`) | `Ethernet4/1-Optical0` | `Ethernet5/1-Optical0` |
| OTN logical channel index | `8000` | `8001` |
| Ethernet logical channel index | `80000` | `80001` |
| Network-instance | `DEFAULT` | `VRF-ZR` (`L3VRF`) |
| IPv4 address | `192.0.2.1/30` | `192.0.2.2/30` |
| IPv6 address | `2001:db8::1/126` | `2001:db8::2/126` |

The names above are used throughout this README. Test automation must resolve
the actual names on the DUT at run time as follows, and use the resolved names
wherever the README uses the names above:

*   Interface: from the testbed port mapping (`dut:port1`, `dut:port2`).
*   Transceiver: `/interfaces/interface/state/transceiver` of the interface.
*   Port: `/components/component/state/parent` of the transceiver.
*   Optical channel: the component whose `/components/component/state/parent`
    is the transceiver and whose `/components/component/state/type` is
    `OPTICAL_CHANNEL`.
*   Default network-instance: `DEFAULT` (or the platform default
    network-instance name provided by the `default_network_instance`
    deviation).

#### Configuration parameters (both ports)

This table and the addresses in **Component and interface names** are the
single source of the DUT configuration; the Canonical OC in TRANSCEIVER-3.3.1
encodes exactly these values.

| Object (port 1 / port 2) | Path | Value |
|---|---|---|
| Transceiver `Ethernet4/1` / `Ethernet5/1` | `/components/component/transceiver/config/enabled` | `true` |
| Port `Ethernet4/1-Port` / `Ethernet5/1-Port` | `/components/component/port/breakout-mode/groups/group/config/{index, breakout-speed, num-breakouts, num-physical-channels}` | `1`, `SPEED_800GB`, `1`, `8` |
| Optical channel `Ethernet4/1-Optical0` / `Ethernet5/1-Optical0` | `/components/component/optical-channel/config/{frequency, target-output-power, operational-mode}` | `196100000` (MHz), `-7` (dBm), `operational_mode` (default `1`) |
| Interface `Ethernet4/1/1` / `Ethernet5/1/1` | `/interfaces/interface/config/{enabled, type}`, `/interfaces/interface/ethernet/config/port-speed` | `true`, `ethernetCsmacd`, `SPEED_800GB` |
| Subinterface `0` | `.../subinterface/ipv4/config/enabled`, `.../subinterface/ipv6/config/enabled`, `.../ipv4/addresses/address/config/{ip, prefix-length}`, `.../ipv6/addresses/address/config/{ip, prefix-length}` | `true`, `true`, addresses from the names table |
| Network-instance `VRF-ZR` | `/network-instances/network-instance/config/{name, type}`, `.../interfaces/interface/config/{id, interface, subinterface}` | `VRF-ZR`, `L3VRF`, `Ethernet5/1/1.0`, `Ethernet5/1/1`, `0` |
| OTN logical channel `8000` / `8001` | `/terminal-device/logical-channels/channel/config/{logical-channel-type, admin-state}`; assignment `1`: `.../logical-channel-assignments/assignment/config/{assignment-type, optical-channel, allocation}` | `PROT_OTN`, `ENABLED`; `OPTICAL_CHANNEL`, `Ethernet4/1-Optical0` / `Ethernet5/1-Optical0`, `800` |
| Ethernet logical channel `80000` / `80001` | `.../channel/config/{logical-channel-type, rate-class, trib-protocol, admin-state}`, `.../ingress/config/{interface, transceiver}`; assignment `1`: `.../assignment/config/{assignment-type, logical-channel, allocation}` | `PROT_ETHERNET`, `TRIB_RATE_800G`, `PROT_800GE`, `ENABLED`; `Ethernet4/1/1` / `Ethernet5/1/1`, `Ethernet4/1` / `Ethernet5/1`; `LOGICAL_CHANNEL`, `8000` / `8001`, `800` |

Platforms that do not model 800ZR through the `terminal-device`
logical-channel hierarchy may omit the two logical-channel rows using the
proposed deviation `terminal_device_logical_channels_unsupported`; all other
configuration still applies.

#### Test flags

| Flag | Type | Default | Required | Description and validation |
|---|---|---|---|---|
| `baseline_firmware_image` | string | none | Yes | Vendor-specific, vendor-signed image installed at the start (TRANSCEIVER-3.3.1) and restored by rollback (TRANSCEIVER-3.3.4) and cleanup. |
| `baseline_firmware_version` | string | none | Yes | Exact version string reported for `baseline_firmware_image`. Must differ from every version in `target_firmware_matrix`. |
| `target_firmware_matrix` | list of `<image>=<version>` | none | Yes (at least 1 entry) | Vendor-signed target images and their exact expected version strings. The **first entry is the primary target**; its version is referred to as `EXPECTED_VERSION`. Additional entries are optional. Because images are vendor-signed, the build numbers available are whatever the vendor ships; builds greater than `255` (so that both build bytes are significant) are recommended but not required. |
| `corrupt_firmware_image` | string | empty | No | An image with a corrupted payload (for example, truncated or with an invalid checksum). The optional bad-image check in TRANSCEIVER-3.3.4 runs only when this flag is set. |
| `operational_mode` | uint16 | `1` | No | 800ZR operational mode used on both optical channels. |
| `pre_fec_ber_threshold` | float | none | Yes | Maximum allowed `pre-fec-ber/instant` for the platform's 800ZR FEC, taken from the module or platform specification. |
| `ping_count` | int | `300` | No | Echo requests per gNOI ping in the DLV. Must be `≥ 100`. |
| `ping_interval` | duration | `100ms` | No | Interval between echo requests in the DLV. |
| `link_up_timeout` | duration | `10m` | No | Maximum wait for transceiver `ACTIVE` and interface `UP`. |
| `link_down_timeout` | duration | `2m` | No | Maximum wait for interface down transitions. |
| `upgrade_timeout` | duration | `90m` | No | Maximum time for an upgrade, downgrade, or failed attempt to complete and the module to re-initialize. |
| `bounce_iterations` | int | `2` | No | Number of transceiver disable/enable iterations in TRANSCEIVER-3.3.3. |
| `flap_iterations` | int | `2` | No | Number of interface flap iterations in TRANSCEIVER-3.3.3. |

The test must fail (`t.Fatal`) before any device change only if a required
flag is missing or `baseline_firmware_version` equals a target version. The
build-number byte patterns of the supplied images are never a reason to fail;
they only affect which forbidden-value checks apply (see below).

#### Derived values for each expected version

For an expected version string `<prefix>.<B>` (where `<prefix>` is everything
before the last `.`, typically `<major>.<minor>`, and `B` is the build number),
the test computes:

*   **Byte-concatenated form** `CONCAT(B)`:
    `<prefix>.<decimal(B >> 8)><decimal(B & 0xFF)>`.
*   **Little-endian form** `LE(B)`:
    `<prefix>.<((B & 0xFF) << 8) | (B >> 8)>`.

Both forms are **forbidden values**: no reported `firmware-version` may equal
them. A forbidden-value check is skipped, and the skip is logged, when:

*   the last dot-separated field of the expected version is not a decimal
    integer from `0` to `65535` (the build number cannot be derived), or
*   the derived form equals the correct version string (for example, `LE(B)`
    for a byte-symmetric build such as `0xFFFF`).

The exact-match check against the expected version always applies.

| Example build `B` | Bytes | Correct suffix | `CONCAT(B)` suffix | `LE(B)` suffix |
|---|---|---|---|---|
| `7424` | `0x1D 0x00` | `7424` | `290` | `29` |
| `2571` | `0x0A 0x0B` | `2571` | `1011` | `2826` |
| `256` | `0x01 0x00` | `256` | `10` | `1` |
| `65535` | `0xFF 0xFF` | `65535` | `255255` | `65535` (check skipped) |

#### Firmware upgrade mechanism

Neither the OpenConfig data models nor gNOI currently define an operation to
upgrade the firmware of an individual transceiver. Every install step in this
test (upgrade, rollback, and the optional bad-image attempt) is therefore
executed with a vendor-specific helper (for example, CLI over gNMI or SSH)
gated by the proposed deviation `transceiver_firmware_upgrade_via_cli`. The
helper takes a transceiver component name and an image, and returns either
success or an error. All pass/fail verification in this test uses OpenConfig
paths only. Vendor-specific upgrade status (for example, an upgrade state
machine that reports success and then returns to idle) may be logged for
debugging, but must not be used as a pass/fail criterion.

*   TODO: Propose a vendor-neutral transceiver firmware upgrade RPC or
    OpenConfig path to [openconfig/gnoi](https://github.com/openconfig/gnoi)
    or [openconfig/public](https://github.com/openconfig/public).

#### Reusable verification procedures

The following procedures are referenced by name from the subtests.

##### Configuration-applied gate (CAG)

Using `gNMI.Subscribe` (`ON_CHANGE`, timeout `link_up_timeout`), wait until
all of the following are true:

*   `/components/component/optical-channel/state/frequency` = `196100000` on
    `Ethernet4/1-Optical0` and `Ethernet5/1-Optical0`.
*   `/components/component/optical-channel/state/operational-mode` =
    `operational_mode` on both optical channels.
*   `/components/component/optical-channel/state/target-output-power` =
    `-7.0` (±`1.0` dB) on both optical channels.
*   `/interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/state/ip`
    = `192.0.2.1` on `Ethernet4/1/1` and `192.0.2.2` on `Ethernet5/1/1`.
*   `/interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/state/ip`
    = `2001:db8::1` on `Ethernet4/1/1` and `2001:db8::2` on `Ethernet5/1/1`.
*   `/network-instances/network-instance/state/type` = `L3VRF` for `VRF-ZR`.

##### Link-up gate (LUG)

Using `gNMI.Subscribe` (`ON_CHANGE`, timeout `link_up_timeout`, or
`upgrade_timeout` when stated), wait until:

*   `/components/component/state/oper-status` = `ACTIVE` on `Ethernet4/1` and
    `Ethernet5/1`.
*   `/components/component/transceiver/state/enabled` = `true` on both
    transceivers.
*   `/interfaces/interface/state/oper-status` = `UP` on `Ethernet4/1/1` and
    `Ethernet5/1/1`.

##### Firmware version verification (FVV)

The FVV takes one or more pairs of transceiver `T` and expected version `V`.
When a step runs the FVV for both transceivers (with the same or different
`V`), both pairs are verified with **one** batched `gNMI.Get` and **one**
`SAMPLE` subscription, so the procedure takes about `30` seconds regardless of
the number of transceivers.

1.  Read `/components/component/state/firmware-version` on every `T` with a
    single batched `gNMI.Get` and verify that each value equals its `V`
    exactly.
2.  Open one `gNMI.Subscribe` `SAMPLE` subscription with a `10` second sample
    interval to `/components/component/state/firmware-version` on every `T`
    and collect `3` samples per transceiver.
3.  Verify that every sample equals its `V` exactly.
4.  Verify that neither the `gNMI.Get` value nor any sample equals
    `CONCAT(B)` or `LE(B)` of its `V` (where applicable), or the empty string.
5.  On failure, report the transceiver, the observed value, `V`, `CONCAT(B)`,
    and `LE(B)` in the failure message.

##### Firmware-version history rule (FHR)

Subtests that start an **FHR subscription** open one `gNMI.Subscribe`
`ON_CHANGE` subscription to `/components/component/state/firmware-version`
and `/components/component/state/oper-status` on `Ethernet4/1` and
`Ethernet5/1`, and declare an **allowed set** of versions for each
transceiver. When the subtest closes the subscription, every received
`firmware-version` notification must satisfy:

*   A non-empty value must be in that transceiver's allowed set.
*   A value equal to `CONCAT(B)` or `LE(B)` of any version in the allowed set
    is always a failure.
*   An empty-string value or a delete notification is accepted **only** if the
    most recent `oper-status` notification for the same transceiver received
    before it is not `ACTIVE` (the module is re-initializing). An empty value
    or delete while the transceiver is `ACTIVE` is a failure.

##### Inventory and stability check (ISC)

Using a single batched `gNMI.Get`, verify on both `Ethernet4/1` and
`Ethernet5/1`:

*   `/components/component/state/part-no`,
    `/components/component/state/serial-no`,
    `/components/component/state/mfg-name`, and
    `/components/component/state/hardware-version` equal the values recorded in
    TRANSCEIVER-3.3.1 Step 3.
*   `/components/component/state/type` = `TRANSCEIVER` and
    `/components/component/transceiver/state/present` = `PRESENT`.
*   `/system/state/boot-time` equals the value recorded in TRANSCEIVER-3.3.1
    Step 3 (`BOOT_TIME`), proving that the DUT did not reboot unexpectedly.

##### Dual-stack link verification procedure (DLV)

This procedure proves that IPv4 and IPv6 packets physically traverse the 800ZR
link in both directions without errors. Because `192.0.2.2` and
`2001:db8::2` are in `VRF-ZR`, they are reachable from the `DEFAULT`
network-instance only through `Ethernet4/1/1` and the fiber.

1.  **Neighbor resolution gate**: Issue a priming `gNOI.System.Ping` with
    `count` = `5` for each of the four pings in step 3 (results ignored).
    Then wait via `gNMI.Subscribe` (`ON_CHANGE`, timeout `60` s) until
    `/interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/state/link-layer-address`
    is populated for neighbor `192.0.2.2` on `Ethernet4/1/1` and for
    neighbor `192.0.2.1` on `Ethernet5/1/1`, and
    `/interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/link-layer-address`
    is populated for neighbor `2001:db8::2` on `Ethernet4/1/1` and for
    neighbor `2001:db8::1` on `Ethernet5/1/1`.
2.  **Counter baseline**: Using a single batched `gNMI.Get`, record on
    `Ethernet4/1/1` and `Ethernet5/1/1`:
    `/interfaces/interface/state/counters/in-pkts`, `out-pkts`, `in-errors`,
    `in-fcs-errors`, and `out-errors`; and
    `/terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks`
    on logical channels `8000` and `8001`.
3.  **Pings**: Issue the following `gNOI.System.Ping` operations, each with
    `count` = `ping_count` (default `300`), `interval` = `ping_interval`
    (default `100000000` ns, 100 ms), `wait` = `1000000000` ns (1 s), and
    `size` = `1400`:
    *   IPv4 forward: `network_instance` = `DEFAULT`, `source` =
        `192.0.2.1`, `destination` = `192.0.2.2`, `l3protocol` = `IPV4`,
        `do_not_fragment` = `true`.
    *   IPv4 reverse: `network_instance` = `VRF-ZR`, `source` =
        `192.0.2.2`, `destination` = `192.0.2.1`, `l3protocol` = `IPV4`,
        `do_not_fragment` = `true`.
    *   IPv6 forward: `network_instance` = `DEFAULT`, `source` =
        `2001:db8::1`, `destination` = `2001:db8::2`, `l3protocol` = `IPV6`.
    *   IPv6 reverse: `network_instance` = `VRF-ZR`, `source` =
        `2001:db8::2`, `destination` = `2001:db8::1`, `l3protocol` = `IPV6`.
4.  **Counter deltas**: Repeat the batched `gNMI.Get` from step 2.
5.  **Pass criteria**:
    *   Each ping reports `received` = `ping_count` (0% loss).
    *   `out-pkts` delta on `Ethernet4/1/1` ≥ `4 × ping_count` and `in-pkts`
        delta on `Ethernet5/1/1` ≥ `4 × ping_count`; `out-pkts` delta on
        `Ethernet5/1/1` ≥ `4 × ping_count` and `in-pkts` delta on
        `Ethernet4/1/1` ≥ `4 × ping_count` (two pings × `ping_count` requests
        or replies per direction per address family; `1200` with the
        default). This proves that the packets crossed the fiber and were not
        delivered locally.
    *   `in-errors`, `in-fcs-errors`, and `out-errors` deltas are `0` on both
        interfaces.
    *   `fec-uncorrectable-blocks` delta is `0` on logical channels `8000`
        and `8001`.
    *   `/terminal-device/logical-channels/channel/otn/state/pre-fec-ber/instant`
        is below `pre_fec_ber_threshold` on logical channels `8000` and
        `8001`.

##### General pass/fail rule

A subtest **passes** only if every gate (CAG, LUG, and each explicit
`gNMI.Subscribe` wait), FVV, FHR evaluation, ISC, and DLV invoked in it
passes, and its subtest-specific criteria below are met. Any timeout, version
mismatch, forbidden value (`CONCAT(B)`, `LE(B)`, or an empty value outside
re-initialization), changed inventory leaf, unexpected reboot, ping loss, or
error/FEC counter increment **fails** the subtest.

### TRANSCEIVER-3.3.1: Configure the 800ZR link and establish the baseline firmware

This subtest configures the DUT, brings up the 800ZR link, installs the known
baseline firmware on both transceivers if needed, and records every baseline
value the later subtests depend on. Starting from a known baseline guarantees
that the upgrade in TRANSCEIVER-3.3.2 actually changes the firmware build, so
the test cannot pass without exercising the upgrade and decode path.

*   **Step 1 - Configure the DUT**:
    *   Push the configuration shown in the Canonical OC below with
        `gNMI.Set`. It contains every row of **Configuration parameters** for
        both ports and the IPv4/IPv6 addresses from **Component and interface
        names**. Test code may build this configuration with the existing
        `cfgplugins` optical-channel, logical-channel, and interface helpers
        (which implement existing vendor deviations) and may apply it in one
        or more `gNMI.Set` requests, provided the resulting configuration is
        equivalent to the Canonical OC.
*   **Step 2 - Wait for the configuration and the link**:
    *   Run the **CAG**, then the **LUG**.
*   **Step 3 - Record baseline values**:
    *   Using a single batched `gNMI.Get`, verify on `Ethernet4/1` and
        `Ethernet5/1` that `/components/component/state/type` =
        `TRANSCEIVER`, `/components/component/transceiver/state/present` =
        `PRESENT`, and `/components/component/transceiver/state/form-factor`
        is `OSFP` or `QSFP_DD`.
    *   Record `/components/component/state/part-no`,
        `/components/component/state/serial-no`,
        `/components/component/state/mfg-name`, and
        `/components/component/state/hardware-version` on both transceivers
        (each must be a non-empty string).
    *   Record `/components/component/state/firmware-version` on both
        transceivers as `INITIAL_VERSION_P1` and `INITIAL_VERSION_P2` (logged
        for debugging and used in cleanup).
    *   Record `/system/state/boot-time` as `BOOT_TIME`.
*   **Step 4 - Install the baseline firmware where needed**:
    *   For each transceiver (`Ethernet4/1`, then `Ethernet5/1`) whose
        `firmware-version` is not already `baseline_firmware_version`, install
        `baseline_firmware_image` with the vendor-specific helper. Skip the
        install for a transceiver that already runs the baseline, to avoid an
        unnecessary flash cycle.
    *   Run the **LUG** with timeout `upgrade_timeout`.
*   **Step 5 - Verify the baseline**:
    *   Run the **FVV** for `Ethernet4/1` and `Ethernet5/1` with `V` =
        `baseline_firmware_version`.
    *   Run the **ISC**.
    *   Run the **DLV**.
*   **Pass/Fail Criteria**: General pass/fail rule, plus:
    *   Both transceivers report exactly `baseline_firmware_version`. A failure
        in this subtest blocks all later subtests.

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
                        "ip": "192.0.2.2",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.2"
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
                        "ip": "2001:db8::2",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8::2"
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
          "name": "VRF-ZR",
          "type": "openconfig-network-instance-types:L3VRF"
        },
        "interfaces": {
          "interface": [
            {
              "config": {
                "id": "Ethernet5/1/1.0",
                "interface": "Ethernet5/1/1",
                "subinterface": 0
              },
              "id": "Ethernet5/1/1.0"
            }
          ]
        },
        "name": "VRF-ZR"
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
          "name": "Ethernet5/1-Port"
        },
        "name": "Ethernet5/1-Port",
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
          "name": "Ethernet5/1"
        },
        "name": "Ethernet5/1",
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
      },
      {
        "config": {
          "name": "Ethernet5/1-Optical0"
        },
        "name": "Ethernet5/1-Optical0",
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
                  "index": 1,
                  "logical-channel": 8000
                },
                "index": 1
              }
            ]
          }
        },
        {
          "config": {
            "admin-state": "ENABLED",
            "index": 8001,
            "logical-channel-type": "openconfig-transport-types:PROT_OTN"
          },
          "index": 8001,
          "logical-channel-assignments": {
            "assignment": [
              {
                "config": {
                  "allocation": "800",
                  "assignment-type": "OPTICAL_CHANNEL",
                  "index": 1,
                  "optical-channel": "Ethernet5/1-Optical0"
                },
                "index": 1
              }
            ]
          }
        },
        {
          "config": {
            "admin-state": "ENABLED",
            "index": 80001,
            "logical-channel-type": "openconfig-transport-types:PROT_ETHERNET",
            "rate-class": "openconfig-transport-types:TRIB_RATE_800G",
            "trib-protocol": "openconfig-transport-types:PROT_800GE"
          },
          "index": 80001,
          "ingress": {
            "config": {
              "interface": "Ethernet5/1/1",
              "transceiver": "Ethernet5/1"
            }
          },
          "logical-channel-assignments": {
            "assignment": [
              {
                "config": {
                  "allocation": "800",
                  "assignment-type": "LOGICAL_CHANNEL",
                  "index": 1,
                  "logical-channel": 8001
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

### TRANSCEIVER-3.3.2: Upgrade both transceivers and verify exact firmware-version decoding

This subtest upgrades both transceivers to the primary target, one at a time,
and verifies the exact decoded version, rejects the `CONCAT(B)` and `LE(B)`
renderings, and checks the link. Upgrading one transceiver at a time isolates
per-port decoding and checks the peer for side effects. No new configuration
is introduced.

*   **Precondition**: TRANSCEIVER-3.3.1 has passed. Both transceivers run
    `baseline_firmware_image`.
*   **Step 1 - Start history monitoring**:
    *   Start an **FHR subscription** with allowed set
        {`baseline_firmware_version`, every version in
        `target_firmware_matrix`} for both transceivers. Keep it open until
        Step 5.
*   **Step 2 - Upgrade DUT port 1 to the primary target**:
    *   Install the primary-target image (first entry of
        `target_firmware_matrix`) on `Ethernet4/1` with the vendor-specific
        helper.
    *   Using `gNMI.Subscribe` (`ON_CHANGE`, timeout `upgrade_timeout`), wait
        for the combined condition
        `/components/component/state/oper-status` = `ACTIVE` **and**
        `/components/component/state/firmware-version` = `EXPECTED_VERSION`
        on `Ethernet4/1`. On timeout, fail and report the last observed
        `firmware-version`, `EXPECTED_VERSION`, `CONCAT(B)`, and `LE(B)`.
    *   Run the **LUG**.
    *   Run the **FVV** for `Ethernet4/1` with `V` = `EXPECTED_VERSION` and
        for `Ethernet5/1` with `V` = `baseline_firmware_version` (the peer
        must not change).
    *   Run the **ISC**.
*   **Step 3 - Upgrade DUT port 2 to the primary target**:
    *   Install the primary-target image on `Ethernet5/1` with the
        vendor-specific helper.
    *   Using `gNMI.Subscribe` (`ON_CHANGE`, timeout `upgrade_timeout`), wait
        for `/components/component/state/oper-status` = `ACTIVE` **and**
        `/components/component/state/firmware-version` = `EXPECTED_VERSION`
        on `Ethernet5/1`, reporting the same diagnostics on timeout.
    *   Run the **LUG**.
    *   Run the **FVV** for `Ethernet5/1` with `V` = `EXPECTED_VERSION` and
        for `Ethernet4/1` with `V` = `EXPECTED_VERSION` (the already-upgraded
        peer must not change).
    *   Run the **ISC**.
*   **Step 4 - Optional additional target images**:
    *   Only if `target_firmware_matrix` has more than one entry: for each
        additional entry `k` (second entry onward), with expected version
        `V_k`, install image `k` on `Ethernet4/1` and `Ethernet5/1` at the same
        time with the vendor-specific helper, wait via `gNMI.Subscribe`
        (`ON_CHANGE`, timeout `upgrade_timeout`) for `oper-status` = `ACTIVE`
        and `firmware-version` = `V_k` on both transceivers, then run the
        **LUG**, the **FVV** for both transceivers with `V` = `V_k`, and the
        **ISC**.
    *   Set `ACTIVE_TARGET_VERSION` to the version of the last image installed
        on both transceivers (`EXPECTED_VERSION` when the matrix has a single
        entry). The primary target is not reinstalled, to avoid an extra flash
        cycle.
*   **Step 5 - History and link verification**:
    *   Close the FHR subscription and evaluate it against the **FHR**.
    *   Run the **DLV**.
*   **Pass/Fail Criteria**: General pass/fail rule, plus:
    *   After every install, each upgraded transceiver reports exactly its
        expected version, and the peer keeps its version during the
        one-at-a-time upgrades in Steps 2 and 3.

### TRANSCEIVER-3.3.3: Verify firmware-version persists across transceiver re-initialization and interface flaps

A module's firmware revision is typically re-read from the module memory map
whenever the host re-initializes the transceiver. This subtest exercises the
re-read paths and checks that streamed updates never expose a transient wrong
value. Configuration leaves are only toggled and restored; no new
configuration is introduced.

*   **Precondition**: TRANSCEIVER-3.3.2 has passed, so both transceivers run
    `ACTIVE_TARGET_VERSION`.
*   **Step 1 - Start history monitoring**:
    *   Start an **FHR subscription** with allowed set
        {`ACTIVE_TARGET_VERSION`} for both transceivers. Keep it open until
        Step 4.
    *   Verify that the initial `firmware-version` updates equal
        `ACTIVE_TARGET_VERSION`.
*   **Step 2 - Transceiver disable/enable** (`bounce_iterations` times;
    default `2`):
    *   Set `/components/component/transceiver/config/enabled` = `false` on
        `Ethernet4/1` and `Ethernet5/1` with one batched `gNMI.Set`.
    *   Wait via `gNMI.Subscribe` (`ON_CHANGE`, timeout `link_down_timeout`)
        until `/components/component/transceiver/state/enabled` = `false` on
        both transceivers and `/interfaces/interface/state/oper-status` is
        `DOWN`, `LOWER_LAYER_DOWN`, or `NOT_PRESENT` on `Ethernet4/1/1` and
        `Ethernet5/1/1`.
    *   Set `/components/component/transceiver/config/enabled` = `true` on
        both transceivers with one batched `gNMI.Set`.
    *   Run the **LUG**.
    *   Read `/components/component/state/firmware-version` on both
        transceivers with one batched `gNMI.Get` and verify both equal
        `ACTIVE_TARGET_VERSION`.
*   **Step 3 - Interface flap** (`flap_iterations` times; default `2`):
    *   Set `/interfaces/interface/config/enabled` = `false` on
        `Ethernet4/1/1`.
    *   Wait via `gNMI.Subscribe` (`ON_CHANGE`, timeout `link_down_timeout`)
        until `/interfaces/interface/state/admin-status` = `DOWN` and
        `/interfaces/interface/state/oper-status` = `DOWN` on
        `Ethernet4/1/1`.
    *   Set `/interfaces/interface/config/enabled` = `true` on
        `Ethernet4/1/1`.
    *   Wait via `gNMI.Subscribe` (`ON_CHANGE`, timeout `link_down_timeout`)
        until `/interfaces/interface/state/admin-status` = `UP` on
        `Ethernet4/1/1`, then run the **LUG**.
    *   Verify `firmware-version` on both transceivers equals
        `ACTIVE_TARGET_VERSION` via one batched `gNMI.Get`.
*   **Step 4 - Evaluate the subscription history**:
    *   Close the FHR subscription and evaluate it against the **FHR**.
*   **Step 5 - Final verification**:
    *   Run the **FVV** for both transceivers with `V` =
        `ACTIVE_TARGET_VERSION`, the **ISC**, and the **DLV**.
*   **Pass/Fail Criteria**: General pass/fail rule, plus:
    *   Every `gNMI.Get` in Steps 2 and 3 returns `ACTIVE_TARGET_VERSION` on
        both transceivers in every iteration.

### TRANSCEIVER-3.3.4: Roll back to the baseline firmware and verify decoding

Downgrading exercises the same decode path in the reverse direction and
returns the optics to their baseline. When `corrupt_firmware_image` is set,
an optional bad-image check then verifies that a rejected install does not
change the reported version. No new configuration is introduced.

*   **Precondition**: TRANSCEIVER-3.3.2 has passed, so both transceivers run
    `ACTIVE_TARGET_VERSION`. This subtest does not depend on
    TRANSCEIVER-3.3.3.
*   **Step 1 - Concurrent rollback**:
    *   Start an **FHR subscription** with allowed set
        {`ACTIVE_TARGET_VERSION`, `baseline_firmware_version`} for both
        transceivers.
    *   Start installation of `baseline_firmware_image` on `Ethernet4/1` and
        `Ethernet5/1` at the same time with the vendor-specific helper.
    *   Wait via `gNMI.Subscribe` (`ON_CHANGE`, timeout `upgrade_timeout`)
        for `/components/component/state/oper-status` = `ACTIVE` and
        `/components/component/state/firmware-version` =
        `baseline_firmware_version` on both transceivers.
*   **Step 2 - Rollback verification**:
    *   Run the **LUG**.
    *   Close the FHR subscription and evaluate it against the **FHR**.
    *   Run the **FVV** for both transceivers with `V` =
        `baseline_firmware_version`.
    *   Run the **ISC** and the **DLV**.
*   **Step 3 - Optional bad-image check** (only when `corrupt_firmware_image`
    is set; otherwise log that the check was skipped):
    *   Start an **FHR subscription** with allowed set
        {`baseline_firmware_version`} for both transceivers.
    *   Attempt to install `corrupt_firmware_image` on `Ethernet4/1` with the
        vendor-specific helper. Log whether the helper returns an error (an
        error is the expected outcome but is not required for pass).
    *   Run the **LUG** with timeout `upgrade_timeout`.
    *   Close the FHR subscription and evaluate it against the **FHR**.
    *   Run the **FVV** for both transceivers with `V` =
        `baseline_firmware_version`, the **ISC**, and the **DLV**.
*   **Pass/Fail Criteria**: General pass/fail rule, plus:
    *   Both transceivers report exactly `baseline_firmware_version` after the
        rollback and, when Step 3 runs, still after the bad-image attempt
        (the corrupted image is not applied and `Ethernet4/1` returns to
        `ACTIVE`).

### Cleanup

Registered with `t.Cleanup()` in test automation so that it runs even if a
subtest fails:

*   Set `/components/component/transceiver/config/enabled` = `true` on
    `Ethernet4/1` and `Ethernet5/1`, and `/interfaces/interface/config/enabled`
    = `true` on `Ethernet4/1/1` and `Ethernet5/1/1`, in case a subtest exited
    early.
*   Install `baseline_firmware_image` on each transceiver that does not
    report `baseline_firmware_version`. Wait for `ACTIVE` and run the **FVV**
    with `V` = `baseline_firmware_version`. (The lab may optionally restore
    `INITIAL_VERSION_P1` / `INITIAL_VERSION_P2` out of band.)
*   Remove the `VRF-ZR` network-instance, the IPv4/IPv6 subinterface
    addresses, and any logical-channel configuration added by this test, and
    revert the optical-channel and interface configuration to the pre-test
    baseline using `gNMI.Set`.
*   Verify via `gNMI.Get` that both transceivers report
    `/components/component/state/oper-status` = `ACTIVE` and that both
    interfaces return to their pre-test `/interfaces/interface/state/oper-status`.

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
  /interfaces/interface/ethernet/config/port-speed:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled:
  /network-instances/network-instance/config/name:
  /network-instances/network-instance/config/type:
  /network-instances/network-instance/interfaces/interface/config/id:
  /network-instances/network-instance/interfaces/interface/config/interface:
  /network-instances/network-instance/interfaces/interface/config/subinterface:
  /terminal-device/logical-channels/channel/config/admin-state:
  /terminal-device/logical-channels/channel/config/index:
  /terminal-device/logical-channels/channel/config/logical-channel-type:
  /terminal-device/logical-channels/channel/config/rate-class:
  /terminal-device/logical-channels/channel/config/trib-protocol:
  /terminal-device/logical-channels/channel/ingress/config/interface:
  /terminal-device/logical-channels/channel/ingress/config/transceiver:
  /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/allocation:
  /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/assignment-type:
  /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/index:
  /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/logical-channel:
  /terminal-device/logical-channels/channel/logical-channel-assignments/assignment/config/optical-channel:

  # Telemetry Parameter coverage
  /components/component/optical-channel/state/frequency:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/operational-mode:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/target-output-power:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/state/firmware-version:
    platform_type: ["TRANSCEIVER"]
  /components/component/state/hardware-version:
    platform_type: ["TRANSCEIVER"]
  /components/component/state/mfg-name:
    platform_type: ["TRANSCEIVER"]
  /components/component/state/oper-status:
    platform_type: ["TRANSCEIVER"]
  /components/component/state/parent:
    platform_type: ["TRANSCEIVER", "OPTICAL_CHANNEL"]
  /components/component/state/part-no:
    platform_type: ["TRANSCEIVER"]
  /components/component/state/serial-no:
    platform_type: ["TRANSCEIVER"]
  /components/component/state/type:
    platform_type: ["TRANSCEIVER", "OPTICAL_CHANNEL"]
  /components/component/transceiver/state/enabled:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/state/form-factor:
    platform_type: ["TRANSCEIVER"]
  /components/component/transceiver/state/present:
    platform_type: ["TRANSCEIVER"]
  /interfaces/interface/state/admin-status:
  /interfaces/interface/state/counters/in-errors:
  /interfaces/interface/state/counters/in-fcs-errors:
  /interfaces/interface/state/counters/in-pkts:
  /interfaces/interface/state/counters/out-errors:
  /interfaces/interface/state/counters/out-pkts:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/state/transceiver:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/state/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/state/link-layer-address:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/state/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/link-layer-address:
  /network-instances/network-instance/state/type:
  /system/state/boot-time:
  /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks:
  /terminal-device/logical-channels/channel/otn/state/pre-fec-ber/instant:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
      on_change: true
      sample: true
  gnoi:
    system.System.Ping:
```

## Required DUT platform

*   FFF or MFF. On MFF, placing DUT port 1 and DUT port 2 on different
    `LINECARD` components is preferred but not required.
