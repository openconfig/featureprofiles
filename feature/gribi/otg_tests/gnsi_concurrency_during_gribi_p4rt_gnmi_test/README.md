# TE-1.25: gNSI Operations During gRIBI/P4RT Programming and gNMI Streaming

## Summary

This test validates control-plane security lifecycle resilience and data-plane
isolation when `gNSI` credential and authorization updates
(`gnsi.certz.v1.Certz.Rotate` and `gnsi.authz.v1.Authz.Rotate`) execute
concurrently with a defined `gRIBI` Abstract Forwarding Table (AFT) route churn
(about `1,000` AFT operations/s), `P4Runtime` (`P4RT`) Packet-I/O punt-table
churn (`wbb.p4info.pb.txt` `acl_wbb_ingress_table`) with `PacketIn` and
`PacketOut` traffic, and continuous `gNMI` streaming telemetry (an `ON_CHANGE`
stream monitoring interface operational state and the protected AFT entries,
plus a `10 s` `SAMPLE` counter stream). It
verifies that established mTLS gRPC sessions stay up with bounded latency (p99
at most `2x` the baseline p99, with an absolute floor), transit forwarding,
`PacketIn` punting and `PacketOut` injection sustain zero packet loss, newly
established connections immediately enforce the rotated certificates, trust
bundles and authorization policies, and unfinalized rotations roll back cleanly
while the churn continues.

## Testbed type

* [`TESTBED_DUT_ATE_4LINKS`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

```text
                             +---------+
  [ ATE port1 ] ------------ |         | ------------ [ ATE port2 ] (Protected Transit)
  (Transit & Punt Src)       |   DUT   |
                             | (mTLS   | ------------ [ ATE port3 ] (Churn NH & PacketOut Egress)
                             |  gNxI)  |
                             |         | ------------ [ ATE port4 ] (PacketOut Isolation Monitor)
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
  * **Link 3 (Churn Next Hop & PacketOut Egress)**: DUT `port3`
    (`192.0.2.9/30`, `2001:db8:3::1/126`, P4RT `id = 3`) <-> ATE `port3`
    (`192.0.2.10/30`, `2001:db8:3::2/126`, MAC `02:00:03:01:01:01`).
  * **Link 4 (PacketOut Isolation Monitor)**: DUT `port4` (`192.0.2.13/30`,
    `2001:db8:4::1/126`, P4RT `id = 4`) <-> ATE `port4` (`192.0.2.14/30`,
    `2001:db8:4::2/126`, MAC `02:00:04:01:01:01`). No flow is sent to `port4`;
    it verifies that `PacketOut` frames egress only the requested port.
* Configure `/components/component/integrated-circuit/config/node-id` (`1`) on
  the forwarding integrated circuit (`IntegratedCircuit0`) and interface
  `/interfaces/interface/config/id` (`1`–`4`) via `gNMI.Set`.
* Record `/lldp/config/enabled` and disable on-box LLDP processing
  (`/lldp/config/enabled = false`) via `gNMI.Set`, as in P4RT-7.1/P4RT-7.2, so
  that LLDP frames reach the controller only through the P4RT punt entry.

#### Credentials and identities

* Input argument `test_infra_id`: the SPIFFE ID presented by the test
  infrastructure clients (as in the gNSI Authz tests). The test-generated
  `infra` client certificates carry the same SPIFFE ID.
* Generate the following X.509 credentials before the test. Every server
  certificate has a unique serial number and the Subject Alternative Name (SAN)
  that the test clients use to reach the DUT.

| Credential | Key | Issuer | Use |
| --- | --- | --- | --- |
| `ca-v1` | RSA 2048 | self-signed root | Trust anchor installed in setup |
| `ca-v2` | RSA 2048 | self-signed root | CA introduced by the RSA pass of TE-1.25.2 |
| `ca-v3` | ECDSA P-256 | self-signed root | CA introduced by the ECDSA pass of TE-1.25.2 |
| `ca-untrusted-rsa`, `ca-untrusted-ecdsa` | RSA 2048, ECDSA P-256 | self-signed roots | Never uploaded; negative checks |
| `server-v1-rsa` | RSA 2048 | `ca-v1` | Server credential installed in setup |
| `server-v2-rsa` | RSA 2048 | `ca-v2` | Server credential of the RSA pass |
| `server-v3-ecdsa` | ECDSA P-256 | `ca-v3` | Server credential of the ECDSA pass |
| `server-v4-temp` | ECDSA P-256 | `ca-v3` | Uploaded but never finalized (TE-1.25.4) |

* Client certificates carry the SPIFFE ID as a URI SAN. Each identity below has
  one certificate issued by each of `ca-v1`, `ca-v2` and `ca-v3`, written
  `<identity>@<ca>` (for example `controller@ca-v1`); the `untrusted` identity
  has certificates issued only by `ca-untrusted-rsa` and `ca-untrusted-ecdsa`.

| Identity | SPIFFE ID |
| --- | --- |
| `infra` | `<test_infra_id>` |
| `controller` | `spiffe://example.org/ns/sdn/sa/controller` |
| `auditor` | `spiffe://example.org/ns/sec/sa/auditor` |
| `viewer` | `spiffe://example.org/ns/ops/sa/viewer` |
| `untrusted` | `spiffe://example.org/ns/test/sa/untrusted` |

* Test clients validate the DUT server certificate against `ca-v1`, `ca-v2`
  and `ca-v3`.
* Credential selection rule: unless a step names another certificate, every new
  connection opened after the profile binding in setup uses an `@ca-v1`
  certificate. From the `FinalizeRequest` of each TE-1.25.2 stage A onward,
  every new connection (verification clients, any reconnection of a persistent
  client, and Cleanup) uses the certificate issued by the CA introduced in that
  stage, so the controller and the Cleanup client always hold a credential that
  the DUT trusts. The persistent controller sessions opened in setup keep their
  `controller@ca-v1` credential for the whole test.

#### Authorization policies

Policies use the gRPC authorization policy JSON schema (gRFC A43) referenced by
`gnsi.authz.v1`. Each row below is one rule whose `source.principals` is the
listed principal and whose `request.paths` are the listed paths. Deny rules are
evaluated before allow rules, and a finalized policy denies every request that
matches no allow rule.

| Rule | Type | Principal | Paths | `policy-v1` | `policy-v2` | `policy-v3-temp` |
| --- | --- | --- | --- | --- | --- | --- |
| `allow-test-infra` | allow | `<test_infra_id>` | `/*` | yes | yes | yes |
| `allow-sdn-controller` | allow | `spiffe://example.org/ns/sdn/sa/controller` | `/gnmi.gNMI/*`, `/gribi.gRIBI/*`, `/p4.v1.P4Runtime/*` | yes | yes | yes |
| `allow-sec-auditor` | allow | `spiffe://example.org/ns/sec/sa/auditor` | `/gnmi.gNMI/Get`, `/gribi.gRIBI/Get`, `/gnsi.authz.v1.Authz/Get` | yes | yes | yes |
| `allow-ops-viewer` | allow | `spiffe://example.org/ns/ops/sa/viewer` | `/gnmi.gNMI/Get` | no | yes | no |
| `deny-auditor-gribi-get` | deny | `spiffe://example.org/ns/sec/sa/auditor` | `/gribi.gRIBI/Get` | no | yes | yes |

| Policy | `name` | `version` | `created_on` (s) | Expected `grpc-authz-policy-created-on` (ns) |
| --- | --- | --- | --- | --- |
| `policy-v1` | `te125-policy-v1` | `authz-v1` | `1700000000` | `1700000000000000000` |
| `policy-v2` | `te125-policy-v2` | `authz-v2` | `1700000100` | `1700000100000000000` |
| `policy-v3-temp` | `te125-policy-v3-temp` | `authz-v3-temp` | `1700000200` | Not applicable (never finalized) |

`gnsi.authz.v1` defines `created_on` in seconds since the Unix epoch, while
`/system/aaa/authorization/state/grpc-authz-policy-created-on` is a
`timeticks64` value in nanoseconds since the Unix epoch, so the expected leaf
value is `created_on x 1,000,000,000`. For example, `policy-v2` is encoded as:

```json
{
  "name": "te125-policy-v2",
  "deny_rules": [
    {
      "name": "deny-auditor-gribi-get",
      "source": {
        "principals": ["spiffe://example.org/ns/sec/sa/auditor"]
      },
      "request": {
        "paths": ["/gribi.gRIBI/Get"]
      }
    }
  ],
  "allow_rules": [
    {
      "name": "allow-test-infra",
      "source": {
        "principals": ["<test_infra_id>"]
      },
      "request": {
        "paths": ["/*"]
      }
    },
    {
      "name": "allow-sdn-controller",
      "source": {
        "principals": ["spiffe://example.org/ns/sdn/sa/controller"]
      },
      "request": {
        "paths": ["/gnmi.gNMI/*", "/gribi.gRIBI/*", "/p4.v1.P4Runtime/*"]
      }
    },
    {
      "name": "allow-sec-auditor",
      "source": {
        "principals": ["spiffe://example.org/ns/sec/sa/auditor"]
      },
      "request": {
        "paths": [
          "/gnmi.gNMI/Get",
          "/gribi.gRIBI/Get",
          "/gnsi.authz.v1.Authz/Get"
        ]
      }
    },
    {
      "name": "allow-ops-viewer",
      "source": {
        "principals": ["spiffe://example.org/ns/ops/sa/viewer"]
      },
      "request": {
        "paths": ["/gnmi.gNMI/Get"]
      }
    }
  ]
}
```

#### gRPC servers, pre-test state and gNSI bootstrap

* mTLS is a precondition (as in Certz-3): this test does not modify
  `config/enable` or `config/transport-security`, because changing transport
  security on a live server would drop the session that issues the change. Via
  `gNMI.Get`, identify the bound servers: every
  `/system/grpc-servers/grpc-server` whose `config/services` includes `GNMI`,
  `GNSI`, `GRIBI` or `P4RT` (on platforms that do not support the `services`
  leaf, every listed gRPC server). Verify that each bound server has
  `config/enable = true` and `config/transport-security = true`.
* The certz state leaves (`state/ssl-profile-id`, `state/certificate-*`,
  `state/ca-trust-bundle-*` and `state/counters/connection-rejects`) are
  defined only for servers that host gNSI (`services` includes `GNSI`), so they
  are read via `gNMI.Get` from the bound servers that host gNSI. In OpenConfig,
  `authz-policy-counters` also has a `when` statement gated on `GNSI`, though
  on multi-server platforms some implementations expose `authz-policy-counters`
  on additional bound servers (such as a separate `GRIBI` server) while others
  expose it only on `GNSI`-hosting servers; therefore TE-1.25.3 aggregates
  `authz-policy-counters` across every bound server that exposes the
  `authz-policy-counters` container and issues denied RPCs against both
  `/gribi.gRIBI/Get` and `/gnsi.authz.v1.Authz/Get`. The TLS connection checks
  in TE-1.25.2 and TE-1.25.4 open `gNMI` connections to the verification
  server: the bound server that hosts gNMI and gNSI (the single `DEFAULT`
  server in the canonical configuration). On platforms that do not support the
  `services` leaf, the bound server named `DEFAULT` (or the only bound server)
  is the server that hosts gNSI and the verification server.
* Capture the pre-test state needed by Cleanup:
  * Each bound server's `config/certificate-id` (or its absence) and
    `state/certificate-id`, and `state/ssl-profile-id` of each bound server
    that hosts gNSI.
  * The active authorization policy via `gnsi.authz.v1.Authz.Get` (`policy`,
    `version` and `created_on`), or `FAILED_PRECONDITION` if no policy is set.
  * `gnsi.certz.v1.Certz.GetProfileList`. If `te125_profile` is already listed
    (left over from an aborted run) and no server is bound to it, delete it
    with `gnsi.certz.v1.Certz.DeleteProfile` first.
* Never modify `system_default_profile`: it is vendor provided and cannot be
  changed or deleted per the gNSI certz specification. The certz service
  accepts one RPC at a time, so all certz RPCs in this test are sequential.
* Create the test SSL profile with `gnsi.certz.v1.Certz.AddProfile`
  (`ssl_profile_id = "te125_profile"`) and verify that `Certz.GetProfileList`
  lists it.
* Populate it with one `gnsi.certz.v1.Certz.Rotate` stream
  (`ssl_profile_id = "te125_profile"`, `force_overwrite = false`): send an
  `UploadRequest` containing an `Entity.certificate_chain` (`CertificateChain`)
  with the `server-v1-rsa` certificate chain and private key
  (`version = "cert-v1-rsa"`, `created_on = 1700001000`) and an
  `Entity.trust_bundle_pkcs7` (`TrustBundle` whose `pkcs7_block` is a
  PEM-encoded PKCS#7 certificate-only bundle — not `Entity.trust_bundle` /
  `CertificateChain`, which in `gnsi.certz.v1` represents only a singly-linked
  signing chain and cannot encode multiple independent root CAs) containing
  `ca-v1` (`version = "bundle-v1"`, `created_on = 1700001000`), wait for
  `UploadResponse`, and send `FinalizeRequest`. No server uses the profile yet,
  so no pre-finalize connection check applies.
* Via `gNMI.Set`, set `/system/grpc-servers/grpc-server/config/certificate-id`
  to `"te125_profile"` on every bound server, then verify via `gNMI.Get` that
  each one reports `state/certificate-id = "te125_profile"` and that each bound
  server that hosts gNSI reports `state/ssl-profile-id = "te125_profile"`,
  `state/certificate-version = "cert-v1-rsa"` and
  `state/ca-trust-bundle-version = "bundle-v1"`. If the binding closes existing
  connections, the test re-establishes them with `@ca-v1` certificates before
  continuing.
* Over a new connection as `infra@ca-v1`, open `gnsi.authz.v1.Authz.Rotate`
  (`force_overwrite = false`), upload `policy-v1`, verify that
  `gnsi.authz.v1.Authz.Probe` returns `ACTION_PERMIT` (version `authz-v1`) for
  `spiffe://example.org/ns/sdn/sa/controller` on `/gribi.gRIBI/Modify`, and
  send `FinalizeRequest`.

#### Persistent sessions

* Establish persistent mTLS client sessions authenticated as `controller@ca-v1`:
  * **gNMI Streaming Client**: Open two persistent `gNMI.Subscribe` (`STREAM`)
    sessions:
    1. `ON_CHANGE` subscription scoped to `/interfaces/interface/state/oper-status`
       for DUT `port1`–`port4` and the protected `DEFAULT` AFT entries and their
       next-hop-groups/next-hops
       (`/network-instances/network-instance[name=DEFAULT]/afts/ipv4-unicast/ipv4-entry[prefix=198.51.100.0/24]/state/...`,
       `/network-instances/network-instance[name=DEFAULT]/afts/ipv6-unicast/ipv6-entry[prefix=2001:db8:100::/64]/state/...`,
       `/network-instances/network-instance[name=DEFAULT]/afts/next-hop-groups/next-hop-group[id=...]/...`,
       and `/network-instances/network-instance[name=DEFAULT]/afts/next-hops/next-hop[index=...]/...`),
       rather than subscribing `ON_CHANGE` on the root `/afts/` tree (avoiding
       `~1,000` churn notifications/s). Certz and authz state leaves under
       `/system/grpc-servers/grpc-server/state/` and
       `/system/aaa/authorization/state/` are not marked
       `oc-ext:telemetry-on-change` in OpenConfig YANG and are queried via
       `gNMI.Get`.
    2. `SAMPLE` subscription (`sample_interval = 10 s`,
       `suppress_redundant = false`) on `/interfaces/interface/state/counters/`
       (for DUT `port1`–`port4`) and
       `/system/grpc-servers/grpc-server/authz-policy-counters/`.
  * **gRIBI Client**: Connect in `SINGLE_PRIMARY` mode with `PRESERVE`
    persistence and `ack_type = RIB_AND_FIB_ACK`
    (`election_id = {high: 0, low: 1}`), and clear initial AFT state via
    `gRIBI.Flush` on `DEFAULT` with `election.id = {high: 0, low: 1}` (or
    `override = {}`).
  * **P4RT Client**: Open `P4Runtime.StreamChannel` (`device_id = 1`,
    `election_id = {high: 0, low: 1}`) and complete primary controller
    arbitration. **Before** pushing the test pipeline, capture the pre-test
    forwarding pipeline config `P_pre` via
    `P4Runtime.GetForwardingPipelineConfig` (`response_type = ALL`; if
    `FAILED_PRECONDITION` / no pipeline is set, record `P_pre = unset`) and, if
    a pipeline is present, read and record all existing dynamic
    (`is_const = false`) table entries `E_pre` via `P4Runtime.Read`. Then push
    the WBB forwarding pipeline (`feature/p4rt/data/wbb.p4info.pb.txt`,
    defining `acl_wbb_ingress_table` with `size = 8`, actions
    `acl_wbb_ingress_copy` and `acl_wbb_ingress_trap`, and `packet_in` /
    `packet_out` metadata) via `P4Runtime.SetForwardingPipelineConfig`
    (`VERIFY_AND_COMMIT`).
* Empty dynamic entries in the punt table so all 8 dynamic slots that
  `acl_wbb_ingress_table` admits are available: `P4Runtime.Read` the table,
  ignore immutable default/const entries (`is_const = true`), delete any
  platform-installed non-const (`is_const = false`) entries via
  `P4Runtime.Write` (`DELETE`) (adding them to `E_pre` if `P_pre` already used
  the same WBB P4Info), and verify that a second `P4Runtime.Read` returns `0`
  non-const (`is_const = false`) entries.
* Verify all 4 DUT ports report `oper-status = UP` via the `ON_CHANGE`
  `gNMI.Subscribe` stream, start OTG protocols, and explicitly resolve IPv4 ARP
  and IPv6 Neighbor Discovery (ND) across all 4 links before starting traffic.
* Take the stability snapshot: `/system/state/boot-time` and the `name`, `pid`
  and `start-time` of every `/system/processes/process` entry.

#### Workload definitions

* **Protected flows** (ATE `port1` -> ATE `port2`): IPv4 to `198.51.100.1`
  (`TTL = 64`) and IPv6 to `2001:db8:100::1` (`HopLimit = 64`), `512`-byte
  frames at `5,000 fps` each (`10,000 fps` aggregate).
* **LLDP flow** (ATE `port1`): `10 fps` of LLDP frames (destination MAC
  `01:80:c2:00:00:0e`, source MAC `02:00:01:01:01:01`, EtherType `0x88cc`, and
  an LLDPDU with Chassis ID, Port ID, TTL and End TLVs).
* **PacketOut generator** (P4RT controller): `10` `PacketOut` messages/s on the
  persistent `StreamChannel`, each carrying a fixed LLDP frame (destination MAC
  `01:80:c2:00:00:0e`, source MAC `02:00:00:00:01:25`, EtherType `0x88cc`) with
  `packet_out` metadata `egress_port = "3"` (DUT `port3`) and
  `submit_to_ingress = 0`.
* **gRIBI churn** (persistent gRIBI session, `DEFAULT` network instance):
  * `NextHop 100` (`192.0.2.10`, ATE `port3`) -> `NextHopGroup 100` and
    `NextHop 101` (`2001:db8:3::2`, ATE `port3`) -> `NextHopGroup 101` are
    programmed in TE-1.25.1 Step 1 and are not churned; each churn
    `ModifyRequest` contains only IPv4/IPv6 entry operations.
  * Each cycle `ADD`s and then `DELETE`s `2,000` IPv4 `/24` entries
    (`100.64.0.0/24`–`100.71.207.0/24`) pointing to `NextHopGroup 100` and
    `2,000` IPv6 `/64` entries (`2001:db8:1000::/64`–`2001:db8:1000:7cf::/64`)
    pointing to `NextHopGroup 101`, i.e., `8,000` AFT operations per cycle.
  * Each `ModifyRequest` carries `100` AFT operations of one address family
    (IPv4 first). A new request is sent every `100 ms` (`1,000` AFT
    operations/s) unless `10` requests are in flight; a request is in flight
    until every one of its operations has a final `AFTResult` (`FIB_PROGRAMMED`,
    `FAILED` or `FIB_FAILED`). The `DELETE` phase starts after every `ADD` of
    the cycle is acknowledged, and the next cycle starts after every `DELETE` is
    acknowledged.
  * The churn never touches `NextHop 1`/`2`, `NextHopGroup 10`/`11` or the
    protected prefixes.
  * The ack latency of an AFT operation is the time from sending the
    `ModifyRequest` that carries it to receiving its `FIB_PROGRAMMED`
    `AFTResult`, measured on the client clock.
* **P4RT churn** (persistent P4RT session): the 7 untagged (`outer_vlan_id`
  wildcarded) non-LLDP entries admitted by the `acl_wbb_ingress_table`
  `@entry_restriction`, all with `priority = 1`: ND/GDP
  (`ether_type == 0x6007 & 0xffff`) and six traceroute entries
  (`is_ipv4 == 1` or `is_ipv6 == 1`, each with `ttl == 0`, `1` or `2 & 0xff`).
  A cycle starts every `1 s`, or immediately after the previous cycle if that
  cycle took longer:
  1. `P4Runtime.Write` `INSERT` of the 7 entries with action
     `acl_wbb_ingress_trap`.
  2. `P4Runtime.Write` `MODIFY` of the 7 entries to action
     `acl_wbb_ingress_copy` (match key and priority unchanged). On platforms
     that do not support P4RT `MODIFY`, this step is a `P4Runtime.Write`
     `DELETE` of the 7 entries followed by a separate `P4Runtime.Write`
     `INSERT` of the 7 entries with action `acl_wbb_ingress_copy`.
  3. `P4Runtime.Read` on `acl_wbb_ingress_table` returns exactly 8 non-const
     (`is_const = false`) entries: the protected LLDP entry
     (`acl_wbb_ingress_trap`) and the 7 churn entries (`acl_wbb_ingress_copy`).
  4. `P4Runtime.Write` `DELETE` of the 7 entries.
  5. `P4Runtime.Read` on `acl_wbb_ingress_table` returns only the protected
     LLDP entry among non-const (`is_const = false`) entries.
  * Each `P4Runtime.Write` is one `WriteRequest` with 7 updates
    (`Atomicity = CONTINUE_ON_ERROR`); its latency is the time from sending the
    request to receiving the response.
* Every unary RPC issued by the test uses a `10 s` deadline.

#### Measurement windows and common pass/fail criteria

Each subtest runs in one or more measurement windows:

1. Start packet captures on ATE `port3` and ATE `port4`, then start the
   protected flows, the LLDP flow and the PacketOut generator.
2. Execute the subtest steps. Once started in TE-1.25.1, the gRIBI and P4RT
   churn runs continuously until TE-1.25.4 Step 3.
3. At least `10 s` after the last step and no earlier than `60 s` after the
   window started, stop the flows and the PacketOut generator, wait until OTG
   reports every flow stopped with stable counters plus `2 s` for frames in
   flight, then stop the captures.

The following criteria apply to every window and are evaluated after traffic
stops (C4 and C5 apply to windows with churn):

* **C1 - Protected forwarding**: for each protected flow, the OTG frames
  received on ATE `port2` equal the frames transmitted (`0` lost).
* **C2 - PacketIn**: the number of `PacketIn` messages received by the
  controller with EtherType `0x88cc`, source MAC `02:00:01:01:01:01` and
  `ingress_port` metadata `"1"` equals the OTG transmit count of the LLDP flow
  (`0` lost). Other `PacketIn` messages, for example hits on the churned
  traceroute entries, are ignored.
* **C3 - PacketOut**: the ATE `port3` capture contains exactly as many frames
  with EtherType `0x88cc` and source MAC `02:00:00:00:01:25` as `PacketOut`
  messages sent in the window, and the ATE `port4` capture contains none.
* **C4 - gRIBI churn**: every churn AFT operation returns `FIB_PROGRAMMED` (no
  `FAILED` or `FIB_FAILED`), no ack latency exceeds `5 s`, the p99 ack latency
  is at most `max(2 x L99_gribi_base, L99_gribi_base + 50 ms)` (from TE-1.25.2
  onward), and the `Modify` stream stays open.
* **C5 - P4RT churn**: every `P4Runtime.Write` returns `OK` with no per-update
  error, every `P4Runtime.Read` returns the expected entries, the p99 `Write`
  latency is at most `max(2 x L99_p4rt_base, L99_p4rt_base + 20 ms)` (from
  TE-1.25.2 onward), and the `StreamChannel` stays open with the client still
  primary.
* **C6 - gNMI streams**: both persistent `gNMI.Subscribe` streams stay open,
  neither sends a second `sync_response`, and the `ON_CHANGE` stream emits no
  deletion or `next-hop-group` modification for the protected `DEFAULT` AFT
  prefixes (`198.51.100.0/24` and `2001:db8:100::/64`); in the `SAMPLE` stream
  (`sample_interval = 10 s`), consecutive updates of `in-pkts` and of
  `out-pkts` of each of DUT `port1`–`port4` arrive at most `20 s` apart (client
  receive time).
* **C7 - No unexpected RPC errors**: no RPC returns a non-`OK` status, including
  `DEADLINE_EXCEEDED` and `UNAVAILABLE`, unless the step expects it.
* **C8 - Stability**: compared with the setup snapshot,
  `/system/state/boot-time` is unchanged, and every process whose `start-time`
  was at least `10` minutes before the snapshot is either still listed with the
  same `pid` and `start-time` or, if it exited, no process with the same `name`
  started after the snapshot.

### TE-1.25.1 - Baseline Protected Forwarding, Telemetry, and Control-Plane Load

* **Step 1 - Program Protected Baseline Entries**:
  * Via `gRIBI.Modify` (`ADD`) in `DEFAULT`, program the protected entries
    `NextHop 1` (`192.0.2.6`, ATE `port2`) -> `NextHopGroup 10` ->
    `198.51.100.0/24` and `NextHop 2` (`2001:db8:2::2`, ATE `port2`) ->
    `NextHopGroup 11` -> `2001:db8:100::/64`, and the churn next hops
    `NextHop 100` (`192.0.2.10`, ATE `port3`) -> `NextHopGroup 100` and
    `NextHop 101` (`2001:db8:3::2`, ATE `port3`) -> `NextHopGroup 101`, which
    no prefix references until the churn starts. Acknowledgements are awaited
    once after the full NH -> NHG -> prefix sequence (timeout `60 s`): every
    operation returns `FIB_PROGRAMMED`, except that on platforms that do not
    send `FIB_PROGRAMMED` for NH/NHG entries that no prefix references yet, the
    `NextHop 100` / `101` and `NextHopGroup 100` / `101` operations return
    `RIB_PROGRAMMED`.
  * Confirm installation via `gRIBI.Get` (`DEFAULT`, AFT type `ALL`), which
    returns exactly these 10 entries, each with `rib_status = PROGRAMMED` and,
    for the 6 protected entries, `fib_status = PROGRAMMED`, and via the
    `ON_CHANGE` `gNMI.Subscribe` stream, in which `198.51.100.0/24` references
    a next-hop-group with `programmed-id = 10` whose only next hop has
    `ip-address = 192.0.2.6`, and `2001:db8:100::/64` references a
    next-hop-group with `programmed-id = 11` whose only next hop has
    `ip-address = 2001:db8:2::2`.
  * Via `P4Runtime.Write` (`INSERT`, `Atomicity = CONTINUE_ON_ERROR`), install
    the protected `acl_wbb_ingress_table` punt entry matching LLDP frames
    (`ether_type == 0x88cc & 0xffff`, `priority = 1`, action
    `acl_wbb_ingress_trap`), and verify that `P4Runtime.Read` returns exactly
    this entry among non-const (`is_const = false`) entries.
* **Step 2 - Start Protected Data-Plane and Packet-I/O Traffic**:
  * Run a `60 s` measurement window (`W0`) without control-plane churn.
* **Step 3 - Start Continuous Control-Plane Churn**:
  * Open measurement window `W1`. Over the persistent `controller@ca-v1`
    sessions, start the gRIBI churn and the P4RT churn (kept active across
    TE-1.25.2–TE-1.25.4), and close `W1` after `120 s` of churn.
  * From `W1`, record the median and 99th-percentile gRIBI ack latency
    (`L50_gribi_base`, `L99_gribi_base`) and `P4Runtime.Write` latency
    (`L50_p4rt_base`, `L99_p4rt_base`).
* **Step 4 - Pass/Fail Criteria**:
  * Every Step 1 check passes.
  * `W0` meets C1–C3 and C6–C8.
  * `W1` meets C1–C8, with C4 and C5 applied without the relative latency
    bounds.

### TE-1.25.2 - Concurrent mTLS Certificate Rotation (gNSI.certz)

Run this procedure twice, first for the `RSA` pass and then for the `ECDSA`
pass, each in its own measurement window while the gRIBI churn, the P4RT churn,
the `gNMI.Subscribe` streams and the OTG flows remain active. Each pass
introduces a new CA (`ca-new`) and retires the previously trusted CA (`ca-old`)
with two separate `gnsi.certz.v1.Certz.Rotate` streams on `te125_profile`
(`force_overwrite = false`), each completed with `FinalizeRequest`. `infra`
issues the `Certz.Rotate` RPCs. All entity versions are unique, so no upload
collides with a version that is already in use.

| Pass | Stage | `UploadRequest` entities | `certificate-version` (`created_on`) | `ca-trust-bundle-version` (`created_on`) | Trusted CAs after `FinalizeRequest` |
| --- | --- | --- | --- | --- | --- |
| RSA (`ca-old = ca-v1`, `ca-new = ca-v2`) | A | `Entity.certificate_chain` (`server-v2-rsa` chain and key); `Entity.trust_bundle_pkcs7` (`pkcs7_block` with {`ca-v1`, `ca-v2`}) | `cert-v2-rsa` (`1700001100`) | `bundle-v1-v2` (`1700001100`) | `ca-v1`, `ca-v2` |
| RSA | B | `Entity.trust_bundle_pkcs7` (`pkcs7_block` with {`ca-v2`}) | `cert-v2-rsa` (unchanged) | `bundle-v2` (`1700001200`) | `ca-v2` |
| ECDSA (`ca-old = ca-v2`, `ca-new = ca-v3`) | A | `Entity.certificate_chain` (`server-v3-ecdsa` chain and key); `Entity.trust_bundle_pkcs7` (`pkcs7_block` with {`ca-v2`, `ca-v3`}) | `cert-v3-ecdsa` (`1700001300`) | `bundle-v2-v3` (`1700001300`) | `ca-v2`, `ca-v3` |
| ECDSA | B | `Entity.trust_bundle_pkcs7` (`pkcs7_block` with {`ca-v3`}) | `cert-v3-ecdsa` (unchanged) | `bundle-v3` (`1700001400`) | `ca-v3` |

Below, `server-new` is `server-v2-rsa` in the RSA pass and `server-v3-ecdsa` in
the ECDSA pass, and `<alg>` is `rsa` or `ecdsa`. Every trust-bundle upload uses
`Entity.trust_bundle_pkcs7` (`TrustBundle` with PEM-encoded PKCS#7
certificate-only `pkcs7_block`, not `Entity.trust_bundle` /
`CertificateChain`). Certz `created_on` values are
seconds since the Unix epoch, while `state/certificate-created-on`,
`state/ca-trust-bundle-created-on` and `state/certificate-expires-on` are
`timeticks64` values in nanoseconds, so their expected values are
`created_on x 1,000,000,000` and the `NotAfter` time of `server-new` in
nanoseconds.

* **Step 1 - Stage A: Rotate the Server Certificate and Add `ca-new`**:
  1. Open the pass's measurement window.
  2. Open `Certz.Rotate` and send the stage A `UploadRequest`
     (`Entity.certificate_chain` + `Entity.trust_bundle_pkcs7`) with the
     versions and `created_on` values from the table, then wait for
     `UploadResponse`.
  3. Before sending `FinalizeRequest`:
     * A new connection as `auditor@<ca-new>` to the verification server
       completes the TLS handshake, the DUT presents `server-new` (serial
       number and SAN match), and `gNMI.Get` of `/system/state/current-datetime`
       returns `OK`.
     * A new connection as `infra@<ca-old>` also succeeds (`gNMI.Get` returns
       `OK`), because `ca-old` is still trusted.
     * A new connection as `untrusted@ca-untrusted-<alg>` fails the TLS
       handshake, so no RPC is served, and
       `/system/grpc-servers/grpc-server/state/counters/connection-rejects` of
       the verification server increases by at least `1` over the value read
       immediately before the attempt.
  4. Send `FinalizeRequest` and close the send direction; the `Certz.Rotate`
     RPC ends with status `OK`.
  5. Verify on every bound server that hosts gNSI, via `gNMI.Get`, that
     `state/certificate-version`, `state/certificate-created-on`,
     `state/certificate-expires-on`, `state/ca-trust-bundle-version` and
     `state/ca-trust-bundle-created-on` equal the stage A values.
* **Step 2 - Stage B: Remove `ca-old`**:
  1. New connections now use `@<ca-new>` certificates (credential selection
     rule).
  2. Open `Certz.Rotate` and send the stage B `UploadRequest`
     (`Entity.trust_bundle_pkcs7` containing {`ca-new`} only), then wait for
     `UploadResponse`.
  3. Before sending `FinalizeRequest`:
     * A new connection as `auditor@<ca-new>` to the verification server
       succeeds (`gNMI.Get` returns `OK`) and the DUT still presents
       `server-new`.
     * A new connection as `controller@<ca-old>` to the verification server
       fails the TLS handshake, and its `connection-rejects` increases by at
       least `1` over the value read immediately before the attempt.
  4. Send `FinalizeRequest` and close the send direction; the `Certz.Rotate`
     RPC ends with status `OK`.
  5. Verify on every bound server that hosts gNSI, via `gNMI.Get`, that
     `state/ca-trust-bundle-version` and `state/ca-trust-bundle-created-on`
     equal the stage B values and that `state/certificate-version` and
     `state/certificate-created-on` are unchanged.
  6. Close the window at least `10 s` after `FinalizeRequest`.
* **Step 3 - Pass/Fail Criteria** (each pass):
  * Every Step 1 and Step 2 check passes.
  * The pass's window meets C1–C8. In particular, the persistent sessions that
    authenticated as `controller@ca-v1` stay established after `ca-v1` (RSA
    pass) and `ca-v2` (ECDSA pass) are removed: trust-bundle and certificate
    rotations must apply only to new TLS handshakes.

### TE-1.25.3 - Concurrent Authorization Policy Rotation (gNSI.authz)

* **Step 1 - Push Updated gRPC Authorization Policy Under Churn**:
  * While `gRIBI`, `P4RT`, `gNMI.Subscribe`, and OTG traffic remain active,
    open a measurement window. `infra@ca-v3` issues every `gnsi.authz.v1` RPC
    over a new connection:
    1. With `policy-v1` active, `Authz.Probe` returns `ACTION_PERMIT` (version
       `authz-v1`) for `spiffe://example.org/ns/sec/sa/auditor` on
       `/gribi.gRIBI/Get` and `ACTION_DENY` (version `authz-v1`) for
       `spiffe://example.org/ns/ops/sa/viewer` on `/gnmi.gNMI/Get`.
    2. Open `gnsi.authz.v1.Authz.Rotate` (`force_overwrite = false`) and send
       an `UploadRequest` with `policy-v2` (`version = "authz-v2"`,
       `created_on = 1700000100`), then wait for `UploadResponse`. `policy-v2`:
       * Retains `allow-test-infra`, `allow-sdn-controller`, and
         `allow-sec-auditor`, so the new deny rule must override an existing
         allow for `/gribi.gRIBI/Get`.
       * Adds the `deny_rules` entry `deny-auditor-gribi-get` blocking
         `spiffe://example.org/ns/sec/sa/auditor` from `/gribi.gRIBI/Get`.
       * Adds the `allow_rules` entry `allow-ops-viewer` permitting
         `spiffe://example.org/ns/ops/sa/viewer` on `/gnmi.gNMI/Get`.
    3. Before sending `FinalizeRequest`, `Authz.Probe` returns, each with
       version `authz-v2`: `ACTION_DENY` for the auditor on `/gribi.gRIBI/Get`,
       `ACTION_PERMIT` for the auditor on `/gnmi.gNMI/Get`, `ACTION_PERMIT` for
       the viewer on `/gnmi.gNMI/Get`, and `ACTION_PERMIT` for
       `spiffe://example.org/ns/sdn/sa/controller` on `/gribi.gRIBI/Modify`
       and on `/p4.v1.P4Runtime/Write`.
    4. Send `FinalizeRequest` and close the send direction; the `Authz.Rotate`
       RPC ends with status `OK`. `gnsi.authz.v1.Authz.Get` returns
       `version = "authz-v2"`, `created_on = 1700000100` and the `policy-v2`
       rules, and repeating the Step 3 probes returns the same actions with
       version `authz-v2`.
    5. Via `gNMI.Get`, identify the counter-bearing servers — every bound
       `/system/grpc-servers/grpc-server` that exposes the
       `authz-policy-counters` container (which always includes every bound
       server hosting `GNSI`, and on multi-server platforms also includes any
       separate `GRIBI` or `GNMI` server that exposes `authz-policy-counters`) —
       and snapshot `access-accepts`, `access-rejects`, `last-access-accept` and
       `last-access-reject` of `rpc[name=/gribi.gRIBI/Get]`,
       `rpc[name=/gnsi.authz.v1.Authz/Get]` and `rpc[name=/gnmi.gNMI/Get]` under
       `/system/grpc-servers/grpc-server/authz-policy-counters/rpcs` across
       those servers. Counters are summed and timestamps take their latest value
       across all counter-bearing bound servers (missing `rpc` entries prior to
       the first hit default to `0`).
    6. Issue live RPCs over newly opened mTLS connections:
       * `auditor@ca-v3` calls `gRIBI.Get` (`DEFAULT`, AFT type `ALL`) `5`
         times -> each returns gRPC `PERMISSION_DENIED`.
       * `viewer@ca-v3` calls `gnsi.authz.v1.Authz.Get` `5` times -> each
         returns gRPC `PERMISSION_DENIED` (because `allow-ops-viewer` permits
         only `/gnmi.gNMI/Get`, exercising a denied RPC directly on the
         `GNSI`-hosting server even when `GRIBI` runs on a separate
         `grpc-server`).
       * `viewer@ca-v3` calls `gNMI.Get` on `/system/state/current-datetime`
         `5` times -> each returns gRPC `OK`.
       * `auditor@ca-v3` calls `gNMI.Get` on `/system/state/current-datetime`
         once -> gRPC `OK`, because `deny-auditor-gribi-get` covers only
         `/gribi.gRIBI/Get`.
    7. Close the window at least `12 s` (`sample_interval + 2 s`) after the
       last Step 1.6 call so at least one post-call `10 s` `SAMPLE` interval
       completes before the window closes.
* **Step 2 - Pass/Fail Criteria**:
  * Via `gNMI.Get`,
    `/system/aaa/authorization/state/grpc-authz-policy-version` equals
    `"authz-v2"` and
    `/system/aaa/authorization/state/grpc-authz-policy-created-on` equals
    `1700000100000000000` (`created_on = 1700000100` s in nanoseconds).
  * Both via `gNMI.Get` after Step 1.6 and in the first `SAMPLE` update received
    at least `12 s` (`sample_interval + 2 s`) after the last Step 1.6 call,
    aggregated across all counter-bearing bound servers and compared with the
    Step 1.5 snapshot:
    * for `rpc[name=/gnsi.authz.v1.Authz/Get]`, `access-rejects` increased by at
      least `5` and `last-access-reject` advanced;
    * for `rpc[name=/gribi.gRIBI/Get]`, whenever the bound server hosting
      `GRIBI` exposes `authz-policy-counters` (including all single-server
      deployments), `access-rejects` increased by at least `5` and
      `last-access-reject` advanced;
    * for `rpc[name=/gnmi.gNMI/Get]`, `access-accepts` increased by at least `6`
      and `last-access-accept` advanced.
  * The window meets C1–C8; `policy-v2` keeps `allow-sdn-controller`, so the
    controller sessions see no errors.

### TE-1.25.4 - Unfinalized gNSI Rotation Rollback Under Churn

* **Step 1 - Abort Certz and Authz Rotations Without Finalizing**:
  * With `gRIBI`, `P4RT`, `gNMI.Subscribe`, and OTG traffic still running, open
    a measurement window. `infra@ca-v3` issues the gNSI RPCs:
    1. Open `gnsi.certz.v1.Certz.Rotate` on `te125_profile`
       (`force_overwrite = false`) and upload the `server-v4-temp` certificate
       chain and private key (`version = "cert-v4-temp"`,
       `created_on = 1700001500`, no trust bundle), then wait for
       `UploadResponse`. Confirm that a new `infra@ca-v3` connection to the
       verification server is presented `server-v4-temp` (serial number and SAN
       match) and that `gNMI.Get` returns `OK`, then cancel the stream without
       sending `FinalizeRequest`.
    2. Within `30 s` of the cancellation, a new `infra@ca-v3` connection to the
       verification server is presented `server-v3-ecdsa`, and every bound
       server that hosts gNSI reports
       `state/certificate-version = "cert-v3-ecdsa"`,
       `state/certificate-created-on = 1700001300000000000` and
       `state/ca-trust-bundle-version = "bundle-v3"`.
    3. Open `gnsi.authz.v1.Authz.Rotate` (`force_overwrite = false`) and upload
       `policy-v3-temp` (`version = "authz-v3-temp"`,
       `created_on = 1700000200`), which is `policy-v2` without the
       `allow-ops-viewer` rule. After `UploadResponse`, `Authz.Probe` returns
       `ACTION_DENY` (version `authz-v3-temp`) for
       `spiffe://example.org/ns/ops/sa/viewer` on `/gnmi.gNMI/Get` and
       `ACTION_PERMIT` (version `authz-v3-temp`) for
       `spiffe://example.org/ns/sdn/sa/controller` on `/gribi.gRIBI/Modify`.
       Then cancel the stream without sending `FinalizeRequest`.
    4. Within `30 s` of the cancellation, `Authz.Probe` returns `ACTION_PERMIT`
       (version `authz-v2`) for the viewer on `/gnmi.gNMI/Get`.
    5. Close the window at least `10 s` after the last step.
* **Step 2 - Pass/Fail Criteria**:
  * Every Step 1 check passes: new TLS handshakes present the rolled-back
    `server-v3-ecdsa` and
    `/system/grpc-servers/grpc-server/state/certificate-version` reports
    `"cert-v3-ecdsa"`.
  * `gnsi.authz.v1.Authz.Get` returns `version = "authz-v2"` and
    `created_on = 1700000100`, and
    `/system/aaa/authorization/state/grpc-authz-policy-version` reports
    `"authz-v2"` with `grpc-authz-policy-created-on = 1700000100000000000`.
  * The window meets C1–C8: `0` errors on the concurrent `gRIBI.Modify`,
    `P4Runtime.Write` and `gNMI.Subscribe` streams and `0` lost transit,
    `PacketIn` and `PacketOut` frames.
* **Step 3 - Stop Churn and Verify Final State**:
  * Stop the P4RT churn after the final `P4Runtime.Read` of the current cycle
    and the gRIBI churn after the `DELETE` phase of the current cycle, then
    `DELETE` `NextHopGroup 100` and `101` and, once they are acknowledged,
    `NextHop 100` and `101` (timeout `60 s`). Each operation returns
    `FIB_PROGRAMMED`, except that on platforms that do not send
    `FIB_PROGRAMMED` for NH/NHG entries that no prefix references, it returns
    `RIB_PROGRAMMED`.
  * `gRIBI.Get` (`DEFAULT`, AFT type `ALL`) returns exactly the 6 protected
    entries programmed in TE-1.25.1 Step 1, each with `fib_status = PROGRAMMED`.
  * Via `gNMI.Get` on `/network-instances/network-instance[name=DEFAULT]/afts/`
    (and confirmed on the `ON_CHANGE` stream for the protected entries), AFT
    telemetry still resolves `198.51.100.0/24` (next-hop-group
    `programmed-id = 10`) to next hop `192.0.2.6` and `2001:db8:100::/64`
    (`programmed-id = 11`) to next hop `2001:db8:2::2`, contains no prefix
    within `100.64.0.0/13` or `2001:db8:1000::/52`, and contains no
    next-hop-group with `programmed-id` `100` or `101`.
  * `P4Runtime.Read` on `acl_wbb_ingress_table` returns only the protected LLDP
    entry among non-const (`is_const = false`) entries.
  * C8 passes.

### Cleanup

* Stop OTG traffic, captures and protocol sessions.
* Read `acl_wbb_ingress_table` via `P4Runtime.Read` and delete all remaining
  non-const (`is_const = false`) entries via `P4Runtime.Write` (`DELETE`)
  (covering both the protected LLDP entry and any churn entries remaining if an
  earlier subtest failed mid-cycle). If `P_pre` was set before the test, restore
  `P_pre` via `P4Runtime.SetForwardingPipelineConfig` (`VERIFY_AND_COMMIT`),
  re-insert the recorded pre-test dynamic (`is_const = false`) entries `E_pre`
  via `P4Runtime.Write` (`INSERT`), and verify them via `P4Runtime.Read`
  (`is_const = false`); then close the P4RT session.
* Execute `gRIBI.Flush` on `DEFAULT` with `election.id = {high: 0, low: 1}` (or
  `override = {}`), close the gRIBI session and close the persistent
  `gNMI.Subscribe` streams.
* Over a connection that uses the current credential (credential selection
  rule), restore each bound server's captured
  `/system/grpc-servers/grpc-server/config/certificate-id` via `gNMI.Set`
  (delete the leaf if it was unset) and verify that `state/certificate-id` and,
  on bound servers that host gNSI, `state/ssl-profile-id` return to their
  captured values.
* Over a connection that uses the testbed's pre-test credentials, delete the
  test profile with `gnsi.certz.v1.Certz.DeleteProfile`
  (`ssl_profile_id = "te125_profile"`) and verify that `Certz.GetProfileList`
  no longer lists it.
* Over the same connection, restore the authorization policy captured during
  setup via `gnsi.authz.v1.Authz.Rotate` (captured `policy`, `version` and
  `created_on`, `force_overwrite = true`) and `FinalizeRequest`, and verify
  that `Authz.Get` returns the captured `version`. If no policy was set before
  the test, rotate and finalize a policy whose only rule is an allow rule with
  empty `source` and `request` (it matches every request, like the default
  policy), because `gnsi.authz.v1` has no delete RPC.
* Restore `/lldp/config/enabled` to its recorded value and the remaining DUT
  OpenConfig state to its pre-test baseline.

## Canonical OC

The `grpc-server` entry shows the precondition server settings (`enable`,
`services` and `transport-security`) together with the `certificate-id` binding
that the test applies. On platforms with several gRPC servers, the same
`certificate-id` is applied to every server that hosts gNMI, gNSI, gRIBI or
P4RT.

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
          "description": "PacketOut Isolation Monitor Port 4",
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
  },
  "system": {
    "grpc-servers": {
      "grpc-server": [
        {
          "config": {
            "certificate-id": "te125_profile",
            "enable": true,
            "name": "DEFAULT",
            "services": [
              "openconfig-system-grpc:GNMI",
              "openconfig-gnsi:GNSI",
              "openconfig-gribi:GRIBI",
              "openconfig-p4rt:P4RT"
            ],
            "transport-security": true
          },
          "name": "DEFAULT"
        }
      ]
    }
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /components/component/integrated-circuit/config/node-id:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/id:
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
  /system/aaa/authorization/state/grpc-authz-policy-created-on:
  /system/aaa/authorization/state/grpc-authz-policy-version:
  /system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/access-accepts:
  /system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/access-rejects:
  /system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/last-access-accept:
  /system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/last-access-reject:
  /system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/name:
  /system/grpc-servers/grpc-server/config/certificate-id:
  /system/grpc-servers/grpc-server/config/enable:
  /system/grpc-servers/grpc-server/config/name:
  /system/grpc-servers/grpc-server/config/services:
  /system/grpc-servers/grpc-server/config/transport-security:
  /system/grpc-servers/grpc-server/state/ca-trust-bundle-created-on:
  /system/grpc-servers/grpc-server/state/ca-trust-bundle-version:
  /system/grpc-servers/grpc-server/state/certificate-created-on:
  /system/grpc-servers/grpc-server/state/certificate-expires-on:
  /system/grpc-servers/grpc-server/state/certificate-id:
  /system/grpc-servers/grpc-server/state/certificate-version:
  /system/grpc-servers/grpc-server/state/counters/connection-rejects:
  /system/grpc-servers/grpc-server/state/ssl-profile-id:
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
  gnsi:
    authz.v1.Authz.Get:
    authz.v1.Authz.Probe:
    authz.v1.Authz.Rotate:
    certz.v1.Certz.AddProfile:
    certz.v1.Certz.DeleteProfile:
    certz.v1.Certz.GetProfileList:
    certz.v1.Certz.Rotate:
  gribi:
    gRIBI.Flush:
    gRIBI.Get:
    gRIBI.Modify:
  # P4Runtime (p4.v1.P4Runtime) RPCs used by this test are not tracked by the
  # OpenConfig RPC coverage tooling: GetForwardingPipelineConfig,
  # SetForwardingPipelineConfig, StreamChannel (arbitration, PacketIn,
  # PacketOut), Write and Read.
```

## Required DUT platform

* FFF
