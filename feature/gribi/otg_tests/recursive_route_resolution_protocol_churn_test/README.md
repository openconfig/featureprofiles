# TE-1.4: gRIBI Recursive Route Resolution under Routing Protocol Churn

## Summary

Verify that gRIBI routes resolving recursively via BGP correctly update their forwarding state when the underlying BGP routes flap (route withdrawal and re-advertisement) without session loss.

## Testbed type

* `TESTBED_DUT_ATE_4LINKS`

## Procedure

### Test environment setup

* Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2, ATE port-3 to DUT port-3, and ATE port-4 to DUT port-4.
* Configure IPv4 addressing on all connected interfaces:
  * DUT port-1: `198.51.100.1/30`, ATE port-1: `198.51.100.2/30` (Traffic Ingress Link, `198.51.100.0/30`)
  * DUT port-2: `198.51.100.5/30`, ATE port-2: `198.51.100.6/30` (Path-1 Egress Link, `198.51.100.4/30`)
  * DUT port-3: `198.51.100.9/30`, ATE port-3: `198.51.100.10/30` (Path-2 Egress Link, `198.51.100.8/30`)
  * DUT port-4: `203.0.113.1/30`, ATE port-4: `203.0.113.2/30` (Negative Control Egress Link, `203.0.113.0/30`)
* Configure IPv4 Loopback addressing:
  * DUT `Loopback0`: `192.0.2.1/32`
  * ATE port-2 simulated Loopback: `192.0.2.2/32`
  * ATE port-3 simulated Loopback: `192.0.2.3/32`
* Establish an IS-IS routing domain (`DEFAULT` instance, `LEVEL_2`, `WIDE_METRIC`) between the DUT and ATE:
  * Configure Area ID `49.0001` with System IDs: DUT `1920.0000.2001` (NET `49.0001.1920.0000.2001.00`), ATE port-2 `1920.0000.2002` (NET `49.0001.1920.0000.2002.00`), and ATE port-3 `1920.0000.2003` (NET `49.0001.1920.0000.2003.00`).
  * Enable IS-IS (`POINT_TO_POINT`, Level 2 metric `10`, `IPV4_UNICAST`) on the connected links for `port-2` and `port-3`.
  * Inject the loopback interfaces (`192.0.2.1/32` as passive on DUT `Loopback0`, `192.0.2.2/32` on ATE port-2, and `192.0.2.3/32` on ATE port-3) into IS-IS.
* Establish iBGP (`IPV4_UNICAST`) sessions in `AS 64500` between the Loopback interfaces:
  * Configure DUT local AS `64500` and `local-address` (update-source) `192.0.2.1` (`Loopback0`), with BGP timers `hold-time: 9` seconds and `keepalive-interval: 3` seconds.
  * Establish an iBGP session between DUT Loopback `192.0.2.1` (`AS 64500`) and ATE port-2 Loopback `192.0.2.2` (`AS 64500`).
  * Establish an iBGP session between DUT Loopback `192.0.2.1` (`AS 64500`) and ATE port-3 Loopback `192.0.2.3` (`AS 64500`).
* Establish a gRIBI client connection with the DUT in the `DEFAULT` network instance (`persistence: PRESERVE`, `redundancy: SINGLE_PRIMARY`, `election_id: {low: 1, high: 0}`), ensure it becomes the leader, and execute `gRIBI.Flush` on `DEFAULT` to ensure a clean initial AFT state.

### TE-1.4.1 - Base gRIBI Recursive Route Resolution

* Step 1 - Validate IS-IS Underlay and BGP Peering.
  * Validate that the IS-IS adjacencies on DUT `port-2` and `port-3` reach `UP` state via gNMI (`.../isis/interfaces/interface/levels/level/adjacencies/adjacency/state/adjacency-state`).
  * Validate that the ATE loopbacks (`192.0.2.2/32` and `192.0.2.3/32`) are learned via IS-IS and installed in the DUT's RIB/FIB using the gNMI AFT path:
    `/network-instances/network-instance[name=DEFAULT]/afts/ipv4-unicast/ipv4-entry[prefix=192.0.2.2/32]/state/prefix`
    `/network-instances/network-instance[name=DEFAULT]/afts/ipv4-unicast/ipv4-entry[prefix=192.0.2.3/32]/state/prefix`
  * Validate that both iBGP sessions (`192.0.2.2` and `192.0.2.3`) reach `ESTABLISHED` state over the IS-IS underlay.
* Step 2 - Advertise BGP routes from ATE port-2.
  * ATE port-2 advertises 1,000 `/24` BGP IPv4 prefixes (`100.64.0.0/24` through `100.67.231.0/24`, i.e., `100.<64 + floor(i/256)>.<i mod 256>.0/24` for $i \in [0, 999]$) with BGP next-hop `192.0.2.2` (ATE port-2 Loopback).
  * Validate DUT receives and installs the BGP routes in the Loc-RIB by checking the gNMI path for a sample prefix (e.g., `100.64.0.0/24`):
    `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=BGP][name=BGP]/bgp/rib/afi-safis/afi-safi[afi-safi-name=IPV4_UNICAST]/ipv4-unicast/loc-rib/routes/route[prefix=100.64.0.0/24]/state/prefix`
* Step 3 - Install gRIBI recursive routes.
  * Use `gRIBI.Modify` RPC (requesting `FIB_PROGRAMMED` ACK) to install 1,000 IPv4 routes (`100.68.0.0/24` through `100.71.231.0/24`, i.e., `100.<68 + floor(i/256)>.<i mod 256>.0/24` for $i \in [0, 999]$) in the `DEFAULT` network instance with a strict 1-to-1 AFT hierarchy for each index $i \in [0, 999]$:
    * `NextHop` (`index = i + 1`, i.e., `1..1000`, `network-instance = DEFAULT`): `ip-address = 100.<64 + floor(i/256)>.<i mod 256>.10` (`100.64.0.10` through `100.67.231.10`), which resolves into the $i$-th advertised BGP prefix (`100.<64 + floor(i/256)>.<i mod 256>.0/24`).
    * `NextHopGroup` (`id = i + 1`, i.e., `1..1000`, `network-instance = DEFAULT`): references `NextHop(index = i + 1)` with `weight = 1`.
    * `IPv4Entry` (`prefix = 100.<68 + floor(i/256)>.<i mod 256>.0/24`, `network-instance = DEFAULT`): references `NextHopGroup(id = i + 1)` (`next-hop-group-network-instance = DEFAULT`).
  * Ensure `FIB_PROGRAMMED` ACK is received for all 3,000 `AFTOperation`s.
  * Validate gRIBI route installation via the gNMI AFT path for a sample prefix:
    `/network-instances/network-instance[name=DEFAULT]/afts/ipv4-unicast/ipv4-entry[prefix=100.68.0.0/24]/state/prefix`
* Step 4 - Send Baseline Traffic.
  * Resolve IPv4 ARP on all ATE interfaces before starting traffic.
  * Send continuous IPv4 traffic (`Flow-1`, frame size `512` bytes, rate `10,000 fps`) from ATE `port-1` (source IP `198.51.100.2`) destined to all 1,000 gRIBI prefixes using 1,000 destination IPs: `100.68.0.10` through `100.71.231.10` (step `0.0.1.0`, count `1,000`, i.e., `100.<68 + floor(i/256)>.<i mod 256>.10` for $i \in [0, 999]$).
  * Send a secondary continuous IPv4 background traffic stream (`Flow-2`, frame size `512` bytes, rate `1,000 fps`) from ATE `port-1` (source IP `198.51.100.2`) destined to the stable connected route on ATE `port-4` (destination IP `203.0.113.2`) to serve as a negative control.
  * Validate that `Flow-1` is correctly forwarded and received at ATE `port-2` with `0%` loss.
  * Validate that `Flow-2` is received at ATE `port-4` with `0%` loss.

### TE-1.4.2 - BGP Flap and Traffic Convergence

* Step 1 - Flap the BGP routes.
  * ATE port-2 withdraws the 1,000 BGP prefixes (`100.64.0.0/24` through `100.67.231.0/24`).
  * ATE port-3 simultaneously advertises the same 1,000 BGP prefixes (`100.64.0.0/24` through `100.67.231.0/24`) with BGP next-hop `192.0.2.3` (ATE port-3 Loopback).
* Step 2 - Validate Traffic Convergence.
  * Wait for the control plane to converge via gNMI by checking the Loc-RIB path to ensure the BGP routes resolve via the new next-hop `192.0.2.3`.
  * Validate that the gRIBI routes dynamically update their hardware forwarding state to follow the new BGP next-hop (`192.0.2.3`) via the IS-IS path through DUT `port-3` without requiring gRIBI reprovisioning.
  * Validate that traffic (`Flow-1`) to the 1,000 gRIBI routes (`100.68.0.10` through `100.71.231.10`) is now received at ATE `port-3` with `0%` steady-state loss.
  * Calculate traffic loss duration during the switchover using `(Tx frames - Rx frames) / frame rate` and verify that the convergence outage duration is `< 1 second`.
  * Verify that the background traffic stream (`Flow-2` received at ATE `port-4`) maintains `0%` loss, confirming no collateral impact on unrelated routes.

### TE-1.4.3 - IS-IS Underlay Failure (Unreachable Recursive Next-Hop)

* Step 1 - Withdraw IS-IS Underlay route.
  * ATE port-3 withdraws the active IS-IS Loopback route (`192.0.2.3/32`), simulating an underlay failure for the active BGP next-hop.
* Step 2 - Validate Route Unreachability.
  * Validate via gNMI AFT that the IS-IS route `192.0.2.3/32` is removed from the DUT FIB (`/network-instances/network-instance[name=DEFAULT]/afts/ipv4-unicast/ipv4-entry[prefix=192.0.2.3/32]/state/prefix`), immediately invalidating recursive resolution for the BGP and gRIBI routes.
  * Validate that the iBGP session to `192.0.2.3` transitions out of `ESTABLISHED` after the configured `9s` hold-time expires and the 1,000 BGP routes (`100.64.0.0/24` through `100.67.231.0/24`) are removed from the Loc-RIB.
  * Verify that traffic (`Flow-1`) to the 1,000 gRIBI routes (`100.68.0.10` through `100.71.231.10`) is completely dropped (`100%` loss) since the recursive next-hops are no longer reachable.
  * Verify that the background traffic stream (`Flow-2` received at ATE `port-4`) maintains `0%` loss.

### TE-1.4.4 - BGP Route Churn

* Step 1 - Restore Underlay and Induce Route Churn.
  * Restore the IS-IS underlay by re-advertising `192.0.2.3/32` from ATE port-3, verifying via gNMI AFT that `192.0.2.3/32` is reinstalled in the DUT FIB, and confirming via gNMI that both iBGP sessions (`192.0.2.2` and `192.0.2.3`) are in the `ESTABLISHED` state.
  * Perform `5` back-to-back churn iterations for the 1,000 BGP prefixes (`100.64.0.0/24` through `100.67.231.0/24`), where each iteration $k \in \{1..5\}$ consists of:
    * **Phase A:** ATE port-2 advertises `100.64.0.0/24` through `100.67.231.0/24` (next-hop `192.0.2.2`) while ATE port-3 withdraws them.
    * **Phase B:** ATE port-3 advertises `100.64.0.0/24` through `100.67.231.0/24` (next-hop `192.0.2.3`) while ATE port-2 withdraws them.
  * At the end of iteration 5 (Phase B), ATE port-3 (`192.0.2.3`) remains the active advertiser of `100.64.0.0/24` through `100.67.231.0/24`.
* Step 2 - Validate Stability.
  * Ensure both iBGP sessions (`192.0.2.2` and `192.0.2.3`) remain `ESTABLISHED` throughout and after the churn by checking the gNMI session state:
    `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=192.0.2.2]/state/session-state`
    `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=192.0.2.3]/state/session-state`
  * Validate via gNMI that the 1,000 BGP prefixes (`100.64.0.0/24` through `100.67.231.0/24`) are installed in the Loc-RIB and that the 1,000 gRIBI prefixes (`100.68.0.0/24` through `100.71.231.0/24`) remain programmed in the AFT resolving to ATE port-3 (`192.0.2.3`).
  * Verify that `Flow-1` traffic (`100.68.0.10` through `100.71.231.10`) resumes forwarding to ATE `port-3` with `0%` steady-state loss, and `Flow-2` background traffic to ATE `port-4` maintains `0%` loss.

#### Canonical OC

```json
{
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "DEFAULT"
        },
        "name": "DEFAULT",
        "protocols": {
          "protocol": [
            {
              "bgp": {
                "neighbors": {
                  "neighbor": [
                    {
                      "config": {
                        "neighbor-address": "192.0.2.2"
                      },
                      "neighbor-address": "192.0.2.2"
                    },
                    {
                      "config": {
                        "neighbor-address": "192.0.2.3"
                      },
                      "neighbor-address": "192.0.2.3"
                    }
                  ]
                }
              },
              "config": {
                "identifier": "BGP",
                "name": "BGP"
              },
              "identifier": "BGP",
              "name": "BGP"
            }
          ]
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/prefix:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/adjacency-state:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv4-reachability/prefixes/prefix/state/prefix:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
  gribi:
    gRIBI.Flush:
    gRIBI.Get:
    gRIBI.Modify:
```

## Required DUT platform

* FFF
