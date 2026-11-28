# RT-1.37: BGP configuration leaves

## Summary

Rewrite the global BGP AS and router-id, create one neighbor, enable the IPv6
unicast AFI, then delete the neighbor. SpyTest writes the current AS and
router-id, creates a neighbor, and enables IPv6 unicast. Session establishment
is covered by the BGP OTG tests. The global AS is written back to its current
value so this test does not renumber the router.

## Testbed type

*   [`featureprofiles/topologies/dut.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut.testbed)

## Procedure

*   Read the global AS. Replace it with that same value. Replace router-id
    with `192.0.2.1` when it was unset, otherwise rewrite the current value.
    Read AS and router-id from config, and AS from state.
*   Replace neighbor `192.0.2.99` with peer-as equal to the global AS.
    Replace the global `IPV6_UNICAST` enabled leaf with true.
    Read peer-as and enabled from config, and the neighbor address from state.
*   Delete the neighbor and check it is gone. Restore the AFI leaf.

#### Canonical OC

```json
{
  "network-instances": {
    "network-instance": [
      {
        "name": "DEFAULT",
        "protocols": {
          "protocol": [
            {
              "identifier": "BGP",
              "name": "BGP",
              "bgp": {
                "global": {
                  "afi-safis": {
                    "afi-safi": [
                      {"afi-safi-name": "IPV6_UNICAST", "config": {"afi-safi-name": "IPV6_UNICAST", "enabled": true}}
                    ]
                  }
                },
                "neighbors": {
                  "neighbor": [
                    {"neighbor-address": "192.0.2.99", "config": {"neighbor-address": "192.0.2.99", "peer-as": 65001}}
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

```yaml
paths:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/as:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/neighbor-address:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/neighbor-address:
rpcs:
  gnmi:
    gNMI.Set:
      replace: true
      delete: true
    gNMI.Subscribe:
```
