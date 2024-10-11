# gNMI-1.22: Controller card port attributes

## Summary

Validate PORT components attached to a CONTROLLER_CARD are modeled with the
expected OC paths.  The operational use case is:

1. As an automated network repair tool, I want to ensure at least one
   interface between redundant CONTROLLER_CARD components is fully
   operational.  Before performing a power off or reboot of a CONTROLLER_CARD
   or a network device wired to a CONTROLLER_CARD PORT, I want to use the OC
   component tree to trace the subject PORT to it's associated redundant PORT.

## Testbed type

* [Single DUT](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut.testbed)

## Testbed prerequisites

There are no DUT configuration pre-requisites for this test.  The DUT must
contain the following component types:

* At least one `CONTROLLER_CARD` component
* Each CONTROLLER_CARD must contain at least one `PORT`

## Procedure

* gNMI-1.22.1: Validate component PORT attributes attached to a CONTROLLER_CARD
  * gNMI Subscribe to the /components and /interfaces tree using ONCE option.
  * Verify each PORT present on a CONTROLLER_CARD has the following paths set:
    * Search the components to to find components of type PORT with parent = CONTROLLER_CARD
      * /components/component/state/parent = the appropriate component of type CONTROLLER_CARD
    * Search the /interfaces/interface/state/hardware-port values to find the expected /components/component/name for the physical port on the CONTROLLER_CARD
      * For each of these interfaces, verify /interfaces/interface/state/management = TRUE

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths and RPC intended to be covered by this test.

```yaml
paths:
  ## State Paths ##
    /components/component/state/name:
        platform_type: [
            "CONTROLLER_CARD",
            "PORT"
        ]
    /components/component/state/type:
        platform_type: [
            "CONTROLLER_CARD",
            "PORT"
        ]
    /interfaces/interface/state/name:
    /interfaces/interface/state/management:
    /interfaces/interface/state/hardware-port:

rpcs:
    gnmi:
        gNMI.Subscribe:
            ONCE: true
```

## Minimum DUT platform requirement

MFF - Modular form factor is specified to ensure coverage for redundant `CONTROLLER_CARD` platform_types.
