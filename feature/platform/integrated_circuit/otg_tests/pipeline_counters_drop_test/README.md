# gNMI-1.29: Pipeline Counters Drops Test

## Summary
Verify that NPU (integrated-circuit) pipeline drop counters are reported correctly via telemetry and correctly increment under conditions that induce the respective drops.

## Testbed type
* [`//topologies/kne/dut_ate_2links.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/kne/dut_ate_2links.testbed)

## Procedure

### Test environment setup
* Configure interfaces between ATE and DUT.
* Configure IPv4/IPv6 addressing.
* Enable telemetry for the `/components/component/integrated-circuit/pipeline-counters/drop/vendor` path.

## Canonical OC
```json
{}
```

### gNMI-1.29.1 - Packet Processing: L3 Route Lookup Failed
* Verify via telemetry that `/components/component[name=<node>:<npu>]/integrated-circuit/pipeline-counters/drop/vendor/CiscoXR/spitfire/packet-processing/state/L3_ROUTE_LOOKUP_FAILED` exists  and returns a non-negative value.

### gNMI-1.29.2 - Packet Processing: L3 Null Adjacency
* Configure a static route on the DUT pointing to `Null0` (discard interface).
* From ATE Port-1, send traffic destined to the prefix configured in the Null0 route.
* Verify via telemetry that `/components/component[name=<node>:<npu>]/integrated-circuit/pipeline-counters/drop/vendor/CiscoXR/spitfire/packet-processing/state/L3_NULL_ADJ` increments.

### gNMI-1.29.3 - Packet Processing: MPLS Label Miss
* Verify via telemetry that `/components/component[name=<node>:<npu>]/integrated-circuit/pipeline-counters/drop/vendor/CiscoXR/spitfire/packet-processing/state/MPLS_TE_MIDPOINT_LDP_LABELS_MISS` exists and returns a non-negative value.

## OpenConfig Path and RPC Coverage
```yaml
paths:
  # TODO: Replace the following non-OC vendor paths with actual OC paths once they are available in the public models.
  # /components/component/integrated-circuit/pipeline-counters/drop/vendor/CiscoXR/spitfire/packet-processing/state/L3_ROUTE_LOOKUP_FAILED:
  # /components/component/integrated-circuit/pipeline-counters/drop/vendor/CiscoXR/spitfire/packet-processing/state/L3_NULL_ADJ*:
  # /components/component/integrated-circuit/pipeline-counters/drop/vendor/CiscoXR/spitfire/packet-processing/state/MPLS_TE_MIDPOINT_LDP_LABELS_MISS*:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Subscribe:
```

## Required DUT platform
* FFF