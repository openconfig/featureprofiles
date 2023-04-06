# gNMI-1.15: Set Requests

## Summary

Ensures that the device respects certain gNMI SetRequest corner case behaviors.

## Procedure

Each test should be implemented as two variants:

*   ContainerOp: performs a get-modify-set on both `/interfaces` and
    `/network-instances`. The SetRequest should contain two `replace`
    operations, one for each container of the list.
*   ItemOp: SetRequest contains `delete`, `replace` or `update` on the list
    items (e.g. under `/interfaces/interface[name]` and
    `/network-instances/network-instance[name]`).

The results MUST be the same.

### Preparation

*   Allocate two bundle interface names using [netutil.NextBundleInterface]. We
    refer to them as `dut:bundle1` and `dut:bundle2` below.
*   Use `--deviation_default_network_instance` for the name of the default VRF.
*   Use `--deviation_static_protocol_name` for the name of the static protocol.

[netutil.NextBundleInterface]: https://pkg.go.dev/github.com/openconfig/ondatra/netutil#NextBundleInterface

### Test: Delete to Reset

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

1.  Initialize the interfaces in the same SetRequest.

    *   Delete `dut:port1` and `dut:port2`.
    *   Configure `dut:bundle1` with member `dut:port1` and IP address
        192.0.2.1/30.
    *   Configure `dut:bundle2` with member `dut:port2` and IP address
        192.0.2.5/30.

    Verify through telemetry that these interfaces are configured correctly.

2.  Modify the interfaces in the same SetRequest:

    *   Delete `dut:bundle1`.
    *   Configure `dut:bundle2` to have the IP address 192.0.2.1/30.

    Verify through telemetry that `dut:bundle2` has the correct IP address.

3.  Clean up by deleting `dut:bundle2`.

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

### Test: Move Interfaces from Default VRF to Non-Default VRF

This test checks that interfaces can be moved from the default VRF to a
non-default VRF while preserving the interface configs.

1.  Initialize the attachment in the same SetRequest:

    *   Configure `dut:port1` with IP address 192.0.2.1/30.
    *   Configure `dut:port2` with IP address 192.0.2.5/30.
    *   Attach both interfaces to the default VRF.

    Verify through telemetry that these interfaces are configured correctly and
    attached to the default VRF.

2.  Modify attachment in the same SetRequest:

    *   Detach `dut:port1` and `dut:port2` from the default VRF.
    *   In the ContainerOp variant, also replace the interfaces `dut:port1` and
        `dut:port2` with exactly the same config as before.
    *   Configure a non-default VRF `BLUE` attaching `dut:port1` and
        `dut:port2`.

3.  Verify through telemetry:

    *   The IP addresses of `dut:port1` and `dut:port2` are as expected.
    *   The `dut:port1` and `dut:port2` interfaces are attached to VRF `BLUE`.

4.  Clean up by deleting VRF `BLUE`.

### Test: Move Interfaces from Non-Default VRF to Non-Default VRF

This test checks that interfaces can be moved from a non-default VRF to another
non-default VRF while preserving the interface configs.

1.  Initialize the attachment in the same SetRequest:

    *   Configure `dut:port1` with IP address 192.0.2.1/30.
    *   Configure `dut:port2` with IP address 192.0.2.5/30.
    *   Configure a non-default VRF `RED` attaching both interfaces.

    Verify through telemetry that these interfaces are configured correctly and
    attached to the default VRF.

2.  Modify attachment in the same SetRequest:

    *   Delete VRF `RED`.
    *   In the ContainerOp variant, also replace the interfaces `dut:port1` and
        `dut:port2` with exactly the same config as before.
    *   Configure a non-default VRF `BLUE` attaching `dut:port1` and
        `dut:port2`.

3.  Verify through telemetry:

    *   The IP addresses of `dut:port1` and `dut:port2` are as expected.
    *   The `dut:port1` and `dut:port2` interfaces are attached to VRF `BLUE`.

4.  Clean up by deleting VRF `BLUE`.

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
