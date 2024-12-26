# RT-1.2: BGP Policy & Route Installation

## Summary

Base BGP policy configuration and route installation.

## Procedure

*   Establish eBGP sessions between:
    *   ATE port-1 and DUT port-1
*   For IPv4 and IPv6 routes:
    *   Advertise IPv4 prefixes over IPv4 neighbor from ATE port-1, observe received prefixes at ATE port-2.
    *   Similarly advertise IPv6 prefixes over IPv6 neighbor from ATE port-1, observe received prefixes at ATE port-2.
    *   Specify table based policy configuration under peer-group AFI to cover
        *   Default accept for policies.
        *   Default deny for policies.
        *   Explicitly specifying local preference.
        *   Explicitly specifying MED value.
        *   Explicitly prepending AS for advertisement with a specified AS
            number.
    *   Validate that traffic can be forwarded to **all** installed routes
        between ATE port-1 and ATE port-2, validate that flows between all
        denied routes cannot be forwarded.
    *   Validate that traffic is not forwarded to withdrawn routes between ATE
        port-1 and ATE port-2.

## OpenConfig Path and RPC Coverage
```yaml
paths:
  ## Config Parameter Coverage
  /routing-policy/policy-definitions/policy-definition/config/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-local-pref:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-med:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/repeat-n:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/asn:

  ## Telemetry Parameter Coverage
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/neighbor-address:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/state/peer-group-name:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/state/export-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/state/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/installed:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received-pre-policy:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/sent:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Subscribe:

```
