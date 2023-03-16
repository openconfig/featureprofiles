# gNMI-1.12: Mixed OpenConfig/CLI Origin

## Summary

Ensure that both CLI and OC configuration can be pushed to the device within the
same `SetRequest`, with OC config as a `replace` operation and the CLI as an
`update` operation. Note that this implies stale CLI config may remain after the
`SetRequest` operation.

e.g. QoS

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
    ascii_val:  "qos traffic-class 0 name target-group-BE0\nqos tx-queue 0 name BE0"
  }
}
```

## Procedure

1.  Make sure QoS queue under test is not already set.
2.  Retrieve current OpenConfig and CLI configs.
3.  Validate that device can accept root replace of current OC config without
    any changes.
4.  Send mixed-origin SetRequest.
5.  Verify QoS queue configuration has been accepted by the target.

## Config Parameter Coverage

*   origin: "cli"
*   /qos/forwarding-groups/forwarding-group/config/output-queue
*   /qos/queues/queue/config/name

## Telemetry Parameter Coverage

*   None
