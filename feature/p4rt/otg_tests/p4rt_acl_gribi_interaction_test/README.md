# TE-1.3: P4RT ACL Interaction with gRIBI Forwarding

## Summary

This test verifies the predictable behavior of the data plane when a packet is
matched by both a P4RT-programmed table entry (such as an ACL) and a
gRIBI-programmed route. The test ensures that the actual packet treatment
conforms to the expected vendor-documented pipeline order (e.g., P4RT ACL
taking precedence over gRIBI L3 lookup), and that the interaction is consistent
and stable.

## Testbed type

* [`TESTBED_DUT_ATE_2_LINKS`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

### Test environment setup

* Configure two interfaces on the DUT connected to ATE port 1 and ATE port 2.
  * ATE port 1 IP: `192.0.2.2/30`
  * DUT port 1 IP: `192.0.2.1/30`
  * ATE port 2 IP: `192.0.2.6/30`
  * DUT port 2 IP: `192.0.2.5/30`
* Bring up the interfaces and verify they are `UP` using telemetry `/interfaces/interface/state/oper-status`.
* Establish gRIBI and P4RT client connections to the DUT.

### TE-1.3.1 - Baseline gRIBI Forwarding

* Step 1 - Program a gRIBI route
  * Program a gRIBI IPv4 route in network instance `DEFAULT` for destination prefix `198.51.100.0/24`.
  * Create a Next-Hop Group pointing to a Next-Hop with IP `192.0.2.6` (ATE port 2).
  * Validate route installation using `gNMI.Subscribe` (ON_CHANGE) or `gNMI.Get` on paths:
    * `/network-instances/network-instance[name=DEFAULT]/afts/ipv4-unicast/ipv4-entry[prefix=198.51.100.0/24]/state/prefix`
    * `/network-instances/network-instance[name=DEFAULT]/afts/ipv4-unicast/ipv4-entry[prefix=198.51.100.0/24]/state/next-hop-group`

* Step 2 - Send Traffic
  * Send IPv4 traffic from ATE port 1 to destination IP `198.51.100.1`.
  * Send IPv4 traffic from ATE port 1 to destination IP `198.51.100.2`.
  * Verify 0% packet loss for both streams to confirm baseline gRIBI forwarding is active.

### TE-1.3.2 - P4RT ACL Drop action takes precedence over gRIBI Forwarding

* Step 1 - Program a P4RT ACL rule
  * Program a P4RT IPv4 ACL rule that matches destination IP `198.51.100.1/32` with action `DROP`.
  * Validate the P4RT ACL installation by ensuring a successful P4RT `WriteResponse` is received.

* Step 2 - Send Traffic
  * Send test stream: IPv4 traffic from ATE port 1 to destination IP `198.51.100.1` (matching both the gRIBI route and the P4RT ACL).
  * Send control stream: IPv4 traffic from ATE port 1 to destination IP `198.51.100.2` (matching only the gRIBI route).

* Step 3 - Validation with pass/fail criteria
  * Verify traffic to `198.51.100.1` is dropped (>99% loss is pass) as the P4RT ACL takes precedence.
  * Verify traffic to `198.51.100.2` is forwarded correctly to ATE port 2 (0% loss is pass) preventing false positives.

### TE-1.3.3 - Scaled gRIBI and P4RT ACL Interaction

* Step 1 - Program Scaled routes and ACLs
  * Program 1,000 gRIBI IPv4 routes (e.g., `10.0.0.0/24` through `10.3.231.0/24`) with next-hop pointing to ATE port 2.
  * Verify all 1,000 routes are programmed via telemetry using `gNMI.Get` on AFT state.
  * Program 1,000 P4RT IPv4 ACL rules matching specific host IPs within each of those routes (e.g., `10.x.y.1/32`) with action `DROP`.
  * Validate installation via P4RT `WriteResponse`.

* Step 2 - Send Traffic
  * Send traffic streams to the 1,000 matched host IPs.
  * Send traffic streams to 1,000 unmatched host IPs within the routed subnets.

* Step 3 - Validation with pass/fail criteria
  * Verify traffic to the matched host IPs is dropped (>99% loss is pass).
  * Verify traffic to the unmatched host IPs is forwarded (0% loss is pass).

### TE-1.3.4 - Reverting P4RT ACL restores gRIBI forwarding

* Step 1 - Delete P4RT ACL rule
  * Delete the P4RT IPv4 ACL rule matching `198.51.100.1/32` (configured in TE-1.3.2).
  * Validate deletion via P4RT `WriteResponse`.

* Step 2 - Send Traffic
  * Send traffic from ATE port 1 to destination IP `198.51.100.1`.

* Step 3 - Validation with pass/fail criteria
  * Verify traffic to `198.51.100.1` is forwarded correctly to ATE port 2 (0% loss is pass), confirming traffic falls back to the gRIBI route.

### TE-1.3.5 - Removal of gRIBI route with active P4RT ACL

* Step 1 - Delete gRIBI route
  * Re-program the P4RT IPv4 ACL rule matching `198.51.100.1/32` with action `DROP`.
  * Delete the gRIBI IPv4 route for destination prefix `198.51.100.0/24`.
  * Validate route deletion via telemetry ensuring the prefix is removed from the AFT state.

* Step 2 - Send Traffic
  * Send test stream: traffic from ATE port 1 to destination IP `198.51.100.1` (matches active P4RT ACL).
  * Send control stream: traffic from ATE port 1 to destination IP `198.51.100.2` (no matching route).

* Step 3 - Validation with pass/fail criteria
  * Verify traffic to `198.51.100.1` is dropped (>99% loss is pass) as it matches the P4RT ACL.
  * Verify traffic to `198.51.100.2` is dropped (>99% loss is pass) because there is no route.

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
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/config/enabled:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/config/name:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

* FFF
