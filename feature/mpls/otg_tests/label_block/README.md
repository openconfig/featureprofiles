# MPLS-1.1: MPLS label blocks using ISIS

## Summary

Define reserved MPLS label blocks: static and MPLS-SR.

## Testbed type

*  [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

Topology: ATE1—DUT1

On DUT1 configure:

*   ISIS adjacency between ATE1 and DUT1
*   Enable MPLS-SR for ISIS (`/network-instances/network-instance/protocols/protocol/isis/global/segment-routing/config/enabled`)
*   reserved-label-block (lower-bound: 1000000 upper-bound: 1048576)
*   Segment Routing Global Block (srgb)  with lower-bound: 400000 upper-bound: 465001
*   Segment Routing Local Block (srlb) with lower-bound: 40000 upper-bound: 41000)

Verify:

*   Defined blocks are configured on DUT1.
*   DUT1 advertises its SRGB and SRLB to ATE1.


## OpenConfig Path and RPC Coverage

```yaml
paths:
  # configuration
  /network-instances/network-instance/mpls/global/interface-attributes/interface/config/mpls-enabled:
  /network-instances/network-instance/mpls/global/reserved-label-blocks/reserved-label-block/config/local-id:
  /network-instances/network-instance/mpls/global/reserved-label-blocks/reserved-label-block/config/lower-bound:
  /network-instances/network-instance/mpls/global/reserved-label-blocks/reserved-label-block/config/upper-bound:
  /network-instances/network-instance/segment-routing/srgbs/srgb/config/local-id:
  /network-instances/network-instance/segment-routing/srgbs/srgb/config/mpls-label-blocks:
  /network-instances/network-instance/segment-routing/srlbs/srlb/local-id:
  /network-instances/network-instance/segment-routing/srlbs/srlb/config/mpls-label-block:
  /network-instances/network-instance/protocols/protocol/isis/global/segment-routing/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/global/segment-routing/config/srgb:
  /network-instances/network-instance/protocols/protocol/isis/global/segment-routing/config/srlb:
  # telemetry
  /network-instances/network-instance/mpls/global/reserved-label-blocks/reserved-label-block/state/local-id:
  /network-instances/network-instance/mpls/global/reserved-label-blocks/reserved-label-block/state/lower-bound:
  /network-instances/network-instance/mpls/global/reserved-label-blocks/reserved-label-block/state/upper-bound:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

* FFF
