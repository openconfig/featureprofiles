# gNMI-1.12: Mixed OpenConfig/CLI Origin

## Summary

Ensure that both CLI and OC configuration can be pushed to the device at the
same time.

## Procedure

Note: this test is intended to cover only the case of pushing some configuration
along with OC paths - since it is unknown what CLI configuration would be
required in the emergency case that is covered by this requirement.

*   Push non-overlapping mixed SetRequest specifying CLI for DUT port-1 and
    OpenConfig for DUT port-2.

    *   `origin: "cli"` containing vendor configuration.

        ```
        interface <DUT port-1>
          description foo1
        ```

    *   `origin: ""` (openconfig, default origin) setting the DUT port-2 string
        value at `/interfaces/interface/config/description` to `"foo2"`.

*   Validate the DUT port-1 and DUT port-2 descriptions through telemetry.

*   Push overlapping mixed SetRequest specifying CLI before OpenConfig for DUT
    port-1.

    *   `origin: "cli"` containing vendor configuration.

        ```
        interface <DUT port-1>
          description from cli
        ```

    *   `origin: ""` (openconfig, default origin) setting the DUT port-1 string
        value at `/interfaces/interface/config/description` to `"from oc"`.

*   Validate that DUT port-1 description is `"from oc"`.

*   Push overlapping mixed SetRequest specifying OpenConfig before CLI for
    DUT port-1.

    *   `origin: ""` (openconfig, default origin) setting the DUT port-1 string
        value at `/interfaces/interface/config/description` to `"from oc"`.

    *   `origin: "cli"` containing vendor configuration.

        ```
        interface <DUT port-1>
          description from cli
        ```

*   Validate that DUT port-1 description is `"from cli"`.
