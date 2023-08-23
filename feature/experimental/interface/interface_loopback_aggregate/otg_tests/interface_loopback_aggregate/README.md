# RT-5.6: Interface Loopback mode

## Summary

Ensure Interface mode can be set to loopback mode and can be added as part of static LAG.

## Procedure

### TestCase-1:

*   Configure DUT port-1 to OTG port-1.
*   Admin down OTG port-1.
*   Verify DUT port-1 is down.
*   On DUT, set LAG interface “loopback mode” to “FACILITY” on AE interface.
*   Add port-1 as part of Static LAG (lacp mode static(on)).
*   Validate that port-1 operational status is “UP”.
*   Validate on DUT that LAG interface status is “UP”.

## Config Parameter Coverage

*   /interfaces/interface/config/loopback-mode
*   /interfaces/interface/ethernet/config/port-speed
*   /interfaces/interface/ethernet/config/duplex-mode
*   /interfaces/interface/ethernet/config/aggregate-id
*   /interfaces/interface/aggregation/config/lag-type
*   /interfaces/interface/aggregation/config/min-links

## Telemetry Parameter Coverage

*   /interfaces/interface/state/loopback-mode

## Protocol/RPC Parameter Coverage

None

## Minimum DUT Platform Requirement

vRX
