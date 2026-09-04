# TE-5.1: gRIBI Get RPC

## Summary

Validate gRIBI Get RPC.

## Testbed type

* `featureprofiles/topologies/atedut_2.testbed`

## Procedure

### Test environment setup

*   Connect ATE port-1 to DUT port-1 and ATE port-2 to DUT port-2.
*   Assign IP addresses to the interfaces:
    *   ATE port-1: 192.0.2.1/30, DUT port-1: 192.0.2.2/30 (IPv4)
    *   ATE port-1: 2001:db8::1/126, DUT port-1: 2001:db8::2/126 (IPv6)
    *   ATE port-2: 198.51.100.1/30, DUT port-2: 198.51.100.2/30 (IPv4)
    *   ATE port-2: 2001:db8:1::1/126, DUT port-2: 2001:db8:1::2/126 (IPv6)
*   Establish gRIBI client connection to DUT referred to as gRIBI-A, along with a second client referred to as gRIBI-B. Both should use `PRESERVE` persistence and `SINGLE_PRIMARY` mode, with FIB ACK requested.
*   Make gRIBI-A become the leader.
*   Configure network instances `DEFAULT` and `VRF-A` on the DUT.

### TestID-5.1.1 - gRIBI Get All and Scale

*   **Step 1 - Generate DUT configuration:**
    *   Via gRIBI-A, program a scaled number of entries:
        *   1,000 IPv4 routes (e.g., `20.0.0.0/24` to `20.3.231.0/24`)
        *   1,000 IPv6 routes (e.g., `2001:db8:20::/64` to `2001:db8:23:e7::/64`)
    *   These routes should point to a NextHopGroup containing a single NextHop resolving to the ATE port-2 IP address (IPv4 `198.51.100.1`, IPv6 `2001:db8:1::1`).
*   **Step 2 - Push configuration to DUT using gRIBI:**
    *   Send the generated AFT entries via the gRIBI `Modify` RPC.
*   **Step 3 - Validation with gNMI and Traffic:**
    *   Use `gNMI.Subscribe` (ON_CHANGE) or `gNMI.Get` to monitor the AFT telemetry paths:
        *   `/network-instances/network-instance[name=DEFAULT]/afts/ipv4-unicast/ipv4-entry/state/prefix`
        *   `/network-instances/network-instance[name=DEFAULT]/afts/ipv6-unicast/ipv6-entry/state/prefix`
    *   Wait until all 2,000 routes are reported as installed in the FIB via telemetry before proceeding. Do not use static wait times.
    *   Send IPv4 and IPv6 traffic from ATE port-1 to a subset of the programmed destination prefixes (e.g., first and last prefix).
    *   Validate that traffic is successfully received at ATE port-2 with no loss, confirming data plane programming.
*   **Step 4 - Validate Get RPC:**
    *   Issue a `Get` RPC from gRIBI-A specifying the `DEFAULT` network instance and requesting all AFT entries (AFT parameter set to ALL).
    *   Ensure that exactly 1,000 IPv4 entries, 1,000 IPv6 entries, and their associated NextHops and NextHopGroups are returned.
    *   Ensure all entries are returned with `fib_status` = `PROGRAMMED`.
    *   Measure the latency of the `Get` RPC response. Ensure it completes in a reasonable time.

### TestID-5.1.2 - gRIBI Get from Non-Leader Client

*   **Step 1 - Validate Get RPC from Secondary Client:**
    *   With the configuration from TestID-5.1.1 still active, issue a `Get` RPC from gRIBI-B (the non-leader client) for all AFT entries in the `DEFAULT` network instance.
    *   Ensure that exactly 1,000 IPv4 entries, 1,000 IPv6 entries, and their associated NH/NHGs are returned.
    *   Ensure all entries are returned with `fib_status` = `PROGRAMMED`.
    *   Measure the latency of the `Get` RPC response.

### TestID-5.1.3 - gRIBI Get Filtering by AFT Type

*   **Step 1 - Validate IPv4 Filter:**
    *   Issue a `Get` RPC from gRIBI-A specifying the `DEFAULT` network instance and filtering for `IPv4` AFT entries.
    *   Ensure only the 1,000 IPv4 entries are returned, with no IPv6, NH, or NHG entries.
*   **Step 2 - Validate NextHopGroup Filter:**
    *   Issue a `Get` RPC from gRIBI-A specifying the `DEFAULT` network instance and filtering for `NEXTHOP_GROUP` AFT entries.
    *   Ensure only the configured NextHopGroup(s) are returned.

### TestID-5.1.4 - gRIBI Get Specific Network Instance

*   **Step 1 - Generate DUT configuration:**
    *   Via gRIBI-A, program 1,000 IPv6 routes to the non-default network-instance (VRF) `VRF-A`.
*   **Step 2 - Push configuration to DUT using gRIBI:**
    *   Send the generated AFT entries via the gRIBI `Modify` RPC for `VRF-A`.
*   **Step 3 - Validation with gNMI and Traffic:**
    *   Validate entries are installed through gNMI AFT telemetry at `/network-instances/network-instance[name=VRF-A]/afts/ipv6-unicast/ipv6-entry/state/prefix` before proceeding.
    *   Send traffic validating the programmed routes in `VRF-A` (e.g., encapsulated/tagged if topology supports).
*   **Step 4 - Validate Get RPC:**
    *   Issue a `Get` RPC from gRIBI-A specifying the network instance `VRF-A` and requesting all entries.
    *   Ensure that exactly the 1,000 IPv6 entries (and associated NHs/NHGs) programmed for `VRF-A` are returned.
    *   Ensure no entries from the `DEFAULT` network instance are included in the response.
    *   Verify the response is consistent with the state reported via gNMI telemetry.

### TestID-5.1.5 - gRIBI Get with Unresolved Next-Hop

*   **Step 1 - Generate DUT configuration:**
    *   Inject an entry that cannot be installed into the FIB due to an unresolved next-hop (e.g., `203.0.113.0/24` -> unresolved `192.0.2.254/32`) via gRIBI-A in the `DEFAULT` network instance.
*   **Step 2 - Validation with gNMI:**
    *   Wait for gNMI AFT telemetry to reflect the state of this entry (should not be present or not programmed in the FIB).
*   **Step 3 - Validate Get RPC:**
    *   Issue a `Get` RPC from gRIBI-A for the `DEFAULT` network instance.
    *   Ensure that the `IPEntry` for `203.0.113.0/24` is returned with `fib_status` = `NOT_PROGRAMMED` and `rib_status` = `PROGRAMMED`.

### TestID-5.1.6 - Negative Test Cases

*   **Step 1 - Validate Non-Existent Network Instance:**
    *   Issue a `Get` RPC from gRIBI-A specifying a non-existent network instance (e.g., `VRF-NONEXISTENT`).
    *   Ensure that the request completes without crashing the gRIBI server and returns an empty result or appropriate error indication.
*   **Step 2 - Validate Empty Network Instance:**
    *   Configure a new network instance `VRF-EMPTY` without programming any gRIBI routes into it.
    *   Issue a `Get` RPC from gRIBI-A specifying `VRF-EMPTY`.
    *   Ensure the request returns an empty result with no entries.

[fib_status]: https://github.com/openconfig/gribi/blob/08d53dffce45e942c6e7f07521c58b557984e4b7/v1/proto/service/gribi.proto#L485
[rib_status]: https://github.com/openconfig/gribi/blob/08d53dffce45e942c6e7f07521c58b557984e4b7/v1/proto/service/gribi.proto#L483

## Canonical OC

```json
{
  "openconfig-network-instance:network-instances": {
    "network-instance": [
      {
        "name": "VRF-A",
        "config": {
          "name": "VRF-A",
          "type": "openconfig-network-instance-types:L3VRF"
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/name:
  /interfaces/interface/config/type:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled:
  /network-instances/network-instance/config/name:
  /network-instances/network-instance/config/type:
  /network-instances/network-instance/interfaces/interface/config/id:
  /network-instances/network-instance/interfaces/interface/config/subinterface:
  /network-instances/network-instance/protocols/protocol/config/identifier:
  /network-instances/network-instance/protocols/protocol/config/name:
  /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/index:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

FFF
