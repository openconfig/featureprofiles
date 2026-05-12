# TE-9.3: FIB FAILURE DUE TO HARDWARE RESOURCE EXHAUST

## Summary

Validate gRIBI FIB_FAILED functionality.

## Procedure

*   Connect ATE port-1 to DUT port-1, and ATE port-2 to DUT port-2.

*   Establish a gRIBI connection (SINGLE_PRIMARY and PRESERVE mode) to the DUT.

*   Establish BGP session between ATE Port1 --- DUT Port1. Inject unique BGP routes to exhaust FIB on DUT.

*   Continuously injecting the following gRIBI structure until FIB FAILED is received. 
    Each DstIP and VIP should be unique and of /32. All the NHG and NH should be unique (of unique ID).
    DstIP/32 -> NHG -> NH {next-hop:} -> VIP/32 -> NHG -> NH {next-hop: AtePort2Ip}
    
*   Expect FIB_PROGRAMMED message until the first FIB_FAILED message received.

*   Validate that traffic for the FIB_FAILED route will not get forwarded. 

*   Pick any route that received FIB_PROGRAMMED. Validate that traffic hitting the route should be forwarded to port2 


## OpenConfig Path and RPC Coverage
```yaml
paths:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/as:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-group:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/export-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-as:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-group-name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result:
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

## Config parameter coverage

## Telemery parameter coverage
