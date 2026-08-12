# TE-14.3: gRIBI Scaling - full scale setup, target T1

## Summary

Validate gRIBI scaling requirements for Target T1.

This test shares its topology, baseline configuration, procedures, test cases, and OpenConfig specifications with the other gRIBI full-scale targets. See the [Generic gRIBI Scaling README](../../README.md) for full details.

---

## Scaling Parameters

The test is configured with the following parameters defined in `gribi_full_scale_t1_test.go`:

### gRIBI & System Configuration
* `GRIBIBatchSize`: `2,000`

### Default VRF
* `NumDefaultNH`: `1,000`
* `NumDefaultNHG`: `1,000`
* `NumDefaultIPv4`: `1,000`
* `DefaultNHGLoadBalance`:
  * 40% (400) NHGs load-balance across 8 NHs
  * 40% (400) NHGs load-balance across 16 NHs
  * 15% (150) NHGs load-balance across 32 NHs
  * 5% (50) NHGs load-balance across 64 NHs
* `DefaultNHGWeight`:
  * 80% (800) NHGs have WCMP granularity `1/512`
  * 20% (200) NHGs have WCMP granularity `1/1024`

### Transit VRF
* `NumTransitNH`: `4,000`
* `NumTransitNHG`: `2,000`
* `TransitNHGLoadBalance`: 100% of NHGs pointing to 2 NHs
* `TransitNHGWeight`: 100% of NHGs have WCMP granularity `1/64` (1:63 weight ratio)
* `NumTransitIPv4`: `17,500`

### Repair VRF
* `NumRepairIPv4`: `17,500`
* `NumRepairNHG`: `1,000`

### Encap VRFs
* `NumEncapVRFs`: `5`
* `NumEncapIPv4PerVRF`: `4,140`
* `NumEncapIPv6PerVRF`: `5,060`
* `NumUniqueEncapNH`: `14,000`
* `NumEncapDefaultNHG`: `3,500`
* `EncapNHGLoadBalance`:
  * 75% NHGs load-balance across 4 NHs
  * 20% NHGs load-balance across 8 NHs
  * 3% NHGs load-balance across 16 NHs
  * 2% NHGs load-balance across 32 NHs
* `EncapNHGWeight`:
  * 75% NHGs have WCMP granularity `1/32`
  * 20% NHGs have WCMP granularity `1/64`
  * 3% NHGs have WCMP granularity `1/128`
  * 2% NHGs have WCMP granularity `1/256`

### Decap VRF
* `NumDecapEntries`: `20`
* `DecapDestsSubsetPct`: `100%`

### OTG / Port Configuration
* `NumPort2VLANs`: `640`

### Traffic Parameters
* `TrafficRateMpps`: `30,000,000` (30 Mpps)
* `TrafficDuration`: `5 minutes`
* `TrafficLossTol`: `5` (per-flow packet drop allowance)

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

