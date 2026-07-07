# TE-14.6: gRIBI Scaling - all scenarios but with minimal scaling parameters

## Summary

Validate gRIBI scaling requirements using a minimized scale setup (Target T0/Minimized).

Useful for debugging and validating the correctness of the script

## Differences from Target T1 (TE-14.3)

This test uses the same topology and structure as [`TE-14.3`](../gribi_full_scale_t1/README.md) (Target T1), but with scale parameters reduced to minimum.


## Canonical OC
```json
{}
```

## OpenConfig Path and RPC Coverage
```yaml
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
  gribi:
    gRIBI.Get:
    gRIBI.Modify:
    gRIBI.Flush:
```
