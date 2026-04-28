# gNMI-1.7: gNMI Resiliency Test

## Summary

- Generate High gNMI Load (get)
- Perform 1 LC OIR
- While LC is rebooting perform gNMI replace
- Operations should succeed once LC OIR completes

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

## Procedure

#### Initial Setup:

*   Connect DUT port-1, 2 to ATE port-1, 2
*   Configure IPv4/IPv6 addresses on the ports

### gNMI-1.7.1: Generate High gNMI Load

*   Perform a `gNMI Get` at the root level every 60 seconds or less
*   Set up gNMI subscribe with `SAMPLE` mode and sample interval of 10 second for interface counters

### gNMI-1.7.2: Trigger Line Card Reset

*   While the gNMI load is active, trigger a reset of one of the line cards

### gNMI-1.7.3: Execute gNMI Set (Replace) Operation

*   While the line cards are initializing, perform a `gNMI Set` operation that modifies or replaces a significant portion of the configuration

### gNMI-1.7.4: Verification

*   Validate if the `gNMI Set` request is successful or not
*   Validate that the `gNMI get` at the root level works through out the test
*   Validate that the `gNMI subscribe` works while the LC is operational

#### Canonical OC
```json
{
}
```

## Protocol/RPC Parameter Coverage

* gNMI
  * Get
  * Set

## Required DUT platform

* FFF

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml 
paths:


rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```
