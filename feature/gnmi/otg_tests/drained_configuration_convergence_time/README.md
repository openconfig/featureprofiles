# gNMI-1.3: Benchmarking: Drained Configuration Convergence Time

## Summary

Measure performance of drained configuration being applied.

## Canonical OC

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
                                  "SET-MED"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "IPV4_UNICAST",
                              "enabled": true
                            }
                          }
                        ]
                      },
                      "config": {
                        "peer-as": 64501,
                        "peer-group-name": "BGP-PEER-GROUP"
                      },
                      "peer-group-name": "BGP-PEER-GROUP"
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
                "identifier": "ISIS",
                "name": "DEFAULT"
              },
              "identifier": "ISIS",
              "isis": {
                "interfaces": {
                  "interface": [
                    {
                      "authentication": {
                        "config": {
                          "auth-mode": "MD5",
                          "auth-password": "ISISAuthPassword",
                          "auth-type": "SIMPLE_KEY",
                          "enabled": true
                        }
                      },
                      "config": {
                        "interface-id": "port1"
                      },
                      "interface-id": "port1"
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
  "routing-policy": {
    "policy-definitions": {
      "policy-definition": [
        {
          "config": {
            "name": "SET-MED"
          },
          "name": "SET-MED",
          "statements": {
            "statement": [
              {
                "actions": {
                  "bgp-actions": {
                    "config": {
                      "set-med": 25,
                      "set-med-action": "SET"
                    }
                  },
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                },
                "config": {
                  "name": "30"
                },
                "name": "30"
              }
            ]
          }
        }
      ]
    }
  }
}
```

## Procedure

Configure DUT with maximum number of IS-IS adjacencies, and BGP
peers - with physical interfaces between ATE and DUT for IS-IS
peers.

First port is used as ingress port to send routes from ATE to DUT.

Configure IS-IS interface authentication at
`/network-instances/network-instance/protocols/protocol/isis/interfaces/interface/authentication/config`
using the `enabled`, `auth-password`, `auth-mode`, and `auth-type` leaves. This
replaces level-specific configuration under
`/network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/hello-authentication/config`.

For each of the following configurations, generate complete device
configuration and measure time for the operation to complete (as
defined in the case):
    *   TODO: IS-IS overload:
        *   At t=0, send Set to DUT marking IS-IS overload bit.
        *   Measure time between t=0 and all IS-IS sessions on ATE to
            report DUT as overloaded.
    *   IS-IS metric change:
        *   At t=0, send Set to DUT marking IS-IS metric as changed for
            all IS-IS interfaces.
        *   Measure time between t=0 and all IS-IS sessions on ATE to
            report changed metric.
    *   BGP AS_PATH prepend:
        *   At t=0, send Set to DUT changing BGP policy for each session
            to prepend AS_PATH.
        *   Replace the complete peer-group AFI-SAFI `apply-policy/config`
            parent when changing the export policy.
        *   Measure time between t=0 and all BGP received routes on ATE
            to report change in as path.
    *   TODO: BGP MED manipulation.   
        *   At t=0, send Set to DUT changing BGP policy for each session to
            set MED to non-default value.
        *   Replace the complete peer-group AFI-SAFI `apply-policy/config`
            parent when changing the export policy.
        *   Measure time between t=0 and all BGP received routes on ATE to
            report changed metric.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml
paths:
  ## Config Parameter coverage
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-med:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-med-action:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/repeat-n:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/asn:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/export-policy:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/afi-safi/af/state/metric:
  /network-instances/network-instance/protocols/protocol/isis/global/lsp-bit/overload-bit/state/set-bit:

  ## Telemetry Parameter coverage
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/sent:  
rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
```
