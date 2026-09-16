# TE-14.6: gRIBI Scaling - all scenarios but with minimal scaling parameters

## Summary

Validate gRIBI scaling requirements using a minimized scale setup (Target Down/Minimized).

This test shares its topology, baseline configuration, procedures, test cases, and OpenConfig specifications with the other gRIBI full-scale targets. See the [Generic gRIBI Scaling README](../../README.md) for full details.

---

## Scaling Parameters

The test is configured with the following parameters defined in `gribi_full_scale_down_test.go`:

### gRIBI & System Configuration
* `GRIBIBatchSize`: `256`

### Default VRF
* `NumDefaultNH`: `2`
* `NumDefaultNHG`: `2`
* `NumDefaultIPv4`: `2`
* `DefaultNHGLoadBalance`: 100% of NHGs load-balance across 2 NHs
* `DefaultNHGWeight`: 100% ECMP (weights are equal to 1)

### Transit VRF
* `NumTransitNH`: `2`
* `NumTransitNHG`: `2`
* `TransitNHGLoadBalance`: 100% of NHGs pointing to 2 NHs
* `TransitNHGWeight`: 100% of NHGs have WCMP granularity `1/64` (1:63 weight ratio)
* `NumTransitIPv4`: `1`

### Repair VRF
* `NumRepairIPv4`: `1`
* `NumRepairNHG`: `1`

### Encap VRFs
* `NumEncapVRFs`: `1`
* `NumEncapIPv4PerVRF`: `1`
* `NumEncapIPv6PerVRF`: `1`
* `NumEncapNHPerVRF`: `1`
* `NumEncapNHGPerVRF`: `1`
* `EncapNHGLoadBalance`: 100% of NHGs pointing to 1 NH
* `EncapNHGWeight`: 100% WCMP granularity `1/32`

### Decap VRF
* `NumDecapEntries`: `1`
* `DecapDestsSubsetPct`: `100%`

### OTG / Port Configuration
* `NumPort2VLANs`: `2`

### Traffic Parameters
* `TrafficRateMpps`: `1,000` (1k pps)
* `TrafficDuration`: `1 minute`
* `TrafficLossTol`: `5`

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

