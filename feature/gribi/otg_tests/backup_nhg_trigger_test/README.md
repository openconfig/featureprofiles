# TE-11.4: gRIBI BACKUP_ACTIVATE Hardware Convergence & FIB ACK Verification

## Summary

Validate that the software-driven/external gRIBI `BACKUP_ACTIVATE` AFT operation triggers immediate dataplane failover to the configured `backup_next_hop_group`, and verify that the `FIB_PROGRAMMED` status in `ModifyResponse` strictly correlates with committed hardware ASIC path switching with zero traffic loss during failover and recovery. In addition, validate telemetry streaming on operational state leaves (`backup-active`) only after hardware FIB programming completes, query feedback via the gRIBI `Get` RPC, graceful restoration upon deletion (`op: DELETE`), dual-stack (IPv4 and IPv6) forwarding behaviors, hierarchical tunnel group activation across multiple egress trunks, and error handling for invalid requests, including non-existent groups, groups lacking a backup path, and deletion of an active NHG with backup enabled.

## Topology

```text
+-------------------+                 +-------------------+
|                   |                 |                   |
|                   |  DUT Port 1     |  ATE Port 1       |
|                   |=================|  (Ingress Source) |
|                   |                 |                   |
|                   |  DUT Port 2     |  ATE Port 2       |
|      DUT          |=================|  (Primary Trunk1) ||
|                   |                 |                   |
|                   |  DUT Port 3     |  ATE Port 3       |
|                   |=================|  (Backup Trunk2)  |
|                   |                 |                   |
|                   |  DUT Port 4     |  ATE Port 4       |
|                   |=================|  (Backup TG Path) |
|                   |                 |                   |
+-------------------+                 +-------------------+
        ATE                                 DUT
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
  * Transit tunnel destination IPs: `10.40.193.1/32`, `10.40.193.2/32`
* Network instances / VRFs:
  * `DEFAULT`
  * `TRANSIT_VRF` (non-default VRF for transit route resolution)

## Testbed Type

`TESTBED_DUT_ATE_4LINKS`

## Procedure

### Test environment setup

1. Configure the DUT and ATE interfaces according to the topology with IPv4 and IPv6 addressing.
2. Ensure that DUT ports 1 through 4 are operationally `UP` at `/interfaces/interface/state/oper-status`.
3. Establish a gRIBI client connection with the DUT using `persistence = PRESERVE` and `redundancy = SINGLE_PRIMARY`.
4. Negotiate the election ID and establish leadership.
5. Send a gRIBI `Flush` RPC targeting network instances `DEFAULT` and `TRANSIT_VRF` to clear stale forwarding entries.

### TE-11.4.1: gRIBI BACKUP_ACTIVATE with FIB ACK and traffic failover (IPv4)

Validate that issuing a gRIBI `BACKUP_ACTIVATE` operation on a primary next-hop group switches IPv4 traffic to its configured backup next-hop group with zero traffic loss, returns `FIB_PROGRAMMED`, updates telemetry only after FIB commit, and restores primary-path forwarding upon deletion without loss.

1. Program the following in network instance `DEFAULT` via gRIBI:
   * `NextHop#1`: egress interface DUT port-2, next-hop IP `198.51.100.6`, MAC `00:1A:11:00:00:01`.
   * `NextHop#2`: egress interface DUT port-3, next-hop IP `198.51.100.10`, MAC `00:1A:11:00:00:02`.
   * `NextHopGroup#20` (backup NHG): `NextHop#2` with weight 1.
   * `NextHopGroup#10` (primary NHG): `NextHop#1` with weight 1 and `backup_next_hop_group: 20`.
   * IPv4 entry `192.0.2.0/24` pointing to `NextHopGroup#10`.
2. Send every AFT operation with `ack_type: RIB_AND_FIB_ACK` and verify that it returns `FIB_PROGRAMMED`.
3. Subscribe via gNMI to `.../afts/next-hop-groups/next-hop-group[id=10]/state/backup-active` and verify the initial value is `false`.
4. Send continuous IPv4 UDP/TCP traffic from ATE port-1 to `192.0.2.1`. Verify 100% egresses via ATE port-2 with zero packet loss and no traffic egresses via ATE port-3.
5. Send a gRIBI `ModifyRequest` with `op: ADD`, `network_instance: "DEFAULT"`, `entry: backup_activate { next_hop_group: 10 }`, and `ack_type: RIB_AND_FIB_ACK`. Verify `ModifyResponse` returns `RIB_PROGRAMMED` and `FIB_PROGRAMMED`, and record that the operation completes within the expected O(ms) convergence window.
6. Verify the hardware forwarding ASIC switches traffic to `NextHopGroup#20`: 100% shifts to ATE port-3, traffic on ATE port-2 ceases, and zero traffic is lost during the transition.
7. Verify gNMI reports `backup-active: true` only after both RIB and FIB programming have completed. Send a gRIBI `GetRequest` with `aft: BACKUP_ACTIVATE` or `all: {}` and verify the returned AFT entry contains `backup_activate: { next_hop_group: 10 }` with `rib_status: PROGRAMMED` and `fib_status: PROGRAMMED`.
8. Send a gRIBI `ModifyRequest` with `op: DELETE`, `network_instance: "DEFAULT"`, `entry: backup_activate { next_hop_group: 10 }`, and `ack_type: RIB_AND_FIB_ACK`. Verify `FIB_PROGRAMMED`, restoration to ATE port-2 with zero loss, and `backup-active: false` only after hardware FIB restoration is committed.

### TE-11.4.2: gRIBI BACKUP_ACTIVATE with FIB ACK and traffic failover (IPv6)

Validate dual-stack support by executing the `BACKUP_ACTIVATE` lifecycle for IPv6 routing and traffic forwarding with hitless failover and recovery.

1. Program the following in `DEFAULT` via gRIBI:
   * `NextHop#11`: egress interface DUT port-2, next-hop IP `2001:db8:2::2`, MAC `00:1A:11:00:00:01`.
   * `NextHop#12`: egress interface DUT port-3, next-hop IP `2001:db8:3::2`, MAC `00:1A:11:00:00:02`.
   * `NextHopGroup#120` (backup NHG): `NextHop#12` with weight 1.
   * `NextHopGroup#110` (primary NHG): `NextHop#11` with weight 1 and `backup_next_hop_group: 120`.
   * IPv6 entry `2001:db8:feed::/64` pointing to `NextHopGroup#110`.
2. Verify each AFT operation returns `FIB_PROGRAMMED`.
3. Send continuous IPv6 traffic from ATE port-1 to `2001:db8:feed::1` and verify 100% egresses via ATE port-2.
4. Send a gRIBI `ModifyRequest` with `op: ADD`, `entry: backup_activate { next_hop_group: 110 }`, and `ack_type: RIB_AND_FIB_ACK`. Verify `FIB_PROGRAMMED`, a zero-loss shift to ATE port-3, and gNMI `backup-active: true` after FIB programming.
5. Send a gRIBI `ModifyRequest` with `op: DELETE` for `backup_activate` on `NextHopGroup#110`. Verify `FIB_PROGRAMMED`, zero-loss restoration to ATE port-2, and telemetry `backup-active: false`.

### TE-11.4.3: Hierarchical encap and transit tunnel activation validation

Validate `BACKUP_ACTIVATE` behavior in hierarchical encapsulation and transit tunnel fast-reroute environments across all four testbed ports.

1. Configure the hierarchical forwarding pipeline via gRIBI:
   * In `DEFAULT`, configure `NextHop#301` as IP-in-IP with `src_ip: 198.51.100.1`, `dst_ip: 10.40.193.1`, and `network_instance: TRANSIT_VRF`; configure `NextHop#302` similarly with `dst_ip: 10.40.193.2`.
   * In `DEFAULT`, configure `NextHopGroup#310` (backup tunnel group) containing `NextHop#302`, and `NextHopGroup#300` (primary tunnel group) containing `NextHop#301` with `backup_next_hop_group: 310`. Point IPv4 entry `192.0.2.0/24` to `NextHopGroup#300`.
   * In `TRANSIT_VRF`, configure `NextHop#401` via DUT port-2 to `198.51.100.6`, `NextHop#402` via DUT port-3 to `198.51.100.10`, and `NextHop#403` via DUT port-4 to `198.51.100.14`.
   * Configure `NextHopGroup#402` containing `NextHop#402`, `NextHopGroup#400` containing `NextHop#401` with `backup_next_hop_group: 402`, and `NextHopGroup#410` containing `NextHop#403`. Point `10.40.193.1/32` to `NextHopGroup#400` and `10.40.193.2/32` to `NextHopGroup#410`.
2. Verify all entries are confirmed with `FIB_PROGRAMMED`.
3. Send traffic from ATE port-1 to `192.0.2.1` and verify egress on ATE port-2 with an IP-in-IP outer destination of `10.40.193.1`.
4. Activate `backup_activate` for `NextHopGroup#400` with `op: ADD` and `ack_type: RIB_AND_FIB_ACK`. Verify `FIB_PROGRAMMED`, failover to `NextHopGroup#402` via ATE port-3 while retaining outer destination `10.40.193.1`, zero loss, and `backup-active: true`.
5. Activate `backup_activate` for `NextHopGroup#300`. Verify `FIB_PROGRAMMED`, failover to `NextHopGroup#310` via ATE port-4, and outer destination `10.40.193.2` with zero loss.
6. Delete `backup_activate` for `NextHopGroup#300` and `NextHopGroup#400`. Verify `FIB_PROGRAMMED` and zero-loss restoration to the primary path on ATE port-2.

### TE-11.4.4: Negative scenarios

1. Target non-existent `NextHopGroup#999999` with `op: ADD`, `network_instance: "DEFAULT"`, `entry: backup_activate { next_hop_group: 999999 }`, and `ack_type: RIB_AND_FIB_ACK`. Verify rejection with `FIB_FAILED` or `RIB_FAILED`, with no forwarding-table or telemetry changes.
2. Program `NextHop#501` in `NextHopGroup#500` without a `backup_next_hop_group`. Activate `backup_activate` for group 500 and verify `FIB_FAILED` or `RIB_FAILED`; traffic must continue via `NextHop#501` without disruption.
3. Program `NextHop#601` in `NextHopGroup#600` with `backup_next_hop_group: 610`, where group 610 contains `NextHop#602`. Activate backup for group 600 and verify `FIB_PROGRAMMED`. Attempt to delete group 600 while backup activation remains active; verify dependency validation rejects the deletion without orphaned or inconsistent hardware state. Delete `backup_activate` first, then verify group 600 can be deleted successfully.

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
      {"name": "TRANSIT_VRF", "config": {"name": "TRANSIT_VRF", "type": "openconfig-network-instance-types:L3VRF"}}
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
  /network-instances/network-instance/afts/next-hops/next-hop/ip-in-ip/state/src-ip:
  /network-instances/network-instance/afts/next-hops/next-hop/ip-in-ip/state/dst-ip:
  # TODO: Proposed OpenConfig paths for backup activation state and structural list.
  # See https://github.com/openconfig/public/pull/1541
  # /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/backup-active
  # /network-instances/network-instance/afts/backup-activate/backup-activate/state/next-hop-group
rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
    gNMI.Get:
  gribi:
    gRIBI.Modify:
    gRIBI.Flush:
    gRIBI.Get:
```

## Minimum DUT Platform Requirement

vRX if the vendor implementation supports FIB-ACK simulation, otherwise FFF.
