# AFT-6.1: AFT Prefix Filtering

## Summary

This test validates the core AFT global filter mechanism, which restricts gNMI
streaming AFT updates to only the prefixes matching a specified routing policy.
Tests cover initial synchronization, dynamic updates, error handling for
non-existent policies, and policy deletion behavior.

See also:
[AFT-6.2](../afts_prefix_filtering_dualstack/README.md) (Dual-Stack),
[AFT-6.3](../afts_prefix_filtering_resilience/README.md) (Resilience),
[AFT-6.4](../afts_prefix_filtering_dynamic/README.md) (Dynamic Updates).

## Testbed type

[atedut_2.testbed](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Test Setup

### Test environment setup

- The `DUT` and `ATE` are connected via two links (`port1` and `port2`).

- Basic interface configuration is applied to the `DUT` and `ATE`.

- The DUT is pre-configured with static routes to populate the AFT. Routes
  include a mix of IPv4 and IPv6 prefixes drawn from RFC 5737 test address
  space (see IP address conventions in `CONTRIBUTING.md`). At minimum the
  following prefixes are installed in the `DEFAULT` network instance:
  `198.51.100.0/24`, `203.0.113.0/28`, `100.64.0.0/24`, `2001:DB8:1::/64`, and
  `2001:DB8:3::/64`.

- The DUT is pre-configured with the following routing policies under
  `/routing-policy/policy-definitions/`:

  - `POLICY-MATCH-ALL`: Matches all routes (unconditional accept).

  - `POLICY-PREFIX-SET-A`: Matches a specific set of IPv4 prefixes:
    `198.51.100.0/24`, `203.0.113.0/28`, and `198.51.100.1/32`.

  - `POLICY-PREFIX-SET-B`: Matches a specific set of IPv6 prefixes:
    `2001:DB8:2::/64` and `2001:DB8:2::1/128`.

  - `POLICY-PREFIX-SET-VRF-A`: Matches any IPv4 prefix within
    `100.64.1.0/24` with a masklength range of `/24` to `/32`.

  - `POLICY-SUBNET`: Matches any IPv4 prefix within `203.0.113.0/24` with a
    masklength range of `/25` to `/32` (i.e., any subnet of that block).

  - `POLICY-SUBNET-V6`: Matches any IPv6 prefix within `2001:DB8:3::/64`
    with a masklength range of `/65` to `/128` (i.e., any subnet of that
    block).

  - `POLICY-MULTI-STMT`: Two accept statements — statement 10 matches
    `PREFIX-SET-A` (ACCEPT_ROUTE), statement 20 matches `PREFIX-SET-SUBNET`
    (ACCEPT_ROUTE). Used to verify that all matching statements contribute to
    the filtered view.

  - `POLICY-DENY-PREFIX-SET-A`: Statement 10 explicitly denies prefixes in
    `PREFIX-SET-A` (REJECT_ROUTE); statement 20 accepts all remaining routes
    (unconditional ACCEPT_ROUTE). The prefix-set acts as an exclusion list.

  - `POLICY-TAG-MATCH`: Statement 10 matches routes carrying tag `999`
    (ACCEPT_ROUTE). No installed route uses this tag, so the policy matches
    nothing.

### Test Case Iteration

**AFT-6.1.1** is a parameterized test. Its subscribe and validate steps must be
repeated for each address family, substituting the appropriate policy:
`POLICY-PREFIX-SET-A` (IPv4) and `POLICY-PREFIX-SET-B` (IPv6), `POLICY-SUBNET`
(IPv4), and `POLICY-SUBNET-V6` (IPv6). For each iteration, only policy should be
configured (e.g., when testing IPv4, only `ipv4-policy` is set). Ensure the AFT
contains both matching and non-matching entries appropriate for the policy and
address family under test before subscribing. (Note: Simultaneous application of
both policies is covered in
[AFT-6.2.1](../afts_prefix_filtering_dualstack/README.md#aft-621---simultaneous-independent-ipv4-and-ipv6-policy-application)).

## Procedure

### AFT-6.1.1 - Validation of Subscription with Prefix-Set Policy

#### Configure Routing Policy and Prefixes

- Ensure `DUT` has `POLICY-PREFIX-SET-A` configured to match prefixes
  `198.51.100.0/24`, `203.0.113.0/28`, and `198.51.100.1/32`.

- Ensure the DUT's AFT contains entries for `198.51.100.0/24`,
  `203.0.113.0/28`, and at least one non-matching prefix (`100.64.0.0/24`).

- Configure the global-filter leaf for the address family under test to the
  selected policy. For example, when testing `POLICY-PREFIX-SET-A`:

  - Set
    `/network-instances/network-instance/afts/global-filter/config/ipv4-policy`
    to `POLICY-PREFIX-SET-A`.
  - Ensure
    `/network-instances/network-instance/afts/global-filter/config/ipv6-policy`
    is NOT set (or set to an empty/matching-none policy).

#### Subscribe

Establish a gNMI STREAM subscription (ON_CHANGE) to the DUT targeting the
following paths within the `DEFAULT` network instance AFT:

```protobuf
subscribe: {
  prefix: {
    target: "target-device"
    origin: "openconfig"
    path: {
      elem: { name: "network-instances" }
      elem: { name: "network-instance" key: { key: "name" value: "DEFAULT" } }
      elem: { name: "afts" }
    }
  }
  subscription: {
    path: {
      elem: { name: "global-filter" }
      elem: { name: "state" }
    }
    mode: ON_CHANGE
  }
  subscription: {
    path: {
      elem: { name: "ipv4-unicast" }
      elem: { name: "ipv4-entry" }
    }
    mode: ON_CHANGE
  }
  subscription: {
    path: {
      elem: { name: "ipv6-unicast" }
      elem: { name: "ipv6-entry" }
    }
    mode: ON_CHANGE
  }
  subscription: {
    path: {
      elem: { name: "next-hop-groups" }
      elem: { name: "next-hop-group" }
    }
    mode: ON_CHANGE
  }
  subscription: {
    path: {
      elem: { name: "next-hops" }
      elem: { name: "next-hop" }
    }
    mode: ON_CHANGE
  }
  mode: STREAM
  encoding: PROTO
}
```

#### Validate Initial Synced Data

- Wait for the initial set of gNMI Notifications and verify `SYNC` is
  received.

- Verify that Notifications are received **only** for prefixes matching
  `POLICY-PREFIX-SET-A` (`198.51.100.0/24`, `203.0.113.0/28`),
  plus any necessary recursive resolution prefixes.

- Verify that the non-matching prefix (`100.64.0.0/24`) is **not** received.

- Verify that all next-hop-groups and next-hops referenced by matching
  prefixes are received.

- Verify that the `atomic` flag is set to `true` on all initial update
  notifications. (See [AFT-3.1](../../../otg_tests/afts_atomic/README.md) for complete atomic-flag behavior coverage.)

#### Validate Dynamic Updates

- Add a new prefix (`198.51.100.1/32`) to the DUT that matches
  `POLICY-PREFIX-SET-A`. Verify receipt of an update notification for this
  prefix and its associated next-hop-groups and next-hops.

- Remove `198.51.100.1/32` from the DUT. Verify receipt of a delete
  notification for the prefix. Verify that the next-hop-group and next-hop
  shared with `198.51.100.0/24` are **not** deleted, since they are still
  referenced by the remaining prefix.

- Add a new prefix (`100.64.1.0/24`) to the DUT that does **not** match the
  routing policy. Verify that **no** gNMI update is received for this prefix.

#### Remove the Filtered View

- Delete the `global-filter` configuration from the DUT.

- Verify receipt of a delete notification for
  `/network-instances/network-instance/afts/global-filter/state/ipv4-policy`
  and
  `/network-instances/network-instance/afts/global-filter/state/ipv6-policy`.

- Verify that the previously excluded prefix `100.64.0.0/24` is now received,
  confirming the filter has been lifted.

### AFT-6.1.2 - Validation with Non-Existent Policy

- Attempt to configure the AFT global filter `ipv4-policy` and `ipv6-policy`
  with `POLICY-DOES-NOT-YET-EXIST`.

  - Verify a `FAILED_PRECONDITION` error is returned.

- Apply a configuration to the DUT defining `POLICY-DOES-NOT-YET-EXIST` to
  match prefix `198.51.100.128/25`.

- Again configure the AFT global filter `ipv4-policy` and `ipv6-policy` to
  `POLICY-DOES-NOT-YET-EXIST`. Verify no error is returned.

- Subscribe to the AFT as in **AFT-6.1.1** and verify:

  - `SYNC` is received.
  - Notifications are received only for `198.51.100.128/25`. All expected
    next-hop-groups and next-hops are received normally.
  - Prefixes that do not match `POLICY-DOES-NOT-YET-EXIST` are **not**
    received.

### AFT-6.1.3 - Validation of Policy Deletion

- Configure the device to filter AFT using `POLICY-PREFIX-SET-A`.

- Establish a gNMI Subscribe session and wait for `SYNC`.

- Attempt to delete `POLICY-PREFIX-SET-A` from the DUT while the global filter
  still references it.

  - Verify a `FAILED_PRECONDITION` error is returned, indicating the
    global-filter reference must be removed first.

- Delete both the global filter and `POLICY-PREFIX-SET-A` in a single atomic
  gNMI.Set request.

  - Verify no errors are returned.

- Verify that the previously excluded prefix `100.64.0.0/24` is now received,
  confirming the filter has been lifted.

- Re-configure `POLICY-PREFIX-SET-A` and set the global filter to reference
  it.

- Verify notifications match the expected filtered set as in **AFT-6.1.1**.

#### Multi-Step Policy Deletion

- Delete the global filter reference in a first gNMI.Set request. Verify no
  error is returned.

- Delete `POLICY-PREFIX-SET-A` in a separate, second gNMI.Set request. Verify
  no error is returned.

- Verify that the previously excluded prefix `100.64.0.0/24` is now received,
  confirming the filter has been lifted.

### AFT-6.1.4 - Changing the Prefix-Set Referenced by the Active Policy

- Configure the global-filter `ipv4-policy` to `POLICY-PREFIX-SET-A`.

- Establish a gNMI Subscribe session as in **AFT-6.1.1** and wait for `SYNC`.
  Verify notifications are received for `198.51.100.0/24` and `203.0.113.0/28`.

- Update `POLICY-PREFIX-SET-A` to reference `PREFIX-SET-B` instead of
  `PREFIX-SET-A` (i.e., swap the prefix-set the policy matches against).

- Verify that delete notifications are received for `198.51.100.0/24` and
  `203.0.113.0/28` (no longer matched by the updated policy).

- Verify that no new update notifications are received for prefixes in
  `PREFIX-SET-B`, since the DUT's AFT contains no installed entries matching
  those prefixes.

### AFT-6.1.5 - Policy with Multiple Statements Referencing Different Prefix-Sets

- Install an additional route `203.0.113.128/25` on the DUT (matches
  `PREFIX-SET-SUBNET` via `203.0.113.0/24` with masklength `/25`–`/32`).

- Configure the global-filter `ipv4-policy` to `POLICY-MULTI-STMT`.

- Establish a gNMI Subscribe session as in **AFT-6.1.1** and wait for `SYNC`.

- Verify that notifications are received for:
  - `198.51.100.0/24` and `203.0.113.0/28` (matched by statement 10 via
    `PREFIX-SET-A`)
  - `203.0.113.128/25` (matched by statement 20 via `PREFIX-SET-SUBNET`)

- Verify that the non-matching prefix `100.64.0.0/24` is **not** received.

- Verify that all next-hop-groups and next-hops referenced by the matching
  prefixes are received.

### AFT-6.1.6 - Policy with DENY Action (Prefix-Set as Exclusion List)

- Configure the global-filter `ipv4-policy` to `POLICY-DENY-PREFIX-SET-A`.

- Establish a gNMI Subscribe session as in **AFT-6.1.1** and wait for `SYNC`.

- Verify that notifications are **not** received for prefixes explicitly denied
  by statement 10: `198.51.100.0/24` and `203.0.113.0/28`.

- Verify that a notification **is** received for `100.64.0.0/24`, which is
  accepted by the unconditional statement 20.

- Verify that all next-hop-groups and next-hops referenced by the accepted
  prefix are received.

### AFT-6.1.7 - Negative: Non-Prefix-Set Match Criteria

#### Fresh Subscribe with Non-Matching Policy

- Configure the global-filter `ipv4-policy` to `POLICY-TAG-MATCH`.

- Establish a gNMI Subscribe session as in **AFT-6.1.1** and wait for `SYNC`.

- Verify that **no** prefix notifications are received (the policy matches no
  installed routes). All expected next-hop-groups and next-hops are received
  normally.

#### Transition from Active Policy to Non-Matching Policy

- Configure the global-filter `ipv4-policy` to `POLICY-PREFIX-SET-A`.

- Establish a gNMI Subscribe session and wait for `SYNC`. Verify notifications
  are received for `198.51.100.0/24` and `203.0.113.0/28`.

- Update the global-filter `ipv4-policy` to `POLICY-TAG-MATCH`.

- Verify that delete notifications are received for the previously streamed
  prefixes `198.51.100.0/24` and `203.0.113.0/28`.

- Verify that **no** new update notifications are received.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  # Global filter config/state paths
  /network-instances/network-instance/afts/global-filter/config/ipv4-policy:
  /network-instances/network-instance/afts/global-filter/config/ipv6-policy:
  /network-instances/network-instance/afts/global-filter/state/ipv4-policy:
  /network-instances/network-instance/afts/global-filter/state/ipv6-policy:

  # Standard AFT state paths
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/weight:
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address:
  /network-instances/network-instance/afts/next-hops/next-hop/interface-ref/state/interface:

  # Paths for configuring policies and prefix-sets (used in setup)
  /routing-policy/defined-sets/prefix-sets/prefix-set/config/name:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range:
  /routing-policy/policy-definitions/policy-definition/config/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result:
  /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/index:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop:

rpcs:
  gnmi:
    gNMI.Subscribe:
      STREAM: true
      ON_CHANGE: true
    gNMI.Set:
      REPLACE: true
      UPDATE: true
      DELETE: true
```

## Canonical OC

The following JSON shows the expected OpenConfig configuration for the routing
policies and static routes used in the test setup. The `global-filter`
configuration is omitted here because the OC schema bundled in featureprofiles
has not yet been updated to include the merged YANG changes.

```json
{
  "routing-policy": {
    "defined-sets": {
      "prefix-sets": {
        "prefix-set": [
          {
            "name": "PREFIX-SET-A",
            "config": {
              "name": "PREFIX-SET-A"
            },
            "prefixes": {
              "prefix": [
                {
                  "ip-prefix": "198.51.100.0/24",
                  "masklength-range": "exact",
                  "config": {
                    "ip-prefix": "198.51.100.0/24",
                    "masklength-range": "exact"
                  }
                },
                {
                  "ip-prefix": "203.0.113.0/28",
                  "masklength-range": "exact",
                  "config": {
                    "ip-prefix": "203.0.113.0/28",
                    "masklength-range": "exact"
                  }
                },
                {
                  "ip-prefix": "198.51.100.1/32",
                  "masklength-range": "exact",
                  "config": {
                    "ip-prefix": "198.51.100.1/32",
                    "masklength-range": "exact"
                  }
                }
              ]
            }
          },
          {
            "name": "PREFIX-SET-B",
            "config": {
              "name": "PREFIX-SET-B"
            },
            "prefixes": {
              "prefix": [
                {
                  "ip-prefix": "2001:DB8:2::/64",
                  "masklength-range": "exact",
                  "config": {
                    "ip-prefix": "2001:DB8:2::/64",
                    "masklength-range": "exact"
                  }
                },
                {
                  "ip-prefix": "2001:DB8:2::1/128",
                  "masklength-range": "exact",
                  "config": {
                    "ip-prefix": "2001:DB8:2::1/128",
                    "masklength-range": "exact"
                  }
                }
              ]
            }
          },
          {
            "name": "PREFIX-SET-VRF-A",
            "config": {
              "name": "PREFIX-SET-VRF-A"
            },
            "prefixes": {
              "prefix": [
                {
                  "ip-prefix": "100.64.1.0/24",
                  "masklength-range": "24..32",
                  "config": {
                    "ip-prefix": "100.64.1.0/24",
                    "masklength-range": "24..32"
                  }
                }
              ]
            }
          },
          {
            "name": "PREFIX-SET-SUBNET",
            "config": {
              "name": "PREFIX-SET-SUBNET"
            },
            "prefixes": {
              "prefix": [
                {
                  "ip-prefix": "203.0.113.0/24",
                  "masklength-range": "25..32",
                  "config": {
                    "ip-prefix": "203.0.113.0/24",
                    "masklength-range": "25..32"
                  }
                }
              ]
            }
          },
          {
            "name": "PREFIX-SET-SUBNET-V6",
            "config": {
              "name": "PREFIX-SET-SUBNET-V6"
            },
            "prefixes": {
              "prefix": [
                {
                  "ip-prefix": "2001:DB8:3::/64",
                  "masklength-range": "65..128",
                  "config": {
                    "ip-prefix": "2001:DB8:3::/64",
                    "masklength-range": "65..128"
                  }
                }
              ]
            }
          }
        ]
      }
    },
    "policy-definitions": {
      "policy-definition": [
        {
          "name": "POLICY-MATCH-ALL",
          "config": {
            "name": "POLICY-MATCH-ALL"
          },
          "statements": {
            "statement": [
              {
                "name": "10",
                "config": { "name": "10" },
                "actions": {
                  "config": { "policy-result": "ACCEPT_ROUTE" }
                }
              }
            ]
          }
        },
        {
          "name": "POLICY-PREFIX-SET-A",
          "config": {
            "name": "POLICY-PREFIX-SET-A"
          },
          "statements": {
            "statement": [
              {
                "name": "10",
                "config": { "name": "10" },
                "conditions": {
                  "match-prefix-set": {
                    "config": {
                      "prefix-set": "PREFIX-SET-A",
                      "match-set-options": "ANY"
                    }
                  }
                },
                "actions": {
                  "config": { "policy-result": "ACCEPT_ROUTE" }
                }
              }
            ]
          }
        },
        {
          "name": "POLICY-PREFIX-SET-B",
          "config": {
            "name": "POLICY-PREFIX-SET-B"
          },
          "statements": {
            "statement": [
              {
                "name": "10",
                "config": { "name": "10" },
                "conditions": {
                  "match-prefix-set": {
                    "config": {
                      "prefix-set": "PREFIX-SET-B",
                      "match-set-options": "ANY"
                    }
                  }
                },
                "actions": {
                  "config": { "policy-result": "ACCEPT_ROUTE" }
                }
              }
            ]
          }
        },
        {
          "name": "POLICY-PREFIX-SET-VRF-A",
          "config": {
            "name": "POLICY-PREFIX-SET-VRF-A"
          },
          "statements": {
            "statement": [
              {
                "name": "10",
                "config": { "name": "10" },
                "conditions": {
                  "match-prefix-set": {
                    "config": {
                      "prefix-set": "PREFIX-SET-VRF-A",
                      "match-set-options": "ANY"
                    }
                  }
                },
                "actions": {
                  "config": { "policy-result": "ACCEPT_ROUTE" }
                }
              }
            ]
          }
        },
        {
          "name": "POLICY-SUBNET",
          "config": {
            "name": "POLICY-SUBNET"
          },
          "statements": {
            "statement": [
              {
                "name": "10",
                "config": { "name": "10" },
                "conditions": {
                  "match-prefix-set": {
                    "config": {
                      "prefix-set": "PREFIX-SET-SUBNET",
                      "match-set-options": "ANY"
                    }
                  }
                },
                "actions": {
                  "config": { "policy-result": "ACCEPT_ROUTE" }
                }
              }
            ]
          }
        },
        {
          "name": "POLICY-SUBNET-V6",
          "config": {
            "name": "POLICY-SUBNET-V6"
          },
          "statements": {
            "statement": [
              {
                "name": "10",
                "config": { "name": "10" },
                "conditions": {
                  "match-prefix-set": {
                    "config": {
                      "prefix-set": "PREFIX-SET-SUBNET-V6",
                      "match-set-options": "ANY"
                    }
                  }
                },
                "actions": {
                  "config": { "policy-result": "ACCEPT_ROUTE" }
                }
              }
            ]
          }
        },
        {
          "name": "POLICY-MULTI-STMT",
          "config": {
            "name": "POLICY-MULTI-STMT"
          },
          "statements": {
            "statement": [
              {
                "name": "10",
                "config": { "name": "10" },
                "conditions": {
                  "match-prefix-set": {
                    "config": {
                      "prefix-set": "PREFIX-SET-A",
                      "match-set-options": "ANY"
                    }
                  }
                },
                "actions": {
                  "config": { "policy-result": "ACCEPT_ROUTE" }
                }
              },
              {
                "name": "20",
                "config": { "name": "20" },
                "conditions": {
                  "match-prefix-set": {
                    "config": {
                      "prefix-set": "PREFIX-SET-SUBNET",
                      "match-set-options": "ANY"
                    }
                  }
                },
                "actions": {
                  "config": { "policy-result": "ACCEPT_ROUTE" }
                }
              }
            ]
          }
        },
        {
          "name": "POLICY-DENY-PREFIX-SET-A",
          "config": {
            "name": "POLICY-DENY-PREFIX-SET-A"
          },
          "statements": {
            "statement": [
              {
                "name": "10",
                "config": { "name": "10" },
                "conditions": {
                  "match-prefix-set": {
                    "config": {
                      "prefix-set": "PREFIX-SET-A",
                      "match-set-options": "ANY"
                    }
                  }
                },
                "actions": {
                  "config": { "policy-result": "REJECT_ROUTE" }
                }
              },
              {
                "name": "20",
                "config": { "name": "20" },
                "actions": {
                  "config": { "policy-result": "ACCEPT_ROUTE" }
                }
              }
            ]
          }
        }
      ]
    }
  },
  "network-instances": {
    "network-instance": [
      {
        "name": "DEFAULT",
        "protocols": {
          "protocol": [
            {
              "identifier": "STATIC",
              "name": "STATIC",
              "config": {
                "identifier": "STATIC",
                "name": "STATIC"
              },
              "static-routes": {
                "static": [
                  {
                    "prefix": "198.51.100.0/24",
                    "config": { "prefix": "198.51.100.0/24" },
                    "next-hops": {
                      "next-hop": [
                        {
                          "index": "0",
                          "config": { "index": "0", "next-hop": "192.0.2.2" }
                        }
                      ]
                    }
                  },
                  {
                    "prefix": "203.0.113.0/28",
                    "config": { "prefix": "203.0.113.0/28" },
                    "next-hops": {
                      "next-hop": [
                        {
                          "index": "0",
                          "config": { "index": "0", "next-hop": "192.0.2.2" }
                        }
                      ]
                    }
                  },
                  {
                    "prefix": "100.64.0.0/24",
                    "config": { "prefix": "100.64.0.0/24" },
                    "next-hops": {
                      "next-hop": [
                        {
                          "index": "0",
                          "config": { "index": "0", "next-hop": "192.0.2.2" }
                        }
                      ]
                    }
                  },
                  {
                    "prefix": "2001:DB8:1::/64",
                    "config": { "prefix": "2001:DB8:1::/64" },
                    "next-hops": {
                      "next-hop": [
                        {
                          "index": "0",
                          "config": { "index": "0", "next-hop": "2001:DB8::2" }
                        }
                      ]
                    }
                  },
                  {
                    "prefix": "2001:DB8:3::/64",
                    "config": { "prefix": "2001:DB8:3::/64" },
                    "next-hops": {
                      "next-hop": [
                        {
                          "index": "0",
                          "config": { "index": "0", "next-hop": "2001:DB8::2" }
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

## Required DUT platform

FFF (Fixed Form Factor) or MFF (Modular Form Factor).
