# SYS-1.2: System gRPC Servers running in more than one network-instance

## Summary

Ensure that a grpc server serving gnmi can be configured on the DEFAULT
network-instance and a second network-instance named "MGMT".

## Testbed type

* Specify the .testbed topology file from the
  [topologies](https://github.com/openconfig/featureprofiles/tree/main/topologies)
  folder to be used with this test

## Procedure

### Test environment setup

*   Use `--deviation_default_network_instance` for the name of the default
network-instance.
*   Use `--deviation_gnmi_server_name` for the name of the gnmi server.

### SYS-1.2.1: Configure two gnmi servers on different network instances

The DUT is expected to have a gnmi server running on the DEFAULT
network-instance already.  Generate and push the following configuration to the
DUT to add a second gnmi server to the DUT on a different network-instance.

## Canonical OC
```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "test interface",
          "name": "port1"
        },
        "name": "port1",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "mgmtipv4",
                        "prefix-length": 32
                      },
                      "ip": "mgmtipv4"
                    }
                  ]
                }
              }
            }
          ]
        }
      }
    ]
  },
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "GNMI_TEST"
        },
        "interfaces": {
          "interface": [
            {
              "config": {
                "id": "port1",
                "interface": "port1"
              },
              "id": "port1"
            }
          ]
        },
        "name": "GNMI_TEST"
      }
    ]
  },
  "system": {
    "grpc-servers": {
      "grpc-server": [
        {
          "config": {
            "enable": true,
            "name": "gmmi-test",
            "network-instance": "GNMI_TEST",
            "port": 9339
          },
          "name": "gmmi-test"
        }
      ]
    }
  }
}
```

### SYS-1.2.2: Perform set and subscribe to each server

* Set the DUT port1 interface description using the default gnmi connection.
* Subscribe ONCE to the interface port1 description to ensure it was changed.
* Set the DUT port1 interface description using the GNMI_TEST connection.
* Subscribe ONCE to the interface port1 using the default gnmi connection and
ensure it was changed.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /system/grpc-servers/grpc-server/config/name:
  /system/grpc-servers/grpc-server/config/enable:
  /system/grpc-servers/grpc-server/config/port:
  /system/grpc-servers/grpc-server/config/network-instance:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Subscribe:

```
