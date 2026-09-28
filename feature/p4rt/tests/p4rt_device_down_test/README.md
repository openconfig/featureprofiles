# P4RT-1.3: P4RT behavior when a device/node is down

## Summary

Verify that the P4RT server handles StreamChannel, Read, and Write RPCs for a device/node as follows:
- Accepts them when the device is available and
- Returns a `NOT_FOUND` error when it is unavailable or down [Point-1 P4RT Spec](https://p4.org/p4-spec/docs/p4runtime-spec-working-draft-html-version.html#_setforwardingpipelineconfig_rpc)

## Testbed type

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

### Test environment setup

*   Connect ATE port-1 and port-2 to DUT port-1 and port-2 respectively.
*   Configure P4RT id and node-id (device_id) with two interfaces on different integrated circuits (LineCards or forwarding NPUs).
    * 1st Linecard `device_id = 111`
    * 2nd Linecard `device_id = 222`
*   Verify via gNMI that the `oper-status` of both components is `ACTIVE`.

### P4RT-1.3.1 - Verify that the P4RT server handles StreamChannel and RPCs for a down device

* Step 1 - Disable Linecard 2 and verify status
    *   Disable the Linecard 2 with `device_id = 222` using gNMI (e.g., by setting the component's `power-admin-state` to `POWER_DISABLED` or using a platform-specific method to simulate a down state).
    *   Verify via gNMI that the `oper-status` of the component for `device_id = 222` transitions to `DISABLED` or `INACTIVE`.

* Step 2 - Send MasterArbitrationUpdate and Pipeline Config
    *   Attempt to establish a `StreamChannel` and send a `MasterArbitrationUpdate` for `device_id = 222`. Verify it is rejected or returns `NOT_FOUND`.
    *   Send the WBB P4Info via the `SetForwardingPipelineConfig` for both `device_id = 111` and `device_id = 222`.
    *   Verify that `SetForwardingPipelineConfig` is successful for `device_id = 111`.
    *   Verify that `SetForwardingPipelineConfig` returns `NOT_FOUND` for `device_id = 222`.

* Step 3 - Send Write RPC
    *   Send RPC Write to install the `AclWbbIngressTableEntry` for LLDP (ethertype: 0x88CC) for both `device_id = 111` and `device_id = 222`.
    *   Verify that the write RPC is successful for `device_id = 111`.
    *   Verify that the write RPC returns `NOT_FOUND` for `device_id = 222`.

* Step 4 - Send Read RPC
    *   Send RPC Read to read back the installed table entries for both `device_id = 111` and `device_id = 222`. 
    *   Verify that the read RPC is successful for `device_id = 111`.
    *   Verify that the read RPC returns `NOT_FOUND` for `device_id = 222`. 

* Step 5 - Verify PacketOut to Down Device
    *   Send a `PacketOut` message over the `StreamChannel` destined for `device_id = 222`. 
    *   Verify the server drops it or returns an error.

* Step 6 - Invalid Device ID
    *   Attempt to send MasterArbitrationUpdate, SetForwardingPipelineConfig, Write, and Read RPCs to an entirely unconfigured `device_id` (e.g., `device_id = 999`).
    *   Verify they all return `NOT_FOUND`.

#### Canonical OC

```json
{
  "components": {
    "component": [
      {
        "name": "NPU1",
        "integrated-circuit": {
          "config": {
            "node-id": "111"
          }
        }
      },
      {
        "name": "NPU2",
        "integrated-circuit": {
          "config": {
            "node-id": "222"
          }
        }
      }
    ]
  }
}
```

### P4RT-1.3.2 - Verify P4RT server behavior when device state transitions

* Step 1 - State Transition (Down to Up)
    *   Re-enable Linecard 2 with `device_id = 222` via gNMI.
    *   Verify via gNMI that the `oper-status` of the component transitions back to `ACTIVE`.
    *   Verify that the P4RT server now accepts Mastership, `SetForwardingPipelineConfig`, `Write`, and `Read` for `device_id = 222`.

* Step 2 - Device goes down mid-operation (Up to Down)
    *   While `device_id = 222` is up, disable the Linecard 2 via gNMI.
    *   Verify that subsequent `Read` and `Write` RPCs to `device_id = 222` now fail with `NOT_FOUND`.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /components/component/integrated-circuit/config/node-id:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /interfaces/interface/config/id:
  /components/component/state/oper-status:
    platform_type: ["LINECARD", "INTEGRATED_CIRCUIT"]
  /components/component/linecard/config/power-admin-state:
    platform_type: ["LINECARD"]

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```

## Required DUT platform

* MFF
