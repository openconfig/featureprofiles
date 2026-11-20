# P4RT-3.5: P4RT Packet-I/O Isolation under gRIBI FIB Exhaustion

## Summary

This test verifies that exhausting the gRIBI-programmable hardware FIB does not
disrupt P4Runtime (`P4RT`) Packet-I/O, P4RT punt-entry programming, or
forwarding through entries that were programmed before the exhaustion, and that
normal programming resumes once the scale entries are removed. It verifies
that:

* Once the FIB is exhausted, `gRIBI.Modify` reports the failure in-band as
  `AFTResult.status = FIB_FAILED` (with `error_details` populated) on a
  `Modify` stream that stays open. Every earlier operation is `FIB_PROGRAMMED`
  and the DUT never returns a gRPC error status or closes the stream.
* While the FIB is exhausted, the P4RT `StreamChannel` stays open, LLDP and
  IPv4/IPv6 traceroute packets that match the WBB `acl_wbb_ingress_table` punt
  entries are delivered as `PacketIn` messages with correct metadata and
  without loss, and every `PacketOut` frame is transmitted on the requested
  port.
* While the FIB is exhausted, WBB punt entries can still be deleted and
  re-inserted with `P4Runtime.Write`, a duplicate `INSERT` is rejected with
  `ALREADY_EXISTS`, and `P4Runtime.Read` returns exactly the installed entries.
* Protected gRIBI entries programmed before the exhaustion forward with `0%`
  loss throughout. Programmed scale entries keep forwarding, and the entry that
  returned `FIB_FAILED` does not forward.
* After the scale entries are removed, the chain that returned `FIB_FAILED` is
  programmed (`FIB_PROGRAMMED`) and forwards, and P4RT programming and
  Packet-I/O remain functional.
* With all eight WBB punt entries installed, gRIBI programming and forwarding
  are unaffected.
* On platforms that report the FIB utilization resource, utilization telemetry
  is consistent with the FIB state (telemetry-only checks).

**Scope and per-API error semantics.** The only P4RT-programmable object in the
WBB pipeline (`feature/p4rt/data/wbb.p4info.pb.txt`) is the ingress punt ACL
`acl_wbb_ingress_table` (`size = 8`: LLDP, GDP and IPv4/IPv6 traceroute
entries). Eight punt entries cannot exhaust the LPM, next-hop and
next-hop-group resources that gRIBI consumes, and the punt table is expected to
be allocated separately from them. This test therefore verifies isolation
rather than cross-API exhaustion: P4RT Packet-I/O and punt-entry programming
must keep working while the gRIBI-programmable FIB is exhausted. The generic
expectation that "the other API receives `RESOURCE_EXHAUSTED`" maps to each API
as follows:

* gRIBI: FIB exhaustion is reported per operation as `AFTResult.status =
  FIB_FAILED` on an open `Modify` stream, never as a gRPC `RESOURCE_EXHAUSTED`
  status (gRIBI specification).
* P4RT: a table-capacity rejection is reported as a per-update `p4.v1.Error`
  with `canonical_code = RESOURCE_EXHAUSTED` inside an outer gRPC `UNKNOWN`
  status. The P4Info table `size` is the requested capacity, not a hard
  maximum (P4Runtime specification), so this is checked only in the optional
  subtest P4RT-3.5.6.

**Relationship to TE-9.3, gNMI-1.21 and P4RT-3.4.** TE-9.3 already verifies
`FIB_FAILED` on hardware exhaustion, gNMI-1.21 verifies integrated-circuit
utilization thresholds, and P4RT-3.4 verifies `PacketIn` during gRIBI
programming. This test reuses the TE-9.3 FIB pre-fill
(`max_out_fib_route_count`) and hierarchical chain structure, and the gNMI-1.21
per-platform FIB resource names and deviations. It adds only the P4RT
dimension: Packet-I/O and WBB punt-entry programming while the FIB is
exhausted.

## Testbed type

* [`TESTBED_DUT_ATE_4LINKS`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

```text
                              +-------+
  [ ATE port1 ] ------------- |       | ------------- [ ATE port2 ] (Protected egress)
  (Traffic and PacketIn src)  |  DUT  |
                              |       | ------------- [ ATE port3 ] (PacketOut egress)
                              |       |
                              |       | ------------- [ ATE port4 ] (Scale egress, BGP pre-fill peer)
                              +-------+
```

* Connect 4 interfaces between the DUT and ATE and configure dual-stack IPv4 and
  IPv6 addresses in the `DEFAULT` network instance:
  * **Link 1 (traffic and punt ingress)**: DUT `port1` (`192.0.2.1/30`,
    `2001:db8:1::1/126`, P4RT `id = 1`) <-> ATE `port1` (`192.0.2.2/30`,
    `2001:db8:1::2/126`).
  * **Link 2 (protected egress)**: DUT `port2` (`192.0.2.5/30`,
    `2001:db8:2::1/126`, P4RT `id = 2`) <-> ATE `port2` (`192.0.2.6/30`,
    `2001:db8:2::2/126`).
  * **Link 3 (PacketOut egress)**: DUT `port3` (`192.0.2.9/30`,
    `2001:db8:3::1/126`, P4RT `id = 3`) <-> ATE `port3` (`192.0.2.10/30`,
    `2001:db8:3::2/126`). No routed test traffic uses this link.
  * **Link 4 (scale egress and BGP pre-fill peer)**: DUT `port4`
    (`192.0.2.13/30`, `2001:db8:4::1/126`, P4RT `id = 4`) <-> ATE `port4`
    (`192.0.2.14/30`, `2001:db8:4::2/126`).
* Verify all 4 DUT interfaces report `oper-status = UP` via `gNMI.Subscribe`
  (`ON_CHANGE`) on `/interfaces/interface/state/oper-status`.
* **Single integrated circuit (P4RT node) assumption**: resolve the integrated
  circuit that hosts DUT `port1`, `port2` and `port3` as the existing P4RT
  tests do: `/interfaces/interface/state/hardware-port` gives the port
  component, whose `/components/component/state/parent` is a component with
  `/components/component/state/type = INTEGRATED_CIRCUIT`. All three ports must
  resolve to the same integrated circuit (`P4RT_NODE` below), otherwise setup
  fails with an explicit testbed error. Via `gNMI.Set`, set
  `/components/component/integrated-circuit/config/node-id = 1` on `P4RT_NODE`
  (P4RT `device_id = 1`) and `/interfaces/interface/config/id` `1`-`4` on DUT
  `port1`-`port4`.
* Record the current value of `/lldp/config/enabled` and set it to `false` via
  `gNMI.Set`, so that LLDP frames are punted to the P4RT client instead of being
  consumed by the on-box LLDP agent (as in P4RT-7.1). The recorded value is
  restored in Cleanup.
* Establish a P4RT client session (`device_id = 1`, `election_id = {high: 0,
  low: 1}`) and become primary via `MasterArbitrationUpdate` on
  `P4Runtime.StreamChannel`.
* Record the pre-test P4RT state before changing it, because the pipeline push
  below clears the forwarding state: read the current pipeline with
  `P4Runtime.GetForwardingPipelineConfig` (`response_type = ALL`) as `P_pre`
  (empty if no pipeline is configured) and, if a pipeline is present, read all
  entries of `acl_wbb_ingress_table` with `P4Runtime.Read` as `E_pre`. Const
  entries (`is_const = true`, for example a `const default_action`) belong to
  the pipeline, cannot be deleted or re-inserted, and are excluded from `E_pre`
  and from every entry count in this test.
* Push the WBB pipeline (`feature/p4rt/data/wbb.p4info.pb.txt`) via
  `P4Runtime.SetForwardingPipelineConfig` (`action = VERIFY_AND_COMMIT`), which
  clears the forwarding state. If `P4Runtime.Read` still returns non-const
  entries in `acl_wbb_ingress_table` (for example entries installed by the
  platform), add them to `E_pre` and delete them with `P4Runtime.Write`
  (`DELETE`). Confirm that `P4Runtime.Read` returns `0` non-const entries.
  `P_pre` and `E_pre` are restored in Cleanup.
* Establish a gRIBI client session with `SessionParameters` `redundancy =
  SINGLE_PRIMARY`, `persistence = PRESERVE` and `ack_type = RIB_AND_FIB_ACK`
  (`election_id = {high: 0, low: 1}`), and flush `DEFAULT` via `gRIBI.Flush`.
  Unless a step states otherwise, every gRIBI operation in this test must be
  acknowledged with `AFTResult.status = FIB_PROGRAMMED` (see the
  `unreferenced_aft_fib_ack_unsupported` row in
  [Parameters and platform gating](#parameters-and-platform-gating) for the
  only NH/NHG exceptions).
* Configure the TE-9.3 pre-fill BGP session in `DEFAULT`: DUT AS `64500`,
  `router-id = 192.0.2.13`, eBGP neighbor `2001:db8:4::2` (ATE `port4`, AS
  `64501`) with `IPV6_UNICAST` enabled and import policy `ALLOW`
  (`ACCEPT_ROUTE`), without a maximum-prefix limit (platforms that apply a
  default prefix limit must remove it, as in TE-9.3). The neighbor is
  configured with `enabled = false` and is enabled only in P4RT-3.5.4, so the
  pre-fill routes are present only while the FIB must be exhausted. The ATE BGP
  peer uses the OTG default IP TTL / hop limit of `64`, so its packets never
  match the WBB traceroute entries (`ttl` `0`-`2`).
* *Telemetry (optional)*: on platforms that report `FIB_RESOURCE_NAME` (see
  [Parameters and platform gating](#parameters-and-platform-gating)), configure
  `used-threshold-upper = 80` and `used-threshold-upper-clear = 70` under
  `/system/utilization/resources/resource/config/` for `FIB_RESOURCE_NAME` via
  `gNMI.Set`. Subscribe (`SAMPLE`, `10 s`) to `used`, `free`, `max-limit`,
  `high-watermark` and `used-threshold-upper-exceeded` of that resource under
  `/components/component/integrated-circuit/utilization/resources/resource/state/`
  on every active `INTEGRATED_CIRCUIT` component, as gNMI-1.21 does.
* Subscribe (`ON_CHANGE`) to the protected AFT entries only: the IPv4 entry
  `198.51.100.0/24`, the IPv6 entry `2001:db8:100::/64`, the next-hop-groups
  they reference and the next hops of those groups. Do not subscribe to the
  whole `/network-instances/network-instance/afts/` tree, which would stream
  every scale entry and load both the DUT and the test harness.
* Push one OTG configuration that is kept for the whole test (until
  P4RT-3.5.5 Step 5): devices on ATE `port1`-`port4`, the ATE `port4` BGP peer
  (AS `64501`) with the `R`-prefix pre-fill route range, an OTG capture on ATE
  `port3`, and every flow of
  [Traffic flows and Packet-I/O frames](#traffic-flows-and-packet-io-frames)
  except `Flow-Chain-F`. Start protocols and wait for IPv4 ARP and IPv6 ND
  resolution on all 4 links before any traffic is started. Flows are started
  and stopped individually; the configuration is not pushed again while the
  pre-fill routes are needed, because a new push restarts the ATE protocols.

#### Parameters and platform gating

| Name | Value |
|---|---|
| `R` | Pre-fill route count: the per-platform `max_out_fib_route_count` deviation (`2,500,000` when not set). This test's `metadata.textproto` has no platform exceptions yet; the automation must add `max_out_fib_route_count` platform exceptions with the same per-platform values as TE-9.3 (currently `2,500,000`, `2,600,000` or `3,750,000` depending on the platform). |
| `FIB_RESOURCE_NAME` | The per-platform FIB resource name used by gNMI-1.21 (`feature/platform/integrated_circuit/otg_tests/utilization_test`). It must match a name reported under `/components/component/integrated-circuit/utilization/resources/resource/state/name`. It is a placeholder in the Canonical OC. |
| `B` | `10` chains (`60` gRIBI operations) per exhaustion batch. |
| `C_max` | `1,000` chains: hard cap of the exhaustion loop (TE-9.3 uses `500`). `R` must be calibrated per platform so that `FIB_FAILED` is reached within `C_max` chains after the pre-fill; otherwise P4RT-3.5.4 fails with an explicit configuration error. `C_max` cannot exceed `1,022` without widening the `Flow-Scale` egress tag. |
| `T_batch` | `60 s` to receive the `AFTResult` of every operation in a batch. |
| `T_max` | `30 min` for the whole exhaustion loop. |
| `T_prefill` | `30 min` for the pre-fill routes to be received or removed. |

| Platform condition | Behavior |
|---|---|
| `gribi_riback_only` is set | `FIB_FAILED` cannot be reported: P4RT-3.5.4 and P4RT-3.5.5 are skipped. The session uses `ack_type = RIB_ACK` and the other subtests accept `RIB_PROGRAMMED` in place of `FIB_PROGRAMMED`. |
| `unreferenced_aft_fib_ack_unsupported` is set | NH/NHG operations that are unreferenced when the DUT processes them may end with `RIB_PROGRAMMED` and no FIB acknowledgement. `RIB_PROGRAMMED` is accepted as the final status for exactly these operations: the NH/NHG `DELETE`s in P4RT-3.5.3 Step 4 and P4RT-3.5.5 Steps 2 and 10, and the NH/NHG `ADD`s in the exhaustion batch that contains the first `FIB_FAILED` (P4RT-3.5.4 Step 4). Their state is confirmed with `gRIBI.Get` instead (deleted entries absent, added entries present), and for IP entries also with `gNMI.Get` of the AFT prefix. All other NH/NHG `ADD`s are referenced within their own batch (every batch carries complete chains) and must be `FIB_PROGRAMMED`. |
| `p4rt_modify_table_entry_unsupported` is set | Each `MODIFY` in P4RT-3.5.5 is replaced by a `DELETE` followed by an `INSERT` of the entry with the new action. |
| `subnet_mask_change_required` is set | Pre-fill prefixes are `/120` instead of `/128` (as in TE-9.3). |
| `use_chassis_aggregate_utilization` or `mismatched_hardware_resource_name_in_component` is set | Utilization is read as in gNMI-1.21: `used` and `free` are summed over the chassis FIB resources (only the `used`-based checks apply), or the component resource name differs from the system resource name. |
| `FIB_RESOURCE_NAME` is not reported | Steps marked *Telemetry (optional)* are skipped and logged. |
| `p4rt_table_size_hard_limit_enforced` is not set (default) | P4RT-3.5.6 is skipped. This per-platform opt-in flag (a metadata flag or test argument, to be added with the automation) is set only for platforms that enforce the P4Info table `size` as a hard limit. |

#### WBB punt entries

All entries are written to `acl_wbb_ingress_table` with `priority = 1`, action
`acl_wbb_ingress_trap` (except during the gated `MODIFY` in P4RT-3.5.5) and
`Atomicity = CONTINUE_ON_ERROR`. A `P4Runtime.Write` "returns `OK`" means the
RPC status is `OK`, i.e. every update in the request succeeded.

| Entry | Match fields (value & mask) | Use |
|---|---|---|
| `E-LLDP` | `ether_type == 0x88cc & 0xffff` | Protected LLDP punt entry; installed in P4RT-3.5.1 and kept until Cleanup. |
| `E-GDP` | `ether_type == 0x6007 & 0xffff` | GDP punt entry (no GDP traffic is sent). |
| `E-T4-0`, `E-T4-1`, `E-T4-2` | `is_ipv4 == 0x1`, `ttl == 0x00` / `0x01` / `0x02 & 0xff` | IPv4 traceroute punt entries. |
| `E-T6-0`, `E-T6-1`, `E-T6-2` | `is_ipv6 == 0x1`, `ttl == 0x00` / `0x01` / `0x02 & 0xff` | IPv6 traceroute punt entries. |
| `E-N1` | `ether_type == 0x6007 & 0xffff`, `outer_vlan_id == 0xfa0 & 0xfff` | Optional P4RT-3.5.6 only: a ninth distinct key. |

`E-SEVEN` denotes the seven non-protected entries `E-GDP`, `E-T4-0`-`E-T4-2`
and `E-T6-0`-`E-T6-2`. All nine entries satisfy the table's
`@entry_restriction`, which constrains the match values of each entry, not the
number of entries.

#### Traffic flows and Packet-I/O frames

| Flow | Path | Packet | Rate and size |
|---|---|---|---|
| `Flow-Protected-v4` | ATE `port1` -> ATE `port2` | IPv4 `192.0.2.2` -> `198.51.100.1`, `TTL = 64` | `5,000 fps`, `512` bytes |
| `Flow-Protected-v6` | ATE `port1` -> ATE `port2` | IPv6 `2001:db8:1::2` -> `2001:db8:100::1`, `hop-limit = 64` | `5,000 fps`, `512` bytes |
| `Flow-LLDP-In` | ATE `port1` (punted) | Ethernet `02:00:35:01:00:01` -> `01:80:c2:00:00:0e`, `ether_type = 0x88cc` | `10 fps`, `128` bytes |
| `Flow-Trace-v4` | ATE `port1` (punted) | IPv4 `192.0.2.2` -> `198.51.100.1`, `TTL = 1` | `100` packets at `10 pps`, `128` bytes |
| `Flow-Trace-v6` | ATE `port1` (punted) | IPv6 `2001:db8:1::2` -> `2001:db8:100::1`, `hop-limit = 1` | `100` packets at `10 pps`, `128` bytes |
| `Flow-Routes-v4` | ATE `port1` -> ATE `port4` | IPv4 to `100.64.10.1`-`100.64.10.100` (increment, count `100`), `TTL = 64` | `1,000 fps`, `512` bytes |
| `Flow-Routes-v6` | ATE `port1` -> ATE `port4` | IPv6 to `2001:db8:3000::1`-`2001:db8:3000::64` (increment, count `100`), `hop-limit = 64` | `1,000 fps`, `512` bytes |
| `Flow-Sentinel` | ATE `port1` -> ATE `port4` | IPv4 to `DstIP_0` (`100.65.0.1`), `TTL = 64` | `1,000 fps`, `512` bytes |
| `Flow-Scale` | ATE `port1` -> ATE `port4` | IPv4 to `DstIP_0`-`DstIP_1000` (`100.65.0.1`-`100.65.3.233`, increment, count `1,001`), `TTL = 64`, with OTG egress tracking (metric tag) on the last `10` bits of the destination address as in TE-9.3; the tag value of `DstIP_i` is `i + 1` | `1,000 fps`, `512` bytes |
| `Flow-Chain-F` | ATE `port1` -> ATE `port4` | IPv4 to `DstIP_F`, `TTL = 64`; added to the OTG configuration in P4RT-3.5.5 Step 5, after the pre-fill routes are removed | `1,000 fps`, `512` bytes |

`PO-LLDP` is the `PacketOut` frame: dst MAC `01:80:c2:00:00:0e`, src MAC
`02:00:35:03:00:01`, `ether_type = 0x88cc`, and an LLDPDU made of a Chassis ID
TLV (subtype `4`, MAC `02:00:35:03:00:01`), a Port ID TLV (subtype `7`, value
`"3"`), a TTL TLV (`120`) and an End TLV, zero-padded to `60` bytes (a `64`-byte
frame on the wire). It is sent as `PacketOut.payload` with metadata
`egress_port = "3"` and `submit_to_ingress = 0`.

#### Measurement window

Every subtest runs its traffic checks inside one or more measurement windows:

1. Start the OTG capture on ATE `port3` and start collecting `PacketIn`
   messages from the `StreamChannel`.
2. Start `Flow-Protected-v4`, `Flow-Protected-v6`, `Flow-LLDP-In` and any other
   continuous flow of the window. Flows that depend on entries programmed
   inside the window are started only after those entries are acknowledged.
3. Run the subtest steps. Once in each window (at the point stated by the
   subtest), send `300` `PO-LLDP` `PacketOut` messages at `10` frames/s.
4. Stop all traffic. Stop the capture `5 s` after the last `PacketOut`.
5. Pass only if all of the following hold, using the per-window change of the
   OTG per-flow counters read after `StopTraffic`:
   * Every forwarded flow has `frames_rx == frames_tx` (`0%` loss), unless the
     subtest states otherwise.
   * Within `30 s` after `StopTraffic`, the number of `PacketIn` messages with
     `ingress_port = "1"` whose payload has src MAC `02:00:35:01:00:01`, dst MAC
     `01:80:c2:00:00:0e` and `ether_type = 0x88cc` equals the `Flow-LLDP-In`
     Tx frame count (none missing, none duplicated).
   * The ATE `port3` capture contains exactly `300` frames with src MAC
     `02:00:35:03:00:01` and `ether_type = 0x88cc`.
   * The `StreamChannel` is still open (no stream termination and no
     `StreamError` received), and the client is still primary for
     `device_id = 1`.
   * The gRIBI `Modify` stream is still open (no gRPC status and no
     end-of-stream received).

### P4RT-3.5.1 - Baseline protected entries and Packet-I/O

* **Step 1 - *Telemetry (optional)*: record baseline utilization**:
  * For `FIB_RESOURCE_NAME`, record `used_0`, `free_0` and
    `U_0 = floor(100 * used_0 / (used_0 + free_0))`.
  * Verify that `/system/utilization/resources/resource/state/` reports
    `used-threshold-upper = 80` and `used-threshold-upper-clear = 70`, that the
    component resource reports the same two values, that `U_0 < 70`, and that
    `used-threshold-upper-exceeded = false`.
* **Step 2 - Program the protected gRIBI entries**:
  * In one `gRIBI.Modify` request, `ADD` the following entries in `DEFAULT`:
    NH `1` (`192.0.2.6`), NHG `10` (NH `1`, weight `1`), IPv4 entry
    `198.51.100.0/24` -> NHG `10`, NH `2` (`2001:db8:2::2`), NHG `11` (NH `2`,
    weight `1`) and IPv6 entry `2001:db8:100::/64` -> NHG `11`.
  * Verify that all `6` operations are `FIB_PROGRAMMED` and that `gRIBI.Get`
    (`DEFAULT`, `aft = ALL`) returns exactly these `6` entries.
  * Verify through the AFT subscription that the IPv4 (IPv6) entry references a
    next-hop-group whose only next hop has `ip-address = 192.0.2.6`
    (`2001:db8:2::2`).
* **Step 3 - Install the protected LLDP punt entry**:
  * Install `E-LLDP` with `P4Runtime.Write` (`INSERT`). Verify that the write
    returns `OK` and that `P4Runtime.Read` returns exactly `E-LLDP`.
* **Step 4 - Baseline traffic and Packet-I/O**:
  * Run measurement window `W1` for at least `60 s`, with no additional flows.
    Send the `PacketOut` burst `10 s` after the window starts.
  * Pass if all measurement window criteria hold.

### P4RT-3.5.2 - gRIBI programming with all eight WBB punt entries installed

* **Step 1 - Start measurement window `W2`** with no additional continuous
  flows.
* **Step 2 - Fill `acl_wbb_ingress_table` to its P4Info size**:
  * Install `E-SEVEN` in one `P4Runtime.Write` (`INSERT`). Verify that the write
    returns `OK` (all `7` updates succeed) and that `P4Runtime.Read` returns
    exactly the `8` entries `E-LLDP` and `E-SEVEN`.
* **Step 3 - Program gRIBI routes while all eight punt entries are installed**:
  * In one `gRIBI.Modify` request, `ADD` NH `3` (`192.0.2.14`), NHG `20` (NH
    `3`, weight `1`), the `100` IPv4 entries `100.64.10.1/32` to
    `100.64.10.100/32` -> NHG `20`, NH `4` (`2001:db8:4::2`), NHG `21` (NH `4`,
    weight `1`), and the `100` IPv6 entries `2001:db8:3000::1/128` to
    `2001:db8:3000::64/128` -> NHG `21`.
  * Verify that all `204` operations are `FIB_PROGRAMMED`, that `gRIBI.Get`
    returns these `204` entries plus the `6` protected entries, and that
    `gNMI.Get` of `state/prefix` of each of the `200` IPv4 and IPv6 AFT entries
    returns the programmed prefix.
* **Step 4 - Forwarding and Packet-I/O with a full punt table**:
  * Start `Flow-Routes-v4` and `Flow-Routes-v6`, then send `Flow-Trace-v4` and
    `Flow-Trace-v6`. Send the `PacketOut` burst after the traceroute probes
    complete. End `W2` at least `30 s` after the routes flows start.
  * In addition to the measurement window criteria, verify:
    * `Flow-Routes-v4` and `Flow-Routes-v6` have `frames_rx == frames_tx` on
      ATE `port4`.
    * Exactly `100` `PacketIn` messages carry an IPv4 payload from `192.0.2.2`
      to `198.51.100.1`, and exactly `100` carry an IPv6 payload from
      `2001:db8:1::2` to `2001:db8:100::1`. Each of them has
      `ingress_port = "1"` and `target_egress_port = "2"`.
    * After the window, `P4Runtime.Read` still returns exactly the `8` entries
      and `gRIBI.Get` returns exactly the `210` entries.

### P4RT-3.5.3 - WBB punt-entry deletion and gRIBI re-programming

* **Step 1 - Start measurement window `W3`** with no additional continuous
  flows.
* **Step 2 - Delete the non-protected punt entries**:
  * Delete `E-SEVEN` in one `P4Runtime.Write` (`DELETE`). Verify that the write
    returns `OK` and that `P4Runtime.Read` returns exactly `E-LLDP`.
* **Step 3 - Re-program the gRIBI routes**:
  * `REPLACE` the `200` IPv4 and IPv6 entries from P4RT-3.5.2 with identical
    contents (NHG `20` and NHG `21`). Verify that all `200` operations are
    `FIB_PROGRAMMED`.
  * Start `Flow-Routes-v4` and `Flow-Routes-v6`, send the `PacketOut` burst,
    and end `W3` at least `30 s` after the routes flows start.
  * In addition to the measurement window criteria, verify that
    `Flow-Routes-v4` and `Flow-Routes-v6` have `frames_rx == frames_tx` on ATE
    `port4`.
* **Step 4 - Remove the temporary gRIBI routes**:
  * In one `gRIBI.Modify` request, `DELETE` the `200` IPv4 and IPv6 entries,
    then NHG `20` and NHG `21`, then NH `3` and NH `4`. Verify that all `204`
    operations are `FIB_PROGRAMMED` (the `4` NH/NHG `DELETE`s may be
    `RIB_PROGRAMMED` with `unreferenced_aft_fib_ack_unsupported`), that
    `gRIBI.Get` returns exactly the `6` protected entries, and that `gNMI.Get`
    no longer returns any of the `200` prefixes.

### P4RT-3.5.4 - Packet-I/O and punt-entry programming under FIB exhaustion

This subtest is skipped when `gribi_riback_only` is set.

Scale chains follow TE-9.3. For chain `i` (`0 <= i <= C_max`), let
`DstIP_i = 100.65.0.1 + i` and `VIP_i = 100.66.0.1 + i` (IPv4 integer
addition; chain `0` uses `100.65.0.1` / `100.66.0.1` and chain `1,000` uses
`100.65.3.233` / `100.66.3.233`). The six operations of chain `i`, in order,
are:

1. `ADD` NH `100001 + 2i` with `ip-address = 192.0.2.14` (ATE `port4`).
2. `ADD` NHG `100001 + 2i` with that NH (weight `1`).
3. `ADD` IPv4 entry `VIP_i/32` -> NHG `100001 + 2i`.
4. `ADD` NH `100002 + 2i` with `ip-address = VIP_i`.
5. `ADD` NHG `100002 + 2i` with that NH (weight `1`).
6. `ADD` IPv4 entry `DstIP_i/32` -> NHG `100002 + 2i`.

Every `DstIP` next hop resolves recursively through a different `VIP`, so the
chains do not share next-hop or next-hop-group entries (as in TE-9.3). The DUT
has no other route that covers `100.65.0.0/16` or `100.66.0.0/16`.

* **Step 1 - Start measurement window `W4A`** with no additional continuous
  flows.
* **Step 2 - Program the sentinel chain**:
  * Send the six operations of chain `0`. Verify that all `6` are
    `FIB_PROGRAMMED`, then start `Flow-Sentinel`.
* **Step 3 - Pre-fill the FIB with BGP routes (TE-9.3)**:
  * Set the DUT BGP neighbor `2001:db8:4::2` to `enabled = true`. Once the
    session is established, the ATE `port4` peer advertises `R` IPv6 prefixes
    `2001:db8:5000::1/128 + k` for `k = 0` to `R - 1` (for `R = 2,500,000`:
    `2001:db8:5000::1/128` to `2001:db8:5000::26:25a0/128`). On platforms with
    `subnet_mask_change_required`, the prefixes are the `R` consecutive `/120`
    blocks that start at `2001:db8:5000::/120`. All pre-fill prefixes are
    inside `2001:db8:5000::/64`.
  * Wait until the neighbor reports `session-state = ESTABLISHED` and
    `prefixes/received = R` for `IPV6_UNICAST` (timeout `T_prefill`). Then wait
    until the FIB has settled: the `aft-entries` counter of origin protocol
    `BGP` under
    `/network-instances/network-instance/afts/aft-summaries/ipv6-unicast/`
    is unchanged across two consecutive `30 s` samples (timeout `T_prefill`).
    On platforms that do not report AFT summaries, wait `5 min` as TE-9.3 does.
  * *Telemetry (optional)*: record `used_P` and
    `U_P = floor(100 * used_P / (used_P + free_P))`, and verify that
    `used_P > used_0` and `U_P >= U_0 + 1`.
* **Step 4 - Inject scale chains until the first `FIB_FAILED`**:
  * Send chains `i = 1, 2, ...` in batches of `B` complete chains. After each
    batch, wait up to `T_batch` for the `AFTResult` of every operation of the
    batch.
  * Stop after the first batch that contains a `FIB_FAILED` result, after
    `C_max` chains, or after `T_max`, whichever comes first. The test fails if
    any operation of a sent batch has no final `AFTResult` after `T_batch`
    (final means `FIB_PROGRAMMED` or `FIB_FAILED`, or `RIB_PROGRAMMED` where the
    gating table allows it), or if any result is `FAILED`. If no `FIB_FAILED`
    result is received before the cap or the timeout, the test fails with an
    explicit configuration error: `R` is under-calibrated for the platform.
  * Let `F` be the chain that contains the lowest-`id` operation acknowledged
    `FIB_FAILED`, `M_gribi = F - 1` the number of scale chains that were fully
    `FIB_PROGRAMMED` before it, and `N_sent` the number of scale chains sent
    (`F <= N_sent <= C_max`). Log `M_gribi`, `F` and `N_sent`.
  * Verify that:
    * every operation with an `id` lower than the first `FIB_FAILED`
      operation is `FIB_PROGRAMMED`, except NH/NHG `ADD`s of the stopping batch
      that may be `RIB_PROGRAMMED` with `unreferenced_aft_fib_ack_unsupported`;
    * every operation sent is either `FIB_PROGRAMMED` or `FIB_FAILED`, with the
      same exception;
    * with `unreferenced_aft_fib_ack_unsupported`, `gRIBI.Get` returns every
      NH/NHG of the stopping batch that ended `RIB_PROGRAMMED`;
    * the first `FIB_FAILED` result has a non-empty
      `error_details.error_message`;
    * the `Modify` stream is still open and the DUT returned no gRPC error
      status (in particular no `RESOURCE_EXHAUSTED`).
  * *Telemetry (optional)*: on every sample during this step, `used + free`
    does not exceed `max-limit` (when `max-limit` is reported), and the
    hysteresis rule holds: `used-threshold-upper-exceeded` is never `true`
    while `U < 70` and never `false` while `U > 80`, where
    `U = floor(100 * used / (used + free))`. One sample of lag between
    `used` / `free` and `used-threshold-upper-exceeded` is tolerated: a
    hysteresis violation fails the test only if it is present in two
    consecutive samples. `used` sampled after each fully `FIB_PROGRAMMED` batch
    never decreases. At the end of the step, `high-watermark` (when reported)
    is greater than or equal to the largest `used` sample.
* **Step 5 - End `W4A`**:
  * Send the `PacketOut` burst after Step 4 ends, then end `W4A`.
  * In addition to the measurement window criteria (protected flows without
    loss and complete LLDP Packet-I/O throughout the pre-fill and the
    exhaustion), verify that `Flow-Sentinel` has `frames_rx == frames_tx` on
    ATE `port4`.
* **Step 6 - Start measurement window `W4B`** with the FIB still exhausted
  (nothing is deleted), with `Flow-Sentinel` and `Flow-Scale` as additional
  continuous flows. `Flow-Scale` is excluded from the `0%` loss criterion
  because it also targets chains that are not programmed.
* **Step 7 - Program WBB punt entries while the FIB is exhausted**:
  * Install `E-SEVEN` (`INSERT`). Verify that the write returns `OK` and that
    `P4Runtime.Read` returns exactly the `8` entries.
  * Delete `E-SEVEN` (`DELETE`). Verify that the write returns `OK` and that
    `P4Runtime.Read` returns exactly `E-LLDP`.
  * Re-install `E-SEVEN` (`INSERT`). Verify that the write returns `OK` and
    that `P4Runtime.Read` returns exactly the `8` entries.
  * Send a single-update `INSERT` of `E-LLDP`, which already exists. Verify
    that the RPC returns outer gRPC status `UNKNOWN` whose details contain
    exactly one `p4.v1.Error` with `canonical_code = ALREADY_EXISTS`, and that
    `P4Runtime.Read` still returns exactly the `8` entries.
* **Step 8 - Packet-I/O and forwarding while the FIB is exhausted**:
  * Send `Flow-Trace-v4` and `Flow-Trace-v6`, then send the `PacketOut` burst.
    End `W4B` at least `60 s` after it started.
  * In addition to the measurement window criteria, verify:
    * Exactly `100` IPv4 and `100` IPv6 traceroute `PacketIn` messages are
      received, as defined in P4RT-3.5.2 Step 4, each with
      `ingress_port = "1"` and `target_egress_port = "2"`.
    * `Flow-Sentinel` has `frames_rx == frames_tx` on ATE `port4`.
    * `Flow-Scale` received at least one packet on ATE `port4` for every tag
      value `1` to `M_gribi + 1` (chains `0` to `M_gribi`), and `0` packets for
      tag value `F + 1` (the chain that returned `FIB_FAILED`).
    * The AFT subscription reports no delete or change of the protected
      entries during `W4A` and `W4B`.

### P4RT-3.5.5 - Recovery after FIB exhaustion

This subtest is skipped when `gribi_riback_only` is set. It continues from the
state at the end of P4RT-3.5.4.

* **Step 1 - Start measurement window `W5A`** with no additional continuous
  flows.
* **Step 2 - Delete the scale chains**:
  * On the same `Modify` stream, `DELETE` chains `N_sent` down to `0` in
    batches of `B` chains. For each chain, delete the `DstIP_i` entry, its NHG
    and its NH, then the `VIP_i` entry, its NHG and its NH.
  * Verify that every `DELETE` is acknowledged `FIB_PROGRAMMED` and that none
    is `FAILED` or `FIB_FAILED`. `RIB_PROGRAMMED` is also accepted for an entry
    whose `ADD` was `FIB_FAILED` and, with
    `unreferenced_aft_fib_ack_unsupported`, for the NH/NHG `DELETE`s. Verify
    that `gRIBI.Get` then returns exactly the `6` protected entries.
* **Step 3 - Remove the pre-fill routes**:
  * Set the DUT BGP neighbor `2001:db8:4::2` to `enabled = false`. Wait until
    `session-state` is no longer `ESTABLISHED` and the `BGP` `aft-entries`
    counter is `0` or absent (timeout `T_prefill`). On platforms that do not
    report AFT summaries, wait `5 min` after the session goes down.
* **Step 4 - End `W5A`**:
  * Send the `PacketOut` burst, then end `W5A`. Pass if all measurement window
    criteria hold (the protected entries are unaffected by the mass deletion).
* **Step 5 - Add `Flow-Chain-F`**:
  * Push the OTG configuration again with `Flow-Chain-F` added (`DstIP_F` is
    now known and the pre-fill routes are no longer needed), start protocols
    and wait for IPv4 ARP and IPv6 ND resolution.
* **Step 6 - Start measurement window `W5B`** with no additional continuous
  flows.
* **Step 7 - Re-program the chain that failed**:
  * Send the six `ADD` operations of chain `F` again. Verify that all `6` are
    `FIB_PROGRAMMED`, then start `Flow-Chain-F`.
* **Step 8 - Update a WBB punt entry after recovery**:
  * `MODIFY` `E-T4-2` to action `acl_wbb_ingress_copy`, and verify that the
    write returns `OK` and that `P4Runtime.Read` reports the new action. Then
    `MODIFY` it back to `acl_wbb_ingress_trap` and verify the same way.
  * On platforms with `p4rt_modify_table_entry_unsupported`, perform each change
    as a `DELETE` of `E-T4-2` followed by an `INSERT` with the new action; each
    write returns `OK`.
  * Verify that `P4Runtime.Read` finally returns exactly the `8` entries, all
    with action `acl_wbb_ingress_trap`.
* **Step 9 - End `W5B`**:
  * Send the `PacketOut` burst, and end `W5B` at least `30 s` after
    `Flow-Chain-F` starts.
  * In addition to the measurement window criteria, verify that `Flow-Chain-F`
    has `frames_rx == frames_tx` on ATE `port4`.
* **Step 10 - Remove the recovery chain**:
  * `DELETE` the six entries of chain `F`: all `FIB_PROGRAMMED`, except that
    the `4` NH/NHG `DELETE`s may be `RIB_PROGRAMMED` with
    `unreferenced_aft_fib_ack_unsupported`. Verify that `gRIBI.Get` returns
    exactly the `6` protected entries.
* **Step 11 - *Telemetry (optional)*: utilization returns to baseline**:
  * Within `5 min`, `|used - used_0| <= 0.01 * (used_0 + free_0)` and
    `used-threshold-upper-exceeded = false`.

### P4RT-3.5.6 - (Optional) P4RT table admission control at the P4Info size

This subtest runs only when the per-platform opt-in flag
`p4rt_table_size_hard_limit_enforced` (a metadata flag or test argument, to be
added with the automation) is set. It is meant for platforms that enforce the
P4Info `acl_wbb_ingress_table` `size` (`8`) as a hard limit. By default the
flag is not set, and the subtest is skipped and logged. The P4Runtime
specification treats `size` as the requested capacity: a target may accept more
entries and clients should not insert beyond it, so accepting a ninth entry is
not a failure on platforms that do not opt in.

* **Step 1 - Precondition**: `P4Runtime.Read` returns exactly the `8` entries
  `E-LLDP` and `E-SEVEN` (as left by P4RT-3.5.5, or re-installed with
  `INSERT` if the earlier subtests were skipped).
* **Step 2 - Start measurement window `W6`** with no additional continuous
  flows.
* **Step 3 - Insert one entry beyond the P4Info size**:
  * Send a single-update `P4Runtime.Write` (`INSERT`) of `E-N1`. Verify that
    the RPC returns outer gRPC status `UNKNOWN` whose details contain exactly
    one `p4.v1.Error` with `canonical_code = RESOURCE_EXHAUSTED`, and that it
    does not return `UNAVAILABLE`, `DEADLINE_EXCEEDED`, `CANCELLED` or
    `PERMISSION_DENIED`.
  * Verify that `P4Runtime.Read` returns exactly the `8` entries (`E-N1` is
    absent).
* **Step 4 - Existing entries keep working**:
  * Send `Flow-Trace-v4` and `Flow-Trace-v6`, then send the `PacketOut` burst,
    and end `W6` at least `30 s` after it started.
  * In addition to the measurement window criteria, verify exactly `100` IPv4
    and `100` IPv6 traceroute `PacketIn` messages as in P4RT-3.5.2 Step 4.

### Cleanup

* Stop all OTG traffic and protocols. Set the DUT BGP neighbor to
  `enabled = false` if it is still enabled, then remove the BGP neighbor and
  global configuration and the `ALLOW` policy.
* Delete all entries installed by the test (`E-LLDP`, `E-SEVEN`, and `E-N1` if
  present) with `P4Runtime.Write` (`DELETE`). If `P_pre` is not empty, re-push
  it with `P4Runtime.SetForwardingPipelineConfig` (`VERIFY_AND_COMMIT`). Then
  re-install (`INSERT`) every entry of `E_pre` that is not already present
  (the platform may re-create some of them), and verify that `P4Runtime.Read`
  returns exactly `E_pre` (non-const entries).
* Run `gRIBI.Flush` on `DEFAULT` and verify that `gRIBI.Get` returns no
  entries.
* Restore `/lldp/config/enabled` to the value recorded in setup, delete the
  `FIB_RESOURCE_NAME` threshold configuration (if it was configured), and
  revert the integrated-circuit `node-id` and the interface configuration to
  the pre-test baseline.

## Canonical OC

`P4RT_NODE` and `FIB_RESOURCE_NAME` are placeholders for the integrated circuit
resolved in setup and the per-platform FIB resource name. The
`system/utilization` block is applied only on platforms that report
`FIB_RESOURCE_NAME`. The BGP neighbor is shown as configured during P4RT-3.5.4
(`enabled = true`); it is `false` at all other times.

```json
{
  "components": {
    "component": [
      {
        "config": {
          "name": "P4RT_NODE"
        },
        "integrated-circuit": {
          "config": {
            "node-id": "1"
          }
        },
        "name": "P4RT_NODE"
      }
    ]
  },
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "Traffic and PacketIn ingress port 1",
          "enabled": true,
          "id": 1,
          "name": "port1",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port1",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.1",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.1"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:1::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8:1::1"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              }
            }
          ]
        }
      },
      {
        "config": {
          "description": "Protected egress port 2",
          "enabled": true,
          "id": 2,
          "name": "port2",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port2",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.5",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.5"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:2::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8:2::1"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              }
            }
          ]
        }
      },
      {
        "config": {
          "description": "PacketOut egress port 3",
          "enabled": true,
          "id": 3,
          "name": "port3",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port3",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.9",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.9"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:3::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8:3::1"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              }
            }
          ]
        }
      },
      {
        "config": {
          "description": "Scale egress and BGP pre-fill port 4",
          "enabled": true,
          "id": 4,
          "name": "port4",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port4",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.13",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.13"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:4::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8:4::1"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              }
            }
          ]
        }
      }
    ]
  },
  "lldp": {
    "config": {
      "enabled": false
    }
  },
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "DEFAULT",
          "type": "openconfig-network-instance-types:DEFAULT_INSTANCE"
        },
        "name": "DEFAULT",
        "protocols": {
          "protocol": [
            {
              "bgp": {
                "global": {
                  "afi-safis": {
                    "afi-safi": [
                      {
                        "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                        "config": {
                          "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                          "enabled": true
                        }
                      }
                    ]
                  },
                  "config": {
                    "as": 64500,
                    "router-id": "192.0.2.13"
                  }
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                            "apply-policy": {
                              "config": {
                                "import-policy": [
                                  "ALLOW"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                              "enabled": true
                            }
                          }
                        ]
                      },
                      "config": {
                        "enabled": true,
                        "neighbor-address": "2001:db8:4::2",
                        "peer-as": 64501
                      },
                      "neighbor-address": "2001:db8:4::2"
                    }
                  ]
                }
              },
              "config": {
                "identifier": "openconfig-policy-types:BGP",
                "name": "BGP"
              },
              "identifier": "openconfig-policy-types:BGP",
              "name": "BGP"
            }
          ]
        }
      }
    ]
  },
  "routing-policy": {
    "policy-definitions": {
      "policy-definition": [
        {
          "config": {
            "name": "ALLOW"
          },
          "name": "ALLOW",
          "statements": {
            "statement": [
              {
                "actions": {
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                },
                "config": {
                  "name": "id-1"
                },
                "name": "id-1"
              }
            ]
          }
        }
      ]
    }
  },
  "system": {
    "utilization": {
      "resources": {
        "resource": [
          {
            "config": {
              "name": "FIB_RESOURCE_NAME",
              "used-threshold-upper": 80,
              "used-threshold-upper-clear": 70
            },
            "name": "FIB_RESOURCE_NAME"
          }
        ]
      }
    }
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  # Interfaces and P4RT port IDs.
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/id:
  /interfaces/interface/state/hardware-port:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  # P4RT node resolution and device ID.
  /components/component/integrated-circuit/config/node-id:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/state/parent:
    platform_type: ["PORT"]
  /components/component/state/type:
    platform_type: ["INTEGRATED_CIRCUIT"]
  # On-box LLDP disabled so that LLDP frames are punted to the P4RT client.
  /lldp/config/enabled:
  # TE-9.3 BGP pre-fill.
  /network-instances/network-instance/protocols/protocol/bgp/global/config/as:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received:
  /routing-policy/policy-definitions/policy-definition/config/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/config/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result:
  # AFT state of the protected entries and FIB settle counter.
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address:
  /network-instances/network-instance/afts/aft-summaries/ipv6-unicast/protocols/protocol/state/counters/aft-entries:
  # Utilization telemetry (optional, platforms that report FIB_RESOURCE_NAME).
  /system/utilization/resources/resource/config/name:
  /system/utilization/resources/resource/config/used-threshold-upper:
  /system/utilization/resources/resource/config/used-threshold-upper-clear:
  /system/utilization/resources/resource/state/name:
  /system/utilization/resources/resource/state/used-threshold-upper:
  /system/utilization/resources/resource/state/used-threshold-upper-clear:
  /components/component/integrated-circuit/utilization/resources/resource/state/name:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/utilization/resources/resource/state/used:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/utilization/resources/resource/state/free:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/utilization/resources/resource/state/max-limit:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/utilization/resources/resource/state/high-watermark:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/utilization/resources/resource/state/used-threshold-upper:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/utilization/resources/resource/state/used-threshold-upper-clear:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/utilization/resources/resource/state/used-threshold-upper-exceeded:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/chassis/utilization/resources/resource/state/used:
    platform_type: ["CHASSIS"]
  /components/component/chassis/utilization/resources/resource/state/free:
    platform_type: ["CHASSIS"]

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
      on_change: true
      sample: true
  gribi:
    gRIBI.Flush:
    gRIBI.Get:
    gRIBI.Modify:
  # P4Runtime RPCs exercised (P4Runtime is not an OpenConfig repository, so they
  # are not parsed by the coverage tooling): P4Runtime.GetForwardingPipelineConfig,
  # P4Runtime.SetForwardingPipelineConfig, P4Runtime.StreamChannel (arbitration,
  # PacketIn, PacketOut), P4Runtime.Write and P4Runtime.Read.
```

## Required DUT platform

* FFF
