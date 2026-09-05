# TE-1.21: gRIBI Tunnel Fallback to Native Routing

## Summary

Verify that when a gRIBI action explicitly instructs the switch to fall back to
the default network instance, traffic correctly yields to the underlying native
BGP/ISIS route. Initially, traffic is encapsulated and forwarded via a standard
gRIBI tunnel entry. A gRIBI Modify command updates the route's NextHop to
explicitly specify a fallback/decap action pointing to the default routing
instance. Traffic seamlessly transitions to following the native BGP
shortest-path route, and gNMI AFT telemetry reflects the updated gRIBI entry
intent and the resulting data path change.

## Testbed type

* [`TESTBED_DUT_ATE_4LINKS`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

* Configure 4 interfaces on the DUT and ATE. Connect DUT port 1 to ATE port 1,
  DUT port 2 to ATE port 2, DUT port 3 to ATE port 3, and DUT port 4 to ATE port 4.
* Configure IPv4 and IPv6 addresses on all links:
  * Link 1 (Traffic Source): ATE IP `192.0.2.2/30`, DUT IP `192.0.2.1/30` | ATE IP `2001:db8:1::2/126`, DUT IP `2001:db8:1::1/126`
  * Link 2 (Initial gRIBI Tunnel): ATE IP `192.0.2.6/30`, DUT IP `192.0.2.5/30` | ATE IP `2001:db8:2::2/126`, DUT IP `2001:db8:2::1/126`
  * Link 3 (BGP over ISIS Native Route): ATE IP `192.0.2.10/30`, DUT IP `192.0.2.9/30` | ATE IP `2001:db8:3::2/126`, DUT IP `2001:db8:3::1/126`
  * Link 4 (ISIS Native Route): ATE IP `192.0.2.14/30`, DUT IP `192.0.2.13/30` | ATE IP `2001:db8:4::2/126`, DUT IP `2001:db8:4::1/126`
* On Link 3, configure ISIS routing between DUT and ATE.
  * ATE advertises a Loopback interface via ISIS: IPv4 `198.51.200.1/32`, IPv6 `2001:db8:200::1/128`.
* Establish an eBGP session between DUT and ATE on Link 3 using the ISIS-learned Loopback IPs.
  * ATE AS: 65536
  * DUT AS: 65537
* Establish an ISIS session between DUT and ATE on Link 4.
* Route Advertisements:
  * Via BGP on Link 3: Advertise 1,000 IPv4 prefixes starting from `198.51.0.0/24` (up to `198.51.3.231/24`) and 1,000 IPv6 prefixes starting from `2001:db8:1000::/64` to `2001:db8:13e7::/64`.
  * Via ISIS on Link 4: Advertise 1,000 IPv4 prefixes starting from `198.52.0.0/24` and 1,000 IPv6 prefixes starting from `2001:db8:2000::/64`.
* Use `gNMI.Get` to verify BGP and ISIS sessions are established and the routes are received and installed in the default network instance RIB on the DUT before proceeding.

### TE-1.21.1: Fallback from gRIBI to Native IPv4 BGP Route

* Step 1 - Inject gRIBI routes for the 1,000 IPv4 BGP prefixes (`198.51.x.0/24`) on the DUT.
  * Set the initial gRIBI NextHop action to forward traffic to ATE Port 2.
* Step 2 - Validate initial gRIBI programming via gNMI AFT telemetry.
  * Wait until `/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group` reflects the new NextHop group ID for all 1,000 prefixes.
* Step 3 - Start continuous IPv4 traffic from ATE port 1 to the destinations in the 1,000 prefixes (e.g., at 100 pps per prefix).
  * Verify 100% of the traffic arrives at ATE port 2 and 0% at ATE port 3.
* Step 4 - Send a gRIBI `Modify` command for the 1,000 prefixes, updating the NextHop group to explicitly specify a `decap-fallback-network-instance` pointing to `DEFAULT`.
* Step 5 - Validate gNMI AFT telemetry via `gNMI.Subscribe` with `ON_CHANGE` or `gNMI.Get` polling.
  * Check that `/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group` for all 1,000 prefixes matches the updated gRIBI NextHop group ID that specifies fallback.
* Step 6 - Validate traffic fallback.
  * Verify traffic seamlessly transitions and completely stops arriving at ATE port 2, now arriving exclusively at ATE port 3 (falling back to the BGP native routing table).
  * Validate that transition packet loss is within acceptable limits (e.g., < 1 second of expected packets lost). Stop traffic.

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
        "policy-forwarding": {
          "policies": {
            "policy": [
              {
                "config": {
                  "policy-id": "fallback_policy"
                },
                "policy-id": "fallback_policy",
                "rules": {
                  "rule": [
                    {
                      "action": {
                        "config": {
                          "decap-fallback-network-instance": "DEFAULT"
                        }
                      },
                      "config": {
                        "sequence-id": 1
                      },
                      "sequence-id": 1
                    }
                  ]
                }
              }
            ]
          }
        }
      }
    ]
  }
}
```

### TE-1.21.2: Fallback from gRIBI to Native IPv6 BGP Route

* Step 1 - Inject gRIBI routes for the 1,000 IPv6 BGP prefixes on the DUT.
  * Set the initial gRIBI NextHop action to forward traffic to ATE Port 2.
* Step 2 - Validate initial gRIBI programming via gNMI AFT telemetry for all 1,000 prefixes.
* Step 3 - Start continuous IPv6 traffic from ATE port 1 to the destinations in the 1,000 prefixes.
  * Verify 100% of the traffic arrives at ATE port 2 and 0% at ATE port 3.
* Step 4 - Send a gRIBI `Modify` command for the 1,000 prefixes, updating the NextHop group to explicitly specify a `decap-fallback-network-instance` pointing to `DEFAULT`.
* Step 5 - Validate gNMI AFT telemetry via `gNMI.Subscribe` or `gNMI.Get` polling to confirm the fallback intent is programmed for all prefixes.
* Step 6 - Validate traffic fallback.
  * Verify traffic completely stops arriving at ATE port 2 and exclusively arrives at ATE port 3.
  * Validate transition packet loss is within acceptable limits. Stop traffic.

### TE-1.21.3: Fallback from gRIBI to Native IPv4 ISIS Route

* Step 1 - Inject gRIBI routes for the 1,000 IPv4 ISIS prefixes (`198.52.x.0/24`) on the DUT.
  * Set the initial gRIBI NextHop action to forward traffic to ATE Port 2.
* Step 2 - Validate initial gRIBI programming via gNMI AFT telemetry for all 1,000 prefixes.
* Step 3 - Start continuous IPv4 traffic from ATE port 1 to the destinations in the 1,000 prefixes.
  * Verify 100% of the traffic arrives at ATE port 2 and 0% at ATE port 4.
* Step 4 - Send a gRIBI `Modify` command for the 1,000 prefixes, updating the NextHop group to explicitly specify a `decap-fallback-network-instance` pointing to `DEFAULT`.
* Step 5 - Validate gNMI AFT telemetry via `gNMI.Subscribe` or `gNMI.Get` polling to confirm the fallback intent is programmed for all prefixes.
* Step 6 - Validate traffic fallback.
  * Verify traffic completely stops arriving at ATE port 2 and exclusively arrives at ATE port 4.
  * Validate transition packet loss is within acceptable limits. Stop traffic.

### TE-1.21.4: Fallback from gRIBI to Native IPv6 ISIS Route

* Step 1 - Inject gRIBI routes for the 1,000 IPv6 ISIS prefixes on the DUT.
  * Set the initial gRIBI NextHop action to forward traffic to ATE Port 2.
* Step 2 - Validate initial gRIBI programming via gNMI AFT telemetry for all 1,000 prefixes.
* Step 3 - Start continuous IPv6 traffic from ATE port 1 to the destinations in the 1,000 prefixes.
  * Verify 100% of the traffic arrives at ATE port 2 and 0% at ATE port 4.
* Step 4 - Send a gRIBI `Modify` command for the 1,000 prefixes, updating the NextHop group to explicitly specify a `decap-fallback-network-instance` pointing to `DEFAULT`.
* Step 5 - Validate gNMI AFT telemetry via `gNMI.Subscribe` or `gNMI.Get` polling to confirm the fallback intent is programmed for all prefixes.
* Step 6 - Validate traffic fallback.
  * Verify traffic completely stops arriving at ATE port 2 and exclusively arrives at ATE port 4.
  * Validate transition packet loss is within acceptable limits. Stop traffic.

### TE-1.21.5: Negative Test - Fallback to non-existent Native Route

* Step 1 - Define an IPv4 prefix (e.g., `198.53.0.0/24`) that is **not** advertised by BGP or ISIS.
* Step 2 - Inject a gRIBI route for this prefix, explicitly specifying a `decap-fallback-network-instance` pointing to `DEFAULT`.
* Step 3 - Validate gNMI AFT telemetry to confirm the entry is programmed with the fallback intent.
* Step 4 - Send traffic from ATE port 1 to the destination `198.53.0.1`.
* Step 5 - Validate that 100% of the traffic is dropped by the DUT, since no underlying native route exists in the default network instance.

### TE-1.21.6: Negative Test - Native Route Withdrawal during Fallback

* Step 1 - Utilize one of the prefixes from TE-1.21.1 (e.g., `198.51.0.0/24`) which is currently successfully falling back to the BGP native route on ATE port 3.
* Step 2 - Start continuous traffic to this prefix and confirm it arrives at ATE port 3.
* Step 3 - Withdraw the BGP route for `198.51.0.0/24` from the ATE on Link 3.
* Step 4 - Validate via gNMI that the BGP route is removed from the DUT's RIB.
* Step 5 - Validate that traffic for this prefix completely stops arriving at ATE port 3 and is dropped by the DUT (assuming no other backup route exists).

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/policy-forwarding/policies/policy/config/policy-id:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decap-fallback-network-instance:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/config/sequence-id:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/adjacencies/adjacency/state/adj-state:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

* FFF
