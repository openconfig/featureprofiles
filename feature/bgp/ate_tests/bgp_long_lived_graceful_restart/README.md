# RT-1.14: BGP Long-Lived Graceful Restart

## Summary

BGP Long-Lived Graceful Restart

## Procedure

*   Establish BGP sessions as follows between ATE and DUT
    *   ATE  Port-1  --------  DUT  Port-1. 
    *   DUT  Port-2  --------  ATE  Port-2.
*   Validate received capabilities at DUT and ATE reflect support for graceful restart at the default 
    timer values.
*   For IPv4 and IPv6 routes:
    *   Advertise prefixes from ATE to DUT. 
    *   Trigger DUT session restart by disconnecting TCP session between DUT - ATE Port2 and determine that:
        *   Forwarded traffic between between ATE port-1 and DUT port-1 for the duration of the  
            specified stale routes time (default), during the stale state traffic shouldnt be impacted. 
        *   Dropped after the stale routes timer has expired.
        *   Forwarded again between ATE port-1 and DUT port-1 after the session is re-established. 
    *   Enable Long-Lived Graceful Restart on a per-bgp neighbor basis on DUT. 
        Specify the LLGR timer to be configured to be 600 seconds. Trigger DUT session restart.  
        *   Ensure that the stale prefixes are not withdrawn until the end of the LLGR timer value. 
        *   Ensure that prefixes are withdrawn, and traffic cannot be forwarded between ATE port-1 
            and port-2 after the LLGR timer expiry. 
    *   Induce
        *   Import policy changes 
        *   Export policy changes 
        *   Add 5 more BGP peers 
        *   Remove 5 more BGP peers(block the BGP using filter) 
        *   Routing Process restart 
    *   During above mentioned changes, LLGR shouldn't be impacted and persist during/after the 
        changes.

## Config Parameter coverage

*   /global/graceful-restart
*   /global/graceful-restart/config 
*   /global/graceful-restart/config/enabled 
*   /global/graceful-restart/config/restart-time 
*   /global/graceful-restart/config/stale-routes-time 
*   /global/graceful-restart/config/helper-only 
*   /global/afi-safis/afi-safi/graceful-restart 
*   /global/afi-safis/afi-safi/graceful-restart/config 
*   /global/afi-safis/afi-safi/graceful-restart/config/enabled 
*   /neighbors/neighbor/graceful-restart 
*   /neighbors/neighbor/graceful-restart/config 
*   /neighbors/neighbor/graceful-restart/config/enabled 
*   /neighbors/neighbor/graceful-restart/config/restart-time 
*   /neighbors/neighbor/graceful-restart/config/stale-routes-time 
*   /neighbors/neighbor/graceful-restart/config/helper-only 
*   /neighbors/neighbor/afi-safis/afi-safi/graceful-restart 
*   /neighbors/neighbor/afi-safis/afi-safi/graceful-restart/config 
*   /neighbors/neighbor/afi-safis/afi-safi/graceful-restart/config/enabled 

## Telemetry Parameter coverage

*   /global/graceful-restart/state 
*   /global/graceful-restart/state/enabled 
*   /global/graceful-restart/state/restart-time 
*   /global/graceful-restart/state/stale-routes-time 
*   /global/graceful-restart/state/helper-only 
*   /global/afi-safis/afi-safi/graceful-restart/state 
*   /global/afi-safis/afi-safi/graceful-restart/state/enabled 
*   /neighbors/neighbor/graceful-restart/state 
*   /neighbors/neighbor/graceful-restart/state/enabled 
*   /neighbors/neighbor/graceful-restart/state/restart-time 
*   /neighbors/neighbor/graceful-restart/state/stale-routes-time 
*   /neighbors/neighbor/graceful-restart/state/helper-only 
*   /neighbors/neighbor/graceful-restart/state/peer-restart-time 
*   /neighbors/neighbor/graceful-restart/state/peer-restarting 
*   /neighbors/neighbor/graceful-restart/state/local-restarting 
*   /neighbors/neighbor/graceful-restart/state/mode 
*   /neighbors/neighbor/afi-safis/afi-safi/graceful-restart/state 
*   /neighbors/neighbor/afi-safis/afi-safi/graceful-restart/state/enabled 
*   /neighbors/neighbor/afi-safis/afi-safi/graceful-restart/state/received 
*   /neighbors/neighbor/afi-safis/afi-safi/graceful-restart/state/advertised 

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /acl/acl-sets/acl-set/acl-entries/acl-entry/actions/config/forwarding-action:
  /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/destination-address:
  /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/source-address:
  /acl/interfaces/interface/ingress-acl-sets/ingress-acl-set/config/set-name:
  /acl/interfaces/interface/ingress-acl-sets/ingress-acl-set/config/type:
  /acl/interfaces/interface/interface-ref/config/interface:
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/type:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/vlan/match/single-tagged/config/vlan-id:
  /network-instances/network-instance/config/type:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/as:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id:
  /network-instances/network-instance/protocols/protocol/bgp/global/graceful-restart/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/global/graceful-restart/config/restart-time:
  /network-instances/network-instance/protocols/protocol/bgp/global/graceful-restart/config/stale-routes-time:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-group:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/peer-restart-time:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/supported-capabilities:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/hold-time:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/keepalive-interval:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/export-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-as:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-group-name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-local-pref:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-med:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result:
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

N/A
