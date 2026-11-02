# RT-1.55: BGP session mode (active/passive)

## Summary

* Validate the correct behavior of BGP session establishment in both active and passive modes.
* Verify the accurate reflection of BGP transport mode in telemetry output.
* Confirm the functionality of passive mode configuration at both the neighbor and peer group levels.

## Topology

DUT Port1 (AS 65501) ---eBGP --- ATE Port1 (AS 65502)

## Procedure

*  Configure both DUT and ATE to operate in BGP passive mode under the neighbor section.
*  Verify that the BGP adjacency will not be established.
*  Verify the telemetry path output to confirm that the neighbor's BGP transport mode is displayed as "passive for the DUT.
*  Configure BGP session on ATE to operate in BGP active mode when interacting with DUT.
*  Verify that a BGP adjacency is established between the ATE and DUT
*  Verify the telemetry path output to confirm that the neighbor's BGP transport mode is displayed as "passive for the DUT.
*  Redo the same above steps but configure the passive mode under the peer group instead of the  bgp neighbor configuration.

## Canonical OC

```json
{
  "network-instances": {
    "network-instance": [
      {
        "name": "DEFAULT",
        "config": {
          "name": "DEFAULT"
        },
        "protocols": {
          "protocol": [
            {
              "identifier": "BGP",
              "name": "BGP",
              "config": {
                "identifier": "BGP",
                "name": "BGP"
              },
              "bgp": {
                "global": {
                  "config": {
                    "as": 65540,
                    "router-id": "192.0.2.1"
                  }
                },
                "peer-groups": {
                  "peer-group": [
                    {
                      "peer-group-name": "eBGP-PEER-GROUP",
                      "config": {
                        "peer-group-name": "eBGP-PEER-GROUP",
                        "peer-as": 65550
                      },
                      "transport": {
                        "config": {
                          "local-address": "192.0.2.1",
                          "passive-mode": true
                        }
                      }
                    }
                  ]
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "neighbor-address": "192.0.2.2",
                      "config": {
                        "neighbor-address": "192.0.2.2",
                        "peer-as": 65550,
                        "peer-group": "eBGP-PEER-GROUP"
                      },
                      "transport": {
                        "config": {
                          "local-address": "192.0.2.1",
                          "passive-mode": true
                        }
                      }
                    }
                  ]
                }
              }
            }
          ]
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

This example yaml defines the OC paths intended to be covered by this test.  OC paths used for test environment setup are not required to be listed here.

```yaml
paths:
  ## Config paths
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/config/local-address:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/config/passive-mode:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/transport/config/local-address:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/transport/config/passive-mode:

  ## State paths
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/state/local-address:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/state/passive-mode:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/transport/state/local-address:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/transport/state/passive-mode:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

* MFF - A modular form factor device containing LINECARDs, FABRIC and redundant CONTROLLER_CARD components
* FFF - fixed form factor
