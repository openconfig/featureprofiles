# TE-14.4: gRIBI Scaling - full scale setup, target T2

## Summary

Validate gRIBI scaling requirements (Target T2).

## Topology & Baseline

Use the same topology as [`TE-14.3`](../gribi_full_scale_t1/README.md) but adjust the scale:

- T1)
    - 70% (700) NHGs should have granularity 1/512
    - 30% (300) NHGs should have granularity 1/1K

- T2) X = 2K
- T3) 8K
- T4) 32K


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
