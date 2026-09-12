# TE-19.1: Group Primary Leader Election

## Summary

Validate `GROUP_PRIMARY` redundancy mode, `redundancy_group`, and Election ID are correctly handled by the gRIBI server, including conflict resolution between different groups and flush operations.

## Procedure

*   Connect DUT port-1 to ATE port-1, DUT port-2 to ATE port-2, DUT port-3 to
    ATE port-3. Assign IPv4 addresses to all ports.
*   Establish three gRIBI clients to the DUT (referred to as `gRIBI-A1`, 
    `gRIBI-A2`, and `gRIBI-B1`).
*   Connect `gRIBI-A1` to DUT specifying `PRESERVE` persistent mode,
    `GROUP_PRIMARY` client redundancy, `redundancy_group` as `Group-A`, and an
    initial Election ID (e.g., 10). Ensure it becomes the leader of `Group-A` and
    no error is reported from the gRIBI server.
*   Connect `gRIBI-A2` to DUT specifying `PRESERVE` persistent mode,
    `GROUP_PRIMARY` client redundancy, `redundancy_group` as `Group-A`, and a
    higher Election ID (e.g., 11). Ensure it becomes the leader of `Group-A`.
*   Connect `gRIBI-B1` to DUT specifying `PRESERVE` persistent mode,
    `GROUP_PRIMARY` client redundancy, `redundancy_group` as `Group-B`, and an
    Election ID (e.g., 10). Ensure it becomes the leader of `Group-B`.
*   Add an `IPv4Entry` for `198.51.100.0/24` pointing to ATE port-2 via
    `gRIBI-A2` (leader of `Group-A`). Ensure that the entry is active through
    AFT telemetry and traffic is routed to ATE port-2.
*   Add an `IPv4Entry` for `198.51.100.0/24` pointing to ATE port-3 via
    `gRIBI-A1` (non-leader of `Group-A`). Ensure that the entry is ignored by the
    DUT.
*   Add an `IPv4Entry` for `198.51.100.0/24` pointing to ATE port-3 via
    `gRIBI-B1` (leader of `Group-B`). Ensure that this operation is accepted by
    the Server (following the "Open" conflict resolution expected behavior) and
    routing is updated to receive packets at ATE port-3.
*   Issue a `FlushRequest` from `gRIBI-A2` supplying its `election` ID and
    `redundancy_group` as `Group-A`. Ensure that all entries are successfully
    flushed from the switch, regardless of which group installed them.

## Protocol/RPC Parameter Coverage

*   gRIBI
    *   ModifyRequest
        *   SessionParameters:
            *   redundancy (GROUP_PRIMARY)
        *   election_id
        *   redundancy_group
    *   FlushRequest
        *   election
        *   redundancy_group

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/type:
  /interfaces/interface/ethernet/config/port-speed:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled:
  /network-instances/network-instance/interfaces/interface/config/id:
rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```
