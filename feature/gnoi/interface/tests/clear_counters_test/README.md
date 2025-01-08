# gNOI-7.1: Clear Interface Counters

## Summary

Validate that all interface statistics have been reset for the provided interface(s).

## Procedure

Scenario 1: Reset a specific interface/port
*   TODO: Use GNMI to collect the current in-octets and in-unicast-pkts count for specified interface
*   TODO: Issue gnoi.ClearInterfaceCounters to a specified interface.
*   TODO: Validate IP interface counters such as in-octets and in-unicast-pkts counters are lowers than they were before the reset.

Scenario 2: Reset all interfaces for a specific device
*   TODO: Identify all interfaces on the device that are active and serving traffic (aka in-octets and in-unicast-pkts have high counters)  
*   TODO: Issue gnoi.ClearInterfaceCounters with no target interface (aka clear all interface counters).
*   TODO: Validate IP interface counters such as in-octets and in-unicast-pkts counters have been reset for all interfaces idetified in step 1.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml
paths:
  ## State Paths ##
  /interfaces/interface/state/counters/in-octets:
  /interfaces/interface/state/counters/in-unicast-pkts:

rpcs:
  gnmi:
    gNMI.Subscribe:
  gnoi:
    (TODO) interface.interface.ClearInterfaceCounters:
```