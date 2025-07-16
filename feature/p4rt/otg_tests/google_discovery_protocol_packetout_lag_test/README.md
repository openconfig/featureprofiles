# P4RT-3.21: Google Discovery Protocol: PacketOut with LAG

## Summary

Verify that GDP packets can be sent by the controller.

## Testbed Type

* https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

## Procedure

* Configure two lag bundles between ATE and DUT with one member port in each of the LAG.
  A[ATE:LAG1] <---> B[LAG1:DUT];
  C[ATE:LAG2] <---> D[LAG2:DUT];
*	Enable the P4RT server on the device.
*	Connect two P4RT clients in a master/secondary configuration.
*	Configure the forwarding pipeline and install the P4RT table entry required for GDP.
*	Send a GDP packet from the master with egress_singleton_port set to one of the connected interfaces.
*	Verify that the GDP packet is received on the ATE port connected to the indicated interface.
*	Repeat sending the packet in the same way but from the secondary connection.
*	Verify that the packet is not received on the ATE.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
    ## Config parameter coverage
    /interfaces/interface/ethernet/config/port-speed:
    /interfaces/interface/ethernet/config/duplex-mode:
    /interfaces/interface/ethernet/config/aggregate-id:
    /interfaces/interface/aggregation/config/lag-type:
    /network-instances/network-instance/vlans/vlan/state/vlan-id:

    ## Telemetry parameter coverage
    /lacp/interfaces/interface/members/member/state/counters/lacp-in-pkts:
    /lacp/interfaces/interface/members/member/state/counters/lacp-out-pkts:
    /lacp/interfaces/interface/members/member/state/counters/lacp-rx-errors:
    /lacp/interfaces/interface/name:
    /lacp/interfaces/interface/state/name:
    /lacp/interfaces/interface/members/member/interface:
    /lacp/interfaces/interface/members/member/state/interface:


    ## Protocol/RPC Parameter Coverage
rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
    gNMI.Get:
```

## Required DUT platform

* FFF
