# XX-X.X: Remote Syslog feature config

## Summary

Verify configuration of remote syslog host (server) in DEFAULT and non-default VRF.

## Procedure
### Configuration
*   connect DUT port-1 with OTG port-1 and DUT port-2 with OTG port-2 
*   Configure DUT and OTG interfaces with:
    *   IPv4 and IPv6 address
    *   Renault routes pointing OTG interface(port-1)
* For interface and default routes in _vrf_ [DEFAULT, non-DEFAULT]
    *   For syslog compliment with [RFCxxxx (BSD/original), RFCxxx (structured)]   
        *   Configure 1st IPv4 Syslog remote hosts in _vrf_ with facility “local7” and severity “debug” 
        *   Configure 2nd IPv4 Syslog remote hosts in _vrf_ with facility “local7” and severity “critical” 
        *   Configure 3nd IPv6 Syslog remote hosts and non-standard remote port in _vrf_ with facility “local1” and severity “debug”
        *   Configure 4nd IPv6 Syslog remote hosts in _vrf_ with facility “local1” and severity “debug”
### Test Procedure 
        * Read configuration of all 4 servers, verify it matches intent
        * enable packet capture on OTG port-1
        * disable OTG port-2 so DUT interface(port-2) goes down, what should generate log
        * Observe on OTG capture:
            *   Syslog packet w/ DstIP of host 1st and 4th and standard dstPort and facility “local7” and “local1” respectively.
            *   Syslog packet w/ DstIP of host 3rd and non-standard dstPort and facility “local1”
            *   Note: no packet w/ DstIP of 2nd host is expected.

## Config parameter coverage

## Telemetry parameter coverage

## DUT
FFF

