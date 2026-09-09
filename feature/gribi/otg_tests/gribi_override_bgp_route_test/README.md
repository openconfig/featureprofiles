# TE-2.3: gRIBI Tunnel Programming over Active BGP

## Summary
Verify that a gRIBI-injected tunnel route for a specific prefix correctly overrides a BGP-learned route for the same prefix. This test ensures the switch prioritizes SDN controller intent for traffic engineering over standard routing protocols. Specifically validates gRIBI Inject (ADD) and Delete (DELETE) operations.

## Testbed type

* [`TESTBED_DUT_ATE_4LINKS`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Topology

* Connect ATE port 1 to DUT port 1 (eBGP peer, original BGP destination).
* Connect ATE port 2 to DUT port 2 (Traffic source).
* Connect ATE port 3 to DUT port 3 (gRIBI tunnel destination).

## Procedure

### Test environment setup

* Configure interfaces on the DUT:
  * DUT port 1: `192.0.2.1/30`, `2001:db8:1::1/126`
  * DUT port 2: `192.0.2.5/30`, `2001:db8:2::1/126`
  * DUT port 3: `192.0.2.9/30`, `2001:db8:3::1/126`

* Configure interfaces on the ATE:
  * ATE port 1: `192.0.2.2/30`, `2001:db8:1::2/126`
  * ATE port 2: `192.0.2.6/30`, `2001:db8:2::2/126`
  * ATE port 3: `192.0.2.10/30`, `2001:db8:3::2/126`

* Establish eBGP sessions (IPv4 and IPv6) between ATE port 1 (AS 65002) and DUT port 1 (AS 65001).
  * Use gNMI to poll `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor[neighbor-address=192.0.2.2]/state/session-state` until `ESTABLISHED`.

* Advertise a scale block of 1000 IPv4 prefixes starting from `198.18.0.0/15` from ATE port 1 via BGP.
* Advertise a scale block of 1000 IPv6 prefixes starting from `2001:db8:4::/48` from ATE port 1 via BGP.

* Validate BGP routes installation:
  * Use gNMI to poll `/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/origin-protocol` for the 1000 prefixes, wait until they are `BGP`.
  * Use gNMI to poll `/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/origin-protocol` for the 1000 prefixes, wait until they are `BGP`.

### TE-2.3.1 - Baseline BGP Forwarding

* Step 1 - Send Traffic
  * Using the OTG API, configure continuous traffic flows from ATE port 2 to the 1000 IPv4 and 1000 IPv6 prefixes at a fixed rate (e.g., 1000 packets per second).
  * Start the traffic flows.

* Step 2 - Validation
  * Verify using OTG flow metrics (`rx_frames`) that traffic arrives at ATE port 1.
  * Verify `tx_frames == rx_frames` (0 packet loss) within acceptable tolerance.
  * Verify ATE port 3 receives 0 packets (`rx_frames == 0`).
  * Stop the traffic flow.

### TE-2.3.2 - gRIBI Tunnel Overrides Active BGP IPv4/IPv6 Route (ADD)

* Step 1 - Generate DUT configuration
In this step, we inject gRIBI routes that override the existing BGP routes.

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

* Step 2 - Start Traffic and Inject the route via gRIBI
  * Using the OTG API, configure continuous traffic flows from ATE port 2 to the 1000 IPv4 and 1000 IPv6 prefixes at a fixed rate (e.g., 1000 packets per second).
  * Start the traffic flows.
  * Use gRIBI to inject 1000 IPv4 tunnel routes and 1000 IPv6 tunnel routes pointing to a NextHop group out of DUT port 3 (towards ATE port 3).

* Step 3 - Validate OpenConfig State
  * Poll using gNMI to verify that the gRIBI routes are installed in the AFT and override the BGP routes.
  * Wait until `/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/origin-protocol` is `GRIBI` for all 1000 IPv4 prefixes.
  * Wait until `/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/origin-protocol` is `GRIBI` for all 1000 IPv6 prefixes.

* Step 4 - Validate Convergence Time and Traffic Flow
  * Stop the OTG traffic flows.
  * Using the OTG API, fetch the flow statistics (`flow.metrics`).
  * Calculate the convergence time: `Convergence Time = (tx_frames - rx_frames) / configured_tx_rate`.
  * Validate that the calculated convergence time is within the acceptable threshold (e.g., `< 500ms`).
  * Verify traffic is successfully forwarded via the gRIBI-specified tunnel path to ATE port 3.
  * Verify that ATE port 1 recorded 0 packets (`rx_frames == 0`) during the transition window, confirming no leakage or flakiness.

### TE-2.3.3 - gRIBI Tunnel Fallback (DELETE)

* Step 1 - Start Traffic and Delete gRIBI Routes
  * Using the OTG API, start the continuous traffic flows from ATE port 2 to the 1000 IPv4 and 1000 IPv6 prefixes at a fixed rate.
  * Delete the 2000 gRIBI routes.

* Step 2 - Validate OpenConfig State
  * Poll using gNMI to verify that the AFT falls back to the BGP-learned routes.
  * Wait until `/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/origin-protocol` is `BGP` for all 1000 IPv4 prefixes.
  * Wait until `/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/origin-protocol` is `BGP` for all 1000 IPv6 prefixes.

* Step 3 - Validate Convergence Time and Traffic Flow
  * Stop the OTG traffic flows.
  * Using the OTG API, fetch the flow statistics (`flow.metrics`).
  * Calculate and validate convergence time as described in TE-2.3.2.
  * Verify traffic shifts back to the BGP path towards ATE port 1.
  * Verify ATE port 3 receives 0 packets (`rx_frames == 0`) after convergence.

### TE-2.3.4 - gRIBI Tunnel with Down/Invalid Next-Hop (Negative Test)

* Step 1 - Inject gRIBI Routes
  * Ensure baseline BGP routing is active.
  * Inject gRIBI routes overriding BGP, pointing to ATE port 3.
  * Verify traffic routes to ATE port 3.

* Step 2 - Simulate Link Failure
  * Using the OTG API, start traffic from ATE port 2.
  * Bring down the physical link between DUT port 3 and ATE port 3.

* Step 3 - Validation
  * Validate that traffic is dropped (blackholed) and does not fall back to the active BGP route.
  * This proves the DUT strictly adheres to the SDN controller's intent and doesn't illegally failover to a lesser-preference protocol when the active path fails without a programmed backup.

## OpenConfig Path and RPC Coverage
```yaml
paths:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/origin-protocol:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/origin-protocol:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/valid-route:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/state/valid-route:
rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

* FFF
