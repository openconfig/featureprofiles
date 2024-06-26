# RT-1.24: BGP 2-Byte and 4-Byte ASN support with policy

## Summary

BGP 2-Byte and 4-Byte ASN support with policy

## Procedure

*   Establish BGP sessions as follows and verify all the sessions are established:
    *   ATE (2-byte) - DUT (4-byte) - eBGP IPv4 with ASN < 65535 on DUT side
    *   ATE (2-byte) - DUT (4-byte) - eBGP IPv6 with ASN < 65535 on DUT side
    *   ATE (4-byte) - DUT (4-byte) - eBGP IPv4
    *   ATE (4-byte) - DUT (4-byte) - eBGP IPv6
    *   ATE (2-byte) - DUT (4-byte) - iBGP IPv4 with ASN < 65535 on DUT side
    *   ATE (4-byte) - DUT (4-byte) - iBGP IPv6 with ASN < 65535 on DUT side
    *   ATE (4-byte) - DUT (4-byte) - iBGP IPv4
    *   ATE (4-byte) - DUT (4-byte) - iBGP IPv6

*   Configure below policies and verify prefix count:
    *   Policy to reject prefix with prefix filter
    *   Policy to reject prefix with community filter
    *   Policy to reject prefix with regex filter to match as-path

## Config Parameter Coverage

*   /global/config/as
*   /neighbors/neighbor/config/peer-as
*   /neighbors/neighbor/config/local-as

## Telemetry Parameter Coverage

*   /global/config/as
*   /neighbors/neighbor/config/peer-as
*   /neighbors/neighbor/config/local-as

## OpenConfig Path and RPC Coverage

```yaml
rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Get:
    gNMI.Subscribe:
```