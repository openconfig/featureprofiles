# TE-5.1: gRIBI Get RPC

## Summary

Validate the accuracy, completeness, filtering options, network-instance
scoping, consistency with gNMI AFT telemetry, and response latency of the
`gRIBI.Get` RPC across single and dual client sessions, dual-stack scale
(`1,000` IPv4 and `1,000` IPv6 entries), a diverse set of entry types (weighted
multi-member and backup `NextHopGroup`s, decapsulation and encapsulation
`NextHop`s, mixed prefix lengths), multiple `L3VRF` network instances, and
negative/edge conditions as a pure control-plane specification.

## Testbed type

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Topology

```text
ATE port-1 <------> port-1 DUT port-2 <------> port-2 ATE
```

## Procedure

### Test environment setup

*   Connect ATE `port-1` to DUT `port-1` and ATE `port-2` to DUT `port-2`:
    *   Link 1 (`port-1`): DUT `192.0.2.1/30` (`2001:db8:1::1/126`), ATE
        `192.0.2.2/30` (`2001:db8:1::2/126`).
    *   Link 2 (`port-2`): DUT `192.0.2.5/30` (`2001:db8:2::1/126`), ATE
        `192.0.2.6/30` (`2001:db8:2::2/126`).
*   On platforms that require interfaces to be explicitly assigned to the
    default network instance, assign subinterface `0` of DUT `port-1` and
    `port-2` to it.
*   Ensure ARP and IPv6 Neighbor Discovery (ND) resolve between DUT and ATE on
    both links so next-hops pointing to ATE `port-1` (`192.0.2.2`) and ATE
    `port-2` (`192.0.2.6` and `2001:db8:2::2`) can be programmed in hardware
    FIB.
*   `DEFAULT` in this README denotes the DUT's default network-instance name,
    which is platform-specific (the `default_network_instance` deviation value
    where set, otherwise `DEFAULT`). Every gRIBI field that names the default
    network instance (`AFTOperation.network_instance`,
    `next_hop_group_network_instance`, `NextHop` `network_instance`, and the
    `GetRequest`/`FlushRequest` `network_instance.name`) and every expected
    `AFTEntry.network_instance` value uses that name, not a hard-coded literal.
*   Connect a primary gRIBI client to the DUT referred to as `gRIBI-A` and a
    secondary non-leader client referred to as `gRIBI-B`, both using `PRESERVE`
    persistence and `SINGLE_PRIMARY` redundancy mode with `RIB_AND_FIB_ACK`
    requested:
    *   Connect `gRIBI-A` first (initial `election_id = (12, 0)`) and make it
        the elected leader. The DUT does not reset the highest election ID it
        has learned, which may come from earlier sessions
        ([gRIBI specification section 4.1.2.1][spec-4121]), so `gRIBI-A`
        learns that value from `ModifyResponse.election_id` and then
        advertises the learned value plus one. This README calls the
        resulting election ID of `gRIBI-A` `EID_A`; it is greater than every
        election ID previously known to the DUT, including `(12, 0)`.
    *   Then connect `gRIBI-B` with `election_id = (11, 0)`, which is strictly
        lower than `EID_A`. Verify that the `ModifyResponse.election_id`
        returned to `gRIBI-B` equals `EID_A`, i.e., `gRIBI-B` observes
        `gRIBI-A` as the current leader and is not primary.
    *   Every `AFTOperation` carries the current election ID of its client
        (`EID_A` for `gRIBI-A`, `(11, 0)` for `gRIBI-B`), and every `Flush`
        from `gRIBI-A` that is not an `override` uses `election.id = EID_A`.
*   Flush all pre-existing gRIBI entries (`Flush` with `override` and
    `network_instance: {all: {}}`, expecting `FlushResponse.result = OK`)
    before starting test cases.
*   Acknowledgement handling: for every `Modify` batch in this test, the
    client first sends the complete `NextHop` -> `NextHopGroup` -> prefix
    sequence and then waits once for all `AFTResult`s within the timeout stated
    in the subtest. Some platforms send `FIB_PROGRAMMED` for a `NextHop` or
    `NextHopGroup` only after an entry that references it is installed
    (deviation `unreferenced_aft_fib_ack_unsupported`), so acknowledgements
    must not be awaited object by object.

#### Canonical OC

The following JSON is the end-state DUT configuration of this test: the test
environment setup configures the interfaces and the default network instance,
`TE-5.1.1` adds the static route for `198.51.100.192/26`, and `TE-5.1.4` adds
the `VRF-A` and `VRF-EMPTY` network instances.

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

### Common validations

The subtests apply the following checks.

*   **Get result check** (for a given `GetRequest` and expected entry set):
    *   Read the `GetResponse` stream until the server closes it with gRPC
        status `OK`; the returned set is the union of the `entry` fields of
        all `GetResponse` messages in the stream. `Get` latency is the time
        measured at the client from sending the `GetRequest` until the end of
        the stream.
    *   The returned set equals the expected set exactly: no missing, extra,
        or duplicate entries, keyed by `AFTEntry.network_instance`, entry type,
        and entry key (`prefix`, `NextHopGroup` `id`, or `NextHop` `index`).
    *   Each returned entry is field-for-field equal to the entry as
        programmed, ignoring `rib_status` and `fib_status`: `prefix`,
        `next_hop_group`, and `next_hop_group_network_instance` for IPv4/IPv6
        entries; every member `index` and `weight` and
        `backup_next_hop_group` for `NextHopGroup`s; and `ip_address`,
        `encapsulate_header`, `ip_in_ip` (`src_ip`, `dst_ip`),
        `decapsulate_header`, and `network_instance` for `NextHop`s. Fields
        that were not programmed must be unset. IP addresses and prefixes are
        compared in canonical form (RFC 5952 for IPv6).
    *   Each returned entry reports [rib_status]=`PROGRAMMED` and
        [fib_status]=`PROGRAMMED`.
*   **AFT consistency check** (for a network instance and the `Get` result for
    it): after `gNMI.Subscribe` (`ON_CHANGE`) has reported every expected
    gRIBI prefix in the network instance's AFT (no static sleeps), read the
    AFT with `gNMI.Get` and verify:
    *   The set of `ipv4-entry` and `ipv6-entry` prefixes whose
        `state/origin-protocol` is `GRIBI` equals the set of IPv4/IPv6
        prefixes in the `Get` result. Non-gRIBI entries (connected and static
        routes, and the AFT `next-hop-group`s and `next-hop`s that only they
        use) are ignored.
    *   An AFT `next-hop-group` matches gRIBI `NextHopGroup` `X` when its
        `state/programmed-id` equals `X` and it has the same number of
        `next-hops/next-hop` members as `X`, each member's `state/index`
        referring to an AFT `next-hop` whose `state/programmed-index` equals
        the corresponding `Get` member `index`, with an equal `state/weight`.
        AFT `next-hop-group/state/id` and `next-hop/state/index` values are
        device-assigned and are never compared directly with gRIBI IDs.
    *   For each such prefix, `state/next-hop-group` (looked up in the network
        instance named by `state/next-hop-group-network-instance`, or in the
        local network instance when that leaf is unset) refers to an AFT
        `next-hop-group` that matches the `Get` entry's `next_hop_group`.
    *   For each gRIBI `NextHopGroup` `X` in the `Get` result, at least one AFT
        `next-hop-group` matches `X` and, when `X` has
        `backup_next_hop_group = Y`, its `state/backup-next-hop-group` refers
        to an AFT `next-hop-group` that matches `Y`.
    *   For each gRIBI `NextHop` in the `Get` result, at least one AFT
        `next-hop` has `state/programmed-index` equal to its `index`, and every
        such AFT `next-hop` reports the same value for every attribute set in
        the `Get` entry: `ip_address` -> `state/ip-address`,
        `encapsulate_header` -> `state/encapsulate-header`, `ip_in_ip` ->
        `ip-in-ip/state/src-ip` and `ip-in-ip/state/dst-ip`,
        `decapsulate_header` -> `state/decapsulate-header`, and
        `network_instance` -> `state/network-instance`.
    *   On platforms that do not report `next-hop/state/encapsulate-header` in
        AFT telemetry, that leaf is excluded from this comparison. On platforms
        that do not report backup next-hop-groups in AFT telemetry,
        `state/backup-next-hop-group` is excluded, and a `NextHopGroup`
        referenced only as a backup, together with a `NextHop` used only by
        such a group, is not required to appear in the AFT. In both cases the
        `Get` response must still return these entries and attributes, as
        verified by the Get result check.

### TE-5.1.1 - Baseline Get RPC, Non-Leader Rejection, and Static Exclusion

*   Inject IPv4Entry cases for `198.51.100.0/26`, `198.51.100.64/26`,
    `198.51.100.128/26` pointing to `NextHopGroup` `42` (`DEFAULT`) and
    `NextHop` `1` (`192.0.2.6`, ATE port-2) via gRIBI-A. Verify that all `5`
    operations (`NextHop` `1`, `NextHopGroup` `42`, and the `3` IPv4Entry
    operations) return `FIB_PROGRAMMED` within `1` minute. Validate entries are
    installed through `gNMI.Subscribe` (`ON_CHANGE`) AFT telemetry: each of the
    `3` prefixes is present in the `DEFAULT` AFT within `1` minute.
*   Attempt to program the same `NextHop` `1`, `NextHopGroup` `42`, and `3`
    prefixes via non-leader `gRIBI-B`. Verify that every `AFTOperation` sent by
    `gRIBI-B` is rejected with `AFTResult.status = FAILED`
    (`ProgrammingFailed`) and that none returns `RIB_PROGRAMMED` or
    `FIB_PROGRAMMED`: in `SINGLE_PRIMARY` mode the server discards operations
    from a non-primary client and returns `FAILED` (`gribi.proto`
    `AFTOperation.election_id`).
*   Issue `gRIBI.Flush` from `gRIBI-B` with its own election ID
    (`election.id = (11, 0)`, lower than `EID_A`, and
    `network_instance: {name: "DEFAULT"}`). Verify
    the RPC fails with gRPC status `FAILED_PRECONDITION` and a
    `FlushResponseError` detail with reason `NOT_PRIMARY`
    ([gRIBI specification section 4.3.1.1][spec-4311]).
*   Issue `Get` RPC from gRIBI-A (`network_instance: all`, `aft: IPV4`) and
    apply the Get result check: exactly the entries for `198.51.100.0/26`,
    `198.51.100.64/26`, `198.51.100.128/26` are returned, each with
    `next_hop_group = 42`, which also verifies that the rejected `gRIBI-B`
    `Modify` and `Flush` requests left the leader's entries unchanged. Measure
    latency of Get RPC and verify it is `<= 5 seconds`.
*   Issue an additional `Get` RPC from gRIBI-A (`network_instance: all`,
    `aft: ALL`), because a `Get` with `aft: IPV4` returns only IPv4 entries
    ([gRIBI specification section 4.2.1][spec-421]). Apply the Get result
    check: exactly `5` entries are returned (the `3` IPv4Entry, `NextHopGroup`
    `42` with member `NextHop` `1` at `weight = 1`, and `NextHop` `1` with
    `ip_address = "192.0.2.6"`), each with [fib_status]=`PROGRAMMED` and
    [rib_status]=`PROGRAMMED`.
*   Issue `Get` RPC from gRIBI-B (`network_instance: all`, with `aft: IPV4`
    and with `aft: ALL`) and ensure that the entries are returned identically
    to the leader client (the same entries, attributes, and statuses as in the
    two previous steps). Measure latency of each Get RPC and verify it is
    `<= 5 seconds`.
*   Configure an OpenConfig static route for `198.51.100.192/26` pointing to
    `192.0.2.6` via `gNMI.Set` and wait until it is present in the `DEFAULT`
    AFT (`gNMI.Subscribe` `ON_CHANGE`, `1` minute timeout). Issue `Get` from
    gRIBI-A (`network_instance: all`, `aft: IPV4`), and ensure that only the
    gRIBI-programmed entries for `198.51.100.0/26`, `198.51.100.64/26`,
    `198.51.100.128/26` are returned, with no entry returned for the static
    route `198.51.100.192/26`. Verify through AFT telemetry that the `3` gRIBI
    prefixes are still present.
    *   Issue `Get` from gRIBI-A (`network_instance: all`, `aft: ALL`) and
        verify that the same `5` entries are returned with
        [fib_status]=`PROGRAMMED` and [rib_status]=`PROGRAMMED`, and that no
        `NextHopGroup` or `NextHop` derived from the static route is returned.
*   Inject an entry whose next-hop is valid but not resolvable: via gRIBI-A,
    program `NextHop` `1001` (`ip_address = "192.0.2.254"`, outside both
    connected subnets), `NextHopGroup` `1042` (`NextHop` `1001`,
    `weight = 1`), and IPv4Entry `203.0.113.0/24` -> `NextHopGroup` `1042` in
    `DEFAULT`. Per the [gRIBI specification section 4.1.3.2][spec-4132], an
    unviable but valid next-hop must not cause a `FAILED` status, and
    `FIB_PROGRAMMED` is returned for such next-hops and for the operations
    that use them (traffic to the prefix may be blackholed). Verify that all
    `3` operations return `FIB_PROGRAMMED` within `1` minute and that none
    returns `FAILED` or `FIB_FAILED`.
    *   Issue a Get RPC from gRIBI-A (`network_instance: all`, `aft: IPV4`) and
        apply the Get result check: exactly `4` entries are returned, the `3`
        prefixes above and `203.0.113.0/24` with `next_hop_group = 1042`, each
        with [rib_status]=`PROGRAMMED` and [fib_status]=`PROGRAMMED` (`Get`
        reports the server-acknowledged programming status,
        [gRIBI specification section 4.2.2][spec-422]).
    *   Issue a Get RPC from gRIBI-A (`network_instance: all`, `aft: ALL`) and
        apply the Get result check: exactly `8` entries are returned (the `4`
        IPv4Entry, `NextHopGroup`s `42` and `1042`, and `NextHop`s `1` and
        `1001` with `ip_address = "192.0.2.254"`).
    *   NOTE: The existing `get_rpc_test.go` automation programs `NextHop`
        `1001` with `ip_address = "192.0.2.254/32"` (a prefix, not an IP
        address) and asserts that `203.0.113.0/24` is absent from the `Get`
        response. It must be updated to program `ip_address = "192.0.2.254"`
        and to expect the `FIB_PROGRAMMED` acknowledgements and `Get` results
        specified above.

### TE-5.1.2 - Dual-Stack Scale Get Accuracy, Completeness, and Latency

*   Flush existing gRIBI entries on `DEFAULT` via `gRIBI-A`
    (`election.id = EID_A`). Verify `FlushResponse.result = OK` and that a
    `Get` (`network_instance: {name: "DEFAULT"}`, `aft: ALL`) returns `0`
    entries. The static route `198.51.100.192/26` from `TE-5.1.1` stays
    configured; it is not a gRIBI entry and must never be returned by `Get`.
*   Via `gRIBI-A` (`Modify` session with `RIB_AND_FIB_ACK`), program the
    following hierarchy in the `DEFAULT` network instance (`2,012`
    `AFTOperation`s). Send all `NextHop`s first, then `NextHopGroup` `301`
    before `NextHopGroup` `300` (which references it as its backup), then the
    remaining `NextHopGroup`s, and then all prefixes:
    *   Scale set:
        *   `NextHop` `10` (`ip_address = "192.0.2.6"`, ATE `port-2`) and
            `NextHopGroup` `100` referencing `NextHop` `10` (`weight = 1`).
        *   `NextHop` `20` (`ip_address = "2001:db8:2::2"`, ATE `port-2`) and
            `NextHopGroup` `200` referencing `NextHop` `20` (`weight = 1`).
        *   `1,000` `IPv4Entry` `/32` prefixes (`100.64.0.1/32` through
            `100.64.3.232/32`, i.e., the contiguous block `100.64.0.0 + i` =
            `100.64.<floor(i/256)>.<i mod 256>/32` for $i \in [1, 1000]$)
            referencing `NextHopGroup` `100`.
        *   `1,000` `IPv6Entry` `/128` prefixes (`2001:db8:20::1/128` through
            `2001:db8:20::3e8/128`, i.e., `2001:db8:20::<hex(i)>/128` for
            $i \in [1, 1000]$) referencing `NextHopGroup` `200`.
    *   Diverse set, exercising the attributes that `Get` must report:
        *   `NextHop` `30` (`ip_address = "192.0.2.2"`, ATE `port-1`).
        *   `NextHop` `31`, a decapsulation next-hop
            (`decapsulate_header = IPV4`, `network_instance = "DEFAULT"`). On
            platforms that do not support `network_instance` on a
            decapsulation next-hop, `NextHop` `31` is programmed without it
            (the lookup after decapsulation then uses the local `DEFAULT`
            instance) and `Get` must return it exactly as programmed.
        *   `NextHop` `32`, an IP-in-IP encapsulation next-hop
            (`encapsulate_header = IPV4`, `ip_in_ip.src_ip = "192.0.2.5"`,
            `ip_in_ip.dst_ip = "192.0.2.6"`, and
            `network_instance = "DEFAULT"`, the network instance in which the
            outer header is resolved).
        *   `NextHopGroup` `301` (backup group) referencing `NextHop` `31`
            (`weight = 1`).
        *   `NextHopGroup` `300`, a weighted multi-member group referencing
            `NextHop` `30` (`weight = 1`) and `NextHop` `10` (`weight = 3`),
            with `backup_next_hop_group = 301`.
        *   `NextHopGroup` `302` referencing `NextHop` `32` (`weight = 1`).
        *   `IPv4Entry` `100.64.4.0/24` referencing `NextHopGroup` `300` and
            `IPv4Entry` `100.64.5.0/28` referencing `NextHopGroup` `302`.
*   After the full `NextHop` -> `NextHopGroup` -> prefix sequence has been
    sent, confirm `FIB_PROGRAMMED` acknowledgements for all `2,012`
    `AFTOperation`s within `2` minutes, with zero `FAILED` or `FIB_FAILED`
    results.
*   Use `gNMI.Subscribe` (`ON_CHANGE`) on the `DEFAULT` AFT paths to verify
    that all `1,002` IPv4 prefixes and `1,000` IPv6 prefixes are present in
    OpenConfig AFT state within `2` minutes without static sleeps.
*   Issue `gRIBI.Get` (`network_instance: {name: "DEFAULT"}`, `aft: ALL`) from
    both `gRIBI-A` (leader) and `gRIBI-B` (non-leader), record end-to-end
    stream completion time until `EOF`, and apply the Get result check to each
    response:
    *   Verify the streamed `GetResponse` contains **exact** parity (`2,012`
        total `AFTEntry` records: `1,002` `ipv4`, `1,000` `ipv6`, `5`
        `next_hop_group`, and `5` `next_hop` entries) with zero missing,
        duplicate, or corrupted keys/attributes.
    *   Verify every returned `AFTEntry` reports `network_instance = "DEFAULT"`,
        `rib_status = PROGRAMMED`, and `fib_status = PROGRAMMED`, and matches
        the programmed entry field for field, including the `1:3` member
        weights and `backup_next_hop_group = 301` of `NextHopGroup` `300` and
        the decapsulation and encapsulation attributes of `NextHop`s `31` and
        `32`.
    *   Verify each `gRIBI.Get` call across the `2,012`-entry table completes
        end-to-end within **`<= 5 seconds`**.
*   Apply the AFT consistency check to `DEFAULT` against the `gRIBI-A` `Get`
    response: the AFT holds exactly `1,002` `ipv4-entry` and `1,000`
    `ipv6-entry` records with `origin-protocol = GRIBI`, at least one matching
    AFT `next-hop-group` for each of the `5` gRIBI `NextHopGroup`s
    (`programmed-id` `100`, `200`, `300`, `301`, and `302`, including the
    backup reference from `300` to `301`), and a matching AFT `next-hop` for
    each of the `5` gRIBI `NextHop`s (`programmed-index` `10`, `20`, `30`,
    `31`, and `32`) with the same next-hop attributes. On platforms that do
    not report backup next-hop-groups in AFT telemetry, `NextHopGroup` `301`
    and `NextHop` `31` are not required to appear in the AFT; `Get` must still
    return them.
*   Scale rationale: `1,000` entries per address family is functional scale
    for verifying `Get` completeness and multi-message streaming. The
    per-family entry counts are test parameters; the counts and the latency
    budget in this README apply to the default values. `Get` performance on a
    full-scale AFT is out of scope for this test and belongs in a dedicated
    scale test.

### TE-5.1.3 - GetRequest AFT Type Filtering (`AFTType` Options)

*   With the `2,012` entries active in `DEFAULT` from `TE-5.1.2`, issue
    `gRIBI.Get` requests from `gRIBI-A` for the `DEFAULT` network instance
    (`network_instance: {name: "DEFAULT"}`) for each specific `AFTType`
    defined in `gribi.proto` (`IPV4`, `IPV6`, `NEXTHOP_GROUP`, `NEXTHOP`,
    `MPLS`, `MAC`, and `POLICY_FORWARDING`; `ALL` is covered by `TE-5.1.2` and
    `TE-5.1.4`, and `INVALID` by `TE-5.1.5`), applying the Get result check
    with the expected subset:
    *   `aft = IPV4`: Verify exactly the `1,002` `IPv4Entry` records (the
        `1,000` `/32` prefixes, `100.64.4.0/24`, and `100.64.5.0/28`) are
        returned and zero `ipv6`, `next_hop_group`, or `next_hop` entries
        appear.
    *   `aft = IPV6`: Verify exactly the `1,000` `IPv6Entry` records are
        returned and zero other AFT types appear.
    *   `aft = NEXTHOP_GROUP`: Verify exactly the `5` `NextHopGroup` records
        (`100`, `200`, `300`, `301`, and `302`) are returned with accurate
        `next_hop` member IDs and weights (`300`: `NextHop` `30` at weight `1`
        and `NextHop` `10` at weight `3`; every other group: its single member
        at weight `1`), and with `backup_next_hop_group = 301` set on `300`
        only.
    *   `aft = NEXTHOP`: Verify exactly the `5` `NextHop` records (`10`, `20`,
        `30`, `31`, and `32`) are returned with exact attributes: `ip_address`
        `192.0.2.6`, `2001:db8:2::2`, and `192.0.2.2` for `10`, `20`, and
        `30`, the decapsulation attributes of `31`, and the encapsulation
        attributes of `32`.
    *   Verify the union of the `IPV4`, `IPV6`, `NEXTHOP_GROUP`, and `NEXTHOP`
        responses equals the `aft = ALL` response from `TE-5.1.2`.
    *   `aft = MPLS`, `aft = MAC`, and `aft = POLICY_FORWARDING`: this test
        programs no entries of these types. For each type, the expected
        outcome is selected per platform before the run (a platform deviation
        declares the AFT type unsupported via gRIBI); the test never accepts
        either outcome at runtime:
        *   On platforms that support the AFT type via gRIBI, verify the RPC
            completes with gRPC status `OK` and returns exactly `0` entries.
        *   On platforms that do not support the AFT type via gRIBI, verify
            the server responds with gRPC status `UNIMPLEMENTED`
            (`gribi.proto` `GetRequest.aft`,
            [gRIBI specification section 4.2.1][spec-421]).
        *   In both cases, verify that neither client's `Modify` session is
            closed.

### TE-5.1.4 - Multi-VRF Network-Instance Scoping (`name` vs `all`)

*   Using `gNMI.Set` (`UPDATE`), create two additional `L3VRF` network
    instances on the DUT:
    *   `VRF-A` (`type = L3VRF`)
    *   `VRF-EMPTY` (`type = L3VRF`, containing zero gRIBI entries)
*   Program `1,000` `IPv6Entry` `/128` prefixes (`2001:db8:30::1/128` through
    `2001:db8:30::3e8/128`, i.e., `2001:db8:30::<hex(i)>/128` for
    $i \in [1, 1000]$) inside `VRF-A` via `gRIBI-A` (`Modify`) with
    `next_hop_group = 200` and `next_hop_group_network_instance = "DEFAULT"`
    (reusing `NextHopGroup` `200` -> `NextHop` `20` in `DEFAULT` from `TE-5.1.2`
    so FIB resolution succeeds across `port-2` without changing interface
    bindings). Confirm `FIB_PROGRAMMED` acknowledgements for all `1,000`
    `AFTOperation`s within `2` minutes, and verify `VRF-A` AFT state via
    `gNMI.Subscribe` (`ON_CHANGE`): all `1,000` prefixes are present within
    `2` minutes.
*   Validate network-instance scoping via `gRIBI.Get` from `gRIBI-A`, applying
    the Get result check to each response:
    *   `GetRequest(network_instance: {name: "VRF-A"}, aft: ALL)`: Verify
        **only** the `1,000` `VRF-A` `IPv6Entry` prefixes (`2001:db8:30::1/128`
        through `2001:db8:30::3e8/128`) are returned (`1,000` total entries)
        with `network_instance = "VRF-A"`, `next_hop_group = 200`,
        `next_hop_group_network_instance = "DEFAULT"`,
        `rib_status = PROGRAMMED`, and `fib_status = PROGRAMMED`, and **zero**
        entries from `DEFAULT` leak into the response.
    *   `GetRequest(network_instance: {name: "DEFAULT"}, aft: ALL)`: Verify
        **only** the `2,012` `DEFAULT` entries from `TE-5.1.2` are returned and
        **zero** `VRF-A` entries appear.
    *   `GetRequest(network_instance: {all: {}}, aft: ALL)`: Verify the exact
        union of `DEFAULT` and `VRF-A` gRIBI entries (`3,012` total entries:
        `2,012` in `DEFAULT` + `1,000` in `VRF-A`) is returned with each
        `AFTEntry.network_instance` accurately populated and every entry equal
        to its programmed parameters, completing within `<= 5 seconds`.
*   Apply the AFT consistency check to `VRF-A` against the `VRF-A` `Get`
    response: the `VRF-A` AFT holds exactly `1,000` `ipv6-entry` records with
    `origin-protocol = GRIBI`, and the `next-hop-group` of each one (resolved
    through `next-hop-group-network-instance` as defined in the check) refers
    to an AFT `next-hop-group` that matches `NextHopGroup` `200`: its
    `programmed-id` is `200` and its single member maps to an AFT `next-hop`
    with `programmed-index` `20` and `ip-address` `2001:db8:2::2`.

### TE-5.1.5 - Negative and Edge-Case Get Validation

*   **Empty `L3VRF` Query**: Issue `gRIBI.Get` with
    `network_instance: {name: "VRF-EMPTY"}` and `aft: ALL`. Verify the RPC
    succeeds with gRPC status `OK` and returns `0` `AFTEntry` elements.
*   **Missing or Empty `network_instance` Specifier**: Issue `gRIBI.Get` with
    `network_instance` unset (`nil`) and with `network_instance: {name: ""}`
    (both with `aft: ALL`). Verify the DUT rejects both requests with
    canonical gRPC error code `INVALID_ARGUMENT` as specified in `gribi.proto`.
*   **Invalid `aft`**: Issue `gRIBI.Get` with
    `network_instance: {name: "DEFAULT"}` and `aft = INVALID` (enum value `0`,
    which is also the value sent when `aft` is left unset). Verify the DUT
    rejects the request with canonical gRPC error code `INVALID_ARGUMENT`
    (`gribi.proto` `GetRequest.aft`;
    [gRIBI specification section 4.2.1][spec-421] requires `aft` to be
    populated).
*   **Non-Existent Network Instance**: Issue `gRIBI.Get` with
    `network_instance: {name: "VRF-NONEXISTENT"}` and `aft: ALL`. Verify the
    DUT responds with gRPC error `INVALID_ARGUMENT`: a `Get` for a network
    instance that does not exist returns an error
    ([gRIBI specification section 4.1.7][spec-417]), and `INVALID_ARGUMENT`
    is the code that the specification defines for a network instance that
    does not exist ([section 4.3.1][spec-431]).
*   **Session stability**: After the requests above, verify that the gRIBI
    daemon has not crashed or dropped the active `Modify` sessions: neither
    `gRIBI-A` nor `gRIBI-B` observes its `Modify` stream being closed by the
    server, and `GetRequest(network_instance: {all: {}}, aft: ALL)` from
    `gRIBI-A` still passes the Get result check with the `3,012` entries from
    `TE-5.1.4`.

### Cleanup

*   From `gRIBI-A`, execute `gRIBI.Flush` with its current leader election ID
    (`election.id = EID_A`) on `network_instance: {name: "VRF-A"}` first and
    then on `network_instance: {name: "DEFAULT"}`. Flushing `VRF-A` first
    removes the `IPv6Entry` records that reference `NextHopGroup` `200` in
    `DEFAULT`, so no non-zero-referenced `NextHopGroup` or `NextHop` remains.
    Verify that both `Flush` calls return `FlushResponse.result = OK` (not
    `NON_ZERO_REFERENCE_REMAIN`) and that
    `Get(network_instance: {all: {}}, aft: ALL)` returns `0` entries.
*   Register a fallback `gRIBI.Flush` (`override`,
    `network_instance: {all: {}}`) in `t.Cleanup()` so that all gRIBI entries
    are removed even if the test aborts before the step above.
*   Remove `VRF-A`, `VRF-EMPTY`, and the static route `198.51.100.192/26` via
    `gNMI.Set` (`DELETE`) registered in `t.Cleanup()`, restoring the DUT to its
    baseline pre-test configuration.

[fib_status]: https://github.com/openconfig/gribi/blob/08d53dffce45e942c6e7f07521c58b557984e4b7/v1/proto/service/gribi.proto#L485
[rib_status]: https://github.com/openconfig/gribi/blob/08d53dffce45e942c6e7f07521c58b557984e4b7/v1/proto/service/gribi.proto#L483
[spec-4121]: https://github.com/openconfig/gribi/blob/master/doc/specification.md#4121-election-id-reset
[spec-4132]: https://github.com/openconfig/gribi/blob/master/doc/specification.md#4132-aft-operation-response
[spec-417]: https://github.com/openconfig/gribi/blob/master/doc/specification.md#417-about-gribi-server-caching
[spec-421]: https://github.com/openconfig/gribi/blob/master/doc/specification.md#421-getrequest-message
[spec-422]: https://github.com/openconfig/gribi/blob/master/doc/specification.md#422-getresponse-message
[spec-431]: https://github.com/openconfig/gribi/blob/master/doc/specification.md#431-flushrequest-message
[spec-4311]: https://github.com/openconfig/gribi/blob/master/doc/specification.md#4311-election-in-flushrequest-message

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
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group-network-instance:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/origin-protocol:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group-network-instance:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/origin-protocol:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/weight:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/backup-next-hop-group:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/programmed-id:
  /network-instances/network-instance/afts/next-hops/next-hop/ip-in-ip/state/dst-ip:
  /network-instances/network-instance/afts/next-hops/next-hop/ip-in-ip/state/src-ip:
  /network-instances/network-instance/afts/next-hops/next-hop/state/decapsulate-header:
  /network-instances/network-instance/afts/next-hops/next-hop/state/encapsulate-header:
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address:
  /network-instances/network-instance/afts/next-hops/next-hop/state/network-instance:
  /network-instances/network-instance/afts/next-hops/next-hop/state/programmed-index:
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

## Required DUT platform

vRX if the vendor implementation supports FIB-ACK simulation, otherwise FFF.
