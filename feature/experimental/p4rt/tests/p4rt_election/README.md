# P4RT-2.1: P4RT Election

## Summary

Validate the P4RT server handles primary election and failover.

## Procedure

* Enable P4RT on a single FAP by configuring an ID on the device and one or more interfaces.
* Connect two P4RT clients to act as potential primary.
* Verify that the right clients become primary. Verify that primary can read & write and that non-primary can only read through the following scenarios:
    * Become Primary
    * Fail to become Primary 
    * Replace Primary after Failure
    * Fail To become Primary after Primary Disconnect
    * Reconnect Primary
    * Double Primary: Second client attempts a connection with the same parameters as the current primary.
    * Slave Cannot Write
    * Slave Can Read
    * Get Notified of Actual Primary
    * Zero ID Controller
    * Primary Downgrades itself
