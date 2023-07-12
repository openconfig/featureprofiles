# TR-6.1: Remote Syslog feature config

## Summary

Verify configuration of remote syslog host (server) in DEFAULT and non-default VRF.

## Procedure
### Configuration
*   connect DUT port-1 with OTG port-1 and DUT port-2 with OTG port-2 
*   Configure DUT and OTG with:
    *   interface(port-1), interface(port-2) with IPv4 and IPv6 address
    *   Default route routes pointing OTG interface(port-1)
    *   loopback interface with IPv4 and IPv6 address and netmasks of /32, /64 respectively
* For interface and default routes in _vrf_ [DEFAULT, non-DEFAULT]
    *   Configure 1st IPv4 Syslog remote hosts in _vrf_ with:
        * facility “local7” and severity “debug”
        * (TODO when OC model published) complince to RFC5424 (structured)
        * source address equall to IPv4 address of loopback interface
    *   Configure 2nd IPv4 Syslog remote hosts in _vrf_ with:
        * facility “local7” and severity “critical” 
        * (TODO when OC model published) complince to RFC53164 (BSD/original)
        * source address equall to IPv4 address of loopback interface
    *   Configure 3nd IPv6 Syslog remote hosts in _vrf_ with:
        *   non-standard remote port 
        *   facility “local1” and severity “debug”
        *   (TODO when OC model published) complince to RFC5424 (structured)
        * source address equall to IPv6 address of loopback interface
    *   Configure 4nd IPv6 Syslog remote hosts in _vrf_ with:
        * facility “local7” and severity “critical” 
        * (TODO when OC model published) complince to RFC53164 (BSD/original)
        * source address equall to IPv6 address of loopback interface

### Test Procedure 
* Read configuration of all 4 servers, verify it matches intent
* enable packet capture on OTG port-1
* disable OTG port-2 so DUT interface(port-2) goes down, what should generate log
* Observe on OTG capture:
    *   Syslog packet w/ DstIP of host 1st and 4th and standard dstPort and facility “local7” and “local1” respectively.
    *   Syslog packet w/ DstIP of host 3rd and non-standard dstPort and facility “local1”
    *   Note: no packet w/ DstIP of 2nd host is expected.

## Config parameter coverage
*   /system/logging/remote-servers/remote-server/config/host
*   /system/logging/remote-servers/remote-server/config/network-instance
*   /system/logging/remote-servers/remote-server/config/remote-port
*   /system/logging/remote-servers/remote-server/config/source-address
*   /system/logging/remote-servers/remote-server/selectors/selector/config/facility
*   /system/logging/remote-servers/remote-server/selectors/selector/config/severity

## Telemetry parameter coverage
*   /system/logging/remote-servers/remote-server/state/host
*   /system/logging/remote-servers/remote-server/state/network-instance
*   /system/logging/remote-servers/remote-server/state/remote-port
*   /system/logging/remote-servers/remote-server/state/source-address
*   /system/logging/remote-servers/remote-server/selectors/selector/state/facility
*   /system/logging/remote-servers/remote-server/selectors/selector/state/severity


## DUT
FFF

