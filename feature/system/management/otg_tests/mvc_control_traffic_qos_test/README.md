# MGT-2: QoS of MVC Control and Management Traffic

## Summary

*   Validate that DUT-originated IPv4 and IPv6 control and management plane
    traffic (NTP, TACACS+, SSH, DNS client, sFlow, Syslog, P4RT, gRIBI, gNMI,
    and gNOI) egresses in forwarding class `AF4` with `DSCP 32` (and never
    `BE1` / `DSCP 0`) both when routed over Linecard (LC) interfaces via eBGP
    in the management VRF (`mgmt`) and when failing over to the Out-of-Band
    (OOB) management fallback interface via a backup static default route.

## Testbed type

*   [topologies/atedut_4.testbed](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Topology

```text
+-----------------------------------+       +----------------------------------+
|            DUT (AS 64512)         |       |           ATE (AS 64511)         |
|                                   |       |                                  |
|  [VRF: mgmt]                      |       |  Mgmt Collectors / Servers:      |
|  <DUT_LO1> (lo1)                  |       |  198.51.100.1 / 2001:db8:100::1  |
|  192.0.2.101/32, 2001:db8::101/128|       |  (BE1 Congestion Source/Sink)    |
|                                   |       |                                  |
|  <DUT_PORT1> (LC1, QoS AF4/BE1)   | eBGP  |  <ATE_PORT1>                     |
|  192.0.2.1/30, 2001:db8::1/126 ---+-------+-- 192.0.2.2/30, 2001:db8::2/126  |
|                                   |       |                                  |
|  <DUT_PORT2> (LC2, QoS AF4/BE1)   | eBGP  |  <ATE_PORT2>                     |
|  192.0.2.5/30, 2001:db8::5/126 ---+-------+-- 192.0.2.6/30, 2001:db8::6/126  |
|                                   |       |                                  |
|  <DUT_PORT3> (OOB, QoS AF4/BE1)   |Static |  <ATE_PORT3>                     |
|  192.0.2.9/30, 2001:db8::9/126 ---+-(220)-+-- 192.0.2.10/30, 2001:db8::a/126 |
|  .................................|       |  ................................|
|  [VRF: DEFAULT]                   |       |  Data-Plane Transit / Isolation: |
|  <DUT_PORT4> (sFlow Ingress)      |Transit|  <ATE_PORT4>                     |
|  192.0.2.13/30, 2001:db8::d/126 --+-------+-- 192.0.2.14/30, 2001:db8::e/126 |
+-----------------------------------+       +----------------------------------+
```

### Applying configuration

For each section of configuration below, prepare a `gnmi.SetBatch` with all the
configuration items appended to one `SetBatch`. Then apply the configuration to
the DUT in one `gnmi.Set` using the `replace` option.

### Initial Setup:

*   Connect `<DUT_PORT1>`, `<DUT_PORT2>`, `<DUT_PORT3>`, and `<DUT_PORT4>` to
    `<ATE_PORT1>`, `<ATE_PORT2>`, `<ATE_PORT3>`, and `<ATE_PORT4>`:
    *   `<DUT_PORT1>` (`port1`, `192.0.2.1/30`, `2001:db8::1/126`) <->
        `<ATE_PORT1>` (`port1`, `192.0.2.2/30`, `2001:db8::2/126`): In-band
        Linecard (LC) management link 1 (`mgmt` VRF).
    *   `<DUT_PORT2>` (`port2`, `192.0.2.5/30`, `2001:db8::5/126`) <->
        `<ATE_PORT2>` (`port2`, `192.0.2.6/30`, `2001:db8::6/126`): In-band
        Linecard (LC) management link 2 (`mgmt` VRF).
    *   `<DUT_PORT3>` (`port3`, `192.0.2.9/30`, `2001:db8::9/126`) <->
        `<ATE_PORT3>` (`port3`, `192.0.2.10/30`, `2001:db8::a/126`):
        Out-of-Band (OOB) management fallback link (`mgmt` VRF).
    *   `<DUT_PORT4>` (`port4`, `192.0.2.13/30`, `2001:db8::d/126`) <->
        `<ATE_PORT4>` (`port4`, `192.0.2.14/30`, `2001:db8::e/126`): Data-plane
        transit link (`DEFAULT` VRF).
*   Create VRF `"mgmt"` on DUT:
    *   `/network-instances/network-instance[name=mgmt]/config/name = mgmt`
    *   `/network-instances/network-instance[name=mgmt]/config/type = L3VRF`
*   Create simulated management collector/server networks (`198.51.100.1/32` and
    `2001:db8:100::1/128`) attached behind `<ATE_PORT1>`, `<ATE_PORT2>`, and
    `<ATE_PORT3>`.
*   Create a loopback interface `<DUT_LO1>` (`lo1`) on DUT and assign IPv4
    (`192.0.2.101/32`) and IPv6 (`2001:db8::101/128`) addresses:
    *   `/interfaces/interface[name=lo1]/config/name = lo1`
    *   `/interfaces/interface[name=lo1]/config/type = softwareLoopback`
    *   `/interfaces/interface[name=lo1]/subinterfaces/subinterface[index=0]/ipv4/addresses/address/config/ip = 192.0.2.101`
    *   `/interfaces/interface[name=lo1]/subinterfaces/subinterface[index=0]/ipv4/addresses/address/config/prefix-length = 32`
    *   `/interfaces/interface[name=lo1]/subinterfaces/subinterface[index=0]/ipv6/addresses/address/config/ip = 2001:db8::101`
    *   `/interfaces/interface[name=lo1]/subinterfaces/subinterface[index=0]/ipv6/addresses/address/config/prefix-length = 128`
*   Configure `<DUT_LO1>` to participate in VRF `"mgmt"`:
    *   `/network-instances/network-instance[name=mgmt]/interfaces/interface[id=lo1]/config/interface = lo1`

##### Configure linecard ports to ATE using BGP

*   Configure IPv4 and IPv6 addresses on `<DUT_PORT1>` and `<DUT_PORT2>` and
    bind them to VRF `"mgmt"`:
    *   `/interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip`
    *   `/interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length`
    *   `/interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip`
    *   `/interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length`
    *   `/network-instances/network-instance[name=mgmt]/interfaces/interface/config/interface`
*   Configure dual-stack eBGP between `<DUT_PORT1>` <--> `<ATE_PORT1>` and
    `<DUT_PORT2>` <--> `<ATE_PORT2>` in VRF `"mgmt"` (`<DUT_AS> = 64512`,
    `<ATE_AS> = 64511`):
    *   `/network-instances/network-instance[name=mgmt]/protocols/protocol[identifier=BGP, name=BGP]/bgp/global/config/as = 64512`
    *   `/network-instances/network-instance[name=mgmt]/protocols/protocol[identifier=BGP, name=BGP]/bgp/global/config/router-id = 192.0.2.101`
    *   `/network-instances/network-instance[name=mgmt]/protocols/protocol[identifier=BGP, name=BGP]/bgp/neighbors/neighbor/config/neighbor-address`
    *   `/network-instances/network-instance[name=mgmt]/protocols/protocol[identifier=BGP, name=BGP]/bgp/neighbors/neighbor/config/peer-as = 64511`
*   Advertise default routes (`0.0.0.0/0`, `::/0`) and management server
    prefixes (`198.51.100.0/24`, `2001:db8:100::/64`) from ATE to DUT through
    both eBGP sessions.
*   Redistribute `<DUT_LO1>` (`192.0.2.101/32` and `2001:db8::101/128`) from DUT
    to ATE through both eBGP sessions:
    ##### Configure redistribution
    *   Configure `table-connections` from `DIRECTLY_CONNECTED` to `BGP` for
        `IPV4` and `IPV6` with `default-import-policy = REJECT_ROUTE` and
        `disable-metric-propagation = false`:
        *   `/network-instances/network-instance/table-connections/table-connection/config/address-family`
        *   `/network-instances/network-instance/table-connections/table-connection/config/src-protocol`
        *   `/network-instances/network-instance/table-connections/table-connection/config/dst-protocol`
        *   `/network-instances/network-instance/table-connections/table-connection/config/default-import-policy`
        *   `/network-instances/network-instance/table-connections/table-connection/config/disable-metric-propagation`
        *   `/network-instances/network-instance/table-connections/table-connection/config/import-policy`
    ##### Configure a prefix-set for route-filtering/matching
    *   Configure prefix-sets `lo1_v4_prefix_set` (`IPV4`, `192.0.2.101/32`,
        `exact`) and `lo1_v6_prefix_set` (`IPV6`, `2001:db8::101/128`, `exact`):
        *   `/routing-policy/defined-sets/prefix-sets/prefix-set/config/name`
        *   `/routing-policy/defined-sets/prefix-sets/prefix-set/config/mode`
        *   `/routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix`
        *   `/routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range`
    ##### Attach the prefix-set to route-policy
    *   Configure policy-definitions `redist_lo1_v4` (`allow_lo1_v4`) and
        `redist_lo1_v6` (`allow_lo1_v6`) matching the prefix-sets with
        `match-set-options = ANY` and `policy-result = ACCEPT_ROUTE`:
        *   `/routing-policy/policy-definitions/policy-definition/config/name`
        *   `/routing-policy/policy-definitions/policy-definition/statements/statement/config/name`
        *   `/routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options`
        *   `/routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set`
        *   `/routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result`

##### Configure OOB fallback port to ATE

*   Configure IPv4 and IPv6 addresses on `<DUT_PORT3>` and `<ATE_PORT3>` and
    bind `<DUT_PORT3>` to VRF `"mgmt"`:
    *   `/interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip`
    *   `/interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length`
    *   `/interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip`
    *   `/interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length`
    *   `/network-instances/network-instance[name=mgmt]/interfaces/interface/config/interface`
*   Configure backup IPv4 (`0.0.0.0/0` -> `192.0.2.10`) and IPv6 (`::/0` ->
    `2001:db8::a`) static default routes on DUT in VRF `"mgmt"` with
    administrative `preference = 220`:
    *   `/network-instances/network-instance[name=mgmt]/protocols/protocol[identifier=STATIC, name=STATIC]/static-routes/static/config/prefix`
    *   `/network-instances/network-instance[name=mgmt]/protocols/protocol[identifier=STATIC, name=STATIC]/static-routes/static/next-hops/next-hop/config/index = 1`
    *   `/network-instances/network-instance[name=mgmt]/protocols/protocol[identifier=STATIC, name=STATIC]/static-routes/static/next-hops/next-hop[index=1]/config/next-hop`
    *   `/network-instances/network-instance[name=mgmt]/protocols/protocol[identifier=STATIC, name=STATIC]/static-routes/static/next-hops/next-hop/config/preference = 220`

##### Configure QoS classifiers, queues, and scheduler-policy

*   Configure queues `AF4` and `BE1`, forwarding-groups `AF4 -> AF4` and
    `BE1 -> BE1`, IPv4/IPv6 classifiers `dscp_af4_v4` and `dscp_af4_v6`
    (`dscp-set = [32] -> AF4`, `dscp-set = [0] -> BE1` so `MGT-2.3` transit
    traffic is classified into `BE1`), bind `dscp_af4_v4` and `dscp_af4_v6`
    under `/qos/interfaces/interface/input/classifiers` on `<DUT_PORT1>`,
    `<DUT_PORT2>`, and `<DUT_PORT3>`, and attach egress strict priority
    scheduler-policy `mgmt_qos_sched` (`AF4` `STRICT` priority over `BE1`) under
    `/qos/interfaces/interface/output` on `<DUT_PORT1>`, `<DUT_PORT2>`, and
    `<DUT_PORT3>`:
    *   `/qos/queues/queue/config/name`
    *   `/qos/forwarding-groups/forwarding-group/config/name`
    *   `/qos/forwarding-groups/forwarding-group/config/output-queue`
    *   `/qos/classifiers/classifier/config/name`
    *   `/qos/classifiers/classifier/config/type`
    *   `/qos/classifiers/classifier/terms/term/config/id`
    *   `/qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set`
    *   `/qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set`
    *   `/qos/classifiers/classifier/terms/term/actions/config/target-group`
    *   `/qos/scheduler-policies/scheduler-policy/config/name`
    *   `/qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/sequence`
    *   `/qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/priority`
    *   `/qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/type`
    *   `/qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/id`
    *   `/qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/input-type`
    *   `/qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue`
    *   `/qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/weight`
    *   `/qos/interfaces/interface/config/interface-id`
    *   `/qos/interfaces/interface/input/classifiers/classifier/config/name`
    *   `/qos/interfaces/interface/input/classifiers/classifier/config/type`
    *   `/qos/interfaces/interface/output/queues/queue/config/name`
    *   `/qos/interfaces/interface/output/scheduler-policy/config/name`

##### Configure 10-Protocol Trigger Suite and Standard Egress AF4 Verification

All subtests (`MGT-2.1`–`MGT-2.7`) reference the following 10 control and
management protocols (sourced from `<DUT_LO1>` `192.0.2.101` / `2001:db8::101`
in VRF `mgmt` toward `198.51.100.1` / `2001:db8:100::1`) and verification
criteria:

*   **10-Protocol Trigger Suite (IPv4 & IPv6)**:
    *   **NTP** (UDP `123`): Enable `/system/ntp/config/enabled = true` with
        servers `198.51.100.1` (`source-address = 192.0.2.101`) and
        `2001:db8:100::1` (`source-address = 2001:db8::101`) in
        `network-instance = mgmt`, and trigger an NTP poll/re-sync via
        `gNMI.Set` so the DUT transmits outbound UDP `123` client requests.
    *   **TACACS+** (TCP `49`): Enable AAA `TACACS_ALL` with server-group
        `TACACS-MGMT` (`198.51.100.1:49` with `source-address = 192.0.2.101`
        and `2001:db8:100::1:49` with `source-address = 2001:db8::101` in VRF
        `mgmt`). Because a stateless OTG TCP `SYN` to port `22` does not reach
        the SSH user-authentication phase, trigger TACACS+ by invoking an
        authenticated management RPC/CLI action or AAA authentication/accounting
        event on the DUT so the AAA daemon initiates outbound TCP `49` `SYN`
        packets from `<DUT_LO1>` in VRF `mgmt`.
    *   **DNS Client** (UDP/TCP `53`): Configure DNS servers `198.51.100.1` and
        `2001:db8:100::1` under `/system/dns/servers/server/config`. Because
        OpenConfig `/system/dns/servers/server/config` only defines `address`
        and `port` (without `network-instance` or `source-address` leaves),
        ensure `198.51.100.0/24` and `2001:db8:100::/64` are reachable in VRF
        `mgmt` (binding the DNS resolver to VRF `mgmt` and source `<DUT_LO1>`)
        and invoke `gNOI.System.Ping` to unresolved hostname
        `mgmt-target.example.com` in VRF `mgmt` sourced from `<DUT_LO1>` to
        trigger outbound DNS queries.
    *   **sFlow** (UDP `6343`): Enable `/sampling/sflow/config/enabled = true`
        (`dscp = 32`, rate `1`, size `256`) on `<DUT_PORT4>` with collectors
        `198.51.100.1` (`source-address = 192.0.2.101`) and `2001:db8:100::1`
        (`source-address = 2001:db8::101`) in `network-instance = mgmt`, and
        send transit traffic from `<ATE_PORT4>` to `<DUT_PORT4>` to trigger
        outbound sFlow datagrams.
    *   **Syslog** (UDP `514`): Configure remote servers `198.51.100.1`
        (`source-address = 192.0.2.101`) and `2001:db8:100::1`
        (`source-address = 2001:db8::101`) (`ALL` / `INFORMATIONAL`) in
        `network-instance = mgmt` and trigger a configuration update via
        `gNMI.Set` to emit outbound UDP `514` syslog messages.
    *   **SSH (`22`), gNMI/gNOI (`9339`), gRIBI (`9340`), and P4RT (`9559`) —
        In-Band OTG TCP `SYN` Probes & Listeners**:
        *   Because `dut.RawAPIs()` executes over the lab's out-of-band
            management network (not in-band VRF `mgmt`) and OTG generates
            stateless packets, enable `/system/ssh-server/config/enable = true`
            and configure `/system/grpc-servers/grpc-server` listeners on
            `<DUT_LO1>` (`192.0.2.101`, `2001:db8::101`) in
            `network-instance = mgmt` for ports `9339` (`GNMI` and `GNOI`, which
            share TCP port `9339`), `9340` (`GRIBI`), and `9559` (`P4RT`) so the
            DUT does not reply with `DSCP 0` TCP `RST` packets.
        *   Send in-band OTG TCP `SYN` probes from `198.51.100.1` and
            `2001:db8:100::1` to `<DUT_LO1>` (`192.0.2.101` / `2001:db8::101`)
            on destination ports `22` (`SSH`), `9339` (`gNMI`/`gNOI`), `9340`
            (`gRIBI`), and `9559` (`P4RT`) (alongside `gNMI.Get`/`Subscribe`,
            `gNOI.System.Time`/`Ping`, `gRIBI.Get`/`Modify`, and
            `P4Runtime.StreamChannel` RPC triggers) and verify the DUT's egress
            TCP `SYN-ACK` responses carry **DSCP = `32`** (`AF4`).
*   **Standard Egress AF4 Verification**:
    *   Validate via packet capture on the active ATE port(s) that **`> 0`
        packets (`packet-count >= 1`) are captured per protocol**, that **100%**
        of captured DUT-originated IPv4 and IPv6 packets across all 10 protocols
        carry **DSCP = `32`** (`AF4`, byte `0x80`), and that **0** packets carry
        `DSCP 0` (`BE1`) or any other DSCP value.
    *   Validate via `/qos/interfaces/interface/output/queues/queue/state` on
        the active DUT egress port(s) that `AF4` `transmit-pkts` and
        `transmit-octets` increment (`delta > 0`) while `AF4` `dropped-pkts`
        remains `0`.

### MGT-2.1
#### Testing QoS marking over Linecard interfaces
---

*   Verify VRF `mgmt` IPv4 and IPv6 eBGP sessions on `<DUT_PORT1>` and
    `<DUT_PORT2>` are `ESTABLISHED` and record baseline `AF4` queue counters.
*   Start packet capture on `<ATE_PORT1>` and `<ATE_PORT2>`, run the
    **10-Protocol Trigger Suite**, then stop packet capture.
*   Validate the **Standard Egress AF4 Verification** on
    `<DUT_PORT1>`/`<ATE_PORT1>` and `<DUT_PORT2>`/`<ATE_PORT2>`.

### MGT-2.2
#### Testing QoS marking on failover to OOB interface
---

*   Record baseline `AF4` queue counters on `<DUT_PORT3>`, disable `<DUT_PORT1>`
    and `<DUT_PORT2>`, and verify VRF `mgmt` eBGP leaves `ESTABLISHED` and
    switches to the backup static default routes (`preference = 220`) over
    `<DUT_PORT3>`.
*   Start packet capture on `<ATE_PORT3>`, run the **10-Protocol Trigger
    Suite**, stop packet capture, and validate the **Standard Egress AF4
    Verification** on `<DUT_PORT3>`/`<ATE_PORT3>`.
*   Bring up `<DUT_PORT1>` and `<DUT_PORT2>`, verify eBGP returns to
    `ESTABLISHED`, and validate traffic fails back to
    `<DUT_PORT1>`/`<DUT_PORT2>` with **DSCP = `32`** (`AF4`).

### MGT-2.3
#### Testing control and management traffic survival under egress port BE1 congestion
---

*   Record baseline `AF4` and `BE1` queue counters on `<DUT_PORT1>` and
    `<DUT_PORT2>`.
*   Start continuous oversubscribed background `BE1` (`DSCP 0`) IPv4 and IPv6
    congestion flows in VRF `mgmt` to induce continuous `BE1` tail-drops on both
    `<DUT_PORT1>` and `<DUT_PORT2>`:
    *   **Traffic Rate & Oversubscription Ratio**: Inject `60%` line-rate `BE1`
        (`DSCP 0`) traffic from `<ATE_PORT2>` plus `48%`–`60%` line-rate from
        `<ATE_PORT3>` toward `<ATE_PORT1>` (`108%`–`120%` total egress load on
        `<DUT_PORT1>`), and symmetrically `60%` line-rate from `<ATE_PORT1>`
        plus `48%`–`60%` line-rate from `<ATE_PORT3>` toward `<ATE_PORT2>`
        (`108%`–`120%` total egress load on `<DUT_PORT2>`, keeping
        `<ATE_PORT3>` aggregate ingress TX `<= 100%` line rate).
    *   **Destination Addressing (Single-Port Pinning vs. ECMP)**: Address the
        `<DUT_PORT1>` congestion flows directly to `<ATE_PORT1>`'s connected
        interface IPs (`192.0.2.2` / `2001:db8::2`) and the `<DUT_PORT2>`
        congestion flows directly to `<ATE_PORT2>`'s connected interface IPs
        (`192.0.2.6` / `2001:db8::6`) so each flow is deterministically routed
        to its intended single egress port rather than ECMP-hashed across both.
*   **Concurrent Execution**: While the background `BE1` (`DSCP 0`) congestion
    flows remain continuously transmitting across `<DUT_PORT1>` and
    `<DUT_PORT2>`, capture on `<ATE_PORT1>` and `<ATE_PORT2>` and execute the
    **10-Protocol Trigger Suite**; stop the background `BE1` congestion flows
    and packet captures only after all protocol triggers complete.
*   Validate `BE1` `dropped-pkts > 0` on both `<DUT_PORT1>` and `<DUT_PORT2>`,
    and validate the **Standard Egress AF4 Verification** passes with **0%
    packet loss** (`AF4` `dropped-pkts == 0`).

### MGT-2.4
#### Testing client-initiated BE1 (DSCP 0) request vs. DUT egress AF4 (DSCP 32) response
---

*   Record baseline `AF4` and `BE1` counters on `<DUT_PORT1>`–`<DUT_PORT3>`.
*   Start packet capture on `<ATE_PORT1>` and `<ATE_PORT2>` while the ATE sends
    inbound TCP `SYN` packets marked with **`DSCP 0` (`BE1`)** and **`DSCP 8`
    (`CS1`)** from `198.51.100.1` and `2001:db8:100::1` to `<DUT_LO1>`
    (`192.0.2.101` / `2001:db8::101`) on the listening management TCP ports
    (`22` for `SSH`, `9339` for `gNMI`/`gNOI`, `9340` for `gRIBI`, and `9559`
    for `P4RT`), then stop capture.
*   Disable `<DUT_PORT1>` and `<DUT_PORT2>`, repeat the inbound `DSCP 0` (`BE1`)
    and `DSCP 8` (`CS1`) TCP `SYN` packets to `<DUT_LO1>` (`192.0.2.101` /
    `2001:db8::101`) on ports `22`, `9339`, `9340`, and `9559` over OOB
    `<ATE_PORT3>`, then stop capture.
*   Validate that all DUT egress TCP `SYN-ACK` (and RPC) response packets on
    `<ATE_PORT1>`–`<ATE_PORT3>` always carry **DSCP = `32`** (`AF4`) (never
    reflecting the client's `DSCP 0` or `DSCP 8`), then bring up `<DUT_PORT1>`
    and `<DUT_PORT2>` and verify eBGP returns to `ESTABLISHED`.

### MGT-2.5
#### Testing partial Linecard failure (ECMP rehash) and rapid flap stability
---

*   Verify baseline traffic across `<DUT_PORT1>` and `<DUT_PORT2>`, then disable
    only `<DUT_PORT1>` while keeping `<DUT_PORT2>` `ESTABLISHED`.
*   Start fresh packet captures on `<ATE_PORT2>` and `<ATE_PORT3>`, run the
    **10-Protocol Trigger Suite**, stop captures, and validate that **0**
    management packets fail over to OOB `<ATE_PORT3>` while all rehashed flows
    egress `<DUT_PORT2>` meeting the **Standard Egress AF4 Verification**
    (**DSCP = `32`** / `AF4`).
*   Flap `<DUT_PORT1>` and `<DUT_PORT2>` 5 times (forcing repeated LC <-> OOB
    transitions) while deleting and re-creating the NTP and Syslog servers via
    `gNMI.Set`.
*   Ensure `<DUT_PORT1>` and `<DUT_PORT2>` are enabled with eBGP `ESTABLISHED`,
    start fresh packet captures on `<ATE_PORT1>`–`<ATE_PORT3>`, run the
    **10-Protocol Trigger Suite**, stop captures, and validate all 10 protocols
    retain **DSCP = `32`** (`AF4`) throughout.

### MGT-2.6
#### Testing negative verification of protocol-level DSCP override and queue isolation
---

*   Set `/sampling/sflow/config/dscp = 0` (`BE1`) via `gNMI.Set`, start packet
    capture on `<ATE_PORT1>` and `<ATE_PORT2>`, run the **10-Protocol Trigger
    Suite**, and stop capture.
*   Validate deterministically that exported `sFlow` (`UDP 6343`) packets egress
    with **DSCP = `0`** (`BE1`) and increment only queue `BE1` (`transmit-pkts`
    delta `> 0`, and **not** queue `AF4`), while the 9 remaining protocols
    continue to egress with **DSCP = `32`** (`AF4`) and increment queue `AF4`
    meeting the **Standard Egress AF4 Verification**.
*   Restore `/sampling/sflow/config/dscp = 32` via `gNMI.Set` and validate
    `/sampling/sflow/state/dscp == 32`.

### MGT-2.7
#### Testing negative verification of total management path failure (VRF isolation)
---

*   Configure temporary default routes (`0.0.0.0/0` via `192.0.2.14`, `::/0` via
    `2001:db8::e`) in the `DEFAULT` VRF over `<DUT_PORT4>`.
*   Disable `<DUT_PORT1>`, `<DUT_PORT2>`, and `<DUT_PORT3>`, start packet
    capture on `<ATE_PORT1>`–`<ATE_PORT4>`, and trigger the **10-Protocol
    Trigger Suite**.
*   Validate negatively on `<ATE_PORT4>` that **0** VRF `mgmt` packets leak into
    the `DEFAULT` VRF.
*   Bring up `<DUT_PORT3>` then `<DUT_PORT1>` and `<DUT_PORT2>`, stop captures,
    validate recovery on `<DUT_PORT3>` and failback to
    `<DUT_PORT1>`/`<DUT_PORT2>` meet the **Standard Egress AF4 Verification**,
    and remove the temporary `DEFAULT` VRF default routes.

### Cleanup

*   **ATE Cleanup**: Stop all active OTG traffic flows, packet captures, and
    protocol sessions on `<ATE_PORT1>`–`<ATE_PORT4>`.
*   **Interface & Subtest State Restoration**: Ensure `<DUT_PORT1>`–`<DUT_PORT4>`
    are enabled (`/interfaces/interface/config/enabled = true`) with
    `/interfaces/interface/state/oper-status = UP`, remove any temporary
    `DEFAULT` VRF static default routes configured on `<DUT_PORT4>` during
    `MGT-2.7`, and ensure `/sampling/sflow/config/dscp` is restored to `32` if
    modified during `MGT-2.6`.
*   **DUT Configuration Teardown**: Remove the 10-protocol management
    configurations (NTP servers, AAA `TACACS-MGMT` server-group and
    authentication method, DNS servers, sFlow collectors and `<DUT_PORT4>`
    sampling interface, remote Syslog servers, and gRPC servers `mgmt-grpc`,
    `mgmt-gribi`, and `mgmt-p4rt`), QoS
    configuration (`mgmt_qos_sched` scheduler-policy, `dscp_af4_v4` and
    `dscp_af4_v6` classifiers, and `AF4`/`BE1` forwarding-groups and queues on
    `<DUT_PORT1>`–`<DUT_PORT3>`), VRF `mgmt` routing configuration (eBGP
    neighbors, backup static default routes, `table-connections`, and
    `routing-policy` prefix-sets and policy-definitions), and VRF `mgmt` /
    interface IP configurations on `<DUT_PORT1>`–`<DUT_PORT4>` and `<DUT_LO1>`
    to restore the DUT to its pre-test baseline state.

## Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "name": "lo1",
        "config": {"enabled": true, "name": "lo1", "type": "iana-if-type:softwareLoopback"},
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "config": {"index": 0},
              "ipv4": {"addresses": {"address": [{"ip": "192.0.2.101", "config": {"ip": "192.0.2.101", "prefix-length": 32}}]}},
              "ipv6": {"addresses": {"address": [{"ip": "2001:db8::101", "config": {"ip": "2001:db8::101", "prefix-length": 128}}]}}
            }
          ]
        }
      },
      {
        "name": "port1",
        "config": {"enabled": true, "name": "port1", "type": "iana-if-type:ethernetCsmacd"},
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "config": {"index": 0},
              "ipv4": {"addresses": {"address": [{"ip": "192.0.2.1", "config": {"ip": "192.0.2.1", "prefix-length": 30}}]}},
              "ipv6": {"addresses": {"address": [{"ip": "2001:db8::1", "config": {"ip": "2001:db8::1", "prefix-length": 126}}]}}
            }
          ]
        }
      },
      {
        "name": "port2",
        "config": {"enabled": true, "name": "port2", "type": "iana-if-type:ethernetCsmacd"},
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "config": {"index": 0},
              "ipv4": {"addresses": {"address": [{"ip": "192.0.2.5", "config": {"ip": "192.0.2.5", "prefix-length": 30}}]}},
              "ipv6": {"addresses": {"address": [{"ip": "2001:db8::5", "config": {"ip": "2001:db8::5", "prefix-length": 126}}]}}
            }
          ]
        }
      },
      {
        "name": "port3",
        "config": {"enabled": true, "name": "port3", "type": "iana-if-type:ethernetCsmacd"},
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "config": {"index": 0},
              "ipv4": {"addresses": {"address": [{"ip": "192.0.2.9", "config": {"ip": "192.0.2.9", "prefix-length": 30}}]}},
              "ipv6": {"addresses": {"address": [{"ip": "2001:db8::9", "config": {"ip": "2001:db8::9", "prefix-length": 126}}]}}
            }
          ]
        }
      },
      {
        "name": "port4",
        "config": {"enabled": true, "name": "port4", "type": "iana-if-type:ethernetCsmacd"},
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "config": {"index": 0},
              "ipv4": {"addresses": {"address": [{"ip": "192.0.2.13", "config": {"ip": "192.0.2.13", "prefix-length": 30}}]}},
              "ipv6": {"addresses": {"address": [{"ip": "2001:db8::d", "config": {"ip": "2001:db8::d", "prefix-length": 126}}]}}
            }
          ]
        }
      }
    ]
  },
  "network-instances": {
    "network-instance": [
      {
        "name": "mgmt",
        "config": {"name": "mgmt", "type": "openconfig-network-instance-types:L3VRF"},
        "interfaces": {
          "interface": [
            {"id": "lo1", "config": {"id": "lo1", "interface": "lo1"}},
            {"id": "port1", "config": {"id": "port1", "interface": "port1"}},
            {"id": "port2", "config": {"id": "port2", "interface": "port2"}},
            {"id": "port3", "config": {"id": "port3", "interface": "port3"}}
          ]
        },
        "protocols": {
          "protocol": [
            {
              "identifier": "openconfig-policy-types:BGP",
              "name": "BGP",
              "config": {"identifier": "openconfig-policy-types:BGP", "name": "BGP"},
              "bgp": {
                "global": {"config": {"as": 64512, "router-id": "192.0.2.101"}},
                "neighbors": {
                  "neighbor": [
                    {"neighbor-address": "192.0.2.2", "config": {"neighbor-address": "192.0.2.2", "peer-as": 64511}},
                    {"neighbor-address": "192.0.2.6", "config": {"neighbor-address": "192.0.2.6", "peer-as": 64511}},
                    {"neighbor-address": "2001:db8::2", "config": {"neighbor-address": "2001:db8::2", "peer-as": 64511}},
                    {"neighbor-address": "2001:db8::6", "config": {"neighbor-address": "2001:db8::6", "peer-as": 64511}}
                  ]
                }
              }
            },
            {
              "identifier": "openconfig-policy-types:DIRECTLY_CONNECTED",
              "name": "DIRECTLY_CONNECTED",
              "config": {"identifier": "openconfig-policy-types:DIRECTLY_CONNECTED", "name": "DIRECTLY_CONNECTED"}
            },
            {
              "identifier": "openconfig-policy-types:STATIC",
              "name": "STATIC",
              "config": {"identifier": "openconfig-policy-types:STATIC", "name": "STATIC"},
              "static-routes": {
                "static": [
                  {
                    "prefix": "0.0.0.0/0",
                    "config": {"prefix": "0.0.0.0/0"},
                    "next-hops": {"next-hop": [{"index": "1", "config": {"index": "1", "next-hop": "192.0.2.10", "preference": 220}}]}
                  },
                  {
                    "prefix": "::/0",
                    "config": {"prefix": "::/0"},
                    "next-hops": {"next-hop": [{"index": "1", "config": {"index": "1", "next-hop": "2001:db8::a", "preference": 220}}]}
                  }
                ]
              }
            }
          ]
        },
        "table-connections": {
          "table-connection": [
            {
              "address-family": "openconfig-types:IPV4",
              "dst-protocol": "openconfig-policy-types:BGP",
              "src-protocol": "openconfig-policy-types:DIRECTLY_CONNECTED",
              "config": {
                "address-family": "openconfig-types:IPV4",
                "default-import-policy": "REJECT_ROUTE",
                "disable-metric-propagation": false,
                "dst-protocol": "openconfig-policy-types:BGP",
                "import-policy": ["redist_lo1_v4"],
                "src-protocol": "openconfig-policy-types:DIRECTLY_CONNECTED"
              }
            },
            {
              "address-family": "openconfig-types:IPV6",
              "dst-protocol": "openconfig-policy-types:BGP",
              "src-protocol": "openconfig-policy-types:DIRECTLY_CONNECTED",
              "config": {
                "address-family": "openconfig-types:IPV6",
                "default-import-policy": "REJECT_ROUTE",
                "disable-metric-propagation": false,
                "dst-protocol": "openconfig-policy-types:BGP",
                "import-policy": ["redist_lo1_v6"],
                "src-protocol": "openconfig-policy-types:DIRECTLY_CONNECTED"
              }
            }
          ]
        },
        "tables": {
          "table": [
            {
              "address-family": "openconfig-types:IPV4",
              "protocol": "openconfig-policy-types:BGP",
              "config": {"address-family": "openconfig-types:IPV4", "protocol": "openconfig-policy-types:BGP"}
            },
            {
              "address-family": "openconfig-types:IPV4",
              "protocol": "openconfig-policy-types:DIRECTLY_CONNECTED",
              "config": {"address-family": "openconfig-types:IPV4", "protocol": "openconfig-policy-types:DIRECTLY_CONNECTED"}
            },
            {
              "address-family": "openconfig-types:IPV6",
              "protocol": "openconfig-policy-types:BGP",
              "config": {"address-family": "openconfig-types:IPV6", "protocol": "openconfig-policy-types:BGP"}
            },
            {
              "address-family": "openconfig-types:IPV6",
              "protocol": "openconfig-policy-types:DIRECTLY_CONNECTED",
              "config": {"address-family": "openconfig-types:IPV6", "protocol": "openconfig-policy-types:DIRECTLY_CONNECTED"}
            }
          ]
        }
      }
    ]
  },
  "qos": {
    "classifiers": {
      "classifier": [
        {
          "name": "dscp_af4_v4",
          "config": {"name": "dscp_af4_v4", "type": "IPV4"},
          "terms": {
            "term": [
              {"id": "0", "config": {"id": "0"}, "conditions": {"ipv4": {"config": {"dscp-set": [32]}}}, "actions": {"config": {"target-group": "AF4"}}},
              {"id": "1", "config": {"id": "1"}, "conditions": {"ipv4": {"config": {"dscp-set": [0]}}}, "actions": {"config": {"target-group": "BE1"}}}
            ]
          }
        },
        {
          "name": "dscp_af4_v6",
          "config": {"name": "dscp_af4_v6", "type": "IPV6"},
          "terms": {
            "term": [
              {"id": "0", "config": {"id": "0"}, "conditions": {"ipv6": {"config": {"dscp-set": [32]}}}, "actions": {"config": {"target-group": "AF4"}}},
              {"id": "1", "config": {"id": "1"}, "conditions": {"ipv6": {"config": {"dscp-set": [0]}}}, "actions": {"config": {"target-group": "BE1"}}}
            ]
          }
        }
      ]
    },
    "forwarding-groups": {
      "forwarding-group": [
        {"name": "AF4", "config": {"name": "AF4", "output-queue": "AF4"}},
        {"name": "BE1", "config": {"name": "BE1", "output-queue": "BE1"}}
      ]
    },
    "interfaces": {
      "interface": [
        {
          "interface-id": "port1",
          "config": {"interface-id": "port1"},
          "input": {
            "classifiers": {
              "classifier": [
                {"type": "IPV4", "config": {"name": "dscp_af4_v4", "type": "IPV4"}},
                {"type": "IPV6", "config": {"name": "dscp_af4_v6", "type": "IPV6"}}
              ]
            }
          },
          "output": {
            "queues": {"queue": [{"name": "AF4", "config": {"name": "AF4"}}, {"name": "BE1", "config": {"name": "BE1"}}]},
            "scheduler-policy": {"config": {"name": "mgmt_qos_sched"}}
          }
        },
        {
          "interface-id": "port2",
          "config": {"interface-id": "port2"},
          "input": {
            "classifiers": {
              "classifier": [
                {"type": "IPV4", "config": {"name": "dscp_af4_v4", "type": "IPV4"}},
                {"type": "IPV6", "config": {"name": "dscp_af4_v6", "type": "IPV6"}}
              ]
            }
          },
          "output": {
            "queues": {"queue": [{"name": "AF4", "config": {"name": "AF4"}}, {"name": "BE1", "config": {"name": "BE1"}}]},
            "scheduler-policy": {"config": {"name": "mgmt_qos_sched"}}
          }
        },
        {
          "interface-id": "port3",
          "config": {"interface-id": "port3"},
          "input": {
            "classifiers": {
              "classifier": [
                {"type": "IPV4", "config": {"name": "dscp_af4_v4", "type": "IPV4"}},
                {"type": "IPV6", "config": {"name": "dscp_af4_v6", "type": "IPV6"}}
              ]
            }
          },
          "output": {
            "queues": {"queue": [{"name": "AF4", "config": {"name": "AF4"}}, {"name": "BE1", "config": {"name": "BE1"}}]},
            "scheduler-policy": {"config": {"name": "mgmt_qos_sched"}}
          }
        }
      ]
    },
    "queues": {
      "queue": [
        {"name": "AF4", "config": {"name": "AF4"}},
        {"name": "BE1", "config": {"name": "BE1"}}
      ]
    },
    "scheduler-policies": {
      "scheduler-policy": [
        {
          "name": "mgmt_qos_sched",
          "config": {"name": "mgmt_qos_sched"},
          "schedulers": {
            "scheduler": [
              {
                "sequence": 0,
                "config": {"priority": "STRICT", "sequence": 0, "type": "openconfig-qos-types:TWO_RATE_THREE_COLOR"},
                "inputs": {"input": [{"id": "AF4", "config": {"id": "AF4", "input-type": "QUEUE", "queue": "AF4", "weight": "100"}}]}
              },
              {
                "sequence": 1,
                "config": {"sequence": 1, "type": "openconfig-qos-types:TWO_RATE_THREE_COLOR"},
                "inputs": {"input": [{"id": "BE1", "config": {"id": "BE1", "input-type": "QUEUE", "queue": "BE1", "weight": "1"}}]}
              }
            ]
          }
        }
      ]
    }
  },
  "routing-policy": {
    "defined-sets": {
      "prefix-sets": {
        "prefix-set": [
          {
            "name": "lo1_v4_prefix_set",
            "config": {"mode": "IPV4", "name": "lo1_v4_prefix_set"},
            "prefixes": {"prefix": [{"ip-prefix": "192.0.2.101/32", "masklength-range": "exact", "config": {"ip-prefix": "192.0.2.101/32", "masklength-range": "exact"}}]}
          },
          {
            "name": "lo1_v6_prefix_set",
            "config": {"mode": "IPV6", "name": "lo1_v6_prefix_set"},
            "prefixes": {"prefix": [{"ip-prefix": "2001:db8::101/128", "masklength-range": "exact", "config": {"ip-prefix": "2001:db8::101/128", "masklength-range": "exact"}}]}
          }
        ]
      }
    },
    "policy-definitions": {
      "policy-definition": [
        {
          "name": "redist_lo1_v4",
          "config": {"name": "redist_lo1_v4"},
          "statements": {
            "statement": [
              {
                "name": "allow_lo1_v4",
                "config": {"name": "allow_lo1_v4"},
                "conditions": {"match-prefix-set": {"config": {"match-set-options": "ANY", "prefix-set": "lo1_v4_prefix_set"}}},
                "actions": {"config": {"policy-result": "ACCEPT_ROUTE"}}
              }
            ]
          }
        },
        {
          "name": "redist_lo1_v6",
          "config": {"name": "redist_lo1_v6"},
          "statements": {
            "statement": [
              {
                "name": "allow_lo1_v6",
                "config": {"name": "allow_lo1_v6"},
                "conditions": {"match-prefix-set": {"config": {"match-set-options": "ANY", "prefix-set": "lo1_v6_prefix_set"}}},
                "actions": {"config": {"policy-result": "ACCEPT_ROUTE"}}
              }
            ]
          }
        }
      ]
    }
  },
  "sampling": {
    "sflow": {
      "config": {"dscp": 32, "enabled": true, "ingress-sampling-rate": 1, "sample-size": 256},
      "collectors": {
        "collector": [
          {"address": "198.51.100.1", "port": 6343, "config": {"address": "198.51.100.1", "network-instance": "mgmt", "port": 6343, "source-address": "192.0.2.101"}},
          {"address": "2001:db8:100::1", "port": 6343, "config": {"address": "2001:db8:100::1", "network-instance": "mgmt", "port": 6343, "source-address": "2001:db8::101"}}
        ]
      },
      "interfaces": {"interface": [{"name": "port4", "config": {"enabled": true, "name": "port4"}}]}
    }
  },
  "system": {
    "aaa": {
      "authentication": {"config": {"authentication-method": ["openconfig-aaa-types:TACACS_ALL"]}},
      "server-groups": {
        "server-group": [
          {
            "name": "TACACS-MGMT",
            "config": {"name": "TACACS-MGMT", "type": "openconfig-aaa:TACACS"},
            "servers": {
              "server": [
                {
                  "address": "198.51.100.1",
                  "config": {"address": "198.51.100.1", "timeout": 5},
                  "tacacs": {"config": {"port": 49, "secret-key": "tacacs-secret", "source-address": "192.0.2.101"}}
                },
                {
                  "address": "2001:db8:100::1",
                  "config": {"address": "2001:db8:100::1", "timeout": 5},
                  "tacacs": {"config": {"port": 49, "secret-key": "tacacs-secret", "source-address": "2001:db8::101"}}
                }
              ]
            }
          }
        ]
      }
    },
    "dns": {
      "servers": {
        "server": [
          {"address": "198.51.100.1", "config": {"address": "198.51.100.1", "port": 53}},
          {"address": "2001:db8:100::1", "config": {"address": "2001:db8:100::1", "port": 53}}
        ]
      }
    },
    "grpc-servers": {
      "grpc-server": [
        {
          "name": "mgmt-grpc",
          "config": {
            "enable": true,
            "listen-addresses": ["192.0.2.101", "2001:db8::101"],
            "name": "mgmt-grpc",
            "network-instance": "mgmt",
            "port": 9339,
            "services": ["GNMI"]
          }
        },
        {
          "name": "mgmt-gribi",
          "config": {
            "enable": true,
            "listen-addresses": ["192.0.2.101", "2001:db8::101"],
            "name": "mgmt-gribi",
            "network-instance": "mgmt",
            "port": 9340,
            "services": ["GRIBI"]
          }
        },
        {
          "name": "mgmt-p4rt",
          "config": {
            "enable": true,
            "listen-addresses": ["192.0.2.101", "2001:db8::101"],
            "name": "mgmt-p4rt",
            "network-instance": "mgmt",
            "port": 9559,
            "services": ["P4RT"]
          }
        }
      ]
    },
    "logging": {
      "remote-servers": {
        "remote-server": [
          {
            "host": "198.51.100.1",
            "config": {"host": "198.51.100.1", "network-instance": "mgmt", "remote-port": 514, "source-address": "192.0.2.101"},
            "selectors": {"selector": [{"facility": "openconfig-system-logging:ALL", "severity": "INFORMATIONAL", "config": {"facility": "openconfig-system-logging:ALL", "severity": "INFORMATIONAL"}}]}
          },
          {
            "host": "2001:db8:100::1",
            "config": {"host": "2001:db8:100::1", "network-instance": "mgmt", "remote-port": 514, "source-address": "2001:db8::101"},
            "selectors": {"selector": [{"facility": "openconfig-system-logging:ALL", "severity": "INFORMATIONAL", "config": {"facility": "openconfig-system-logging:ALL", "severity": "INFORMATIONAL"}}]}
          }
        ]
      }
    },
    "ntp": {
      "config": {"enabled": true},
      "servers": {
        "server": [
          {"address": "198.51.100.1", "config": {"address": "198.51.100.1", "network-instance": "mgmt", "source-address": "192.0.2.101"}},
          {"address": "2001:db8:100::1", "config": {"address": "2001:db8:100::1", "network-instance": "mgmt", "source-address": "2001:db8::101"}}
        ]
      }
    },
    "ssh-server": {"config": {"enable": true}}
  }
}
```

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
  ## Config paths
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/name:
  /interfaces/interface/config/type:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /network-instances/network-instance/config/name:
  /network-instances/network-instance/config/type:
  /network-instances/network-instance/interfaces/interface/config/interface:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/as:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/neighbor-address:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as:
  /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/index:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/preference:
  /network-instances/network-instance/table-connections/table-connection/config/address-family:
  /network-instances/network-instance/table-connections/table-connection/config/default-import-policy:
  /network-instances/network-instance/table-connections/table-connection/config/disable-metric-propagation:
  /network-instances/network-instance/table-connections/table-connection/config/dst-protocol:
  /network-instances/network-instance/table-connections/table-connection/config/import-policy:
  /network-instances/network-instance/table-connections/table-connection/config/src-protocol:
  /qos/classifiers/classifier/config/name:
  /qos/classifiers/classifier/config/type:
  /qos/classifiers/classifier/terms/term/actions/config/target-group:
  /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set:
  /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set:
  /qos/classifiers/classifier/terms/term/config/id:
  /qos/forwarding-groups/forwarding-group/config/name:
  /qos/forwarding-groups/forwarding-group/config/output-queue:
  /qos/interfaces/interface/config/interface-id:
  /qos/interfaces/interface/input/classifiers/classifier/config/name:
  /qos/interfaces/interface/input/classifiers/classifier/config/type:
  /qos/interfaces/interface/output/queues/queue/config/name:
  /qos/interfaces/interface/output/scheduler-policy/config/name:
  /qos/queues/queue/config/name:
  /qos/scheduler-policies/scheduler-policy/config/name:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/priority:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/sequence:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/id:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/input-type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/weight:
  /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode:
  /routing-policy/defined-sets/prefix-sets/prefix-set/config/name:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range:
  /routing-policy/policy-definitions/policy-definition/config/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set:
  /routing-policy/policy-definitions/policy-definition/statements/statement/config/name:
  /sampling/sflow/collectors/collector/config/address:
  /sampling/sflow/collectors/collector/config/network-instance:
  /sampling/sflow/collectors/collector/config/port:
  /sampling/sflow/collectors/collector/config/source-address:
  /sampling/sflow/config/dscp:
  /sampling/sflow/config/enabled:
  /sampling/sflow/config/ingress-sampling-rate:
  /sampling/sflow/config/sample-size:
  /sampling/sflow/interfaces/interface/config/enabled:
  /sampling/sflow/interfaces/interface/config/name:
  /system/aaa/authentication/config/authentication-method:
  /system/aaa/server-groups/server-group/config/name:
  /system/aaa/server-groups/server-group/config/type:
  /system/aaa/server-groups/server-group/servers/server/config/address:
  /system/aaa/server-groups/server-group/servers/server/config/timeout:
  /system/aaa/server-groups/server-group/servers/server/tacacs/config/port:
  /system/aaa/server-groups/server-group/servers/server/tacacs/config/secret-key:
  /system/aaa/server-groups/server-group/servers/server/tacacs/config/source-address:
  /system/dns/servers/server/config/address:
  /system/dns/servers/server/config/port:
  /system/grpc-servers/grpc-server/config/enable:
  /system/grpc-servers/grpc-server/config/listen-addresses:
  /system/grpc-servers/grpc-server/config/name:
  /system/grpc-servers/grpc-server/config/network-instance:
  /system/grpc-servers/grpc-server/config/port:
  /system/grpc-servers/grpc-server/config/services:
  /system/logging/remote-servers/remote-server/config/host:
  /system/logging/remote-servers/remote-server/config/network-instance:
  /system/logging/remote-servers/remote-server/config/remote-port:
  /system/logging/remote-servers/remote-server/config/source-address:
  /system/logging/remote-servers/remote-server/selectors/selector/config/facility:
  /system/logging/remote-servers/remote-server/selectors/selector/config/severity:
  /system/ntp/config/enabled:
  /system/ntp/servers/server/config/address:
  /system/ntp/servers/server/config/network-instance:
  /system/ntp/servers/server/config/source-address:
  /system/ssh-server/config/enable:

  ## State paths
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /qos/interfaces/interface/output/queues/queue/state/dropped-pkts:
  /qos/interfaces/interface/output/queues/queue/state/name:
  /qos/interfaces/interface/output/queues/queue/state/transmit-octets:
  /qos/interfaces/interface/output/queues/queue/state/transmit-pkts:
  /qos/interfaces/interface/output/scheduler-policy/state/name:
  /qos/scheduler-policies/scheduler-policy/state/name:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/state/priority:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/state/sequence:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/state/type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/state/id:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/state/input-type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/state/queue:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/state/weight:
  /sampling/sflow/collectors/collector/state/address:
  /sampling/sflow/collectors/collector/state/network-instance:
  /sampling/sflow/collectors/collector/state/port:
  /sampling/sflow/collectors/collector/state/source-address:
  /sampling/sflow/state/dscp:
  /sampling/sflow/state/enabled:
  /system/aaa/authentication/state/authentication-method:
  /system/aaa/server-groups/server-group/servers/server/state/address:
  /system/aaa/server-groups/server-group/servers/server/tacacs/state/port:
  /system/aaa/server-groups/server-group/servers/server/tacacs/state/source-address:
  /system/dns/servers/server/state/address:
  /system/dns/servers/server/state/port:
  /system/grpc-servers/grpc-server/state/enable:
  /system/grpc-servers/grpc-server/state/listen-addresses:
  /system/grpc-servers/grpc-server/state/name:
  /system/grpc-servers/grpc-server/state/network-instance:
  /system/grpc-servers/grpc-server/state/port:
  /system/grpc-servers/grpc-server/state/services:
  /system/logging/remote-servers/remote-server/state/host:
  /system/logging/remote-servers/remote-server/state/network-instance:
  /system/logging/remote-servers/remote-server/state/remote-port:
  /system/logging/remote-servers/remote-server/state/source-address:
  /system/ntp/state/enabled:
  /system/ntp/servers/server/state/address:
  /system/ntp/servers/server/state/network-instance:
  /system/ntp/servers/server/state/source-address:
  /system/ssh-server/state/enable:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
  gnoi:
    system.System.Ping:
    system.System.Time:
  gribi:
    gRIBI.Get:
    gRIBI.Modify:
```

## Required DUT platform

*   MFF
