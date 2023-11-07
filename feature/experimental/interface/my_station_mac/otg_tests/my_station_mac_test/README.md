# TE-1.2: My Station MAC

## Summary

Ensure my MAC entries installed on the DUT are honored and used for routing.

## Procedure

*   Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2.
*   Configure MyStationMAC whose value is 00:1A:11:00:00:01.
*   Configure a traffic flow from ATE port-1 to ATE port-2.
*   The destination MAC for the flow source is set to MyStationMAC 00:1A:11:00:00:01.
*   Validate that packets are forwarded without drops.
*   Remove the MyStationMAC configuration. 
*   Validate that traffic is blackholed.

## Config Parameter Coverage

*   /system/mac-address/config/routing-mac.

## Telemetry Parameter Coverage

*   /system/mac-address/state/routing-mac.

## Protocol/RPC Parameter Coverage

N/A
