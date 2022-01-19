# RT-6.1: Core LLDP TLV Population

## Summary

Determine LLDP configuration, advertisement and reception operates correctly.

## Procedure

* Connect ATE port-1 to DUT port-1, enable LLDP on each port.
* Configure LLDP enabled=true  for DUT port-1 and ATE port-1
* Determine that telemetry from:
  * ATE reports correct values from DUT.
  * DUT reports correct values from ATE.
* Configure LLDP is disabled at global level for DUT. 
  * Set /lldp/config/enabled to FALSE.
  * Ensure that DUT does not generate any LLDP messages irrespective of the
    configuration of lldp/interfaces/interface/config/enabled (TRUE or FALSE)
    on any interface.
