# System-1.7: Hash seed configuration

## Summary

Replace the global hashing-policy seed and read it back. The generated
OpenConfig client in this module does not include openconfig-hashing, so the
test uses a JSON-IETF gNMI leaf. SpyTest writes the seed and restores it.
This test also covers the case where the seed was unset.

## Testbed type

*   [`featureprofiles/topologies/dut.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut.testbed)

## Procedure

*   Read `/system/hashing/hashing-policies/hashing-policy[name=GLOBAL]/config/seed`.
*   Replace it with 1, or with 2 when the current value is already 1.
*   Read the seed back.
*   Restore the original value, or delete the leaf when it was unset.

#### Canonical OC

```json
{
  "system": {
    "hashing": {
      "hashing-policies": {
        "hashing-policy": [
          {
            "name": "GLOBAL",
            "config": {
              "name": "GLOBAL",
              "seed": 1
            }
          }
        ]
      }
    }
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /system/hashing/hashing-policies/hashing-policy/config/seed:
  /system/hashing/hashing-policies/hashing-policy/state/seed:
rpcs:
  gnmi:
    gNMI.Set:
      replace: true
      delete: true
    gNMI.Get:
```
