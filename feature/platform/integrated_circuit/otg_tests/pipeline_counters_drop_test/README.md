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
* All counters in this test are read by subscribing to
  `/components/component/integrated-circuit/pipeline-counters/drop/vendor`
  directly on the DUT. Platforms that only expose these counters on vendor
  native paths may use a functional translator (deviation
  `ciscoxr_vendordrop_ft`) to translate the native paths to the same OC path.
  When a functional translator is used:
    * The translator must support the DUT software version; otherwise the test
      fails.
    * Every native input path of the translator is subscribed to, and the test
      fails if any of them cannot be read or returns no data.
* Counters are parsed from
  `.../drop/vendor/<vendor>/<platform>/<category>/state/<counter>` as defined in
  the [vendor counter guide](https://github.com/openconfig/public/blob/master/doc/vendor_counter_guide.md).
  Each drop scenario is matched by the counter name produced by the functional
  translator or by the name streamed natively by IOS-XR 25.4.15 and later
  (case-insensitive name prefix):

  | Scenario | Functional translator counter (`CiscoXR/spitfire`) | Native IOS-XR 25.4.15+ counter (`...-ext:cisco/<asic>`) |
  | --- | --- | --- |
  | L3 route lookup failed | `packet-processing/L3_ROUTE_LOOKUP_FAILED` | `packet-processing/l3_drop_adj_non_inject` |
  | L3 null adjacency | `packet-processing/L3_NULL_ADJ(D*)` | `packet-processing/l3_null_adj` |
  | MPLS label miss | `packet-processing/MPLS_TE_MIDPOINT_LDP_LABELS_MISS(D*)` | `packet-processing/mpls_te_midpoint_ldp_labels_miss` |

* Each traffic scenario sends a fixed number of packets (5000) from ATE Port-1,
  waits until the ATE flow has stopped transmitting (and verifies all packets
  were transmitted) and until the DUT Port-1
  input counter reflects the packets, then reads the drop counters and verifies
  that the scenario counter, summed across all integrated circuits, increased
  by at least the number of packets sent.
  When the counters are streamed natively, the DUT may refresh them
  periodically, so the scenario counter is first watched with a SAMPLE
  subscription (10 second interval, up to 2 minutes) until it reflects the
  packets sent.

## Canonical OC
```json
{}
```

### gNMI-1.29.0 - Vendor Drop Path Is Streamed
* Subscribe (ONCE) to `/components/component[name=*]/integrated-circuit/pipeline-counters/drop/vendor`.
* Verify that the DUT streams at least one leaf under the path before the SyncResponse,
  and that at least one vendor drop counter is reported.

### gNMI-1.29.1 - Packet Processing: L3 Route Lookup Failed
* From ATE Port-1, send traffic destined to an address with no route on the DUT
  (`198.51.100.1`).
* Verify via telemetry that the L3 route lookup failed counter (see table above) increments by at least the number of packets sent.

### gNMI-1.29.2 - Packet Processing: L3 Null Adjacency
* Configure a static route on the DUT pointing to `Null0` (discard interface).
* From ATE Port-1, send traffic destined to the prefix configured in the Null0 route.
* Verify via telemetry that the L3 null adjacency counter (see table above) increments by at least the number of packets sent.

### gNMI-1.29.3 - Packet Processing: MPLS Label Miss
* Verify via telemetry that the MPLS label miss counter (see table above) exists and returns a non-negative value.
* TODO: Trigger the drop and verify the counter increments. This requires an
  MPLS-TE midpoint with LDP, which the `dut_ate_2links` topology does not
  configure.

### Cleanup
* Remove the `Null0` static route configured in gNMI-1.29.2.

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