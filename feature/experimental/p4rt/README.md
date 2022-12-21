# P4RT Implementation Guide

This document specifies the requirements for p4rt test implementation.

1.  Use the [cisco-open/go-p4 library](https://github.com/cisco-open/go-p4).

2.  The client should import or make use of the following WBB information in the
    following Google compatible format:

    1.  WBB P4 Protobuf file:
        https://github.com/openconfig/featureprofiles/blob/main/feature/experimental/p4rt/wbb.p4info.pb.txt

3.  The client should make use of Ondatra Raw API
    `dut.RawAPIs().P4RT().Default(t)`

4.  Use `p4rtutils.P4RTNodesByPort()` to get the p4rt node name instead of
    passing the node names through flags.
