# gNMI-1.8: Configuration Metadata-only Retrieve and Replace

## Summary

Ensure that when a replace operation is done on the metadata-only path metadata
is replaced, ensure that metadata can be received and is persisted over a device
reboot.

## Procedure

*   Build a protobuf message, marshal, and base64 encode the result.
*   Make a gNMI Set request for the path
    `/@/openconfig-metadata:protobuf-metadata` whose value is the base64 encoded
    result.
*   Make a gNMI Get for the same path to retrieve the value back.
*   Validate that the returned value can be base64 decoded, unmarshaled, and
    matches the original protobuf.

## Config Parameter Coverage

No configuration coverage.

## Telemetry Parameter Coverage

*   /@/openconfig-metadata:protobuf-metadata

Note: WBB implementations need not support this annotation at paths deeper than
the root (i.e., a configuration that contains
`openconfig-metadata:protobuf-metadata` at any level other than under the root
can be rejected). The WBB device implementation can map this to an internal path
to store the configuration.
