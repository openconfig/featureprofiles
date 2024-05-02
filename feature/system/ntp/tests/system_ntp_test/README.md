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

Note:  [TODO]the source address of NTP need to be specified

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

TODO(OCPATH): Populate path from test already written.

```yaml
paths:
  ## Config paths
  /system/ntp/config/enabled:
  /system/ntp/servers/server/config/address:
  #[TODO]/system/ntp/servers/server/config/source-address:
  /system/ntp/servers/server/config/network-instance:

  ## State paths
  /system/ntp/servers/server/state/address:
  #[TODO]/system/ntp/servers/server/state/source-address:
  #[TODO]/system/ntp/servers/server/state/port:
  /system/ntp/servers/server/state/network-instance:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```
