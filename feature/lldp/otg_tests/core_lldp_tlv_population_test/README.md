# RT-6.1: Core LLDP TLV Population

## Summary

Determine that LLDP advertisement, reception, per-interface counters and packet
timestamps, and global/per-interface enablement operate correctly over gNMI.

## Testbed type

*   [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

### Test environment setup

*   Connect ATE `port-1` to DUT `port-1`.

### RT-6.1.1: Global and Per-Interface LLDP Configuration and Neighbor Telemetry

*   Configure global LLDP parameters on DUT via `gNMI.Set`:
    *   `/lldp/config/enabled` = `true`
    *   `/lldp/config/hello-timer` = `20` (seconds)
    *   `/lldp/config/ttl` = `80` (seconds)
    *   `/lldp/config/system-description` = `"DUT"`
*   Configure per-interface LLDP parameters on DUT `port-1` via `gNMI.Set`:
    *   `/interfaces/interface/config/enabled` = `true`
    *   `/lldp/interfaces/interface/config/name` = `port-1`
    *   `/lldp/interfaces/interface/config/enabled` = `true`
    *   `/lldp/interfaces/interface/config/port-description` = `"DUT port-1"`
*   Configure ATE `port-1` with LLDP enabled (`enabled=true`), advertising
    Chassis ID (`MAC_ADDRESS`), Port ID (`INTERFACE_NAME`), Port Description,
    System Name, System Description, and Management Address (`192.0.2.2`).
*   Verify DUT global LLDP operational state via `gNMI.Get` / `gNMI.Subscribe`:
    *   `/lldp/state/enabled` reports `true`.
    *   `/lldp/state/hello-timer` reports `20`.
    *   `/lldp/state/ttl` reports `80`.
    *   `/lldp/state/chassis-id`, `/lldp/state/chassis-id-type`,
        `/lldp/state/system-name`, and `/lldp/state/system-description`
        (`"DUT"`) are populated with expected DUT values.
*   Verify DUT per-interface LLDP operational state on `port-1`:
    *   `/lldp/interfaces/interface/state/name` matches `port-1`.
    *   `/lldp/interfaces/interface/state/enabled` reports `true`.
    *   `/lldp/interfaces/interface/state/port-description` reports
        `"DUT port-1"`.
*   Determine that telemetry from ATE reports correct LLDP TLV values received
    from DUT (Chassis ID, Chassis ID Type, Port ID, Port ID Type, Port
    Description `"DUT port-1"`, System Name, System Description `"DUT"`, and TTL
    `80`).
*   Establish a streaming `gNMI.Subscribe` (`ON_CHANGE` mode) subscription on
    DUT for `/lldp/interfaces/interface/neighbors/neighbor/state/` on `port-1`
    and verify that DUT reports correct neighbor TLV values from ATE:
    *   `/lldp/interfaces/interface/neighbors/neighbor/state/chassis-id`
    *   `/lldp/interfaces/interface/neighbors/neighbor/state/chassis-id-type`
    *   `/lldp/interfaces/interface/neighbors/neighbor/state/port-id`
    *   `/lldp/interfaces/interface/neighbors/neighbor/state/port-id-type`
    *   `/lldp/interfaces/interface/neighbors/neighbor/state/port-description`
    *   `/lldp/interfaces/interface/neighbors/neighbor/state/system-name`
    *   `/lldp/interfaces/interface/neighbors/neighbor/state/system-description`
    *   `/lldp/interfaces/interface/neighbors/neighbor/state/management-address`
        (`192.0.2.2`)

### RT-6.1.2: Per-Interface LLDP Counters and Packet Timestamps

*   Establish a streaming `gNMI.Subscribe` (`SAMPLE` mode) subscription on DUT
    `port-1` for:
    *   `/lldp/interfaces/interface/state/counters/frame-in`
    *   `/lldp/interfaces/interface/state/counters/frame-out`
    *   `/lldp/interfaces/interface/state/counters/frame-discard`
    *   `/lldp/interfaces/interface/state/last-packet-transmitted`
    *   `/lldp/interfaces/interface/state/last-good-packet-received`
*   Observe telemetry over at least two `hello-timer` intervals and verify:
    *   `frame-in` and `frame-out` are greater than `0` and increment as LLDP
        PDUs are received from ATE and transmitted by DUT.
    *   `frame-discard` is reported (`0` during valid LLDP PDU exchange, and
        increments when malformed/discardable LLDP frames missing mandatory TLVs
        are sent from ATE `port-1`).
    *   `last-packet-transmitted` and `last-good-packet-received` report valid
        non-zero `timeticks64` nanosecond timestamps that advance upon
        subsequent LLDP PDU transmission and reception.

### RT-6.1.3: Per-Interface and Global LLDP Disable Behavior

*   Configure LLDP disabled at the interface level on DUT `port-1`:
    *   Set `/lldp/interfaces/interface/config/enabled` to `false` while
        `/lldp/config/enabled` remains `true`.
    *   Verify `/lldp/interfaces/interface/state/enabled` reports `false` and
        DUT ceases generating LLDP messages on `port-1` (`frame-out` stops
        incrementing).
    *   Re-enable `/lldp/interfaces/interface/config/enabled` (`true`) and
        verify `/lldp/interfaces/interface/state/enabled` returns to `true` and
        LLDP neighbor state is restored.
*   Configure LLDP disabled at global level for DUT:
    *   Set `/lldp/config/enabled` to `false`.
    *   Verify `/lldp/state/enabled` reports `false`.
    *   Ensure that DUT does not generate any LLDP messages irrespective of the
        configuration of `/lldp/interfaces/interface/config/enabled` (`true` or
        `false`) on any interface.

### Cleanup

*   Revert all modified global and per-interface LLDP and interface
    configurations on DUT and stop protocols on ATE to restore the testbed to
    its baseline operational state.

## Canonical OC

```json
{
  "openconfig-lldp:lldp": {
    "config": {
      "enabled": true,
      "hello-timer": 20,
      "system-description": "DUT",
      "ttl": 80
    },
    "interfaces": {
      "interface": [
        {
          "name": "port1",
          "config": {
            "enabled": true,
            "name": "port1",
            "port-description": "DUT port-1"
          }
        }
      ]
    }
  },
  "openconfig-interfaces:interfaces": {
    "interface": [
      {
        "name": "port1",
        "config": {
          "enabled": true,
          "name": "port1",
          "type": "iana-if-type:ethernetCsmacd"
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml
paths:
  ## Config Paths ##
  /lldp/config/enabled:
  /lldp/config/hello-timer:
  /lldp/config/system-description:
  /lldp/config/ttl:
  /lldp/interfaces/interface/config/enabled:
  /lldp/interfaces/interface/config/name:
  /lldp/interfaces/interface/config/port-description:

  ## State Paths ##
  /lldp/interfaces/interface/neighbors/neighbor/state/chassis-id:
  /lldp/interfaces/interface/neighbors/neighbor/state/chassis-id-type:
  /lldp/interfaces/interface/neighbors/neighbor/state/management-address:
  /lldp/interfaces/interface/neighbors/neighbor/state/port-description:
  /lldp/interfaces/interface/neighbors/neighbor/state/port-id:
  /lldp/interfaces/interface/neighbors/neighbor/state/port-id-type:
  /lldp/interfaces/interface/neighbors/neighbor/state/system-description:
  /lldp/interfaces/interface/neighbors/neighbor/state/system-name:
  /lldp/interfaces/interface/state/counters/frame-discard:
  /lldp/interfaces/interface/state/counters/frame-in:
  /lldp/interfaces/interface/state/counters/frame-out:
  /lldp/interfaces/interface/state/enabled:
  /lldp/interfaces/interface/state/last-good-packet-received:
  /lldp/interfaces/interface/state/last-packet-transmitted:
  /lldp/interfaces/interface/state/name:
  /lldp/interfaces/interface/state/port-description:
  /lldp/state/chassis-id:
  /lldp/state/chassis-id-type:
  /lldp/state/enabled:
  /lldp/state/hello-timer:
  /lldp/state/system-description:
  /lldp/state/system-name:
  /lldp/state/ttl:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

vRX
