# RT-1.26: Basic Static Route Support

## Summary

This test validates the foundational routing capabilities for static routes on a
DUT. It consolidates verification for:

- IPv4 and IPv6 static routes across DEFAULT, NON-DEFAULT, and MANAGEMENT network-instances.
- Cross-address family (XAF) next-hops (IPv4 route with IPv6 next-hop and vice-versa).
- Route properties: ECMP, metric, administrative distance (preference), and tags.
- Next-hop actions: DROP next-hops, invalid/blackhole next-hops, and disabling recursive next-hop resolution.
- Dynamic modifications: Adding and removing next-hops dynamically.

## Testbed type

* [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

* Connect DUT port-1, port-2, port-3 and port-4 to ATE port-1, port-2, port-3 and port-4 respectively.
* Configure IPv4 and IPv6 addresses on DUT and ATE interfaces.
* Configure one IPv4 destination network `ipv4-network = 203.0.113.0/24` connected to ATE port 1 and 2.
* Configure one IPv6 destination network `ipv6-network = 2001:db8:128:128::/64` connected to ATE port 1 and 2.

### RT-1.26.1 - Validate Static Route ECMP

* Step 1 - Configure IPv4 and IPv6 static routes for ECMP by setting the values at `/network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix` and `/network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop`:
  * Configure an IPv4 static route `ipv4-route-a` on the DUT for destination
    `203.0.113.0/24` with the next-hop set to the IPv4 address of ATE port-1.
  * Configure an IPv4 static route `ipv4-route-b` on the DUT for destination
    `203.0.113.0/24` with the next-hop set to the IPv4 address of ATE port-2.
  * Configure an IPv6 static route `ipv6-route-a` on the DUT for destination
    `2001:db8:128:128::/64` with the next-hop set to the IPv6 address of ATE port-1.
  * Configure an IPv6 static route `ipv6-route-b` on the DUT for destination
    `2001:db8:128:128::/64` with the next-hop set to the IPv6 address of ATE port-2.
* Step 2 - Push configuration to DUT using gnmi.Set with REPLACE option.
* Step 3 - Validate both routes are configured and reported correctly by checking that
  the value of `/network-instances/network-instance/protocols/protocol/static-routes/static/state/prefix`
  matches the configured prefixes.
* Step 4 - Send IPv4 and IPv6 Traffic from ATE port-3 towards destination
  `203.0.113.0/24` and `2001:db8:128:128::/64`.
* Step 5 - Verify that traffic is received from DUT on both port-1 and port-2,
  confirming ECMP works.

### RT-1.26.2 - Validate Static Route Metric

* Step 1 - Set the value of `/network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/metric` to `100` for both `ipv4-route-b` and `ipv6-route-b`.
* Step 2 - Push configuration to DUT using gnmi.Set with REPLACE option.
* Step 3 - Validate that the metric is set correctly by checking that the value of `/network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/state/metric` is `100`.
* Step 4 - Send Traffic from ATE port-3 towards destinations.
* Step 5 - Verify that traffic is received from DUT on port-1 and NOT on port-2
  (as route-b has a higher metric).

### RT-1.26.3 - Validate Static Route Preference

* Step 1 - Set the value of `/network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/preference` to `50` for both `ipv4-route-a` and `ipv6-route-a`.
* Step 2 - Push configuration to DUT.
* Step 3 - Validate that the preference is set correctly by checking that the value of `/network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/state/preference` is `50`.
* Step 4 - Send Traffic from ATE port-3 towards destinations.
* Step 5 - Verify that traffic is now received from DUT on port-2 and NOT on
  port-1 (as route-a has a higher preference value/lower priority).

### RT-1.26.4 - Validate Static Route Tag

* Step 1 - Configure a tag of value `10` on the IPv4 and IPv6 static routes via
  `/network-instances/network-instance/protocols/protocol/static-routes/static/config/set-tag`.
* Step 2 - Push configuration to DUT.
* Step 3 - Validate the tag is set correctly by checking that the value of `/network-instances/network-instance/protocols/protocol/static-routes/static/state/set-tag` is `10`.

### RT-1.26.5 - Validate Cross-Address Family (XAF) Next-Hops

* Step 1 - Delete the configuration using a gNMI Set DELETE on the specific `metric` and `preference` paths for `ipv4-route-b`, `ipv6-route-b`, `ipv4-route-a`, and `ipv6-route-a`.
* Step 2 - Configure IPv6 static route `2001:db8:128:128::/64` with next-hops
  set to the IPv4 address of ATE port-1 and ATE port-2.
* Step 3 - Configure IPv4 static route `203.0.113.0/24` with next-hops set to
  the IPv6 address of ATE port-1 and ATE port-2.
* Step 4 - Push configuration to DUT.
* Step 5 - Validate the routes are configured and the cross-family next-hops are
  reported correctly by checking the next-hop IP addresses via `/network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/state/next-hop`.
* Step 6 - Send Traffic from ATE port-3 towards both destinations.
* Step 7 - Verify that traffic is received from DUT on both port-1 and port-2
  and ECMP works for both XAF routes.

### RT-1.26.6 - Validate Static Route with DROP Next-Hop

* Step 1 - Configure an IPv4 static route for `203.0.113.0/24` by setting `/network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop` to the OpenConfig `local-defined-next-hop` value `DROP`.
* Step 2 - Configure an IPv6 static route for `2001:db8:128:128::/64` by setting `/network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop` to the OpenConfig `local-defined-next-hop` value `DROP`.
* Step 3 - Push configuration to DUT.
* Step 4 - Validate the route is configured and reported correctly.
* Step 5 - Send Traffic from ATE port-3 towards destinations.
* Step 6 - Verify that traffic is dropped on DUT and not received on port-1 and
  port-2.

### RT-1.26.7 - Validate Disabling Recursive Next-Hop Resolution

* Step 1 - Configure IPv4 and IPv6 IS-IS between ATE port-1 <-> DUT port-1 and
  ATE port-2 <-> DUT port-2.
* Step 2 - Configure one IPv4 `/32` host route (`198.51.100.100/32`) and one
  IPv6 `/128` host route (`2001:db8::64:64::1/128`) connected to ATE and
  advertised to DUT through both IS-IS adjacencies.
* Step 3 - Configure an IPv4 static route for `203.0.113.0/24` with next-hop
  `198.51.100.100`. Configure an IPv6 static route for `2001:db8:128:128::/64`
  with next-hop `2001:db8::64:64::1`.
* Step 4 - Push configuration to DUT.
* Step 5 - Send Traffic and Verify that traffic is received from DUT.
* Step 6 - Disable static route next-hop recursive lookup by setting `/network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/recurse` to `false`.
* Step 7 - Push configuration to DUT.
* Step 8 - Validate by checking that the value of `/network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/state/recurse` is `false`.
* Step 9 - Send Traffic and Verify that traffic is NOT received from DUT, as the
  recursive next-hop resolution is disabled.

### RT-1.26.8 - Validate Dynamic Add and Remove of Next-Hops

* Step 1 - Configure one IPv4 static route with next-hops set to the IPv4
  address of ATE port-2 (index 0) and port-3 (index 1).
* Step 2 - Push configuration to DUT.
* Step 3 - Update the static route by adding next-hops for ATE port-1 (index 2)
  and port-4 (index 3).
* Step 4 - Push configuration to DUT.
* Step 5 - Validate all four next-hops and indexes are reported correctly using the `/network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/state/index` and `/network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/state/next-hop` paths.
* Step 6 - Remove two next-hops (e.g., indexes 0 and 3).
* Step 7 - Push configuration to DUT and validate that only two next-hops
  remain.

### RT-1.26.9 - Direct Interface IP Deletion (Negative)

* Step 1 - Configure a static route that resolves via a directly connected
  interface IP.
* Step 2 - Delete the IP address of that direct interface using a gNMI Set
  DELETE.
* Step 3 - Send Traffic.
* Step 4 - Verify the traffic drops, and the static route becomes inactive
  without crashing the device.

### RT-1.26.10 - Overlapping Prefixes / LPM (Corner)

* Step 1 - Configure overlapping static routes: `10.0.0.0/8` pointing to ATE
  port-1, and `10.1.1.0/24` pointing to ATE port-2.
* Step 2 - Push configuration to DUT.
* Step 3 - Send traffic to destination `10.1.1.1`.
* Step 4 - Verify that traffic strictly adheres to Longest Prefix Match (LPM)
  routing and flows exclusively to ATE port-2.

### RT-1.26.11 - Route Resolution Loop (Negative)

* Step 1 - Configure Static Route A pointing to Next-Hop IP B.
* Step 2 - Configure Static Route B pointing to Next-Hop IP A.
* Step 3 - Push configuration to DUT.
* Step 4 - Verify the device's control plane detects or breaks the recursion
  loop safely without hanging or crashing.

## Canonical OC

```json
{
  "openconfig-network-instance:network-instances": {
    "network-instance": [
      {
        "name": "default",
        "config": {
          "name": "default"
        },
        "protocols": {
          "protocol": [
            {
              "identifier": "openconfig-policy-types:STATIC",
              "name": "DEFAULT",
              "config": {
                "identifier": "openconfig-policy-types:STATIC",
                "name": "DEFAULT"
              },
              "static-routes": {
                "static": [
                  {
                    "prefix": "203.0.113.0/24",
                    "config": {
                      "prefix": "203.0.113.0/24",
                      "set-tag": "10"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "index": "0",
                          "config": {
                            "index": "0",
                            "metric": 100,
                            "preference": 50,
                            "next-hop": "192.0.2.1",
                            "recurse": true
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
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  ## Config Paths ##
  /interfaces/interface/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled:
  /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix:
  /network-instances/network-instance/protocols/protocol/static-routes/static/config/set-tag:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/metric:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/preference:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/recurse:

  ## State Paths ##
  /network-instances/network-instance/protocols/protocol/static-routes/static/state/prefix:
  /network-instances/network-instance/protocols/protocol/static-routes/static/state/set-tag:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/state/next-hop:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/state/metric:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/state/preference:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/state/recurse:

rpcs:
  gnmi:
    gNMI.Subscribe:
      on_change: true
    gNMI.Set:
      replace: true
```

## Required DUT platform

* FFF
