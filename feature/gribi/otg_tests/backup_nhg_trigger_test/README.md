# TE-11.4: gRIBI BACKUP_ACTIVATE Hardware Convergence & FIB ACK Verification

## Summary

Validate that the software-driven/external gRIBI `BACKUP_ACTIVATE` AFT operation triggers immediate dataplane failover to the configured `backup_next_hop_group` (configured as a fallback to `REPAIR_VRF` with decapsulation and re-encapsulation into `TRANSIT_VRF`), and verify that the `FIB_PROGRAMMED` status in `ModifyResponse` correlates with hardware ASIC path switching within the expected convergence threshold ($\le 300\text{ ms}$) during failover and recovery. In addition, validate telemetry on operational state leaves (`backup-active`), query feedback via the gRIBI `Get` RPC, graceful restoration upon deletion (`op: DELETE`), dual-stack (IPv4 and IPv6) forwarding behaviors, hierarchical tunnel group activation across multiple egress trunks, and robust exception handling. This includes rejecting operations on non-existent groups, groups lacking a backup path, deletion of active groups, reverse deletion of inactive triggers, AFT state and identifier reclamation, local gRIBI process restart resilience, Non-Stop Routing (NSR) supervisor switchover synchronization, and asynchronous late-binding of backup next-hop groups.

## Feature Interactions & Architectural Dependencies

`BACKUP_ACTIVATE` is a trigger, not the mechanism that creates the backup path. All primary and backup `NextHop` and `NextHopGroup` entries, including the primary group's `backup_next_hop_group` reference, must first be installed in the `DEFAULT` network instance through ordinary gRIBI `ADD` operations and acknowledged with `RIB_AND_FIB_ACK`. In accordance with WBB gRIBI forwarding designs, the backup NHG points to a next-hop that falls back to `REPAIR_VRF` (`network_instance: "REPAIR_VRF"`), where a matching route performs decapsulation and re-encapsulation (`decap + re-encap`) into `TRANSIT_VRF`. The backup NHG and repair pipeline are therefore programmed and available in the FIB before activation; `BACKUP_ACTIVATE` selects that already-installed backup group for forwarding. Removing the trigger restores the primary group, while deleting referenced groups is expected to fail until the trigger is removed.

Implementations supporting NSR must preserve and synchronize the gRIBI ownership, NHG relationship, active backup-trigger state, and resulting RIB/FIB programming status to the standby process or control-plane component. A switchover or process restart must reconcile the persisted trigger and both NHGs; it must not transiently select the primary path, lose the backup relationship, or report `FIB_PROGRAMMED` before hardware programming is complete. The restart scenario in TE-11.4.5 verifies this behavior and requires forwarding continuity and no duplicate or orphaned AFT entries.

The forwarding driver should pre-resolve backup next hops, including ARP/ND and egress rewrite entries, and install them in a dormant hardware FRR indirection entry. Activation should atomically switch the hardware pointer rather than perform route recomputation or neighbor resolution during failover.

The mechanism complements physical link-down or BFD-driven FRR by handling gray failures, remote blackholes, and VIP drains detected by external probers while the local carrier remains up. Hardware forwarding must remain decoupled from the gRIBI process lifecycle, and deleting an NHG must remove the entry from operational AFT state (`/network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id`) and allow immediate reuse of the deleted identifier without stale state.

## Topology

```text
+-------------------+                 +-------------------+
|                   |                 |                   |
|                   |  DUT Port 1     |  ATE Port 1       |
|                   |=================|  (Ingress Source) |
|                   |                 |                   |
|                   |  DUT Port 2     |  ATE Port 2       |
|      DUT          |=================|  (Primary Trunk1) |
|                   |                 |                   |
|                   |  DUT Port 3     |  ATE Port 3       |
|                   |=================|  (Backup Trunk2)  |
|                   |                 |                   |
|                   |  DUT Port 4     |  ATE Port 4       |
|                   |=================|  (Backup TG Path) |
|                   |                 |                   |
+-------------------+                 +-------------------+
        DUT                                 ATE
```

* Connect ATE port-1 to DUT port-1 (ingress traffic source).
* Connect ATE port-2 to DUT port-2 (primary egress path / trunk 1).
* Connect ATE port-3 to DUT port-3 (backup egress path / trunk 2).
* Connect ATE port-4 to DUT port-4 (alternate / backup TG path / trunk 3).

Configure the following IP addresses on interfaces:

| Link | DUT | ATE |
| --- | --- | --- |
| Port 1 | `198.51.100.1/30`, `2001:db8:1::1/126` | `198.51.100.2/30`, `2001:db8:1::2/126` |
| Port 2 | `198.51.100.5/30`, `2001:db8:2::1/126` | `198.51.100.6/30`, `2001:db8:2::2/126` |
| Port 3 | `198.51.100.9/30`, `2001:db8:3::1/126` | `198.51.100.10/30`, `2001:db8:3::2/126` |
| Port 4 | `198.51.100.13/30`, `2001:db8:4::1/126` | `198.51.100.14/30`, `2001:db8:4::2/126` |

* Destination prefixes:
  * IPv4 data destination: `192.0.2.0/24` (target host: `192.0.2.1`)
  * IPv6 data destination: `2001:db8:feed::/64` (target host: `2001:db8:feed::1`)
  * Transit tunnel destination IPs: `198.18.193.1/32`, `198.18.193.2/32`
* Network instances / VRFs:
  * `DEFAULT` (all gRIBI `NextHop` and `NextHopGroup` entries are programmed in `DEFAULT`)
  * `TRANSIT_VRF` (non-default L3 VRF for primary and backup transit tunnel route resolution)
  * `REPAIR_VRF` (non-default L3 VRF for backup NHG fallback and `decap + re-encap` resolution)

## Testbed Type

`TESTBED_DUT_ATE_4LINKS`

## Procedure

### Test environment setup

1. Configure the DUT and ATE interfaces according to the topology with IPv4 and IPv6 addressing, and create the `DEFAULT`, `TRANSIT_VRF`, and `REPAIR_VRF` network instances.
2. Ensure that DUT ports 1 through 4 are operationally `UP` at `/interfaces/interface/state/oper-status`.
3. Establish a gRIBI client connection with the DUT using `persistence = PRESERVE` and `redundancy = SINGLE_PRIMARY`.
4. Negotiate the election ID and establish leadership.
5. Send a gRIBI `Flush` RPC targeting network instances `DEFAULT`, `TRANSIT_VRF`, and `REPAIR_VRF` to clear stale forwarding entries.

### TE-11.4.1: gRIBI BACKUP_ACTIVATE with FIB ACK and traffic failover (IPv4)

Validate that issuing a gRIBI `BACKUP_ACTIVATE` operation on a primary next-hop group triggers fallback to `REPAIR_VRF` (which performs `decap + re-encap` into the backup transit tunnel in `TRANSIT_VRF`), switches IPv4 traffic within the expected convergence window, returns `FIB_PROGRAMMED`, updates `backup-active` telemetry, and restores primary-path forwarding upon deletion.

1. Program the following `NextHop` and `NextHopGroup` entries in network instance `DEFAULT` via gRIBI (reusing the standard WBB FNT `DEFAULT` / `TRANSIT_VRF` / `REPAIR_VRF` forwarding hierarchy):
   * `NextHop#1`: egress interface DUT port-2, next-hop IP `198.51.100.6`, MAC `00:1A:11:00:00:01`.
   * `NextHop#2`: egress interface DUT port-3, next-hop IP `198.51.100.10`, MAC `00:1A:11:00:00:02`.
   * `NextHop#20`: fallback to `network_instance: "REPAIR_VRF"`.
   * `NextHop#30`: `decap + re-encap` with `decapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4`, `encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4`, `ip_in_ip { src_ip: 198.51.100.1, dst_ip: 198.18.193.2 }`, and `network_instance: "TRANSIT_VRF"`.
   * `NextHop#40`: IP-in-IP decap with `decapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4` and fallback to `network_instance: "DEFAULT"`.
   * `NextHop#100`: IP-in-IP encap with `encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4`, `ip_in_ip { src_ip: 198.51.100.1, dst_ip: 198.18.193.1 }`, and `network_instance: "TRANSIT_VRF"`.
   * `NextHopGroup#20` (backup NHG): `NextHop#20` with weight 1 (fallback to `REPAIR_VRF`).
   * `NextHopGroup#10` (primary transit NHG): `NextHop#1` with weight 1 and `backup_next_hop_group: 20`.
   * `NextHopGroup#40` (decap fallback NHG): `NextHop#40` with weight 1.
   * `NextHopGroup#30` (repair `decap + re-encap` NHG): `NextHop#30` with weight 1 and `backup_next_hop_group: 40`.
   * `NextHopGroup#50` (backup transit NHG): `NextHop#2` with weight 1 and `backup_next_hop_group: 40`.
   * `NextHopGroup#100` (ingress encap NHG): `NextHop#100` with weight 1.
   Program the following IPv4 prefix entries with `next_hop_group_network_instance: "DEFAULT"`:
   * In `DEFAULT`: IPv4 entry `192.0.2.0/24` pointing to `NextHopGroup#100`.
   * In `TRANSIT_VRF`: IPv4 entry `198.18.193.1/32` pointing to `NextHopGroup#10`, and IPv4 entry `198.18.193.2/32` pointing to `NextHopGroup#50`.
   * In `REPAIR_VRF`: IPv4 entry `198.18.193.1/32` pointing to `NextHopGroup#30`.
2. Send every AFT operation with `ack_type: RIB_AND_FIB_ACK` and verify that it returns `FIB_PROGRAMMED`.
3. Subscribe via gNMI to `.../afts/next-hop-groups/next-hop-group[id=10]/state/backup-active` and verify the initial value is `false`.
4. Send continuous IPv4 UDP/TCP traffic at a fixed rate of `1,000 packets per second (pps)` from ATE port-1 to `192.0.2.1`. Verify 100% egresses via ATE port-2 (encapsulated with outer destination `198.18.193.1`) with zero packet loss and no traffic egresses via ATE port-3.
5. Send a gRIBI `ModifyRequest` with `op: ADD`, `network_instance: "DEFAULT"`, `entry: backup_activate { next_hop_group: 10 }`, and `ack_type: RIB_AND_FIB_ACK`. Verify `ModifyResponse` returns `RIB_PROGRAMMED` and `FIB_PROGRAMMED`.
6. Verify the hardware forwarding ASIC switches `NextHopGroup#10` to its backup `NextHopGroup#20` (falling back to `REPAIR_VRF`, where `198.18.193.1/32` performs `decap + re-encap` to `198.18.193.2` in `TRANSIT_VRF` via `NextHopGroup#30`): 100% of traffic shifts to ATE port-3 (`NextHopGroup#50`) and traffic on ATE port-2 ceases. Estimate dataplane convergence time from packet loss: `Convergence Time (ms) = ((Tx Packets - Rx Packets) / Packet Rate (pps)) * 1000`. At `1,000 pps`, `300` dropped packets correspond to approximately `300 ms` of convergence time. Verify that convergence time is within the target `300 ms` threshold (allowing up to `350`–`400` dropped packets at `1,000 pps` for measurement tolerance).
7. Verify via gNMI (`Get` / `Subscribe`) that `.../afts/next-hop-groups/next-hop-group[id=10]/state/backup-active` reports `true`. Send a gRIBI `GetRequest` with `aft: BACKUP_ACTIVATE` or `all: {}` and verify the returned AFT entry contains `backup_activate: { next_hop_group: 10 }` with `rib_status: PROGRAMMED` and `fib_status: PROGRAMMED`.
8. Send a gRIBI `ModifyRequest` with `op: DELETE`, `network_instance: "DEFAULT"`, `entry: backup_activate { next_hop_group: 10 }`, and `ack_type: RIB_AND_FIB_ACK`. Verify `FIB_PROGRAMMED`, restoration to ATE port-2 within the `300 ms` convergence threshold (`<= 350`–`400` dropped packets at `1,000 pps`), and gNMI `backup-active: false`.

### TE-11.4.2: gRIBI BACKUP_ACTIVATE with FIB ACK and traffic failover (IPv6)

Validate dual-stack support by executing the `BACKUP_ACTIVATE` lifecycle for IPv6 routing and traffic forwarding with `REPAIR_VRF` fallback.

1. Using the `DEFAULT`, `TRANSIT_VRF`, and `REPAIR_VRF` tunnel hierarchy from TE-11.4.1 (or equivalent IPv6 next-hops `NextHop#11` on DUT port-2 and `NextHop#12` on DUT port-3 in `DEFAULT`, where primary `NextHopGroup#110` references `backup_next_hop_group: 120` configured to fall back to `REPAIR_VRF`, and `REPAIR_VRF` performs `decap + re-encap` into `TRANSIT_VRF` towards ATE port-3), program IPv6 entry `2001:db8:feed::/64` in `DEFAULT` pointing to the ingress/primary group (`NextHopGroup#100` / `NextHopGroup#110`).
2. Verify each AFT operation returns `FIB_PROGRAMMED`.
3. Send continuous IPv6 traffic at `1,000 pps` from ATE port-1 to `2001:db8:feed::1` and verify 100% egresses via ATE port-2.
4. Send a gRIBI `ModifyRequest` with `op: ADD`, `network_instance: "DEFAULT"`, `entry: backup_activate { next_hop_group: 10 }` (or `110`), and `ack_type: RIB_AND_FIB_ACK`. Verify `FIB_PROGRAMMED`, traffic shift to ATE port-3 via the `REPAIR_VRF` fallback path within the `300 ms` convergence threshold (`<= 350`–`400` dropped packets at `1,000 pps`), and gNMI `backup-active: true`.
5. Send a gRIBI `ModifyRequest` with `op: DELETE` for `backup_activate`. Verify `FIB_PROGRAMMED`, restoration to ATE port-2 within the `300 ms` convergence threshold, and telemetry `backup-active: false`.

### TE-11.4.3: Hierarchical encap and transit tunnel activation validation

Validate `BACKUP_ACTIVATE` behavior in hierarchical encapsulation and transit tunnel fast-reroute environments across all four testbed ports.

1. Configure the hierarchical forwarding pipeline via gRIBI. All `NextHop` (`301, 302, 303, 310, 401, 402, 403`) and `NextHopGroup` (`300, 310, 320, 400, 402, 410`) objects are defined in `DEFAULT`, while prefix routes are installed in `DEFAULT`, `TRANSIT_VRF`, and `REPAIR_VRF` with `next_hop_group_network_instance: "DEFAULT"`:
   * In `DEFAULT`, configure:
     * `NextHop#301`: IP-in-IP encap with `src_ip: 198.51.100.1`, `dst_ip: 198.18.193.1`, and `network_instance: "TRANSIT_VRF"`.
     * `NextHop#302`: IP-in-IP encap with `src_ip: 198.51.100.1`, `dst_ip: 198.18.193.2`, and `network_instance: "TRANSIT_VRF"`.
     * `NextHop#303`: IP-in-IP `decap + re-encap` (`decapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4`, `encapsulate_header: OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV4`) with `src_ip: 198.51.100.1`, `dst_ip: 198.18.193.1`, forwarding via DUT port-3 (`198.51.100.10`).
     * `NextHop#310`: VRF fallback with `network_instance: "REPAIR_VRF"`.
     * `NextHop#401`: egress via DUT port-2 to `198.51.100.6`.
     * `NextHop#402`: egress via DUT port-3 to `198.51.100.10`.
     * `NextHop#403`: egress via DUT port-4 to `198.51.100.14`.
     * `NextHopGroup#310` (backup VRF-fallback group in `DEFAULT`): contains `NextHop#310` (`network_instance: "REPAIR_VRF"`).
     * `NextHopGroup#300` (primary tunnel group in `DEFAULT`): contains `NextHop#301` with `backup_next_hop_group: 310`.
     * `NextHopGroup#320` (repair tunnel group in `DEFAULT`): contains `NextHop#302`.
     * `NextHopGroup#402` (repair transit group in `DEFAULT`): contains `NextHop#303` / `NextHop#402`.
     * `NextHopGroup#400` (primary transit group in `DEFAULT`): contains `NextHop#401` with `backup_next_hop_group: 310` (fallback to `REPAIR_VRF`).
     * `NextHopGroup#410` (backup transit group in `DEFAULT`): contains `NextHop#403`.
   * Configure prefix routes pointing back to the NextHopGroups in `DEFAULT`:
     * In `DEFAULT`: point `192.0.2.0/24` to `NextHopGroup#300`.
     * In `TRANSIT_VRF`: point `198.18.193.1/32` to `NextHopGroup#400` and `198.18.193.2/32` to `NextHopGroup#410`.
     * In `REPAIR_VRF`: point `198.18.193.1/32` to `NextHopGroup#402` (`decap + re-encap` towards ATE port-3) and `192.0.2.0/24` to `NextHopGroup#320` (re-encapsulating to `198.18.193.2/32` in `TRANSIT_VRF` towards ATE port-4).
2. Verify all entries are confirmed with `FIB_PROGRAMMED`.
3. Send traffic at `1,000 pps` from ATE port-1 to `192.0.2.1` and verify egress on ATE port-2 with an IP-in-IP outer destination of `198.18.193.1`.
4. Activate `backup_activate` for `NextHopGroup#400` with `op: ADD` and `ack_type: RIB_AND_FIB_ACK`. Verify `FIB_PROGRAMMED`, fallback to `REPAIR_VRF` and failover to `NextHopGroup#402` via ATE port-3 while retaining outer destination `198.18.193.1`, convergence within `300 ms` (`<= 350`–`400` dropped packets at `1,000 pps`), and `backup-active: true`.
5. Activate `backup_activate` for `NextHopGroup#300`. Verify `FIB_PROGRAMMED`, fallback to `REPAIR_VRF` and failover via `NextHopGroup#320` and `NextHopGroup#410` to ATE port-4 with outer destination `198.18.193.2` within the `300 ms` convergence threshold.
6. Delete `backup_activate` for `NextHopGroup#300` and `NextHopGroup#400`. Verify `FIB_PROGRAMMED` and restoration to the primary path on ATE port-2 within the `300 ms` convergence threshold.

### TE-11.4.4: Negative scenarios

1. Target non-existent `NextHopGroup#999999` with `op: ADD`, `network_instance: "DEFAULT"`, `entry: backup_activate { next_hop_group: 999999 }`, and `ack_type: RIB_AND_FIB_ACK`. Verify rejection with `FAILED`, with no forwarding-table or telemetry changes.
2. Target non-existent `NextHopGroup#999999` with `op: DELETE`, `network_instance: "DEFAULT"`, `entry: backup_activate { next_hop_group: 999999 }`, and `ack_type: RIB_AND_FIB_ACK`. Verify rejection with `FAILED`, with no forwarding-table or telemetry changes.
3. Program `NextHop#501` in `NextHopGroup#500` (in `DEFAULT`) without a `backup_next_hop_group`. Activate `backup_activate` for group 500 and verify `FAILED`; traffic must continue via `NextHop#501` without disruption.
4. In `DEFAULT`, program `NextHop#601` in `NextHopGroup#600` with `backup_next_hop_group: 610`, where group 610 contains `NextHop#602` configured with `network_instance: "REPAIR_VRF"`. Activate backup for group 600 and verify `FIB_PROGRAMMED`. Attempt to delete group 600 while backup activation remains active; verify dependency validation rejects the deletion (`FAILED`) without inconsistent forwarding state. Delete `backup_activate` first, then verify group 600 and `NextHop#601` can be deleted successfully.
   After deletion, verify via gNMI (`Get` / `Subscribe`) that `/network-instances/network-instance[name=DEFAULT]/afts/next-hop-groups/next-hop-group[id=600]/state/id` and `/network-instances/network-instance[name=DEFAULT]/afts/next-hops/next-hop[index=601]/state/index` are removed from operational AFT state, and verify via gRIBI `Get` that `NextHopGroup#600` and `NextHop#601` are absent. Re-program `NextHop#601` and `NextHopGroup#600` reusing the same IDs with `ack_type: RIB_AND_FIB_ACK` and verify `FIB_PROGRAMMED` to confirm the AFT identifiers and associated forwarding resources were cleanly reclaimed.

### TE-11.4.5: gRIBI process restart during active backup

Validate that a local device process restart while `BACKUP_ACTIVATE` is active reconciles the control-plane state without data-plane traffic loss or core dumps.

1. Program the IPv4 forwarding entries and primary/backup next-hop groups from TE-11.4.1, activate `backup_activate` for `NextHopGroup#10`, and verify `FIB_PROGRAMMED`, `backup-active: true`, and traffic forwarding via ATE port-3.
2. Start continuous IPv4 traffic at `1,000 pps` from ATE port-1 to `192.0.2.1`, record packet loss and forwarding counters, and trigger a restart of the device's local gRIBI process while preserving the DUT and ASIC state.
3. Verify the process returns to service and reconciles the persisted gRIBI and `BACKUP_ACTIVATE` state. The active backup remains installed, `backup-active` reports `true` after reconciliation, and traffic continues via ATE port-3 with zero packet loss.
4. Verify there are no core dumps, unexpected process crashes, stale or duplicate AFT entries, or inconsistent RIB/FIB status. Issue a gRIBI `Get` and confirm the active `backup_activate` entry is `PROGRAMMED` in both RIB and FIB.
5. Delete `backup_activate` and verify `FIB_PROGRAMMED`, `backup-active: false`, and recovery of traffic to the primary path on ATE port-2 within the `300 ms` convergence threshold.

### TE-11.4.6: High availability and Non-Stop Routing supervisor switchover

Validate that active `BACKUP_ACTIVATE` state is synchronized across redundant supervisors and that traffic remains on the backup path during a switchover.

1. Verify redundant controller cards report `PRIMARY` and `SECONDARY` through `/components/component/state/redundant-role`, and that the standby supervisor is ready.
2. Under continuous traffic at `1,000 pps`, activate `backup_activate` for `NextHopGroup#10`. Verify `FIB_PROGRAMMED`, `backup-active: true`, and forwarding via ATE port-3.
3. Issue the gNOI `system.System.SwitchControlProcessor` RPC and verify that the standby supervisor assumes the `PRIMARY` role.
4. Verify that traffic remains on the backup path with zero loss or only the vendor-documented sub-second NSR convergence loss, without reverting to the primary path.
5. Connect a gRIBI client to the new supervisor and issue `gRIBI.Get`. Verify the active trigger remains `PROGRAMMED` in both RIB and FIB and telemetry continues to report `backup-active: true`.
6. Delete the trigger from the new supervisor and verify `FIB_PROGRAMMED`, restoration to ATE port-2 within the `300 ms` convergence threshold, and `backup-active: false`.

### TE-11.4.7: Asynchronous late binding of backup next-hop groups

Validate that a backup NHG can be attached after primary forwarding is already active without disrupting traffic, and then immediately activated via `BACKUP_ACTIVATE`.

1. In `DEFAULT`, program `NextHop#701` (DUT port-2) and `NextHopGroup#700` without a backup reference, bind `198.18.193.1/32` in `TRANSIT_VRF` to group 700 (with `192.0.2.0/24` in `DEFAULT` encapsulating to `198.18.193.1` in `TRANSIT_VRF`), and verify `FIB_PROGRAMMED`, primary forwarding via ATE port-2, an unpopulated backup reference, and `backup-active: false`.
2. While traffic continues at `1,000 pps`, program `NextHop#702` (`network_instance: "REPAIR_VRF"`) and `NextHopGroup#720` in `DEFAULT` (with `198.18.193.1/32` in `REPAIR_VRF` performing `decap + re-encap` to `198.18.193.2/32` in `TRANSIT_VRF` towards ATE port-3), then update group 700 to add `backup_next_hop_group: 720`. Verify `FIB_PROGRAMMED`, zero traffic loss on the active primary path, telemetry showing backup group 720, and `backup-active: false`.
3. Activate `backup_activate` for group 700 and verify `FIB_PROGRAMMED`, traffic shift to ATE port-3 within the `300 ms` convergence threshold (`<= 350`–`400` dropped packets at `1,000 pps`), and `backup-active: true`.
4. Delete the trigger and verify `FIB_PROGRAMMED`, restoration to ATE port-2 within the `300 ms` convergence threshold, and `backup-active: false`.

## Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "name": "Ethernet1/1",
        "config": {"name": "Ethernet1/1", "enabled": true, "type": "iana-if-type:ethernetCsmacd"},
        "subinterfaces": {"subinterface": [{"index": 0, "config": {"index": 0, "enabled": true}, "ipv4": {"addresses": {"address": [{"ip": "198.51.100.1", "config": {"ip": "198.51.100.1", "prefix-length": 30}}]}}, "ipv6": {"addresses": {"address": [{"ip": "2001:db8:1::1", "config": {"ip": "2001:db8:1::1", "prefix-length": 126}}]}}}]}
      },
      {
        "name": "Ethernet1/2",
        "config": {"name": "Ethernet1/2", "enabled": true, "type": "iana-if-type:ethernetCsmacd"},
        "subinterfaces": {"subinterface": [{"index": 0, "config": {"index": 0, "enabled": true}, "ipv4": {"addresses": {"address": [{"ip": "198.51.100.5", "config": {"ip": "198.51.100.5", "prefix-length": 30}}]}}, "ipv6": {"addresses": {"address": [{"ip": "2001:db8:2::1", "config": {"ip": "2001:db8:2::1", "prefix-length": 126}}]}}}]}
      },
      {
        "name": "Ethernet1/3",
        "config": {"name": "Ethernet1/3", "enabled": true, "type": "iana-if-type:ethernetCsmacd"},
        "subinterfaces": {"subinterface": [{"index": 0, "config": {"index": 0, "enabled": true}, "ipv4": {"addresses": {"address": [{"ip": "198.51.100.9", "config": {"ip": "198.51.100.9", "prefix-length": 30}}]}}, "ipv6": {"addresses": {"address": [{"ip": "2001:db8:3::1", "config": {"ip": "2001:db8:3::1", "prefix-length": 126}}]}}}]}
      },
      {
        "name": "Ethernet1/4",
        "config": {"name": "Ethernet1/4", "enabled": true, "type": "iana-if-type:ethernetCsmacd"},
        "subinterfaces": {"subinterface": [{"index": 0, "config": {"index": 0, "enabled": true}, "ipv4": {"addresses": {"address": [{"ip": "198.51.100.13", "config": {"ip": "198.51.100.13", "prefix-length": 30}}]}}, "ipv6": {"addresses": {"address": [{"ip": "2001:db8:4::1", "config": {"ip": "2001:db8:4::1", "prefix-length": 126}}]}}}]}
      }
    ]
  },
  "network-instances": {
    "network-instance": [
      {
        "name": "DEFAULT",
        "config": {"name": "DEFAULT", "type": "openconfig-network-instance-types:DEFAULT_INSTANCE"},
        "interfaces": {"interface": [
          {"id": "Ethernet1/1.0", "config": {"id": "Ethernet1/1.0", "interface": "Ethernet1/1", "subinterface": 0}},
          {"id": "Ethernet1/2.0", "config": {"id": "Ethernet1/2.0", "interface": "Ethernet1/2", "subinterface": 0}},
          {"id": "Ethernet1/3.0", "config": {"id": "Ethernet1/3.0", "interface": "Ethernet1/3", "subinterface": 0}},
          {"id": "Ethernet1/4.0", "config": {"id": "Ethernet1/4.0", "interface": "Ethernet1/4", "subinterface": 0}}
        ]}
      },
      {"name": "TRANSIT_VRF", "config": {"name": "TRANSIT_VRF", "type": "openconfig-network-instance-types:L3VRF"}},
      {"name": "REPAIR_VRF", "config": {"name": "REPAIR_VRF", "type": "openconfig-network-instance-types:L3VRF"}}
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /interfaces/interface/config/name:
  /interfaces/interface/config/type:
  /interfaces/interface/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/config/index:
  /interfaces/interface/subinterfaces/subinterface/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /network-instances/network-instance/config/name:
  /network-instances/network-instance/config/type:
  /network-instances/network-instance/interfaces/interface/config/id:
  /network-instances/network-instance/interfaces/interface/config/interface:
  /network-instances/network-instance/interfaces/interface/config/subinterface:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/subinterfaces/subinterface/state/oper-status:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group-network-instance:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group-network-instance:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/backup-next-hop-group:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/weight:
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address:
  /network-instances/network-instance/afts/next-hops/next-hop/state/mac-address:
  /network-instances/network-instance/afts/next-hops/next-hop/interface-ref/state/interface:
  /network-instances/network-instance/afts/next-hops/next-hop/interface-ref/state/subinterface:
  /network-instances/network-instance/afts/next-hops/next-hop/state/network-instance:
  /network-instances/network-instance/afts/next-hops/next-hop/state/decapsulate-header:
  /network-instances/network-instance/afts/next-hops/next-hop/state/encapsulate-header:
  /network-instances/network-instance/afts/next-hops/next-hop/ip-in-ip/state/src-ip:
  /network-instances/network-instance/afts/next-hops/next-hop/ip-in-ip/state/dst-ip:
  /components/component/state/redundant-role:
    platform_type: ["CONTROLLER_CARD"]
  # TODO: Proposed OpenConfig paths for backup activation state and structural list.
  # See https://github.com/openconfig/public/pull/1541
  # /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/backup-active
  # /network-instances/network-instance/afts/backup-activate/backup-activate/state/next-hop-group
rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
      on_change: true
    gNMI.Get:
  gribi:
    gRIBI.Modify:
    gRIBI.Flush:
    gRIBI.Get:
  gnoi:
    system.System.KillProcess:
    system.System.SwitchControlProcessor:
```

## Minimum DUT Platform Requirement

MFF for subtests requiring redundant controller-card switchover (TE-11.4.6); vRX or FFF for all other subtests.
