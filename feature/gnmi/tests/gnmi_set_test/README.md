# gNMI-1.15: Set Requests

## Summary

Ensures that the device respects certain gNMI SetRequest corner case behaviors.

## Topology

*   ATE port-1 and DUT port-1
*   ATE port-2 and DUT port-2

## Procedure

Each test should be implemented as three variants:

*   RootOp: performs a get-modify-set of the full config at root. The SetRequest
    contains one `replace` operation.
*   ContainerOp: performs a get-modify-set on both `/interfaces` and
    `/network-instances`. The SetRequest contains two `replace` operations, one
    for each container of the list.
*   ItemOp: SetRequest contains `delete`, `replace` or `update` on the list
    items (e.g. under `/interfaces/interface[name]` and
    `/network-instances/network-instance[name]`).

The results MUST be the same.

Notes:

*   Use `--deviation_default_network_instance` for the name of the default VRF.
*   Use `--deviation_static_protocol_name` for the name of the static protocol.

### Test: Get and Set

This test checks that the config read from the device can be written back.

1.  Obtain the full config at root using gNMI Get.
2.  Deploy the config back to the same device using a gNMI SetRequest.

### Test: Delete Interface

This test checks that the config of a physical interface can be reset to the
default value using the delete operation.

1.  Initialize the interfaces in the same SetRequest.

    *   Configure `dut:port1` with the description `dut:port1`.
    *   Configure `dut:port2` with the description `dut:port2`.

    Verify through telemetry that these interfaces are configured correctly.

2.  Delete `dut:port1` and `dut:port2` in the same SetRequest.

    In the ContainerOp variant, delete the interfaces by omission.

    Verify through telemetry that the interfaces still exist, but the
    description has been reset to no value.

### Test: Reuse IP

This test checks that the IP address of a deleted interface can be immediately
reused by another interface.

Allocate two aggregate interface names using [netutil.NextAggregateInterface].
We refer to them as `dut:agg1` and `dut:agg2` below.

[netutil.NextAggregateInterface]: https://pkg.go.dev/github.com/openconfig/ondatra/netutil#NextAggregateInterface

1.  Initialize the interfaces in the same SetRequest.

    *   Delete `dut:port1`, `dut:port2`, `dut:agg1` and `dut:agg2`.
    *   Configure `dut:agg1` with member `dut:port1` and IP address
        192.0.2.1/30.
    *   Configure `dut:agg2` with member `dut:port2` and IP address
        192.0.2.5/30.

    Verify through telemetry that these interfaces are configured correctly.

2.  Modify the interfaces in the same SetRequest:

    *   Delete `dut:agg1`.
    *   Configure `dut:agg2` to have the IP address 192.0.2.1/30.

    Verify through telemetry that `dut:agg2` has the correct IP address.

3.  Clean up by deleting `dut:agg2`.

### Test: Swap IPs

This test checks that the IP addresses of two interfaces can be swapped in the
same SetRequest.

1.  Initialize the interfaces in the same SetRequest:

    *   Configure `dut:port1` with IP address 192.0.2.1/30.
    *   Configure `dut:port2` with IP address 192.0.2.5/30.

    Verify through telemetry that these interfaces are configured correctly.

2.  Modify the interfaces in the same SetRequest:

    *   Set `dut:port1` address to 192.0.2.5/30.
    *   Set `dut:port2` address to 192.0.2.1/30.

    Verify through telemetry that the interfaces have the correct IP addresses.

### Test: Delete Non-Existing VRF

This test checks that a non-existing VRF can be deleted.

1.  Initialize by making sure the VRF `GREEN` does not exist.

    This is no-op for ContainerOp and RootOp. Only ItemOp will generate a DELETE
    operation in the SetRequest. The request should succeed.

### Test: Delete Non-Default VRF

This test checks that a non-default VRF can be deleted.

1.  Initialize the interfaces in the same SetRequest:

    *   Configure `dut:port1` with IP address 192.0.2.1/30.
    *   Configure `dut:port2` with IP address 192.0.2.5/30.
    *   Configure a non-default VRF `BLUE` attaching both interfaces.

    Verify through telemetry that these interfaces are configured correctly and
    attached to the non-default VRF.

2.  Clean up by deleting VRF `BLUE`.

    Verify through telemetry that the VRF is not present.

### Test: Move Interfaces Between VRFs

This test checks that interfaces can be moved from one VRF to a different VRF
while preserving the interface configs.

There should be two variants of this test:

*   Moving from the default VRF to non-default VRF `BLUE`.
*   Moving from non-default VRF `RED` to another non-default VRF `BLUE`.

Steps:

1.  Initialize the attachment in the same SetRequest:

    *   Configure `dut:port1` with IP address 192.0.2.1/30.
    *   Configure `dut:port2` with IP address 192.0.2.5/30.
    *   Attach both interfaces to the first VRF. Create the first VRF as L3VRF
        if it is not the default.

    Verify through telemetry that these interfaces are configured correctly and
    attached to the first VRF.

2.  Modify attachment in the same SetRequest:

    *   Detach `dut:port1` and `dut:port2` from the first VRF. If the first VRF
        is not the default VRF, delete it.
    *   In the ContainerOp variant, also replace the interfaces `dut:port1` and
        `dut:port2` with exactly the same config as before.
    *   Configure the second VRF as L3VRF attaching `dut:port1` and `dut:port2`.

3.  Verify through telemetry:

    *   The IP addresses of `dut:port1` and `dut:port2` are as expected.
    *   The `dut:port1` and `dut:port2` interfaces are attached to the second
        VRF.

4.  Clean up by deleting the second VRF.

### Test: Static Protocol

This test checks that the static protocol name is usable.

1.  Initialize the attachment in the same SetRequest:

    *   Configure `dut:port1` with IP address 192.0.2.1/30.
    *   Configure `dut:port2` with IP address 192.0.2.5/30.
    *   Configure a non-default VRF `BLUE` attaching both interfaces.
    *   Configure the static routes in VRF `BLUE` as follows:
        *   Prefix 198.51.100.0/24 has next-hop 192.0.2.2 and interface
            `dut:port1`.
        *   Prefix 203.0.113.0/24 has next-hop 192.0.2.6 and interface
            `dut:port2`.

    Verify through telemetry that the static routes are configured correctly.

2.  Modify the static routes in VRF `BLUE` as follows in the same SetRequest.

    *   Prefix 198.51.100.0/24 has next-hop 192.0.2.6 and interface `dut:port2`.
    *   Prefix 203.0.113.0/24 has next-hop 192.0.2.2 and interface `dut:port1`.

    Verify through telemetry that the static routes are configured correctly.

3.  Clean up by deleting VRF `BLUE`.

## RPC Coverage

*   gNMI.Set

## OpenConfig Path and RPC Coverage

```yaml
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Subscribe:

```
