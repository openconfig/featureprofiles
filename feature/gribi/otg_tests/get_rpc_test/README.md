# TE-5.1: gRIBI Get RPC

## Summary

Validate the accuracy, completeness, filtering options, network-instance
scoping, and response latency of the `gRIBI.Get` RPC across single and dual
client sessions, dual-stack scale (`1,000` IPv4 and `1,000` IPv6 entries),
multiple `L3VRF` network instances, and negative/edge conditions as a pure
control-plane specification.

## Testbed type

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

### Test environment setup

*   Connect ATE `port-1` to DUT `port-1` and ATE `port-2` to DUT `port-2`:
    *   Link 1 (`port-1`): DUT `192.0.2.1/30` (`2001:db8:1::1/126`), ATE
        `192.0.2.2/30` (`2001:db8:1::2/126`).
    *   Link 2 (`port-2`): DUT `192.0.2.5/30` (`2001:db8:2::1/126`), ATE
        `192.0.2.6/30` (`2001:db8:2::2/126`).
*   Ensure ARP and IPv6 Neighbor Discovery (ND) resolve between DUT and ATE on
    both links so next-hops pointing to ATE `port-2` (`192.0.2.6` and
    `2001:db8:2::2`) can be programmed in hardware FIB.
*   Connect a primary gRIBI client to the DUT referred to as `gRIBI-A`
    (`election_id = (12, 0)`), along with a secondary non-leader client
    referred to as `gRIBI-B` (`election_id = (11, 0)`), both using `PRESERVE`
    persistence and `SINGLE_PRIMARY` redundancy mode with `RIB_AND_FIB_ACK`
    requested. Ensure `gRIBI-A` becomes the elected leader and flush all
    pre-existing gRIBI entries before starting test cases.

#### Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "dutPort1",
          "enabled": true,
          "name": "port1",
          "type": "iana-if-type:ethernetCsmacd"
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
                        "ip": "192.0.2.1",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.1"
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
                        "ip": "2001:db8:1::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8:1::1"
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
          "description": "dutPort2",
          "enabled": true,
          "name": "port2",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port2",
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
                        "ip": "192.0.2.5",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.5"
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
                        "ip": "2001:db8:2::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8:2::1"
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
      }
    ]
  },
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "DEFAULT",
          "type": "openconfig-network-instance-types:DEFAULT_INSTANCE"
        },
        "name": "DEFAULT",
        "protocols": {
          "protocol": [
            {
              "config": {
                "identifier": "openconfig-policy-types:STATIC",
                "name": "STATIC"
              },
              "identifier": "openconfig-policy-types:STATIC",
              "name": "STATIC",
              "static-routes": {
                "static": [
                  {
                    "config": {
                      "prefix": "198.51.100.192/26"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "config": {
                            "index": "0",
                            "next-hop": "192.0.2.6"
                          },
                          "index": "0"
                        }
                      ]
                    },
                    "prefix": "198.51.100.192/26"
                  }
                ]
              }
            }
          ]
        }
      },
      {
        "config": {
          "name": "VRF-A",
          "type": "openconfig-network-instance-types:L3VRF"
        },
        "name": "VRF-A"
      },
      {
        "config": {
          "name": "VRF-EMPTY",
          "type": "openconfig-network-instance-types:L3VRF"
        },
        "name": "VRF-EMPTY"
      }
    ]
  }
}
```

### TE-5.1.1 - Baseline Get RPC, Non-Leader Rejection, and Static Exclusion

*   Inject IPv4Entry cases for `198.51.100.0/26`, `198.51.100.64/26`,
    `198.51.100.128/26` pointing to `NextHopGroup` `42` (`DEFAULT`) and
    `NextHop` `1` (`192.0.2.6`, ATE port-2) via gRIBI-A. Validate entries are
    installed through `gNMI.Subscribe` (`ON_CHANGE`) AFT telemetry. Attempt to
    program the same prefixes via non-leader `gRIBI-B` and confirm the
    operations fail with `NOT_PRIMARY` (`ProgrammingFailed`).

*   Issue `Get` RPC from gRIBI-A (`network_instance: all`, `aft: IPV4`), ensure
    that all entries for `198.51.100.0/26`, `198.51.100.64/26`,
    `198.51.100.128/26` are returned. Measure latency of Get RPC.

    *   TODO: ensure all AFTEntry in the GetResponse for the IPv4Entry, NHG and
        NH are returned with [fib_status]=`PROGRAMMED` and
        [rib_status]=`PROGRAMMED`.

*   Issue `Get` RPC from gRIBI-B (`network_instance: all`, `aft: IPV4`), ensure
    that all entries for `198.51.100.0/26`, `198.51.100.64/26`,
    `198.51.100.128/26` are returned identically to the leader client. Measure
    latency of Get RPC.

    *   TODO: ensure all IPv4Entry, NHG and NH are returned with
        [fib_status]=`PROGRAMMED` and [rib_status]=`PROGRAMMED`.

*   Configure an OpenConfig static route for `198.51.100.192/26` (originally
    noted as `198.51.100.192/64`) pointing to `192.0.2.6` via `gNMI.Set`, issue
    `Get` from gRIBI-A, and ensure that only gRIBI-programmed entries for
    `198.51.100.0/26`, `198.51.100.64/26`, `198.51.100.128/26` are returned,
    with no entry returned for the static route `198.51.100.192/26`.

    *   TODO: ensure all IPEntry, NHG and NH are returned with
        [fib_status]=`PROGRAMMED`.

*   Inject an entry that cannot be installed into the FIB due to an unresolved
    next-hop (`203.0.113.0/24` -> unresolved `192.0.2.254`, originally noted as
    `192.0.2.254/32`). Issue a Get RPC from gRIBI-A and ensure that the entry
    for `203.0.113.0/24` is not returned as FIB-programmed.

    *   TODO: ensure that the IPEntry for `203.0.113.0/24` (when programmed
        with unresolved IPv4 next-hop `192.0.2.254`) is returned with
        [fib_status]=`NOT_PROGRAMMED` and [rib_status]=`PROGRAMMED`.

### TE-5.1.2 - Dual-Stack Scale Get Accuracy, Completeness, and Latency

*   Flush existing gRIBI entries on `DEFAULT` via `gRIBI-A`.
*   Via `gRIBI-A` (`Modify` with `RIB_AND_FIB_ACK`), program the following scale
    hierarchy in the `DEFAULT` network instance:
    *   `NextHop` `10` (`ip_address = "192.0.2.6"`, ATE `port-2`) and
        `NextHopGroup` `100` referencing `NextHop` `10` (`weight = 1`).
    *   `NextHop` `20` (`ip_address = "2001:db8:2::2"`, ATE `port-2`) and
        `NextHopGroup` `200` referencing `NextHop` `20` (`weight = 1`).
    *   `1,000` `IPv4Entry` `/32` prefixes (`100.64.0.1/32` through
        `100.64.3.232/32`, i.e., `100.64.<floor((i-1)/256)>.<((i-1) mod 256) +
        1>/32` for $i \in [1, 1000]$) referencing `NextHopGroup` `100`.
    *   `1,000` `IPv6Entry` `/128` prefixes (`2001:db8:20::1/128` through
        `2001:db8:20::3e8/128`) referencing `NextHopGroup` `200`.
*   Confirm `FIB_PROGRAMMED` acknowledgements for all `2,004` `AFTOperation`s
    and use `gNMI.Subscribe` (`ON_CHANGE`) on the `DEFAULT` AFT paths to verify
    all `1,000` IPv4 prefixes, `1,000` IPv6 prefixes, `2` `NextHopGroup`s, and
    `2` `NextHop`s are present in OpenConfig AFT state without static sleeps.
*   Issue `gRIBI.Get` (`network_instance: {name: "DEFAULT"}`, `aft: ALL`) from
    both `gRIBI-A` (leader) and `gRIBI-B` (non-leader) and record end-to-end
    stream completion time until `EOF`:
    *   Verify the streamed `GetResponse` contains **exact** parity (`2,004`
        total `AFTEntry` records: `1,000` `ipv4`, `1,000` `ipv6`, `2`
        `next_hop_group`, and `2` `next_hop` entries) with zero missing,
        duplicate, or corrupted keys/attributes.
    *   Verify every returned `AFTEntry` reports `network_instance = "DEFAULT"`,
        `rib_status = PROGRAMMED`, and `fib_status = PROGRAMMED`, matching the
        `gNMI` AFT state tree.
    *   Verify each `gRIBI.Get` call across the `2,004`-entry table completes
        end-to-end within **`<= 5 seconds`**.

### TE-5.1.3 - GetRequest AFT Type Filtering (`AFTType` Options)

*   With the `2,004` entries active in `DEFAULT` from `TE-5.1.2`, issue
    `gRIBI.Get` requests (`network_instance: {name: "DEFAULT"}`) for each
    specific `AFTType` filter defined in `gribi.proto`:
    *   `aft = IPV4`: Verify exactly the `1,000` `IPv4Entry` records are
        returned and zero `ipv6`, `next_hop_group`, or `next_hop` entries
        appear.
    *   `aft = IPV6`: Verify exactly the `1,000` `IPv6Entry` records are
        returned and zero other AFT types appear.
    *   `aft = NEXTHOP_GROUP`: Verify exactly the `2` `NextHopGroup` records
        (`100` and `200`) are returned with accurate `next_hop` member IDs and
        weights.
    *   `aft = NEXTHOP`: Verify exactly the `2` `NextHop` records (`10` and
        `20`) are returned with exact `ip_address` attributes (`192.0.2.6` and
        `2001:db8:2::2`).
    *   `aft = MPLS`: On platforms supporting gRIBI `LabelEntry` AFTs, verify
        `0` entries are returned when none are programmed (or exact matching
        `LabelEntry` records if programmed); on platforms that do not support
        the `MPLS` AFT via gRIBI, verify the server responds cleanly with gRPC
        status `UNIMPLEMENTED` per `gribi.proto` (or `0` entries) without
        closing client sessions.

### TE-5.1.4 - Multi-VRF Network-Instance Scoping (`name` vs `all`)

*   Using `gNMI.Set` (`UPDATE`), create two additional `L3VRF` network
    instances on the DUT:
    *   `VRF-A` (`type = L3VRF`)
    *   `VRF-EMPTY` (`type = L3VRF`, containing zero gRIBI entries)
*   Program `1,000` `IPv6Entry` `/128` prefixes (`2001:db8:30::1/128` through
    `2001:db8:30::3e8/128`) inside `VRF-A` via `gRIBI-A` (`Modify`) with
    `next_hop_group = 200` and `next_hop_group_network_instance = "DEFAULT"`
    (reusing `NextHopGroup` `200` -> `NextHop` `20` in `DEFAULT` from `TE-5.1.2`
    so FIB resolution succeeds across `port-2` without changing interface
    bindings). Confirm `FIB_PROGRAMMED` acknowledgements and verify `VRF-A` AFT
    state via `gNMI.Subscribe` (`ON_CHANGE`).
*   Validate network-instance scoping via `gRIBI.Get`:
    *   `GetRequest(network_instance: {name: "VRF-A"}, aft: ALL)`: Verify
        **only** the `1,000` `VRF-A` `IPv6Entry` prefixes (`2001:db8:30::1/128`
        through `2001:db8:30::3e8/128`) are returned (`1,000` total entries)
        with `network_instance = "VRF-A"`, `rib_status = PROGRAMMED`, and
        `fib_status = PROGRAMMED`, and **zero** entries from `DEFAULT` leak into
        the response.
    *   `GetRequest(network_instance: {name: "DEFAULT"}, aft: ALL)`: Verify
        **only** the `2,004` `DEFAULT` entries from `TE-5.1.2` are returned and
        **zero** `VRF-A` entries appear.
    *   `GetRequest(network_instance: {all: {}}, aft: ALL)`: Verify the exact
        union of `DEFAULT` and `VRF-A` gRIBI entries (`3,004` total entries:
        `2,004` in `DEFAULT` + `1,000` in `VRF-A`) is returned with each
        `AFTEntry.network_instance` accurately populated, completing within
        `<= 5 seconds`.

### TE-5.1.5 - Negative and Edge-Case Get Validation

*   **Empty `L3VRF` Query**: Issue `gRIBI.Get` with
    `network_instance: {name: "VRF-EMPTY"}` and `aft: ALL`. Verify the RPC
    succeeds with gRPC status `OK` and returns `0` `AFTEntry` elements.
*   **Missing or Empty `network_instance` Specifier**: Issue `gRIBI.Get` with
    `network_instance` unset (`nil`) and with `network_instance: {name: ""}`.
    Verify the DUT rejects both requests with canonical gRPC error code
    `INVALID_ARGUMENT` as specified in `gribi.proto`.
*   **Non-Existent Network Instance**: Issue `gRIBI.Get` with
    `network_instance: {name: "VRF-NONEXISTENT"}` and `aft: ALL`. Verify the
    DUT responds cleanly with gRPC error `INVALID_ARGUMENT` or `NOT_FOUND` (or
    an empty `GetResponse` where unconfigured VRFs are treated as empty) and
    that the gRIBI daemon does not crash or drop active `Modify` sessions on
    `gRIBI-A` or `gRIBI-B`.

### Cleanup

*   Execute `gRIBI.Flush` across all network instances (`VRF-A` and `DEFAULT`)
    using `override` or the active leader election ID.
*   Remove `VRF-A`, `VRF-EMPTY`, and the static route `198.51.100.192/26` via
    `gNMI.Set` (`DELETE`) registered in `t.Cleanup()`, restoring the DUT to its
    baseline pre-test configuration.

[fib_status]: https://github.com/openconfig/gribi/blob/08d53dffce45e942c6e7f07521c58b557984e4b7/v1/proto/service/gribi.proto#L485
[rib_status]: https://github.com/openconfig/gribi/blob/08d53dffce45e942c6e7f07521c58b557984e4b7/v1/proto/service/gribi.proto#L483

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/name:
  /interfaces/interface/config/type:
  /interfaces/interface/subinterfaces/subinterface/config/index:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group-network-instance:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/weight:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address:
  /network-instances/network-instance/config/name:
  /network-instances/network-instance/config/type:
  /network-instances/network-instance/interfaces/interface/config/id:
  /network-instances/network-instance/interfaces/interface/config/subinterface:
  /network-instances/network-instance/protocols/protocol/config/identifier:
  /network-instances/network-instance/protocols/protocol/config/name:
  /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/index:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop:
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
      on_change: true
  gribi:
    gRIBI.Flush:
    gRIBI.Get:
    gRIBI.Modify:
```

## Minimum DUT platform requirement

vRX if the vendor implementation supports FIB-ACK simulation, otherwise FFF.
