# TE-2.3: gRIBI IPv6 Entry with Prefix Length > 64

## Summary

Validate IPv6 route entry support in gRIBI, specifically focusing on IPv6 prefixes with prefix length strictly greater than 64 bits (such as /65, /96, /126, /127, and /128). This verifies that the device agent (DA) and underlying switch ASIC (e.g. TCAM / ALPM paired memory tables) correctly program and forward traffic for longer IPv6 prefixes without prefix bitmask truncation, incorrect table allocation, or route lookup failures.

## Testbed type

* [TESTBED_DUT_ATE_4LINKS](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4ports.testbed)

## Procedure

*   Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2, and ATE port-3 to DUT port-3.
*   Establish gRIBI client connection with DUT, negotiating `RIB_AND_FIB_ACK` as the requested `ack_type` and persistence mode `PRESERVE`. Make it become leader. Flush all entries after each case.

### Test Cases

*   Using gRIBI Modify RPC install the following IPv6Entry sets, and validate the specified behaviours:
    *   **Single next-hop with prefix length > 64**:
        *   Install `2001:db8:100::/65` to NextHopGroup containing one NextHop specified to ATE port-2.
        *   Forward packets from ATE port-1 destined to `2001:db8:100::/65` and verify 100% traffic arrives on ATE port-2 with 0% traffic loss.
    *   **Multiple next-hops (ECMP) with prefix length > 64**:
        *   Install `2001:db8:100::/65` to NextHopGroup containing two NextHop entries specified to ATE ports 2 and 3.
        *   Forward packets destined to `2001:db8:100::/65` and verify traffic is balanced across ATE ports 2 and 3.
    *   **Multiple next-hops with MAC override with prefix length > 64**:
        *   Install `2001:db8:100::/65` to NextHopGroup containing NextHops with destination MAC override, and verify traffic forwards successfully without packet loss.
    *   **Non-existent next-hop with prefix length > 64**:
        *   Send Modify() installing `2001:db8:100::/65` referencing next-hops that do not exist. Validate that FAILED error is received and traffic is dropped.
    *   **Downed next-hop interface with prefix length > 64**:
        *   Install `2001:db8:100::/65` to NextHopGroup with NextHops on port 2 and port 3.
        *   Set link state down on port 2.
        *   Forward packets and verify 100% traffic reroutes to port 3 without packet loss.
    *   **Longest Prefix Match (LPM) Discrimination (/64 vs /65)**:
        *   Install `2001:db8:200::/64` -> NextHop(ATE port-2).
        *   Install `2001:db8:200::/65` -> NextHop(ATE port-3).
        *   Send Flow 1 destined to `2001:db8:200::1/128` (matches `/64` and `/65`) -> verify it arrives on ATE port-3 due to LPM.
        *   Send Flow 2 destined to `2001:db8:200:0:8000::1/128` (matches `/64` but has bit 65 = 1, so outside `/65`) -> verify it arrives on ATE port-2.
    *   **Boundary Prefix Length Sweep (/65, /96, /126, /127, /128)**:
        *   Install and verify forwarding for boundary prefixes:
            *   `/65` (`2001:db8:101::/65`)
            *   `/96` (`2001:db8:102::/96`)
            *   `/126` (`2001:db8:103::/126`)
            *   `/127` (`2001:db8:104::/127`)
            *   `/128` (`2001:db8:105::1/128`)
        *   Verify zero packet loss for each prefix length to ensure hardware table allocation succeeds across all bit boundaries.
    *   **Route Deletion and Flush**:
        *   Verify route removal and FlushAll cleanly clears all IPv6 entries from FIB.

## Config Parameter coverage

N/A

## Telemetry Parameter coverage

N/A

## Protocol/RPC Parameter coverage

*   gRIBI
    *   Modify()
        *   ModifyRequest:
            *   AFTOperation:
                *   id
                *   network_instance
                *   op
                *   Ipv6
                    *   Ipv6EntryKey: prefix
                    *   Ipv6Entry: next_hop_group
                *   next_hop_group
                    *   NextHopGroupKey: id
                    *   NextHopGroup: next_hop
                *   next_hop
                    *   NextHopKey: id
                    *   NextHop:
                        *   ip_address
        *   ModifyResponse:
            *   AFTResult:
                *   id
                *   status
                
## OpenConfig Path and RPC Coverage
```yaml
paths:
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/name:
  /interfaces/interface/config/type:
  /interfaces/interface/ethernet/config/port-speed:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/config/link-layer-address:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/config/link-layer-address:
  /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled:
  /network-instances/network-instance/interfaces/interface/config/id:
  /network-instances/network-instance/interfaces/interface/config/interface:
  /network-instances/network-instance/interfaces/interface/config/subinterface:
  /network-instances/network-instance/protocols/protocol/config/identifier:
  /network-instances/network-instance/protocols/protocol/config/name:
  /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/index:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/interface-ref/config/interface:
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
  gribi:
    gRIBI.Flush:
    gRIBI.Get:
    gRIBI.Modify:
```

## Canonical OC

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
