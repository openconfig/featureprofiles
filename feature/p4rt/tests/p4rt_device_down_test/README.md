# P4RT-1.3: P4RT behavior when a device/node is dowm

## Summary

Verify that the P4RT server handles Read/Write RPCs for a device/node as follows:
- Accepts them when the device is available and
- Returns a `NOT_FOUND` error when it is unavailable or down [Point-1 P4RT Spec](https://p4.org/p4-spec/docs/p4runtime-spec-working-draft-html-version.html#_setforwardingpipelineconfig_rpc)

## Testbed type

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

### Initial Setup

*   Connect ATE port-1 and port-2 to DUT port-1 and port-2 respectively.
*   Configure P4RT id and node-id (device_id) with two interfaces on different LineCards.
    * 1st Linecard `device_id = 111`
    * 2nd Linecard `device_id = 222`
*   Disable the Linecard 2 with `device_id = 222`

### P4RT-1.3.1 - Verify that the P4RT server handles Read/Write RPCs as below:

*   For both `device_id = 111` and `device_id = 222`
    *   Send the WBB P4Info via the SetForwardingPipelineConfig
    *   Send RPC Write to install the `AclWbbIngressTableEntry` for LLDP (ethertype: 0x88CC)
*   Verify that the write RPC is successful for `device_id = 111`
*   Validate Verify that the write RPC returns `NOT_FOUND` for `device_id = 222`
*   Send RPC Read to read back the installed table entries. 
*   Verify that the read RPC is successful for `device_id = 111`
*   Validate Verify that the read RPC returns `NOT_FOUND` for `device_id = 222` 

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /components/component/integrated-circuit/config/node-id:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /interfaces/interface/config/id:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```

## Required DUT platform

* MFF

