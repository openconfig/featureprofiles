# LB-1.1: Load Balancing Hashing Polarization Detection

## Summary

Validate that the DUT's hashing implementation does not produce
polarization when traffic traverses multiple levels of weighted
forwarding paths. The test sends traffic through a two-level recursive
gRIBI hierarchy, captures packets on a target port, then replays only
those captured flows after perturbing the hash configuration. If the
hash is non-linear, the replayed flows scatter across all ports; if
linear, they remain correlated and the test fails.

## Topology

```
                  +-------+
   ATE:port1 --->| port1 |
                 |       |
                 |  DUT  |---> port2 \
                 |       |---> port3 / LAG 1
                 |       |
                 |       |---> port4 \
                 +-------+---> port5 / LAG 2
```

*   **Ingress**: ATE port1 -> DUT port1 (L3 interface)
*   **LAG 1** (ports 2, 3): target capture port group
*   **LAG 2** (ports 4, 5): secondary path group

## Procedure

### Setup

1.  Configure DUT port1 as a routed L3 interface.
2.  Configure two static LAGs on the DUT:
    *   LAG 1 with ports 2 and 3.
    *   LAG 2 with ports 4 and 5.
3.  Program gRIBI entries to create a two-level recursive weighted
    forwarding hierarchy for `198.51.100.0/24`:

    ```
    198.51.100.0/24 --> NHG 101 (weight 8:1)
    |
    +--[8/9]--> NH 1011 --> 203.0.113.1 --> NHG 2010 (weight 8:1)
    |            +--[8/9]--> NH 1501 --> LAG1 (port2, port3)
    |            +--[1/9]--> NH 1601 --> LAG2 (port4, port5)
    |
    +--[1/9]--> NH 1012 --> 203.0.113.2 --> NHG 3000 (weight 1:1)
                 +--[1/2]--> NH 1602 --> LAG2 (port4, port5)
                 +--[1/2]--> NH 1603 --> LAG2 (port4, port5)
    ```

    All NHG weights are defined as named constants in the test for easy
    tuning. The expected per-port distribution is computed automatically
    from these weights. With the default weights, port2 receives ~39.5%
    of traffic.

4.  Configure ATE ports and protocols.
5.  Generate a large set of unique IPv4/UDP flow tuples (varying source IP and
    UDP ports, fixed destination IP within `198.51.100.0/24`). All addresses are
    confined to reserved ranges only — RFC 5737 documentation blocks
    (`192.0.2.0/24`, `198.51.100.0/24`, `203.0.113.0/24`) and the RFC 2544
    benchmarking block (`198.18.0.0/15`) — so no test traffic can leak onto or
    spoof a real public network.

### Test: Iterative Replay

Run a **Baseline** round followed by a set of **Replay** rounds. Each
round:

1.  **Perturb hash**: Apply a vendor-specific configuration change to
    alter how the DUT makes path selections.

    | Vendor  | Mechanism                                      |
    |---------|-------------------------------------------------|
    | Cisco   | Loopback0 IP address change                    |

    Other vendors should add their implementation to `perturbHashConfig`
    in the test file.

2.  **Send traffic in batches**: Inject the current flow list through
    ATE port1 in a configurable batch count of packets. Each batch: push config,
    start capture on port2, send traffic, stop and download capture,
    accumulate captured packets and per-port Rx counters. Each batch also
    asserts the flow is delivered without loss (Rx ≈ Tx).
3.  **Assert**: Verify the DUT forwarded the traffic without loss — the frames
    received across ports2-5 must account for nearly all packets sent this round
    — then verify that port2 received the expected share of total packets
    (derived from NHG weights) within a configurable 2% tolerance. The loss
    check runs first so that drops on ports3-5 cannot be masked by the
    port2-only distribution check.
4.  **Feed back**: The packets captured on port2 across all batches
    become the sole input for the next round. The packet count shrinks
    each round (~39.5% retained per round with default weights).

### Why Replay Detects Polarization

After the Baseline, we know exactly which flows hashed to port2. If we
replay only those flows after perturbing the hash and the hash is
non-linear, the flows scatter across all ports — with the default 8:1
weights, only ~39.5% should land on port2 again. If the hash is
linear, perturbing it shifts all flows by the same constant; flows
that grouped together stay grouped, and nearly 100% of the replayed
flows land on port2 again.

### Why Batch Packets

The Ixia hardware capture buffer holds a limited number of packets per port. Sending
more packets per batch than this the limit (different in different hardware) causes captured packets to be
overwritten. The configurable batch size ensures every packet arriving on
port2 is captured and available for replay in the next round. 

### Pass/Fail Criteria

*   **Pass**: In every round, the DUT forwards all injected traffic without
    loss (frames received across ports2-5 ≈ packets sent) and port2 receives the
    expected share of injected packets (within 2% of total).
*   **Fail**: The DUT drops forwarded traffic on any of ports2-5, or port2
    receives significantly more or less than expected, indicating flows remained
    correlated (polarized) despite the hash perturbation.

## Config Parameter Coverage

N/A (gRIBI-programmed forwarding; no OC config paths for hash seeding)

## Telemetry Parameter Coverage

N/A

## Protocol/RPC Parameter Coverage

*   gRIBI
    *   Modify: NextHopEntry, NextHopGroupEntry (with weights),
        IPv4Entry
    *   Flush

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /interfaces/interface/config/name:
  /interfaces/interface/config/type:
  /interfaces/interface/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/config/link-layer-address:
  /interfaces/interface/ethernet/config/aggregate-id:
  /interfaces/interface/aggregation/config/lag-type:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
  gribi:
    gRIBI.Modify:
    gRIBI.Flush:
```

## TODO

*   Add IP-in-IP encap flow variant
*   Add IP-in-IP decap flow variant
*   Add IP-in-IP transit flow variant

## Minimum DUT Platform Requirement

N/A
