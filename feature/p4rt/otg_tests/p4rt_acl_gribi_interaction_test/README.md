# TE-1.3: P4RT ACL Interaction with gRIBI Forwarding

## Summary

This test verifies the predictable behavior of the data plane when a packet is
matched by both a P4RT-programmed table entry (such as an ACL) and a
gRIBI-programmed route. The test ensures that the actual packet treatment
conforms to the expected vendor-documented pipeline order (e.g., P4RT ACL
taking precedence over gRIBI L3 lookup), and that the interaction is consistent
and stable.

## Testbed type

* [`TESTBED_DUT_ATE_2LINKS`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

### Test environment setup

* Connect ATE `port-1` to DUT `port-1` and ATE `port-2` to DUT `port-2`.
* Configure IPv4 addressing on the connected interfaces in the `DEFAULT` network instance:
  * DUT `port-1` (`subinterface 0`): `192.0.2.1/30`, ATE `port-1`: `192.0.2.2/30` (Traffic Ingress Link, subnet `192.0.2.0/30`)
  * DUT `port-2` (`subinterface 0`): `192.0.2.5/30`, ATE `port-2`: `192.0.2.6/30` (Traffic Egress Link, subnet `192.0.2.4/30`)
* Bring up the interfaces and verify they are `UP` using telemetry `/interfaces/interface/state/oper-status`.
* Resolve IPv4 ARP on ATE `port-1` (`192.0.2.2`) and ATE `port-2` (`192.0.2.6`) before sending traffic.
* Configure P4RT node and interface IDs on the DUT via gNMI:
  * Configure `/components/component/integrated-circuit/config/node-id` (e.g., `device_id = 1`) for the target forwarding integrated circuit.
  * Configure `/interfaces/interface/config/id` on DUT `port-1` (`id = 1`) and DUT `port-2` (`id = 2`).
* Establish a P4RT client connection to the DUT (`device_id = 1`, `election_id = {high: 0, low: 1}`), perform `MasterArbitrationUpdate` via `P4Runtime.StreamChannel` to become the primary controller, and push the P4Info forwarding pipeline configuration via `P4Runtime.SetForwardingPipelineConfig` (`action = VERIFY_AND_COMMIT`).
* Establish a gRIBI client connection to the DUT in the `DEFAULT` network instance (`persistence: PRESERVE`, `redundancy: SINGLE_PRIMARY`, `election_id: {low: 1, high: 0}`), verify it becomes the leader, and execute `gRIBI.Flush` on `DEFAULT` to ensure a clean initial AFT state.

### TE-1.3.1 - Baseline gRIBI Forwarding

* Step 1 - Program a gRIBI route
  * Use `gRIBI.Modify` RPC (requesting `FIB_PROGRAMMED` ACK) to install the following AFT entries in network instance `DEFAULT`:
    * `NextHop` (`index = 1`, `network-instance = DEFAULT`): `ip-address = 192.0.2.6` (ATE `port-2`).
    * `NextHopGroup` (`id = 1`, `network-instance = DEFAULT`): references `NextHop(index = 1)` with `weight = 1`.
    * `IPv4Entry` (`prefix = 198.51.100.0/24`, `network-instance = DEFAULT`): references `NextHopGroup(id = 1)` (`next-hop-group-network-instance = DEFAULT`).
  * Validate route installation using `gNMI.Subscribe` (ON_CHANGE) or `gNMI.Get` on paths:
    * `/network-instances/network-instance[name=DEFAULT]/afts/ipv4-unicast/ipv4-entry[prefix=198.51.100.0/24]/state/prefix`
    * `/network-instances/network-instance[name=DEFAULT]/afts/ipv4-unicast/ipv4-entry[prefix=198.51.100.0/24]/state/next-hop-group`
    * `/network-instances/network-instance[name=DEFAULT]/afts/next-hop-groups/next-hop-group[id=1]/state/id`
    * `/network-instances/network-instance[name=DEFAULT]/afts/next-hops/next-hop[index=1]/state/ip-address`

* Step 2 - Send Traffic
  * Send IPv4 test stream (`Flow-1`, frame size `512` bytes, rate `1,000 fps`) from ATE `port-1` (source IP `192.0.2.2`) to destination IP `198.51.100.1`.
  * Send IPv4 control stream (`Flow-2`, frame size `512` bytes, rate `1,000 fps`) from ATE `port-1` (source IP `192.0.2.2`) to destination IP `198.51.100.2`.
  * Verify `0%` packet loss (`Rx frames == Tx frames`) at ATE `port-2` for both `Flow-1` and `Flow-2` to confirm baseline gRIBI forwarding is active.

### TE-1.3.2 - P4RT ACL Drop action takes precedence over gRIBI Forwarding

* Step 1 - Program a P4RT ACL rule
  * Use `P4Runtime.Write` (`Update.Type = INSERT`) to program an ingress ACL table entry (`acl_ingress_table`, `priority = 100`) matching IPv4 traffic (`is_ipv4 = 0x1`) with ternary destination IP `dst_ip = 198.51.100.1` and `/32` mask `0xffffffff` (`198.51.100.1/32`) and action `acl_drop` (`DROP`).
  * Validate the P4RT ACL installation by ensuring a successful `P4Runtime.Write` response is received and verifying the entry via `P4Runtime.Read`.

* Step 2 - Send Traffic
  * Send test stream (`Flow-1`, frame size `512` bytes, rate `1,000 fps`): IPv4 traffic from ATE `port-1` (source IP `192.0.2.2`) to destination IP `198.51.100.1` (matching both the gRIBI route `198.51.100.0/24` and the P4RT ACL `198.51.100.1/32`).
  * Send control stream (`Flow-2`, frame size `512` bytes, rate `1,000 fps`): IPv4 traffic from ATE `port-1` (source IP `192.0.2.2`) to destination IP `198.51.100.2` (matching only the gRIBI route `198.51.100.0/24`).

* Step 3 - Validation with pass/fail criteria
  * Verify traffic to `198.51.100.1` (`Flow-1`) is completely dropped (`100%` loss, `0` packets received at ATE `port-2`) as the P4RT ACL `DROP` action takes precedence over the gRIBI route.
  * Verify traffic to `198.51.100.2` (`Flow-2`) is forwarded correctly to ATE `port-2` with `0%` loss (`Rx frames == Tx frames`), preventing false positives.

### TE-1.3.3 - Scaled gRIBI and P4RT ACL Interaction

* Step 1 - Program Scaled routes and ACLs
  * Use `gRIBI.Modify` (requesting `FIB_PROGRAMMED` ACK) to program 1,000 IPv4 routes in network instance `DEFAULT` for prefixes `100.64.0.0/24` through `100.67.231.0/24` (i.e., `100.<64 + floor(i/256)>.<i mod 256>.0/24` for $i \in [0, 999]$), all referencing `NextHopGroup(id = 1)` (`NextHop(index = 1)` = `192.0.2.6` on ATE `port-2`).
  * Verify all 1,000 routes (`100.64.0.0/24` through `100.67.231.0/24`) are programmed in the AFT via batched `gNMI.Get` / `gNMI.Subscribe` on `/network-instances/network-instance[name=DEFAULT]/afts/ipv4-unicast/ipv4-entry[prefix=...]/state/prefix`.
  * Use `P4Runtime.Write` (`Update.Type = INSERT`) to program 1,000 P4RT IPv4 ACL rules (`acl_ingress_table`, `priority = 100`, `is_ipv4 = 0x1`) matching the `.1/32` host IP (`mask = 0xffffffff`) within each of the 1,000 prefixes: `dst_ip = 100.64.0.1 & 0xffffffff` through `100.67.231.1 & 0xffffffff` (i.e., `100.<64 + floor(i/256)>.<i mod 256>.1/32` for $i \in [0, 999]$) with action `acl_drop` (`DROP`).
  * Validate installation via successful `P4Runtime.Write` response and `P4Runtime.Read` verification.

* Step 2 - Send Traffic
  * Send matched scaled traffic stream (`Flow-Scaled-Drop`, frame size `512` bytes, rate `10,000 fps`) from ATE `port-1` (source IP `192.0.2.2`) to the 1,000 ACL-matched destination host IPs: `100.64.0.1` through `100.67.231.1` (count `1,000`, i.e., `100.<64 + floor(i/256)>.<i mod 256>.1` for $i \in [0, 999]$).
  * Send unmatched scaled traffic stream (`Flow-Scaled-Forward`, frame size `512` bytes, rate `10,000 fps`) from ATE `port-1` (source IP `192.0.2.2`) to the 1,000 ACL-unmatched destination host IPs within the same routed `/24` subnets: `100.64.0.2` through `100.67.231.2` (count `1,000`, i.e., `100.<64 + floor(i/256)>.<i mod 256>.2` for $i \in [0, 999]$).

* Step 3 - Validation with pass/fail criteria
  * Verify traffic to the 1,000 matched host IPs (`Flow-Scaled-Drop`, `100.64.0.1` through `100.67.231.1`) is completely dropped (`100%` loss, `0` packets received at ATE `port-2`).
  * Verify traffic to the 1,000 unmatched host IPs (`Flow-Scaled-Forward`, `100.64.0.2` through `100.67.231.2`) is forwarded to ATE `port-2` with `0%` loss (`Rx frames == Tx frames`).
  * Clean up the scaled entries before proceeding: delete the 1,000 scaled P4RT ACL entries (`100.64.0.1/32` through `100.67.231.1/32`) via `P4Runtime.Write` (`DELETE`) and delete the 1,000 scaled gRIBI `IPv4Entry` prefixes (`100.64.0.0/24` through `100.67.231.0/24`) via `gRIBI.Modify` (`DELETE`).

### TE-1.3.4 - Reverting P4RT ACL restores gRIBI forwarding

* Step 1 - Delete P4RT ACL rule
  * Use `P4Runtime.Write` (`Update.Type = DELETE`) to delete the P4RT IPv4 ACL rule matching `dst_ip = 198.51.100.1 & 0xffffffff` (`198.51.100.1/32`, configured in TE-1.3.2).
  * Validate deletion via successful `P4Runtime.Write` response and confirm via `P4Runtime.Read` that the rule is no longer present.

* Step 2 - Send Traffic
  * Send `Flow-1` (source IP `192.0.2.2` to destination IP `198.51.100.1`, frame size `512` bytes, rate `1,000 fps`) and `Flow-2` (source IP `192.0.2.2` to destination IP `198.51.100.2`, frame size `512` bytes, rate `1,000 fps`) from ATE `port-1`.

* Step 3 - Validation with pass/fail criteria
  * Verify traffic to `198.51.100.1` (`Flow-1`) and `198.51.100.2` (`Flow-2`) is forwarded correctly to ATE `port-2` with `0%` loss (`Rx frames == Tx frames`), confirming traffic to `198.51.100.1` falls back to the active gRIBI route (`198.51.100.0/24`).

### TE-1.3.5 - Removal of gRIBI route with active P4RT ACL

* Step 1 - Delete gRIBI route
  * Re-program (`P4Runtime.Write`, `Update.Type = INSERT`) the P4RT IPv4 ACL rule (`acl_ingress_table`, `priority = 100`, `is_ipv4 = 0x1`) matching `dst_ip = 198.51.100.1 & 0xffffffff` (`198.51.100.1/32`) with action `acl_drop` (`DROP`), and verify installation via `P4Runtime.Read`.
  * Use `gRIBI.Modify` (`DELETE`, requesting `FIB_PROGRAMMED` ACK) to delete the gRIBI `IPv4Entry` for destination prefix `198.51.100.0/24` in network instance `DEFAULT`.
  * Validate route deletion via gNMI telemetry ensuring `/network-instances/network-instance[name=DEFAULT]/afts/ipv4-unicast/ipv4-entry[prefix=198.51.100.0/24]/state/prefix` is removed from the AFT state.

* Step 2 - Send Traffic
  * Send test stream (`Flow-1`, frame size `512` bytes, rate `1,000 fps`): IPv4 traffic from ATE `port-1` (source IP `192.0.2.2`) to destination IP `198.51.100.1` (matches active P4RT ACL `198.51.100.1/32`).
  * Send control stream (`Flow-2`, frame size `512` bytes, rate `1,000 fps`): IPv4 traffic from ATE `port-1` (source IP `192.0.2.2`) to destination IP `198.51.100.2` (no matching route in AFT).

* Step 3 - Validation with pass/fail criteria
  * Verify traffic to `198.51.100.1` (`Flow-1`) is completely dropped (`100%` loss, `0` packets received at ATE `port-2`) as it matches the active P4RT ACL rule.
  * Verify traffic to `198.51.100.2` (`Flow-2`) is completely dropped (`100%` loss, `0` packets received at ATE `port-2`) because there is no matching route in the FIB.

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
  /components/component/integrated-circuit/config/node-id:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /interfaces/interface/config/id:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/config/enabled:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address:
  /network-instances/network-instance/config/name:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
  gribi:
    gRIBI.Flush:
    gRIBI.Get:
    gRIBI.Modify:
```

## Required DUT platform

* FFF
