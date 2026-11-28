# RT-1.74: Static route configuration

## Summary

Create one static route with an IP next hop and one with an interface next hop,
then delete them. SpyTest creates only the IP next hop. Forwarding is
[RT-1.26](../../otg_tests/basic_static_route_support_test/README.md).

## Testbed type

*   [`featureprofiles/topologies/dut.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut.testbed)

## Procedure

*   In the default network instance, replace `203.0.113.0/24` with next hop
    `192.0.2.254`. Read the next hop from config.
*   Replace `203.0.113.10/32` with an interface-ref to the first Ethernet
    port and no next-hop address. Read the interface from config.
*   Read the first prefix from state.
*   Delete the IP-next-hop route and check it is gone from config.

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
              "identifier": "STATIC",
              "name": "DEFAULT",
              "static-routes": {
                "static": [
                  {
                    "prefix": "203.0.113.0/24",
                    "next-hops": {
                      "next-hop": [
                        {"index": "0", "config": {"index": "0", "next-hop": "192.0.2.254"}}
                      ]
                    }
                  }
                ]
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
  /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/interface-ref/config/interface:
  /network-instances/network-instance/protocols/protocol/static-routes/static/state/prefix:
rpcs:
  gnmi:
    gNMI.Set:
      replace: true
      delete: true
    gNMI.Subscribe:
```
