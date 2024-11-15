# RT-1.19: BGP 2-Byte and 4-Byte ASN support

## Summary

BGP 2-Byte and 4-Byte ASN support

## Procedure

*   Establish BGP sessions as follows and verify all the sessions are established
    *   ATE (2-byte) - DUT (4-byte) - eBGP IPv4 with ASN < 65535 on DUT side
    *   ATE (2-byte) - DUT (4-byte) - eBGP IPv6 with ASN < 65535 on DUT side
    *   ATE (4-byte) - DUT (4-byte) - eBGP IPv4
    *   ATE (4-byte) - DUT (4-byte) - eBGP IPv6
    *   ATE (2-byte) - DUT (4-byte) - iBGP IPv4 with ASN < 65535 on DUT side
    *   ATE (4-byte) - DUT (4-byte) - iBGP IPv6 with ASN < 65535 on DUT side
    *   ATE (4-byte) - DUT (4-byte) - iBGP IPv4
    *   ATE (4-byte) - DUT (4-byte) - iBGP IPv6

## OpenConfig Path and RPC Coverage
```yaml
paths:
    ## Config Parameter Coverage

    /network-instances/network-instance/protocols/protocol/bgp/global/config/as:
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as:
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/local-as:

    ## Telemetry Parameter Coverage

    /network-instances/network-instance/protocols/protocol/bgp/global/state/as:
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/peer-as:
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/local-as:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
```
