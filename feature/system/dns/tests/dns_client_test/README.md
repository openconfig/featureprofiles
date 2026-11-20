# System-1.6: DNS client configuration

## Summary

Verify that the DUT accepts DNS client configuration through gNMI and reports
it in state: name servers (IPv4 and IPv6, with port), the search list, and
replace semantics for `/system/dns`. The domain name is covered by
[System-1.3](../../../system_base_test/tests/hostname_test/README.md).

## Testbed type

*   [`featureprofiles/topologies/dut.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut.testbed)

## Procedure

### Test environment setup

*   Save the DUT `/system/dns` config. Restore it after every subtest.
*   No ATE is needed. The DUT does not have to reach the name servers.

### System-1.6.1 - DNS server create and delete

*   Update `/system/dns/servers/server[address=192.0.2.53]/config/address`.
*   Verify the address in config and in
    `/system/dns/servers/server/state/address`.
*   Delete the server. Verify it is gone from config and state.

#### Canonical OC

```json
{
  "system": {
    "dns": {
      "servers": {
        "server": [
          {
            "address": "192.0.2.53",
            "config": {
              "address": "192.0.2.53"
            }
          }
        ]
      }
    }
  }
}
```

### System-1.6.2 - IPv4 and IPv6 DNS servers with port

*   Configure four servers: `192.0.2.53` port 53, `192.0.2.54` port 53,
    `2001:db8::53` port 53, and `2001:db8::54` port 5353.
*   Verify every address and port in config and state, and that the servers are
    listed in the order they were configured.

#### Canonical OC

```json
{
  "system": {
    "dns": {
      "servers": {
        "server": [
          {
            "address": "192.0.2.53",
            "config": {
              "address": "192.0.2.53",
              "port": 53
            }
          },
          {
            "address": "2001:db8::54",
            "config": {
              "address": "2001:db8::54",
              "port": 5353
            }
          }
        ]
      }
    }
  }
}
```

### System-1.6.3 - DNS search list

*   Replace `/system/dns/config/search` with `example.test` and
    `lab.example.test`. Verify config and state.
*   Replace it with only `example.test`. Verify config and state.
*   Delete the search list. Verify config is empty.

#### Canonical OC

```json
{
  "system": {
    "dns": {
      "config": {
        "search": [
          "example.test",
          "lab.example.test"
        ]
      }
    }
  }
}
```

### System-1.6.4 - Replace the DNS container

*   Replace `/system/dns` with servers `192.0.2.53` and `2001:db8::53`, in that
    order, and search `example.test`. Verify the servers and their order in
    config and state.
*   Replace `/system/dns` with the same two servers in reverse order. The server
    list is `ordered-by user`, so config and state must follow the new order.
*   Replace `/system/dns` with server `192.0.2.55` only. Verify that config and
    state list only `192.0.2.55`.

#### Canonical OC

```json
{
  "system": {
    "dns": {
      "config": {
        "search": [
          "example.test"
        ]
      },
      "servers": {
        "server": [
          {
            "address": "192.0.2.55",
            "config": {
              "address": "192.0.2.55"
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
  ## Config paths
  /system/dns/config/search:
  /system/dns/servers/server/config/address:
  /system/dns/servers/server/config/port:

  ## State paths
  /system/dns/state/search:
  /system/dns/servers/server/state/address:
  /system/dns/servers/server/state/port:

rpcs:
  gnmi:
    gNMI.Set:
      replace: true
      update: true
      delete: true
    gNMI.Get:
    gNMI.Subscribe:
```

## Required DUT platform

*   FFF
