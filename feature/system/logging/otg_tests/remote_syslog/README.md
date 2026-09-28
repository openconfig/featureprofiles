# TR-6.1: Remote Syslog feature config

## Summary

Verify configuration of remote syslog host (server) in DEFAULT and non-default
VRF.

## Procedure

*   Connect DUT port-1 with OTG port-1 and DUT port-2 with OTG port-2
*   Configure DUT $VRF-name network-instance and OTG with:
  *   interface(port-1), interface(port-2) with IPv4 and IPv6 address
  *   static host routes to syslog server addresses pointing OTG
      interface(port-1) IP
  *   loopback interface with IPv4 and IPv6 address and netmasks of /32, /64
      respectively
*   Configure syslog servers DUT
  *   Configure 1st IPv4 Syslog remote hosts in $VRF-name with:
    *   facility “local7” and severity “debug”
    *   (TODO when OC model published) compliance to RFC5424 (structured)
    *   source address equal to IPv4 address of loopback interface
  *   Configure 2nd IPv4 Syslog remote hosts in $VRF-name with:
    *   facility “local7” and severity “critical”
    *   (TODO when OC model published) compliance to RFC3164 (BSD/original)
    *   source address equal to IPv4 address of loopback interface
  *   Configure 3rd IPv6 Syslog remote hosts in $VRF-name with:
    *   non-standard remote port
    *   facility “local1” and severity “debug”
    *   (TODO when OC model published) compliance to RFC5424 (structured)
    *   source address equal to IPv6 address of loopback interface
  *   Configure 4th IPv6 Syslog remote hosts in $VRF-name with:
    *   facility “local7” and severity “critical”
    *   (TODO when OC model published) compliance to RFC3164 (BSD/original)
    *   source address equal to IPv6 address of loopback interface
*   Test Procedure
  *   Read configuration of all 4 servers, verify it matches intent
  *   enable packet capture on OTG port-1
  *   disable OTG port-2 so DUT interface(port-2) goes down, which should
      generate log
  *   Observe on OTG capture:
    *   Syslog packet w/ DstIP of host 1st and 4th and standard dstPort.
    *   Syslog packet w/ DstIP of host 3rd and non-standard dstPort
    *   Note: no packet w/ DstIP of 2nd host is expected.

### Test Case #1 - Default network instance

```
* Execute above procedure for $VRF-name = "DEFAULT" (default VRF)
```

### Test Case #2 - Non-Default network instance

```
* Execute above procedure for $VRF-name = "VRF-foo"
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  ## Config parameter coverage
  /system/logging/remote-servers/remote-server/config/host:
  /system/logging/remote-servers/remote-server/config/network-instance:
  /system/logging/remote-servers/remote-server/config/remote-port:
  /system/logging/remote-servers/remote-server/config/source-address:
  /system/logging/remote-servers/remote-server/selectors/selector/config/facility:
  /system/logging/remote-servers/remote-server/selectors/selector/config/severity:

  ## Telemetry parameter coverage
  /system/logging/remote-servers/remote-server/state/host:
  /system/logging/remote-servers/remote-server/state/network-instance:
  /system/logging/remote-servers/remote-server/state/remote-port:
  /system/logging/remote-servers/remote-server/state/source-address:
  /system/logging/remote-servers/remote-server/selectors/selector/state/facility:
  /system/logging/remote-servers/remote-server/selectors/selector/state/severity:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Subscribe:
    gNMI.Set:
```

## DUT

FFF
