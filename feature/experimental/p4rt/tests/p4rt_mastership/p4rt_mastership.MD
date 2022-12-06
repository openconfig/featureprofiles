# P4RT-2.1: P4RT Mastership

## Summary

Validate the P4RT server handles master election and failover

## Procedure

* Enable P4RT on a single FAP by configuring an ID on the device and one or more interfaces.
* Connect two P4RT clients to act as potential masters.
* Verify that the right clients become masters. Verify that masters can read & write and that non-masters can only read through the following scenarios:
    * Become Master
    * Fail to become Master 
    * Replace Master after Failure
    * Fail To become Master after Master Disconnect
    * Reconnect Master
    * Double Master
    * Slave Cannot Write
    * Slave Can Read
    * Get Notified of Actual Master
    * Zero ID Controller
    * Master Downgrades itself
