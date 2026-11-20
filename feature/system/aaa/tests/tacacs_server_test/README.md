# SYS-3.2: TACACS server configuration

## Summary

Create a TACACS server group with an IPv4 server and an IPv6 server, then
delete the group. SpyTest writes one server address and port and does not
write a shared key. This test also sets a per-server timeout. It does not
set a secret, a user, or an authentication method. Those steps are described
in [SYS-3.1](../tacacs/README.md), which does not yet have a Go test.

## Testbed type

*   [`featureprofiles/topologies/dut.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut.testbed)

## Procedure

*   Replace server group `FP-TACACS` with type TACACS.
*   Add `192.0.2.49` port 49 timeout 4, and `2001:db8::49` port 49 timeout 5.
*   Read type, address, timeout, and port from config. Read each port from state.
*   Delete the group and check it is gone.

#### Canonical OC

```json
{
  "system": {
    "aaa": {
      "server-groups": {
        "server-group": [
          {
            "name": "FP-TACACS",
            "config": {"name": "FP-TACACS", "type": "TACACS"},
            "servers": {
              "server": [
                {
                  "address": "192.0.2.49",
                  "config": {"address": "192.0.2.49", "timeout": 4},
                  "tacacs": {"config": {"port": 49}}
                }
              ]
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
  /system/aaa/server-groups/server-group/config/name:
  /system/aaa/server-groups/server-group/config/type:
  /system/aaa/server-groups/server-group/servers/server/config/address:
  /system/aaa/server-groups/server-group/servers/server/config/timeout:
  /system/aaa/server-groups/server-group/servers/server/tacacs/config/port:
  /system/aaa/server-groups/server-group/servers/server/tacacs/state/port:
rpcs:
  gnmi:
    gNMI.Set:
      replace: true
      delete: true
    gNMI.Subscribe:
```
