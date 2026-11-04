# RT-1.3: BGP Route Propagation

## Summary

BGP Route Propagation

## Testbed type

*  [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

Establish eBGP sessions between:

*   ATE port-1 and DUT port-1
*   ATE port-2 and DUT port-2

IPv4 and IPv6 use separate sessions on each port: the IPv4 neighbors carry
IPV4_UNICAST and the IPv6 neighbors carry IPV6_UNICAST. The DUT port-1
neighbors are members of peer-group `BGP-PEER-GROUP1` and the DUT port-2
neighbors are members of peer-group `BGP-PEER-GROUP2`; both peer-groups apply an
accept-all import and export policy. Both ATE peers announce the ADD-Path
capability for IPv4-unicast and IPv6-unicast. ATE port-1 advertises 4 IPv4
prefixes (`198.51.100.0/32` to `198.51.100.3/32`) and 4 IPv6 prefixes
(`2001:db8:1::1/128` to `2001:db8:1::4/128`).


### RT-1.3.1: MRAI: [TODO: https://github.com/openconfig/featureprofiles/issues/3035]
*   DUT: Configure the Minimum Route Advertisement Interval (MRAI) for desired behavior.
*   ATE Port 2: Verify received routes adhere to the MRAI timing.

### RT-1.3.2: RFC5549
*   DUT: Enable RFC5549 support:
    *   Update the BGP peer group configuration to enable extended next hop encoding using  `/network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/ipv4-unicast/config/extended-next-hop-encoding`  
*   ATE Port 1: Advertise IPv4 routes with IPv6 next-hops.
*   ATE Port 2: Validate correct acceptance and propagation of routes with IPv6 next-hops.

### RT-1.3.3: Add-Path receive

*   For the Add-Path subtests the ATE port-1 session is iBGP: ATE port-1 uses
    the DUT AS (65537) and acts as a route reflector that sends multiple paths
    to its client, the DUT.
*   ATE Port 1: Advertise each prefix four times with path IDs 1, 2, 3 and 4.
*   RT-1.3.3.1: Add-Path receive on the neighbor
    *   DUT: On the ATE port-1 neighbors set
        `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/config/receive`
        to `true` for IPv4 and IPv6.
    *   Verification (Telemetry):
        `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/state/receive`
        is `true` on the ATE port-1 neighbors.
    *   Verification (Telemetry):
        `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received`
        on the ATE port-1 neighbors is `16` (4 prefixes x 4 paths): the DUT
        keeps every path.
*   RT-1.3.3.2: Add-Path receive on the peer-group
    *   DUT: Remove the `add-paths` configuration from the ATE port-1
        neighbors and set
        `/network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/config/receive`
        to `true` on `BGP-PEER-GROUP1` for IPv4 and IPv6.
    *   Verification (Telemetry):
        `/network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/state/receive`
        is `true` on `BGP-PEER-GROUP1`.
    *   Verification (Telemetry): `prefixes/received` on the ATE port-1
        neighbors is `16` (receive inherited from the peer-group).
*   ADD-Path is negotiated when a session is established, so the BGP sessions
    are re-established after each configuration change.

#### Canonical OC

The DUT configuration for RT-1.3.3.1:

```json
{
  "network-instances": {
    "network-instance": [
      {
        "name": "DEFAULT",
        "config": { "name": "DEFAULT" },
        "protocols": {
          "protocol": [
            {
              "identifier": "BGP",
              "name": "BGP",
              "config": { "identifier": "BGP", "name": "BGP" },
              "bgp": {
                "neighbors": {
                  "neighbor": [
                    {
                      "neighbor-address": "192.0.2.2",
                      "config": { "neighbor-address": "192.0.2.2", "peer-as": 65537 },
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "IPV4_UNICAST",
                            "config": { "afi-safi-name": "IPV4_UNICAST", "enabled": true },
                            "add-paths": { "config": { "receive": true } }
                          }
                        ]
                      }
                    },
                    {
                      "neighbor-address": "2001:db8::2",
                      "config": { "neighbor-address": "2001:db8::2", "peer-as": 65537 },
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "IPV6_UNICAST",
                            "config": { "afi-safi-name": "IPV6_UNICAST", "enabled": true },
                            "add-paths": { "config": { "receive": true } }
                          }
                        ]
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

### RT-1.3.4: Disabling Add-Path Send: [TODO: https://github.com/openconfig/featureprofiles/issues/3037]
*   DUT: Disable Add-Path send for the neighbor connected to ATE Port 2 for both IPv4 and IPv6.
*   Verification (Telemetry): Confirm that the DUT's telemetry reflects the disabled Add-Path send status.
*   ATE Port 1: Readvertise multiple paths.
*   ATE Port 2: Verify that only a single best path is received by ATE Port 2 due to disabled Add-Path send on the DUT.

### RT-1.3.5: Disabling Add-Path Receive
*   DUT: Set `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/config/receive`
    to `false` on the ATE port-1 neighbors for IPv4 and IPv6.
*   Verification (Telemetry): `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/state/receive`
    is `false` on the ATE port-1 neighbors.
*   ATE Port 1: Advertise each prefix with path IDs 1 to 4 again.
*   Verification (Telemetry): `prefixes/received` on the ATE port-1 neighbors
    is `4`: without receive, each new path replaces the previous one, so the
    DUT keeps one path per prefix.
*   ATE Port 2: Verify that the DUT has received and propagated only one single path.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
  ## Config paths
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/config/receive:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/config/send:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/config/send-max:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/ipv4-unicast/config/extended-next-hop-encoding:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/config/receive:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/config/send:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/config/send-max:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/config/minimum-advertisement-interval:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/ipv4-unicast/config/extended-next-hop-encoding:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/config/receive:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/config/send:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/config/send-max:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/minimum-advertisement-interval:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/config/extended-next-hop-encoding:

  ## State paths
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/state/receive:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/state/send:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/state/send-max:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/ipv4-unicast/state/extended-next-hop-encoding:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/state/receive:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/state/send:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/state/send-max:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/state/minimum-advertisement-interval:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/ipv4-unicast/state/extended-next-hop-encoding:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/state/receive:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/state/send:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/state/send-max:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/state/minimum-advertisement-interval:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/state/extended-next-hop-encoding:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/supported-capabilities:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

* MFF - A modular form factor device containing LINECARDs, FABRIC and redundant CONTROLLER_CARD components
* FFF - fixed form factor
