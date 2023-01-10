# P4RT Implementation Guide

This document specifies the requirements for p4rt test implementation.

1.  Use the [cisco-open/go-p4 library](https://github.com/cisco-open/go-p4).

2.  The client should import or make use of the following WBB information in the
    following Google compatible format:

    1.  WBB P4 Protobuf file:
        https://github.com/openconfig/featureprofiles/blob/main/feature/experimental/p4rt/wbb.p4info.pb.txt

3.  Tests should instantiate new P4RT clients using
    `p4rt_client.NewP4RTClient()` function. This instantiates the
    `StreamTermErr` channel required to investigate errors when the p4rt stream
    terminates.

4.  Tests should make use of Ondatra Raw API `dut.RawAPIs().P4RT().Default(t)`
    during client instantiation.

5.  To avoid indefinite blocking during Get response
    `client.StreamChannelGetArbitrationResp()`, tests should make use of
    `p4rtutils.StreamTermErr()` function which returns an Error if the p4rt
    stream has been terminated.

6.  Tests should get the P4RT Node Name by walking the Components OC tree.
    Components of type `INTEGRATED_CIRCUIT` should have child Components of type
    `PORT`. These PORT Components can be mapped to currently reserved Interfaces
    using the `hardware-port` leaf in the Interfaces tree. Such an
    implementation already exists in `p4rtutils` library:
    `p4rtutils.P4RTNodesByPort()`.

7.  If P4RT Node Names cannot be resolved by walking the Components tree, use
    deviation flag `deviation_explicit_p4rt_node_component` and pass the node
    names through args `arg_p4rt_node_name_1`, `arg_p4rt_node_name_2`.
