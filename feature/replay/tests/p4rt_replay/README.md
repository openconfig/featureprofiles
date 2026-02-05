# Replay-1.2: P4RT Replay Test

## Summary

This is an example record/replay test. It is meant to reproduce an error when
replaying P4RT messages.

At this time, no vendor is expected to run this test.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /components/component/config/name:
    platform_type: [INTEGRATED_CIRCUIT]
  /components/component/integrated-circuit/config/node-id:
    platform_type: [INTEGRATED_CIRCUIT]
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
  gribi:
    gRIBI.Flush:
    gRIBI.Get:
    gRIBI.Modify:
```
