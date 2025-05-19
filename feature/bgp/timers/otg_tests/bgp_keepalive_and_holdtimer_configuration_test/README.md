# RT-1.10: BGP Keepalive and HoldTimer Configuration Test

## Summary

BGP Keepalive and HoldTimer Configuration Test

## Procedure

1   Establish eBGP sessions as follows between ATE and DUT
* The DUT has eBGP peering with ATE port 1 and ATE port 2.
* Enable an Accept-route all import-policy/export-policy under the BGP peer-group AFI/SAFI.
* The first pair is called the "source" pair, and the second the "destination" pair

2  Validate BGP session state on DUT using telemetry.
3  Modify BGP timer values on iBGP peers to 10/30 and on the eBGP peering to 5/15.
4  Verify that the sessions are established after soft reset.
5  Repeat Step 3 by modifying BGP timers at peer-group level to 10/30(keepalive-interval/hold-time) and on eBGP peering to 5/15(keepalive-interval/hold-time) and verify the timers

## Canonical OpenConfig for keepalive-interval and hold-time at neighbour and peer-group level for BGP

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
                "neighbors": {
                  "neighbor": [
                    {
                      "config": {
                        "neighbor-address": "192.0.2.6"
                      },
                      "neighbor-address": "192.0.2.6",
                      "timers": {
                        "config": {
                          "hold-time": 30,
                          "keepalive-interval": 10
                        }
                      }
                    }
                  ]
                },
                "peer-groups": {
                  "peer-group": [
                    {
                      "config": {
                        "peer-group-name": "peer_group"
                      },
                      "peer-group-name": "peer_group",
                      "timers": {
                        "config": {
                          "hold-time": 30,
                          "keepalive-interval": 10
                        }
                      }
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
            }
          ]
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.
OC paths used for test setup are not listed here.

```yaml
paths:
  ## Config Paths ##
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/keepalive-interval:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/hold-time:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/config/keepalive-interval:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/config/hold-time:

  ## State Paths ##
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/state/keepalive-interval:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/state/hold-time:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/state/keepalive-interval:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/state/hold-time:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Subscribe:
      on_change: true
    gNMI.Set:
```

## Minimum DUT platform requirement

vRX

