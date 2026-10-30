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
* `NumDefaultNH`: `640`
* `NumDefaultNHG`: `512`
* `NumDefaultIPv4`: `1,000`
* `DefaultNHGLoadBalance`:
  * 40% NHGs load-balance across 8 NHs
  * 40% NHGs load-balance across 16 NHs
  * 15% NHGs load-balance across 32 NHs
  * 5% NHGs load-balance across 64 NHs
* `DefaultNHGWeight`:
  * 40% NHGs have WCMP granularity `1/32`
  * 40% NHGs have WCMP granularity `1/64`
  * 20% NHGs have WCMP granularity `1/256`

### Transit VRF
* `NumTransitNH`: `4,000`
* `NumTransitNHG`: `512`
* `TransitNHGLoadBalance`: 100% of NHGs pointing to 2 NHs
* `TransitNHGWeight`: 100% of NHGs have WCMP granularity `1/64` (1:63 weight ratio)
* `NumTransitIPv4`: `17,500`

### Repair VRF
* `NumRepairIPv4`: `17,500`
* `NumRepairNHG`: `1,000`

### Encap VRFs
* `NumEncapVRFs`: `7`
* `NumEncapIPv4PerVRF`: `5,785`
* `NumEncapIPv6PerVRF`: `7,072`
* `NumEncapNHPerVRF`: `2,800`
* `NumEncapNHGPerVRF`: `500`
* `EncapNHGLoadBalance`:
  * 75% NHGs load-balance across 4 NHs
  * 20% NHGs load-balance across 8 NHs
  * 3% NHGs load-balance across 16 NHs
  * 2% NHGs load-balance across 32 NHs
* `EncapNHGWeight`:
  * 75% NHGs have WCMP granularity `1/16`
  * 20% NHGs have WCMP granularity `1/32`
  * 5% NHGs have WCMP granularity `1/64`

### Decap VRF
* `NumDecapEntries`: `50` for all vendors and `20` for Juniper
* `DecapDestsSubsetPct`: `100%`

### OTG / Port Configuration
* `NumPort2VLANs`: `640`

### Traffic Parameters
* `TrafficRateMpps`: `20,000,000` (20 Mpps)
* `TrafficDuration`: `5 minutes`
* `TrafficLossTol`: `0`

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

