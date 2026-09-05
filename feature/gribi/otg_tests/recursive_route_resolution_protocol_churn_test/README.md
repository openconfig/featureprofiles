# TE-1.4: gRIBI Recursive Route Resolution under Routing Protocol Churn

## Summary

Verify that gRIBI routes resolving recursively via BGP correctly update their forwarding state when the underlying BGP routes flap (route withdrawal and re-advertisement) without session loss.

## Testbed type

* `TESTBED_DUT_ATE_4LINKS`

## Procedure

### Test environment setup

* Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2, ATE port-3 to DUT port-3, and ATE port-4 to DUT port-4.
* Configure IPv4 addressing on all connected interfaces.
  * DUT port-1: `198.51.100.1/30`, ATE port-1: `198.51.100.2/30`
  * DUT port-2: `198.51.100.5/30`, ATE port-2: `198.51.100.6/30`
  * DUT port-3: `198.51.100.9/30`, ATE port-3: `198.51.100.10/30`
  * DUT port-4: `203.0.113.1/30`, ATE port-4: `203.0.113.2/30`
* Configure IPv4 Loopback addressing.
  * DUT Loopback: `192.0.2.1/32`
  * ATE port-2 Loopback: `192.0.2.2/32`
  * ATE port-3 Loopback: `192.0.2.3/32`
* Establish an IS-IS routing domain between the DUT and ATE.
  * Enable IS-IS on the connected links for port-2 and port-3.
  * Inject the loopback interfaces (`192.0.2.1/32`, `192.0.2.2/32`, `192.0.2.3/32`) into IS-IS.
* Establish BGP sessions between the Loopback interfaces.
  * Establish a BGP session between DUT Loopback `192.0.2.1` and ATE port-2 Loopback `192.0.2.2`.
  * Establish a BGP session between DUT Loopback `192.0.2.1` and ATE port-3 Loopback `192.0.2.3`.
* Establish a gRIBI client connection with the DUT and ensure it becomes the leader.

### TE-1.4.1 - Base gRIBI Recursive Route Resolution

* Step 1 - Validate IS-IS Underlay.
  * Validate that the ATE loopbacks (`192.0.2.2` and `192.0.2.3`) are successfully learned via IS-IS and installed in the DUT's RIB/FIB using the gNMI AFT path:
    `/network-instances/network-instance[name=DEFAULT]/afts/ipv4-unicast/ipv4-entry[prefix=192.0.2.2/32]/state/prefix`
  * Ensure the BGP sessions come up successfully over the IS-IS underlay.
* Step 2 - Advertise BGP routes from ATE port-2.
  * ATE port-2 advertises 1,000 BGP IPv4 prefixes (`10.0.0.0/24` through `10.0.3.231/24`) with next-hop `192.0.2.2` (ATE port-2 Loopback).
  * Validate DUT receives and installs the BGP routes in the Loc-RIB by checking the gNMI path for a sample prefix (e.g., `10.0.0.0/24`):
    `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=BGP][name=BGP]/bgp/rib/afi-safis/afi-safi[afi-safi-name=IPV4_UNICAST]/ipv4-unicast/loc-rib/routes/route[prefix=10.0.0.0/24]/state/prefix`
* Step 3 - Install gRIBI recursive routes.
  * Use gRIBI Modify RPC to install 1,000 IPv4 routes (`20.0.0.0/24` through `20.0.3.231/24`) in the `DEFAULT` network instance.
  * Each gRIBI route recursively resolves to a NextHopGroup containing a NextHop that points to a specific IP within the advertised BGP prefixes (e.g., gRIBI route `20.0.0.0/24` points to NextHop `10.0.0.10`).
  * Ensure FIB ACK is received for all AFTOperations.
  * Validate gRIBI route installation via the gNMI AFT path for a sample prefix:
    `/network-instances/network-instance[name=DEFAULT]/afts/ipv4-unicast/ipv4-entry[prefix=20.0.0.0/24]/state/prefix`
* Step 4 - Send Baseline Traffic.
  * Send continuous IPv4 traffic from ATE port-1 destined to the 1,000 gRIBI routes (e.g., `20.0.0.10` through `20.0.3.10`).
  * Send a secondary background traffic stream to a stable, non-flapping route (e.g., to ATE port-4 IP `203.0.113.2`) to serve as a negative control.
  * Validate that the main traffic stream is correctly forwarded and received at ATE port-2 with `0%` loss.
  * Validate the background traffic has `0%` loss.

### TE-1.4.2 - BGP Flap and Traffic Convergence

* Step 1 - Flap the BGP routes.
  * ATE port-2 withdraws the 1,000 BGP prefixes (`10.0.0.0/24` through `10.0.3.231/24`).
  * ATE port-3 simultaneously advertises the same 1,000 BGP prefixes with next-hop `192.0.2.3` (ATE port-3 Loopback).
* Step 2 - Validate Traffic Convergence.
  * Wait for the control plane to converge via gNMI by checking the Loc-RIB path to ensure the next-hop for the BGP routes updates to `192.0.2.3`.
  * Validate that the gRIBI routes dynamically update their hardware forwarding state to the new BGP next-hop without requiring gRIBI reprovisioning. This correctly resolves via the new IS-IS path through Port-3.
  * Validate that traffic to the 1,000 gRIBI routes is now received at ATE port-3.
  * Calculate traffic loss using the formula: `(Tx frames - Rx frames) / frame rate`. Verify that the loss duration is within acceptable convergence limits (e.g., `< 1 second`).
  * Verify that the background traffic stream has `0%` loss, ensuring no collateral impact on unrelated routes.

### TE-1.4.3 - IS-IS Underlay Failure (Unreachable Recursive Next-Hop)

* Step 1 - Withdraw IS-IS Underlay route.
  * ATE port-3 withdraws the active IS-IS Loopback route (`192.0.2.3/32`), simulating an underlay failure.
* Step 2 - Validate Route Unreachability.
  * Validate via gNMI that the IS-IS route `192.0.2.3/32` is removed from the FIB.
  * Validate that the BGP session eventually drops and the BGP routes are removed from the Loc-RIB.
  * Verify that traffic to the 1,000 gRIBI routes is completely dropped (`100%` loss) since the recursive next-hops are no longer reachable.
  * Verify that the background traffic stream maintains `0%` loss.

### TE-1.4.4 - BGP Route Churn

* Step 1 - Induce Route Churn.
  * Rapidly advertise and withdraw the 1,000 BGP prefixes alternating between ATE port-2 and ATE port-3 for 5 to 10 iterations (ensuring the IS-IS underlay for both paths is restored and active).
* Step 2 - Validate Stability.
  * Ensure the BGP sessions remain established throughout the churn by checking the gNMI session state:
    `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=192.0.2.2]/state/session-state`
  * Validate that the final iteration successfully programs the routes to the last advertised next-hop.
  * Verify traffic flow resumes to the correct ATE port with `0%` steady-state loss.

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
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/adjacencies/adjacency/state/adjacency-state:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/extended-ipv4-reachability/prefixes/prefix/state/prefix:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```

## Required DUT platform

* FFF
