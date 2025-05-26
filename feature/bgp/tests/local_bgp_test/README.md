# RT-1.7: Local BGP Test

## Summary

The local\_bgp\_test brings up two OpenConfig controlled devices and tests that for an eBGP session 

* Established between them.
* Disconnected between them.
* Verify BGP neighbor parameters

Enable an Accept-route all import-policy/export-policy for eBGP session under the BGP peer-group AFI/SAFI.

This test is suitable for running in a KNE environment.

## Canonical OpenConfig
```json
{
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "DEFAULT"
        },
        "name": "DEFAULT",
        "protocols": {
          "protocol": [
            {
              "bgp": {
                "global": {
                  "config": {
                    "as": 64500,
                    "router-id": "192.0.2.21"
                  }
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "config": {
                        "auth-password": "password",
                        "local-as": 64500,
                        "neighbor-address": "192.168.10.2",
                        "peer-as": 64501
                      },
                      "neighbor-address": "192.168.10.2",
                      "timers": {
                        "config": {
                          "hold-time": 10,
                          "keepalive-interval": 30
                        }
                      }
                    }
                  ]
                },
                "peer-groups": {
                  "peer-group": [
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "IPV4_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "ALLOW"
                                ],
                                "import-policy": [
                                  "ALLOW"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "IPV4_UNICAST"
                            }
                          }
                        ]
                      },
                      "apply-policy": {
                        "config": {
                          "export-policy": [
                            "ALLOW"
                          ],
                          "import-policy": [
                            "ALLOW"
                          ]
                        }
                      },
                      "config": {
                        "description": "Description for BGP-PEER-GROUP",
                        "peer-group-name": "BGP-PEER-GROUP"
                      },
                      "peer-group-name": "BGP-PEER-GROUP"
                    }
                  ]
                }
              },
              "config": {
                "identifier": "BGP",
                "name": "bgp"
              },
              "identifier": "BGP",
              "name": "bgp"
            }
          ]
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
    ## Parameter Coverage

   /network-instances/network-instance/protocols/protocol/bgp/global/config/as:
   /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id:
   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/auth-password:
   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/local-as:
   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/neighbor-address:
   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as:
   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/neighbor-address:
   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/messages/received/last-notification-error-code:
   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/supported-capabilities:
   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/hold-time:
   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/keepalive-interval:
   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/description:
   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/import-policy:
   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/export-policy:
   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/state/import-policy:
   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/state/export-policy:
   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/import-policy:
   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/export-policy:
   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/state/import-policy:
   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/state/export-policy:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
```


