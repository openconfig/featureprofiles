# OC-26.1: Network Time Protocol (NTP)

## Summary

Ensure DUT can be configured as a Network Time Protocol (NTP) client.

## Procedure

*   For the following cases, enable NTP on the DUT and validate telemetry reports the servers are configured:
    *   4x IPv4 NTP server in default VRF#### Canonical OC
    *   4x IPv6 NTP server in default VRF
    *   4x IPv4 NTP server in non-default VRF
    *   4x IPv6 NTP server in non-default VRF
 
*   The source address of the ipv4 and ipv6 NTP servers will be Loopback ipv4 and ipv6 source addresses respectively.  

#### Canonical OC
```json
{
  "openconfig-network-instance:network-instances": {
    "network-instance": [
      {
        "name": "VRF-1",
        "config": {
          "name": "VRF-1",
          "type": "openconfig-network-instance-types:L3VRF"
        }
      }
    ]
  },
  "openconfig-system:system": {
    "ntp": {
      "config": {
        "enabled": true
      },
      "servers": {
        "server": [
          {
            "address": "192.0.2.1",
            "config": {
              "address": "192.0.2.1",
              "source-address": "203.0.113.1"
            }
          },
          {
            "address": "192.0.2.2",
            "config": {
              "address": "192.0.2.2",
              "source-address": "203.0.113.1"
            }
          },
          {
            "address": "192.0.2.3",
            "config": {
              "address": "192.0.2.3",
              "source-address": "203.0.113.1"
            }
          },
          {
            "address": "192.0.2.4",
            "config": {
              "address": "192.0.2.4",
              "source-address": "203.0.113.1"
            }
          },
          {
            "address": "2001:db8::1",
            "config": {
              "address": "2001:db8::1",
              "source-address": "2001:db8::1:1:1:1"
            }
          },
          {
            "address": "2001:db8::2",
            "config": {
              "address": "2001:db8::2",
              "source-address": "2001:db8::1:1:1:1"
            }
          },
          {
            "address": "2001:db8::3",
            "config": {
              "address": "2001:db8::3",
              "source-address": "2001:db8::1:1:1:1"
            }
          },
          {
            "address": "2001:db8::4",
            "config": {
              "address": "2001:db8::4",
              "source-address": "2001:db8::1:1:1:1"
            }
          },
          {
            "address": "192.0.2.10",
            "config": {
              "address": "192.0.2.10",
              "network-instance": "VRF-1",
              "source-address": "203.0.113.1"
            }
          },
          {
            "address": "192.0.2.11",
            "config": {
              "address": "192.0.2.11",
              "network-instance": "VRF-1",
              "source-address": "203.0.113.1"
            }
          },
          {
            "address": "192.0.2.12",
            "config": {
              "address": "192.0.2.12",
              "network-instance": "VRF-1",
              "source-address": "203.0.113.1"
            }
          },
          {
            "address": "192.0.2.9",
            "config": {
              "address": "192.0.2.9",
              "network-instance": "VRF-1",
              "source-address": "203.0.113.1"
            }
          },
          {
            "address": "2001:db8::9",
            "config": {
              "address": "2001:db8::9",
              "network-instance": "VRF-1",
              "source-address": "2001:db8::1:1:1:1"
            }
          },
          {
            "address": "2001:db8::a",
            "config": {
              "address": "2001:db8::a",
              "network-instance": "VRF-1",
              "source-address": "2001:db8::1:1:1:1"
            }
          },
          {
            "address": "2001:db8::b",
            "config": {
              "address": "2001:db8::b",
              "network-instance": "VRF-1",
              "source-address": "2001:db8::1:1:1:1"
            }
          },
          {
            "address": "2001:db8::c",
            "config": {
              "address": "2001:db8::c",
              "network-instance": "VRF-1",
              "source-address": "2001:db8::1:1:1:1"
            }
          }
        ]
      }
    }
  }
}
```

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

TODO(OCPATH): Populate path from test already written.

```yaml
paths:
  ## Config paths
  /system/ntp/config/enabled:
  /system/ntp/servers/server/config/address:
  #[TODO]/system/ntp/servers/server/config/source-address:
  /system/ntp/servers/server/config/network-instance:

  ## State paths
  /system/ntp/servers/server/state/address:
  #[TODO]/system/ntp/servers/server/state/source-address:
  #[TODO]/system/ntp/servers/server/state/port:
  /system/ntp/servers/server/state/network-instance:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```
