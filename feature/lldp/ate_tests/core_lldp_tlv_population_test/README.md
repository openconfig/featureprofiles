# RT-6.1: Core LLDP TLV Population

## Summary

Determine LLDP advertisement and reception operates correctly.

## Procedure

*   Connect ATE port-1 to DUT port-1, enable LLDP on each port.
*   Configure LLDP enabled=true for DUT port-1 and ATE port-1
*   Determine that telemetry from:
    *   ATE reports correct values from DUT.
    *   DUT reports correct values from ATE.
*   Configure LLDP is disabled at global level for DUT.
    *   Set /lldp/config/enabled to FALSE.
    *   Ensure that DUT does not generate any LLDP messages irrespective of the
        configuration of lldp/interfaces/interface/config/enabled (TRUE or
        FALSE) on any interface.

## Config Parameter coverage

*   /lldp/config/enabled
*   /lldp/interfaces/interface/config/enabled

## Telemetry Parameter coverage

*   /lldp/interfaces/interface/neighbors/neighbor/state/chassis-id
*   /lldp/interfaces/interface/neighbors/neighbor/state/chassis-id-subtype
*   /lldp/interfaces/interface/neighbors/neighbor/state/port-id
*   /lldp/interfaces/interface/neighbors/neighbor/state/port-id-subtype
*   /lldp/interfaces/interface/neighbors/neighbor/state/system-name
*   /lldp/interfaces/interface/state/name
*   /lldp/state/chassis-id
*   /lldp/state/chassis-id-type
*   /lldp/state/system-name

## Protocol/RPC Parameter coverage

LLDP:

*   /lldp/config/enabled = true
*   /lldp/interfaces/interface/config/enabled = true

## Minimum DUT platform requirement

vRX
