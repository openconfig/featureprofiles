# gNMI-1.12: Mixed OpenConfig/CLI Origin

## Summary

Ensure that both CLI and OC configuration can be pushed to the device within the
same `SetRequest`, with the CLI as a `replace` operation and the OC as an
`update` operation.

e.g. QoS

```textproto
SetRequest:
prefix:  {
  target:  "device-name"
}
replace:  {
  path:  {
    origin:  "cli"
  }
  val:  {
    ascii_val:  "! comment\nusername admin role network-admin secret 3 foobarbaz\nusername foo privilege 23 role network-admin secret 4 tuesday\n!\nend\n"
  }
}
update:  {
  path:  {
    origin:  "openconfig"
    elem:  {
      name:  "qos"
    }
  }
  val:  {
    json_ietf_val:  "{\n  \"openconfig-qos:forwarding-groups\": {\n    \"forwarding-group\": [\n      {\n        \"config\": {\n          \"name\": \"target-group-BE0\",\n          \"output-queue\": \"BE0\"\n        },\n        \"name\": \"target-group-BE0\"\n      }\n    ]\n  },\n  \"openconfig-qos:queues\": {\n    \"queue\": [\n      {\n        \"config\": {\n          \"name\": \"BE0\"\n        },\n        \"name\": \"BE0\"\n      }\n    ]\n  }\n}"
  }
}
```

## Procedure

1.  Make sure QoS queue under test is not already set.
2.  Retrieve current running-config.
3.  Send mixed-origin SetRequest.
4.  Verify QoS queue configuration has been accepted by the target.

## Config Parameter Coverage

*   origin: "cli"
*   /qos/forwarding-groups/forwarding-group/config/output-queue
*   /qos/queues/queue/config/name

## Telemetry Parameter Coverage

*   None
