# TE-1.22: Traffic Blackhole on gRIBI Invalid Next-Hop (Strict FIB Precedence)

## Summary

Validate device-level hardware adherence to controller routing intent. If a programmed
gRIBI tunnel next-hop goes down (invalid/disconnected), the device must strictly
honor the programmed FIB route and blackhole the traffic, rather than autonomously
falling back to a less-specific native route (e.g. BGP underlay). This ensures that
controller-programmed routing intent overrides default hardware behaviors,
preventing traffic leakage into the native BGP underlay.

## Testbed type

* `TESTBED_DUT_ATE_4LINKS`

## Procedure

### Test environment setup

*   **Topology map:**
    *   `port1`: ATE Port 1 connected to DUT Port 1.
    *   `port2`: ATE Port 2 connected to DUT Port 2.
    *   `port3`: ATE Port 3 connected to DUT Port 3.
*   **IP Addressing:**
    *   `port1`: DUT `192.0.2.1/30` & `2001:db8:1::1/126`, ATE `192.0.2.2/30` & `2001:db8:1::2/126`
    *   `port2`: DUT `192.0.2.5/30` & `2001:db8:2::1/126`, ATE `192.0.2.6/30` & `2001:db8:2::2/126`
    *   `port3`: DUT `192.0.2.9/30` & `2001:db8:3::1/126`, ATE `192.0.2.10/30` & `2001:db8:3::2/126`
*   **BGP Configuration:**
    *   Establish IPv4 and IPv6 eBGP sessions between ATE Port 1 and DUT Port 1 (DUT ASN `65001`, ATE ASN `65002`).
    *   Establish IPv4 and IPv6 eBGP sessions between ATE Port 2 and DUT Port 2 (DUT ASN `65001`, ATE ASN `65003`).
    *   Use `gNMI Subscribe` (ON_CHANGE) to `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=192.0.2.2]/state/session-state` (and equivalent paths for all neighbors) to verify session states reach `ESTABLISHED`.

### TE-1.22.1 - IPv4 Strict FIB Precedence Blackholing at Scale

*   Step 1 - Base Route Injection
    *   Inject 1,000 IPv4 BGP routes (`198.51.0.0/24` through `198.51.3.231/24`) from ATE Port 1, with next-hop pointing to ATE Port 1.
    *   Use `gNMI Subscribe` to verify the routes are installed in the AFT: `/network-instances/network-instance[name=DEFAULT]/afts/ipv4-unicast/ipv4-entry[prefix=198.51.x.0/24]/state/prefix`.
    *   Send IPv4 traffic from ATE Port 3 cycling through all 1,000 destinations (`198.51.0.1` through `198.51.3.231.1`).
    *   Verify 0% traffic loss (< 0.1% tolerance) at ATE Port 1 Rx.

*   Step 2 - gRIBI Route Programming
    *   Using gRIBI, program 1,000 more specific IPv4 routes (`198.51.0.0/25` through `198.51.3.231/25`) pointing to a NextHopGroup containing a single NextHop of ATE Port 2.
    *   Use `gNMI Subscribe` to verify the 1,000 gRIBI routes are successfully programmed in the AFT: `/network-instances/network-instance[name=DEFAULT]/afts/ipv4-unicast/ipv4-entry[prefix=198.51.x.0/25]/state/prefix`.

*   Step 3 - Traffic Forwarding Update
    *   Send IPv4 traffic from ATE Port 3 cycling through all 1,000 destinations.
    *   Verify 0% traffic loss (< 0.1% tolerance) at ATE Port 2 Rx, confirming traffic has shifted to the gRIBI routes.

*   Step 4 - Trigger Failure
    *   Bring down DUT Port 2 administratively via `gNMI Set` on `/interfaces/interface[name=port2]/config/enabled` to `false`.
    *   Use `gNMI Subscribe` on `/interfaces/interface[name=port2]/state/oper-status` and wait for it to transition to `DOWN`.

*   Step 5 - Validation and Pass/Fail Criteria
    *   Send IPv4 traffic from ATE Port 3 cycling through all 1,000 destinations.
    *   Pass Criteria:
        *   Exactly 100% of the traffic is dropped (blackholed) by the DUT. ATE Port 1 and Port 2 Rx must receive 0 packets.
        *   Verify via `gNMI Get` that `/interfaces/interface[name=port1]/state/counters/out-pkts` and `/interfaces/interface[name=port2]/state/counters/out-pkts` do not increment during the traffic run.
        *   Verify via `gNMI Get` that `/interfaces/interface[name=port3]/state/counters/in-pkts` increments appropriately, proving traffic enters the hardware but is intentionally dropped.
        *   Verify via `gNMI Get` that the 1,000 gRIBI routes remain programmed in the AFT (`/network-instances/network-instance[name=DEFAULT]/afts/ipv4-unicast/ipv4-entry[prefix=198.51.x.0/25]/state/prefix`).
    *   Fail Criteria: Traffic is forwarded out DUT Port 1 (BGP underlay route fallback) or the gRIBI routes are automatically withdrawn from the AFT.

#### Canonical OC

```json
{
  "network-instances": {
    "network-instance": [
      {
        "name": "DEFAULT",
        "config": {
          "name": "DEFAULT"
        }
      }
    ]
  }
}

### TE-1.22.2 - IPv6 Strict FIB Precedence Blackholing at Scale

*   Step 1 - Base Route Injection
    *   Inject 1,000 IPv6 BGP routes (`2001:db8:1000::/48` through `2001:db8:13e7::/48`) from ATE Port 1, with next-hop pointing to ATE Port 1.
    *   Use `gNMI Subscribe` to verify the routes are installed in the AFT: `/network-instances/network-instance[name=DEFAULT]/afts/ipv6-unicast/ipv6-entry[prefix=2001:db8:x::/48]/state/prefix`.
    *   Send IPv6 traffic from ATE Port 3 cycling through all 1,000 destinations (`2001:db8:1000::1` through `2001:db8:13e7::1`).
    *   Verify 0% traffic loss (< 0.1% tolerance) at ATE Port 1 Rx.

*   Step 2 - gRIBI Route Programming
    *   Using gRIBI, program 1,000 more specific IPv6 routes (`2001:db8:1000:1::/64` through `2001:db8:13e7:1::/64`) pointing to a NextHopGroup containing a single NextHop of ATE Port 2.
    *   Use `gNMI Subscribe` to verify the 1,000 gRIBI routes are successfully programmed in the AFT: `/network-instances/network-instance[name=DEFAULT]/afts/ipv6-unicast/ipv6-entry[prefix=2001:db8:x:1::/64]/state/prefix`.

*   Step 3 - Traffic Forwarding Update
    *   Send IPv6 traffic from ATE Port 3 cycling through all 1,000 destinations.
    *   Verify 0% traffic loss (< 0.1% tolerance) at ATE Port 2 Rx, confirming traffic has shifted to the gRIBI routes.

*   Step 4 - Trigger Failure
    *   Bring down DUT Port 2 administratively via `gNMI Set` on `/interfaces/interface[name=port2]/config/enabled` to `false`.
    *   Use `gNMI Subscribe` on `/interfaces/interface[name=port2]/state/oper-status` and wait for it to transition to `DOWN`.

*   Step 5 - Validation and Pass/Fail Criteria
    *   Send IPv6 traffic from ATE Port 3 cycling through all 1,000 destinations.
    *   Pass Criteria:
        *   Exactly 100% of the traffic is dropped (blackholed) by the DUT. ATE Port 1 and Port 2 Rx must receive 0 packets.
        *   Verify via `gNMI Get` that `/interfaces/interface[name=port1]/state/counters/out-pkts` and `/interfaces/interface[name=port2]/state/counters/out-pkts` do not increment during the traffic run.
        *   Verify via `gNMI Get` that `/interfaces/interface[name=port3]/state/counters/in-pkts` increments appropriately, proving traffic enters the hardware but is intentionally dropped.
        *   Verify via `gNMI Get` that the 1,000 gRIBI routes remain programmed in the AFT (`/network-instances/network-instance[name=DEFAULT]/afts/ipv6-unicast/ipv6-entry[prefix=2001:db8:x:1::/64]/state/prefix`).
    *   Fail Criteria: Traffic is forwarded out DUT Port 1 (BGP underlay route fallback) or the gRIBI routes are automatically withdrawn from the AFT.

### TE-1.22.3 - Negative Test: Controller Withdraws gRIBI Intent During Blackhole

*   Step 1 - Pre-requisites
    *   Ensure the environment is in the final state of TE-1.22.2 (DUT Port 2 is down, traffic is blackholed, gRIBI routes remain programmed).
*   Step 2 - Withdraw gRIBI Routes
    *   Using gRIBI, delete the 1,000 more specific IPv6 routes (`2001:db8:1000:1::/64` through `2001:db8:13e7:1::/64`).
    *   Use `gNMI Subscribe` to verify the routes are removed from the AFT: `/network-instances/network-instance[name=DEFAULT]/afts/ipv6-unicast/ipv6-entry[prefix=2001:db8:x:1::/64]/state/prefix`.
*   Step 3 - Traffic Forwarding Update
    *   Send IPv6 traffic from ATE Port 3 cycling through all 1,000 destinations.
    *   Verify 0% traffic loss (< 0.1% tolerance) at ATE Port 1 Rx. 
    *   Pass Criteria: Traffic immediately resumes forwarding via DUT Port 1 (BGP underlay) because the controller intentionally withdrew its override intent.

### TE-1.22.4 - Negative Test: Withdraw BGP Underlay and Port Recovery

*   Step 1 - Pre-requisites
    *   Re-program the 1,000 more specific IPv6 routes (`2001:db8:1000:1::/64` through `2001:db8:13e7:1::/64`) using gRIBI. DUT Port 2 is still down.
    *   Use `gNMI Subscribe` to verify the gRIBI routes are installed. Traffic should once again be 100% blackholed.
*   Step 2 - Withdraw BGP Routes
    *   Withdraw the 1,000 IPv6 BGP routes (`2001:db8:1000::/48` through `2001:db8:13e7::/48`) from ATE Port 1.
    *   Use `gNMI Subscribe` to verify the routes are removed from the AFT.
*   Step 3 - Traffic Validation
    *   Send IPv6 traffic from ATE Port 3 cycling through all 1,000 destinations.
    *   Verify exactly 100% loss. Traffic must remain blackholed by the gRIBI routes.
*   Step 4 - Interface Recovery
    *   Bring up DUT Port 2 administratively via `gNMI Set` on `/interfaces/interface[name=port2]/config/enabled` to `true`.
    *   Use `gNMI Subscribe` on `/interfaces/interface[name=port2]/state/oper-status` and wait for it to transition to `UP`.
*   Step 5 - Final Validation
    *   Send IPv6 traffic from ATE Port 3 cycling through all 1,000 destinations.
    *   Verify 0% traffic loss (< 0.1% tolerance) at ATE Port 2 Rx, confirming the gRIBI routes correctly recovered when the next-hop became valid again, independently of the BGP underlay.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /interfaces/interface/config/enabled:
  /interfaces/interface/state/counters/in-pkts:
  /interfaces/interface/state/counters/out-pkts:
  /interfaces/interface/state/oper-status:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

* FFF - fixed form factor
