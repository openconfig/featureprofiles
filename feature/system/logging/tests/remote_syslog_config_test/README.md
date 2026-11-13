# TR-6.3: Remote syslog configuration

## Summary

Create one remote syslog server with a non-default port and TLS profile, then
delete it. SpyTest also writes an SSL profile and a source interface. Those
leaves are not in the OpenConfig logging structs this module vendors, so they
are not set here. Delivery of a syslog message is
[TR-6.1](../../otg_tests/remote_syslog/README.md).

## Testbed type

*   [`featureprofiles/topologies/dut.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut.testbed)

## Procedure

*   Replace `/system/logging/remote-servers/remote-server[host=192.0.2.51]`
    with remote port 6514, transport-security true, and tls-profile-id
    `SYSLOG-TLS`.
*   Read host, port, transport-security, and tls-profile-id from config.
    Read transport-security and tls-profile-id from state.
*   Delete the server and check it is gone from config.

#### Canonical OC

```json
{
  "system": {
    "logging": {
      "remote-servers": {
        "remote-server": [
          {
            "host": "192.0.2.51",
            "config": {
              "host": "192.0.2.51",
              "remote-port": 6514,
              "transport-security": true,
              "tls-profile-id": "SYSLOG-TLS"
            }
          }
        ]
      }
    }
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /system/logging/remote-servers/remote-server/config/host:
  /system/logging/remote-servers/remote-server/config/remote-port:
  /system/logging/remote-servers/remote-server/config/transport-security:
  /system/logging/remote-servers/remote-server/config/tls-profile-id:
  /system/logging/remote-servers/remote-server/state/transport-security:
  /system/logging/remote-servers/remote-server/state/tls-profile-id:
rpcs:
  gnmi:
    gNMI.Set:
      replace: true
      delete: true
    gNMI.Subscribe:
```
