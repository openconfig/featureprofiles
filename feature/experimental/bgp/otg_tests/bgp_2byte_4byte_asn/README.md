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

## Config Parameter Coverage

*   /global/config/as
*   /neighbors/neighbor/config/peer-as
*   /neighbors/neighbor/config/local-as

## Telemetry Parameter Coverage

*   /global/config/as
*   /neighbors/neighbor/config/peer-as
*   /neighbors/neighbor/config/local-as
