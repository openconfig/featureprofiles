# gNMI-1.1: cli Origin

## Summary

Ensure that both CLI and OC configuration can be pushed to the device at the
same time

## Procedure

Note: this test is intended to cover only the case of pushing some configuration
along with OC paths - since it is unknown what CLI configuration would be
required in the emergency case that is covered by this requirement.

*   TODO: Push base configuration to DUT specifying an interface configuration
    for DUT port-1 and DUT port-2.

*   Validate mixed OC/CLI schema by sending both CLI and OC config concurrently
    for two different interfaces.

*   Push configuration using SetRequest specifying:

    *   `origin: "cli"` - containing modelled configuration.

        ~~~
        interface <DUT port-1>
        description foo1
        ```
        ~~~

    *   `origin: ""` (openconfig, default origin) - containing modelled
        configuration for DUT port-2.

*   Validate that DUT port-1 and DUT port-2 description through telemetry.

*   Validate order dependence by trying to modify the same config using CLI and
    OC.

*   Push configuration using SetRequest specifying:

    *   `origin: "cli"` - containing modelled configuration.

        ~~~
        interface <DUT port-1>
        description foo1
        ```
        ~~~

    *   `origin: ""` (openconfig, default origin) - containing modelled
        configuration for DUT port-1.:

*   Validate that DUT port-1 description through telemetry.

## Config Parameter coverage

## Telemetry Parameter coverage
