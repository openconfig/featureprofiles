# AFT-6.3: AFT Prefix Filtering Resilience

## Summary

This test validates AFT prefix filtering behavior under stress and recovery
conditions, including device reboot persistence, scale behavior with multiple
simultaneous subscriptions, and per-network-instance filter isolation with
multiple collectors.

See [AFT-6.1](../afts_prefix_filtering/README.md) for common test setup and
policy definitions.

## Testbed type

[atedut_2.testbed](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Test Setup

Use the test environment and routing policies described in
[AFT-6.1](../afts_prefix_filtering/README.md#test-setup).

## Procedure

### AFT-6.3.1 - Validation After Device Reboot

#### Establish Subscription

- Configure `POLICY-PREFIX-SET-A` and set the global filter `ipv4-policy` and
  `ipv6-policy` to it.

- Establish a successful gNMI subscription and verify the initial filtered set
  of AFT entries is received.

#### Reboot DUT

- Issue a reboot command to the DUT via `gNOI.System/Reboot` while the
  subscription is active.

- Verify the gNMI stream terminates.

#### Await DUT Readiness

- Wait for the DUT to become reachable and for gNMI to be available after
  reboot.

#### Re-establish Subscription

- Re-establish the same gNMI subscription.

- Verify the subscription is successfully established. Only
  endpoint-unreachable errors during the reboot window are acceptable; no
  internal errors should be returned.

- Verify that `global-filter/config/ipv4-policy` and
  `global-filter/config/ipv6-policy` still hold `POLICY-PREFIX-SET-A`,
  confirming the configuration persisted across reboot.

- Verify that the DUT streams the correct filtered set of AFT entries matching
  `POLICY-PREFIX-SET-A`.

### AFT-6.3.2 - Scale Test

- Let `X` be the number of IPv4 routes to advertise from the ATE. **(User
  Adjustable Value, default: 5000)**

- Let `Y` be the number of IPv6 routes to advertise from the ATE. **(User
  Adjustable Value, default: 2000)**

- Let `K` be the maximum allowed synchronization time in seconds. **(User
  Adjustable Value, default: 120)**

- Populate the AFT with `X` IPv4 routes and `Y` IPv6 routes by advertising
  them from the connected ATE.

- Configure three routing policies matching approximately 1%, 5%, and 20% of
  the total route set for the address family under test, respectively.

- Establish two simultaneous gNMI STREAM subscriptions (ON_CHANGE) to the AFT.
  For each policy scenario and address family, wait until both subscriptions
  receive `SYNC` and all expected leaves.

- Measure the time taken for initial synchronization for each scenario.

- Verify that in all cases, synchronization completes within `K` seconds.

- Verify correct filtering is applied in all scenarios (only matching prefixes
  for the active address family are streamed).

### AFT-6.3.3 - Per Network-Instance Filtering with Multiple Collectors

To validate that AFT filters are applied independently per network instance and
that multiple collectors can subscribe to different network instances with their
respective filters.

#### Setup

- Configure two network instances on the DUT: `DEFAULT` and `VRF-A`.

- Populate both instances with distinct static routes. The prefix
  `198.51.100.0/24` appears in both instances to verify filter independence.

  - `DEFAULT`: `198.51.100.0/24`, `203.0.113.0/28`, `100.64.0.0/24`
  - `VRF-A`: `198.51.100.0/24`, `100.64.1.0/24`, `203.0.113.128/28`

- Configure the following routing policies:

  - `POLICY-PREFIX-SET-A`: Matches `198.51.100.0/24`, `203.0.113.0/28`, and
    `198.51.100.1/32`.
  - `POLICY-PREFIX-SET-VRF-A`: Matches `100.64.1.0/24` and subnets up to
    `/32`.
  - `POLICY-MATCH-ALL`: Matches all routes.

- Configure AFT filters:

  - `DEFAULT`:
    `/network-instances/network-instance[name=DEFAULT]/afts/global-filter/config/ipv4-policy` =
    `POLICY-PREFIX-SET-A`
    `/network-instances/network-instance[name=DEFAULT]/afts/global-filter/config/ipv6-policy` =
    `POLICY-PREFIX-SET-A`
  - `VRF-A`:
    `/network-instances/network-instance[name=VRF-A]/afts/global-filter/config/ipv4-policy` =
    `POLICY-PREFIX-SET-VRF-A`
    `/network-instances/network-instance[name=VRF-A]/afts/global-filter/config/ipv6-policy` =
    `POLICY-PREFIX-SET-VRF-A`

#### Validation

- **Collector 1**: Establishes a gNMI subscription to AFT paths within the
  `DEFAULT` network instance.

- **Collector 2**: Establishes a gNMI subscription to AFT paths within the
  `VRF-A` network instance.

- Collector 1: Verify `SYNC` and receipt of only `198.51.100.0/24` and
  `203.0.113.0/28` from `DEFAULT`. Verify all expected next-hop-groups and
  next-hops are received normally. Verify `100.64.0.0/24` is **not** received.

- Collector 2: Verify `SYNC` and receipt of only `100.64.1.0/24` from `VRF-A`.
  Verify all expected next-hop-groups and next-hops are received normally.
  Verify `198.51.100.0/24` and `203.0.113.128/28` are **not** received.

- Add `100.64.2.0/24` to `DEFAULT`. Verify **neither** collector receives an
  update.

- Add `203.0.113.64/28` to `DEFAULT`. Verify **neither** collector receives an
  update (not matched by either collector's active policy).

- Add `198.51.100.1/32` (matched by `POLICY-PREFIX-SET-A` via exact match) to
  `DEFAULT`. Verify **Collector 1** receives an update and **Collector 2**
  does not.

- Add `100.64.1.128/25` (matched by `POLICY-PREFIX-SET-VRF-A` via prefix
  range) to `VRF-A`. Verify **Collector 2** receives an update and **Collector
  1** does not.

- Change the filter for `VRF-A`:
  `/network-instances/network-instance[name=VRF-A]/afts/global-filter/config/ipv4-policy` =
  `POLICY-MATCH-ALL`
  `/network-instances/network-instance[name=VRF-A]/afts/global-filter/config/ipv6-policy` =
  `POLICY-MATCH-ALL`.

- **Collector 1**: Verify its received AFT set for `DEFAULT` remains unchanged
  and the connection remains stable throughout.

- **Collector 2**: After the policy change to `POLICY-MATCH-ALL`, verify
  within 60 seconds that Collector 2 receives update notifications for the
  previously filtered-out prefixes (`198.51.100.0/24` and
  `203.0.113.128/28`) that are now included under the new policy. If the DUT
  terminates Collector 2's stream due to the policy change, re-establish the
  subscription and verify the full set of `VRF-A` prefixes
  (`198.51.100.0/24`, `100.64.1.0/24`, `203.0.113.128/28`, and the
  dynamically added `100.64.1.128/25`) is received after `SYNC`.

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
  gnoi:
    system.System.Reboot: true
```

## Canonical OC

See [AFT-6.1](../afts_prefix_filtering/README.md#canonical-oc) for the full
canonical OpenConfig configuration. This test uses `POLICY-PREFIX-SET-A`,
`POLICY-PREFIX-SET-VRF-A`, and `POLICY-MATCH-ALL` as defined there.

```json
{
  "routing-policy": {
    "defined-sets": {
      "prefix-sets": {
        "prefix-set": [
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
        }
      ]
    }
  }
}
```

## Required DUT platform

FFF (Fixed Form Factor) or MFF (Modular Form Factor).
