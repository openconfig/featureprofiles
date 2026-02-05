# RT-2.14: IS-IS Drain Test

## Summary

Ensure that IS-IS metric change can drain traffic from a DUT trunk interface

## Canonical OC
```json
{}
```  

## Procedure
* Connect three ATE ports to the DUT
* Port-2 and port-3 each makes a one-member trunk port with the same ISIS metric 10 configured for the trunk interfaces (trunk-2 and trunk-3).  
* Configure a destination network-a connected to trunk-2 and trunk-3.
* Verify that IPv4 prefix advertised by ATE is correctly installed into DUTs forwarding table.
* Send 10K IPv4 traffic flows from ATE port-1 to network-a. Validate that traffic is going via trunk-2 and trunk-3 and there is no traffic loss
* Change the ISIS metric of trunk-2 to 1000 value. Verify prefix is correctly installed into DUTs forwarding table.Validate that 100% of the traffic is going out of only trunk-3 and there is no traffic loss.
* Revert back the ISIS metric on trunk-2. Verify prefix is correctly installed into DUTs forwarding table.Validate that the traffic is going via both trunk-2 and trunk-3, and there is no traffic loss.

## OpenConfig Path and RPC Coverage
```yaml
paths:
  /interfaces/interface/aggregation/config/lag-type:
  /interfaces/interface/aggregation/config/min-links:
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/name:
  /interfaces/interface/config/type:
  /interfaces/interface/ethernet/config/aggregate-id:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/protocols/protocol/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/global/afi-safi/af/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/global/config/instance:
  /network-instances/network-instance/protocols/protocol/isis/global/config/level-capability:
  /network-instances/network-instance/protocols/protocol/isis/global/config/max-ecmp-paths:
  /network-instances/network-instance/protocols/protocol/isis/global/config/net:
  /network-instances/network-instance/protocols/protocol/isis/global/lsp-bit/overload-bit/config/set-bit:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/afi-safi/af/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/config/circuit-type:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/interface-ref/config/interface:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/interface-ref/config/subinterface:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/adjacency-state:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/afi-safi/af/config/metric:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/config/metric-style:
rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT Platform Requirement

vRX
