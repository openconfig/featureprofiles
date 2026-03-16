# AFT-6.1: AFT Prefix Filtering

## Summary

This test validates the AFT global filter mechanism, which restricts gNMI
streaming AFT updates to only the prefixes matching a specified routing policy.
Tests cover initial synchronization, dynamic updates, error handling for
non-existent and deleted policies, device reboot persistence, scale behavior,
and per-network-instance filter isolation with multiple collectors.

## Testbed type

[atedut_2.testbed](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Test Setup

### Test environment setup

-   The `DUT` and `ATE` are connected via two links (`port1` and `port2`).

-   Basic interface configuration is applied to the `DUT` and `ATE`.

-   The DUT is pre-configured with static routes to populate the AFT. Routes
    include a mix of IPv4 and IPv6 prefixes drawn from RFC 5737 test address
    space (see IP address conventions in `CONTRIBUTING.md`). At minimum the
    following prefixes are installed in the `DEFAULT` network instance:
    `198.51.100.0/24`, `203.0.113.0/28`, `100.64.0.0/24`, `2001:DB8:1::/64`, and
    `2001:DB8:3::/64`.

-   The DUT is pre-configured with the following routing policies under
    `/routing-policy/policy-definitions/`:

    -   `POLICY-MATCH-ALL`: Matches all routes (unconditional accept).

    -   `POLICY-PREFIX-SET-A`: Matches a specific set of IPv4 prefixes:
        `198.51.100.0/24`, `203.0.113.0/28`, and `198.51.100.1/32`.

    -   `POLICY-PREFIX-SET-B`: Matches a specific set of IPv6 prefixes:
        `2001:DB8:2::/64` and `2001:DB8:2::1/128`.

    -   `POLICY-PREFIX-SET-VRF-A`: Matches any IPv4 prefix within
        `100.64.1.0/24` with a masklength range of `/24` to `/32`.

    -   `POLICY-SUBNET`: Matches any IPv4 prefix within `203.0.113.0/24` with a
        masklength range of `/25` to `/32` (i.e., any subnet of that block).

    -   `POLICY-SUBNET-V6`: Matches any IPv6 prefix within `2001:DB8:3::/64`
        with a masklength range of `/65` to `/128` (i.e., any subnet of that
        block).

### Test Case Iteration

**AFT-6.1.1** is a parameterized test. Its subscribe and validate steps must be
repeated for each address family, substituting the appropriate policy:
`POLICY-PREFIX-SET-A` (IPv4) and `POLICY-PREFIX-SET-B` (IPv6), `POLICY-SUBNET`
(IPv4), and `POLICY-SUBNET-V6` (IPv6). For each iteration, only policy should be
configured (e.g., when testing IPv4, only `ipv4-policy` is set). Ensure the AFT
contains both matching and non-matching entries appropriate for the policy and
address family under test before subscribing. (Note: Simultaneous application of
both policies is covered in **AFT-6.1.7**).

## AFT-6.1.1 - Validation of Subscription with Prefix-Set Policy

### Configure Routing Policy and Prefixes

-   Ensure `DUT` has `POLICY-PREFIX-SET-A` configured to match prefixes
    `198.51.100.0/24`, `203.0.113.0/28`, and `198.51.100.1/32`.

-   Ensure the DUT's AFT contains entries for `198.51.100.0/24`,
    `203.0.113.0/28`, and at least one non-matching prefix (`100.64.0.0/24`).

-   Configure the global-filter leaf for the address family under test to the
    selected policy. For example, when testing `POLICY-PREFIX-SET-A`:

    -   Set
        `/network-instances/network-instance/afts/global-filter/config/ipv4-policy`
        to `POLICY-PREFIX-SET-A`.
    -   Ensure
        `/network-instances/network-instance/afts/global-filter/config/ipv6-policy`
        is NOT set (or set to an empty/matching-none policy).

### Subscribe

Establish a long-lived gNMI STREAM subscription to the DUT targeting the
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

### Validate Initial Synced Data

-   Wait for the initial set of gNMI Notifications and verify `SYNC` is
    received.

-   Verify that Notifications are received **only** for prefixes matching
    `POLICY-PREFIX-SET-A` (`198.51.100.0/24`, `203.0.113.0/28`,
    `198.51.100.1/32`), plus any necessary recursive resolution prefixes.

-   Verify that the non-matching prefix (`100.64.0.0/24`) is **not** received.

-   Verify that all next-hop-groups and next-hops referenced by matching
    prefixes are received.

-   Verify that every received next-hop-group is referenced by at least one
    received prefix.

-   Verify that every received next-hop is referenced by at least one received
    next-hop-group.

-   Verify that the `atomic` flag is set to `true` on all initial update
    notifications. (See AFT-3.1 for complete atomic-flag behavior coverage.)

### Validate Dynamic Updates

-   Add a new prefix (`198.51.100.1/32`) to the DUT that matches
    `POLICY-PREFIX-SET-A`. Verify receipt of an update notification for this
    prefix and its associated next-hop-groups and next-hops.

-   Remove `198.51.100.1/32` from the DUT. Verify receipt of a delete
    notification for the prefix and its associated next-hop-groups and
    next-hops.

-   Add a new prefix (`100.64.1.0/24`) to the DUT that does **not** match the
    routing policy. Verify that **no** gNMI update is received for this prefix.

### Remove the Filtered View

-   Delete the `global-filter` configuration from the DUT.

-   Verify receipt of a delete notification for
    `/network-instances/network-instance/afts/global-filter/state/ipv4-policy`
    and
    `/network-instances/network-instance/afts/global-filter/state/ipv6-policy`.

-   Verify that the previously excluded prefix `100.64.0.0/24` is now received,
    confirming the filter has been lifted.

## AFT-6.1.2 - Validation with Non-Existent Policy

-   Attempt to configure the AFT global filter `ipv4-policy` and `ipv6-policy`
    with `POLICY-DOES-NOT-YET-EXIST`.

    -   Verify a `FAILED_PRECONDITION` error is returned.

-   Apply a configuration to the DUT defining `POLICY-DOES-NOT-YET-EXIST` to
    match prefix `198.51.100.128/25`.

-   Again configure the AFT global filter `ipv4-policy` and `ipv6-policy` to
    `POLICY-DOES-NOT-YET-EXIST`. Verify no error is returned.

-   Subscribe to the AFT as in **AFT-6.1.1** and verify:

    -   `SYNC` is received.
    -   Notifications are received only for `198.51.100.128/25` and its
        associated next-hops/groups.
    -   Prefixes that do not match `POLICY-DOES-NOT-YET-EXIST` are **not**
        received.

## AFT-6.1.3 - Validation of Policy Deletion

-   Configure the device to filter AFT using `POLICY-PREFIX-SET-A`.

-   Establish a gNMI Subscribe session and wait for `SYNC`.

-   Attempt to delete `POLICY-PREFIX-SET-A` from the DUT while the global filter
    still references it.

    -   Verify a `FAILED_PRECONDITION` error is returned, indicating the
        global-filter reference must be removed first.

-   Delete both the global filter and `POLICY-PREFIX-SET-A` in a single atomic
    gNMI.Set request.

    -   Verify no errors are returned.

-   Verify that the previously excluded prefix `100.64.0.0/24` is now received,
    confirming the filter has been lifted.

-   Re-configure `POLICY-PREFIX-SET-A` and set the global filter to reference
    it.

-   Verify notifications match the expected filtered set as in **AFT-6.1.1**.

## AFT-6.1.4 - Validation After Device Reboot

### Establish Subscription

-   Configure `POLICY-PREFIX-SET-A` and set the global filter `ipv4-policy` and
    `ipv6-policy` to it.

-   Establish a successful gNMI subscription and verify the initial filtered set
    of AFT entries is received.

### Reboot DUT

-   Issue a reboot command to the DUT via `gNOI.System/Reboot` while the
    subscription is active.

-   Verify the gNMI stream terminates.

### Await DUT Readiness

-   Wait for the DUT to become reachable and for gNMI to be available after
    reboot.

### Re-establish Subscription

-   Re-establish the same gNMI subscription.

-   Verify the subscription is successfully established. Only
    endpoint-unreachable errors during the reboot window are acceptable; no
    internal errors should be returned.

-   Verify that `global-filter/config/ipv4-policy` and
    `global-filter/config/ipv6-policy` still hold `POLICY-PREFIX-SET-A`,
    confirming the configuration persisted across reboot.

-   Verify that the DUT streams the correct filtered set of AFT entries matching
    `POLICY-PREFIX-SET-A`.

## AFT-6.1.5 - Scale Test

-   Let `X` be the number of IPv4 routes to advertise from the ATE. **(User
    Adjustable Value, default: 5000)**

-   Let `Y` be the number of IPv6 routes to advertise from the ATE. **(User
    Adjustable Value, default: 2000)**

-   Let `K` be the maximum allowed synchronization time in seconds. **(User
    Adjustable Value, default: 120)**

-   Populate the AFT with `X` IPv4 routes and `Y` IPv6 routes by advertising
    them from the connected ATE.

-   Configure three routing policies matching approximately 1%, 5%, and 20% of
    the total route set for the address family under test, respectively.

-   Establish two simultaneous gNMI STREAM subscriptions to the AFT. For each
    policy scenario and address family, wait until both subscriptions receive
    `SYNC` and all expected leaves.

-   Measure the time taken for initial synchronization for each scenario.

-   Verify that in all cases, synchronization completes within `K` seconds.

-   Verify correct filtering is applied in all scenarios (only matching prefixes
    for the active address family are streamed).

## AFT-6.1.6 - Per Network-Instance Filtering with Multiple Collectors

To validate that AFT filters are applied independently per network instance and
that multiple collectors can subscribe to different network instances with their
respective filters.

### Setup

-   Configure two network instances on the DUT: `DEFAULT` and `VRF-A`.

-   Populate both instances with distinct static routes. The prefix
    `198.51.100.0/24` appears in both instances to verify filter independence.

    -   `DEFAULT`: `198.51.100.0/24`, `203.0.113.0/28`, `100.64.0.0/24`
    -   `VRF-A`: `198.51.100.0/24`, `100.64.1.0/24`, `203.0.113.128/28`

-   Configure the following routing policies:

    -   `POLICY-PREFIX-SET-A`: Matches `198.51.100.0/24`, `203.0.113.0/28`, and
        `198.51.100.1/32`.
    -   `POLICY-PREFIX-SET-VRF-A`: Matches `100.64.1.0/24` and subnets up to
        `/32`.
    -   `POLICY-MATCH-ALL`: Matches all routes.

-   Configure AFT filters:

    -   `DEFAULT`:
        `/network-instances/network-instance[name=DEFAULT]/afts/global-filter/config/ipv4-policy` =
        `POLICY-PREFIX-SET-A`
        `/network-instances/network-instance[name=DEFAULT]/afts/global-filter/config/ipv6-policy` =
        `POLICY-PREFIX-SET-A`
    -   `VRF-A`:
        `/network-instances/network-instance[name=VRF-A]/afts/global-filter/config/ipv4-policy` =
        `POLICY-PREFIX-SET-VRF-A`
        `/network-instances/network-instance[name=VRF-A]/afts/global-filter/config/ipv6-policy` =
        `POLICY-PREFIX-SET-VRF-A`

### Validation

-   **Collector 1**: Establishes a gNMI subscription to AFT paths within the
    `DEFAULT` network instance.

-   **Collector 2**: Establishes a gNMI subscription to AFT paths within the
    `VRF-A` network instance.

-   Collector 1: Verify `SYNC` and receipt of only `198.51.100.0/24` and
    `203.0.113.0/28` from `DEFAULT`, with associated next-hops/groups. Verify
    `100.64.0.0/24` is **not** received.

-   Collector 2: Verify `SYNC` and receipt of only `100.64.1.0/24` from `VRF-A`,
    with associated next-hops/groups. Verify `198.51.100.0/24` and
    `203.0.113.128/28` are **not** received.

-   Add `100.64.2.0/24` to `DEFAULT`. Verify **neither** collector receives an
    update.

-   Add `203.0.113.64/28` to `DEFAULT`. Verify **neither** collector receives an
    update (not matched by either collector's active policy).

-   Add `198.51.100.1/32` (matched by `POLICY-PREFIX-SET-A` via exact match) to
    `DEFAULT`. Verify **Collector 1** receives an update and **Collector 2**
    does not.

-   Add `100.64.1.128/25` (matched by `POLICY-PREFIX-SET-VRF-A` via prefix
    range) to `VRF-A`. Verify **Collector 2** receives an update and **Collector
    1** does not.

-   Change the filter for `VRF-A`:
    `/network-instances/network-instance[name=VRF-A]/afts/global-filter/config/ipv4-policy` =
    `POLICY-MATCH-ALL`
    `/network-instances/network-instance[name=VRF-A]/afts/global-filter/config/ipv6-policy` =
    `POLICY-MATCH-ALL`.

-   **Collector 1**: Verify its received AFT set for `DEFAULT` remains unchanged
    and the connection remains stable throughout.

-   **Collector 2**: Verify the stream is terminated. Upon resubscription,
    verify it receives all AFT entries from `VRF-A`: `198.51.100.0/24`,
    `100.64.1.0/24`, `203.0.113.128/28`, and the dynamically added
    `100.64.1.128/25`.

## AFT-6.1.7 - Simultaneous Independent IPv4 and IPv6 Policy Application

To validate that IPv4 and IPv6 filters can be applied independently and
concurrently using different routing policies.

### Setup

-   Use the routing policies defined in the setup: `POLICY-PREFIX-SET-A` (IPv4
    prefixes) and `POLICY-PREFIX-SET-B` (IPv6 prefixes).

-   Configure the global filter such that different address families use
    different policies:

    -   `/network-instances/network-instance[name=DEFAULT]/afts/global-filter/config/ipv4-policy` =
        `POLICY-PREFIX-SET-A`
    -   `/network-instances/network-instance[name=DEFAULT]/afts/global-filter/config/ipv6-policy` =
        `POLICY-PREFIX-SET-B`

### Validation

-   Establish a gNMI subscription to the AFT of the `DEFAULT` network instance
    as in **AFT-6.1.1**.

-   Wait for `SYNC`.

-   Verify that the received IPv4 entries match `POLICY-PREFIX-SET-A`
    (`198.51.100.0/24`, `203.0.113.0/28`, `198.51.100.1/32`).

-   Verify that the received IPv6 entries match `POLICY-PREFIX-SET-B`
    (`2001:DB8:2::/64`, `2001:DB8:2::1/128`).

-   Verify that prefixes NOT covered by their respective policies are barred
    from the stream (e.g., IPv4 prefix `100.64.0.0/24`).

-   Update the configuration to swap the policies:

    -   `ipv4-policy` = `POLICY-PREFIX-SET-B` (Matches nothing for IPv4)
    -   `ipv6-policy` = `POLICY-PREFIX-SET-A` (Matches nothing for IPv6)

-   Verify the stream is terminated. Upon resubscription, verify that **no**
    IPv4 or IPv6 entries from the prefix sets are received, as the cross-family
    matching results in an empty set.

## AFT-6.1.8 - Dynamic Prefix-Set Updates

To validate that the AFT filter dynamically responds to changes in the
underlying prefix sets without requiring a re-binding of the policy itself.

### Setup

-   Configure `POLICY-PREFIX-SET-A` which references `PREFIX-SET-A`.
-   Set the global filter `ipv4-policy` to `POLICY-PREFIX-SET-A`.
-   Ensure the DUT has static routes for `198.51.100.0/24` and `203.0.113.0/28`.
-   Establish a gNMI subscription and wait for `SYNC`.

### Validation

#### 6.1.8.1 - Addition of Prefix to Active Set

-   Add a new prefix `192.0.2.0/24` to `PREFIX-SET-A` on the DUT.
-   Ensure the DUT has a RIB/AFT entry for `192.0.2.0/24`.
-   Verify receipt of a gNMI update notification for `192.0.2.0/24`.

#### 6.1.8.2 - Deletion of Prefix from Active Set

-   Remove prefix `198.51.100.0/24` from `PREFIX-SET-A` on the DUT.
-   Verify receipt of a gNMI delete notification for `198.51.100.0/24`, even
    though the route still exists in the DUT's RIB/AFT.

#### 6.1.8.3 - Simultaneous Addition and Deletion

-   Perform an atomic gNMI update to `PREFIX-SET-A`:
    -   Add `198.51.100.0/24` back to the set.
    -   Remove `203.0.113.0/28` from the set.
-   Verify receipt of an update for `198.51.100.0/24` and a delete for
    `203.0.113.0/28`.

## OpenConfig Path and RPC Coverage

> **TODO:** The `global-filter` container and its `config/ipv4-policy`,
> `config/ipv6-policy`, `state/ipv4-policy` and `state/ipv6-policy` leaves are
> proposed extensions to the OpenConfig AFT model and are not yet present model
> and are not yet present in the master branch of
> [openconfig/public](https://github.com/openconfig/public). This README may be
> merged before the TODO is resolved. Link to the openconfig/public pull request
> here when available.

```yaml
paths:
  # Proposed paths for the new filter mechanism (not yet in openconfig/public)
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
  gnoi:
    system.System.Reboot: true
```

## Canonical OC

The following JSON shows the expected OpenConfig configuration for the routing
policies and static routes used in the test setup. The `global-filter`
configuration uses proposed OC paths and is excluded from this JSON (see TODO
above).

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
