# gNMI-1.24: gNMI Leaf-List Update Test

## Description

This test validates the behavior of gNMI `Replace` and `Update` operations on a leaf-list field in the OpenConfig model. The test specifically targets the `/system/dns/config/search` path.

The test follows these steps:

1.  **Initial Replace**: It uses a gNMI `Replace` operation to set the value of `/system/dns/config/search` to `["google.com"]`.
2.  **Validation**: It verifies that the DUT successfully updated the path and that the new value is `["google.com"]`.
3.  **Update**: It then uses a gNMI `Update` operation to add `"youtube.com"` to the existing leaf-list.
4.  **Final Validation**: It verifies that the leaf-list at `/system/dns/state/search` now contains both `"google.com"` and `"youtube.com"`.

The test will fail if the values at any stage do not match the expected results.

## Canonical OC

```json
{
  "system": {
    "dns": {
      "config": {
        "search": [
          "google.com",
          "youtube.com"
        ]
      }
    }
  }
}
```

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
  /system/dns/config/search:
  /system/dns/state/search:
rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```
