# AFT-6.4: AFT Prefix Filtering Dynamic Updates

## Summary

This test validates that the AFT filter dynamically responds to changes in the
underlying prefix sets without requiring a re-binding of the policy itself. Both
IPv4 and IPv6 prefix-set modifications are covered.

See [AFT-6.1](../afts_prefix_filtering/README.md) for common test setup and
policy definitions.

## Testbed type

[atedut_2.testbed](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Test Setup

Use the test environment and routing policies described in
[AFT-6.1](../afts_prefix_filtering/README.md#test-setup).

### IPv4 Setup

- Configure `POLICY-PREFIX-SET-A` which references `PREFIX-SET-A`.
- Set the global filter `ipv4-policy` to `POLICY-PREFIX-SET-A`.
- Ensure the DUT has static routes for `198.51.100.0/24` and `203.0.113.0/28`.
- Establish a gNMI subscription and wait for `SYNC`.

### IPv6 Setup

- Configure `POLICY-PREFIX-SET-B` which references `PREFIX-SET-B`.
- Set the global filter `ipv6-policy` to `POLICY-PREFIX-SET-B`.
- Ensure the DUT has static routes for `2001:DB8:2::/64` and
  `2001:DB8:2::1/128`.
- Establish a gNMI subscription and wait for `SYNC`.

## Procedure

### AFT-6.4.1 - Dynamic Prefix-Set Updates (IPv4)

#### 6.4.1.1 - Addition of Prefix to Active Set

- Add a new prefix `192.0.2.0/24` to `PREFIX-SET-A` on the DUT.
- Ensure the DUT has a RIB/AFT entry for `192.0.2.0/24`.
- Verify receipt of a gNMI update notification for `192.0.2.0/24`.

#### 6.4.1.2 - Deletion of Prefix from Active Set

- Remove prefix `198.51.100.0/24` from `PREFIX-SET-A` on the DUT.
- Verify receipt of a gNMI delete notification for `198.51.100.0/24`, even
  though the route still exists in the DUT's RIB/AFT.

#### 6.4.1.3 - Simultaneous Addition and Deletion

- Perform an atomic gNMI update to `PREFIX-SET-A`:
  - Add `198.51.100.0/24` back to the set.
  - Remove `203.0.113.0/28` from the set.
- Verify receipt of an update for `198.51.100.0/24` and a delete for
  `203.0.113.0/28`.

### AFT-6.4.2 - Dynamic Prefix-Set Updates (IPv6)

#### 6.4.2.1 - Addition of IPv6 Prefix to Active Set

- Add a new prefix `2001:DB8:2::2/128` to `PREFIX-SET-B` on the DUT.
- Ensure the DUT has a RIB/AFT entry for `2001:DB8:2::2/128`.
- Verify receipt of a gNMI update notification for `2001:DB8:2::2/128`.

#### 6.4.2.2 - Deletion of IPv6 Prefix from Active Set

- Remove prefix `2001:DB8:2::/64` from `PREFIX-SET-B` on the DUT.
- Verify receipt of a gNMI delete notification for `2001:DB8:2::/64`, even
  though the route still exists in the DUT's RIB/AFT.

#### 6.4.2.3 - Simultaneous IPv6 Addition and Deletion

- Perform an atomic gNMI update to `PREFIX-SET-B`:
  - Add `2001:DB8:2::/64` back to the set.
  - Remove `2001:DB8:2::1/128` from the set.
- Verify receipt of an update for `2001:DB8:2::/64` and a delete for
  `2001:DB8:2::1/128`.

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

  # Paths for configuring prefix-sets
  /routing-policy/defined-sets/prefix-sets/prefix-set/config/name:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range:

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

See [AFT-6.1](../afts_prefix_filtering/README.md#canonical-oc) for the full
canonical OpenConfig configuration. This test uses `PREFIX-SET-A`,
`PREFIX-SET-B`, and their associated policies as defined there.

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
