# AFT-6.2: AFT Prefix Filtering Dual-Stack

## Summary

This test validates that IPv4 and IPv6 AFT filters can be applied independently
and concurrently using different routing policies on the same network instance.

See [AFT-6.1](../afts_prefix_filtering/README.md) for common test setup and
policy definitions.

## Testbed type

[atedut_2.testbed](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Test Setup

Use the test environment and routing policies described in
[AFT-6.1](../afts_prefix_filtering/README.md#test-setup). In particular, ensure
the following policies and prefixes are configured:

- `POLICY-PREFIX-SET-A`: Matches IPv4 prefixes `198.51.100.0/24`,
  `203.0.113.0/28`, and `198.51.100.1/32`.

- `POLICY-PREFIX-SET-B`: Matches IPv6 prefixes `2001:DB8:2::/64` and
  `2001:DB8:2::1/128`.

- The DUT's AFT contains entries for both matching and non-matching prefixes in
  each address family (e.g., `100.64.0.0/24` for IPv4, `2001:DB8:1::/64` for
  IPv6).

## Procedure

### AFT-6.2.1 - Simultaneous Independent IPv4 and IPv6 Policy Application

#### Setup

- Configure the global filter such that different address families use
  different policies:

  - `/network-instances/network-instance[name=DEFAULT]/afts/global-filter/config/ipv4-policy` =
    `POLICY-PREFIX-SET-A`
  - `/network-instances/network-instance[name=DEFAULT]/afts/global-filter/config/ipv6-policy` =
    `POLICY-PREFIX-SET-B`

#### Validation

- Establish a gNMI STREAM subscription (ON_CHANGE) to the AFT of the `DEFAULT`
  network instance as described in
  [AFT-6.1.1](../afts_prefix_filtering/README.md#aft-611---validation-of-subscription-with-prefix-set-policy).

- Wait for `SYNC`.

- Verify that the received IPv4 entries match `POLICY-PREFIX-SET-A`
  (`198.51.100.0/24`, `203.0.113.0/28`, `198.51.100.1/32`).

- Verify that the received IPv6 entries match `POLICY-PREFIX-SET-B`
  (`2001:DB8:2::/64`, `2001:DB8:2::1/128`).

- Verify that prefixes NOT covered by their respective policies are barred
  from the stream (e.g., IPv4 prefix `100.64.0.0/24`).

- Update the configuration to swap the policies:

  - `ipv4-policy` = `POLICY-PREFIX-SET-B` (Matches nothing for IPv4)
  - `ipv6-policy` = `POLICY-PREFIX-SET-A` (Matches nothing for IPv6)

- Verify that deletes are received, deleting all prefixes, as the cross-family
  matching results in an empty set.

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
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address:

rpcs:
  gnmi:
    gNMI.Subscribe:
      STREAM: true
      ON_CHANGE: true
    gNMI.Set:
      REPLACE: true
      UPDATE: true
```

## Canonical OC

See [AFT-6.1](../afts_prefix_filtering/README.md#canonical-oc) for the full
canonical OpenConfig configuration. This test uses `POLICY-PREFIX-SET-A` and
`POLICY-PREFIX-SET-B` as defined there.

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
          }
        ]
      }
    },
    "policy-definitions": {
      "policy-definition": [
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
        }
      ]
    }
  }
}
```

## Required DUT platform

FFF (Fixed Form Factor) or MFF (Modular Form Factor).
