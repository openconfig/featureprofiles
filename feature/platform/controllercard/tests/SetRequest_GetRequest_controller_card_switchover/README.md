# gNMI-1.19: ConfigPush and ConfigPull after Control Card switchover

## Summary
This test verifies if a large config can be pushed and/or pulled via gNMI SetRequest/GetRequest within 2 minutes after Control Card switchover.

## Procedure

* Prepare a large OpenConfig config file to be pushed within a single setRequest.
  * 150 LAG interfaces w/ ip, ipv6 configuration
  * 800 Ethernet interfaces as member s of LAG
  * 28 IPv4 and 28 IPv6 BGP neighbors
  * ISIS on all trunk/LAG ports

### sub-Test 1: SetRequest
[TODO: [issue #2407](https://github.com/openconfig/featureprofiles/issues/2407) ]
* Store indexes of ACTIVE and BACKUP Controller Card in "previous_ACTIVE" and "previous_BACKUP"
* Initiate Control Card switchover using gNOI SwitchControlProcessorRequest; store timestamp in "SwitchControlProcessorRequest_time"
* Wait for `SwitchControlProcessorResponse` but no longer than 120s. If not received, test FAILED.
* Immediately after receiving `SwitchControlProcessorResponse` for gNOI switchover, send gNMI `setRequest` with a prepared large config. Store timestamp as "SwitchControlProcessorResponse_time".
* Wait for `SetResponse` but no longer than 30s.
  * If not received, the test waits 10s and sends gNMI `setRequest` with prepared large config. Repeat Wait for `SetResponse`.
  * If received at time <= "SwitchControlProcessorResponse"+110s and a non-zero grpc status code is returned, wait 10s and send gNMI `setRequest` with prepared large config. Repeat Wait for `SetResponse`
  * If received at time > "SwitchControlProcessorResponse"+110s and a non-zero grpc status code is returned, test FAILED
  * If received at time <= "SwitchControlProcessorResponse"+120s and SUCCESS is returned, proceed
* Retrieve configuration from DUT using gNMI `GetRequest`.
* Verify:
  * The gNMI `SetResponse` has been received within 120s after `setRequest` by comparing with "SwitchControlProcessorResponse_time", and
  * The gNOI `SwitchControlProcessorResponse` has been received and switchover was executed by DUT (compare "previous_ACTIVE" with DUT state), and
  * Count the logical elements in the response to ensure the total number of configured LAG interfaces and BGP neighbors exactly matches the intended configuration.

### sub-Test 2: GetRequest
[TODO: [issue #2451](https://github.com/openconfig/featureprofiles/issues/2451) ]
* Store indexes of ACTIVE and BACKUP Controller Card in "previous_ACTIVE" and "previous_BACKUP"
* **Verify (Before):** Query the device via `GetRequest` and assert that all Interfaces and BGP neighbors are present *before* the switchover.
* Initiate Control Card switchover using gNOI SwitchControlProcessorRequest; store timestamp in "SwitchControlProcessorRequest_time"
* Wait for `SwitchControlProcessorResponse` but no longer than 120s. If not received, test FAILED.
* Immediately after receiving `SwitchControlProcessorResponse` for gNOI switchover, send gNMI `getRequest`. Store timestamp as "SwitchControlProcessorResponse_time".
* Wait for `getResponse` but no longer than 10s.
  * If not received, the test waits 10s and sends gNMI `getRequest`. Repeat from "Wait for `getResponse` but no longer than 10s" above.
  * If received at time <= "SwitchControlProcessorResponse"+110s and a non-zero grpc status code is returned, wait 10s and send gNMI `getRequest`. Repeat from "Wait for `getResponse` but no longer than 10s" above.
  * If received at time > "SwitchControlProcessorResponse"+110s and a non-zero grpc status code is returned, test FAILED
  * If received at time <= "SwitchControlProcessorResponse"+120s and SUCCESS is returned. 
* **Verify (After):** Once the gNMI agent is responsive, count the configured elements again. If the number of Interfaces and BGP neighbors matches the "Before" snapshot, the test passes.

## Testbed topology
dut.testbed

## Config Parameter coverage
N/A

## Telemetry Parameter coverage
N/A

## Canonical OC

```json

{
  "interfaces": {
    "interface": [
      {
        "aggregation": {
          "config": {
            "lag-type": "STATIC"
          }
        },
        "config": {
          "description": "LAG Interface 1",
          "enabled": true,
          "name": "lag1",
          "type": "ieee8023adLag"
        },
        "name": "lag1",
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
                        "ip": "192.0.2.10",
                        "prefix-length": 31
                      },
                      "ip": "192.0.2.10"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8::1:1",
                        "prefix-length": 127
                      },
                      "ip": "2001:db8::1:1"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              }
            }
          ]
        }
      },
      {
        "config": {
          "enabled": true,
          "name": "port1",
          "type": "ethernetCsmacd"
        },
        "ethernet": {
          "config": {
            "aggregate-id": "lag1"
          }
        },
        "name": "port1"
      }
    ]
  },
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "DEFAULT",
          "type": "DEFAULT_INSTANCE"
        },
        "name": "DEFAULT",
        "protocols": {
          "protocol": [
            {
              "bgp": {
                "global": {
                  "afi-safis": {
                    "afi-safi": [
                      {
                        "afi-safi-name": "IPV4_UNICAST",
                        "config": {
                          "afi-safi-name": "IPV4_UNICAST",
                          "enabled": true
                        }
                      },
                      {
                        "afi-safi-name": "IPV6_UNICAST",
                        "config": {
                          "afi-safi-name": "IPV6_UNICAST",
                          "enabled": true
                        }
                      }
                    ]
                  },
                  "config": {
                    "as": 65536,
                    "router-id": "192.0.2.1"
                  }
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "IPV4_UNICAST",
                            "config": {
                              "afi-safi-name": "IPV4_UNICAST",
                              "enabled": true
                            }
                          },
                          {
                            "afi-safi-name": "IPV6_UNICAST",
                            "config": {
                              "afi-safi-name": "IPV6_UNICAST",
                              "enabled": false
                            }
                          }
                        ]
                      },
                      "config": {
                        "enabled": true,
                        "neighbor-address": "192.0.2.5",
                        "peer-as": 64501,
                        "peer-group": "BGP-PEER-GROUP1"
                      },
                      "neighbor-address": "192.0.2.5"
                    }
                  ]
                },
                "peer-groups": {
                  "peer-group": [
                    {
                      "config": {
                        "peer-as": 64501,
                        "peer-group-name": "BGP-PEER-GROUP1"
                      },
                      "peer-group-name": "BGP-PEER-GROUP1"
                    }
                  ]
                }
              },
              "config": {
                "identifier": "BGP",
                "name": "BGP"
              },
              "identifier": "BGP",
              "name": "BGP"
            },
            {
              "config": {
                "enabled": true,
                "identifier": "ISIS",
                "name": "DEFAULT"
              },
              "identifier": "ISIS",
              "isis": {
                "global": {
                  "config": {
                    "instance": "DEFAULT"
                  }
                },
                "interfaces": {
                  "interface": [
                    {
                      "config": {
                        "circuit-type": "POINT_TO_POINT",
                        "interface-id": "lag1"
                      },
                      "interface-id": "lag1"
                    }
                  ]
                }
              },
              "name": "DEFAULT"
            }
          ]
        }
      }
    ]
  },
  "system": {
    "config": {
      "hostname": "ondatraHost"
    }
  }
}
```
  
## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
  /components/component/state/redundant-role:
    platform_type: [CONTROLLER_CARD]
  /components/component/state/switchover-ready:
    platform_type: [CONTROLLER_CARD]
  /components/component/state/type:
    platform_type: [CONTROLLER_CARD]
  /interfaces/interface/aggregation/config/lag-type:
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/type:
  /interfaces/interface/ethernet/config/aggregate-id:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /network-instances/network-instance/config/name:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/as:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-group:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-as:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-group-name:
  /network-instances/network-instance/protocols/protocol/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/config/circuit-type:
  /system/config/hostname:
rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```
