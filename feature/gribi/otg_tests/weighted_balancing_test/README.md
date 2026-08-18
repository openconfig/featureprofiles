# TE-3.2: Traffic Balancing According to Weights

## Canonical OC

```json
{}
```

## Summary

Ensure that traffic splits within a `NextHopGroup` are correctly honoured,
including seamless traffic redirection (soft drain) when next-hop weights are
dynamically updated to zero.

## Testbed type

* [`atedut_8.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_8.testbed)

## Procedure

### Test environment setup

*   Configure ATE port-1 connected to DUT port-1 (Ingress), and ATE ports 2-8 connected to
    DUT ports 2-8 (Egress). Connect to gRIBI with persistence `PRESERVE`, make it become
    leader and flush all entries before starting the tests.
*   Establish background traffic to a different `NextHopGroup` (e.g., NHG 30) to ensure
    modifications to the test NHGs do not cause cross-talk or flaps on unrelated routes.

### TE-3.2.1 - Base Traffic Balancing and Weight Ratios

*   **Step 1 - Push configuration to DUT**:
    *   Via gRIBI, install an `IPv4Entry` for 1000 routes (e.g., 203.0.0.0/16 block) pointing to a
        `NextHopGroup` id 10.
    *   Install next-hops corresponding to each of ATE port 2-8 from the DUT mapped
        to an IPv4 address, e.g., 192.0.2.2/30 corresponding to ATE port 2.
*   **Step 2 - Send Traffic**:
    *   Start generating continuous IPv4 traffic with sufficient entropy (mixed IPv4 source and
        destination ports) from ATE port-1 to the 1000 programmed routes.
*   **Step 3 - Validation with pass/fail criteria**:
    *   With NHG 10 containing 1 next hop, 100% of traffic is forwarded to the
        installed next-hop for all 1000 routes.
    *   With NHG 10 containing 2 next hops with no associated weights assigned,
        50% of traffic is forwarded to each next-hop.
    *   With NHG 10 containing 7 next hops, with no associated weights assigned,
        14.29% of traffic is forwarded to each next-hop.
    *   With NHG 10 containing 2 next-hops, specify and validate the following
        ratios:
        *   Weight 1:1 - 50% per-NH.
        *   Weight 2:1 - 66% traffic to NH1, 33% to NH2.
        *   Weight 9:1 - 90% traffic to NH1, 10% to NH2.
        *   Weight 31:1 - ~96.9% traffic to NH1, ~3.1% to NH2.
        *   Weight 63:1 - ~98.4% traffic to NH1, ~1.6% to NH2.
    *   Validate that weights of:
        *   <64K are supported
        *   >64K are correctly balanced if the device supports it.

### TE-3.2.2 - Sequential Port Shutdown

*   **Step 1 - Generate DUT configuration**: N/A (Builds on TE-3.2.1)
*   **Step 2 - Push configuration to DUT**: N/A
*   **Step 3 - Send Traffic**: (Continuous traffic from TE-3.2.1)
*   **Step 4 - Validation with pass/fail criteria**:
    *   With NHG 10 containing 7 next-hops, with a weight of 1 assigned to each,
        sequentially remove each next-hop by turning down the port at the ATE
        (invalidates nexthop).
    *   Ensure that traffic is rebalanced across remaining NHs until only one NH remains.
    *   Restore all ports before proceeding to the next test.

### TE-3.2.3 - IPv4 Soft Drain (Zero Weight)

*   **Step 1 - Push configuration to DUT**:
    *   Via gRIBI, install an `IPv4Entry` for a new block of 1000 routes (e.g., 198.51.0.0/16 block) pointing to a
        `NextHopGroup` ID 20.
    *   Install two next-hops in `NextHopGroup` ID 20: NH1 mapped to an IPv4
        address 192.0.2.2/30 (corresponding to ATE port-2) and NH2 mapped to
        192.0.2.6/30 (corresponding to ATE port-3), both initially assigned a weight of 1.
*   **Step 2 - Send Traffic**:
    *   Start generating continuous IPv4 traffic from ATE port-1 to the 1000 programmed routes on NHG 20.
*   **Step 3 - Validation with pass/fail criteria**:
    *   Verify that traffic is load-balanced between ATE port-2 and ATE port-3 evenly across all routes.
    *   Via gRIBI, update the `NextHopGroup` ID 20 to modify the weight of NH1 to 0.
    *   Verify that 100% of the traffic seamlessly shifts to NH2 (ATE port-3).
    *   Verify that the transition is lossless (0% packet loss during redirection, or within an acceptable hardware-specific threshold). Monitor continuous packet rate at the ATE to detect any micro-bursts or drops.
    *   Verify via gNMI telemetry that the operational weight of NH1 is updated to 0:
        *   `/network-instances/network-instance[name=DEFAULT]/afts/next-hop-groups/next-hop-group[id=20]/next-hops/next-hop[index=NH1]/state/weight` == 0
    *   Verify that the physical interface for NH1 remains up and active without any flaps:
        *   `/interfaces/interface[name=DUT_PORT_2]/state/oper-status` == `UP`
    *   Verify no disruption to the background traffic on NHG 30.

### TE-3.2.4 - IPv6 Soft Drain (Zero Weight)

*   **Step 1 - Push configuration to DUT**:
    *   Restore the weight of NH1 in `NextHopGroup` ID 20 back to 1.
    *   Via gRIBI, install an `IPv6Entry` for a block of 1000 routes (e.g., 2001:db8:1::/48 block)
        pointing to `NextHopGroup` ID 21.
    *   Install two next-hops in `NextHopGroup` ID 21: NH1 mapped to an IPv6
        address 2001:db8:2::2/126 (ATE port-2) and NH2 mapped to
        2001:db8:3::2/126 (ATE port-3), both initially assigned a weight of 1.
*   **Step 2 - Send Traffic**:
    *   Start generating continuous IPv6 traffic from ATE port-1 to the 1000 programmed IPv6 routes.
*   **Step 3 - Validation with pass/fail criteria**:
    *   Verify that IPv6 traffic is load-balanced between ATE port-2 and ATE port-3.
    *   Via gRIBI, update the `NextHopGroup` ID 21 to modify the weight of NH1 to 0.
    *   Verify that 100% of the traffic seamlessly shifts to NH2 (ATE port-3) with 0% packet loss.
    *   Verify via gNMI telemetry that the operational weight of NH1 is updated to 0.

### TE-3.2.5 - Zero-Weight NH Port Down (Negative Test)

*   **Step 1 - Generate DUT configuration**: N/A (Builds on TE-3.2.4)
*   **Step 2 - Push configuration to DUT**: N/A (Change physical state)
*   **Step 3 - Send Traffic**: (Continuous IPv6 traffic on NHG 21 with NH1 weight 0)
*   **Step 4 - Validation with pass/fail criteria**:
    *   Physically shut down the ATE port connected to NH1 (ATE port-2).
    *   Verify that this link-down event causes no disruption or traffic flap on NH2 (ATE port-3).
    *   Traffic should remain 100% on the active path.

### TE-3.2.6 - All Next-Hops Zero Weight (Negative Test)

*   **Step 1 - Push configuration to DUT**:
    *   Bring ATE port-2 back up.
    *   Restore NH1 weight to 1 for NHG 20 (IPv4) and NHG 21 (IPv6).
    *   Use gRIBI to update the `NextHopGroup` ID 20 such that both NH1 and NH2 are assigned a weight of 0.
*   **Step 2 - Send Traffic**: (Continuous IPv4 traffic to NHG 20)
*   **Step 3 - Validation with pass/fail criteria**:
    *   Verify the resulting behavior: either the gRIBI `Modify` RPC is rejected
        by the DUT (returning an error), OR the DUT accepts the modification and
        subsequently drops 100% of the traffic to the 1000 routes (since no active
        paths with non-zero weight remain).
    *   If dropped, verify ATE Rx traffic drops to exactly 0 bps while Tx continues, and verify DUT drop counters increase.
    *   If the RPC is accepted, verify via gNMI telemetry that the operational
        weights for both next-hops are updated to 0.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
  /interfaces/interface/state/oper-status:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/weight:
rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
  gribi:
    Modify:
      ModifyRequest:
        AFTOperation:
          next_hop_group:
            NextHopGroupKey: id
            NextHopGroup: next_hop
              NextHopKey: index
              NextHop: weight
```

## Required DUT platform

* FFF