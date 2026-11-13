# RT-1.38: Routing policy configuration

## Summary

Create an IPv6 prefix-set and a routing policy that matches it and accepts the
route, then delete both. SpyTest creates a policy with `ACCEPT_ROUTE` and does
not match a prefix. Applying the policy to a neighbor and forwarding traffic
is covered by the BGP policy OTG tests.

## Testbed type

*   [`featureprofiles/topologies/dut.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut.testbed)

## Procedure

*   Replace a prefix-set named `FP_EFT1_RPL` with `2001:db8:e::/64` exact, mode
    IPV6.
*   Replace a policy of the same name. Statement 10 matches that prefix-set
    with option ANY and sets policy-result `ACCEPT_ROUTE`.
*   Read the policy name and the prefix from config, and the policy result
    from state.
*   Delete the policy and check it is gone.

#### Canonical OC

```json
{
  "routing-policy": {
    "defined-sets": {
      "prefix-sets": {
        "prefix-set": [
          {
            "name": "FP_EFT1_RPL",
            "config": {"name": "FP_EFT1_RPL", "mode": "IPV6"},
            "prefixes": {
              "prefix": [
                {"ip-prefix": "2001:db8:e::/64", "masklength-range": "exact", "config": {"ip-prefix": "2001:db8:e::/64", "masklength-range": "exact"}}
              ]
            }
          }
        ]
      }
    },
    "policy-definitions": {
      "policy-definition": [
        {
          "name": "FP_EFT1_RPL",
          "statements": {
            "statement": [
              {
                "name": "10",
                "actions": {"config": {"policy-result": "ACCEPT_ROUTE"}}
              }
            ]
          }
        }
      ]
    }
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /routing-policy/defined-sets/prefix-sets/prefix-set/config/name:
  /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix:
  /routing-policy/policy-definitions/policy-definition/config/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/state/policy-result:
rpcs:
  gnmi:
    gNMI.Set:
      replace: true
      delete: true
    gNMI.Subscribe:
```
