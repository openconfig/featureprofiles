# gNMI-1.12: Mixed OpenConfig/CLI Origin

## Summary

Ensure that both CLI and OC configuration can be pushed to the device within the
same `SetRequest`, with OC config as a `replace` operation and the CLI as an
`update` operation. Note that this implies stale CLI config may remain after the
`SetRequest` operation.

## Procedure

1.  Delete the `TEST` queue in OpenConfig in case it is still there, and check
    that it is no longer present.
2.  Retrieve currently-running OpenConfig and CLI configs.
3.  Validate that device can accept root replace of current OC config without
    any changes (currently skipped).
4.  Construct and send mixed-origin SetRequest.
    1.  CLI configuration consists of the below example, where a name is given
        to a traffic class and a queue.
    2.  Modify currently-running OpenConfig to create the queue and traffic
        classes as per named via CLI, and map the queue to the traffic class.
5.  Verify QoS queue and traffic class configuration has been accepted by the
    target.
6.  Repeat above steps, but replacing on the `/qos` path instead of at root
    level (root-level test currently skipped).

The configuration used in this test is a QoS configuration wherein the
OpenConfig configuration depends on the CLI configuration:

Arista Example:

```textproto
SetRequest:
prefix:  {
  target:  "device-name"
}
replace:  {
  path:  {
    origin:  "openconfig"
  }
  val:  {
    json_ietf_val:  "{\n  full config omitted \n}"
  }
}
update:  {
  path:  {
    origin:  "cli"
  }
  val:  {
    ascii_val:  "qos traffic-class 0 name target-group-TEST\nqos tx-queue 0 name TEST"
  }
}
```

TODO: Support other vendor CLIs and place examples here.

## Config Parameter Coverage

*   origin: "cli"
*   /qos/forwarding-groups/forwarding-group/config/output-queue
*   /qos/queues/queue/config/name

## Telemetry Parameter Coverage

*   None
