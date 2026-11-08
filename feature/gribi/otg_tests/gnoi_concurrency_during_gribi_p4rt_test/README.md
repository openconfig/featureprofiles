# TE-1.24: gNOI Operations During gRIBI and P4RT Programming

## Summary

This test validates control-plane and data-plane resilience when operational
`gNOI` RPCs (`System` `Time` / `Ping` / `Traceroute`, `Healthz`, `File`, and
`LinkQualification` on a non-transit interface) execute concurrently with paced
`gRIBI` Abstract Forwarding Table (AFT) churn and `P4Runtime` (`P4RT`) churn of
the WBB `acl_wbb_ingress_table` punt entries (`wbb.p4info.pb.txt`), while LLDP
`PacketIn` and `PacketOut` traffic is exchanged with the P4RT controller. It
verifies that every gNOI operation completes with its defined result, that
`gRIBI` and `P4RT` operations keep succeeding with latency bounded relative to
a baseline measured with the same load and no gNOI operations, that no `gRIBI`,
`P4RT`, or `gNMI` session is dropped, that protected transit traffic,
`PacketIn`, and `PacketOut` sustain zero loss, and that the DUT remains stable
(no reboot, process restart, or component state change).

## Testbed type

* [`TESTBED_DUT_ATE_4LINKS`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

```text
                             +---------+
  [ ATE port1 ] ------------ |         | ------------ [ ATE port2 ] (Protected Transit Egress)
  (Transit & LLDP Src)       |   DUT   | ------------ [ ATE port3 ] (Churn Next Hops & PacketOut)
                             |         |
                             |         | ------------ [ ATE port4 ] (Non-Transit gNOI Port)
                             +---------+
```

* Connect 4 interfaces between the DUT and ATE and configure dual-stack IPv4 and
  IPv6 addresses in the `DEFAULT` network instance:
  * **Link 1 (Transit & Packet-In Ingress)**: DUT `port1` (`192.0.2.1/30`,
    `2001:db8:1::1/126`, P4RT `id = 1`) <-> ATE `port1` (`192.0.2.2/30`,
    `2001:db8:1::2/126`, MAC `02:00:01:01:01:01`).
  * **Link 2 (Protected Transit Egress)**: DUT `port2` (`192.0.2.5/30`,
    `2001:db8:2::1/126`, P4RT `id = 2`) <-> ATE `port2` (`192.0.2.6/30`,
    `2001:db8:2::2/126`, MAC `02:00:02:01:01:01`).
  * **Link 3 (Churn Next Hops & PacketOut Egress)**: DUT `port3`
    (`192.0.2.9/30`, `2001:db8:3::1/126`, P4RT `id = 3`) <-> ATE `port3`
    (`192.0.2.10/30`, `2001:db8:3::2/126`, MAC `02:00:03:01:01:01`).
  * **Link 4 (Dedicated Non-Transit Port)**: DUT `port4` (`192.0.2.13/30`,
    `2001:db8:4::1/126`, P4RT `id = 4`) <-> ATE `port4` (`192.0.2.14/30`,
    `2001:db8:4::2/126`, MAC `02:00:04:01:01:01`).
* Verify all 4 DUT interfaces report `oper-status = UP` via `gNMI.Subscribe`
  (`ON_CHANGE`) on `/interfaces/interface/state/oper-status`.
* Record the pre-test value of `/lldp/config/enabled` and set it to `false` via
  `gNMI.Set` to disable on-box LLDP processing (as in P4RT-7.1 and P4RT-7.2),
  so that LLDP frames are handled only by the P4RT punt entry.
* Configure `/components/component/integrated-circuit/config/node-id` to `1`
  (P4RT `device_id = 1`) for the integrated circuit that hosts DUT `port1` to
  `port4`, and the interface `/interfaces/interface/config/id` values (`1`–`4`)
  via `gNMI.Set`.
* Establish a `P4Runtime` client session (`device_id = 1`, `election_id =
  {high: 0, low: 1}`) and complete primary controller arbitration via
  `MasterArbitrationUpdate` on `P4Runtime.StreamChannel`.
* Record the pre-test P4RT state before changing it, because pushing a pipeline
  with `VERIFY_AND_COMMIT` clears forwarding state per P4Runtime v1 §7.1: read
  the current pipeline with `P4Runtime.GetForwardingPipelineConfig`
  (`response_type = ALL`) as `P_pre` (empty if no pipeline is configured) and,
  if a pipeline is present, read all entries of `acl_wbb_ingress_table` with
  `P4Runtime.Read` as `E_pre`. Const entries (`is_const = true`, such as the
  `const_default_action_id` defined in `wbb.p4info.pb.txt`) belong to the
  pipeline, cannot be deleted or re-inserted, and are excluded from `E_pre` and
  from every `P4Runtime.Read` entry count in this test (`is_const = false`
  filter).
* Push the WBB forwarding pipeline (`feature/p4rt/data/wbb.p4info.pb.txt`,
  defining `acl_wbb_ingress_table` with `size = 8`, actions
  `acl_wbb_ingress_copy` and `acl_wbb_ingress_trap`, `packet_in` metadata
  `ingress_port` / `target_egress_port`, and `packet_out` metadata
  `egress_port` / `submit_to_ingress`) via
  `P4Runtime.SetForwardingPipelineConfig` (`action = VERIFY_AND_COMMIT`). If
  `P4Runtime.Read` (`is_const = false`) still returns non-const entries in
  `acl_wbb_ingress_table` (for example, entries installed automatically by the
  platform when P4RT is enabled), add them to `E_pre`, `DELETE` them via
  `P4Runtime.Write`, and verify that a second `P4Runtime.Read` returns `0`
  non-const entries. `P_pre` and `E_pre` are restored in Cleanup.
* Establish a `gRIBI` client session in `SINGLE_PRIMARY` mode with `PRESERVE`
  persistence and `ack_type = RIB_AND_FIB_ACK`, elect the client as leader
  (`election_id = {high: 0, low: 1}`), and flush the `DEFAULT` network instance
  via `gRIBI.Flush` (`election.id = {high: 0, low: 1}` or `override = {}`,
  satisfying `FlushRequest.election` in `SINGLE_PRIMARY` mode per
  `gribi.proto`).
* Select the Healthz target component `HC`: the component whose
  `/components/component/state/type` is `CHASSIS`. On platforms that do not
  support Healthz RPCs against the chassis-level component
  (`chassis_get_rpc_unsupported`), `HC` is the active `CONTROLLER_CARD`
  component (or `<active_rp>-<process_name>` on platforms whose Healthz
  implementation targets per-RP process components). Every `gnoi.types.Path`
  passed to `Healthz` sets `origin = "openconfig"`, except on platforms with
  deviation `skip_origin` where `origin` is left empty. Issue
  `gnoi.healthz.Healthz.Check` (logging and continuing if active `Check` is
  unsupported for `HC`) and then `gnoi.healthz.Healthz.Get` for
  `/components/component[name=HC]`; `Get` must return gRPC `OK` with status
  `STATUS_HEALTHY` (`Get` is issued after `Check` because `Get` returns an
  error when no health status exists yet). On platforms that enforce a minimum
  cooldown between consecutive `Healthz` calls (for example, `120 s`), honor
  that cooldown before the next `Healthz` round.
* Capture the stability snapshot `S0` via `gNMI.Get`:
  * `/system/state/boot-time`.
  * The stable process set: every process whose `state/name`, `state/pid`, and
    `state/start-time` under `/system/processes/process` are identical in two
    reads taken `60 s` apart.
  * `/components/component/state/oper-status`, and
    `/components/component/state/last-reboot-time` where reported, of every
    component of type `CHASSIS`, `CONTROLLER_CARD`, `FAN`,
    `INTEGRATED_CIRCUIT`, `LINECARD`, or `POWER_SUPPLY`.
* Start OTG protocols and explicitly resolve IPv4 ARP and IPv6 ND on all 4 links
  before starting traffic.

### Background load and common window criteria

Each measurement window (TE-1.24.1 Step 3, TE-1.24.2, TE-1.24.3, and TE-1.24.4)
starts the background load B1–B5 below and stops it as described after the
list. TE-1.24.1 Step 2 uses B1–B3 only.

* **B1 - Protected transit flows**: OTG IPv4 (`TTL = 64`) and IPv6
  (`HopLimit = 64`) flows from ATE `port1` to `198.51.100.1` and
  `2001:db8:100::1` (`512`-byte frames, `5,000 fps` each, `10,000 fps`
  aggregate), expected on ATE `port2`.
* **B2 - LLDP `PacketIn`**: an OTG raw Ethernet flow on ATE `port1`
  (destination MAC `01:80:c2:00:00:0e`, source MAC `02:00:01:01:01:01`,
  EtherType `0x88cc`, `128`-byte frames) at `10 fps`, trapped by the protected
  LLDP entry and delivered to the controller as `PacketIn` messages.
* **B3 - `PacketOut`**: the controller sends `10` `PacketOut` messages per
  second on `P4Runtime.StreamChannel`, each carrying the same LLDP frame
  (destination MAC `01:80:c2:00:00:0e`, source MAC `02:00:00:00:00:aa`,
  EtherType `0x88cc`, with Chassis ID, Port ID, TTL, and End TLVs) and the
  metadata `egress_port = "3"` (DUT `port3`) and `submit_to_ingress = 0`. An
  OTG packet capture runs on ATE `port3` for the whole window.
* **B4 - P4RT churn**: one cycle per second (a cycle that takes longer than
  `1 s` is followed immediately by the next) over the `7` churn entries, which
  are the non-LLDP entries allowed by the table's `@entry_restriction`, all
  with `priority = 1`: `6` traceroute entries (`is_ipv4 = 1` or `is_ipv6 = 1`,
  each with `ttl == 0`, `1`, or `2` `& 0xff`) and `1` ND/GDP entry
  (`ether_type == 0x6007 & 0xffff`). Each `P4Runtime.Write` carries all `7`
  updates (`Atomicity = CONTINUE_ON_ERROR`). A cycle is:
  1. `INSERT` of the 7 entries with action `acl_wbb_ingress_trap`.
  2. `MODIFY` of the 7 entries changing only the action to
     `acl_wbb_ingress_copy` (match key and priority unchanged). On platforms
     that do not support the P4Runtime `MODIFY` update type, this step is a
     `DELETE` of the 7 entries followed by an `INSERT` of the same entries with
     action `acl_wbb_ingress_copy`.
  3. `P4Runtime.Read` of `acl_wbb_ingress_table`.
  4. `DELETE` of the 7 entries.
  5. `P4Runtime.Read` of `acl_wbb_ingress_table`.
* **B5 - gRIBI churn**: repeated cycles of `ADD` then `DELETE` of `2,000` IPv4
  `/24` entries (`100.64.0.0/24`–`100.71.207.0/24`, i.e.
  `100.<64 + floor(i / 256)>.<i mod 256>.0/24`) pointing to `NextHopGroup 100`
  and `2,000` IPv6 `/64` entries (`2001:db8:1000::/64`–`2001:db8:17cf::/64`,
  i.e. `2001:db8:<hex(0x1000 + i)>::/64`) pointing to `NextHopGroup 101`, for
  `i = 0` to `1999`, in `DEFAULT` (`NextHopGroup 100` and `101` on ATE `port3`
  are programmed in TE-1.24.1 Step 1 and are not churned). `ModifyRequest`s
  carry `100` `AFTOperation`s each and are sent at a paced rate of `10`
  requests per second (`1,000` AFT operations/s), with at most `10` requests
  awaiting acknowledgement (a request is awaiting acknowledgement until each of
  its operations has returned `FIB_PROGRAMMED`, `FAILED`, or `FIB_FAILED`). A
  cycle is `40` `ADD` requests (IPv4, then IPv6) followed, once every `ADD` is
  acknowledged, by `40` `DELETE` requests in the same order (`8,000`
  operations, about `8 s`). The ack latency of an operation is the time from
  sending its `ModifyRequest` to receiving its `FIB_PROGRAMMED` `AFTResult`.

To stop a window, let B5 and B4 finish their current cycle (all churn prefixes
and churn entries deleted), stop B3, stop the B1 and B2 flows, wait `10 s`, and
stop the capture. The following criteria are then evaluated:

* **W1 - Transit**: for each B1 flow, OTG flow metrics report `frames_rx ==
  frames_tx` (`0%` loss).
* **W2 - `PacketIn`**: the number of `PacketIn` messages received in the window
  whose payload has EtherType `0x88cc` and source MAC `02:00:01:01:01:01`, and
  whose `ingress_port` metadata is `"1"`, equals the B2 `frames_tx`. Other
  `PacketIn` messages (for example, IPv6 packets with hop limit `1` matched by
  a churn traceroute entry) are ignored.
* **W3 - `PacketOut`**: no `PacketOut` is rejected with a `StreamChannel`
  error, and the number of frames captured on ATE `port3` with EtherType
  `0x88cc` and source MAC `02:00:00:00:00:aa` equals the number of `PacketOut`
  messages sent.
* **W4 - gRIBI**: every B5 `AFTOperation` returns `FIB_PROGRAMMED` (none
  returns `FAILED` or `FIB_FAILED`), no ack latency exceeds `5 s`, and the
  `gRIBI.Modify` stream is never closed and returns no error (including
  `DEADLINE_EXCEEDED`).
* **W5 - P4RT writes**: every B4 `P4Runtime.Write` returns gRPC `OK` with no
  per-update error; every Read (`is_const = false`) in cycle step 3 returns
  exactly `8` non-const entries (the protected LLDP entry with
  `acl_wbb_ingress_trap` and the 7 churn entries with `acl_wbb_ingress_copy`),
  and every Read (`is_const = false`) in cycle step 5 returns exactly the
  protected LLDP entry.
* **W6 - P4RT session**: the `StreamChannel` stays open and the client remains
  the primary controller (no arbitration change).
* **W7 - gNMI and ports**: `S1` (opened in TE-1.24.1 Step 1) is not closed by
  the DUT (`0` disconnects); DUT `port1`–`port4` report no `oper-status` other
  than `UP` (except DUT `port4` in TE-1.24.4, see there); every subscribed
  counter leaf receives `SAMPLE` updates at most `20 s` apart; and no
  notification changes or deletes the `next-hop-group` of the two protected
  AFT entries.
* **W8 - Post-churn state**: `gRIBI.Get` (`DEFAULT`, `aft = ALL`) returns
  exactly `NextHop` `1`, `2`, `100`, `101`, `NextHopGroup` `10`, `11`, `100`,
  `101`, and the prefixes `198.51.100.0/24` and `2001:db8:100::/64`. The
  protected prefixes, `NextHopGroup` `10` and `11`, and `NextHop` `1` and `2`
  report `fib_status = PROGRAMMED`; the churn next hops and next-hop groups,
  which no prefix references after the churn, report `rib_status =
  PROGRAMMED`. A `gNMI.Get` of the `DEFAULT` AFT contains none of the `4,000`
  churn prefixes.
* **W9 - Stability**: compared with `S0`, `/system/state/boot-time` is
  unchanged, every process of the stable set is still present with the same
  `pid` and `start-time` (processes outside the set, such as short-lived
  helpers spawned by gNOI `Ping` or `Traceroute`, are ignored), and the
  `oper-status` and `last-reboot-time` of every component in `S0` are
  unchanged.
* **W10 - Latency** (TE-1.24.2, TE-1.24.3, and TE-1.24.4 only): over the
  operations sent between the start of the first and the completion of the
  last gNOI RPC of the subtest, the p99 B5 ack latency is
  `<= max(2 x L99_gribi_base, L99_gribi_base + 50 ms)`, and the p99 B4
  `P4Runtime.Write` latency (request sent to response received) is
  `<= max(2 x L99_p4rt_base, L99_p4rt_base + 20 ms)`. The absolute floors keep
  the bounds meaningful when a baseline p99 is only a few milliseconds (the
  P4RT baseline p99 is computed from only a few hundred writes); the `5 s`
  per-operation limit of W4 still applies.

### TE-1.24.1 - Baseline Forwarding, Packet I/O, and Programming Latency

* **Step 1 - Program Protected Baseline Entries**:
  * Via `gRIBI.Modify` (`ADD`) in `DEFAULT`, program the protected entries
    `NextHop 1` (`192.0.2.6`, ATE `port2`) -> `NextHopGroup 10` ->
    `198.51.100.0/24` and `NextHop 2` (`2001:db8:2::2`, ATE `port2`) ->
    `NextHopGroup 11` -> `2001:db8:100::/64`, and the churn next hops
    `NextHop 100` (`192.0.2.10`, ATE `port3`) -> `NextHopGroup 100` and
    `NextHop 101` (`2001:db8:3::2`, ATE `port3`) -> `NextHopGroup 101`.
    Acknowledgements are awaited once after the full NH -> NHG -> prefix
    sequence (timeout `60 s`): every operation returns `FIB_PROGRAMMED`, except
    that on platforms that do not send `FIB_PROGRAMMED` for NH/NHG entries that
    no prefix references yet, the `NextHop 100` / `101` and
    `NextHopGroup 100` / `101` operations return `RIB_PROGRAMMED`.
  * Via `P4Runtime.Write` (`INSERT`, `Atomicity = CONTINUE_ON_ERROR`), program
    the protected LLDP punt entry in `acl_wbb_ingress_table` (`ether_type ==
    0x88cc & 0xffff`, `priority = 1`, action `acl_wbb_ingress_trap`) and verify
    that `P4Runtime.Read` (`is_const = false`) returns exactly this non-const
    entry.
  * Verify via `gRIBI.Get` the entry set defined in W8, and via `gNMI.Get` that
    the AFT entry for `198.51.100.0/24` references a next-hop-group with
    `programmed-id = 10` whose only next hop has `ip-address = 192.0.2.6`, and
    that the AFT entry for `2001:db8:100::/64` references a next-hop-group with
    `programmed-id = 11` whose only next hop has `ip-address = 2001:db8:2::2`.
  * Open the persistent `gNMI.Subscribe` `STREAM` subscription `S1`, kept open
    until Cleanup, with `ON_CHANGE` on `/interfaces/interface/state/oper-status`
    of DUT `port1`–`port4`, `SAMPLE` (`sample_interval = 10 s`,
    `suppress_redundant = false`) on
    `/interfaces/interface/state/counters/in-pkts` and `out-pkts` of DUT
    `port1`–`port3`, and `ON_CHANGE` on `state/next-hop-group` of the two
    protected `DEFAULT` AFT entries.
* **Step 2 - Verify Protected Forwarding and Packet I/O Without Churn**:
  * Run B1 for `30 s` together with B2 and B3 limited to `300` frames each
    (`30 s` at `10` frames/s), with no churn and no gNOI RPCs, then stop them
    as for a window.
  * Pass: W1, W2, W3, W6, and W7 hold (`300` LLDP `PacketIn` messages and
    `300` `PacketOut` frames on ATE `port3`).
* **Step 3 - Measure the No-gNOI Baseline `T_base`**:
  * Run B1–B5 for `120 s` without any gNOI RPC, then stop them.
  * Record `L99_gribi_base` (p99 B5 ack latency) and `L99_p4rt_base` (p99 B4
    `P4Runtime.Write` latency) over the window.
  * Pass: W1–W9 hold.

### TE-1.24.2 - System and Healthz gNOI Operations Under gRIBI/P4RT Churn

* **Step 1 - Start the Background Load**: start B1–B5.
* **Step 2 - Execute System and Healthz gNOI RPCs**:
  * Record the DUT time `T2` via `gnoi.system.System.Time`.
  * Execute **Healthz round 1** for `HC` (honoring any platform `Healthz`
    cooldown since setup): issue `gnoi.healthz.Healthz.Check` (`300 s`
    deadline, if active `Check` is supported for `HC`), followed by
    `gnoi.healthz.Healthz.Get` and `gnoi.healthz.Healthz.List` (`60 s`
    deadline each).
  * Run `20` System RPC iterations back-to-back concurrently with the
    background load. Within an iteration the following RPCs are issued
    sequentially (each with a `60 s` deadline):
    1. `gnoi.system.System.Time`, immediately followed by a `gNMI.Get` of
       `/system/state/current-datetime`.
    2. `gnoi.system.System.Ping` from the DUT to `192.0.2.14` (`count = 5`,
       `l3protocol = IPV4`) and then to `2001:db8:4::2` (`count = 5`,
       `l3protocol = IPV6`) on non-transit ATE `port4`.
    3. `gnoi.system.System.Traceroute` to `192.0.2.14` on ATE `port4`
       (`l3protocol = IPV4`, `l4protocol = ICMP`, `max_ttl = 1`). On platforms
       whose gNOI Traceroute supports only UDP probes, `l4protocol = UDP` and
       the destination is the DUT `port4` address `192.0.2.13`.
  * Execute **Healthz round 2** for `HC` after the `20` System iterations
    complete (waiting first if needed so any platform `Healthz` cooldown, such
    as `120 s`, has elapsed since round 1 while B1–B5 continue running): issue
    `gnoi.healthz.Healthz.Check` (`300 s` deadline, if supported for `HC`),
    `gnoi.healthz.Healthz.Get`, and `gnoi.healthz.Healthz.List` (`60 s`
    deadline each). Keeping `Healthz` in `2` spaced rounds avoids compounding
    vendor `Healthz` cooldowns across `20` iterations while validating `Check`,
    `Get`, and `List` under sustained gRIBI/P4RT churn.
* **Step 3 - Stop the Background Load** as described above.
* **Step 4 - Pass/Fail Criteria**:
  * Every gNOI RPC returns gRPC `OK` before its deadline (except `Healthz.Check`
    on platforms where active `Check` is unsupported for `HC`), and:
    * Each supported `Healthz.Check` and both `Healthz.Get` rounds report
      `STATUS_HEALTHY` for `HC`.
    * The `Time` values strictly increase across the `20` iterations, and each
      is within `±5 s` of the `current-datetime` read right after it.
    * Each `Ping` final summary reports `sent == 5` and `received == 5`.
    * Each `Traceroute` returns at least one hop-`1` response, and every hop-`1`
      response has `address` equal to the destination.
    * Neither `Healthz.List` response contains a `STATUS_UNHEALTHY` status whose
      `created` time is later than `T2`.
  * W1–W10 hold.

### TE-1.24.3 - File Service gNOI Operations During gRIBI/P4RT Programming

* `<file_path>` is `<file_dir>/te_1_24_gnoi_blob.bin`, where `<file_dir>` is a
  platform-specific absolute directory that is writable through gNOI `File`
  (for example, a temporary or flash file system), supplied to the test as a
  parameter.
* **Step 1 - Start the Background Load**: start B1–B5, then verify that
  `gnoi.file.File.Stat` of `<file_path>` returns gRPC `NOT_FOUND`.
* **Step 2 - Execute File Lifecycle Cycles Under Churn**: run `5` consecutive
  cycles, each with a new random `10 MiB` (`10,485,760`-byte) payload:
  1. `gnoi.file.File.Put` (`300 s` deadline): an `open` message with
     `remote_file = <file_path>` and `permissions = 644`, `160` `contents`
     messages of `64 KiB` (`65,536` bytes), and a final `hash` message with
     `method = SHA256` and the SHA-256 digest of the payload.
  2. `gnoi.file.File.Stat` of `<file_path>` (`60 s` deadline).
  3. `gnoi.file.File.Get` of `<file_path>` (`300 s` deadline).
  4. `gnoi.file.File.Remove` of `<file_path>` (`60 s` deadline), followed by
     `gnoi.file.File.Stat` of `<file_path>`.
* **Step 3 - Stop the Background Load** as described above.
* **Step 4 - Pass/Fail Criteria**:
  * In all `5` cycles: `Put` returns `OK`; `Stat` returns exactly one
    `StatInfo` with `size = 10,485,760`; `Get` returns exactly `10,485,760`
    bytes whose SHA-256 digest equals that of the payload, and its final `hash`
    message matches the received bytes using the hash method it names; `Remove`
    returns `OK`; and the subsequent `Stat` returns `NOT_FOUND`.
  * W1–W10 hold.

### TE-1.24.4 - Non-Transit Port gNOI Link Qualification Isolation

In this subtest DUT `port4` is the packet link qualification reflector and ATE
`port4` is the packet source; no gNOI endpoint is required on the ATE.

* **Step 1 - Prepare the Qualification**:
  * Start B1–B5.
  * Issue `gnoi.packet_link_qualification.LinkQualification.Capabilities`. The
    response must include `reflector.pmd_loopback` or
    `reflector.asic_loopback`. The reflector type `R` is `pmd_loopback` if it
    is reported, otherwise `asic_loopback`; record the `min_setup_duration` and
    `min_teardown_duration` reported for `R`. `pmd_loopback` is preferred here
    because a port-level loopback returns every frame received from ATE
    `port4`, whereas an ASIC loopback runs in the forwarding engine and may
    reflect only traffic that matches its qualification header-match fields.
  * Issue `LinkQualification.List`; if an entry with ID `te-1-24-4-port4`
    exists, remove it via `LinkQualification.Delete`.
  * Read the DUT `port4` MAC address via `gNMI.Get` of
    `/interfaces/interface/ethernet/state/mac-address`.
* **Step 2 - Run the Qualification on DUT `port4` Under Churn**:
  * Issue `LinkQualification.Create` with one configuration: `id =
    te-1-24-4-port4`, `interface_name` = DUT `port4`, endpoint type `R`, and
    `RPCSyncedTiming` with `pre_sync_duration = 0 s`, `setup_duration` =
    `min_setup_duration`, `duration = 120 s`, `post_sync_duration = 0 s`, and
    `teardown_duration` = `min_teardown_duration`.
  * Poll `LinkQualification.List` every `5 s` until the ID reports
    `QUALIFICATION_STATE_RUNNING`, then start the OTG reflection flow on ATE
    `port4`: raw Ethernet frames (destination MAC = DUT `port4` MAC, source MAC
    `02:00:04:01:01:01`, EtherType `0x88b5`, the IEEE 802 local experimental
    EtherType, which no punt entry matches; `512`-byte frames) at `1,000 fps`
    for a fixed `60,000` frames, transmitted and received on ATE `port4`.
  * While the ID reports `QUALIFICATION_STATE_RUNNING`, read the DUT `port4`
    `oper-status` from `S1`.
  * Keep polling `LinkQualification.List` until the ID reports
    `QUALIFICATION_STATE_COMPLETED` (timeout: `setup_duration + duration +
    teardown_duration + 60 s` after `Create`), then issue
    `LinkQualification.Get` and `LinkQualification.Delete` for the ID and a
    final `LinkQualification.List`.
  * Stop the background load as described above.
* **Step 3 - Pass/Fail Criteria**:
  * `Capabilities`, `Create`, `List`, `Get`, and `Delete` return gRPC `OK`
    before their `30 s` deadline; `CreateResponse.status` and
    `DeleteResponse.results` report code `OK` for the ID.
  * The ID never reports `QUALIFICATION_STATE_ERROR`; `Get` returns `state =
    QUALIFICATION_STATE_COMPLETED`, no error `status`, and `interface_name` =
    DUT `port4`; the final `List` no longer contains the ID.
  * If `R` is `pmd_loopback`, the reflection flow reports `frames_rx ==
    frames_tx == 60,000` (every frame was returned). If `R` is
    `asic_loopback`, the reflection flow `frames_rx` is logged for information
    only and is not a pass/fail criterion; the qualification is judged by the
    `Create`, `List`, `Get`, and `Delete` results above.
  * DUT `port4` reports `oper-status = TESTING` while the qualification is
    running (except on platforms that do not report `TESTING` for an interface
    under qualification) and `UP` within `120 s` after `Delete`; these are the
    only `oper-status` changes allowed by W7.
  * W1–W10 hold: DUT `port1`–`port3` stay `UP`, the protected transit flows,
    LLDP `PacketIn`, and `PacketOut` to `port3` have `0%` loss, and gRIBI and
    P4RT programming stay within the latency bounds.

### Cleanup

* Stop any OTG flow, `PacketOut` sender, or churn loop that is still running.
* If `gnoi.file.File.Stat` finds `<file_path>`, remove it via
  `gnoi.file.File.Remove`.
* If `LinkQualification.List` contains `te-1-24-4-port4`, delete it via
  `LinkQualification.Delete` and verify that DUT `port4` returns to
  `oper-status = UP`.
* Delete all non-const entries (`is_const = false`) currently in
  `acl_wbb_ingress_table` via `P4Runtime.Write` (`DELETE`). If `P_pre` is not
  empty, re-push it via `P4Runtime.SetForwardingPipelineConfig`
  (`VERIFY_AND_COMMIT`). Then re-insert (`INSERT`) every entry of `E_pre` that
  is not already present, and verify via `P4Runtime.Read` (`is_const = false`)
  that the table matches `E_pre`.
* Flush the `DEFAULT` network instance via `gRIBI.Flush`
  (`election.id = {high: 0, low: 1}` or `override = {}`) and verify that
  `gRIBI.Get` returns `0` entries.
* Close `S1` and the `gRIBI` and `P4RT` sessions.
* Restore `/lldp/config/enabled` to its pre-test value, stop OTG protocols, and
  restore the baseline DUT configuration.

## Canonical OC

```json
{
  "components": {
    "component": [
      {
        "config": {
          "name": "IntegratedCircuit0"
        },
        "integrated-circuit": {
          "config": {
            "node-id": "1"
          }
        },
        "name": "IntegratedCircuit0"
      }
    ]
  },
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "Transit Ingress Port 1",
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
          "description": "Protected Transit Egress Port 2",
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
          "description": "Churn Next Hop and PacketOut Egress Port 3",
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
          "description": "Non-Transit gNOI Port 4",
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
        "name": "DEFAULT"
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /components/component/integrated-circuit/config/node-id:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/state/last-reboot-time:
    platform_type: ["CHASSIS", "CONTROLLER_CARD", "INTEGRATED_CIRCUIT", "LINECARD"]
  /components/component/state/oper-status:
    platform_type: ["CHASSIS", "CONTROLLER_CARD", "FAN", "INTEGRATED_CIRCUIT", "LINECARD", "POWER_SUPPLY"]
  /components/component/state/type:
    platform_type: ["CHASSIS", "CONTROLLER_CARD"]
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/id:
  /interfaces/interface/ethernet/state/mac-address:
  /interfaces/interface/state/counters/in-pkts:
  /interfaces/interface/state/counters/out-pkts:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /lldp/config/enabled:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/programmed-id:
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address:
  /system/processes/process/state/name:
  /system/processes/process/state/pid:
  /system/processes/process/state/start-time:
  /system/state/boot-time:
  /system/state/current-datetime:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
      on_change: true
      sample: true
  gnoi:
    file.File.Get:
    file.File.Put:
    file.File.Remove:
    file.File.Stat:
    healthz.Healthz.Check:
    healthz.Healthz.Get:
    healthz.Healthz.List:
    packet_link_qualification.LinkQualification.Capabilities:
    packet_link_qualification.LinkQualification.Create:
    packet_link_qualification.LinkQualification.Delete:
    packet_link_qualification.LinkQualification.Get:
    packet_link_qualification.LinkQualification.List:
    system.System.Ping:
    system.System.Time:
    system.System.Traceroute:
  gribi:
    gRIBI.Flush:
    gRIBI.Get:
    gRIBI.Modify:
  # P4Runtime (p4.v1.P4Runtime) RPCs exercised; not listed above because they
  # are not part of the OpenConfig RPC catalog checked by this block:
  # GetForwardingPipelineConfig, SetForwardingPipelineConfig, StreamChannel
  # (arbitration, PacketIn, PacketOut), Write, Read.
```

## Required DUT platform

* FFF
