# HA-1.0: BGP Neighbor Telemetry Presence and Compliance

## Summary

This test verifies the presence and OpenConfig compliance of BGP neighbor
telemetry paths. It ensures that the DUT correctly streams neighbor state,
message counters, and prefix limits across all configured network instances and
BGP sessions.

## Testbed Requirements

*   DUT with one or more active BGP sessions.
*   BGP sessions should ideally be in the `ESTABLISHED` state for full counter
    and prefix validation, though the test handles other states.

## Procedure

### TestTelemetryBGPNeighborState

This test case performs integrated validation of critical BGP neighbor telemetry
paths.

1.  **Dynamic Discovery**:
    *   Discover all Network Instances configured on the DUT.
    *   For each Network Instance, identify all protocols of type `BGP`.
    *   For each BGP protocol, discover all configured neighbor addresses.
2.  **Integrated Path Validation**: For each discovered neighbor, the test
    validates several OpenConfig paths by performing the following sequence:
    *   **Presence Check**: Verify the gNMI path exists in the DUT's telemetry
        stream.
    *   **Compliance Check**: If the path is present, validate that the
        streamed value adheres to OpenConfig standards:
        *   **Type Enforcement**: Verify correct data types (Enum, uint64,
            uint32, uint8, string, etc.).
        *   **Session State**: Validate `session-state` against standard BGP
            states (IDLE, CONNECT, ACTIVE, OPENSENT, OPENCONFIRM, ESTABLISHED)
            using granular case statements.
        *   **Timestamps**: Ensure `last-established` contains a valid
            non-negative value.
        *   **Thresholds**: Validate that `warning-threshold-pct` is a valid
            percentage (0-100%).
        *   **Addresses**: Verify that `neighbor-address` and
            `transport/local-address` are correctly streamed.
        *   **Counters**: Ensure message and prefix counters are successfully
            retrieved and logged.
3.  **Deviation Handling**: Dynamically identify and verify prefix-limit paths
    based on DUT-supported deviations (e.g., `prefix-limit` vs
    `prefix-limit-received`).

## OpenConfig Paths Validated

<!-- disableFinding(LINE_OVER_80) -->

*   `openconfig-network-instance:network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state`
*   `openconfig-network-instance:network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/admin-status`
*   `openconfig-network-instance:network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/peer-as`
*   `openconfig-network-instance:network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/neighbor-address`
*   `openconfig-network-instance:network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/state/local-address`
*   `openconfig-network-instance:network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/last-established`
*   `openconfig-network-instance:network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/established-transitions`
*   `openconfig-network-instance:network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/messages/received/UPDATE`
*   `openconfig-network-instance:network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/messages/received/last-notification-error-code`
*   `openconfig-network-instance:network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/messages/sent/UPDATE`
*   `openconfig-network-instance:network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv6-unicast/prefix-limit/state/max-prefixes`
*   `openconfig-network-instance:network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received`
*   `openconfig-network-instance:network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/sent`
*   `openconfig-network-instance:network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit/state/warning-threshold-pct`
