# OC-26.1: Network Time Protocol (NTP)

## Summary

Ensure DUT can be configured as a Network Time Protocol (NTP) client.

## Procedure

*   For the following cases, enable NTP on the DUT and validate telemetry reports the servers are configured:
    *   4x IPv4 NTP server in default VRF
    *   4x IPv6 NTP server in default VRF
    *   4x IPv4 & 4x IPv6 NTP server in default VRF
    *   4x IPv4 NTP server in non-default VRF
    *   4x IPv6 NTP server in non-default VRF
    *   4x IPv4 & 4x IPv6 NTP server in non-default VRF

## Config Parameter Coverage

*   /system/ntp/config/enabled
*   /system/ntp/servers/server/config/address
*   /system/ntp/servers/server/config/network-instance

## Telemetry Parameter Coverage

*   /system/ntp/servers/server/state/address
*   /system/ntp/servers/server/state/network-instance
