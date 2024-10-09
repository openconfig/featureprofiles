# RT-2.15 IS-IS Extensions for Segment Routing

## Summary

* This test case provides comprehensive coverage of IS-IS extensions for Segment Routing (SR), including:
    * Node SID advertisement in IS-IS TLVs.
    * SRGB and SRLB configuration and validation.
    * Test coverage for Prefix SIDs and Anycast SIDs.

## Testbed type

*  [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

### Configuration

1) Create the topology below:

    ```
    ATE1—DUT1–ATE2
    ```

2) Enable SR and MPLS:
    * Enable Segment Routing for ISIS on the DUT.
    * Enable MPLS forwarding on all interfaces.
    * Configure appropriate IGP settings to ensure adjacency formation and prefix exchange between the DUT and ATEs.

### SR-1.2.1: SRGB and SRLB Configuration.

*   Configure a non-default SRGB on the DUT with a specific lower and upper bound (17000-20000).
*   Configure an SRLB on the DUT with a specific lower and upper bound (24000-27000).
*   Verify that the DUT allocates and advertises labels for prefixes from its configured SRGB range.
*   Verify that the DUT allocates and utilizes labels for adjacencies from its configured SRLB range.

### SR-1.2.2: Node SID Validation.

*   Configure the DUT to advertise its Node SID to ATE1 and ATE2.
*   Advertise prefixe (1) from ATE2 to the DUT
*   Send labeled traffic transiting through the DUT (using its node-SID) matching prefix (1).
*   Verify that the DUT advertises its Node SID in IS-IS TLVs.
*   Verify that ATE2 receives traffic with node-SID label popped.
*   Verify that traffic arrives to ATE Port 2.
*   Verify that corresponding SID forwarding counters are incremented.

### SR-1.2.3: Prefix SID Validation.

*   Configure the DUT to advertise two loopback prefixes with Prefix SIDs.
*   Verify that the DUT advertises the loopback prefixes with the correct Prefix SIDs.
*   Send labeled traffic from ATE1 to the loopback prefixes on the DUT
*   Verify correct forwarding using Prefix SIDs.
*   Verify that corresponding SID forwarding counters are incremented.

### SR-1.2.4: Anycast SID Validation.

*   Configure the DUT to advertise an Anycast SID representing a service reachable via both loopback interfaces.  
*   Verify that the DUT advertises the Anycast SID.
*   Send traffic from both ATEs towards the Anycast SID.
*   Verify that traffic is load-balanced between the DUT's loopback interfaces based on IGP metrics.
*   Verify that corresponding SID forwarding counters are incremented.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  ## Config paths
  /network-instances/network-instance/mpls/global/reserved-label-blocks/reserved-label-block/config/local-id:
  /network-instances/network-instance/mpls/global/reserved-label-blocks/reserved-label-block/config/lower-bound:
  /network-instances/network-instance/mpls/global/reserved-label-blocks/reserved-label-block/config/upper-bound:
  /network-instances/network-instance/mpls/global/interface-attributes/interface/config/mpls-enabled:
  /network-instances/network-instance/segment-routing/srgbs/srgb/config/local-id:
  /network-instances/network-instance/segment-routing/srgbs/srgb/config/mpls-label-blocks:
  /network-instances/network-instance/segment-routing/srlbs/srlb/config/mpls-label-block:
  /network-instances/network-instance/protocols/protocol/isis/global/segment-routing/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/global/segment-routing/config/srgb:
  /network-instances/network-instance/protocols/protocol/isis/global/segment-routing/config/srlb:

  ## Telemetry 
  /network-instances/network-instance/protocols/protocol/isis/global/segment-routing/state/enabled:
  /network-instances/network-instance/mpls/global/reserved-label-blocks/reserved-label-block/state/local-id:
  /network-instances/network-instance/mpls/global/reserved-label-blocks/reserved-label-block/state/lower-bound:
  /network-instances/network-instance/mpls/global/reserved-label-blocks/reserved-label-block/state/upper-bound:
  /network-instances/network-instance/mpls/signaling-protocols/segment-routing/aggregate-sid-counters/aggregate-sid-counter/state/in-pkts:
  /network-instances/network-instance/mpls/signaling-protocols/segment-routing/aggregate-sid-counters/aggregate-sid-counter/state/out-pkts:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
```

## Minimum DUT platform requirement
* FFF - fixed form factor