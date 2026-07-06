# TE-14.5: gRIBI Scaling - full scale setup, target T0

## Summary

Validate gRIBI current scaling requirements (Target T0).

## Differences from Target T1 (TE-14.3)

This test uses the same topology and structure as [`TE-14.3`](../gribi_full_scale_t1/README.md) (Target T1), but with reduced scale parameters:

| Parameter | Target T0 (TE-14.4) | Target T1 (TE-14.3) | Description |
| :--- | :--- | :--- | :--- |
| `NumRepairNHG` | 500 | 1,000 | Number of NextHop Groups in REPAIR_VRF |
| `NumEncapDefaultNHG` | 2,500 | 4,000 | NHGs in default VRF backing encap VRF entries |
| `NumUniqueEncapNH` | 10,000 | 16,000 | Total unique encapsulation NextHops |
| `NumTransitIPv4` | 12,600 | 200,000 | IPv4 prefixes in transit VRF `TE_VRF_111` |
| `NumRepairIPv4` | 12,600 | 200,000 | IPv4 prefixes in `REPAIR_VRF` |
| `NumEncapVRFs` | 5 | 16 | Number of encapsulation VRFs |


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
