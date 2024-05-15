# MPLS-1.1: MPLS label blocks

## Summary

Define reserved MPLS label blocks: static and MPLS-SR.

## Procedure

Topology: ATE1—DUT1

On DUT1 configure:

*   ISIS adjacency between ATE1 and DUT1
*   Enable MPLS-SR
*   Static Global Block (start: 1000000 range: 48576)
*   Segment Routing Global Block (start: 400000 range: 65001)
*   Segment Routing Local Block (start: 40000 range: 1000)

Verify:

*   Defined blocks are configured on DUT1.
*   DUT1 advertises its SR global block to ATE1.

## Config Parameter Coverage

* `/network-instances/network-instance/mpls/global/reserved-label-blocks/reserved-label-block/config`
* `/network-instances/network-instance/protocols/protocol/isis/global/segment-routing/config`
* `/network-instances/network-instance/segment-routing/srlbs/srlb/config`

## Telemetry Parameter Coverage

* `/network-instances/network-instance/mpls/global/reserved-label-blocks/reserved-label-block/state`