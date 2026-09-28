# P4RT-5.2: Traceroute Packetout

## Summary

Verify that traceroute packets can be sent by the controller.

### Submit to ingress specific behavior

The egress port value must be set to a non empty value but will not be used. The
setting must not be interpreted as the actual egress port id.

## Procedure

*   Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2

*   Install a set of routes on the device in both the default and TE VRFs.

*   Enable the P4RT server on the device..

*   Connect a P4RT client and configure the forwarding pipeline.

*   Send IPv4 traceroute packets from the client with varying size.

*   Verify that the packet is received on the ATE on the port corresponding to the routing table in the default VRF.

*   Send an IPv6 traceroute packets from the client with varying size and verify that it is received correctly by the ATE.

*   Repeat for each packet metadata combination shown in the table below.

| egress_port | submit_to_ingress | padding | expected behaviour
| ------ | ------ | ------ | ------ |
| DUT port-2 | 0x0 | 0x0 | traffic received on ATE port-2
| DUT port-2 | | 0x0 | traffic received on ATE port-2
| DUT port-2 | | | traffic received on ATE port-2
| DUT port-2 | 0x1 | 0x0 | traffic received on ATE port-1
| | 0x1 | | traffic received on ATE port-1
|  | 0x1 | 0x0 | traffic received on ATE port-1
"TBD BY SWITCH" | 0x1 | 0x0 | traffic received on ATE port-1
"TBD BY SWITCH" | 0x1 | | traffic received on ATE port-1
| DUT port-2 | 0x1 | | traffic received on ATE port-1
"TBD BY SWITCH" | 0x0 | 0x0 | no traffic received
"TBD BY SWITCH" | 0x0 | | no traffic received
"TBD BY SWITCH" | | 0x0 | no traffic received
| "TBD BY SWITCH" | | | no traffic received
|  | 0x0 | 0x0 | no traffic received
| | 0x0 | | no traffic received
| | | 0x0 | no traffic received
| | | | no traffic received

*   Validate:

    *   Traffic received over the appropriate ATE port.
## OpenConfig Path and RPC Coverage

This example yaml defines the OC paths intended to be covered by this test.  OC paths used for test environment setup are not required to be listed here.

```yaml
paths:
  # config paths
  /interfaces/interface/config/id:
  /components/component/integrated-circuit/config/node-id:
    platform_type: ["INTEGRATED_CIRCUIT"]
  # state paths 
  /interfaces/interface/state/id:
  /components/component/integrated-circuit/state/node-id:
    platform_type: ["INTEGRATED_CIRCUIT"]

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```
